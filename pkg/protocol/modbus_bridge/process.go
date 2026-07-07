package modbus_bridge

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"
)

// Process manages a Python subprocess.
type Process struct {
	pythonPath string
	scriptDir  string

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
}

// NewProcess creates a new process manager.
func NewProcess(pythonPath, scriptDir string) *Process {
	return &Process{
		pythonPath: pythonPath,
		scriptDir:  scriptDir,
		done:       make(chan struct{}),
	}
}

// Start launches the Python main.py subprocess.
func (p *Process) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	cmd := exec.Command(p.pythonPath, "main.py")
	cmd.Dir = p.scriptDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	p.cmd = cmd
	slog.Info("ModbusBridge: Python process started", "pid", cmd.Process.Pid, "dir", p.scriptDir)

	// Monitor process in background
	go func() {
		err := cmd.Wait()
		if err != nil {
			slog.Warn("ModbusBridge: Python process exited", "error", err)
		} else {
			slog.Info("ModbusBridge: Python process exited normally")
		}
		close(p.done)
	}()

	return nil
}

// Stop kills the Python subprocess.
func (p *Process) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cmd == nil || p.cmd.Process == nil {
		return
	}

	slog.Info("ModbusBridge: stopping Python process", "pid", p.cmd.Process.Pid)

	if runtime.GOOS == "windows" {
		// On Windows, use taskkill to force kill the process tree
		kill := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", p.cmd.Process.Pid))
		kill.Run()
	} else {
		// On Linux/Mac, send SIGTERM first
		p.cmd.Process.Signal(os.Interrupt)
		// Give it 3 seconds to exit gracefully
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
