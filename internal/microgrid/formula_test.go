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

func newStore(points ...*config.Point) *library.Store {
	return library.NewStore(points)
}

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

// ─── Core tests ───

func TestExpressionParser(t *testing.T) {
	tests := []struct {
		expr   string
		expect float64
		fail   bool
	}{
		{"2 + 3", 5, false}, {"4 * 5", 20, false}, {"2 + 3 * 4", 14, false},
		{"(2 + 3) * 4", 20, false}, {"((1+2)*(3+4))", 21, false}, {"10 - 3 - 2", 5, false},
		{"100 / 4", 25, false}, {"-5 + 10", 5, false}, {"3.5 * 2", 7, false},
		{"-(3+2)", -5, false}, {"(10 + 2) * (8 - 3) / 2", 30, false},
		{"", 0, true}, {"5 / 0", 0, true}, {"(1 + 2", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			v, err := evaluateExpr(tt.expr)
			if tt.fail {
				if err == nil { t.Errorf("expected error, got %f", v) }
				return
			}
			if err != nil { t.Fatalf("unexpected: %v", err) }
			if v != tt.expect { t.Errorf("got %f, want %f", v, tt.expect) }
		})
	}
}

func TestFormulaRefRE(t *testing.T) {
	matches := formulaRefRE.FindAllStringSubmatch("{PV1_Power}+{Load1_Power}-{Battery1_Power}", -1)
	if len(matches) != 3 { t.Fatalf("got %d refs", len(matches)) }
}

func TestResolveWithStore(t *testing.T) {
	s := newStore(makeTestPoint(1, "PV1_Power", 45), makeTestPoint(2, "Load1_Power", 30), makeTestPoint(3, "Battery1_Power", -10))
	v, _ := resolveAndEval(s, "{Battery1_Power}+{Load1_Power}")
	if v != 20 { t.Errorf("got %f, want 20", v) }
}

func TestFormulaEvalOnStore(t *testing.T) {
	s := newStore(makeTestPoint(1, "PV1_Power", 50), makeTestPoint(2, "PV2_Power", 30),
		makeTestPoint(3, "Load1_Power", 40), makeTestPoint(4, "GRID_P", 0))
	v, _ := resolveAndEval(s, "{PV1_Power}+{PV2_Power}-{Load1_Power}")
	if v != 40 { t.Errorf("got %f, want 40", v) }
}

// ─── Requirement validation ───

func TestReq1_GridFormula(t *testing.T) {
	cases := []struct{ pv, bat, load, ch, want float64 }{
		{50, 0, 0, 0, -50}, {0, 0, 30, 0, 30}, {0, 20, 0, 0, 20}, {0, -20, 0, 0, -20},
		{0, 10, 30, 10, 50}, {80, -20, 20, 0, -80}, {48.5, -30, 32, 15.5, -31},
	}
	for _, c := range cases {
		got := c.load + c.ch + c.bat - c.pv
		if got != c.want { t.Errorf("GRID=%f+%f+%f-%f=%f, want %f", c.load, c.ch, c.bat, c.pv, got, c.want) }
	}
}

func TestReq4_BatterySign(t *testing.T) {
	// charge=+, discharge=- (remote mode = AO follow)
	topo := &Topology{Devices: []Device{
		{ID: "bat1", Type: CompBattery, Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{RatedPowerKW_B: 200}},
	}}
	store := newStore(makeTestPoint(1, "bat1_Setpoint", 50))
	eng := &Engine{topology: topo, store: store, cfg: InstanceConfig{TickMs: 1000}, history: NewHistoryBuffer(3600)}
	if p := eng.calcBatteryPowerLocked(topo.Devices[0]); p != 50 {
		t.Errorf("setpoint=+50 → battery=%f (want +50=charging)", p)
	}
	eng.store = newStore(makeTestPoint(1, "bat1_Setpoint", -80))
	if p := eng.calcBatteryPowerLocked(topo.Devices[0]); p != -80 {
		t.Errorf("setpoint=-80 → battery=%f (want -80=discharging)", p)
	}
}

func TestReq4_SOCUpdate(t *testing.T) {
	dev := Device{ID: "bat1", Type: CompBattery, Params: DeviceParams{CapacityKWH: 100, SOCMin: 10, SOCMax: 90, InitSOC: 50}}
	eng := &Engine{topology: &Topology{Devices: []Device{dev}}, cfg: InstanceConfig{TickMs: 3600000},
		soc: map[string]float64{"bat1": 50}, history: NewHistoryBuffer(3600)}

	eng.updateSOC(dev, 20)  // charge: SOC↑
	if eng.soc["bat1"] <= 50 { t.Error("SOC should increase on charge") }
	eng.soc["bat1"] = 50
	eng.updateSOC(dev, -20) // discharge: SOC↓
	if eng.soc["bat1"] >= 50 { t.Error("SOC should decrease on discharge") }
}

func TestReq5_PointsSorted(t *testing.T) {
	store := newStore(makeTestPoint(5, "E", 0), makeTestPoint(1, "A", 0),
		makeTestPoint(3, "C", 0), makeTestPoint(2, "B", 0), makeTestPoint(4, "D", 0))
	pts := store.GetAll()
	sort.Slice(pts, func(i, j int) bool { return pts[i].IOA < pts[j].IOA })
	for i := 1; i < len(pts); i++ {
		if pts[i].IOA < pts[i-1].IOA { t.Errorf("not sorted at %d", i) }
	}
}

func TestReq_DashboardFormat(t *testing.T) {
	devs := []Device{
		{ID: "pv1", Type: CompPV, Name: "PV1", Switch: DeviceSwitch{Closed: true}},
		{ID: "bat1", Type: CompBattery, Name: "BAT1", Switch: DeviceSwitch{Closed: true}},
		{ID: "load1", Type: CompLoad, Name: "L1", Switch: DeviceSwitch{Closed: true}},
	}
	store := newStore(makeTestPoint(1, "GRID_P", -46.5), makeTestPoint(101, "pv1_Power", 48.5),
		makeTestPoint(201, "bat1_Power", -30), makeTestPoint(301, "load1_Power", 32))
	eng := &Engine{topology: &Topology{Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 1000}},
		store: store, cfg: InstanceConfig{}, soc: map[string]float64{"bat1": 68},
		history: NewHistoryBuffer(3600)}
	dash := eng.Dashboard()
	if _, ok := dash["frequency_hz"]; ok { t.Error("dashboard should NOT have frequency_hz") }
	if _, ok := dash["grid_power_kw"]; !ok { t.Error("dashboard missing grid_power_kw") }
	if _, ok := dash["pv"]; !ok { t.Error("dashboard missing pv array") }
	if pvArr := dash["pv"].([]map[string]interface{}); len(pvArr) != 1 || pvArr[0]["power_kw"] != 48.5 {
		t.Error("PV power incorrect")
	}
}

func TestReq_ControlMode(t *testing.T) {
	// Remote mode: PV follows AO
	remotePV := Device{ID: "pv1", Type: CompPV, Switch: DeviceSwitch{Closed: true},
		Params: DeviceParams{RatedPowerKW: 100, Efficiency: 0.85}, ControlMode: ModeRemote}
	store := newStore(makeTestPoint(1, "pv1_Setpoint", 75))
	eng := &Engine{topology: &Topology{Devices: []Device{remotePV}}, store: store,
		history: NewHistoryBuffer(3600), pvPower: make(map[string]float64)}
	eng.tick()
	if eng.pvPower["pv1"] != 75 { t.Errorf("remote PV should follow AO=75, got %f", eng.pvPower["pv1"]) }

	// Local mode: PV ignores AO, uses irradiance
	localPV := Device{ID: "pv2", Type: CompPV, Switch: DeviceSwitch{Closed: true},
		Params: DeviceParams{RatedPowerKW: 100, Efficiency: 0.85}, ControlMode: ModeLocal}
	store2 := newStore(makeTestPoint(1, "pv2_Setpoint", 75))
	eng2 := &Engine{topology: &Topology{Devices: []Device{localPV}}, store: store2,
		history: NewHistoryBuffer(3600), pvPower: make(map[string]float64)}
	eng2.tick()
	if eng2.pvPower["pv2"] == 75 { t.Errorf("local PV should NOT follow AO=75, got %f", eng2.pvPower["pv2"]) }
}
