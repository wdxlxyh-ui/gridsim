package microgrid

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
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
	batPower  map[string]float64 // device_id → battery power
	gridPower float64

	// IOA 索引: name → IOA (由 buildPointIndex 构建)
	pointIOA map[string]uint32

	// 历史
	history *HistoryBuffer
}

// buildPointIndex 扫描 store 所有点，建立 name→IOA 索引
// 同时为每个设备创建 dev.ID 别名字段（内部引擎用dev.ID, 外部展示用EGC中文名）
func (e *Engine) buildPointIndex() {
	e.pointIOA = make(map[string]uint32)
	if e.store == nil { return }

	// Always scan store for all point names
	for _, p := range e.store.GetAll() {
		e.pointIOA[p.Name] = p.IOA
	}

	// Try loading from persisted point table for additional entries
	if e.cfg.PointsJSON != "" {
		var entries []struct{ IOA uint32 `json:"ioa"`; Name string `json:"name"` }
		if json.Unmarshal([]byte(e.cfg.PointsJSON), &entries) == nil {
			for _, ent := range entries {
				e.pointIOA[ent.Name] = ent.IOA
			}
		}
	}

	// Add dev.ID aliases so engine can use internal names
	for idx, dev := range e.topology.Devices {
		prefix := typeChinese[dev.Type] + itoa(idx+1)
		suffixMap := internalSuffixes
		for cnSuffix, engSuffix := range suffixMap {
			cnName := prefix + cnSuffix
			if ioa, ok := e.pointIOA[cnName]; ok {
				e.pointIOA[dev.ID+engSuffix] = ioa
			}
		}
	}
}

// readPt 通过 IOA 精确读取测点值（O(1)）
func (e *Engine) readPt(name string) float64 {
	ioa, ok := e.pointIOA[name]
	if !ok {
		slog.Warn("readPt: name not found in pointIOA", "name", name)
		return 0
	}
	if e.store == nil {
		return 0
	}
	if p, found := e.store.Get(ioa); found {
		return p.Value
	}
	return 0
}

// writePt 通过 IOA 精确写入测点值（O(1)）
func (e *Engine) writePt(name string, value float64) {
	ioa, ok := e.pointIOA[name]
	if !ok {
		slog.Warn("writePt: name not found in pointIOA", "name", name)
		return
	}
	if e.store == nil {
		return
	}
	e.store.SetValue(ioa, value)
}

// NewEngine 创建微电网仿真引擎
func NewEngine(topology *Topology, store *library.Store, cfg InstanceConfig) *Engine {
	soc := make(map[string]float64)
	for _, dev := range topology.Devices {
		if dev.Type == CompBattery {
			soc[dev.ID] = dev.Params.InitSOC
		}
	}
	eng := &Engine{
		topology:  topology,
		store:     store,
		cfg:       cfg,
		soc:       soc,
		pvPower:   make(map[string]float64),
		loadPower: make(map[string]float64),
		batPower:  make(map[string]float64),
		history:   NewHistoryBuffer(3600),
	}
	eng.buildPointIndex()
	return eng
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

	// Rebuild point IOA index and auto-create GRID formula
	e.buildPointIndex()
	e.ensureGridFormula()

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

// ensureGridFormula 自动创建关口功率公式并添加到拓扑
func (e *Engine) ensureGridFormula() {
	if e.store == nil { return }
	// Build formula: GRID_P = (load + charger + battery(charge)) - (pv + battery(discharge))
	var loadTerms, genTerms []string
	for idx, dev := range e.topology.Devices {
		tc := typeChinese[dev.Type]
		if tc == "" { continue }
		prefix := tc + itoa(idx+1)
		switch dev.Type {
		case CompPV:
			genTerms = append(genTerms, "{"+prefix+"_有功功率}")
		case CompLoad:
			loadTerms = append(loadTerms, "{"+prefix+"_有功功率}")
		case CompCharger:
			loadTerms = append(loadTerms, "{"+prefix+"_充电功率}")
		case CompBattery:
			loadTerms = append(loadTerms, "{"+prefix+"_充放电功率}")
		}
	}
	if len(loadTerms) == 0 && len(genTerms) == 0 { return }

	expr := strings.Join(loadTerms, "+")
	if expr == "" { expr = "0" }
	genPart := strings.Join(genTerms, "+")
	if genPart == "" { genPart = "0" }
	expr = "(" + expr + ") - (" + genPart + ")"

	// Remove any old auto-generated GRID formula then create new one
	filtered := make([]FormulaRule, 0, len(e.topology.Formulas))
	for _, f := range e.topology.Formulas {
		if f.ID == "auto-grid" { continue }
		filtered = append(filtered, f)
	}
	e.topology.Formulas = append(filtered, FormulaRule{
		ID: "auto-grid", Name: "关口功率", Target: "关口表_有功功率", Expression: expr, Enabled: true,
	})
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

// ReloadTopology 从配置文件重新加载拓扑（线程安全）
func (e *Engine) ReloadTopology(topo *Topology) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Preserve existing SOC values before reload (device IDs may change)
	oldSOC := make(map[string]float64, len(e.soc))
	for k, v := range e.soc {
		oldSOC[k] = v
	}

	e.topology = topo
	e.syncStoreWithTopology() // 先补充 store 中缺失的测点
	e.buildPointIndex()       // 再重建索引（此时 store 已有所有点）
	e.ensureGridFormula()

	// 重建 SOC 映射：尽量保留既有值
	// 优先级：旧 ID 映射 > store 当前值（按中文名）> InitSOC
	newSOC := make(map[string]float64)
	for idx, dev := range topo.Devices {
		if dev.Type != CompBattery {
			continue
		}
		var socVal float64
		var found bool

		// 1) Same dev ID → preserve old value
		if s, ok := oldSOC[dev.ID]; ok {
			socVal = s
			found = true
		}

		// 2) Try reading current SOC from store by Chinese point name
		if !found && e.store != nil {
			prefix := typeChinese[dev.Type] + itoa(idx+1)
			for _, p := range e.store.GetAll() {
				if p.Name == prefix+"_电池SOC" {
					socVal = p.Value
					found = true
					break
				}
			}
		}

		// 3) Fall back to InitSOC
		if !found {
			socVal = dev.Params.InitSOC
			if socVal <= 0 {
				socVal = 50
			}
		}

		newSOC[dev.ID] = socVal
	}
	e.soc = newSOC
}
func (e *Engine) SetSwitch(devID string, closed bool) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range e.topology.Devices {
		if e.topology.Devices[i].ID == devID {
			e.topology.Devices[i].Switch.Closed = closed
			swVal := 0.0
			if closed { swVal = 1.0 }
			e.writePt(devID+"_SwStatus", swVal)
			e.writePt(devID+"_SwCtrl", swVal)
			e.writePt(devID+"_Status", swVal)
			if !closed { e.writePt(devID+"_Power", 0) }
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
		power := e.readPt(dev.ID + "_Power")
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

	gridPower := totalLoadKW + totalCharger + totalBat - totalPV
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

	// 1. Calculate PV power: AO setpoint only (no irradiance)
	for _, dev := range e.topology.Devices {
		if dev.Type != CompPV {
			continue
		}
		if !dev.Switch.Closed {
			e.pvPower[dev.ID] = 0
			continue
		}
		ratedP := dev.Params.RatedPowerKW
		if ratedP <= 0 { ratedP = 100 }
		// Remote: follow AO setpoint; Local: keep current value
		if dev.ControlMode != ModeLocal {
			setpoint := e.readPt(dev.ID + "_Setpoint")
			if setpoint > 0 {
				if setpoint > ratedP { setpoint = ratedP }
				e.pvPower[dev.ID] = setpoint
				continue
			}
		}
		// Keep current value (no random irradiance)
		e.pvPower[dev.ID] = e.readPt(dev.ID + "_Power")
	}

	// 2. Calculate load/charger power: keep current value from store
	for _, dev := range e.topology.Devices {
		switch dev.Type {
		case CompLoad:
			if dev.Switch.Closed {
				e.loadPower[dev.ID] = e.readPt(dev.ID + "_Power")
			} else {
				e.loadPower[dev.ID] = 0
			}
		case CompCharger:
			if dev.Switch.Closed {
				e.loadPower[dev.ID] = e.readPt(dev.ID + "_Power")
			} else {
				e.loadPower[dev.ID] = 0
			}
		}
	}

	// 3. Calculate battery power: AO setpoint only (no time dispatch)
	for _, dev := range e.topology.Devices {
		if dev.Type == CompBattery {
			if !dev.Switch.Closed {
				e.batPower[dev.ID] = 0
				continue
			}
			if dev.ControlMode != ModeLocal {
				setpoint := e.readPt(dev.ID + "_Setpoint")
				slog.Debug("battery setpoint", "dev", dev.ID, "setpoint", setpoint)
				if setpoint != 0 {
					ratedP := dev.Params.RatedPowerKW_B
					if ratedP <= 0 { ratedP = 50 }
					if setpoint > ratedP { setpoint = ratedP } else if setpoint < -ratedP { setpoint = -ratedP }
					e.batPower[dev.ID] = setpoint
					slog.Debug("battery power from setpoint", "dev", dev.ID, "power", setpoint)
					continue
				}
			}
			e.batPower[dev.ID] = e.readPt(dev.ID + "_Power")
			slog.Debug("battery power fallback", "dev", dev.ID, "power", e.batPower[dev.ID])
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
	setpoint := e.readPt(dev.ID + "_Setpoint")
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

// syncStoreLocked 将所有计算值同步到 store
func (e *Engine) syncStoreLocked() {
	if e.store == nil {
		return
	}
	for _, dev := range e.topology.Devices {
		switch dev.Type {
		case CompPV:
			e.writePt(dev.ID+"_Power", e.pvPower[dev.ID])
		case CompBattery:
			e.writePt(dev.ID+"_Power", e.batPower[dev.ID])
			slog.Debug("syncStore battery power", "dev", dev.ID, "power", e.batPower[dev.ID])
		case CompLoad:
			e.writePt(dev.ID+"_Power", e.loadPower[dev.ID])
		case CompCharger:
			e.writePt(dev.ID+"_Power", e.loadPower[dev.ID])
		}
		if s, ok := e.soc[dev.ID]; ok {
			e.writePt(dev.ID+"_SOC", s)
		}
		// Switch status: 1=closed, 0=open
		swVal := 0.0
		if dev.Switch.Closed { swVal = 1.0 }
		e.writePt(dev.ID+"_SwStatus", swVal)
		e.writePt(dev.ID+"_SwCtrl", swVal)
		e.writePt(dev.ID+"_Status", swVal)
	}
	// Grid values from latest power balance
	// Write to the actual IEC104 point names (关口表_*), not "GRID_P"
	e.writePt("关口表_有功功率", e.gridPower)
	e.writePt("关口表_无功功率", e.gridPower*0.15)
	e.writePt("关口表_运行状态", 1)
	if e.topology.GridMeter.IslandMode {
		e.writePt("关口表_运行状态", 0)
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
			return fmt.Sprintf("%f", e.readPt(name))
		})
		result, err := evaluateExpr(expr)
		if err != nil {
			slog.Warn("公式计算失败", "formula", f.Name, "expr", expr, "error", err)
			continue
		}
		e.writePt(f.Target, result)
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

// syncStoreWithTopology 确保 store 中有所有拓扑展开的测点
// 新增设备/自定义测点后补充缺失的 IOA 到 store，保证写入不会静默失败
func (e *Engine) syncStoreWithTopology() {
	if e.store == nil || e.topology == nil {
		return
	}
	expanded := e.topology.ExpandPoints()
	for _, p := range expanded {
		if _, found := e.store.Get(p.IOA); !found {
			if err := e.store.AddPoint(p); err != nil {
				slog.Warn("syncStoreWithTopology: AddPoint failed", "ioa", p.IOA, "name", p.Name, "error", err)
			} else {
				slog.Debug("syncStoreWithTopology: added missing point", "ioa", p.IOA, "name", p.Name)
			}
		}
	}
}

// ─── Store helpers ───

