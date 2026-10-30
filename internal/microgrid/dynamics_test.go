package microgrid

import (
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func dispatchTopology() *Topology {
	closed := DeviceSwitch{Closed: true, Controllable: true}
	return &Topology{
		BusName:      "10kV",
		BusVoltageKV: 10,
		GridMeter:    GridMeterConfig{RatedCapacityKW: 5000},
		Devices: []Device{
			{ID: "wind1", Type: CompWind, Name: "风机1", IOABase: 101, Switch: closed,
				Params: DeviceParams{RatedPowerKW: 200}, ControlMode: ModeRemote},
			{ID: "pv1", Type: CompPV, Name: "光伏1", IOABase: 151, Switch: closed,
				Params: DeviceParams{RatedPowerKW: 100, AvailableMode: "limit"}, ControlMode: ModeRemote},
			{ID: "bat1", Type: CompBattery, Name: "储能1", IOABase: 201, Switch: closed,
				Params: DeviceParams{RatedPowerKW_B: 100, CapacityKWH: 100, InitSOC: 50, SOCMin: 10, SOCMax: 90, SOCProtection: true},
				ControlMode: ModeRemote},
			{ID: "load1", Type: CompLoad, Name: "负荷1", IOABase: 251, Switch: closed,
				Params: DeviceParams{LoadRatedKW: 300}, ControlMode: ModeRemote},
		},
	}
}

func newDispatchEngine(topo *Topology) *Engine {
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()
	return eng
}

func (e *Engine) set(name string, v float64) { e.store.SetValue(e.pointIOA[name], v) }

func TestRampToward(t *testing.T) {
	cases := []struct{ cur, target, up, down, dt, want float64 }{
		{0, 100, 0, 0, 1, 100},   // 不限速
		{0, 100, 10, 0, 1, 10},   // 只配 up, 两方向共用
		{50, 0, 10, 0, 1, 40},    // 下降方向共用 up
		{50, 0, 10, 25, 1, 25},   // 分别配置
		{95, 100, 10, 10, 1, 100}, // 不过冲
		{10, 10, 5, 5, 1, 10},
	}
	for i, c := range cases {
		if got := rampToward(c.cur, c.target, c.up, c.down, c.dt); !near(got, c.want, 1e-9) {
			t.Errorf("case %d: got %v want %v", i, got, c.want)
		}
	}
}

func TestWindTheoryPower(t *testing.T) {
	p := DeviceParams{RatedPowerKW: 200}
	cases := []struct{ speed, want float64 }{
		{2, 0}, {3, 0}, {12, 200}, {20, 200}, {25, 0}, {30, 0},
	}
	for _, c := range cases {
		if got := windTheoryPower(c.speed, p); !near(got, c.want, 1e-9) {
			t.Errorf("speed %v: got %v want %v", c.speed, got, c.want)
		}
	}
	mid := windTheoryPower(7.5, p)
	if mid <= 0 || mid >= 200 {
		t.Errorf("7.5 m/s should be partial power, got %v", mid)
	}
	if !(windTheoryPower(6, p) < windTheoryPower(9, p)) {
		t.Error("power must rise with wind speed between cut-in and rated")
	}
}

func TestWind_LimitedByTheoryPower(t *testing.T) {
	eng := newDispatchEngine(dispatchTopology())
	eng.set("风电1_风速", 7.5)
	theory := windTheoryPower(7.5, DeviceParams{RatedPowerKW: 200})

	eng.set("风电1_功率设定", 200)
	eng.tick()
	if !near(eng.pvPower["wind1"], theory, 0.01) {
		t.Errorf("setpoint above theory: power=%v want theory %v", eng.pvPower["wind1"], theory)
	}

	eng.set("风电1_功率设定", 30)
	eng.tick()
	if !near(eng.pvPower["wind1"], 30, 0.01) {
		t.Errorf("setpoint below theory: power=%v want 30", eng.pvPower["wind1"])
	}

	eng.set("风电1_风速", 1) // 低于切入风速
	eng.tick()
	if eng.pvPower["wind1"] != 0 {
		t.Errorf("below cut-in: power=%v want 0", eng.pvPower["wind1"])
	}
}

func TestPV_LimitMode_TheoryPower(t *testing.T) {
	eng := newDispatchEngine(dispatchTopology())
	eng.set("光伏2_理论功率", 60) // 辐照决定的理论功率
	eng.set("光伏2_功率设定", 100)
	eng.tick()
	if !near(eng.pvPower["pv1"], 60, 1e-9) {
		t.Errorf("PV limit mode: power=%v want 60", eng.pvPower["pv1"])
	}
	eng.set("光伏2_功率设定", 40)
	eng.tick()
	if !near(eng.pvPower["pv1"], 40, 1e-9) {
		t.Errorf("PV limit mode: power=%v want 40", eng.pvPower["pv1"])
	}
}

func TestPV_LegacyMode_Unchanged(t *testing.T) {
	topo := dispatchTopology()
	topo.Devices[1].Params.AvailableMode = ""
	eng := newDispatchEngine(topo)
	eng.set("光伏2_功率设定", 80)
	eng.tick()
	if eng.pvPower["pv1"] != 80 {
		t.Errorf("legacy PV: power=%v want 80", eng.pvPower["pv1"])
	}
	if _, ok := eng.pointIOA["光伏2_理论功率"]; ok {
		t.Error("legacy PV must not expose a theory-power point")
	}
}

func TestRamp_PVFollowsSetpointGradually(t *testing.T) {
	topo := dispatchTopology()
	topo.Devices[1].Params.AvailableMode = ""
	topo.Devices[1].Params.RampUpKWPerS = 10
	topo.Devices[1].Params.RampDownKWPerS = 20
	eng := newDispatchEngine(topo)
	eng.set("光伏2_功率设定", 100)

	want := []float64{10, 20, 30}
	for i, w := range want {
		eng.tick()
		if !near(eng.pvPower["pv1"], w, 1e-9) {
			t.Fatalf("tick %d: power=%v want %v", i+1, eng.pvPower["pv1"], w)
		}
	}
	eng.set("光伏2_功率设定", 0) // 远方下发 0: 设定值不是置数, 光伏停机并按下降率回落
	eng.tick()
	if !near(eng.pvPower["pv1"], 10, 1e-9) {
		t.Errorf("ramp down: power=%v want 10", eng.pvPower["pv1"])
	}
}

func TestDelay_CommandAppliesAfterDelay(t *testing.T) {
	topo := dispatchTopology()
	topo.Devices[3].Params.ResponseDelayMs = 3000 // 负荷 3 秒后响应
	eng := newDispatchEngine(topo)
	eng.set("负荷4_功率设定", 120)

	var got []float64
	for i := 0; i < 5; i++ {
		eng.tick()
		got = append(got, eng.loadPower["load1"])
	}
	if got[0] != 0 || got[1] != 0 || got[2] != 0 {
		t.Errorf("before delay expires power should stay 0, got %v", got)
	}
	if got[3] != 120 || got[4] != 120 {
		t.Errorf("after delay power should be 120, got %v", got)
	}
}

func TestManualSet_BypassesDelayAndRamp(t *testing.T) {
	topo := dispatchTopology()
	l := &topo.Devices[3].Params
	l.ResponseDelayMs = 5000
	l.RampUpKWPerS = 5
	eng := newDispatchEngine(topo)

	eng.set("负荷4_功率设定", 200)
	eng.tick() // 指令进入延迟队列
	eng.set("负荷4_有功功率", 77) // 置数
	eng.tick()
	if eng.loadPower["load1"] != 77 {
		t.Errorf("manual set must take effect immediately, power=%v want 77", eng.loadPower["load1"])
	}
	for i := 0; i < 8; i++ {
		eng.tick()
	}
	// 置数前入队的旧指令不能在延迟到期后把置数值冲掉
	if eng.loadPower["load1"] < 77 {
		t.Errorf("stale queued command must not override manual set, power=%v", eng.loadPower["load1"])
	}
}

func TestZeroSetpoint_KeepsManualSetSemantics(t *testing.T) {
	eng := newDispatchEngine(dispatchTopology())
	eng.set("储能3_功率设定", 50)
	eng.tick()
	if eng.batPower["bat1"] != 50 {
		t.Fatalf("setup: battery=%v want 50", eng.batPower["bat1"])
	}
	// 置数: setpoint 为 0, 直接把功率点写成 0 → 立即为 0
	eng.set("储能3_功率设定", 0)
	eng.set("储能3_充放电功率", 0)
	eng.tick()
	if eng.batPower["bat1"] != 0 {
		t.Errorf("manual set to 0: battery=%v want 0", eng.batPower["bat1"])
	}
}

func TestZeroIsCommand_OptIn(t *testing.T) {
	topo := dispatchTopology()
	topo.Devices[2].Params.ZeroIsCommand = true
	eng := newDispatchEngine(topo)
	eng.set("储能3_功率设定", 50)
	eng.tick()
	eng.set("储能3_功率设定", 0) // 不碰功率点: 下发 0 即真实 0 指令
	eng.tick()
	if eng.batPower["bat1"] != 0 {
		t.Errorf("ZeroIsCommand: battery=%v want 0", eng.batPower["bat1"])
	}
}

func TestBattery_SOCProtection_StopsChargeAtMax(t *testing.T) {
	topo := dispatchTopology()
	topo.Devices[2].Params.InitSOC = 89.9
	eng := newDispatchEngine(topo)
	eng.soc["bat1"] = 89.9
	eng.set("储能3_功率设定", 100)

	for i := 0; i < 600; i++ {
		eng.tick()
	}
	if eng.soc["bat1"] > 90.0001 {
		t.Errorf("SOC overshoot: %v", eng.soc["bat1"])
	}
	if !near(eng.batPower["bat1"], 0, 1e-6) {
		t.Errorf("at SOC max charge power must be 0, got %v", eng.batPower["bat1"])
	}
	if got := eng.readPt("储能3_最大可充功率"); !near(got, 0, 1e-6) {
		t.Errorf("max-charge point = %v, want 0", got)
	}
	if got := eng.readPt("储能3_最大可放功率"); got <= 0 {
		t.Errorf("max-discharge point = %v, want > 0", got)
	}
	// 并网点按受限后的真实功率平衡: 储能 0, 其余设备出力为 0, 负荷 0 → 0
	if !near(eng.gridPower, 0, 0.11) {
		t.Errorf("grid power = %v, want ~0", eng.gridPower)
	}
}

func TestBattery_SOCProtection_StopsDischargeAtMin(t *testing.T) {
	eng := newDispatchEngine(dispatchTopology())
	eng.soc["bat1"] = 10.05
	eng.set("储能3_功率设定", -100)
	for i := 0; i < 600; i++ {
		eng.tick()
	}
	if eng.soc["bat1"] < 9.9999 {
		t.Errorf("SOC undershoot: %v", eng.soc["bat1"])
	}
	if !near(eng.batPower["bat1"], 0, 1e-6) {
		t.Errorf("at SOC min discharge power must be 0, got %v", eng.batPower["bat1"])
	}
}

func TestBattery_WithoutProtection_Unchanged(t *testing.T) {
	topo := dispatchTopology()
	topo.Devices[2].Params.SOCProtection = false
	eng := newDispatchEngine(topo)
	eng.soc["bat1"] = 90
	eng.set("储能3_功率设定", 100)
	eng.tick()
	if eng.batPower["bat1"] != 100 {
		t.Errorf("legacy battery keeps commanded power, got %v", eng.batPower["bat1"])
	}
	if _, ok := eng.pointIOA["储能3_最大可充功率"]; ok {
		t.Error("legacy battery must not expose protection points")
	}
}

func TestGridBalance_WindPVBatteryLoad(t *testing.T) {
	eng := newDispatchEngine(dispatchTopology())
	eng.set("风电1_风速", 12)
	eng.set("风电1_功率设定", 150)
	eng.set("光伏2_理论功率", 100)
	eng.set("光伏2_功率设定", 80)
	eng.set("储能3_功率设定", -40) // 放电
	eng.set("负荷4_功率设定", 200)
	eng.tick()

	// 负荷 200 + 储能充放 (-40) - 风 150 - 光 80 = -70 (向电网送电)
	if !near(eng.gridPower, -70, 0.11) {
		t.Errorf("grid power = %v, want -70", eng.gridPower)
	}
	if got := eng.readPt("关口表_有功功率"); !near(got, -70, 0.11) {
		t.Errorf("grid meter point = %v, want -70", got)
	}
	d := eng.Dashboard()
	if d["total_wind_kw"].(float64) != 150 {
		t.Errorf("dashboard total_wind_kw = %v, want 150", d["total_wind_kw"])
	}
	if !near(d["grid_power_kw"].(float64), -70, 0.11) {
		t.Errorf("dashboard grid_power_kw = %v, want -70", d["grid_power_kw"])
	}
}

func TestWindTopology_Validate_And_IOAUnique(t *testing.T) {
	topo := dispatchTopology()
	if err := topo.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if err := validateIOAUnique(topo); err != nil {
		t.Fatalf("ioa unique: %v", err)
	}
	topo.Devices[0].Params.RatedPowerKW = 0
	if topo.Validate() == nil {
		t.Error("wind without rated power must fail validation")
	}
}
