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
// 所有功率值从 store 读取，保证与 IEC104 送值一致。
func (e *Engine) Dashboard() map[string]interface{} {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 从 store 读取各设备功率
	pvPowers := make([]map[string]interface{}, 0)
	batPowers := make([]map[string]interface{}, 0)
	loadPowers := make([]map[string]interface{}, 0)
	chargerPowers := make([]map[string]interface{}, 0)

	totalGen := 0.0
	totalLoad := 0.0
	totalPV := 0.0
	totalBat := 0.0
	totalLoadKW := 0.0
	totalCharger := 0.0
	batSOCSum := 0.0
	batCnt := 0

	for _, dev := range e.topology.Devices {
		power := e.readStoreValue(dev.ID + "_Power")
		switch dev.Type {
		case CompPV:
			pvPowers = append(pvPowers, map[string]interface{}{
				"id": dev.ID, "name": dev.Name, "power_kw": math.Round(power*10)/10,
				"closed": dev.Switch.Closed, "mode": dev.ControlMode,
			})
			if dev.Switch.Closed {
				totalPV += power
				totalGen += power
			}
		case CompBattery:
			entry := map[string]interface{}{
				"id": dev.ID, "name": dev.Name, "power_kw": math.Round(power*10)/10,
				"closed": dev.Switch.Closed, "mode": dev.ControlMode,
			}
			if s, ok := e.soc[dev.ID]; ok {
				entry["soc"] = math.Round(s*10) / 10
			}
			batPowers = append(batPowers, entry)
			if dev.Switch.Closed {
				if power > 0 {
					totalLoad += power
				} else {
					totalGen += -power
				}
				totalBat += power
				if s, ok := e.soc[dev.ID]; ok {
					batSOCSum += s
					batCnt++
				}
			}
		case CompLoad:
			loadPowers = append(loadPowers, map[string]interface{}{
				"id": dev.ID, "name": dev.Name, "power_kw": math.Round(power*10)/10,
				"closed": dev.Switch.Closed, "mode": dev.ControlMode,
			})
			if dev.Switch.Closed && power > 0 {
				totalLoad += power
				totalLoadKW += power
			}
		case CompCharger:
			chargerPowers = append(chargerPowers, map[string]interface{}{
				"id": dev.ID, "name": dev.Name, "power_kw": math.Round(power*10)/10,
				"closed": dev.Switch.Closed, "mode": dev.ControlMode,
			})
			if dev.Switch.Closed && power > 0 {
				totalLoad += power
				totalCharger += power
			}
		}
	}

	gridPower := e.readStoreValue("GRID_P")
	avgSOC := 0.0
	if batCnt > 0 {
		avgSOC = batSOCSum / float64(batCnt)
	}

	return map[string]interface{}{
		"grid_power_kw":    math.Round(gridPower*10) / 10,
		"pv":               pvPowers,
		"battery":          batPowers,
		"load":             loadPowers,
		"charger":          chargerPowers,
		"total_pv_kw":      math.Round(totalPV*10) / 10,
		"total_bat_kw":     math.Round(totalBat*10) / 10,
		"total_load_kw":    math.Round(totalLoadKW*10) / 10,
		"total_charger_kw": math.Round(totalCharger*10) / 10,
		"battery_soc":      math.Round(avgSOC*10) / 10,
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

	// 1. Calculate PV power: AO setpoint > irradiance fallback
	for _, dev := range e.topology.Devices {
		if dev.Type != CompPV {
			continue
		}
		if !dev.Switch.Closed {
			e.pvPower[dev.ID] = 0
			continue
		}
		ratedP := dev.Params.RatedPowerKW
		if ratedP <= 0 {
			ratedP = 100
		}
		// Try AO setpoint first
		setpoint := e.readStoreValue(dev.ID + "_Setpoint")
		if setpoint > 0 {
			if setpoint > ratedP {
				setpoint = ratedP
			}
			e.pvPower[dev.ID] = setpoint
		} else {
			irradiance := 300.0 + rand.Float64()*600.0
			p := irradiance / 1000.0 * dev.Params.RatedPowerKW * dev.Params.Efficiency
			if p < 0 {
				p = 0
			}
			e.pvPower[dev.ID] = p
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
			if dev.Switch.Closed {
				e.batPower[dev.ID] = e.calcBatteryPowerLocked(dev)
			} else {
				e.batPower[dev.ID] = 0
			}
		}
	}

	// 3.5 Update SOC for all batteries based on final power
	for _, dev := range e.topology.Devices {
		if dev.Type == CompBattery && dev.Switch.Closed {
			e.updateSOC(dev, e.batPower[dev.ID])
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

// calcBatteryPowerLocked 计算电池功率 (+充电, -放电)
// 优先使用 AO 设定值，回退到按时段调度
func (e *Engine) calcBatteryPowerLocked(dev Device) float64 {
	ratedP := dev.Params.RatedPowerKW_B
	if ratedP <= 0 {
		ratedP = 50
	}

	// 1. Try AO setpoint from store (遥控/遥调设定值)
	setpoint := e.readStoreValue(dev.ID + "_Setpoint")
	if setpoint != 0 {
		if setpoint > ratedP {
			setpoint = ratedP
		} else if setpoint < -ratedP {
			setpoint = -ratedP
		}
		return setpoint
	}

	// 2. Fall back to time-based dispatch
	hour := time.Now().Hour()
	if hour >= 10 && hour <= 15 {
		// PV peak - charge battery (+ = charge)
		chargeP := ratedP * 0.3
		return chargeP
	} else if hour >= 18 && hour <= 22 {
		// Evening peak - discharge (- = discharge)
		dischargeP := ratedP * 0.4
		return -dischargeP
	}
	// Off-peak: charge at low rate
	if ratedP < 1 {
		ratedP = 50
	}
	return 0
}

// updateSOC 根据充放电功率更新 SOC
// power > 0 = 充电 → SOC↑, power < 0 = 放电 → SOC↓
func (e *Engine) updateSOC(dev Device, power float64) {
	if dev.Params.CapacityKWH <= 0 {
		return
	}
	s, ok := e.soc[dev.ID]
	if !ok {
		return
	}
	dtHours := float64(e.cfg.TickMs) / 1000.0 / 3600.0
	if e.cfg.TickMs <= 0 {
		dtHours = 1.0 / 3600.0
	}
	deltaSOC := power * dtHours / dev.Params.CapacityKWH * 100
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
			if p < 0 {
				totalGen += -p // discharge = generation
			} else {
				totalLoad += p // charge = load
			}
		case CompLoad:
			totalLoad += e.loadPower[dev.ID]
		case CompCharger:
			totalLoad += e.loadPower[dev.ID]
		}
	}

	island := e.topology.GridMeter.IslandMode
	grossLoad := totalLoad
	grossGen := totalGen
	gridPower := 0.0

	if !island {
		cap := e.topology.GridMeter.RatedCapacityKW
		if cap <= 0 {
			cap = 10000
		}
		// GRID > 0 = 从电网用电, GRID < 0 = 向电网送电
		raw := grossLoad - grossGen
		if raw > cap {
			gridPower = cap
		} else if raw < -cap {
			gridPower = -cap
		} else {
			gridPower = raw
		}
	}

	// Frequency: load-centric (more load → lower freq, more gen → higher freq)
	freq := 50.0
	denom := grossGen + grossLoad
	if denom > 1 {
		freq = 50.0 - (gridPower/denom)*0.5
	}

	return &PowerBalanceResult{
		TotalGenerationKW: totalGen,
		TotalLoadKW:       totalLoad,
		BatteryPowerKW:    batChargeTotal,
		GridPowerKW:       math.Round(gridPower*10) / 10,
		GridReactiveKVAR:  math.Round(gridPower*0.15*10) / 10,
		ImbalanceKW:       math.Round((grossLoad - grossGen)*10) / 10,
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
