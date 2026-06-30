//go:build windows

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jchv/go-webview2"
)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
	procAllocConsole  = kernel32.NewProc("AllocConsole")
)

const ATTACH_PARENT_PROCESS = ^uint32(0) // -1

// attachConsole attaches to the parent process's console (if any).
// This is needed because we build with -H windowsgui which detaches from console.
func attachConsole() {
	r, _, _ := procAttachConsole.Call(uintptr(ATTACH_PARENT_PROCESS))
	if r == 0 {
		return
	}
	// Reopen stdout/stderr to the attached console
	hOut, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	hErr, _ := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)
	os.Stdout = os.NewFile(uintptr(hOut), "stdout")
	os.Stderr = os.NewFile(uintptr(hErr), "stderr")
}

func main() {
	// If launched with explicit subcommand, use headless CLI mode (same as Linux).
	if len(os.Args) > 1 {
		// Attach to parent console for CLI output
		attachConsole()

		switch os.Args[1] {
		case "serve":
			runServerMode()
			return
		case "--help", "-h":
			fmt.Println("GridSim - IEC104/Modbus Simulator")
			fmt.Println()
			fmt.Println("Usage:")
			fmt.Println("  gridsim.exe              Launch GUI mode (WebView2 desktop window)")
			fmt.Println("  gridsim.exe serve [...]  Headless server mode (same as Linux)")
			fmt.Println("  gridsim.exe -c <file>    Legacy single-instance mode")
			fmt.Println()
			return
		default:
			// Legacy mode: -c flag etc.
			runLegacyMode()
			return
		}
	}

	// ─── GUI Mode: double-click exe → start server + open WebView2 window ───
	runGUIMode()
}

// runGUIMode starts the GridSim backend server, then opens a native WebView2 window.
func runGUIMode() {
	// Ensure working directory is the package root (so ./config, ./web/dist resolve)
	ensureWorkingDir()

	// 1. Find a free port for the HTTP server
	port, err := findFreePort()
	if err != nil {
		showError("无法分配端口: " + err.Error())
		return
	}
	httpAddr := fmt.Sprintf(":%d", port)

	// 2. Start the backend server in a goroutine
	go func() {
		os.Args = []string{os.Args[0], "serve", "--http", httpAddr, "--config-dir", "./config", "--log-dir", "./logs", "--log", "info"}
		runServerMode()
	}()

	// 3. Wait for the server to be ready
	url := fmt.Sprintf("http://localhost:%d", port)
	if !waitForServer(url, 15*time.Second) {
		showError("服务启动超时，请检查日志目录 logs/")
		return
	}

	// 4. Open WebView2 window
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  fmt.Sprintf("GridSim v%s - IEC104/Modbus 模拟器", version),
			Width:  1400,
			Height: 900,
			IconId: 2,
			Center: true,
		},
	})
	if w == nil {
		// WebView2 runtime not available — fall back to opening browser
		fallbackOpenBrowser(url)
		return
	}
	defer w.Destroy()

	w.SetSize(1400, 900, webview2.HintNone)
	w.Navigate(url)
	w.Run() // Blocks until window is closed

	// Window closed → process exits
	os.Exit(0)
}

// ensureWorkingDir sets cwd to the package root directory.
// Expected layout: root/bin/gridsim.exe, root/config/, root/web/dist/
func ensureWorkingDir() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}
	exeDir := filepath.Dir(exePath) // e.g. root/bin/

	// If exe is in a "bin" subdirectory, go up one level to package root
	if strings.EqualFold(filepath.Base(exeDir), "bin") {
		root := filepath.Dir(exeDir)
		if dirExists(filepath.Join(root, "config")) || dirExists(filepath.Join(root, "web")) {
			os.Chdir(root)
			return
		}
	}

	// Otherwise assume exe is at root level
	if dirExists(filepath.Join(exeDir, "config")) || dirExists(filepath.Join(exeDir, "web")) {
		os.Chdir(exeDir)
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// findFreePort returns port 8989 if available, otherwise a random free port.
func findFreePort() (int, error) {
	if isPortFree(8989) {
		return 8989, nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port, nil
}

func isPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// waitForServer polls the URL until it responds with HTTP 200.
func waitForServer(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/api/v1/status")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

// fallbackOpenBrowser opens the URL in the default browser when WebView2
// runtime is not installed. Allocates a console and blocks.
func fallbackOpenBrowser(url string) {
	// Allocate a console for output since we're a GUI app
	procAllocConsole.Call()
	hOut, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	hErr, _ := syscall.GetStdHandle(syscall.STD_ERROR_HANDLE)
	os.Stdout = os.NewFile(uintptr(hOut), "stdout")
	os.Stderr = os.NewFile(uintptr(hErr), "stderr")

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("  WebView2 运行时未安装，已使用默认浏览器打开。")
	fmt.Println("  如需原生窗口体验，请安装 Microsoft Edge WebView2 Runtime:")
	fmt.Println("  https://developer.microsoft.com/en-us/microsoft-edge/webview2/")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()
	fmt.Printf("  Web UI: %s\n", url)
	fmt.Println("  按 Ctrl+C 或关闭此窗口停止服务...")
	fmt.Println()

	exec.Command("cmd", "/c", "start", url).Start()

	// Block until Ctrl+C
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
}

// showError displays an error to the user via Windows MessageBox.
func showError(msg string) {
	fmt.Fprintf(os.Stderr, "ERROR: %s\n", msg)
	exec.Command("powershell", "-Command",
		fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.MessageBox]::Show('%s', 'GridSim 错误', 'OK', 'Error')`,
			strings.ReplaceAll(msg, "'", "''"))).Run()
}
