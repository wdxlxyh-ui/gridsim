package microgrid

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"gridsim/pkg/library"
)

var formulaRefRE = regexp.MustCompile(`\{([^}]+)\}`)

// Engine 微电网仿真引擎
type Engine struct {
	mu       sync.Mutex
	topology *Topology
	store    *library.Store
	cfg      InstanceConfig

	running  bool
	stopCh   chan struct{}
	tickDone chan struct{}

	// 仿真状态
	soc       map[string]float64 // device_id → current SOC
	pvPower   map[string]float64 // device_id → generated PV power
	loadPower map[string]float64 // device_id → load/charger power
	batPower  map[string]float64 // device_id → battery power (+discharge, -charge)
	gridPower float64

	// 历史
	history *HistoryBuffer
}

// NewEngine 创建微电网仿真引擎
func NewEngine(topology *Topology, store *library.Store, cfg InstanceConfig) *Engine {
	soc := make(map[string]float64)
	for _, dev := range topology.Devices {
		if dev.Type == CompBattery {
			soc[dev.ID] = dev.Params.InitSOC
		}
	}
	return &Engine{
		topology:  topology,
		store:     store,
		cfg:       cfg,
		soc:       soc,
		pvPower:   make(map[string]float64),
		loadPower: make(map[string]float64),
		batPower:  make(map[string]float64),
		history:   NewHistoryBuffer(3600),
	}
}

// Start 启动仿真引擎
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return nil
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.tickDone = make(chan struct{})

	tickMs := e.cfg.TickMs
	if tickMs <= 0 {
		tickMs = 1000
	}
	speed := e.cfg.SpeedFactor
	if speed <= 0 {
		speed = 1
	}
	interval := time.Duration(float64(tickMs)/speed) * time.Millisecond
	if interval < 50*time.Millisecond {
		interval = 50 * time.Millisecond
	}

	go e.runLoop(interval)
	slog.Info("微电网仿真引擎已启动", "tickMs", tickMs, "speed", speed, "interval", interval)
	return nil
}

// Stop 停止仿真引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return
	}
	e.running = false
	close(e.stopCh)
	<-e.tickDone
	slog.Info("微电网仿真引擎已停止")
}

// IsRunning 返回引擎是否运行
func (e *Engine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// GetSOC 返回指定储能设备的 SOC
func (e *Engine) GetSOC(devID string) (float64, bool) {
	v, ok := e.soc[devID]
	return v, ok
}

// GetHistory 获取历史缓冲区
func (e *Engine) GetHistory() []SimSnapshot {
	return e.history.Snapshots()
}

// SetSwitch 设置开关状态并更新 Store
func (e *Engine) SetSwitch(devID string, closed bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.topology.Devices {
		if e.topology.Devices[i].ID == devID {
			e.topology.Devices[i].Switch.Closed = closed
			e.updateSwitchPoints(devID, closed)
			return nil
		}
	}
	return nil
}

// GetDevices 返回设备列表 (线程安全)
func (e *Engine) GetDevices() []Device {
	e.mu.Lock()
	defer e.mu.Unlock()
	devs := make([]Device, len(e.topology.Devices))
	copy(devs, e.topology.Devices)
	return devs
}

// Dashboard 返回仪表盘数据 (read-only, no side effects)
func (e *Engine) Dashboard() map[string]interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	totalGen := 0.0
	totalLoad := 0.0
	batPower := 0.0
	batSOC := 0.0
	batCnt := 0

	for _, dev := range e.topology.Devices {
		if !dev.Switch.Closed {
			continue
		}
		switch dev.Type {
		case CompPV:
			totalGen += e.pvPower[dev.ID]
		case CompBattery:
			p := e.batPower[dev.ID]
			if p > 0 {
				totalGen += p
			} else {
				totalLoad += -p
			}
			batPower += p
			if s, ok := e.soc[dev.ID]; ok {
				batSOC += s
				batCnt++
			}
		case CompLoad:
			totalLoad += e.loadPower[dev.ID]
		case CompCharger:
			totalLoad += e.loadPower[dev.ID]
		}
	}
	avgSOC := 0.0
	if batCnt > 0 {
		avgSOC = batSOC / float64(batCnt)
	}
	grid := totalGen - totalLoad
	return map[string]interface{}{
		"total_generation_kw": math.Round(totalGen*10) / 10,
		"total_load_kw":       math.Round(totalLoad*10) / 10,
		"grid_power_kw":       math.Round(grid*10) / 10,
		"battery_power_kw":    math.Round(batPower*10) / 10,
		"battery_soc":         math.Round(avgSOC*10) / 10,
		"frequency_hz":        50.0 + grid*0.001,
	}
}

// ─── internal ───

func (e *Engine) runLoop(interval time.Duration) {
	defer close(e.tickDone)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

func (e *Engine) tick() {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. Calculate PV power
	for _, dev := range e.topology.Devices {
		if dev.Type == CompPV && dev.Switch.Closed {
			irradiance := 300.0 + rand.Float64()*600.0
			p := irradiance / 1000.0 * dev.Params.RatedPowerKW * dev.Params.Efficiency
			if p < 0 {
				p = 0
			}
			e.pvPower[dev.ID] = p
		} else if dev.Type == CompPV {
			e.pvPower[dev.ID] = 0
		}
	}

	// 2. Calculate load/charger power
	for _, dev := range e.topology.Devices {
		switch dev.Type {
		case CompLoad:
			if dev.Switch.Closed {
				e.loadPower[dev.ID] = dev.Params.LoadRatedKW * (0.5 + rand.Float64()*0.5)
			} else {
				e.loadPower[dev.ID] = 0
			}
		case CompCharger:
			if dev.Switch.Closed {
				e.loadPower[dev.ID] = dev.Params.ChargerRatedKW * 0.3 * (0.5 + rand.Float64()*0.5)
			} else {
				e.loadPower[dev.ID] = 0
			}
		}
	}

	// 3. Calculate battery power from AO setpoint or dispatch
	for _, dev := range e.topology.Devices {
		if dev.Type == CompBattery {
			e.batPower[dev.ID] = e.calcBatteryPowerLocked(dev)
		}
	}

	// 4. Power balance
	result := e.calcPowerBalanceLocked()
	e.gridPower = result.GridPowerKW

	// 5. Update store with all computed values
	e.syncStoreLocked()

	// 5.5 Evaluate user-defined formulas (override store values)
	e.evaluateFormulasLocked()

	// 6. Record snapshot
	snap := SimSnapshot{
		Timestamp: time.Now().UnixMilli(),
		Values:    make(map[string]float64),
	}
	for _, dev := range e.topology.Devices {
		switch dev.Type {
		case CompPV:
			snap.Values[dev.ID] = e.pvPower[dev.ID]
		case CompBattery:
			snap.Values[dev.ID] = e.batPower[dev.ID]
			if s, ok := e.soc[dev.ID]; ok {
				snap.Values[dev.ID+"_SOC"] = s
			}
		case CompLoad, CompCharger:
			snap.Values[dev.ID] = e.loadPower[dev.ID]
		}
	}
	snap.Values["_grid"] = result.GridPowerKW
	e.history.Push(snap)
}

// calcBatteryPowerLocked 计算电池功率 (+放电, -充电)
// 优先使用 AO 设定值，回退到按时段调度
func (e *Engine) calcBatteryPowerLocked(dev Device) float64 {
	ratedP := dev.Params.RatedPowerKW_B
	if ratedP <= 0 {
		ratedP = 50
	}

	// 1. Try AO setpoint from store (遥控/遥调设定值)
	setpoint := e.readStoreValue(dev.ID + "_Setpoint")
	if setpoint != 0 {
		// Cap by rated power (不超过额定功率)
		if setpoint > ratedP {
			setpoint = ratedP
		} else if setpoint < -ratedP {
			setpoint = -ratedP
		}
		// Update SOC based on power exchange
		e.updateSOC(dev, setpoint)
		return setpoint
	}

	// 2. Fall back to time-based dispatch
	hour := time.Now().Hour()
	if hour >= 10 && hour <= 15 {
		// PV peak - charge battery
		chargeP := ratedP * 0.3
		if s, ok := e.soc[dev.ID]; ok {
			newSOC := s + chargeP*0.05/dev.Params.CapacityKWH*100
			if newSOC > dev.Params.SOCMax {
				chargeP = 0
			} else {
				e.soc[dev.ID] = newSOC
			}
		}
		return -chargeP
	} else if hour >= 18 && hour <= 22 {
		// Evening peak - discharge
		dischargeP := ratedP * 0.4
		if s, ok := e.soc[dev.ID]; ok {
			newSOC := s - dischargeP*0.05/dev.Params.CapacityKWH*100
			if newSOC < dev.Params.SOCMin {
				dischargeP = 0
			} else {
				e.soc[dev.ID] = newSOC
			}
		}
		return dischargeP
	}
	// Off-peak: charge at low rate
	if s, ok := e.soc[dev.ID]; ok && s < 50 {
		chargeP := ratedP * 0.15
		newSOC := s + chargeP*0.05/dev.Params.CapacityKWH*100
		if newSOC > dev.Params.SOCMax {
			chargeP = 0
		} else {
			e.soc[dev.ID] = newSOC
		}
		return -chargeP
	}
	return 0
}

// updateSOC 根据充放电功率更新 SOC
func (e *Engine) updateSOC(dev Device, power float64) {
	if dev.Params.CapacityKWH <= 0 {
		return
	}
	s, ok := e.soc[dev.ID]
	if !ok {
		return
	}
	// power positive = discharge (decrease SOC), negative = charge (increase SOC)
	// deltaSOC = -power * dt / capacity * 100
	dtHours := float64(e.cfg.TickMs) / 1000.0 / 3600.0
	if e.cfg.TickMs <= 0 {
		dtHours = 1.0 / 3600.0 // default 1s
	}
	deltaSOC := -power * dtHours / dev.Params.CapacityKWH * 100
	newSOC := s + deltaSOC
	if newSOC > dev.Params.SOCMax {
		newSOC = dev.Params.SOCMax
	} else if newSOC < dev.Params.SOCMin {
		newSOC = dev.Params.SOCMin
	}
	e.soc[dev.ID] = newSOC
}

// readStoreValue 从 store 读取点名对应的值
func (e *Engine) readStoreValue(name string) float64 {
	if e.store == nil {
		return 0
	}
	for _, p := range e.store.GetAll() {
		if p.Name == name {
			return p.Value
		}
	}
	return 0
}

// syncStoreLocked 将所有计算值同步到 store
func (e *Engine) syncStoreLocked() {
	if e.store == nil {
		return
	}
	for _, dev := range e.topology.Devices {
		switch dev.Type {
		case CompPV:
			e.updateStoreValue(dev.ID+"_Power", e.pvPower[dev.ID])
		case CompBattery:
			e.updateStoreValue(dev.ID+"_Power", e.batPower[dev.ID])
		case CompLoad:
			e.updateStoreValue(dev.ID+"_Power", e.loadPower[dev.ID])
		case CompCharger:
			e.updateStoreValue(dev.ID+"_Power", e.loadPower[dev.ID])
		}
		if s, ok := e.soc[dev.ID]; ok {
			e.updateStoreValue(dev.ID+"_SOC", s)
		}
	}
	// Grid values from latest power balance
	e.updateStoreValue("GRID_P", e.gridPower)
	e.updateStoreValue("GRID_Q", e.gridPower*0.15)
	e.updateStoreValue("GRID_Connected", 1)
	if e.topology.GridMeter.IslandMode {
		e.updateStoreValue("GRID_Connected", 0)
	}
}

// ─── Formula evaluation (Plan A) ───

func (e *Engine) evaluateFormulasLocked() {
	if e.store == nil {
		return
	}
	for _, f := range e.topology.Formulas {
		if !f.Enabled || f.Expression == "" {
			continue
		}
		expr := formulaRefRE.ReplaceAllStringFunc(f.Expression, func(match string) string {
			name := match[1 : len(match)-1]
			return fmt.Sprintf("%f", e.readStoreValue(name))
		})
		result, err := evaluateExpr(expr)
		if err != nil {
			slog.Warn("公式计算失败", "formula", f.Name, "expr", expr, "error", err)
			continue
		}
		e.updateStoreValue(f.Target, result)
	}
}

func evaluateExpr(s string) (float64, error) {
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0, fmt.Errorf("empty expression")
	}
	val, pos, err := parseAddExpr(s, 0)
	if err != nil {
		return 0, err
	}
	if pos < len(s) {
		return 0, fmt.Errorf("unexpected char at pos %d", pos)
	}
	return val, nil
}

func parseAddExpr(s string, pos int) (float64, int, error) {
	left, pos, err := parseMulExpr(s, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(s) && (s[pos] == '+' || s[pos] == '-') {
		op := s[pos]
		pos++
		right, np, err := parseMulExpr(s, pos)
		if err != nil {
			return 0, np, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
		pos = np
	}
	return left, pos, nil
}

func parseMulExpr(s string, pos int) (float64, int, error) {
	left, pos, err := parseUnary(s, pos)
	if err != nil {
		return 0, pos, err
	}
	for pos < len(s) && (s[pos] == '*' || s[pos] == '/') {
		op := s[pos]
		pos++
		right, np, err := parseUnary(s, pos)
		if err != nil {
			return 0, np, err
		}
		if op == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, np, fmt.Errorf("division by zero")
			}
			left /= right
		}
		pos = np
	}
	return left, pos, nil
}

func parseUnary(s string, pos int) (float64, int, error) {
	if pos >= len(s) {
		return 0, pos, fmt.Errorf("unexpected end")
	}
	if s[pos] == '+' {
		pos++
		return parseUnary(s, pos)
	}
	if s[pos] == '-' {
		pos++
		val, np, err := parseUnary(s, pos)
		return -val, np, err
	}
	return parseAtom(s, pos)
}

func parseAtom(s string, pos int) (float64, int, error) {
	if pos >= len(s) {
		return 0, pos, fmt.Errorf("unexpected end")
	}
	if s[pos] == '(' {
		pos++
		val, pos, err := parseAddExpr(s, pos)
		if err != nil {
			return 0, pos, err
		}
		if pos >= len(s) || s[pos] != ')' {
			return 0, pos, fmt.Errorf("missing ')'")
		}
		pos++
		return val, pos, nil
	}
	start := pos
	for pos < len(s) && (('0' <= s[pos] && s[pos] <= '9') || s[pos] == '.') {
		pos++
	}
	if pos == start {
		return 0, pos, fmt.Errorf("expected number at pos %d", start)
	}
	v, err := strconv.ParseFloat(s[start:pos], 64)
	if err != nil {
		return 0, pos, fmt.Errorf("invalid number %q", s[start:pos])
	}
	return v, pos, nil
}

// ─── Power Balance ───

type PowerBalanceResult struct {
	TotalGenerationKW float64
	TotalLoadKW       float64
	BatteryPowerKW    float64
	GridPowerKW       float64
	GridReactiveKVAR  float64
	ImbalanceKW       float64
	Frequency         float64
	Island            bool
}

func (e *Engine) calcPowerBalanceLocked() *PowerBalanceResult {
	totalGen := 0.0
	totalLoad := 0.0
	batChargeTotal := 0.0

	for _, dev := range e.topology.Devices {
		if !dev.Switch.Closed {
			continue
		}
		switch dev.Type {
		case CompPV:
			totalGen += e.pvPower[dev.ID]
		case CompBattery:
			p := e.batPower[dev.ID]
			batChargeTotal += p
			if p > 0 {
				totalGen += p
			} else {
				totalLoad += -p
			}
		case CompLoad:
			totalLoad += e.loadPower[dev.ID]
		case CompCharger:
			totalLoad += e.loadPower[dev.ID]
		}
	}

	imbalance := totalGen - totalLoad
	island := e.topology.GridMeter.IslandMode
	gridPower := 0.0

	if !island {
		cap := e.topology.GridMeter.RatedCapacityKW
		if cap <= 0 {
			cap = 10000
		}
		if imbalance > cap {
			gridPower = cap
		} else if imbalance < -cap {
			gridPower = -cap
		} else {
			gridPower = imbalance
		}
	}

	freq := 50.0
	if totalGen > 1 {
		freq = 50.0 + (imbalance/totalGen)*0.2
	}

	return &PowerBalanceResult{
		TotalGenerationKW: totalGen,
		TotalLoadKW:       totalLoad,
		BatteryPowerKW:    batChargeTotal,
		GridPowerKW:       math.Round(gridPower*10) / 10,
		GridReactiveKVAR:  math.Round(gridPower*0.15*10) / 10,
		ImbalanceKW:       math.Round(imbalance*10) / 10,
		Frequency:         math.Round(freq*100) / 100,
		Island:            island,
	}
}

// ─── Store helpers ───

func (e *Engine) updateStoreValue(name string, value float64) {
	if e.store == nil {
		return
	}
	for _, p := range e.store.GetAll() {
		if p.Name == name {
			e.store.SetValue(p.IOA, value)
			return
		}
	}
}

func (e *Engine) updateSwitchPoints(devID string, closed bool) {
	v := 0.0
	if closed {
		v = 1.0
	}
	for _, p := range e.store.GetAll() {
		if p.Name == devID+"_SwStatus" || p.Name == devID+"_SwCtrl" {
			e.store.SetValue(p.IOA, v)
		}
		if p.Name == devID+"_Power" && !closed {
			e.store.SetValue(p.IOA, 0)
		}
	}
}
