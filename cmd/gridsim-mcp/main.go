package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	mcp_srv "github.com/mark3labs/mcp-go/server"

	"gridsim/internal/mcp"
)

func main() {
	simulatorURL := flag.String("simulator", "http://localhost:8989",
		"IEC104 模拟器的 HTTP API 地址")
	mode := flag.String("mode", "both",
		"MCP 服务模式: instance / data / both")
	flag.Parse()

	client := mcp.NewSimulatorClient(*simulatorURL)

	switch *mode {
	case "instance":
		s := mcp.NewInstanceManagerServer(client)
		log.Printf("启动 IEC104 Instance Manager MCP Server (stdio)")
		log.Printf("连接模拟器: %s", *simulatorURL)
		if err := mcp_srv.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	case "data":
		s := mcp.NewDataInterfaceServer(client)
		log.Printf("启动 IEC104 Data Interface MCP Server (stdio)")
		log.Printf("连接模拟器: %s", *simulatorURL)
		if err := mcp_srv.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	case "proxy":
		s := mcp.NewProxyServer(client)
		log.Printf("启动 IEC104 API Tester MCP Server (stdio)")
		log.Printf("连接模拟器: %s", *simulatorURL)
		if err := mcp_srv.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	case "both":
		// Combined server: all tools from all services in one MCP server
		s := mcp_srv.NewMCPServer(
			"IEC104 Simulator MCP",
			"1.0.0",
			mcp_srv.WithLogging(),
		)
		instSrv := mcp.NewInstanceManagerServer(client)
		for _, t := range instSrv.ListTools() {
			s.AddTool(t.Tool, t.Handler)
		}
		dataSrv := mcp.NewDataInterfaceServer(client)
		for _, t := range dataSrv.ListTools() {
			s.AddTool(t.Tool, t.Handler)
		}
		proxySrv := mcp.NewProxyServer(client)
		for _, t := range proxySrv.ListTools() {
			s.AddTool(t.Tool, t.Handler)
		}
		log.Printf("启动 IEC104 Simulator MCP Server (全部工具, stdio)")
		log.Printf("连接模拟器: %s", *simulatorURL)
		if err := mcp_srv.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "未知模式: %s (可选: instance / data / proxy / both)\n", *mode)
		os.Exit(1)
	}
}
