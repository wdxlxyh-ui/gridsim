package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gridsim/internal/model"
)

// handlePyMicrogridSpec 处理规范配置接口
func (ws *webServer) handlePyMicrogridSpec(w http.ResponseWriter, r *http.Request, instanceID string) {
	specPath := filepath.Join(ws.config.ConfigDir, "py_microgrid_instances", instanceID, "spec.json")
	
	switch r.Method {
	case http.MethodGet:
		ws.getPyMicrogridSpec(w, r, instanceID, specPath)
	case http.MethodPut:
		ws.putPyMicrogridSpec(w, r, instanceID, specPath)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// getPyMicrogridSpec 获取规范配置
func (ws *webServer) getPyMicrogridSpec(w http.ResponseWriter, r *http.Request, instanceID, specPath string) {
	spec, etag, err := ws.loadPyMicrogridSpec(specPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "spec not found: "+err.Error())
		return
	}
	
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	writeJSON(w, spec)
}

// putPyMicrogridSpec 更新规范配置
func (ws *webServer) putPyMicrogridSpec(w http.ResponseWriter, r *http.Request, instanceID, specPath string) {
	// 检查 If-Match 头
	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		writeError(w, 428, "If-Match header required")
		return
	}
	
	// 加载当前配置并检查版本
	currentSpec, currentETag, err := ws.loadPyMicrogridSpec(specPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "spec not found: "+err.Error())
		return
	}
	
	expectedETag := strings.Trim(ifMatch, `"`)
	if currentETag != expectedETag {
		writeError(w, http.StatusPreconditionFailed, "configuration version mismatch")
		return
	}
	
	// 解析新配置
	var newSpec model.PyMicrogridSpec
	if err := json.NewDecoder(r.Body).Decode(&newSpec); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	
	// 校验配置
	if err := ws.validatePyMicrogridSpec(&newSpec); err != nil {
		writeError(w, http.StatusBadRequest, "validation failed: "+err.Error())
		return
	}
	
	// 保存配置
	if err := ws.savePyMicrogridSpec(specPath, &newSpec); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save spec: "+err.Error())
		return
	}
	
	// 返回新的 ETag
	newETag, _ := ws.calculateSpecETag(&newSpec)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, newETag))
	writeJSON(w, map[string]string{"status": "updated"})
}

// handlePyMicrogridValidate 验证规范配置
func (ws *webServer) handlePyMicrogridValidate(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	var spec model.PyMicrogridSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	
	if err := ws.validatePyMicrogridSpec(&spec); err != nil {
		writeJSON(w, map[string]interface{}{
			"valid": false,
			"error": err.Error(),
		})
		return
	}
	
	writeJSON(w, map[string]interface{}{
		"valid": true,
	})
}

// handlePyMicrogridRuntime 获取运行时状态
func (ws *webServer) handlePyMicrogridRuntime(w http.ResponseWriter, r *http.Request, instanceID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	// TODO: 实现运行时状态获取
	// 目前返回示例数据
	status := model.RuntimeStatus{
		OverallState: "running",
		Process: model.ProcessStatus{
			PID:            1234,
			StartedAt:      time.Now().Add(-1*time.Hour).Format(time.RFC3339),
			LastTickAt:     time.Now().Format(time.RFC3339),
			RuntimeVersion: "2.0.0",
			ConfigRevision: "sha256:abcd1234",
			LastError:      "",
		},
		Modbus: model.ModbusStatus{
			Connected:  true,
			LastPollAt: time.Now().Format(time.RFC3339),
			LastError:  "",
		},
		Replay: model.ReplayStatus{
			State:               "running",
			LastRequestAt:       time.Now().Add(-30*time.Second).Format(time.RFC3339),
			LastSuccessAt:       time.Now().Add(-30*time.Second).Format(time.RFC3339),
			ConsecutiveFailures: 0,
			TargetLagSec:        60,
			Caches:              []model.CacheStatus{},
		},
	}
	
	writeJSON(w, status)
}

// loadPyMicrogridSpec 加载规范配置
func (ws *webServer) loadPyMicrogridSpec(specPath string) (*model.PyMicrogridSpec, string, error) {
	data, err := ws.readFile(specPath)
	if err != nil {
		return nil, "", err
	}
	
	var spec model.PyMicrogridSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, "", err
	}
	
	etag, _ := ws.calculateSpecETag(&spec)
	return &spec, etag, nil
}

// savePyMicrogridSpec 保存规范配置
func (ws *webServer) savePyMicrogridSpec(specPath string, spec *model.PyMicrogridSpec) error {
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	
	return ws.writeFile(specPath, data)
}

// calculateSpecETag 计算配置的ETag
func (ws *webServer) calculateSpecETag(spec *model.PyMicrogridSpec) (string, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8]), nil
}

// validatePyMicrogridSpec 验证规范配置
func (ws *webServer) validatePyMicrogridSpec(spec *model.PyMicrogridSpec) error {
	// 基本字段验证
	if spec.SchemaVersion != 2 {
		return fmt.Errorf("unsupported schema version: %d", spec.SchemaVersion)
	}
	
	// 设备验证
	deviceKeys := make(map[string]bool)
	slaveIDs := make(map[int]bool)
	
	for i, device := range spec.Devices {
		// 检查设备标识符唯一性
		if deviceKeys[device.DeviceKey] {
			return fmt.Errorf("device[%d]: duplicate device_key: %s", i, device.DeviceKey)
		}
		deviceKeys[device.DeviceKey] = true
		
		// 检查 SlaveID 唯一性
		if slaveIDs[device.SlaveID] {
			return fmt.Errorf("device[%d]: duplicate slave_id: %d", i, device.SlaveID)
		}
		slaveIDs[device.SlaveID] = true
		
		// 验证设备特定配置
		if err := ws.validateDeviceConfig(&device); err != nil {
			return fmt.Errorf("device[%d] (%s): %v", i, device.DeviceKey, err)
		}
	}
	
	// EnOS 回放配置验证
	if spec.Replay.Enabled {
		if spec.Replay.CredentialRef == "" {
			return fmt.Errorf("replay.credential_ref is required when replay is enabled")
		}
		
		// 验证时间约束
		if spec.Replay.DelaySec > spec.Replay.TargetLagSec {
			return fmt.Errorf("replay.delay_sec (%d) must not exceed target_lag_sec (%d)", 
				spec.Replay.DelaySec, spec.Replay.TargetLagSec)
		}
		
		if spec.Replay.TargetLagSec > spec.Replay.DelaySec + spec.Replay.LookbackSec {
			return fmt.Errorf("replay.target_lag_sec (%d) must not exceed delay_sec (%d) + lookback_sec (%d)",
				spec.Replay.TargetLagSec, spec.Replay.DelaySec, spec.Replay.LookbackSec)
		}
		
		minCacheRetention := maxInt(spec.Replay.LookbackSec, spec.Replay.TargetLagSec + 2*spec.Replay.FetchIntervalSec)
		if spec.Replay.CacheRetentionSec < minCacheRetention {
			return fmt.Errorf("replay.cache_retention_sec (%d) must be at least %d", 
				spec.Replay.CacheRetentionSec, minCacheRetention)
		}
		
		if spec.Replay.MaxStalenessSec < spec.Replay.FetchIntervalSec {
			return fmt.Errorf("replay.max_staleness_sec (%d) must not be less than fetch_interval_sec (%d)",
				spec.Replay.MaxStalenessSec, spec.Replay.FetchIntervalSec)
		}
	}
	
	return nil
}

// validateDeviceConfig 验证设备配置
func (ws *webServer) validateDeviceConfig(device *model.DeviceConfig) error {
	switch device.DeviceType {
	case "meter":
		return ws.validateMeterConfig(device)
	case "pv":
		return ws.validatePVConfig(device)
	case "bess":
		return ws.validateBESSConfig(device)
	case "ev":
		return ws.validateEVConfig(device)
	case "load":
		return ws.validateLoadConfig(device)
	case "wind":
		return ws.validateWindConfig(device)
	default:
		return fmt.Errorf("unsupported device type: %s", device.DeviceType)
	}
}

// validateMeterConfig 验证关口表配置
func (ws *webServer) validateMeterConfig(device *model.DeviceConfig) error {
	if device.Source.Kind != "calculated" && device.Source.Kind != "enos" {
		return fmt.Errorf("meter source.kind must be 'calculated' or 'enos'")
	}
	
	if device.Source.Kind == "enos" {
		if device.Source.EnOS == nil {
			return fmt.Errorf("enos source configuration required")
		}
		if device.Source.EnOS.AssetID == "" || device.Source.EnOS.PowerPoint == "" {
			return fmt.Errorf("asset_id and power_point required for enos source")
		}
	}
	
	return nil
}

// validatePVConfig 验证PV配置
func (ws *webServer) validatePVConfig(device *model.DeviceConfig) error {
	if device.Params.RatedPowerKW == nil || *device.Params.RatedPowerKW <= 0 {
		return fmt.Errorf("rated_power_kw must be positive")
	}
	
	return ws.validateCommonDeviceSource(device, []string{"synthetic", "csv", "enos"})
}

// validateBESSConfig 验证BESS配置
func (ws *webServer) validateBESSConfig(device *model.DeviceConfig) error {
	if device.Params.RatedCapacityKWH == nil || *device.Params.RatedCapacityKWH <= 0 {
		return fmt.Errorf("rated_capacity_kwh must be positive")
	}
	if device.Params.MaxChargePowerKW == nil || *device.Params.MaxChargePowerKW < 0 {
		return fmt.Errorf("max_charge_power_kw must be non-negative")
	}
	if device.Params.MaxDischargePowerKW == nil || *device.Params.MaxDischargePowerKW < 0 {
		return fmt.Errorf("max_discharge_power_kw must be non-negative")
	}
	
	if device.Params.SocMinPct != nil && device.Params.SocMaxPct != nil {
		if *device.Params.SocMinPct >= *device.Params.SocMaxPct {
			return fmt.Errorf("soc_min_pct must be less than soc_max_pct")
		}
	}
	
	if device.Params.InitialSocPct != nil && device.Params.SocMinPct != nil && device.Params.SocMaxPct != nil {
		if *device.Params.InitialSocPct < *device.Params.SocMinPct || *device.Params.InitialSocPct > *device.Params.SocMaxPct {
			return fmt.Errorf("initial_soc_pct must be between soc_min_pct and soc_max_pct")
		}
	}
	
	return ws.validateCommonDeviceSource(device, []string{"synthetic", "csv", "enos"})
}

// validateEVConfig 验证EV配置
func (ws *webServer) validateEVConfig(device *model.DeviceConfig) error {
	if device.Params.RatedPowerKW == nil || *device.Params.RatedPowerKW <= 0 {
		return fmt.Errorf("rated_power_kw must be positive")
	}
	
	if device.Params.ChargerType != "" && device.Params.ChargerType != "AC" && device.Params.ChargerType != "DC" {
		return fmt.Errorf("charger_type must be 'AC' or 'DC'")
	}
	
	if device.Params.PhaseMode != nil && (*device.Params.PhaseMode != 1 && *device.Params.PhaseMode != 3) {
		return fmt.Errorf("phase_mode must be 1 or 3")
	}
	
	return ws.validateCommonDeviceSource(device, []string{"synthetic", "csv", "enos"})
}

// validateLoadConfig 验证Load配置
func (ws *webServer) validateLoadConfig(device *model.DeviceConfig) error {
	if device.Params.BasePowerKW == nil || *device.Params.BasePowerKW < 0 {
		return fmt.Errorf("base_power_kw must be non-negative")
	}
	
	return ws.validateCommonDeviceSource(device, []string{"synthetic", "csv", "enos"})
}

// validateWindConfig 验证Wind配置
func (ws *webServer) validateWindConfig(device *model.DeviceConfig) error {
	if device.Params.RatedPowerKW == nil || *device.Params.RatedPowerKW <= 0 {
		return fmt.Errorf("rated_power_kw must be positive")
	}
	
	supportedKinds := []string{"csv", "enos", "enos_aggregate"}
	return ws.validateCommonDeviceSource(device, supportedKinds)
}

// validateCommonDeviceSource 验证设备数据源配置
func (ws *webServer) validateCommonDeviceSource(device *model.DeviceConfig, supportedKinds []string) error {
	// 检查数据源类型是否支持
	found := false
	for _, kind := range supportedKinds {
		if device.Source.Kind == kind {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unsupported source.kind '%s' for device type %s", device.Source.Kind, device.DeviceType)
	}
	
	// 验证具体数据源配置
	switch device.Source.Kind {
	case "csv":
		if device.Source.CSV == nil || device.Source.CSV.FileID == "" {
			return fmt.Errorf("csv source requires file_id")
		}
	case "enos":
		if device.Source.EnOS == nil {
			return fmt.Errorf("enos source configuration required")
		}
		if device.Source.EnOS.AssetID == "" {
			return fmt.Errorf("enos source requires asset_id")
		}
		// 根据设备类型验证必需的测点
		switch device.DeviceType {
		case "pv", "bess", "ev", "load":
			if device.Source.EnOS.PowerPoint == "" {
				return fmt.Errorf("enos source requires power_point for device type %s", device.DeviceType)
			}
		case "wind":
			if device.Source.EnOS.TheoryPowerPoint == "" {
				return fmt.Errorf("enos source requires theory_power_point for wind device")
			}
		}
	case "enos_aggregate":
		if device.DeviceType != "wind" {
			return fmt.Errorf("enos_aggregate source only supported for wind devices")
		}
		if device.Source.EnOS == nil || len(device.Source.EnOS.AggregateAssetIDs) == 0 {
			return fmt.Errorf("enos_aggregate source requires aggregate_asset_ids")
		}
	}
	
	return nil
}

// 辅助函数
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}