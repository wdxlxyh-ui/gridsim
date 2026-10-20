package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// ─── TSDB replay configuration for Python microgrid instances ───────────────
//
// The Python simulator reads two files from its working directory:
//   config/enos_config.json    EnOS API connection
//   config/replay_config.json  replay engine (started only when mode == "replay")
//
// Both live in the per-instance workspace py_instances/<id>/ so every instance
// has fully independent credentials and subscriptions.

const (
	tsdbEnosFile   = "enos_config.json"
	tsdbReplayFile = "replay_config.json"
	tsdbTimeLayout = "2006-01-02 15:04:05"
	tsdbMaxBody    = 1 << 20
)

var tsdbHostPattern = regexp.MustCompile(`^[A-Za-z0-9.\-]+(:[0-9]{1,5})?$`)

type EnosConfig struct {
	ApigwAddress string `json:"apigw_address"`
	AccessKey    string `json:"access_key"`
	SecretKey    string `json:"secret_key"`
	OrgID        string `json:"org_id"`
}

type AssetSubscription struct {
	AssetID  string   `json:"asset_id"`
	PointIDs []string `json:"point_ids"`
}

type ReplayConfig struct {
	Enabled          bool                `json:"enabled"`
	TimeMode         string              `json:"time_mode"` // "relative" | "absolute"
	ReplayStartTime  string              `json:"replay_start_time"`
	FetchIntervalSec int                 `json:"fetch_interval_sec"`
	LookbackSec      int                 `json:"lookback_sec"`
	DelaySec         int                 `json:"delay_sec"`
	TargetLagSec     int                 `json:"target_lag_sec"`
	Assets           []AssetSubscription `json:"assets"`
}

type TsdbConfig struct {
	Enos   EnosConfig   `json:"enos"`
	Replay ReplayConfig `json:"replay"`
	// Read-only status flags, ignored on PUT.
	WorkspaceReady  bool `json:"workspace_ready"`
	EngineAvailable bool `json:"engine_available"`
}

func defaultReplayConfig() ReplayConfig {
	return ReplayConfig{
		Enabled:          false,
		TimeMode:         "relative",
		FetchIntervalSec: 30,
		LookbackSec:      300,
		DelaySec:         0,
		TargetLagSec:     0,
		Assets:           []AssetSubscription{},
	}
}

func readJSONMap(path string) map[string]interface{} {
	m := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &m)
	}
	return m
}

func jsonString(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

func jsonInt(m map[string]interface{}, key string, def int) int {
	if f, ok := m[key].(float64); ok {
		return int(f)
	}
	return def
}

// writeFileAtomic writes via temp file + rename so the Python process never
// observes a half-written file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// tsdbWorkspace returns the per-instance workspace dir, whether it already
// exists, and whether the replay engine code is present in the code base that
// the instance runs (workspace if ready, otherwise the template script_dir).
func (ws *webServer) tsdbWorkspace(id string) (workDir string, ready bool, engine bool, err error) {
	if strings.ContainsAny(id, `/\.`) {
		return "", false, false, fmt.Errorf("invalid instance id")
	}
	scriptDir, _, err := ws.resolveScriptDir(id)
	if err != nil {
		return "", false, false, err
	}
	workDir = filepath.Join(ws.cfgDir, "py_instances", id)
	if info, statErr := os.Stat(workDir); statErr == nil && info.IsDir() {
		ready = true
		scriptDir = workDir
	}
	if _, statErr := os.Stat(filepath.Join(scriptDir, "src", "datasource", "replay_provider.py")); statErr == nil {
		engine = true
	}
	return workDir, ready, engine, nil
}

func (ws *webServer) handlePyMicrogridTsdbConfig(w http.ResponseWriter, r *http.Request, id string) {
	workDir, ready, engine, err := ws.tsdbWorkspace(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	configDir := filepath.Join(workDir, "config")
	enosPath := filepath.Join(configDir, tsdbEnosFile)
	replayPath := filepath.Join(configDir, tsdbReplayFile)

	switch r.Method {
	case http.MethodGet:
		resp := TsdbConfig{Replay: defaultReplayConfig(), WorkspaceReady: ready, EngineAvailable: engine}
		if ready {
			em := readJSONMap(enosPath)
			resp.Enos = EnosConfig{
				ApigwAddress: jsonString(em, "apigw_address"),
				AccessKey:    jsonString(em, "access_key"),
				SecretKey:    jsonString(em, "secret_key"),
				OrgID:        jsonString(em, "org_id"),
			}
			rm := readJSONMap(replayPath)
			if len(rm) > 0 {
				rc := &resp.Replay
				rc.Enabled = jsonString(rm, "mode") == "replay"
				rc.ReplayStartTime = jsonString(rm, "replay_start_time")
				if rc.ReplayStartTime != "" {
					rc.TimeMode = "absolute"
				}
				rc.FetchIntervalSec = jsonInt(rm, "fetch_interval_sec", rc.FetchIntervalSec)
				rc.LookbackSec = jsonInt(rm, "lookback_sec", rc.LookbackSec)
				rc.DelaySec = jsonInt(rm, "delay_sec", rc.DelaySec)
				rc.TargetLagSec = jsonInt(rm, "target_lag_sec", rc.TargetLagSec)
				if assets, ok := rm["assets"].([]interface{}); ok {
					for _, a := range assets {
						am, ok := a.(map[string]interface{})
						if !ok {
							continue
						}
						as := AssetSubscription{AssetID: jsonString(am, "asset_id"), PointIDs: []string{}}
						if pids, ok := am["point_ids"].([]interface{}); ok {
							for _, p := range pids {
								if ps, ok := p.(string); ok {
									as.PointIDs = append(as.PointIDs, ps)
								}
							}
						}
						rc.Assets = append(rc.Assets, as)
					}
				}
			}
		}
		writeJSON(w, http.StatusOK, resp)

	case http.MethodPut:
		if !ready {
			writeError(w, http.StatusConflict, "实例的独立工作区尚未创建，请先启动一次该实例后再配置 TSDB 回放")
			return
		}
		var req TsdbConfig
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, tsdbMaxBody)).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		enos, rc, verr := normalizeTsdbConfig(req)
		if verr != nil {
			writeError(w, http.StatusBadRequest, verr.Error())
			return
		}
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			writeError(w, http.StatusInternalServerError, "create config dir: "+err.Error())
			return
		}

		// Merge into existing files so unknown keys (comments etc.) survive.
		em := readJSONMap(enosPath)
		em["apigw_address"] = enos.ApigwAddress
		em["access_key"] = enos.AccessKey
		em["secret_key"] = enos.SecretKey
		em["org_id"] = enos.OrgID
		enosData, _ := json.MarshalIndent(em, "", "  ")
		if err := writeFileAtomic(enosPath, enosData, 0o600); err != nil {
			writeError(w, http.StatusInternalServerError, "save enos config: "+err.Error())
			return
		}

		rm := readJSONMap(replayPath)
		if rc.Enabled {
			rm["mode"] = "replay" // the only value that starts the engine
		} else {
			rm["mode"] = "disabled"
		}
		rm["enos_config"] = "config/" + tsdbEnosFile
		if rc.TimeMode == "absolute" {
			rm["replay_start_time"] = rc.ReplayStartTime
		} else {
			delete(rm, "replay_start_time")
		}
		rm["fetch_interval_sec"] = rc.FetchIntervalSec
		rm["lookback_sec"] = rc.LookbackSec
		rm["delay_sec"] = rc.DelaySec
		rm["target_lag_sec"] = rc.TargetLagSec
		rm["assets"] = rc.Assets
		delete(rm, "asset_id") // legacy single-asset keys would be ignored anyway
		delete(rm, "point_ids")
		replayData, _ := json.MarshalIndent(rm, "", "  ")
		if err := writeFileAtomic(replayPath, replayData, 0o644); err != nil {
			writeError(w, http.StatusInternalServerError, "save replay config: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// normalizeTsdbConfig trims, de-duplicates and validates a submitted config.
func normalizeTsdbConfig(req TsdbConfig) (EnosConfig, ReplayConfig, error) {
	enos := EnosConfig{
		ApigwAddress: strings.TrimSpace(req.Enos.ApigwAddress),
		AccessKey:    strings.TrimSpace(req.Enos.AccessKey),
		SecretKey:    strings.TrimSpace(req.Enos.SecretKey),
		OrgID:        strings.TrimSpace(req.Enos.OrgID),
	}
	rc := req.Replay
	rc.ReplayStartTime = strings.TrimSpace(rc.ReplayStartTime)

	switch rc.TimeMode {
	case "relative", "":
		rc.TimeMode = "relative"
		rc.ReplayStartTime = ""
	case "absolute":
		if _, err := time.Parse(tsdbTimeLayout, rc.ReplayStartTime); err != nil {
			return enos, rc, fmt.Errorf("绝对时间格式应为 YYYY-MM-DD HH:MM:SS（东八区）")
		}
	default:
		return enos, rc, fmt.Errorf("time_mode 只能是 relative 或 absolute")
	}

	if rc.FetchIntervalSec < 5 || rc.FetchIntervalSec > 3600 {
		return enos, rc, fmt.Errorf("拉取间隔需在 5~3600 秒之间")
	}
	if rc.LookbackSec < 10 || rc.LookbackSec > 86400 {
		return enos, rc, fmt.Errorf("回溯时长需在 10~86400 秒之间")
	}
	if rc.DelaySec < 0 || rc.DelaySec > 86400 || rc.TargetLagSec < 0 || rc.TargetLagSec > 86400 {
		return enos, rc, fmt.Errorf("延迟与目标滞后需在 0~86400 秒之间")
	}

	seen := map[string]int{}
	assets := make([]AssetSubscription, 0, len(rc.Assets))
	for _, a := range rc.Assets {
		aid := strings.TrimSpace(a.AssetID)
		if aid == "" {
			return enos, rc, fmt.Errorf("资产 ID 不能为空")
		}
		idx, ok := seen[aid]
		if !ok {
			assets = append(assets, AssetSubscription{AssetID: aid, PointIDs: []string{}})
			idx = len(assets) - 1
			seen[aid] = idx
		}
		have := map[string]bool{}
		for _, p := range assets[idx].PointIDs {
			have[p] = true
		}
		for _, p := range a.PointIDs {
			p = strings.TrimSpace(p)
			if p != "" && !have[p] {
				assets[idx].PointIDs = append(assets[idx].PointIDs, p)
				have[p] = true
			}
		}
	}
	for _, a := range assets {
		if len(a.PointIDs) == 0 {
			return enos, rc, fmt.Errorf("资产 %s 至少需要一个测点 ID", a.AssetID)
		}
	}
	rc.Assets = assets

	if rc.Enabled {
		if enos.ApigwAddress == "" || enos.AccessKey == "" || enos.SecretKey == "" || enos.OrgID == "" {
			return enos, rc, fmt.Errorf("启用回放前请填写完整的 EnOS 连接信息")
		}
		if len(assets) == 0 {
			return enos, rc, fmt.Errorf("启用回放前请至少订阅一个资产")
		}
	}
	return enos, rc, nil
}

// handlePyMicrogridTsdbTest checks that the EnOS API gateway is reachable.
// Credentials are NOT verified here; the Python engine validates them when the
// replay starts. The message says so explicitly.
func (ws *webServer) handlePyMicrogridTsdbTest(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, _, _, err := ws.tsdbWorkspace(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req EnosConfig
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, tsdbMaxBody)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	host := strings.TrimSpace(req.ApigwAddress)
	host = strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://")
	host = strings.TrimSuffix(host, "/")
	if host == "" || !tsdbHostPattern.MatchString(host) {
		writeError(w, http.StatusBadRequest, "API 网关地址格式不正确，应为域名，例如 ag-cn5.envisioniot.com")
		return
	}
	if !strings.Contains(host, ":") {
		host += ":443"
	}
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": false,
			"message": "无法连接 API 网关 " + host + "：" + err.Error(),
		})
		return
	}
	_ = conn.Close()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "API 网关 " + host + " 可达（仅检测网络连通，AK/SK 在回放启动时由引擎校验）",
	})
}
