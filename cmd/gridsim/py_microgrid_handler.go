package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gridsim/pkg/config"
	"gridsim/pkg/protocol/modbus_bridge"
	"github.com/xuri/excelize/v2"
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
		case strings.HasPrefix(action, "config"):
			ws.handlePyMicrogridConfig(w, r, id)
		case strings.HasPrefix(action, "curves"):
			ws.handlePyMicrogridCurves(w, r, id)
		case strings.HasPrefix(action, "upload-curve"):
			ws.handlePyMicrogridUploadCurve(w, r, id)
		case strings.HasPrefix(action, "export-points"):
			ws.handlePyMicrogridExportPoints(w, r, id)
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
	scriptDir, deviceJSON, err := ws.resolveScriptDir(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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

	scriptDir, deviceJSON, err := ws.resolveScriptDir(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
			"start_time":  cfg.ModbusBridgeConfig.StartTime,
			"modbus_port": cfg.ModbusBridgeConfig.ModbusPort,
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

func (ws *webServer) resolveScriptDir(id string) (string, string, error) {
	cfg, ok := ws.mgr.GetConfig(id)
	if !ok || cfg.Protocol != "modbus_bridge" {
		return "", "", fmt.Errorf("not a modbus_bridge instance")
	}
	bc := cfg.ModbusBridgeConfig
	if bc == nil {
		return "", "", fmt.Errorf("modbus_bridge_config is nil")
	}

	// Prefer per-instance isolated workspace (py_instances/<id>/) if it exists
	instanceWorkDir := filepath.Join(ws.cfgDir, "py_instances", id)
	if info, err := os.Stat(instanceWorkDir); err == nil && info.IsDir() {
		deviceJSON := bc.DeviceJSON
		if deviceJSON == "" {
			deviceJSON = "config/device.json"
		}
		return instanceWorkDir, deviceJSON, nil
	}

	// Fallback to configured or default script_dir
	scriptDir := bc.ScriptDir
	if scriptDir == "" {
		scriptDir = "py_simulator"
	}
	if !filepath.IsAbs(scriptDir) {
		scriptDir = filepath.Join(ws.cfgDir, scriptDir)
	}
	deviceJSON := bc.DeviceJSON
	if deviceJSON == "" {
		deviceJSON = "config/device.json"
	}
	return scriptDir, deviceJSON, nil
}

func (ws *webServer) handlePyMicrogridConfig(w http.ResponseWriter, r *http.Request, id string) {
	scriptDir, deviceJSON, err := ws.resolveScriptDir(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fullPath := filepath.Join(scriptDir, deviceJSON)

	switch r.Method {
	case http.MethodGet:
		data, err := os.ReadFile(fullPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "read config: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "read body: "+err.Error())
			return
		}
		// Validate JSON
		var check map[string]interface{}
		if err := json.Unmarshal(body, &check); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		// Pretty print and save
		pretty, _ := json.MarshalIndent(check, "", "  ")
		if err := os.WriteFile(fullPath, pretty, 0644); err != nil {
			writeError(w, http.StatusInternalServerError, "write config: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handlePyMicrogridCurves(w http.ResponseWriter, r *http.Request, id string) {
	scriptDir, _, err := ws.resolveScriptDir(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	configDir := filepath.Join(scriptDir, "config")
	entries, _ := os.ReadDir(configDir)
	files := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".csv") {
			files = append(files, e.Name())
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"files": files})
}

func (ws *webServer) handlePyMicrogridUploadCurve(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	scriptDir, _, err := ws.resolveScriptDir(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "parse form: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no file: "+err.Error())
		return
	}
	defer file.Close()

	dst := filepath.Join(scriptDir, "config", header.Filename)
	out, err := os.Create(dst)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create file: "+err.Error())
		return
	}
	defer out.Close()
	io.Copy(out, file)
	writeJSON(w, http.StatusOK, map[string]string{"filename": header.Filename})
}

func (ws *webServer) handlePyMicrogridExportPoints(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	scriptDir, deviceJSON, err := ws.resolveScriptDir(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	devices, err := modbus_bridge.ParseDeviceJSON(scriptDir, deviceJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "parse devices: "+err.Error())
		return
	}

	// Create a zip archive containing one xlsx per device
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=py-microgrid-points.zip")

	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	headers := []string{"point-name", "point-number", "value-type", "point-type", "coefficient", "base-value", "alias", "register-address", "function-code", "data-format", "slave-id", "device-type", "device-key"}

	for _, dev := range devices {
		// Create xlsx for this device
		f := excelize.NewFile()

		// Sheet "说明"
		f.SetSheetName("Sheet1", "说明")
		f.SetCellValue("说明", "A1", "Python微电网模拟器点表")
		f.SetCellValue("说明", "A2", fmt.Sprintf("设备: %s", dev.DeviceKey))
		f.SetCellValue("说明", "A3", fmt.Sprintf("类型: %s", dev.DeviceType))
		f.SetCellValue("说明", "A4", fmt.Sprintf("Slave ID: %d", dev.SlaveID))
		f.SetCellValue("说明", "A5", fmt.Sprintf("测点数: %d", len(dev.Points)))
		f.SetCellValue("说明", "A7", "协议: Modbus TCP")
		f.SetCellValue("说明", "A8", "数据编码: INT32 (实际值×100)")
		f.SetCellValue("说明", "A9", "字节序: Big-Endian (ABCD)")
		f.SetCellValue("说明", "A10", "每测点占2个Holding Registers")

		// Sheet "point" (必须)
		f.NewSheet("point")
		for i, h := range headers {
			col, _ := excelize.ColumnNumberToName(i + 1)
			f.SetCellValue("point", col+"1", h)
		}

		rowIdx := 2
		for _, pt := range dev.Points {
			pointType := "AI"
			if pt.PointType == config.TypeAO {
				pointType = "AO"
			} else if pt.PointType == config.TypeDI {
				pointType = "DI"
			}
			fc := 3
			if pt.Writable {
				fc = 16
			}
			// alias = register identifier (e.g., BS.ActivePW)
			regName := pt.Name
			if dotIdx := len(dev.DeviceKey) + 1; dotIdx < len(pt.Name) {
				regName = pt.Name[dotIdx:]
			}

			col := func(n int) string { c, _ := excelize.ColumnNumberToName(n); return c }
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(1), rowIdx), pt.Name)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(2), rowIdx), pt.IOA)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(3), rowIdx), "float")
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(4), rowIdx), pointType)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(5), rowIdx), 1)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(6), rowIdx), 0)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(7), rowIdx), regName)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(8), rowIdx), pt.RegisterOffset)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(9), rowIdx), fc)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(10), rowIdx), "SW_FLOAT")
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(11), rowIdx), dev.SlaveID)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(12), rowIdx), dev.DeviceType)
			f.SetCellValue("point", fmt.Sprintf("%s%d", col(13), rowIdx), dev.DeviceKey)
			rowIdx++
		}

		// Write xlsx into the zip archive
		zipEntry, err := zipWriter.Create(dev.DeviceKey + ".xlsx")
		if err != nil {
			continue
		}
		f.Write(zipEntry)
		f.Close()
	}
}
