package microgrid

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gridsim/internal/model"
	"gridsim/pkg/library"
)

// uidCounter is a simple incrementing counter for generating unique formula IDs.
var uidCounter int64

// ManagerBridge is the subset of manager.Manager needed by microgrid handlers.
// Defined here to avoid an import cycle (manager → microgrid → manager).
type ManagerBridge interface {
	GetConfig(id string) (model.InstanceConfig, bool)
	UpdateConfig(cfg model.InstanceConfig) error
	GetMicrogridEngine(id string) *Engine
	GetStore(id string) *library.Store
}

// HandleMicrogridTopology GET/PUT /api/v1/microgrid/{id}/topology
func HandleMicrogridTopology(mgr ManagerBridge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := extractMicrogridID(r.URL.Path, "/api/v1/microgrid/", "/topology")

		switch r.Method {
		case http.MethodGet:
			cfg, ok := mgr.GetConfig(id)
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			topo := defaultTopology()
			if cfg.MicrogridConfig != nil && cfg.MicrogridConfig.TopologyJSON != "" {
				json.Unmarshal([]byte(cfg.MicrogridConfig.TopologyJSON), &topo)
			}
			writeJSON(w, http.StatusOK, topo)

		case http.MethodPut:
			cfg, ok := mgr.GetConfig(id)
			if !ok {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			var topo Topology
			if err := json.NewDecoder(r.Body).Decode(&topo); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
				return
			}
			b, _ := json.Marshal(topo)
			if cfg.MicrogridConfig == nil {
				cfg.MicrogridConfig = &model.MicrogridInstanceConfig{}
			}
			cfg.MicrogridConfig.TopologyJSON = string(b)
			if err := mgr.UpdateConfig(cfg); err != nil {
				writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	}
}

// HandleMicrogridDevice 设备增删改
func HandleMicrogridDevice(mgr ManagerBridge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		id := extractMicrogridID(path, "/api/v1/microgrid/", "/device")

		cfg, ok := mgr.GetConfig(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		topo := defaultTopology()
		if cfg.MicrogridConfig != nil && cfg.MicrogridConfig.TopologyJSON != "" {
			json.Unmarshal([]byte(cfg.MicrogridConfig.TopologyJSON), &topo)
		}

		switch r.Method {
		case http.MethodPost:
			var dev Device
			if err := json.NewDecoder(r.Body).Decode(&dev); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			dev.ID = fmt.Sprintf("dev-%d", len(topo.Devices)+1)
			if dev.Switch.Name == "" {
				dev.Switch.Name = fmt.Sprintf("QF%d", len(topo.Devices)+1)
			}
			topo.Devices = append(topo.Devices, dev)
			saveTopology(mgr, cfg, &topo)
			writeJSON(w, http.StatusCreated, dev)

		case http.MethodPut:
			var dev Device
			if err := json.NewDecoder(r.Body).Decode(&dev); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			updated := false
			for i := range topo.Devices {
				if topo.Devices[i].ID == dev.ID {
					topo.Devices[i] = dev
					updated = true
					break
				}
			}
			if !updated {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
				return
			}
			saveTopology(mgr, cfg, &topo)
			writeJSON(w, http.StatusOK, dev)

		case http.MethodDelete:
			devID := strings.TrimPrefix(path, "/api/v1/microgrid/"+id+"/device/")
			found := false
			for i := range topo.Devices {
				if topo.Devices[i].ID == devID {
					topo.Devices = append(topo.Devices[:i], topo.Devices[i+1:]...)
					found = true
					break
				}
			}
			if !found {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "device not found"})
				return
			}
			saveTopology(mgr, cfg, &topo)
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	}
}

// HandleMicrogridControl 开关控制 /api/v1/microgrid/{id}/control/{devId}?closed=true
func HandleMicrogridControl(mgr ManagerBridge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		path := r.URL.Path
		parts := strings.Split(strings.TrimPrefix(path, "/api/v1/microgrid/"), "/")
		if len(parts) < 3 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
			return
		}
		id := parts[0]
		devID := parts[2]

		closed := r.URL.Query().Get("closed") != "false"

		if eng := mgr.GetMicrogridEngine(id); eng != nil {
			eng.SetSwitch(devID, closed)
			writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "closed": closed})
			return
		}

		cfg, ok := mgr.GetConfig(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		topo := defaultTopology()
		if cfg.MicrogridConfig != nil && cfg.MicrogridConfig.TopologyJSON != "" {
			json.Unmarshal([]byte(cfg.MicrogridConfig.TopologyJSON), &topo)
		}
		if _, err := topo.ToggleSwitch(devID); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		saveTopology(mgr, cfg, &topo)
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "ok", "closed": closed})
	}
}

// HandleMicrogridDashboard GET /api/v1/microgrid/{id}/dashboard
func HandleMicrogridDashboard(mgr ManagerBridge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
			return
		}
		id := extractMicrogridID(r.URL.Path, "/api/v1/microgrid/", "/dashboard")
		if eng := mgr.GetMicrogridEngine(id); eng != nil {
			dash := eng.Dashboard()
			writeJSON(w, http.StatusOK, dash)
			return
		}
		cfg, ok := mgr.GetConfig(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var topo Topology
		if cfg.MicrogridConfig != nil && cfg.MicrogridConfig.TopologyJSON != "" {
			json.Unmarshal([]byte(cfg.MicrogridConfig.TopologyJSON), &topo)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":              "stopped",
			"device_count":        len(topo.Devices),
			"total_generation_kw": 0,
			"total_load_kw":       0,
			"grid_power_kw":       0,
		})
	}
}

// HandleMicrogridPoints GET /api/v1/microgrid/{id}/points
func HandleMicrogridPoints(mgr ManagerBridge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := extractMicrogridID(r.URL.Path, "/api/v1/microgrid/", "/points")

		if store := mgr.GetStore(id); store != nil {
			pts := store.GetAll()
			result := make([]map[string]interface{}, len(pts))
			for i, p := range pts {
				unit := ""
			if idx := strings.Index(p.Alias, "|"); idx >= 0 {
				unit = p.Alias[idx+1:]
			}
			result[i] = map[string]interface{}{
				"ioa":        p.IOA,
				"name":       p.Name,
				"point_type": string(p.PointType),
				"value":      p.Value,
				"unit":       unit,
			}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"points": result})
			return
		}

		cfg, ok := mgr.GetConfig(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		var topo Topology
		if cfg.MicrogridConfig != nil && cfg.MicrogridConfig.TopologyJSON != "" {
			json.Unmarshal([]byte(cfg.MicrogridConfig.TopologyJSON), &topo)
		}
		points := topo.FormatPointTable()
		writeJSON(w, http.StatusOK, map[string]interface{}{"points": points})
	}
}

// HandleMicrogridFormulas GET/POST/PUT/DELETE /api/v1/microgrid/{id}/formulas
func HandleMicrogridFormulas(mgr ManagerBridge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		id := extractMicrogridID(r.URL.Path, "/api/v1/microgrid/", "/formulas")

		cfg, ok := mgr.GetConfig(id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		topo := defaultTopology()
		if cfg.MicrogridConfig != nil && cfg.MicrogridConfig.TopologyJSON != "" {
			json.Unmarshal([]byte(cfg.MicrogridConfig.TopologyJSON), &topo)
		}

		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, topo.Formulas)

		case http.MethodPost:
			var rule FormulaRule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			uidCounter++
			rule.ID = fmt.Sprintf("fml-%d", uidCounter)
			topo.Formulas = append(topo.Formulas, rule)
			saveTopology(mgr, cfg, &topo)
			writeJSON(w, http.StatusCreated, rule)

		case http.MethodPut:
			var rule FormulaRule
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			updated := false
			for i, f := range topo.Formulas {
				if f.ID == rule.ID {
					topo.Formulas[i] = rule
					updated = true
					break
				}
			}
			if !updated {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "formula not found"})
				return
			}
			saveTopology(mgr, cfg, &topo)
			writeJSON(w, http.StatusOK, rule)

		case http.MethodDelete:
			// DELETE /api/v1/microgrid/{id}/formulas/{formulaID}
			formulaID := strings.TrimPrefix(r.URL.Path, "/api/v1/microgrid/"+id+"/formulas/")
			found := false
			for i, f := range topo.Formulas {
				if f.ID == formulaID {
					topo.Formulas = append(topo.Formulas[:i], topo.Formulas[i+1:]...)
					found = true
					break
				}
			}
			if !found {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "formula not found"})
				return
			}
			saveTopology(mgr, cfg, &topo)
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	}
}

// ─── helpers ───

func extractMicrogridID(path, prefix, suffix string) string {
	s := strings.TrimPrefix(path, prefix)
	if idx := strings.Index(s, "/"); idx > 0 {
		return s[:idx]
	}
	return strings.TrimSuffix(s, suffix)
}

func defaultTopology() Topology {
	return Topology{
		BusName:      "10kV 母线",
		BusVoltageKV: 10,
		GridMeter: GridMeterConfig{
			RatedCapacityKW: 500,
		},
	}
}

func saveTopology(mgr ManagerBridge, cfg model.InstanceConfig, topo *Topology) {
	b, _ := json.Marshal(topo)
	if cfg.MicrogridConfig == nil {
		cfg.MicrogridConfig = &model.MicrogridInstanceConfig{}
	}
	cfg.MicrogridConfig.TopologyJSON = string(b)
	mgr.UpdateConfig(cfg)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
