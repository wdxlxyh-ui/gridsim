package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"gridsim/pkg/config"
	"gridsim/pkg/protocol/modbus_bridge"
)

func (ws *webServer) registerPyMicrogridRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/py-microgrid/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path

		// Extract instance ID: /api/v1/py-microgrid/{id}/...
		trimmed := strings.TrimPrefix(path, "/api/v1/py-microgrid/")
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, http.StatusBadRequest, "missing instance ID")
			return
		}
		id := parts[0]
		action := ""
		if len(parts) > 1 {
			action = parts[1]
		}

		switch {
		case strings.HasPrefix(action, "dashboard"):
			ws.handlePyMicrogridDashboard(w, r, id)
		case strings.HasPrefix(action, "topology"):
			ws.handlePyMicrogridTopology(w, r, id)
		case strings.HasPrefix(action, "control"):
			ws.handlePyMicrogridControl(w, r, id)
		default:
			writeError(w, http.StatusNotFound, "unknown py-microgrid endpoint: "+action)
		}
	})
}

func (ws *webServer) handlePyMicrogridDashboard(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	inst := ws.mgr.GetInstance(id)
	if inst == nil || inst.Store == nil {
		writeError(w, http.StatusNotFound, "instance not running")
		return
	}

	cfg, ok := ws.mgr.GetConfig(id)
	if !ok || cfg.Protocol != "modbus_bridge" {
		writeError(w, http.StatusBadRequest, "not a modbus_bridge instance")
		return
	}

	// Parse device mappings to know which IOAs belong to which device type
	bc := cfg.ModbusBridgeConfig
	scriptDir := bc.ScriptDir
	if scriptDir == "" {
		scriptDir = "py_simulator"
	}
	if scriptDir != "" && scriptDir[0] != '/' && (len(scriptDir) < 2 || scriptDir[1] != ':') {
		scriptDir = ws.cfgDir + "/" + scriptDir
	}
	deviceJSON := bc.DeviceJSON
	if deviceJSON == "" {
		deviceJSON = "config/device.json"
	}

	devices, err := modbus_bridge.ParseDeviceJSON(scriptDir, deviceJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "parse device.json: "+err.Error())
		return
	}

	// Aggregate power data from store
	type devInfo struct {
		ID      string  `json:"id"`
		Name    string  `json:"name"`
		PowerKW float64 `json:"power_kw"`
		Closed  bool    `json:"closed"`
		SOC     float64 `json:"soc,omitempty"`
		Mode    string  `json:"mode,omitempty"`
	}

	var (
		totalPV      float64
		totalBat     float64
		totalLoad    float64
		totalCharger float64
		gridPower    float64
		batterySoc   float64
		pvList       []devInfo
		batList      []devInfo
		loadList     []devInfo
		chargerList  []devInfo
	)

	for _, dev := range devices {
		power := 0.0
		soc := 0.0
		// Read first point (offset 0) as power for all device types
		if len(dev.Points) > 0 {
			pt, ok := inst.Store.Get(dev.Points[0].IOA)
			if ok {
				power = pt.Value
			}
		}

		switch dev.DeviceType {
		case "PV":
			totalPV += power
			pvList = append(pvList, devInfo{ID: dev.DeviceKey, Name: dev.DeviceKey, PowerKW: power, Closed: true, Mode: "csv"})
		case "BESS":
			totalBat += power
			// SOC is the second point (index 1, register offset 2)
			if len(dev.Points) > 1 {
				pt, ok := inst.Store.Get(dev.Points[1].IOA)
				if ok {
					soc = pt.Value
				}
			}
			batterySoc = soc
			mode := "idle"
			if power > 0 {
				mode = "charging"
			} else if power < 0 {
				mode = "discharging"
			}
			batList = append(batList, devInfo{ID: dev.DeviceKey, Name: dev.DeviceKey, PowerKW: power, Closed: true, SOC: soc, Mode: mode})
		case "EV":
			totalCharger += power
			mode := "AC"
			chargerList = append(chargerList, devInfo{ID: dev.DeviceKey, Name: dev.DeviceKey, PowerKW: power, Closed: true, Mode: mode})
		case "Load":
			totalLoad += power
			loadList = append(loadList, devInfo{ID: dev.DeviceKey, Name: dev.DeviceKey, PowerKW: power, Closed: true, Mode: "csv"})
		case "Meter":
			gridPower = power
		}
	}

	status := "stopped"
	state, _ := ws.mgr.GetState(id)
	if state != nil && state.Status == "running" {
		status = "running"
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":           status,
		"grid_power_kw":    gridPower,
		"total_pv_kw":      totalPV,
		"total_bat_kw":     totalBat,
		"total_load_kw":    totalLoad,
		"total_charger_kw": totalCharger,
		"battery_soc":      batterySoc,
		"pv":               pvList,
		"battery":          batList,
		"load":             loadList,
		"charger":          chargerList,
		"device_count":     len(devices),
	})
}

func (ws *webServer) handlePyMicrogridTopology(w http.ResponseWriter, r *http.Request, id string) {
	cfg, ok := ws.mgr.GetConfig(id)
	if !ok || cfg.Protocol != "modbus_bridge" {
		writeError(w, http.StatusBadRequest, "not a modbus_bridge instance")
		return
	}

	bc := cfg.ModbusBridgeConfig
	scriptDir := bc.ScriptDir
	if scriptDir == "" {
		scriptDir = "py_simulator"
	}
	if scriptDir != "" && scriptDir[0] != '/' && (len(scriptDir) < 2 || scriptDir[1] != ':') {
		scriptDir = ws.cfgDir + "/" + scriptDir
	}
	deviceJSON := bc.DeviceJSON
	if deviceJSON == "" {
		deviceJSON = "config/device.json"
	}

	switch r.Method {
	case http.MethodGet:
		devices, err := modbus_bridge.ParseDeviceJSON(scriptDir, deviceJSON)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "parse device.json: "+err.Error())
			return
		}

		type deviceResp struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Name       string `json:"name"`
			SlaveID    uint8  `json:"slave_id"`
			PointCount int    `json:"point_count"`
			IOABase    uint32 `json:"ioa_base"`
		}

		devList := make([]deviceResp, 0, len(devices))
		for _, d := range devices {
			ioaBase := uint32(0)
			if len(d.Points) > 0 {
				ioaBase = d.Points[0].IOA
			}
			devList = append(devList, deviceResp{
				ID:         d.DeviceKey,
				Type:       d.DeviceType,
				Name:       d.DeviceKey,
				SlaveID:    d.SlaveID,
				PointCount: len(d.Points),
				IOABase:    ioaBase,
			})
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"bus_name":    "AC Bus",
			"bus_voltage": 0.4,
			"start_time":  bc.StartTime,
			"modbus_port": bc.ModbusPort,
			"devices":     devList,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handlePyMicrogridControl(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	inst := ws.mgr.GetInstance(id)
	if inst == nil || inst.Store == nil {
		writeError(w, http.StatusNotFound, "instance not running")
		return
	}

	var body struct {
		IOA   uint32  `json:"ioa"`
		Value float64 `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	pt, ok := inst.Store.Get(body.IOA)
	if !ok {
		writeError(w, http.StatusNotFound, "point not found")
		return
	}

	if pt.PointType != config.TypeAO {
		writeError(w, http.StatusBadRequest, "only AO points can be controlled")
		return
	}

	inst.Store.SetValue(body.IOA, body.Value)
	inst.Protocol.Publish(pt)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"ioa":     body.IOA,
		"value":   body.Value,
	})
}
