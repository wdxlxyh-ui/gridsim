package microgrid

import "testing"

func emsTopology() *Topology {
	closed := DeviceSwitch{Closed: true, Controllable: true}
	return &Topology{
		BusName: "10kV", BusVoltageKV: 10,
		GridMeter: GridMeterConfig{RatedCapacityKW: 5000},
		Devices: []Device{
			{ID: "w1", Type: CompWind, Name: "风机1", IOABase: 101, Switch: closed,
				Params: DeviceParams{RatedPowerKW: 200}, ControlMode: ModeRemote},
			{ID: "w2", Type: CompWind, Name: "风机2", IOABase: 151, Switch: closed,
				Params: DeviceParams{RatedPowerKW: 100}, ControlMode: ModeRemote},
			{ID: "pv1", Type: CompPV, Name: "光伏1", IOABase: 201, Switch: closed,
				Params: DeviceParams{RatedPowerKW: 100, AvailableMode: "limit"}, ControlMode: ModeRemote},
			{ID: "b1", Type: CompBattery, Name: "储能1", IOABase: 251, Switch: closed,
				Params: DeviceParams{RatedPowerKW_B: 100, CapacityKWH: 200, InitSOC: 50, SOCMin: 10, SOCMax: 90, SOCProtection: true},
				ControlMode: ModeRemote},
			{ID: "b2", Type: CompBattery, Name: "储能2", IOABase: 301, Switch: closed,
				Params: DeviceParams{RatedPowerKW_B: 100, CapacityKWH: 100, InitSOC: 50, SOCMin: 10, SOCMax: 90, SOCProtection: true},
				ControlMode: ModeRemote},
			{ID: "load1", Type: CompLoad, Name: "负荷1", IOABase: 351, Switch: closed,
				Params: DeviceParams{LoadRatedKW: 400}, ControlMode: ModeRemote},
			{ID: "ems-w", Type: CompEMS, Name: "风电EMS", IOABase: 401, Switch: closed,
				Params: DeviceParams{EMSKind: "wind"}, ControlMode: ModeRemote},
			{ID: "ems-p", Type: CompEMS, Name: "光伏EMS", IOABase: 451, Switch: closed,
				Params: DeviceParams{EMSKind: "pv"}, ControlMode: ModeRemote},
			{ID: "ems-s", Type: CompEMS, Name: "储能EMS", IOABase: 501, Switch: closed,
				Params: DeviceParams{EMSKind: "storage"}, ControlMode: ModeRemote},
		},
	}
}

func newEMSEngine() *Engine {
	eng := newDispatchEngine(emsTopology())
	eng.soc["b1"], eng.soc["b2"] = 50, 50
	return eng
}

// 测点名 = 前缀 + 后缀, 前缀由 devPrefix 按全局序号生成
func nm(topo *Topology, id, suffix string) string {
	for i, d := range topo.Devices {
		if d.ID == id {
			return devPrefix(d, i) + suffix
		}
	}
	return "?" + id
}

func TestEMS_PointNamesAndAliases(t *testing.T) {
	topo := emsTopology()
	if got := nm(topo, "ems-w", "_功率设定"); got != "风电EMS7_功率设定" {
		t.Errorf("wind EMS point name = %q", got)
	}
	if got := nm(topo, "ems-s", "_加权SOC"); got != "储能EMS9_加权SOC" {
		t.Errorf("storage EMS point name = %q", got)
	}
	pts := topo.ExpandPoints()
	seen := map[uint32]string{}
	for _, p := range pts {
		if prev, dup := seen[p.IOA]; dup {
			t.Fatalf("duplicate IOA %d: %s vs %s", p.IOA, prev, p.Name)
		}
		seen[p.IOA] = p.Name
	}
	for _, want := range []string{"风电EMS7_功率设定", "风电EMS7_有功功率", "光伏EMS8_远方控制", "储能EMS9_通讯状态", "储能EMS9_加权SOC"} {
		found := false
		for _, p := range pts {
			if p.Name == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing EMS point %s", want)
		}
	}
	for _, p := range pts { // 风/光 EMS 不应有 SOC 点
		if p.Name == "风电EMS7_加权SOC" {
			t.Error("wind EMS must not expose SOC")
		}
	}
	if err := topo.Validate(); err != nil {
		t.Errorf("validate: %v", err)
	}
	topo.Devices[6].Params.EMSKind = "diesel"
	if topo.Validate() == nil {
		t.Error("invalid EMS kind must fail validation")
	}
}

func TestEMS_WindSplitProportionalToAvailable(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.set(nm(topo, "w1", "_风速"), 12) // 理论 200
	eng.set(nm(topo, "w2", "_风速"), 12) // 理论 100
	eng.set(nm(topo, "ems-w", "_功率设定"), 150)
	eng.tick() // 第一个 tick 先刷新理论功率并下发
	eng.tick()

	if !near(eng.pvPower["w1"], 100, 0.01) || !near(eng.pvPower["w2"], 50, 0.01) {
		t.Errorf("split = %.2f / %.2f, want 100 / 50", eng.pvPower["w1"], eng.pvPower["w2"])
	}
	if got := eng.readPt(nm(topo, "ems-w", "_有功功率")); !near(got, 150, 0.01) {
		t.Errorf("EMS total power = %v, want 150", got)
	}
	if got := eng.readPt(nm(topo, "ems-w", "_可调上限")); !near(got, 300, 0.01) {
		t.Errorf("EMS upper limit = %v, want 300", got)
	}
}

func TestEMS_WindSetpointCappedByAvailable(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.set(nm(topo, "w1", "_风速"), 12)
	eng.set(nm(topo, "w2", "_风速"), 2) // 低于切入风速, 理论 0
	eng.set(nm(topo, "ems-w", "_功率设定"), 280)
	eng.tick()
	eng.tick()
	if !near(eng.pvPower["w1"], 200, 0.01) || eng.pvPower["w2"] != 0 {
		t.Errorf("power = %.2f / %.2f, want 200 / 0", eng.pvPower["w1"], eng.pvPower["w2"])
	}
}

func TestEMS_ZeroSetpointIsRealCommand(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.set(nm(topo, "ems-w", "_功率设定"), 300)
	eng.tick()
	eng.tick()
	if eng.pvPower["w1"] <= 0 {
		t.Fatal("setup: wind should be generating")
	}
	eng.set(nm(topo, "ems-w", "_功率设定"), 0)
	eng.tick()
	if eng.pvPower["w1"] != 0 || eng.pvPower["w2"] != 0 {
		t.Errorf("EMS setpoint 0 must curtail to 0, got %.2f / %.2f", eng.pvPower["w1"], eng.pvPower["w2"])
	}
}

func TestEMS_LocalModeDoesNotDispatch(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.set(nm(topo, "ems-w", "_远方控制"), 0)
	eng.set(nm(topo, "ems-w", "_功率设定"), 300)
	eng.set(nm(topo, "w1", "_功率设定"), 20) // 单元仍可被直接控制
	eng.tick()
	eng.tick()
	if !near(eng.pvPower["w1"], 20, 0.01) {
		t.Errorf("EMS local: unit follows its own setpoint, got %.2f want 20", eng.pvPower["w1"])
	}
	if got := eng.readPt(nm(topo, "ems-w", "_远方状态")); got != 0 {
		t.Errorf("remote status = %v, want 0", got)
	}
}

func TestEMS_CommLossHoldsLastCommand(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.set(nm(topo, "ems-w", "_功率设定"), 150)
	eng.tick()
	eng.tick()
	before := eng.pvPower["w1"]
	eng.set(nm(topo, "ems-w", "_通讯状态"), 0)
	eng.set(nm(topo, "ems-w", "_功率设定"), 0) // 通讯中断: 新指令不生效
	eng.tick()
	if eng.pvPower["w1"] != before {
		t.Errorf("comm loss must hold last command: %.2f -> %.2f", before, eng.pvPower["w1"])
	}
}

func TestEMS_PVLimitModeFollowsTheory(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.set(nm(topo, "pv1", "_理论功率"), 60)
	eng.set(nm(topo, "ems-p", "_功率设定"), 100)
	eng.tick()
	eng.tick()
	if !near(eng.pvPower["pv1"], 60, 0.01) {
		t.Errorf("PV via EMS = %.2f, want 60 (theory)", eng.pvPower["pv1"])
	}
}

func TestEMS_StorageSplitByHeadroom(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.soc["b1"] = 50 // 可充 (90-50)%*200 = 80 kWh
	eng.soc["b2"] = 80 // 可充 (90-80)%*100 = 10 kWh
	eng.set(nm(topo, "ems-s", "_功率设定"), 90)
	eng.tick()
	// 单 tick 1s, 能力受 100kW 额定限制: b1 cap=100, b2 cap=100 (SOC room 远大于 1s 电量) → 各 45
	if !near(eng.batPower["b1"], 45, 0.01) || !near(eng.batPower["b2"], 45, 0.01) {
		t.Errorf("charge split = %.2f / %.2f, want 45 / 45", eng.batPower["b1"], eng.batPower["b2"])
	}
	if got := eng.readPt(nm(topo, "ems-s", "_有功功率")); !near(got, 90, 0.01) {
		t.Errorf("EMS storage power = %v, want 90", got)
	}
	wantSOC := (eng.soc["b1"]*200 + eng.soc["b2"]*100) / 300
	if got := eng.readPt(nm(topo, "ems-s", "_加权SOC")); !near(got, wantSOC, 0.01) {
		t.Errorf("weighted SOC = %v, want %v", got, wantSOC)
	}
}

func TestEMS_StorageDischargeAndSOCLimit(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.soc["b1"] = 10 // 已到下限, 不能放
	eng.soc["b2"] = 50
	eng.set(nm(topo, "ems-s", "_功率设定"), -80)
	eng.tick()
	if eng.batPower["b1"] != 0 {
		t.Errorf("b1 at SOC min must not discharge, got %.2f", eng.batPower["b1"])
	}
	if !near(eng.batPower["b2"], -80, 0.01) {
		t.Errorf("b2 should carry the whole discharge: got %.2f want -80", eng.batPower["b2"])
	}
}

func TestEMS_StorageRequestBeyondCapacityIsCapped(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.set(nm(topo, "ems-s", "_功率设定"), 500) // 两台合计最多 200
	eng.tick()
	if !near(eng.batPower["b1"]+eng.batPower["b2"], 200, 0.01) {
		t.Errorf("total = %.2f, want 200", eng.batPower["b1"]+eng.batPower["b2"])
	}
}

func TestEMS_GridBalanceWithAllSources(t *testing.T) {
	eng := newEMSEngine()
	topo := eng.topology
	eng.set(nm(topo, "w1", "_风速"), 12)
	eng.set(nm(topo, "w2", "_风速"), 12)
	eng.set(nm(topo, "pv1", "_理论功率"), 100)
	eng.set(nm(topo, "ems-w", "_功率设定"), 300)
	eng.set(nm(topo, "ems-p", "_功率设定"), 100)
	eng.set(nm(topo, "ems-s", "_功率设定"), -100) // 放电
	eng.set(nm(topo, "load1", "_功率设定"), 400)
	eng.tick()
	eng.tick()
	// 负荷 400 - 风 300 - 光 100 - 储放 100 = -100
	if !near(eng.gridPower, -100, 0.11) {
		t.Errorf("grid power = %v, want -100", eng.gridPower)
	}
	d := eng.Dashboard()
	ems, ok := d["ems"].([]map[string]interface{})
	if !ok || len(ems) != 3 {
		t.Fatalf("dashboard ems = %v", d["ems"])
	}
	if d["grid_power_kw"].(float64) > -99.9 || d["grid_power_kw"].(float64) < -100.1 {
		t.Errorf("dashboard grid = %v", d["grid_power_kw"])
	}
}

func TestEMS_ExistingTopologiesUnaffected(t *testing.T) {
	topo := regressionTopology()
	for _, d := range topo.Devices {
		if d.Type == CompEMS {
			t.Fatal("legacy topology has no EMS")
		}
	}
	eng := newDispatchEngine(topo)
	eng.tick()
	if len(eng.emsCmd) != 0 {
		t.Error("no EMS → no EMS-driven units")
	}
}
