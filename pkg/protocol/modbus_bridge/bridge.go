package modbus_bridge

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"gridsim/internal/model"
	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

// Bridge implements the Protocol interface for Python microgrid simulator.
// It manages a Python subprocess and polls data via Modbus TCP.
type Bridge struct {
	cfg     model.ModbusBridgeConfig
	cfgDir  string
	store   *library.Store
	proc    *Process
	devices []DeviceMapping

	mu        sync.RWMutex
	connected bool
	conn      net.Conn

	startTime   time.Time
	pollTicker  *time.Ticker
	stopCh      chan struct{}
	interrogCnt atomic.Int64
	controlCnt  atomic.Int64
	spontCnt    atomic.Int64
}

// New creates a new ModbusBridge protocol instance.
func New(cfg model.InstanceConfig, cfgDir string) *Bridge {
	bc := cfg.ModbusBridgeConfig
	if bc.PythonPath == "" {
		bc.PythonPath = "python3"
	}
	if bc.ModbusPort == 0 {
		bc.ModbusPort = 5021
	}
	if bc.PollIntervalMs == 0 {
		bc.PollIntervalMs = 1000
	}
	if bc.ScriptDir == "" {
		bc.ScriptDir = "py_simulator"
	}
	if bc.DeviceJSON == "" {
		bc.DeviceJSON = "config/device.json"
	}
	return &Bridge{
		cfg:    *bc,
		cfgDir: cfgDir,
		stopCh: make(chan struct{}),
	}
}

func (b *Bridge) Name() string { return "modbus_bridge" }

func (b *Bridge) SetStore(store *library.Store) {
	b.store = store
}

func (b *Bridge) Start() error {
	b.startTime = time.Now()

	// Parse device.json to build point mappings
	scriptDir := b.resolveScriptDir()
	devices, err := ParseDeviceJSON(scriptDir, b.cfg.DeviceJSON)
	if err != nil {
		return fmt.Errorf("parse device.json: %w", err)
	}
	b.devices = devices
	slog.Info("ModbusBridge: parsed devices", "count", len(devices))

	// Update start_time in device.json if configured
	startTime := b.cfg.StartTime
	if startTime == "" {
		startTime = time.Now().Format("15:04")
	}
	if err := UpdateStartTime(scriptDir, b.cfg.DeviceJSON, startTime); err != nil {
		slog.Warn("ModbusBridge: failed to update start_time", "error", err)
	}

	// Start Python subprocess
	b.proc = NewProcess(b.cfg.PythonPath, scriptDir)
	if err := b.proc.Start(); err != nil {
		return fmt.Errorf("start python: %w", err)
	}

	// Wait for Modbus port to be available
	if err := b.waitForPort(10 * time.Second); err != nil {
		b.proc.Stop()
		return fmt.Errorf("python modbus not ready: %w", err)
	}

	// Start polling loop
	b.pollTicker = time.NewTicker(time.Duration(b.cfg.PollIntervalMs) * time.Millisecond)
	go b.pollLoop()

	slog.Info("ModbusBridge: started", "port", b.cfg.ModbusPort, "poll_ms", b.cfg.PollIntervalMs)
	return nil
}

func (b *Bridge) Stop() {
	slog.Info("ModbusBridge: stopping")
	close(b.stopCh)
	if b.pollTicker != nil {
		b.pollTicker.Stop()
	}
	b.mu.Lock()
	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	b.connected = false
	b.mu.Unlock()

	if b.proc != nil {
		b.proc.Stop()
	}
	slog.Info("ModbusBridge: stopped")
}

func (b *Bridge) ClientConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

func (b *Bridge) ClientAddr() string {
	return fmt.Sprintf("127.0.0.1:%d", b.cfg.ModbusPort)
}

func (b *Bridge) Stats() (interrog, control, spont int64) {
	return b.interrogCnt.Load(), b.controlCnt.Load(), b.spontCnt.Load()
}

func (b *Bridge) Uptime() int64 {
	return int64(time.Since(b.startTime).Seconds())
}

func (b *Bridge) Publish(point *config.Point) {
	// When a point is written via API, write it back to Python via Modbus FC16
	if point.PointType == config.TypeAO {
		go b.writeToDevice(point)
	}
	b.spontCnt.Add(1)
}

// ─── Internal methods ───

func (b *Bridge) resolveScriptDir() string {
	dir := b.cfg.ScriptDir
	if dir != "" && dir[0] != '/' && (len(dir) < 2 || dir[1] != ':') {
		return b.cfgDir + "/" + dir
	}
	return dir
}

func (b *Bridge) waitForPort(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", b.cfg.ModbusPort)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			slog.Info("ModbusBridge: Python Modbus port ready", "addr", addr)
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}

func (b *Bridge) ensureConnection() (net.Conn, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.conn != nil {
		return b.conn, nil
	}
	addr := fmt.Sprintf("127.0.0.1:%d", b.cfg.ModbusPort)
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		b.connected = false
		return nil, err
	}
	b.conn = conn
	b.connected = true
	return conn, nil
}

func (b *Bridge) pollLoop() {
	for {
		select {
		case <-b.stopCh:
			return
		case <-b.pollTicker.C:
			b.pollAllDevices()
		}
	}
}

func (b *Bridge) pollAllDevices() {
	conn, err := b.ensureConnection()
	if err != nil {
		slog.Debug("ModbusBridge: connection failed", "error", err)
		return
	}

	for _, dev := range b.devices {
		if err := b.readDevice(conn, dev); err != nil {
			slog.Warn("ModbusBridge: read failed", "device", dev.DeviceKey, "error", err)
			// Connection may be broken, reset
			b.mu.Lock()
			if b.conn != nil {
				b.conn.Close()
				b.conn = nil
			}
			b.connected = false
			b.mu.Unlock()
			return
		}
	}
	b.interrogCnt.Add(1)
}

func (b *Bridge) readDevice(conn net.Conn, dev DeviceMapping) error {
	if len(dev.Points) == 0 {
		return nil
	}

	// Find max register offset to determine read range
	maxOffset := uint16(0)
	for _, pt := range dev.Points {
		if pt.RegisterOffset > maxOffset {
			maxOffset = pt.RegisterOffset
		}
	}
	// Each point uses 2 registers (32-bit), read from 0 to maxOffset+2
	quantity := maxOffset + 2

	// Build Modbus TCP FC03 request
	txID := uint16(time.Now().UnixNano() & 0xFFFF)
	req := make([]byte, 12)
	binary.BigEndian.PutUint16(req[0:2], txID)         // Transaction ID
	binary.BigEndian.PutUint16(req[2:4], 0)            // Protocol ID
	binary.BigEndian.PutUint16(req[4:6], 6)            // Length
	req[6] = dev.SlaveID                               // Unit ID
	req[7] = 0x03                                      // Function Code: Read Holding Registers
	binary.BigEndian.PutUint16(req[8:10], 0)           // Start Address
	binary.BigEndian.PutUint16(req[10:12], quantity)   // Quantity

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("write FC03: %w", err)
	}

	// Read response header (MBAP: 7 bytes + FC + byte count)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	header := make([]byte, 9)
	if _, err := io.ReadFull(conn, header); err != nil {
		return fmt.Errorf("read header: %w", err)
	}

	if header[7] == 0x83 {
		return fmt.Errorf("modbus error: exception code 0x%02x", header[8])
	}

	byteCount := int(header[8])
	data := make([]byte, byteCount)
	if _, err := io.ReadFull(conn, data); err != nil {
		return fmt.Errorf("read data: %w", err)
	}

	// Parse register values and update store
	// Python encodes: int_value = actual_value * 100, stored as 32-bit signed big-endian
	const COEFFICIENT = 100.0
	for _, pt := range dev.Points {
		offset := int(pt.RegisterOffset) * 2 // byte offset in data
		if offset+4 > len(data) {
			continue
		}
		intVal := int32(binary.BigEndian.Uint32(data[offset : offset+4]))
		value := float64(intVal) / COEFFICIENT

		if b.store != nil {
			oldPt, ok := b.store.Get(pt.IOA)
			if ok {
				changed := false
				switch oldPt.PointType {
				case config.TypeAI:
					if oldPt.Value != value {
						b.store.SetValue(pt.IOA, value)
						changed = true
					}
				case config.TypeDI:
					boolVal := value != 0
					if oldPt.BoolValue != boolVal {
						b.store.SetBoolValue(pt.IOA, boolVal)
						changed = true
					}
				case config.TypeAO:
					// AO points: only update if not manually set (avoid overwriting user commands)
					// Skip update for AO — user writes take priority
				}
				_ = changed
			}
		}
	}
	return nil
}

func (b *Bridge) writeToDevice(point *config.Point) {
	// Find the device and register for this IOA
	for _, dev := range b.devices {
		for _, pt := range dev.Points {
			if pt.IOA == point.IOA && pt.Writable {
				b.modbusWrite(dev.SlaveID, pt.RegisterOffset, point.Value)
				b.controlCnt.Add(1)
				slog.Info("ModbusBridge: wrote to Python", "device", dev.DeviceKey, "reg", pt.RegisterOffset, "value", point.Value)
				return
			}
		}
	}
}

func (b *Bridge) modbusWrite(slaveID uint8, regOffset uint16, value float64) error {
	conn, err := b.ensureConnection()
	if err != nil {
		return err
	}

	const COEFFICIENT = 100.0
	intVal := int32(value * COEFFICIENT)
	if intVal > 2147483647 {
		intVal = 2147483647
	}
	if intVal < -2147483648 {
		intVal = -2147483648
	}

	// Build FC16 Write Multiple Registers (2 registers = 4 bytes for 32-bit value)
	txID := uint16(time.Now().UnixNano() & 0xFFFF)
	req := make([]byte, 17)
	binary.BigEndian.PutUint16(req[0:2], txID)         // Transaction ID
	binary.BigEndian.PutUint16(req[2:4], 0)            // Protocol ID
	binary.BigEndian.PutUint16(req[4:6], 11)           // Length (unit + fc + addr + qty + count + data)
	req[6] = slaveID                                   // Unit ID
	req[7] = 0x10                                      // Function Code 16: Write Multiple Registers
	binary.BigEndian.PutUint16(req[8:10], regOffset)   // Start Address
	binary.BigEndian.PutUint16(req[10:12], 2)          // Quantity (2 registers for 32-bit)
	req[12] = 4                                        // Byte count
	binary.BigEndian.PutUint32(req[13:17], uint32(intVal)) // Data (signed as unsigned bytes)

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(req); err != nil {
		return fmt.Errorf("write FC16: %w", err)
	}

	// Read response (12 bytes for FC16 response)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp := make([]byte, 12)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return fmt.Errorf("read FC16 response: %w", err)
	}

	return nil
}
