package microgrid

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/xuri/excelize/v2"
	"gridsim/internal/model"
	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

// uidCounter is a simple incrementing counter for generating unique formula IDs.
var uidCounter int64

// ManagerBridge is the subset of manager.Manager needed by microgrid handlers.
type ManagerBridge interface {
	GetConfig(id string) (model.InstanceConfig, bool)
	UpdateConfig(cfg model.InstanceConfig) error
	SaveConfigOnly(cfg model.InstanceConfig) error
	GetMicrogridEngine(id string) *Engine
	GetStore(id string) *library.Store
	GetAutoChangeActiveIOAs(id string) []uint32
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
			saveTopology(mgr, id, cfg, &topo)
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
			dev.ID = fmt.Sprintf("dev-%d", atomic.AddInt64(&uidCounter, 1))
			if dev.Switch.Name == "" {
				dev.Switch.Name = fmt.Sprintf("QF%d", len(topo.Devices)+1)
			}
			// 系统自动分配 IOABase
			dev.IOABase = nextAvailableIOABase(topo.Devices)
			if dev.IOABase == 0 {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "IOA 地址空间已满"})
				return
			}
			topo.Devices = append(topo.Devices, dev)
			saveTopology(mgr, id, cfg, &topo)
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
			saveTopology(mgr, id, cfg, &topo)
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
			saveTopology(mgr, id, cfg, &topo)
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
			// Persist switch state to config so state survives restart
			if cfg, ok := mgr.GetConfig(id); ok {
				topo := defaultTopology()
				if cfg.MicrogridConfig != nil && cfg.MicrogridConfig.TopologyJSON != "" {
					json.Unmarshal([]byte(cfg.MicrogridConfig.TopologyJSON), &topo)
				}
				for i := range topo.Devices {
					if topo.Devices[i].ID == devID {
						topo.Devices[i].Switch.Closed = closed
						break
					}
				}
				b, _ := json.Marshal(topo)
				if cfg.MicrogridConfig == nil {
					cfg.MicrogridConfig = &model.MicrogridInstanceConfig{}
				}
				cfg.MicrogridConfig.TopologyJSON = string(b)
				mgr.SaveConfigOnly(cfg)
			}
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
		saveTopology(mgr, id, cfg, &topo)
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
			sort.Slice(pts, func(i, j int) bool { return pts[i].IOA < pts[j].IOA })
			result := make([]map[string]interface{}, len(pts))

			// Get IOAs with active auto-change strategies (local mode)
			localIOAs := make(map[uint32]bool)
			if activeIOAs := mgr.GetAutoChangeActiveIOAs(id); activeIOAs != nil {
				for _, ioa := range activeIOAs {
					localIOAs[ioa] = true
				}
			}

			// Build toggle-able points: AI power measurement points can switch remote/local
			canToggle := make(map[string]bool)
			if cfg, ok := mgr.GetConfig(id); ok && cfg.MicrogridConfig != nil && cfg.MicrogridConfig.TopologyJSON != "" {
				var topo Topology
				json.Unmarshal([]byte(cfg.MicrogridConfig.TopologyJSON), &topo)
				for idx, dev := range topo.Devices {
					prefix := typeChinese[dev.Type] + itoa(idx+1)
					switch dev.Type {
					case CompPV:
						canToggle[prefix+"_有功功率"] = true
					case CompBattery:
						canToggle[prefix+"_充放电功率"] = true
					case CompLoad:
						canToggle[prefix+"_有功功率"] = true
					case CompCharger:
						canToggle[prefix+"_充电功率"] = true
					}
				}
				canToggle["关口表_有功功率"] = false // grid meter is always remote
			}

			for i, p := range pts {
				unit := ""
				if idx := strings.Index(p.Alias, "|"); idx >= 0 { unit = p.Alias[idx+1:] }
				result[i] = map[string]interface{}{
					"ioa": p.IOA, "name": p.Name, "point_type": string(p.PointType),
					"value": p.Value, "unit": unit,
					"can_toggle": canToggle[p.Name],
					"local_mode": localIOAs[p.IOA],
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
			rule.ID = fmt.Sprintf("fml-%d", atomic.AddInt64(&uidCounter, 1))
			topo.Formulas = append(topo.Formulas, rule)
			saveTopology(mgr, id, cfg, &topo)
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
			saveTopology(mgr, id, cfg, &topo)
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
			saveTopology(mgr, id, cfg, &topo)
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

// HandleMicrogridExportXLSX GET /api/v1/microgrid/{id}/export-xlsx
func HandleMicrogridExportXLSX(mgr ManagerBridge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := extractMicrogridID(r.URL.Path, "/api/v1/microgrid/", "/export-xlsx")

		var pts []*config.Point
		if store := mgr.GetStore(id); store != nil {
			pts = store.GetAll()
			sort.Slice(pts, func(i, j int) bool { return pts[i].IOA < pts[j].IOA })
		} else {
			// Fallback: read from topology config
			cfg, ok := mgr.GetConfig(id)
			if !ok || cfg.MicrogridConfig == nil || cfg.MicrogridConfig.TopologyJSON == "" {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "topology not configured"})
				return
			}
			var topo Topology
			json.Unmarshal([]byte(cfg.MicrogridConfig.TopologyJSON), &topo)
			pts = topo.ExpandPoints()
		}

		if len(pts) == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no points"})
			return
		}

		// Group by device: extract prefix before last "_"
		groups := make(map[string][]*config.Point)
		for _, p := range pts {
			lastIdx := strings.LastIndex(p.Name, "_")
			prefix := p.Name
			if lastIdx > 0 { prefix = p.Name[:lastIdx] }
			groups[prefix] = append(groups[prefix], p)
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+id+`_points.zip"`)
		zw := zip.NewWriter(w)
		defer zw.Close()

		// 1) Per-device xlsx
		for prefix, groupPts := range groups {
			if len(groupPts) == 0 { continue }
			entry, _ := zw.Create(prefix + ".xlsx")
			writePointXLSX(entry, groupPts)
		}

		// 2) Complete xlsx with all points
		allEntry, _ := zw.Create("完整点表.xlsx")
		writePointXLSX(allEntry, pts)
	}
}

// writePointXLSX writes a point table xlsx into w.
func writePointXLSX(w io.Writer, pts []*config.Point) {
	f := excelize.NewFile()
	sheet := "point"
	f.SetSheetName("Sheet1", sheet)
	headers := []string{"point-name", "point-number", "value-type", "point-type", "efficient", "base-value", "alias"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for i, p := range pts {
		row := i + 2
		vt := "DOUBLE"
		switch p.ValueType {
		case config.VTFloat: vt = "DOUBLE"
		case config.VTBit: vt = "BIT"
		case config.VTInt: vt = "INT"
		}
		alias := p.Alias
		if idx := strings.Index(alias, "|"); idx >= 0 { alias = p.Alias[:idx] }
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), p.Name)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), p.IOA)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), vt)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), string(p.PointType))
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), p.Efficient)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), 0) // 导出的点表 base-value 统一为 0，实际基值由 ExpandPoints 定义
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), alias)
	}
	f.Write(w)
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

func saveTopology(mgr ManagerBridge, id string, cfg model.InstanceConfig, topo *Topology) {
	b, _ := json.Marshal(topo)
	if cfg.MicrogridConfig == nil {
		cfg.MicrogridConfig = &model.MicrogridInstanceConfig{}
	}
	cfg.MicrogridConfig.TopologyJSON = string(b)

	pts := topo.ExpandPoints()
	type ptEntry struct {
		IOA  uint32 `json:"ioa"`
		Name string `json:"name"`
		Type string `json:"type"`
	}
	var entries []ptEntry
	for _, p := range pts {
		entries = append(entries, ptEntry{IOA: p.IOA, Name: p.Name, Type: string(p.PointType)})
	}
	ptJSON, _ := json.Marshal(entries)
	cfg.MicrogridConfig.PointsJSON = string(ptJSON)

	mgr.SaveConfigOnly(cfg)

	// IOA 冲突检查：仅阻止引擎热加载，配置仍然保存
	if err := validateIOAUnique(topo); err != nil {
		slog.Warn("Topology has IOA conflicts, engine not hot-reloaded", "id", id, "error", err)
		return
	}

	if eng := mgr.GetMicrogridEngine(id); eng != nil {
		eng.ReloadTopology(topo)
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
