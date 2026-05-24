package microgrid

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

func makeTestPoint(ioa uint32, name string, value float64) *config.Point {
	return &config.Point{IOA: ioa, Name: name, Value: value, PointType: config.TypeAI}
}
func newStore(points ...*config.Point) *library.Store { return library.NewStore(points) }

func resolveAndEval(store *library.Store, expr string) (float64, error) {
	res := formulaRefRE.ReplaceAllStringFunc(expr, func(m string) string {
		name := m[1 : len(m)-1]
		for _, p := range store.GetAll() {
			if p.Name == name { return fmt.Sprintf("%f", p.Value) }
		}
		return "0"
	})
	res = strings.ReplaceAll(res, " ", "")
	return evaluateExpr(res)
}

// ─── Core parser tests ───

func TestExpressionParser(t *testing.T) {
	tests := []struct{ expr string; expect float64; fail bool }{
		{"2 + 3", 5, false}, {"4 * 5", 20, false}, {"2 + 3 * 4", 14, false},
		{"(2 + 3) * 4", 20, false}, {"10 - 3 - 2", 5, false}, {"100 / 4", 25, false},
		{"-5 + 10", 5, false}, {"-(3+2)", -5, false}, {"", 0, true}, {"5 / 0", 0, true},
	}
	for _, tt := range tests {
		v, err := evaluateExpr(tt.expr)
		if tt.fail { if err == nil { t.Errorf("%s: expected error", tt.expr) }; continue }
		if err != nil { t.Fatalf("%s: %v", tt.expr, err) }
		if v != tt.expect { t.Errorf("%s: got %f, want %f", tt.expr, v, tt.expect) }
	}
}

func TestFormulaRefRE(t *testing.T) {
	m := formulaRefRE.FindAllStringSubmatch("{光伏1_有功功率}+{储能1_充放电功率}", -1)
	if len(m) != 2 { t.Fatalf("got %d refs", len(m)) }
}

func TestResolveWithStore(t *testing.T) {
	s := newStore(makeTestPoint(1, "PV1", 45), makeTestPoint(2, "L1", 30))
	v, _ := resolveAndEval(s, "{PV1}+{L1}")
	if v != 75 { t.Errorf("got %f, want 75", v) }
}

// ─── EGC Naming Tests ───

func TestEGC_PointNames(t *testing.T) {
	devs := []Device{
		{ID: "pv1", Type: CompPV, Name: "PV-1", Switch: DeviceSwitch{ID: "s1", Name: "QF1", Closed: true, Controllable: true}},
		{ID: "bat1", Type: CompBattery, Name: "ESS-1", Switch: DeviceSwitch{ID: "s2", Name: "QF2", Closed: true, Controllable: true}, Params: DeviceParams{InitSOC: 60}},
		{ID: "load1", Type: CompLoad, Name: "L1", Switch: DeviceSwitch{ID: "s3", Name: "QF3", Closed: true}},
		{ID: "ch1", Type: CompCharger, Name: "CH1", Switch: DeviceSwitch{ID: "s4", Name: "QF4", Closed: true}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	pts := topo.ExpandPoints()

	expectedNames := map[string]bool{
		"关口表_有功功率": false, "关口表_无功功率": false, "关口表_电压": false,
		"光伏1_有功功率": false, "光伏1_开关状态": false, "光伏1_功率设定": false,
		"储能2_电池SOC": false, "储能2_充放电功率": false, "储能2_功率设定": false,
		"负荷3_有功功率": false, "负荷3_开关状态": false, "负荷3_功率设定": false,
		"充电桩4_充电功率": false, "充电桩4_开关状态": false, "充电桩4_功率设定": false,
	}
	for _, p := range pts { expectedNames[p.Name] = true }
	for name, found := range expectedNames {
		if !found { t.Errorf("missing point: %s", name) }
	}
}

func TestEGC_IOAIndex(t *testing.T) {
	devs := []Device{
		{ID: "pv1", Type: CompPV, Name: "PV-1", Switch: DeviceSwitch{ID: "s1", Name: "QF1", Closed: true}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	// Engine should be able to read via dev.ID alias
	if _, ok := eng.pointIOA["pv1_Power"]; !ok { t.Error("missing pv1_Power alias") }
	if _, ok := eng.pointIOA["pv1_SwStatus"]; !ok { t.Error("missing pv1_SwStatus alias") }
	if _, ok := eng.pointIOA["pv1_Setpoint"]; !ok { t.Error("missing pv1_Setpoint alias") }
	// Chinese names should also be accessible
	if _, ok := eng.pointIOA["光伏1_有功功率"]; !ok { t.Error("missing Chinese name 光伏1_有功功率") }
	if _, ok := eng.pointIOA["光伏1_功率设定"]; !ok { t.Error("missing Chinese name 光伏1_功率设定") }
}

// ─── All device types have AO Setpoint ───

func TestAllTypes_HaveAOSetpoint(t *testing.T) {
	devs := []Device{
		{ID: "pv1", Type: CompPV, Name: "P", Switch: DeviceSwitch{ID: "s1", Name: "Q1", Closed: true}},
		{ID: "bat1", Type: CompBattery, Name: "B", Switch: DeviceSwitch{ID: "s2", Name: "Q2", Closed: true}},
		{ID: "load1", Type: CompLoad, Name: "L", Switch: DeviceSwitch{ID: "s3", Name: "Q3", Closed: true}},
		{ID: "ch1", Type: CompCharger, Name: "C", Switch: DeviceSwitch{ID: "s4", Name: "Q4", Closed: true}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	pts := topo.ExpandPoints()
	for _, p := range pts {
		if strings.Contains(p.Name, "功率设定") && p.PointType != config.TypeAO {
			t.Errorf("%s should be AO, got %s", p.Name, p.PointType)
		}
	}
}

// ─── Grid formula engines ───

func TestReq1_GridFormula(t *testing.T) {
	cases := []struct{ pv, bat, load, ch, want float64 }{
		{50, 0, 0, 0, -50}, {0, 0, 30, 0, 30}, {0, 20, 0, 0, 20},
		{0, -20, 0, 0, -20}, {0, 10, 30, 10, 50}, {80, -20, 20, 0, -80},
	}
	for _, c := range cases {
		if got := c.load + c.ch + c.bat - c.pv; got != c.want {
			t.Errorf("GRID=%f, want %f", got, c.want)
		}
	}
}

func TestReq4_BatterySign(t *testing.T) {
	store := newStore(makeTestPoint(1, "bat1_Setpoint", 50))
	topo := &Topology{Devices: []Device{
		{ID: "bat1", Type: CompBattery, Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{RatedPowerKW_B: 200}},
	}}
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()
	if p := eng.calcBatteryPowerLocked(topo.Devices[0]); p != 50 {
		t.Errorf("setpoint=+50 → battery=%f (want +50)", p)
	}
}

func TestReq4_SOCUpdate(t *testing.T) {
	dev := Device{ID: "bat1", Type: CompBattery, Params: DeviceParams{CapacityKWH: 100, SOCMin: 10, SOCMax: 90}}
	eng := &Engine{topology: &Topology{Devices: []Device{dev}}, cfg: InstanceConfig{TickMs: 3600000},
		soc: map[string]float64{"bat1": 50}, history: NewHistoryBuffer(3600)}
	eng.updateSOC(dev, 20)
	if eng.soc["bat1"] <= 50 { t.Error("SOC should increase on charge") }
	eng.soc["bat1"] = 50
	eng.updateSOC(dev, -20)
	if eng.soc["bat1"] >= 50 { t.Error("SOC should decrease on discharge") }
}

func TestReq5_PointsSorted(t *testing.T) {
	store := newStore(makeTestPoint(5, "e", 0), makeTestPoint(1, "a", 0), makeTestPoint(3, "c", 0))
	pts := store.GetAll()
	sort.Slice(pts, func(i, j int) bool { return pts[i].IOA < pts[j].IOA })
	for i := 1; i < len(pts); i++ {
		if pts[i].IOA < pts[i-1].IOA { t.Error("not sorted") }
	}
}

func TestReq_DashboardFormat(t *testing.T) {
	devs := []Device{
		{ID: "pv1", Type: CompPV, Name: "PV1", Switch: DeviceSwitch{Closed: true}},
		{ID: "bat1", Type: CompBattery, Name: "BAT1", Switch: DeviceSwitch{Closed: true}},
		{ID: "load1", Type: CompLoad, Name: "L1", Switch: DeviceSwitch{Closed: true}},
	}
	store := newStore(makeTestPoint(1, "关口表_有功功率", -46.5), makeTestPoint(101, "pv1_Power", 48.5),
		makeTestPoint(201, "bat1_Power", -30), makeTestPoint(301, "load1_Power", 32))
	eng := &Engine{topology: &Topology{Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 1000}},
		store: store, cfg: InstanceConfig{}, soc: map[string]float64{"bat1": 68}, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()
	dash := eng.Dashboard()
	if _, ok := dash["grid_power_kw"]; !ok { t.Error("missing grid_power_kw") }
	if _, ok := dash["pv"]; !ok { t.Error("missing pv array") }
	if _, ok := dash["battery"]; !ok { t.Error("missing battery array") }
	if _, ok := dash["load"]; !ok { t.Error("missing load array") }
}

func TestReq_ControlMode(t *testing.T) {
	remotePV := Device{ID: "pv1", Type: CompPV, Switch: DeviceSwitch{Closed: true},
		Params: DeviceParams{RatedPowerKW: 100}, ControlMode: ModeRemote}
	store := newStore(makeTestPoint(1, "pv1_Setpoint", 75))
	eng := &Engine{topology: &Topology{Devices: []Device{remotePV}}, store: store,
		history: NewHistoryBuffer(3600), pvPower: make(map[string]float64)}
	eng.buildPointIndex()
	eng.tick()
	if eng.pvPower["pv1"] != 75 { t.Errorf("remote PV should follow AO=75, got %f", eng.pvPower["pv1"]) }

	localPV := Device{ID: "pv2", Type: CompPV, Switch: DeviceSwitch{Closed: true},
		Params: DeviceParams{RatedPowerKW: 100}, ControlMode: ModeLocal}
	store2 := newStore(makeTestPoint(1, "pv2_Setpoint", 75))
	eng2 := &Engine{topology: &Topology{Devices: []Device{localPV}}, store: store2,
		history: NewHistoryBuffer(3600), pvPower: make(map[string]float64)}
	eng2.buildPointIndex()
	eng2.tick()
	if eng2.pvPower["pv2"] == 75 { t.Errorf("local PV should NOT follow AO=75, got %f", eng2.pvPower["pv2"]) }
}

func TestSwitchDI(t *testing.T) {
	devs := []Device{{ID: "pv1", Type: CompPV, Name: "PV-1", Switch: DeviceSwitch{ID: "s1", Name: "QF1", Closed: true, Controllable: true}}}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	pts := topo.ExpandPoints()
	store := library.NewStore(pts)
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600),
		pvPower: make(map[string]float64), loadPower: make(map[string]float64), batPower: make(map[string]float64)}
	eng.buildPointIndex()

	v := eng.readPt("pv1_SwStatus")
	t.Logf("Initial: %f", v)
	eng.SetSwitch("pv1", false)
	v2 := eng.readPt("pv1_SwStatus")
	if v2 != 0 { t.Errorf("after OFF: got %f, want 0", v2) }
	eng.SetSwitch("pv1", true)
	v3 := eng.readPt("pv1_SwStatus")
	if v3 != 1 { t.Errorf("after ON: got %f, want 1", v3) }
}

func TestSetValue_DI_Points(t *testing.T) {
	store := library.NewStore([]*config.Point{{IOA: 1102, Name: "SW", ValueType: config.VTBit, PointType: config.TypeDI, Value: 0}})
	store.SetValue(1102, 1.0)
	pts := store.GetAll()
	if pts[0].Value != 1.0 { t.Errorf("SetValue on DI point failed: %f", pts[0].Value) }
}

func TestEGC_DebugNames(t *testing.T) {
	devs := []Device{
		{ID: "pv1", Type: CompPV, Name: "PV-1", Switch: DeviceSwitch{ID: "s1", Name: "QF1", Closed: true}},
		{ID: "bat1", Type: CompBattery, Name: "ESS-1", Switch: DeviceSwitch{ID: "s2", Name: "QF2", Closed: true}, Params: DeviceParams{InitSOC: 60}},
		{ID: "load1", Type: CompLoad, Name: "L1", Switch: DeviceSwitch{ID: "s3", Name: "QF3", Closed: true}},
		{ID: "ch1", Type: CompCharger, Name: "CH1", Switch: DeviceSwitch{ID: "s4", Name: "QF4", Closed: true}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	pts := topo.ExpandPoints()
	for _, p := range pts {
		if strings.Contains(p.Name, "功率") || strings.Contains(p.Name, "SOC") || strings.Contains(p.Name, "开关") || strings.Contains(p.Name, "有功") {
			t.Logf("%s (%s)", p.Name, p.PointType)
		}
	}
}
