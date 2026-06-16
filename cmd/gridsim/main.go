package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"strconv"

	"gridsim/internal/detail"
	"gridsim/internal/manager"
	"gridsim/internal/microgrid"
	"gridsim/internal/model"
	"gridsim/internal/storage"
	"gridsim/pkg/api"
	"gridsim/pkg/config"
	apierrors "gridsim/pkg/errors"
	"gridsim/pkg/events"
	"gridsim/pkg/firewall"
	"gridsim/pkg/iec104"
	"gridsim/pkg/library"
	"gridsim/pkg/middleware"
	"gridsim/pkg/openapi"
	"gridsim/pkg/protocol"
	"gridsim/pkg/recording"

	"github.com/spf13/pflag"
	"golang.org/x/crypto/bcrypt"
)

//go:embed resources/*.json
var builtinResources embed.FS

var (
	version    = "dev"
	gitCommit  = "unknown"
	gitBranch  = "unknown"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		runServerMode()
	} else {
		runLegacyMode()
	}
}

// ─── Legacy Mode (backward compatible) ─────────────────────────────────────

func runLegacyMode() {
	var (
		port     int
		cfgPath  string
		httpAddr string
		logLvl   string
	)

	pflag.IntVarP(&port, "port", "p", 2404, "IEC104 TCP 端口号")
	pflag.StringVarP(&cfgPath, "config", "c", "", "配置文件路径 (.xlsx)")
	pflag.StringVarP(&httpAddr, "http", "H", ":8989", "HTTP API 监听地址")
	pflag.StringVarP(&logLvl, "log", "l", "info", "日志级别: debug/info/warn/error")
	pflag.Parse()

	if cfgPath == "" {
		slog.Error("必须指定配置文件路径 (-c)")
		os.Exit(1)
	}

	setupLogLevel(logLvl)
	slog.Info("启动模拟器 (传统模式)", "port", port, "config", cfgPath, "http", httpAddr)

	points, err := config.LoadFromXLSX(cfgPath, "")
	if err != nil {
		slog.Error("加载配置文件失败", "error", err)
		os.Exit(1)
	}

	counts := countByType(points)
	slog.Info("点表加载完成",
		"totalPoints", len(points),
		"AI", counts["AI"], "DI", counts["DI"],
		"PI", counts["PI"], "DO", counts["DO"], "AO", counts["AO"],
	)

	store := library.NewStore(points)
	server := iec104.NewServer(port, store)
	if err := server.Start(); err != nil {
		slog.Error("启动IEC104服务端失败", "error", err)
		os.Exit(1)
	}

	apiHandler := api.NewHandler(store, server, server)
	mux := http.NewServeMux()
	apiHandler.Register(mux)

	if p := parsePort(httpAddr); p > 0 {
		firewall.EnsurePort(p, "iec104-sim-http")
	}
	firewall.EnsurePort(port, "iec104-sim-data")

	httpSrv := newHTTPServer(httpAddr, mux)
	go func() {
		slog.Info("HTTP API 已启动", "addr", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务失败", "error", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("收到信号，正在关闭", "signal", sig)

	server.Stop()
	httpSrv.Close()
	slog.Info("模拟器已关闭")
}

// ─── Server Mode (web management) ──────────────────────────────────────────

type webServer struct {
	mgr          *manager.Manager
	httpSrv      *http.Server
	cfgDir       string
	userConfig   *model.UserConfig
	proxyStore   *api.ProxyStore
	proxyHandler *api.ProxyHandler
	eventBus     *events.Bus
	recorder     *recording.Recorder
}

func runServerMode() {
	var (
		httpAddr  string
		configDir string
		logDir    string
		logLvl    string
	)

	pflag.StringVarP(&httpAddr, "http", "H", ":8989", "管理API监听地址")
	pflag.StringVarP(&configDir, "config-dir", "c", "./config", "配置文件目录")
	pflag.StringVarP(&logDir, "log-dir", "L", "./logs", "日志文件目录")
	pflag.StringVarP(&logLvl, "log", "l", "info", "日志级别: debug/info/warn/error")
	pflag.Parse()

	setupLogLevel(logLvl)

	os.MkdirAll(configDir, 0755)
	os.MkdirAll(logDir, 0755)

	// Init storage & manager
	cfgStore := storage.NewConfigStore(filepath.Join(configDir, "instances.json"))
	if err := cfgStore.Load(); err != nil {
		slog.Warn("加载实例配置失败，使用空配置", "error", err)
	}

	mgr := manager.New(cfgStore, configDir)

	proxyStore := api.NewProxyStore(configDir)
	if err := proxyStore.Load(); err != nil {
		slog.Warn("加载代理配置失败", "error", err)
	}
	if err := seedBuiltinProxyData(proxyStore); err != nil {
		slog.Warn("写入内置 API 集合失败", "error", err)
	}

	// Build HTTP mux
	mux := http.NewServeMux()
	userCfg := loadUserConfig(configDir)
	ws := &webServer{mgr: mgr, cfgDir: configDir, userConfig: userCfg, proxyStore: proxyStore, eventBus: events.NewBus(), recorder: recording.NewRecorder(filepath.Join(configDir, "recordings"))}
	ws.registerRoutes(mux, configDir, httpAddr)

	if p := parsePort(httpAddr); p > 0 {
		firewall.EnsurePort(p, "iec104-sim-http")
	}

	// Start HTTP server
	recordingHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ws.recorder.IsActive() && (r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete) {
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()
			r.Body = io.NopCloser(bytes.NewReader(body))
			ws.recorder.Record(r.Method, r.URL.Path, body)
		}
		mux.ServeHTTP(w, r)
	})
	httpSrv := newHTTPServer(httpAddr, middleware.IdempotencyMiddleware(recordingHandler))
	go func() {
		slog.Info("管理服务已启动", "http", httpAddr, "configDir", configDir)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("管理服务启动失败", "error", err)
		}
	}()

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("收到信号，正在关闭所有实例", "signal", sig)

	mgr.StopAll()
	httpSrv.Close()
	slog.Info("管理服务已关闭")
}

func (ws *webServer) registerRoutes(mux *http.ServeMux, configDir string, httpAddr string) {
	// Resolve web/dist relative to executable path.
	exePath, _ := os.Executable()
	webDir := filepath.Join(filepath.Dir(exePath), "..", "web", "dist")

	// 公开路由（无需认证）
	mux.HandleFunc("/api/v1/auth/login", ws.handleAuthLogin)
	mux.HandleFunc("/api/v1/instances", ws.handleInstances)
	mux.HandleFunc("/api/v1/instances/", ws.handleInstanceByID)
	mux.HandleFunc("/api/v1/status", ws.handleStatus)
	mux.HandleFunc("/api/v1/state", ws.handleState)
	mux.HandleFunc("/api/v1/upload", ws.handleUpload)
	mux.HandleFunc("/api/v1/files", ws.handleFiles)
	mux.HandleFunc("/api/v1/protocols", ws.handleProtocols)

	// Proxy API Tester routes
	ws.proxyHandler = api.NewProxyHandler()
	ws.proxyHandler.Register(mux)
	mux.HandleFunc("/api/v1/proxy/collections", ws.handleCollections)
	mux.HandleFunc("/api/v1/proxy/collections/", ws.handleCollectionByID)
	mux.HandleFunc("/api/v1/proxy/environments", ws.handleEnvironments)
	mux.HandleFunc("/api/v1/proxy/environments/", ws.handleEnvironmentByID)
	mux.HandleFunc("/api/v1/proxy/export", ws.handleExport)

	// Microgrid management routes
	ws.registerMicrogridRoutes(mux)

	mux.Handle("/openapi.json", openapi.New(strings.TrimPrefix(httpAddr, ":")))
	mux.HandleFunc("/api/v1/events", func(w http.ResponseWriter, r *http.Request) {
		ws.eventBus.ServeSSE(w, r)
	})
	mux.HandleFunc("/api/v1/recordings", ws.handleRecordings)
	mux.HandleFunc("/api/v1/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/openapi.json", http.StatusMovedPermanently)
	})

	// Serve static frontend if built
	if _, err := os.Stat(webDir); err == nil {
		mux.Handle("/", http.FileServer(http.Dir(webDir)))
	} else {
		slog.Warn("前端构建目录不存在，Web UI 不可用", "path", webDir)
	}
}

func (ws *webServer) registerMicrogridRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/microgrid/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/topology"):
			microgrid.HandleMicrogridTopology(ws.mgr)(w, r)
		case strings.Contains(path, "/control"):
			microgrid.HandleMicrogridControl(ws.mgr)(w, r)
		case strings.Contains(path, "/device"):
			microgrid.HandleMicrogridDevice(ws.mgr)(w, r)
		case strings.Contains(path, "/dashboard"):
			microgrid.HandleMicrogridDashboard(ws.mgr)(w, r)
		case strings.Contains(path, "/formulas"):
			microgrid.HandleMicrogridFormulas(ws.mgr)(w, r)
		case strings.Contains(path, "/export-xlsx"):
			microgrid.HandleMicrogridExportXLSX(ws.mgr)(w, r)
		case strings.Contains(path, "/points"):
			microgrid.HandleMicrogridPoints(ws.mgr)(w, r)
		default:
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown microgrid endpoint"})
		}
	})
}

// ─── Management API Handlers ───────────────────────────────────────────────

func (ws *webServer) handleInstances(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		states := ws.mgr.ListStates()
		result := make([]map[string]interface{}, len(states))
		for i, s := range states {
			result[i] = instanceStateToMap(s)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"instances": result})

	case http.MethodPost:
		var req model.InstanceConfig
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		if err := validateConfig(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		created, err := ws.mgr.CreateConfig(req)
		if err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, created)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handleInstanceByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/instances/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "missing instance ID")
		return
	}

	id := parts[0]

	if len(parts) >= 2 {
		switch parts[1] {
		case "start":
			ws.execAction(w, id, ws.mgr.StartInstance)
		case "stop":
			ws.execAction(w, id, ws.mgr.StopInstance)
		case "restart":
			ws.execAction(w, id, ws.mgr.RestartInstance)
		case "points":
			ws.handleInstancePoints(w, r, id)
		case "upload-csv":
			ws.withDetailHandler(w, r, id, func(dh *detail.DetailHandler) { dh.HandleUploadCSV(w, r) })
		case "csv-files":
			ws.withDetailHandler(w, r, id, func(dh *detail.DetailHandler) { dh.HandleListCSVFiles(w, r) })
		case "csv-content":
			ws.withDetailHandler(w, r, id, func(dh *detail.DetailHandler) { dh.HandleReadCSVHeaders(w, r) })
		case "csv-replay":
			ws.withDetailHandler(w, r, id, func(dh *detail.DetailHandler) { dh.HandleConfigCSVReplay(w, r) })
		case "batch-replay":
			ws.withDetailHandler(w, r, id, func(dh *detail.DetailHandler) { dh.HandleBatchReplay(w, r) })
		case "metrics":
			ws.withDetailHandler(w, r, id, func(dh *detail.DetailHandler) { dh.HandleMetrics(w, r) })
		default:
			writeError(w, http.StatusBadRequest, "unknown action: "+parts[1])
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		state, err := ws.mgr.GetState(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, instanceStateToMap(state))

	case http.MethodPut:
		var req model.InstanceConfig
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}
		req.ID = id
		if existing, ok := ws.mgr.GetConfig(id); ok {
			if req.Name == "" {
				req.Name = existing.Name
			}
			if req.IEC104Port == 0 {
				req.IEC104Port = existing.IEC104Port
			}
			if req.XLSXFile == "" {
				req.XLSXFile = existing.XLSXFile
			}
			if req.Protocol == "" {
				req.Protocol = existing.Protocol
			}
		if !req.HttpEnabled && req.HttpPort == 0 {
			req.HttpPort = existing.HttpPort
		}
		if req.Protocol == "microgrid" && req.MicrogridConfig == nil {
			req.MicrogridConfig = existing.MicrogridConfig
		}
		}
		if err := ws.mgr.UpdateConfig(req); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, req)

	case http.MethodDelete:
		if err := ws.mgr.DeleteConfig(id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type actionFunc func(string) error

func (ws *webServer) execAction(w http.ResponseWriter, id string, fn actionFunc) {
	if err := fn(id); err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "not running") {
			writeError(w, http.StatusNotFound, errMsg)
		} else {
			writeError(w, http.StatusConflict, errMsg)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id})
}

func (ws *webServer) withDetailHandler(w http.ResponseWriter, r *http.Request, id string, fn func(*detail.DetailHandler)) {
	store := ws.mgr.GetStore(id)
	engine := ws.mgr.GetEngine(id)
	if store == nil || engine == nil {
		writeError(w, http.StatusNotFound, "instance not running")
		return
	}
	fn(detail.NewDetailHandler(id, store, engine, ws.mgr.CfgDir()))
}

func (ws *webServer) handleInstancePoints(w http.ResponseWriter, r *http.Request, id string) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("panic recovered in handleInstancePoints", "instance", id, "recover", rec)
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
	}()

	store := ws.mgr.GetStore(id)
	engine := ws.mgr.GetEngine(id)
	if store == nil || engine == nil {
		writeError(w, http.StatusNotFound, "instance not running")
		return
	}

	detailHandler := detail.NewDetailHandler(id, store, engine, ws.mgr.CfgDir())
	suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/instances/"+id+"/points")
	if suffix == "" || suffix == "/" {
		detailHandler.HandlePoints(w, r)
		return
	}
	suffix = strings.TrimPrefix(suffix, "/")
	parts := strings.Split(suffix, "/")

	switch {
	case parts[0] == "export" && len(parts) == 1:
		detailHandler.HandleExportCSV(w, r)
	case parts[0] == "auto-change":
		detailHandler.HandleAutoChangeConfig(w, r, parts)
	case parts[0] == "batch" && r.Method == http.MethodPost:
		detailHandler.HandleBatchSetValue(w, r)
	case parts[0] == "batch" && r.Method == http.MethodGet:
		detailHandler.HandleBatchRead(w, r)
	default:
		ioa, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid IOA: "+parts[0])
			return
		}
		switch r.Method {
		case http.MethodGet:
			detailHandler.HandleGetSnapshot(w, uint32(ioa))
		case http.MethodPut:
			detailHandler.HandleSetValue(w, r, uint32(ioa))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}
}

func (ws *webServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	states := ws.mgr.ListStates()
	running := 0
	stopped := 0
	for _, s := range states {
		if s.Status == model.StatusRunning {
			running++
		} else {
			stopped++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":    version,
		"git_commit": gitCommit,
		"git_branch": gitBranch,
		"mode":       "serve",
		"configured": len(states),
		"running":    running,
		"stopped":    stopped,
		"max":        manager.MaxInstances,
	})
}

func (ws *webServer) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	states := ws.mgr.ListStates()
	configs := ws.mgr.ListConfigs()

	configMap := make(map[string]model.InstanceConfig, len(configs))
	for _, c := range configs {
		configMap[c.ID] = c
	}

	running, stopped := 0, 0
	items := make([]map[string]interface{}, 0, len(states))

	for _, s := range states {
		if s.Status == model.StatusRunning {
			running++
		} else {
			stopped++
		}

		cfg := s.Config
		if c, ok := configMap[s.Config.ID]; ok {
			cfg = c
		}

		port := cfg.IEC104Port
		if cfg.Protocol == "modbus" && cfg.ModbusConfig != nil {
			port = cfg.ModbusConfig.Port
		}

		item := map[string]interface{}{
			"id":               cfg.ID,
			"name":             cfg.Name,
			"port":             port,
			"protocol":         cfg.Protocol,
			"status":           s.Status,
			"point_count":      s.TotalPoints,
			"client_connected": s.ClientConnected,
			"uptime_seconds":   s.UptimeSeconds,
		}
		if s.Error != "" {
			item["error"] = s.Error
		}
		items = append(items, item)
	}

	resp := map[string]interface{}{
		"version": version,
		"summary": map[string]interface{}{
			"configured": len(states),
			"running":    running,
			"stopped":    stopped,
			"max":        manager.MaxInstances,
		},
		"instances": items,
	}

	writeJSON(w, http.StatusOK, resp)
}

func (ws *webServer) handleRecordings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		names, err := recording.ListRecordings(filepath.Join(ws.cfgDir, "recordings"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"recordings":    names,
			"is_recording":  ws.recorder.IsActive(),
			"recordings_dir": filepath.Join(ws.cfgDir, "recordings"),
		})

	case http.MethodPost:
		var req struct {
			Action string `json:"action"`
			Name   string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		switch req.Action {
		case "start":
			if req.Name == "" {
				req.Name = "recording-" + time.Now().Format("20060102-150405")
			}
			if ws.recorder.Start(req.Name) {
				writeJSON(w, http.StatusOK, map[string]interface{}{"started": true, "name": req.Name})
			} else {
				writeError(w, http.StatusConflict, "recording already in progress")
			}
		case "stop":
			rec, err := ws.recorder.Stop()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			if rec == nil {
				writeError(w, http.StatusBadRequest, "no recording in progress")
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"stopped": true, "name": rec.Name, "operations": len(rec.Operations)})
		default:
			writeError(w, http.StatusBadRequest, "unknown action, use 'start' or 'stop'")
		}

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

var (
	uploadMaxFileSize     int64 = 10 * 1024 * 1024
	uploadAllowedExts          = map[string]bool{".xlsx": true, ".xls": true, ".csv": true}
	uploadAllowedMIME         = []string{
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"application/vnd.ms-excel",
		"text/csv",
	}
	uploadBlockExts = []string{".exe", ".sh", ".php", ".js", ".html", ".sql", ".bat", ".cmd", ".pkl", ".so", ".dll"}
)

func (ws *webServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(uploadMaxFileSize); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or malformed")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no file provided")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !uploadAllowedExts[ext] {
		writeError(w, http.StatusBadRequest, "file type not allowed: "+ext)
		return
	}

	contentType := header.Header.Get("Content-Type")
	allowedMIME := false
	for _, m := range uploadAllowedMIME {
		if contentType == m {
			allowedMIME = true
			break
		}
	}
	if !allowedMIME {
		writeError(w, http.StatusBadRequest, "invalid file content type")
		return
	}

	baseName := strings.ToLower(header.Filename)
	for _, blocked := range uploadBlockExts {
		if strings.HasSuffix(baseName, blocked) {
			writeError(w, http.StatusBadRequest, "extension not allowed")
			return
		}
	}

	safeName := sanitizeFilename(header.Filename)
	dst := filepath.Join(ws.cfgDir, safeName)

	overwrite := r.URL.Query().Get("overwrite") == "true"

	if _, err := os.Stat(dst); err == nil {
		if !overwrite {
			writeError(w, http.StatusConflict, "file already exists")
			return
		}
		os.Remove(dst)
		slog.Info("覆盖已有文件", "filename", safeName, "uploader", r.RemoteAddr)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create file")
		return
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, file); err != nil {
		os.Remove(dst)
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	slog.Info("文件上传成功", "filename", safeName, "size", header.Size, "uploader", r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]string{"status": "uploaded", "filename": safeName})
}

func (ws *webServer) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	entries, err := os.ReadDir(ws.cfgDir)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"files": []interface{}{}})
		return
	}
	files := make([]map[string]interface{}, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".xlsx") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, map[string]interface{}{
			"name":    name,
			"size":    info.Size(),
			"modtime": info.ModTime().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"files": files})
}

func (ws *webServer) handleProtocols(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"protocols": protocol.SupportedProtocols()})
}

// ─── Proxy API Tester Handlers ─────────────────────────────────────────────

func (ws *webServer) handleCollections(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{"collections": ws.proxyStore.GetCollections()})
	case http.MethodPost:
		var item api.CollectionItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := ws.proxyStore.SaveCollection(&item); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, item)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handleCollectionByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy/collections/")
	switch r.Method {
	case http.MethodDelete:
		if err := ws.proxyStore.DeleteCollection(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handleEnvironments(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"environments": ws.proxyStore.GetEnvironments(),
			"active_id":    ws.proxyStore.ActiveEnvID,
		})
	case http.MethodPost:
		var env api.Environment
		if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if err := ws.proxyStore.SaveEnvironment(&env); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, env)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (ws *webServer) handleEnvironmentByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/proxy/environments/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]

	if len(parts) == 2 && parts[1] == "activate" {
		if r.Method == http.MethodPost {
			if err := ws.proxyStore.SetActiveEnv(id); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
	}

	switch r.Method {
	case http.MethodDelete:
		if err := ws.proxyStore.DeleteEnvironment(id); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func instanceStateToMap(s *model.InstanceState) map[string]interface{} {
	proto := s.Config.Protocol
	if proto == "" {
		proto = "iec104"
	}
	m := map[string]interface{}{
		"id":           s.Config.ID,
		"name":         s.Config.Name,
		"iec104_port":  s.Config.IEC104Port,
		"xlsx_file":    s.Config.XLSXFile,
		"enabled":      s.Config.Enabled,
		"http_enabled": s.Config.HttpEnabled,
		"http_port":    s.Config.HttpPort,
		"protocol":        proto,
		"status":          string(s.Status),
	}
	if s.Config.MicrogridConfig != nil {
		m["microgrid_config"] = s.Config.MicrogridConfig
	}
	if s.Status == model.StatusRunning {
		m["stats"] = map[string]interface{}{
			"uptime_seconds":   s.UptimeSeconds,
			"total_points":     s.TotalPoints,
			"client_connected": s.ClientConnected,
			"interrogations":   s.Interrogations,
			"controls":         s.Controls,
			"spontaneous":      s.Spontaneous,
		}
	}
	if s.Error != "" {
		m["error"] = s.Error
	}
	return m
}

func validateConfig(cfg model.InstanceConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("name is required")
	}
	proto := cfg.Protocol
	if proto == "" {
		proto = "iec104"
	}
	port := cfg.IEC104Port
	if proto == "modbus_tcp" && cfg.ModbusConfig != nil && cfg.ModbusConfig.Port > 0 {
		port = cfg.ModbusConfig.Port
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	if cfg.Protocol != "microgrid" && cfg.XLSXFile == "" {
		return fmt.Errorf("xlsx_file is required")
	}
	if cfg.HttpEnabled && (cfg.HttpPort < 1 || cfg.HttpPort > 65535) {
		return fmt.Errorf("http_port must be 1-65535 when http is enabled")
	}
	return nil
}

func loadUserConfig(configDir string) *model.UserConfig {
	userConfigPath := filepath.Join(configDir, "users.json")
	data, err := os.ReadFile(userConfigPath)
	if err != nil {
		slog.Warn("加载用户配置失败，使用空配置", "error", err)
		return &model.UserConfig{}
	}
	var cfg model.UserConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		slog.Warn("解析用户配置失败，使用空配置", "error", err)
		return &model.UserConfig{}
	}
	return &cfg
}

func (ws *webServer) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if ws.userConfig == nil {
		writeError(w, http.StatusInternalServerError, "user config not loaded")
		return
	}

	var matchedUser *model.User
	for i := range ws.userConfig.Users {
		if ws.userConfig.Users[i].Username == req.Username {
			matchedUser = &ws.userConfig.Users[i]
			break
		}
	}

	if matchedUser == nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(matchedUser.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := middleware.GenerateToken(matchedUser)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user": map[string]string{
			"id":       matchedUser.ID,
			"username": matchedUser.Username,
			"role":     matchedUser.Role,
		},
	})
}

func (ws *webServer) saveUploadedFile(file multipart.File, header *multipart.FileHeader) string {
	filename := filepath.Base(header.Filename)
	dst := filepath.Join(ws.cfgDir, filename)
	dstFile, err := os.Create(dst)
	if err != nil {
		return ""
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, file); err != nil {
		return ""
	}
	slog.Info("文件上传成功", "filename", filename, "path", dst)
	return filename
}

func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
}

func setupLogLevel(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "info":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})
	slog.SetDefault(slog.New(h))
}

func parsePort(addr string) int {
	_, s, ok := strings.Cut(addr, ":")
	if !ok {
		return 0
	}
	p, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return p
}

func countByType(points []*config.Point) map[string]int {
	counts := map[string]int{"AI": 0, "DI": 0, "PI": 0, "DO": 0, "AO": 0}
	for _, p := range points {
		counts[string(p.PointType)]++
	}
	return counts
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if m, ok := data.(map[string]string); ok {
		if msg, exists := m["error"]; exists {
			json.NewEncoder(w).Encode(apierrors.Response{
				Error: apierrors.APIError{
					Code:    apierrors.CodeFromMessage(msg),
					Message: msg,
				},
			})
			return
		}
	}
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	apierrors.RespondSimple(w, status, msg)
}

func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return -1
		}
		return r
	}, name)
	name = strings.ReplaceAll(name, "..", "")
	return name
}

func (ws *webServer) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":  "gridsim-proxy-export-v1",
		"exported_at": time.Now().UTC().Format(time.RFC3339),
		"collections": ws.proxyStore.GetCollections(),
		"environments": ws.proxyStore.GetEnvironments(),
		"active_env_id": ws.proxyStore.ActiveEnvID,
	})
}

var builtinCollectionIDs = map[string]bool{}
var builtinEnvIDs = map[string]bool{}

func init() {
	if data, err := builtinResources.ReadFile("resources/gridsim-builtin.postman_collection.json"); err == nil {
		var pm struct {
			Item []map[string]any `json:"item"`
		}
		if json.Unmarshal(data, &pm) == nil {
			collectIDs(pm.Item, builtinCollectionIDs)
		}
	}
	if data, err := builtinResources.ReadFile("resources/gridsim-builtin.env.json"); err == nil {
		var envFile struct {
			Environments []*api.Environment `json:"environments"`
		}
		if json.Unmarshal(data, &envFile) == nil {
			for _, e := range envFile.Environments {
				builtinEnvIDs[e.ID] = true
			}
		}
	}
}

func collectIDs(items []map[string]any, out map[string]bool) {
	for _, item := range items {
		if id, ok := item["id"].(string); ok && id != "" {
			out[id] = true
		}
		if child, ok := item["item"].([]any); ok {
			for _, c := range child {
				if m, ok := c.(map[string]any); ok {
					if id, ok := m["id"].(string); ok && id != "" {
						out[id] = true
					}
					if sub, ok := m["item"].([]any); ok {
						inner := make([]map[string]any, 0, len(sub))
						for _, s := range sub {
							if mm, ok := s.(map[string]any); ok {
								inner = append(inner, mm)
							}
						}
						collectIDs(inner, out)
					}
				}
			}
		}
	}
}

func seedBuiltinProxyData(store *api.ProxyStore) error {
	if data, err := builtinResources.ReadFile("resources/gridsim-builtin.env.json"); err == nil {
		var envFile struct {
			Environments []*api.Environment `json:"environments"`
			ActiveEnvID  string              `json:"active_env_id"`
		}
		if json.Unmarshal(data, &envFile) == nil {
			existing := store.GetEnvironments()
			have := map[string]bool{}
			for _, e := range existing {
				have[e.ID] = true
			}
			for _, e := range envFile.Environments {
				if !have[e.ID] {
					if err := store.SaveEnvironment(e); err != nil {
						return fmt.Errorf("save env: %w", err)
					}
				}
			}
			if envFile.ActiveEnvID != "" {
				_ = store.SetActiveEnv(envFile.ActiveEnvID)
			}
		}
	}

	if data, err := builtinResources.ReadFile("resources/gridsim-builtin.postman_collection.json"); err == nil {
		var pm struct {
			Item []map[string]any `json:"item"`
		}
		if json.Unmarshal(data, &pm) == nil {
			existing := store.GetCollections()
			have := map[string]bool{}
			for _, c := range existing {
				have[c.ID] = true
			}
			added := 0
			for _, item := range pm.Item {
				root := postmanItemToCollection(item)
				if root != nil && !have[root.ID] {
					if err := store.SaveCollection(root); err != nil {
						return fmt.Errorf("save collection: %w", err)
					}
					added++
				}
			}
			if added > 0 {
				slog.Info("已写入内置 API 集合", "folders", added)
			}
		}
	}
	return nil
}

func postmanItemToCollection(item map[string]any) *api.CollectionItem {
	name, _ := item["name"].(string)
	id, _ := item["id"].(string)
	if id == "" {
		id = genBuiltinID(name)
	}
	node := &api.CollectionItem{
		ID:   id,
		Name: name,
		Type: "folder",
	}
	if req, ok := item["request"].(map[string]any); ok {
		node.Type = "request"
		node.Method, _ = req["method"].(string)
		if u, ok := req["url"].(map[string]any); ok {
			if raw, ok := u["raw"].(string); ok {
				node.URL = raw
			}
		} else if u, ok := req["url"].(string); ok {
			node.URL = u
		}
		headers := map[string]string{}
		if hs, ok := req["header"].([]any); ok {
			for _, h := range hs {
				if hm, ok := h.(map[string]any); ok {
					k, _ := hm["key"].(string)
					v, _ := hm["value"].(string)
					if k != "" {
						headers[k] = v
					}
				}
			}
		}
		if len(headers) > 0 {
			node.Headers = headers
		}
		if body, ok := req["body"].(map[string]any); ok {
			if raw, ok := body["raw"].(string); ok {
				node.Body = raw
			}
		}
		if evts, ok := item["event"].([]any); ok {
			var pre, post string
			for _, e := range evts {
				em, _ := e.(map[string]any)
				listen, _ := em["listen"].(string)
				if script, ok := em["script"].(map[string]any); ok {
					if exec, ok := script["exec"].([]any); ok {
						lines := make([]string, 0, len(exec))
						for _, l := range exec {
							if s, ok := l.(string); ok {
								lines = append(lines, s)
							}
						}
						body := joinLines(lines)
						if listen == "prerequest" {
							pre = body
						} else if listen == "test" {
							post = body
						}
					}
				}
			}
			if pre != "" {
				node.PreScript = pre
			}
			if post != "" {
				node.TestScript = post
			}
		}
	}
	if child, ok := item["item"].([]any); ok {
		for _, c := range child {
			cm, _ := c.(map[string]any)
			converted := postmanItemToCollection(cm)
			if converted != nil {
				node.Children = append(node.Children, converted)
			}
		}
	}
	return node
}

func joinLines(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}

func genBuiltinID(name string) string {
	h := uint32(2166136261)
	for i := 0; i < len(name); i++ {
		h ^= uint32(name[i])
		h *= 16777619
	}
	return fmt.Sprintf("builtin-%08x", h)
}
