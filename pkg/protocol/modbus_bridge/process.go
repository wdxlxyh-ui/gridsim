package modbus_bridge

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Process manages the py-microgrid-sim subprocess.
type Process struct {
	scriptDir string // working directory (contains config/device.json)
	cfgDir    string // config directory (bin/py-microgrid-sim is relative to exe)

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
}

// NewProcess creates a new process manager.
func NewProcess(scriptDir, cfgDir string) *Process {
	return &Process{
		scriptDir: scriptDir,
		cfgDir:    cfgDir,
		done:      make(chan struct{}),
	}
}

// findBinary locates the py-microgrid-sim executable.
// Search order: 1) bin/py-microgrid-sim/ dir next to gridsim exe
//               2) cfgDir/../bin/py-microgrid-sim/
//               3) scriptDir itself (fallback to python3 main.py)
func (p *Process) findBinary() (string, []string) {
	// Get directory of the current executable
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	// Possible binary locations
	binName := "py-microgrid-sim"
	if runtime.GOOS == "windows" {
		binName = "py-microgrid-sim.exe"
	}

	candidates := []string{
		filepath.Join(exeDir, "py-microgrid-sim", binName),     // bin/py-microgrid-sim/py-microgrid-sim
		filepath.Join(exeDir, binName),                          // bin/py-microgrid-sim (single file)
		filepath.Join(exeDir, "..", "bin", "py-microgrid-sim", binName), // ../bin/py-microgrid-sim/
		filepath.Join(p.cfgDir, "..", "bin", "py-microgrid-sim", binName),
	}

	for _, path := range candidates {
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			slog.Info("ModbusBridge: found binary", "path", path)
			return path, nil
		}
	}

	// Fallback: use python3 main.py (requires Python installed)
	slog.Warn("ModbusBridge: compiled binary not found, falling back to python3")
	return "python3", []string{"main.py"}
}

// Start launches the py-microgrid-sim subprocess.
func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	binary, args := p.findBinary()

	var cmd *exec.Cmd
	if len(args) > 0 {
		cmd = exec.Command(binary, args...)
	} else {
		cmd = exec.Command(binary)
	}
	cmd.Dir = p.scriptDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", binary, err)
	}

	p.cmd = cmd
	slog.Info("ModbusBridge: process started", "pid", cmd.Process.Pid, "binary", binary, "workdir", p.scriptDir)

	// Monitor process in background
	go func() {
		err := cmd.Wait()
		if err != nil {
			slog.Warn("ModbusBridge: process exited", "error", err)
		} else {
			slog.Info("ModbusBridge: process exited normally")
		}
		close(p.done)
	}()

	return nil
}

// Stop kills the subprocess.
func (p *Process) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return
	}

	slog.Info("ModbusBridge: stopping process", "pid", p.cmd.Process.Pid)

	if runtime.GOOS == "windows" {
		kill := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", p.cmd.Process.Pid))
		kill.Run()
	} else {
		p.cmd.Process.Signal(os.Interrupt)
		select {
		case <-p.done:
			return
		case <-time.After(3 * time.Second):
			p.cmd.Process.Kill()
		}
	}
}

// IsRunning checks if the process is still alive.
func (p *Process) IsRunning() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}
