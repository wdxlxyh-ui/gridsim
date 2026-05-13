package detail

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"iec104-sim/internal/model"
	"iec104-sim/pkg/config"
	"iec104-sim/pkg/library"
)

type DetailHandler struct {
	store  *library.Store
	engine *Engine
	cfgDir string
	instID string
}

func NewDetailHandler(instID string, store *library.Store, engine *Engine, cfgDir string) *DetailHandler {
	return &DetailHandler{
		store:  store,
		engine: engine,
		cfgDir: cfgDir,
		instID: instID,
	}
}

func (h *DetailHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/instances/"+h.instID+"/points", h.handlePoints)
	mux.HandleFunc("/api/v1/instances/"+h.instID+"/points/", h.handlePointsByIOA)
	mux.HandleFunc("/api/v1/instances/"+h.instID+"/upload-csv", h.handleUploadCSV)
}

func (h *DetailHandler) handlePoints(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSnapshots(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *DetailHandler) handlePointsByIOA(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/instances/"+h.instID+"/points/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing IOA"})
		return
	}

	// Handle sub-routes
	if parts[0] == "export" {
		h.exportCSV(w, r)
		return
	}
	if parts[0] == "auto-change" {
		h.handleAutoChangeConfig(w, r, parts)
		return
	}

	ioa, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid IOA: " + parts[0]})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getSnapshot(w, uint32(ioa))
	case http.MethodPut:
		h.setValue(w, r, uint32(ioa))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *DetailHandler) handleAutoChangeConfig(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) >= 2 && parts[1] == "batch" {
		h.batchAutoChange(w, r)
		return
	}
	if len(parts) >= 2 && parts[1] == "export" {
		h.exportAutoConfig(w, r)
		return
	}
	if len(parts) >= 2 && parts[1] == "import" {
		h.importAutoConfig(w, r)
		return
	}

	if len(parts) < 2 || parts[1] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing IOA for auto-change"})
		return
	}

	ioa, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid IOA"})
		return
	}

	p, ok := h.store.Get(uint32(ioa))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "point not found"})
		return
	}
	if IsAODO(p.PointType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "AO/DO does not support auto-change"})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getAutoChange(w, uint32(ioa))
	case http.MethodPut:
		h.putAutoChange(w, r, uint32(ioa))
	case http.MethodDelete:
		h.deleteAutoChange(w, uint32(ioa))
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *DetailHandler) listSnapshots(w http.ResponseWriter, r *http.Request) {
	points := h.store.GetAll()
	snapshots := make([]model.PointSnapshot, 0, len(points))
	for _, p := range points {
		snapshots = append(snapshots, pointToSnapshot(p))
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"points":       snapshots,
		"refreshed_at": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
	})
}

func (h *DetailHandler) getSnapshot(w http.ResponseWriter, ioa uint32) {
	p, ok := h.store.Get(ioa)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "point not found"})
		return
	}
	writeJSON(w, http.StatusOK, pointToSnapshot(p))
}

func (h *DetailHandler) setValue(w http.ResponseWriter, r *http.Request, ioa uint32) {
	p, ok := h.store.Get(ioa)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "point not found"})
		return
	}

	// Check API update policy
	if !h.engine.CheckAPIWriteAllowed(ioa) {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "该点已配置自动变化策略，不允许通过接口写入",
		})
		return
	}

	if !IsSetValueAllowed(p.PointType) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "AO/DO does not support set-value"})
		return
	}

	var body struct {
		Value     *float64 `json:"value"`
		BoolValue *bool    `json:"bool_value"`
		IntValue  *int32   `json:"int_value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	var changed bool
	switch p.PointType {
	case config.TypeAI:
		if body.Value != nil {
			if _, err := h.store.SetValue(ioa, *body.Value); err == nil {
				changed = true
			}
		}
	case config.TypeDI:
		if body.BoolValue != nil {
			if _, err := h.store.SetBoolValue(ioa, *body.BoolValue); err == nil {
				changed = true
			}
		} else if body.Value != nil {
			bv := int64(*body.Value) != 0
			if _, err := h.store.SetBoolValue(ioa, bv); err == nil {
				changed = true
			}
		}
	case config.TypePI:
		if body.IntValue != nil {
			if _, err := h.store.SetIntValue(ioa, *body.IntValue); err == nil {
				changed = true
			}
		} else if body.Value != nil {
			if _, err := h.store.SetIntValue(ioa, int32(*body.Value)); err == nil {
				changed = true
			}
		}
	}

	if changed {
		h.engine.HandleAOFollow(ioa)
		pub, ok := h.store.Get(ioa)
		if ok {
			// We need the publisher from the engine. Since Handler owns engine reference,
			// we call publish via the same publisher interface stored in engine.
			h.engine.pub.Publish(pub)
		}
		slog.Info("置数成功", "ioa", ioa, "instance", h.instID)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"ioa":     ioa,
		"changed": changed,
	})
}

func (h *DetailHandler) getAutoChange(w http.ResponseWriter, ioa uint32) {
	cfg, ok := h.engine.GetConfig(ioa)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no auto-change config"})
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (h *DetailHandler) putAutoChange(w http.ResponseWriter, r *http.Request, ioa uint32) {
	var req struct {
		Strategy string           `json:"strategy"`
		Enabled  bool             `json:"enabled"`
		Params   model.StrategyParams `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Params.PeriodMs > 0 && req.Params.PeriodMs < 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period_ms must be >= 100"})
		return
	}

	cfg := &model.AutoChangeConfig{
		PointIOA:  ioa,
		Strategy:  model.StrategyType(req.Strategy),
		Enabled:   req.Enabled,
		Params:    req.Params,
		UpdatedAt: time.Now(),
	}

	if err := h.engine.StartOrUpdate(cfg); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true, "ioa": ioa})
}

func (h *DetailHandler) deleteAutoChange(w http.ResponseWriter, ioa uint32) {
	if err := h.engine.Remove(ioa); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"success": true})
}

func (h *DetailHandler) batchAutoChange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		IOAs   []uint32            `json:"ioas"`
		Config json.RawMessage     `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	var autoCfg struct {
		Strategy string           `json:"strategy"`
		Enabled  bool             `json:"enabled"`
		Params   model.StrategyParams `json:"params"`
	}
	if err := json.Unmarshal(req.Config, &autoCfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid config: " + err.Error()})
		return
	}

	if autoCfg.Params.PeriodMs > 0 && autoCfg.Params.PeriodMs < 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "period_ms must be >= 100"})
		return
	}

	success := 0
	failed := 0
	var errors []string

	for _, ioa := range req.IOAs {
		p, ok := h.store.Get(ioa)
		if !ok {
			failed++
			errors = append(errors, fmt.Sprintf("IOA %d: point not found", ioa))
			continue
		}
		if IsAODO(p.PointType) {
			failed++
			errors = append(errors, fmt.Sprintf("IOA %d: AO/DO not supported", ioa))
			continue
		}

		cfg := &model.AutoChangeConfig{
			PointIOA: ioa,
			Strategy: model.StrategyType(autoCfg.Strategy),
			Enabled:  autoCfg.Enabled,
			Params:   autoCfg.Params,
			UpdatedAt: time.Now(),
		}
		if err := h.engine.StartOrUpdate(cfg); err != nil {
			failed++
			errors = append(errors, fmt.Sprintf("IOA %d: %s", ioa, err.Error()))
			continue
		}
		success++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"total":   len(req.IOAs),
		"ok":      success,
		"failed":  failed,
		"errors":  errors,
	})
}

func (h *DetailHandler) exportAutoConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	configs := h.engine.AllConfigs()
	serializable := make(map[string]interface{})
	for ioa, cfg := range configs {
		serializable[strconv.FormatUint(uint64(ioa), 10)] = cfg
	}

	result := map[string]interface{}{
		"autoChanges": serializable,
		"exportTime":  time.Now().UTC().Format(time.RFC3339Nano),
		"instanceId":  h.instID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="auto_changes_%s.json"`, h.instID))
	json.NewEncoder(w).Encode(result)
}

func (h *DetailHandler) importAutoConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req struct {
		AutoChanges map[string]json.RawMessage `json:"autoChanges"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	configs := make(map[uint32]*model.AutoChangeConfig)
	for ioaStr, raw := range req.AutoChanges {
		ioa, err := strconv.ParseUint(ioaStr, 10, 32)
		if err != nil {
			continue
		}
		var cfg model.AutoChangeConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			continue
		}
		cfg.PointIOA = uint32(ioa)
		configs[uint32(ioa)] = &cfg
	}

	if err := h.engine.SaveAll(configs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"count":   len(configs),
	})
}

func (h *DetailHandler) exportCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	points := h.store.GetAll()

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="points_%s.csv"`, h.instID))

	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write([]string{"信息体地址", "测点名称", "测点类型", "实时值", "测点值更新时间"})
	for _, p := range points {
		val := formatPointValueStr(p)
		ts := p.Timestamp.Format("2006-01-02 15:04:05.000")
		writer.Write([]string{
			strconv.FormatUint(uint64(p.IOA), 10),
			p.Name,
			string(p.PointType),
			val,
			ts,
		})
	}
}

func (h *DetailHandler) handleUploadCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	r.ParseMultipartForm(32 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no file provided"})
		return
	}
	defer file.Close()

	csvDir := filepath.Join(h.cfgDir, "csv", h.instID)
	os.MkdirAll(csvDir, 0755)

	filename := filepath.Base(header.Filename)
	dst := filepath.Join(csvDir, filename)
	dstFile, err := os.Create(dst)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save file"})
		return
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to write file"})
		return
	}

	slog.Info("CSV上传成功", "instance", h.instID, "filename", filename)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"filename": filename,
		"path":     "csv/" + h.instID + "/" + filename,
	})
}

func pointToSnapshot(p *config.Point) model.PointSnapshot {
	return model.PointSnapshot{
		IOA:       p.IOA,
		Name:      p.Name,
		PointType: string(p.PointType),
		Value:     p.Value,
		BoolValue: p.BoolValue,
		IntValue:  p.IntValue,
		UpdatedAt: p.Timestamp,
		Unit:      "",
	}
}

func formatPointValueStr(p *config.Point) string {
	switch p.PointType {
	case config.TypeAI, config.TypeAO:
		return strconv.FormatFloat(p.Value, 'f', 2, 64)
	case config.TypeDI, config.TypeDO:
		if p.BoolValue {
			return "1"
		}
		return "0"
	case config.TypePI:
		return strconv.Itoa(int(p.IntValue))
	default:
		return ""
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Public wrappers for external routing (called from main.go management server)
func (h *DetailHandler) HandlePoints(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSnapshots(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *DetailHandler) HandleGetSnapshot(w http.ResponseWriter, ioa uint32) {
	h.getSnapshot(w, ioa)
}

func (h *DetailHandler) HandleSetValue(w http.ResponseWriter, r *http.Request, ioa uint32) {
	h.setValue(w, r, ioa)
}

func (h *DetailHandler) HandleExportCSV(w http.ResponseWriter, r *http.Request) {
	h.exportCSV(w, r)
}

func (h *DetailHandler) HandleUploadCSV(w http.ResponseWriter, r *http.Request) {
	h.handleUploadCSV(w, r)
}

func (h *DetailHandler) HandleAutoChangeConfig(w http.ResponseWriter, r *http.Request, parts []string) {
	h.handleAutoChangeConfig(w, r, parts)
}
