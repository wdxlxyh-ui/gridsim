package main

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"iec104-sim/api"
	"iec104-sim/config"
	"iec104-sim/iec104"
	"iec104-sim/library"

	"github.com/spf13/pflag"
)

var (
	port   int
	cfg    string
	httpAddr string
	logLvl string
)

func main() {
	pflag.IntVarP(&port, "port", "p", 2404, "IEC104 TCP 端口号")
	pflag.StringVarP(&cfg, "config", "c", "", "配置文件路径 (.xlsx)")
	pflag.StringVarP(&httpAddr, "http", "H", ":8080", "HTTP API 监听地址")
	pflag.StringVarP(&logLvl, "log", "l", "info", "日志级别: debug/info/warn/error")
	pflag.Parse()

	if cfg == "" {
		slog.Error("必须指定配置文件路径 (-c)")
		os.Exit(1)
	}

	setupLogLevel(logLvl)

	slog.Info("启动模拟器", "port", port, "config", cfg, "http", httpAddr)

	// Load config
	points, err := config.LoadFromXLSX(cfg)
	if err != nil {
		slog.Error("加载配置文件失败", "error", err)
		os.Exit(1)
	}

	counts := countByType(points)
	slog.Info("点表加载完成",
		"totalPoints", len(points),
		"AI", counts["AI"],
		"DI", counts["DI"],
		"PI", counts["PI"],
		"DO", counts["DO"],
		"AO", counts["AO"],
	)

	// Init store
	store := library.NewStore(points)

	// Init IEC104 server
	server := iec104.NewServer(port, store)
	if err := server.Start(); err != nil {
		slog.Error("启动IEC104服务端失败", "error", err)
		os.Exit(1)
	}

	// Init HTTP API
	apiHandler := api.NewHandler(store, server, server)
	mux := http.NewServeMux()
	apiHandler.Register(mux)

	httpSrv := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	go func() {
		slog.Info("HTTP API 已启动", "addr", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务失败", "error", err)
		}
	}()

	// Wait for signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("收到信号，正在关闭", "signal", sig)

	server.Stop()
	httpSrv.Close()
	slog.Info("模拟器已关闭")
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

func countByType(points []*config.Point) map[string]int {
	counts := map[string]int{"AI": 0, "DI": 0, "PI": 0, "DO": 0, "AO": 0}
	for _, p := range points {
		counts[string(p.PointType)]++
	}
	return counts
}
