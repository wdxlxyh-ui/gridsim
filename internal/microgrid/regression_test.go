package microgrid

import (
	"sort"
	"strings"
	"testing"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

// =============================================================================
// Regression Test Suite for Microgrid Engine
// =============================================================================
// Run: go test ./internal/microgrid/ -v -run "TestRegression"
//
// Covers:
//   1. Point Table Expansion (all device types, custom points, IOA ranges)
//   2. IOA Index Building (store, PointsJSON, dev.ID aliases)
//   3. Engine Simulation Tick (PV/battery/load/charger, SOC, power balance)
//   4. Formula Evaluation (parser, engine formulas, auto-grid)
//   5. Topology Reload (store sync, index rebuild, formula regen)
//   6. Dashboard (structure, values)
//   7. Switch Control (SetSwitch, power zero on open)
//   8. Edge Cases (nil store, missing names, control modes)
// =============================================================================

// ─── helpers ───

func regressionTopology() *Topology {
	return &Topology{
		BusName:      "10kV 母线",
		BusVoltageKV: 10,
		GridMeter:    GridMeterConfig{RatedCapacityKW: 500},
		Devices: []Device{
			{ID: "pv1", Type: CompPV, Name: "光伏1", IOABase: 101,
				Switch:  DeviceSwitch{ID: "s1", Name: "QF1", Closed: true, Controllable: true},
				Params:  DeviceParams{RatedPowerKW: 100},
				ControlMode: ModeRemote},
			{ID: "bat1", Type: CompBattery, Name: "储能1", IOABase: 151,
				Switch:  DeviceSwitch{ID: "s2", Name: "QF2", Closed: true, Controllable: true},
				Params:  DeviceParams{RatedPowerKW_B: 200, CapacityKWH: 100, InitSOC: 50, SOCMin: 10, SOCMax: 90},
				ControlMode: ModeRemote},
			{ID: "load1", Type: CompLoad, Name: "负荷1", IOABase: 201,
				Switch:  DeviceSwitch{ID: "s3", Name: "QF3", Closed: true},
				Params:  DeviceParams{LoadRatedKW: 50},
				ControlMode: ModeLocal},
			{ID: "ch1", Type: CompCharger, Name: "充电桩1", IOABase: 251,
				Switch:  DeviceSwitch{ID: "s4", Name: "QF4", Closed: true},
				Params:  DeviceParams{ChargerRatedKW: 30},
				ControlMode: ModeLocal},
		},
	}
}

func regressionStore(topo *Topology) *library.Store {
	return topo.StoreFromTopology()
}

func regressionEngine(topo *Topology, store *library.Store) *Engine {
	soc := make(map[string]float64)
	for _, dev := range topo.Devices {
		if dev.Type == CompBattery {
			soc[dev.ID] = dev.Params.InitSOC
			if soc[dev.ID] <= 0 {
				soc[dev.ID] = 50
			}
		}
	}
	return &Engine{
		topology:  topo,
		store:     store,
		cfg:       InstanceConfig{TickMs: 1000},
		soc:       soc,
		pvPower:   make(map[string]float64),
		loadPower: make(map[string]float64),
		batPower:  make(map[string]float64),
		history:   NewHistoryBuffer(3600),
	}
}

// =============================================================================
// 1. Point Table Expansion
// =============================================================================

func TestRegression_PointExpansion(t *testing.T) {
	topo := regressionTopology()
	pts := topo.ExpandPoints()

	// Grid meter: 6 points
	gridNames := []string{"关口表_有功功率", "关口表_无功功率", "关口表_电压", "关口表_频率", "关口表_运行状态", "关口表_孤岛状态"}
	for _, name := range gridNames {
		found := false
		for _, p := range pts {
			if p.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing grid meter point: %s", name)
		}
	}

	// PV: 6 points
	pvNames := []string{"光伏1_有功功率", "光伏1_日发电量", "光伏1_运行状态", "光伏1_开关状态", "光伏1_功率设定", "光伏1_远程启机"}
	for _, name := range pvNames {
		found := false
		for _, p := range pts {
			if p.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing PV point: %s", name)
		}
	}

	// Battery (idx=1): 6 points
	batNames := []string{"储能2_电池SOC", "储能2_充放电功率", "储能2_运行状态", "储能2_开关状态", "储能2_功率设定", "储能2_远程启机"}
	for _, name := range batNames {
		found := false
		for _, p := range pts {
			if p.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing Battery point: %s", name)
		}
	}

	// Load (idx=2): 5 points
	loadNames := []string{"负荷3_有功功率", "负荷3_运行状态", "负荷3_开关状态", "负荷3_功率设定", "负荷3_遥控分合"}
	for _, name := range loadNames {
		found := false
		for _, p := range pts {
			if p.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing Load point: %s", name)
		}
	}

	// Charger (idx=3): 5 points
	chNames := []string{"充电桩4_充电功率", "充电桩4_运行状态", "充电桩4_开关状态", "充电桩4_功率设定", "充电桩4_遥控分合"}
	for _, name := range chNames {
		found := false
		for _, p := range pts {
			if p.Name == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing Charger point: %s", name)
		}
	}
}

func TestRegression_PointTypes(t *testing.T) {
	topo := regressionTopology()
	pts := topo.ExpandPoints()

	type check struct {
		name  string
		ptype config.PointType
	}
	checks := []check{
		{"关口表_有功功率", config.TypeAI},
		{"关口表_无功功率", config.TypeAI},
		{"关口表_电压", config.TypeAI},
		{"关口表_频率", config.TypeAI},
		{"关口表_运行状态", config.TypeDI},
		{"关口表_孤岛状态", config.TypeDI},
		{"光伏1_有功功率", config.TypeAI},
		{"光伏1_日发电量", config.TypeAI},
		{"光伏1_运行状态", config.TypeDI},
		{"光伏1_开关状态", config.TypeDI},
		{"光伏1_功率设定", config.TypeAO},
		{"光伏1_远程启机", config.TypeDO},
		{"储能2_电池SOC", config.TypeAI},
		{"储能2_充放电功率", config.TypeAI},
		{"储能2_运行状态", config.TypeDI},
		{"储能2_开关状态", config.TypeDI},
		{"储能2_功率设定", config.TypeAO},
		{"储能2_远程启机", config.TypeDO},
		{"负荷3_有功功率", config.TypeAI},
		{"负荷3_运行状态", config.TypeDI},
		{"负荷3_开关状态", config.TypeDI},
		{"负荷3_功率设定", config.TypeAO},
		{"负荷3_遥控分合", config.TypeDO},
	}

	for _, c := range checks {
		found := false
		for _, p := range pts {
			if p.Name == c.name {
				found = true
				if p.PointType != c.ptype {
					t.Errorf("%s: type=%s, want %s", c.name, p.PointType, c.ptype)
				}
				break
			}
		}
		if !found {
			t.Errorf("point not found: %s", c.name)
		}
	}
}

func TestRegression_IOAUniqueness(t *testing.T) {
	topo := regressionTopology()
	pts := topo.ExpandPoints()

	ioaMap := make(map[uint32]bool)
	for _, p := range pts {
		if ioaMap[p.IOA] {
			t.Errorf("duplicate IOA: %d (%s)", p.IOA, p.Name)
		}
		ioaMap[p.IOA] = true
	}
}

func TestRegression_CustomPoints(t *testing.T) {
	devs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "ESS",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50},
			CustomPoints: []CustomPoint{
				{Name: "温度", Type: "AI", Alias: "Temperature"},
				{Name: "告警", Type: "DI"},
				{Name: "使能", Type: "DO"},
			},
		},
	}
	topo := &Topology{Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	pts := topo.ExpandPoints()

	ptMap := make(map[string]*config.Point)
	for _, p := range pts {
		ptMap[p.Name] = p
	}

	// Custom AI
	if p := ptMap["储能1_温度"]; p == nil {
		t.Error("missing custom AI point: 储能1_温度")
	} else if p.PointType != config.TypeAI {
		t.Errorf("custom AI type=%s, want AI", p.PointType)
	} else if p.Alias != "Temperature" {
		t.Errorf("custom AI alias=%s, want Temperature", p.Alias)
	}

	// Custom DI
	if p := ptMap["储能1_告警"]; p == nil {
		t.Error("missing custom DI point: 储能1_告警")
	} else if p.PointType != config.TypeDI {
		t.Errorf("custom DI type=%s, want DI", p.PointType)
	}

	// Custom DO
	if p := ptMap["储能1_使能"]; p == nil {
		t.Error("missing custom DO point: 储能1_使能")
	} else if p.PointType != config.TypeDO {
		t.Errorf("custom DO type=%s, want DO", p.PointType)
	}
}

func TestRegression_IOARanges(t *testing.T) {
	devs := []Device{
		{ID: "pv1", Type: CompPV, Name: "PV1", IOABase: 101, Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{RatedPowerKW: 100}},
		{ID: "bat1", Type: CompBattery, Name: "BAT1", IOABase: 201, Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50}},
		{ID: "load1", Type: CompLoad, Name: "L1", IOABase: 301, Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{LoadRatedKW: 50}},
	}
	topo := &Topology{Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	pts := topo.ExpandPoints()

	// Check IOA base ranges (compact layout, everything near IOABase)
	// PV (base=101): AI=101-102, DI=111-112, AO=121, DO=131
	// Battery (base=201): AI=201-202, DI=211-212, AO=221, DO=231
	// Load (base=301): AI=301, DI=311-312, AO=321, DO=331
	type ioaCheck struct {
		name string
		ioa  uint32
	}
	checks := []ioaCheck{
		{"光伏1_有功功率", 101}, {"光伏1_日发电量", 102}, {"光伏1_远程启机", 131},
		{"储能2_电池SOC", 201}, {"储能2_充放电功率", 202}, {"储能2_远程启机", 231},
		{"负荷3_有功功率", 301}, {"负荷3_遥控分合", 331},
	}
	for _, c := range checks {
		found := false
		for _, p := range pts {
			if p.Name == c.name {
				found = true
				if p.IOA != c.ioa {
					t.Errorf("%s: IOA=%d, want %d", c.name, p.IOA, c.ioa)
				}
				break
			}
		}
		if !found {
			t.Errorf("point not found: %s", c.name)
		}
	}

	// Test fallback for devices without IOABase (legacy migration)
	legacyDevs := []Device{
		{ID: "pv1", Type: CompPV, Name: "PV1", Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{RatedPowerKW: 100}},
	}
	legacyTopo := &Topology{Devices: legacyDevs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	legacyPts := legacyTopo.ExpandPoints()
	found := false
	for _, p := range legacyPts {
		if p.Name == "光伏1_有功功率" {
			found = true
			// fallback should use 101 + idx*50 = 101 + 0*50 = 101
			if p.IOA != 101 {
				t.Errorf("legacy fallback: IOA=%d, want 101", p.IOA)
			}
			break
		}
	}
	if !found {
		t.Error("legacy fallback: point not found")
	}
}

// =============================================================================
// 2. IOA Index Building
// =============================================================================

func TestRegression_IOAIndex_StoreScan(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// All Chinese names from store
	for _, p := range store.GetAll() {
		if _, ok := eng.pointIOA[p.Name]; !ok {
			t.Errorf("Chinese name %q (IOA=%d) missing from pointIOA", p.Name, p.IOA)
		}
	}
}

func TestRegression_IOAIndex_DevIDAliases(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// All device types must have correct aliases via internalSuffixes
	checks := []struct{ devID, alias string }{
		{"pv1", "Power"}, {"pv1", "DailyEnergy"}, {"pv1", "Status"}, {"pv1", "SwStatus"}, {"pv1", "Setpoint"}, {"pv1", "SwCtrl"},
		{"bat1", "Power"}, {"bat1", "SOC"}, {"bat1", "Status"}, {"bat1", "SwStatus"}, {"bat1", "Setpoint"}, {"bat1", "SwCtrl"},
		{"load1", "Power"}, {"load1", "Status"}, {"load1", "SwStatus"}, {"load1", "Setpoint"}, {"load1", "SwCtrl"},
		{"ch1", "Power"}, {"ch1", "Status"}, {"ch1", "SwStatus"}, {"ch1", "Setpoint"}, {"ch1", "SwCtrl"},
	}
	for _, c := range checks {
		key := c.devID + "_" + c.alias
		if _, ok := eng.pointIOA[key]; !ok {
			t.Errorf("alias missing: %s", key)
		}
	}
}

func TestRegression_IOAIndex_PointsJSON(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)

	// Simulate PointsJSON from persisted config
	pts := topo.ExpandPoints()
	var entries []struct{ IOA uint32 `json:"ioa"`; Name string `json:"name"` }
	for _, p := range pts {
		entries = append(entries, struct {
			IOA  uint32 `json:"ioa"`
			Name string `json:"name"`
		}{IOA: p.IOA, Name: p.Name})
	}
	eng.cfg.PointsJSON = `[{"ioa":9999,"name":"PointsJSON_Extra"}]`
	eng.buildPointIndex()

	// PointsJSON extra point must be in index
	if _, ok := eng.pointIOA["PointsJSON_Extra"]; !ok {
		t.Error("PointsJSON extra point not in index")
	}
	if ioa := eng.pointIOA["PointsJSON_Extra"]; ioa != 9999 {
		t.Errorf("PointsJSON extra IOA=%d, want 9999", ioa)
	}
}

func TestRegression_IOAIndex_NoDevNameAliases(t *testing.T) {
	devs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "SameName",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50}},
		{ID: "bat2", Type: CompBattery, Name: "SameName",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50}},
	}
	topo := &Topology{Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// dev.Name aliases must NOT exist (removed with old alias loop)
	if _, ok := eng.pointIOA["SameName_Power"]; ok {
		t.Error("SameName_Power (dev.Name based) should NOT exist")
	}
	if _, ok := eng.pointIOA["SameName_SOC"]; ok {
		t.Error("SameName_SOC (dev.Name based) should NOT exist")
	}
}

// =============================================================================
// 3. Engine Simulation Tick
// =============================================================================

func TestRegression_PVRemoteFollowsSetpoint(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// Set PV setpoint
	store.SetValue(eng.pointIOA["光伏1_功率设定"], 75.0)
	eng.tick()

	if eng.pvPower["pv1"] != 75.0 {
		t.Errorf("remote PV power = %f, want 75.0 (AO setpoint)", eng.pvPower["pv1"])
	}
}

func TestRegression_PVRemoteSetpointCapped(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// Set PV setpoint above rated power
	store.SetValue(eng.pointIOA["光伏1_功率设定"], 999.0)
	eng.tick()

	if eng.pvPower["pv1"] > 100.0 {
		t.Errorf("PV power capped at rated: got %f, want <=100", eng.pvPower["pv1"])
	}
}

func TestRegression_PVSwitchOpen_PowerZero(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	topo.Devices[0].Switch.Closed = false
	eng.tick()

	if eng.pvPower["pv1"] != 0.0 {
		t.Errorf("PV power with open switch = %f, want 0", eng.pvPower["pv1"])
	}
}

func TestRegression_PVLocalMode_IgnoresSetpoint(t *testing.T) {
	topo := regressionTopology()
	topo.Devices[0].ControlMode = ModeLocal
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// Set PV AO setpoint
	store.SetValue(eng.pointIOA["光伏1_功率设定"], 75.0)
	eng.tick()

	// Local mode: should NOT follow setpoint, keep current value (which is 0)
	if eng.pvPower["pv1"] == 75.0 {
		t.Error("local PV should NOT follow AO setpoint")
	}
}

func TestRegression_BatteryRemoteFollowsSetpoint(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// Battery setpoint: +50 (charge)
	store.SetValue(eng.pointIOA["储能2_功率设定"], 50.0)
	eng.tick()

	if eng.batPower["bat1"] != 50.0 {
		t.Errorf("remote battery power = %f, want 50.0", eng.batPower["bat1"])
	}

	// Battery setpoint: -30 (discharge)
	store.SetValue(eng.pointIOA["储能2_功率设定"], -30.0)
	eng.tick()

	if eng.batPower["bat1"] != -30.0 {
		t.Errorf("remote battery discharge = %f, want -30.0", eng.batPower["bat1"])
	}
}

func TestRegression_BatterySetpointCapped(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	store.SetValue(eng.pointIOA["储能2_功率设定"], 999.0)
	eng.tick()

	// Rated power for battery is 200
	if eng.batPower["bat1"] > 200.0 {
		t.Errorf("battery charge capped: got %f, want <=200", eng.batPower["bat1"])
	}

	store.SetValue(eng.pointIOA["储能2_功率设定"], -999.0)
	eng.tick()

	if eng.batPower["bat1"] < -200.0 {
		t.Errorf("battery discharge capped: got %f, want >=-200", eng.batPower["bat1"])
	}
}

func TestRegression_BatterySwitchOpen_PowerZero(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	topo.Devices[1].Switch.Closed = false
	store.SetValue(eng.pointIOA["储能2_功率设定"], 50.0)
	eng.tick()

	if eng.batPower["bat1"] != 0.0 {
		t.Errorf("battery power with open switch = %f, want 0", eng.batPower["bat1"])
	}
}

func TestRegression_LoadChargerPower(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// Set initial store values for load and charger
	store.SetValue(eng.pointIOA["负荷3_有功功率"], 30.0)
	store.SetValue(eng.pointIOA["充电桩4_充电功率"], 15.0)
	eng.tick()

	if eng.loadPower["load1"] != 30.0 {
		t.Errorf("load power = %f, want 30.0", eng.loadPower["load1"])
	}
	if eng.loadPower["ch1"] != 15.0 {
		t.Errorf("charger power = %f, want 15.0", eng.loadPower["ch1"])
	}
}

func TestRegression_LoadChargerSwitchOpen(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	store.SetValue(eng.pointIOA["负荷3_有功功率"], 30.0)
	store.SetValue(eng.pointIOA["充电桩4_充电功率"], 15.0)

	topo.Devices[2].Switch.Closed = false // load switch open
	topo.Devices[3].Switch.Closed = false // charger switch open
	eng.tick()

	if eng.loadPower["load1"] != 0.0 {
		t.Errorf("load switch open: power = %f, want 0", eng.loadPower["load1"])
	}
	if eng.loadPower["ch1"] != 0.0 {
		t.Errorf("charger switch open: power = %f, want 0", eng.loadPower["ch1"])
	}
}

func TestRegression_SOCUpdate(t *testing.T) {
	dev := Device{ID: "bat1", Type: CompBattery,
		Params: DeviceParams{CapacityKWH: 100, SOCMin: 10, SOCMax: 90}}
	eng := &Engine{
		cfg:     InstanceConfig{TickMs: 3600000}, // 1 hour tick
		soc:     map[string]float64{"bat1": 50},
		history: NewHistoryBuffer(3600),
	}

	// Charge at 20kW for 1 hour → SOC +20%
	eng.updateSOC(dev, 20)
	if eng.soc["bat1"] < 69.5 || eng.soc["bat1"] > 70.5 {
		t.Errorf("SOC after charge = %f, want ~70", eng.soc["bat1"])
	}

	// Discharge at -10kW for 1 hour → SOC -10%
	eng.soc["bat1"] = 50
	eng.updateSOC(dev, -10)
	if eng.soc["bat1"] < 39.5 || eng.soc["bat1"] > 40.5 {
		t.Errorf("SOC after discharge = %f, want ~40", eng.soc["bat1"])
	}
}

func TestRegression_SOCClamping(t *testing.T) {
	dev := Device{ID: "bat1", Type: CompBattery,
		Params: DeviceParams{CapacityKWH: 100, SOCMin: 10, SOCMax: 90}}
	eng := &Engine{
		cfg: InstanceConfig{TickMs: 3600000},
		soc: map[string]float64{"bat1": 50},
		history: NewHistoryBuffer(3600),
	}

	// Massive charge: would push SOC above max
	eng.updateSOC(dev, 500)
	if eng.soc["bat1"] > 90.0 {
		t.Errorf("SOC above max: %f, want <=90", eng.soc["bat1"])
	}

	eng.soc["bat1"] = 50
	eng.updateSOC(dev, -500)
	if eng.soc["bat1"] < 10.0 {
		t.Errorf("SOC below min: %f, want >=10", eng.soc["bat1"])
	}
}

func TestRegression_PowerBalance(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// PV=50, Bat=0 (no setpoint), Load=30, Charger=15
	store.SetValue(eng.pointIOA["光伏1_功率设定"], 50.0)
	store.SetValue(eng.pointIOA["负荷3_有功功率"], 30.0)
	store.SetValue(eng.pointIOA["充电桩4_充电功率"], 15.0)
	eng.tick()

	// Grid = (Load + Charger + Bat) - PV = (30 + 15 + 0) - 50 = -5
	// Negative grid = exporting to grid
	if eng.gridPower != -5.0 {
		t.Errorf("grid power = %f, want -5.0 (PV=50, Load=30, Charger=15)", eng.gridPower)
	}

	// Verify store was updated
	if v, _ := store.Get(eng.pointIOA["关口表_有功功率"]); v.Value != -5.0 {
		t.Errorf("store 关口表_有功功率 = %f, want -5.0", v.Value)
	}
}

func TestRegression_PowerBalance_IslandMode(t *testing.T) {
	topo := regressionTopology()
	topo.GridMeter.IslandMode = true
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	store.SetValue(eng.pointIOA["光伏1_功率设定"], 50.0)
	store.SetValue(eng.pointIOA["负荷3_有功功率"], 30.0)
	store.SetValue(eng.pointIOA["充电桩4_充电功率"], 15.0)
	eng.tick()

	// Island: grid should be 0
	if eng.gridPower != 0 {
		t.Errorf("island grid power = %f, want 0", eng.gridPower)
	}

	// Island mode: 关口表_运行状态 = 0
	if v, _ := store.Get(eng.pointIOA["关口表_运行状态"]); v.Value != 0 {
		t.Errorf("island 运行状态 = %f, want 0", v.Value)
	}
}

func TestRegression_SyncStore_PowerAndSwitch(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	store.SetValue(eng.pointIOA["光伏1_功率设定"], 60.0)
	eng.tick()

	// After tick, syncStoreLocked writes: device power, SOC, switch status, grid values
	// Check PV power written
	if v, _ := store.Get(eng.pointIOA["光伏1_有功功率"]); v.Value != 60.0 {
		t.Errorf("光伏1_有功功率 = %f, want 60.0", v.Value)
	}
	// Check switch status
	if v, _ := store.Get(eng.pointIOA["光伏1_开关状态"]); v.Value != 1.0 {
		t.Errorf("光伏1_开关状态 = %f, want 1.0 (closed)", v.Value)
	}
	if v, _ := store.Get(eng.pointIOA["光伏1_运行状态"]); v.Value != 1.0 {
		t.Errorf("光伏1_运行状态 = %f, want 1.0", v.Value)
	}
}

// =============================================================================
// 4. Formula Evaluation
// =============================================================================

func TestRegression_FormulaParser(t *testing.T) {
	cases := []struct{ expr string; want float64; fail bool }{
		{"0", 0, false},
		{"123", 123, false},
		{"-5", -5, false},
		{"+5", 5, false},
		{"2+3", 5, false},
		{"10-3", 7, false},
		{"4*5", 20, false},
		{"100/4", 25, false},
		{"2+3*4", 14, false},
		{"(2+3)*4", 20, false},
		{"10-3-2", 5, false},
		{"-5+10", 5, false},
		{"-(3+2)", -5, false},
		{"-(3+2)*-1", 5, false},
		{"", 0, true},
		{"5/0", 0, true},
		{"2++3", 5, false},
		{"abc", 0, true},
	}
	for _, c := range cases {
		v, err := evaluateExpr(c.expr)
		if c.fail {
			if err == nil {
				t.Errorf("%q: expected error", c.expr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", c.expr, err)
			continue
		}
		if v != c.want {
			t.Errorf("%q: got %f, want %f", c.expr, v, c.want)
		}
	}
}

func TestRegression_FormulaEvaluationInEngine(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// Add a user-defined formula: 关口表_有功功率 = 光伏1_有功功率 * 2
	topo.Formulas = []FormulaRule{
		{ID: "test-fml", Name: "双倍光伏", Target: "关口表_有功功率",
			Expression: "{光伏1_有功功率} * 2", Enabled: true},
	}

	// Set PV power via tick
	store.SetValue(eng.pointIOA["光伏1_功率设定"], 40.0)
	eng.tick()

	// After tick, syncStore writes actual PV=40 to 光伏1_有功功率
	// Then evaluateFormulasLocked computes: 40 * 2 = 80 → 关口表_有功功率
	if v, _ := store.Get(eng.pointIOA["关口表_有功功率"]); v.Value != 80.0 {
		t.Errorf("formula target 关口表_有功功率 = %f, want 80.0 (40*2)", v.Value)
	}
}

func TestRegression_FormulaDisabled(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// Disabled formula should not affect store
	topo.Formulas = []FormulaRule{
		{ID: "disabled", Name: "Disabled", Target: "关口表_有功功率",
			Expression: "{光伏1_有功功率} * 999", Enabled: false},
	}

	store.SetValue(eng.pointIOA["光伏1_功率设定"], 40.0)
	eng.tick()

	// Without formula, grid power comes from power balance
	v, _ := store.Get(eng.pointIOA["关口表_有功功率"])
	if v.Value == 40.0*999 {
		t.Errorf("disabled formula still executed: value=%f", v.Value)
	}
}

func TestRegression_AutoGridFormula(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	eng.ensureGridFormula()

	// auto-grid formula should be created
	found := false
	for _, f := range topo.Formulas {
		if f.ID == "auto-grid" {
			found = true
			if !f.Enabled {
				t.Error("auto-grid formula should be enabled")
			}
			if f.Target != "关口表_有功功率" {
				t.Errorf("auto-grid target=%s, want 关口表_有功功率", f.Target)
			}
			break
		}
	}
	if !found {
		t.Error("auto-grid formula not created")
	}

	// ensureGridFormula is idempotent: call again, only one auto-grid
	eng.ensureGridFormula()
	count := 0
	for _, f := range topo.Formulas {
		if f.ID == "auto-grid" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("auto-grid formula count = %d, want 1", count)
	}
}

// =============================================================================
// 5. Topology Reload
// =============================================================================

func TestRegression_ReloadTopology_UpdatesEverything(t *testing.T) {
	// Initial: 1 battery
	devs1 := []Device{
		{ID: "bat1", Type: CompBattery, Name: "储能1", IOABase: 101,
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100}},
	}
	topo1 := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs1, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := regressionStore(topo1)
	eng := regressionEngine(topo1, store)
	eng.buildPointIndex()

	// New topology: + PV device + custom point on bat1
	newDevs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "储能1", IOABase: 101,
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100},
			CustomPoints: []CustomPoint{{Name: "温度", Type: "AI"}},
		},
		{ID: "pv1", Type: CompPV, Name: "光伏1", IOABase: 151,
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{RatedPowerKW: 100}},
	}
	newTopo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: newDevs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}

	eng.ReloadTopology(newTopo)

	// Store must have new points
	// bat1 custom point at IOABase + 40 + 0*2 = 141
	if _, ok := store.Get(141); !ok {
		t.Error("custom point (IOA=141) not in store after ReloadTopology")
	}
	// pv1 battery SOC at IOABase + 0 = 151
	if _, ok := store.Get(151); !ok {
		t.Error("PV 光伏1_电池SOC (IOA=151) not in store after ReloadTopology")
	}

	// Index must have new aliases
	if _, ok := eng.pointIOA["pv1_Power"]; !ok {
		t.Error("pv1_Power alias missing after ReloadTopology")
	}

	// Custom point accessible by Chinese name
	if _, ok := eng.pointIOA["储能1_温度"]; !ok {
		t.Error("储能1_温度 missing after ReloadTopology")
	}

	// Can write/read via new device
	eng.writePt("pv1_Power", 55.0)
	if v := eng.readPt("pv1_Power"); v != 55.0 {
		t.Errorf("pv1_Power read=%f, want 55.0", v)
	}
}

// =============================================================================
// 6. Dashboard
// =============================================================================

func TestRegression_Dashboard_Structure(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	store.SetValue(eng.pointIOA["光伏1_功率设定"], 50.0)
	eng.tick()

	dash := eng.Dashboard()

	requiredKeys := []string{"grid_power_kw", "pv", "battery", "load", "charger",
		"total_pv_kw", "total_bat_kw", "total_load_kw", "total_charger_kw"}
	for _, key := range requiredKeys {
		if _, ok := dash[key]; !ok {
			t.Errorf("dashboard missing key: %s", key)
		}
	}
}

func TestRegression_Dashboard_Values(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	store.SetValue(eng.pointIOA["光伏1_功率设定"], 50.0)
	store.SetValue(eng.pointIOA["负荷3_有功功率"], 30.0)
	store.SetValue(eng.pointIOA["充电桩4_充电功率"], 10.0)
	eng.tick()

	dash := eng.Dashboard()

	// PV=50, Bat=0, Load=30, Charger=10
	// Grid = (30+10+0) - 50 = -10
	if v := dash["grid_power_kw"].(float64); v != -10.0 {
		t.Errorf("grid_power_kw = %f, want -10.0", v)
	}
	if v := dash["total_pv_kw"].(float64); v != 50.0 {
		t.Errorf("total_pv_kw = %f, want 50.0", v)
	}
}

func TestRegression_Dashboard_DeviceArrays(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	store.SetValue(eng.pointIOA["光伏1_功率设定"], 50.0)
	eng.tick()

	dash := eng.Dashboard()

	pvArr := dash["pv"].([]map[string]interface{})
	if len(pvArr) != 1 {
		t.Fatalf("pv array length = %d, want 1", len(pvArr))
	}
	if pvArr[0]["id"] != "pv1" {
		t.Errorf("pv[0].id = %v, want pv1", pvArr[0]["id"])
	}

	batArr := dash["battery"].([]map[string]interface{})
	if len(batArr) != 1 {
		t.Fatalf("battery array length = %d, want 1", len(batArr))
	}
	if _, ok := batArr[0]["soc"]; !ok {
		t.Error("battery[0] missing soc field")
	}
}

// =============================================================================
// 7. Switch Control
// =============================================================================

func TestRegression_SetSwitch(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// Turn OFF
	err := eng.SetSwitch("bat1", false)
	if err != nil {
		t.Fatalf("SetSwitch OFF: %v", err)
	}

	// Switch status should be 0
	if v := eng.readPt("bat1_SwStatus"); v != 0 {
		t.Errorf("SwStatus after OFF = %f, want 0", v)
	}
	if v := eng.readPt("bat1_SwCtrl"); v != 0 {
		t.Errorf("SwCtrl after OFF = %f, want 0", v)
	}

	// Turn ON
	err = eng.SetSwitch("bat1", true)
	if err != nil {
		t.Fatalf("SetSwitch ON: %v", err)
	}
	if v := eng.readPt("bat1_SwStatus"); v != 1 {
		t.Errorf("SwStatus after ON = %f, want 1", v)
	}
}

func TestRegression_SetSwitch_UnknownDevice(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	err := eng.SetSwitch("nonexistent", false)
	if err != nil {
		t.Errorf("SetSwitch unknown device returned error: %v", err)
	}
}

// =============================================================================
// 8. Edge Cases
// =============================================================================

func TestRegression_EmptyTopology(t *testing.T) {
	topo := &Topology{BusName: "test", GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	// tick must not panic with 0 devices
	eng.tick()

	// Dashboard must not panic
	dash := eng.Dashboard()
	if dash == nil {
		t.Error("dashboard is nil with empty topology")
	}
}

func TestRegression_NilStore(t *testing.T) {
	eng := &Engine{
		topology: regressionTopology(),
		history:  NewHistoryBuffer(3600),
		pointIOA: map[string]uint32{"test": 1},
	}

	// Must not panic
	eng.readPt("test")
	eng.writePt("test", 1.0)
	eng.buildPointIndex()
	eng.syncStoreWithTopology()
}

func TestRegression_NilTopology(t *testing.T) {
	eng := &Engine{history: NewHistoryBuffer(3600)}
	eng.syncStoreWithTopology() // must not panic
	eng.buildPointIndex()       // must not panic
}

func TestRegression_HistoryBuffer(t *testing.T) {
	hb := NewHistoryBuffer(3)

	// Empty
	if snaps := hb.Snapshots(); len(snaps) != 0 {
		t.Errorf("empty buffer: len=%d, want 0", len(snaps))
	}

	// Push 3
	hb.Push(SimSnapshot{Timestamp: 1, Values: map[string]float64{"a": 1}})
	hb.Push(SimSnapshot{Timestamp: 2, Values: map[string]float64{"b": 2}})
	hb.Push(SimSnapshot{Timestamp: 3, Values: map[string]float64{"c": 3}})

	snaps := hb.Snapshots()
	if len(snaps) != 3 {
		t.Fatalf("after 3 pushes: len=%d, want 3", len(snaps))
	}
	if snaps[0].Timestamp != 1 || snaps[2].Timestamp != 3 {
		t.Errorf("timestamps out of order: %v", snaps)
	}

	// Wrap around
	hb.Push(SimSnapshot{Timestamp: 4, Values: map[string]float64{"d": 4}})
	snaps = hb.Snapshots()
	if len(snaps) != 3 {
		t.Fatalf("after wrap: len=%d, want 3", len(snaps))
	}
	if snaps[0].Timestamp != 2 || snaps[2].Timestamp != 4 {
		t.Errorf("wrap timestamps: got %v, want [2,3,4]", timestampsOf(snaps))
	}

	// Clear
	hb.Clear()
	if snaps := hb.Snapshots(); len(snaps) != 0 {
		t.Errorf("after clear: len=%d, want 0", len(snaps))
	}
}

func timestampsOf(snaps []SimSnapshot) []int64 {
	r := make([]int64, len(snaps))
	for i, s := range snaps {
		r[i] = s.Timestamp
	}
	return r
}

// =============================================================================
// 9. Complete Topology Round-Trip
// =============================================================================

// TestRegression_CompleteTopologyRoundTrip exercises the full topology lifecycle:
// expand → store → index → tick → dashboard → reload → tick again
func TestRegression_CompleteTopologyRoundTrip(t *testing.T) {
	topo := regressionTopology()
	store := regressionStore(topo)
	eng := regressionEngine(topo, store)
	eng.buildPointIndex()

	t.Logf("Initial points: %d devices, %d store points", len(topo.Devices), store.TotalCount())

	// ── Phase 1: Tick with values ──
	store.SetValue(eng.pointIOA["光伏1_功率设定"], 60.0)
	store.SetValue(eng.pointIOA["负荷3_有功功率"], 25.0)
	store.SetValue(eng.pointIOA["充电桩4_充电功率"], 10.0)
	eng.tick()

	// Verify: grid = (25+10+0) - 60 = -25
	if eng.gridPower != -25.0 {
		t.Errorf("phase1 grid = %f, want -25.0", eng.gridPower)
	}

	// ── Phase 2: Reload — add 2nd battery ──
	newDevs := append(topo.Devices, Device{
		ID: "bat2", Type: CompBattery, Name: "储能2",
		Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{RatedPowerKW_B: 100, InitSOC: 50, CapacityKWH: 100},
	})
	newTopo := &Topology{BusName: topo.BusName, BusVoltageKV: topo.BusVoltageKV,
		Devices: newDevs, GridMeter: topo.GridMeter}
	eng.ReloadTopology(newTopo)

	// Store should have bat2 points
	if _, ok := eng.pointIOA["bat2_Power"]; !ok {
		t.Error("bat2_Power alias missing after reload")
	}

	// ── Phase 3: Tick with bat2 ──
	eng.writePt("bat2_Setpoint", 15.0)
	store.SetValue(eng.pointIOA["光伏1_功率设定"], 60.0)
	eng.tick()

	// bat2 charges at 15kW
	if eng.batPower["bat2"] != 15.0 {
		t.Errorf("bat2 power = %f, want 15.0", eng.batPower["bat2"])
	}

	// ── Phase 4: Dashboard ──
	dash := eng.Dashboard()
	if dash == nil {
		t.Fatal("dashboard is nil")
	}
	batArr := dash["battery"].([]map[string]interface{})
	if len(batArr) != 2 {
		t.Errorf("battery array = %d, want 2 (bat1 + bat2)", len(batArr))
	}

	// ── Phase 5: Turn off bat1 switch ──
	eng.SetSwitch("bat1", false)
	eng.tick()

	// bat1 should be 0 power (switch open)
	if eng.batPower["bat1"] != 0.0 {
		t.Errorf("bat1 power after switch OFF = %f, want 0", eng.batPower["bat1"])
	}
}

// TestRegression_StoreFromTopology verifies StoreFromTopology produces a valid store.
func TestRegression_StoreFromTopology(t *testing.T) {
	topo := regressionTopology()
	store := topo.StoreFromTopology()

	// Total expected: 6 grid + 6 pv + 6 battery + 5 load + 5 charger = 28
	expectedCount := 6 + 6 + 6 + 5 + 5
	if store.TotalCount() != expectedCount {
		t.Errorf("store total = %d, want %d", store.TotalCount(), expectedCount)
	}

	// Sort points for deterministic comparison
	pts := store.GetAll()
	sort.Slice(pts, func(i, j int) bool { return pts[i].IOA < pts[j].IOA })
	for i := 1; i < len(pts); i++ {
		if pts[i].IOA <= pts[i-1].IOA {
			t.Errorf("IOA not strictly increasing: %d <= %d at index %d", pts[i].IOA, pts[i-1].IOA, i)
		}
	}
}

func TestRegression_NewEngine_SOCInit(t *testing.T) {
	devs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "B1",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 65, RatedPowerKW_B: 100}},
		{ID: "bat2", Type: CompBattery, Name: "B2",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 35, RatedPowerKW_B: 100}},
		{ID: "pv1", Type: CompPV, Name: "P1",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{RatedPowerKW: 100}},
	}
	topo := &Topology{Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := regressionStore(topo)
	eng := NewEngine(topo, store, InstanceConfig{})

	if s := eng.soc["bat1"]; s != 65 {
		t.Errorf("bat1 SOC initial = %f, want 65", s)
	}
	if s := eng.soc["bat2"]; s != 35 {
		t.Errorf("bat2 SOC initial = %f, want 35", s)
	}
	if _, ok := eng.soc["pv1"]; ok {
		t.Error("pv1 should not have SOC entry")
	}
}

// TestRegression_FormatPointTable verifies the point table formatting.
func TestRegression_FormatPointTable(t *testing.T) {
	topo := regressionTopology()
	table := topo.FormatPointTable()

	if len(table) == 0 {
		t.Fatal("point table is empty")
	}
	for _, entry := range table {
		if _, ok := entry["ioa"]; !ok {
			t.Error("point table entry missing 'ioa'")
		}
		if _, ok := entry["name"]; !ok {
			t.Error("point table entry missing 'name'")
		}
		if _, ok := entry["type"]; !ok {
			t.Error("point table entry missing 'type'")
		}
		name := entry["name"].(string)
		if !strings.Contains(name, "关口表") && !strings.Contains(name, "光伏") &&
			!strings.Contains(name, "储能") && !strings.Contains(name, "负荷") &&
			!strings.Contains(name, "充电桩") {
			t.Errorf("unexpected point name: %s", name)
		}
	}
}
