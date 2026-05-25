package microgrid

import (
	"testing"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

// ─── Fix 1: Old-style alias loop deletion ───

// TestCustomPointNamedPower_DoesNotHijackDevID verifies that a custom point named "Power"
// does NOT overwrite the dev.ID+"_Power" mapping that the engine uses internally.
func TestCustomPointNamedPower_DoesNotHijackDevID(t *testing.T) {
	devs := []Device{
		{
			ID: "bat1", Type: CompBattery, Name: "ESS-1",
			Switch: DeviceSwitch{ID: "s1", Name: "QF1", Closed: true},
			Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100},
			CustomPoints: []CustomPoint{{Name: "Power", Type: "AI"}},
		},
		{
			ID: "bat2", Type: CompBattery, Name: "ESS-2",
			Switch: DeviceSwitch{ID: "s2", Name: "QF2", Closed: true},
			Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100},
		},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	// bat1_Power must map to 储能1_充放电功率 (IOA 102), NOT the custom point (IOA 141)
	ioa, ok := eng.pointIOA["bat1_Power"]
	if !ok {
		t.Fatal("bat1_Power alias missing from pointIOA")
	}
	if ioa == 141 {
		t.Errorf("bat1_Power hijacked by custom point: IOA=141 (custom), want 102 (储能1_充放电功率)")
	}
	if ioa != 102 {
		t.Errorf("bat1_Power → IOA %d, want 102 (储能1_充放电功率)", ioa)
	}

	// bat2_Power should correctly map to 储能2_充放电功率 (IOA 152)
	ioa2, ok := eng.pointIOA["bat2_Power"]
	if !ok {
		t.Fatal("bat2_Power alias missing from pointIOA")
	}
	if ioa2 != 152 {
		t.Errorf("bat2_Power → IOA %d, want 152 (储能2_充放电功率)", ioa2)
	}
}

// TestCustomPointSOC_DoesNotHijackBatterySOC verifies that a custom point named "SOC"
// does NOT overwrite dev.ID+"_SOC" which should point to 储能X_电池SOC.
func TestCustomPointSOC_DoesNotHijackBatterySOC(t *testing.T) {
	devs := []Device{
		{
			ID: "bat1", Type: CompBattery, Name: "ESS-1",
			Switch: DeviceSwitch{ID: "s1", Name: "QF1", Closed: true},
			Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100},
			CustomPoints: []CustomPoint{{Name: "SOC", Type: "AI"}},
		},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	// bat1_SOC must map to 储能1_电池SOC (IOA 101), NOT the custom point (IOA 106)
	ioa, ok := eng.pointIOA["bat1_SOC"]
	if !ok {
		t.Fatal("bat1_SOC alias missing from pointIOA")
	}
	if ioa == 106 {
		t.Errorf("bat1_SOC hijacked by custom point: IOA=106 (custom), want 101 (储能1_电池SOC)")
	}
	if ioa != 101 {
		t.Errorf("bat1_SOC → IOA %d, want 101 (储能1_电池SOC)", ioa)
	}
}

// TestCustomPointNameCollision_AllSuffixes exhaustively checks that custom points
// named after ANY of the old alias loop suffixes (_Power/_SOC/_Setpoint/_SwStatus/_SwCtrl/_Status)
// cannot hijack the correct dev.ID+s mappings.
func TestCustomPointNameCollision_AllSuffixes(t *testing.T) {
	type check struct {
		suffix   string
		wantIOA  uint32
		chinese  string
	}
	checks := []check{
		// Compact layout near IOABase (default fallback 101):
		// AI 101(_电池SOC), 102(_充放电功率), DI 111(_运行状态), 112(_开关状态)
		// AO 121(_功率设定), DO 131(_远程启机)
		// Custom point at 101+40+0*2 = 141
		{"Power",    102, "储能1_充放电功率"},
		{"SOC",      101, "储能1_电池SOC"},
		{"Setpoint", 121, "储能1_功率设定"},
		{"SwStatus", 112, "储能1_开关状态"},
		{"SwCtrl",   131, "储能1_远程启机"},
		{"Status",   111, "储能1_运行状态"},
	}
	for _, c := range checks {
		t.Run(c.suffix, func(t *testing.T) {
			devs := []Device{
				{
					ID: "bat1", Type: CompBattery, Name: "ESS",
					Switch: DeviceSwitch{ID: "s1", Name: "QF1", Closed: true},
					Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100},
					CustomPoints: []CustomPoint{{Name: c.suffix, Type: "AI"}},
				},
			}
			topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
			store := topo.StoreFromTopology()
			eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
			eng.buildPointIndex()

			aliasKey := "bat1_" + c.suffix
			ioa, ok := eng.pointIOA[aliasKey]
			if !ok {
				t.Fatalf("alias %s missing — custom point may have overwritten it", aliasKey)
			}
			if ioa != c.wantIOA {
				t.Errorf("%s → IOA %d, want %d (%s). Custom point at IOA 141 should NOT hijack",
					aliasKey, ioa, c.wantIOA, c.chinese)
			}
		})
	}
}

// TestNoDevNameAliasesInPointIndex verifies that dev.Name-based aliases are NOT created
// (they were removed with the old-style alias loop). This prevents name collision issues
// when multiple devices share the same display name.
func TestNoDevNameAliasesInPointIndex(t *testing.T) {
	// Two devices with the SAME display name
	devs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "储能电池", Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50}},
		{ID: "bat2", Type: CompBattery, Name: "储能电池", Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	// dev.ID-based aliases must exist (unique)
	if _, ok := eng.pointIOA["bat1_Power"]; !ok {
		t.Error("bat1_Power missing — dev.ID alias should exist")
	}
	if _, ok := eng.pointIOA["bat2_Power"]; !ok {
		t.Error("bat2_Power missing — dev.ID alias should exist")
	}

	// dev.Name-based aliases must NOT exist (non-unique, removed in fix)
	if _, ok := eng.pointIOA["储能电池_Power"]; ok {
		t.Error("储能电池_Power (dev.Name based) should NOT exist with duplicate names")
	}
	if _, ok := eng.pointIOA["储能电池_SOC"]; ok {
		t.Error("储能电池_SOC (dev.Name based) should NOT exist with duplicate names")
	}
}

// TestMultiBatteryAliases_Isolated verifies aliases are correct when both batteries
// are identical (same name, same custom points) — each maps to its own IOA range.
func TestMultiBatteryAliases_Isolated(t *testing.T) {
	devs := []Device{
		{
			ID: "bat1", Type: CompBattery, Name: "储能A",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100},
		},
		{
			ID: "bat2", Type: CompBattery, Name: "储能B",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 60, RatedPowerKW_B: 100},
		},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	// bat1 → IOA range 101+
	if ioa := eng.pointIOA["bat1_Power"]; ioa != 102 {
		t.Errorf("bat1_Power → %d, want 102", ioa)
	}
	if ioa := eng.pointIOA["bat1_SOC"]; ioa != 101 {
		t.Errorf("bat1_SOC → %d, want 101", ioa)
	}

	// bat2 → IOA range 151+ (isolated from bat1)
	if ioa := eng.pointIOA["bat2_Power"]; ioa != 152 {
		t.Errorf("bat2_Power → %d, want 152", ioa)
	}
	if ioa := eng.pointIOA["bat2_SOC"]; ioa != 151 {
		t.Errorf("bat2_SOC → %d, want 151", ioa)
	}

	// No cross-device IOA aliases
	if _, ok := eng.pointIOA["bat1_Power"]; !ok { t.Error("bat1_Power missing") }
	if _, ok := eng.pointIOA["bat2_Power"]; !ok { t.Error("bat2_Power missing") }
	if _, ok := eng.pointIOA["储能A_Power"]; ok { t.Error("储能A_Power (dev.Name) should NOT exist") }
	if _, ok := eng.pointIOA["储能B_Power"]; ok { t.Error("储能B_Power (dev.Name) should NOT exist") }
}

// TestEngineReadWriteViaAlias_Isolated verifies writePt/readPt via dev.ID aliases
// target the correct IOA per device, with no cross-talk between batteries.
func TestEngineReadWriteViaAlias_Isolated(t *testing.T) {
	devs := []Device{
		{
			ID: "bat1", Type: CompBattery, Name: "ESS-1",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100},
		},
		{
			ID: "bat2", Type: CompBattery, Name: "ESS-2",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 60, RatedPowerKW_B: 100},
		},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	// Write different values to each battery
	eng.writePt("bat1_Power", 50.0)
	eng.writePt("bat2_Power", -30.0)

	// Read back — must be isolated
	if v := eng.readPt("bat1_Power"); v != 50.0 {
		t.Errorf("bat1_Power = %f, want 50.0 (cross-talk from bat2?)", v)
	}
	if v := eng.readPt("bat2_Power"); v != -30.0 {
		t.Errorf("bat2_Power = %f, want -30.0 (cross-talk from bat1?)", v)
	}
}

// ─── Fix 2: syncStoreWithTopology ───

// TestStoreAddPoint verifies the new Store.AddPoint method.
func TestStoreAddPoint(t *testing.T) {
	s := library.NewStore([]*config.Point{
		{IOA: 1, Name: "existing", PointType: config.TypeAI},
	})
	// Add new point
	pt := &config.Point{IOA: 2, Name: "new_point", PointType: config.TypeAI}
	if err := s.AddPoint(pt); err != nil {
		t.Fatalf("AddPoint failed: %v", err)
	}
	if p, ok := s.Get(2); !ok || p.Name != "new_point" {
		t.Errorf("point 2 not found or wrong name: %+v", p)
	}
	// Duplicate IOA must fail
	if err := s.AddPoint(&config.Point{IOA: 1, Name: "dup"}); err == nil {
		t.Error("AddPoint should fail for duplicate IOA")
	}
	// Added point should also appear in byType index
	aiPoints := s.GetByType(config.TypeAI)
	if len(aiPoints) != 2 {
		t.Errorf("expected 2 AI points, got %d", len(aiPoints))
	}
}

// TestSyncStoreWithTopology_AddsNewDevice points verifies that after adding
// a new device to the topology, syncStoreWithTopology adds its points to the store.
func TestSyncStoreWithTopology_AddsNewDevice(t *testing.T) {
	// Start with 1 battery
	devs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "ESS-1",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	initialCount := store.TotalCount()

	// Add a new device to topology (simulates user adding device via API)
	eng.topology.Devices = append(eng.topology.Devices, Device{
		ID: "bat2", Type: CompBattery, Name: "ESS-2",
		Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 60, RatedPowerKW_B: 100},
	})

	// Before sync, store should NOT have bat2 points
	beforeCount := store.TotalCount()
	if beforeCount != initialCount {
		t.Fatalf("store count changed before sync: %d → %d", initialCount, beforeCount)
	}

	// syncStoreWithTopology — must add bat2's 6 points
	eng.syncStoreWithTopology()

	afterCount := store.TotalCount()
	expectedNew := 6 // battery has 6 standard points: 2 AI + 2 DI + 1 AO + 1 DO
	if afterCount != initialCount+expectedNew {
		t.Errorf("store count after sync: %d, want %d (+%d new points)", afterCount, initialCount+expectedNew, expectedNew)
	}

	// Rebuild index and verify new aliases work
	eng.buildPointIndex()
	if _, ok := eng.pointIOA["bat2_Power"]; !ok {
		t.Error("bat2_Power alias still missing after syncStoreWithTopology")
	}

	// Write/read via new alias must work
	eng.writePt("bat2_Power", 25.0)
	if v := eng.readPt("bat2_Power"); v != 25.0 {
		t.Errorf("bat2_Power read after write = %f, want 25.0", v)
	}
}

// TestSyncStoreWithTopology_AddsCustomPoint verifies adding a custom point
// to an existing device correctly adds it to the store.
func TestSyncStoreWithTopology_AddsCustomPoint(t *testing.T) {
	devs := []Device{
		{
			ID: "bat1", Type: CompBattery, Name: "ESS-1",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100},
		},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	initialCount := store.TotalCount()

	// Add a custom point to the existing device
	eng.topology.Devices[0].CustomPoints = []CustomPoint{
		{Name: "Temperature", Type: "AI"},
	}

	// syncStoreWithTopology — must add the custom point
	eng.syncStoreWithTopology()

	afterCount := store.TotalCount()
	if afterCount != initialCount+1 {
		t.Errorf("store count after custom point sync: %d, want %d (+1)", afterCount, initialCount+1)
	}

	// Verify the custom point is in the store by name
	eng.buildPointIndex()
	customName := "储能1_Temperature"
	ioa, ok := eng.pointIOA[customName]
	if !ok {
		t.Fatalf("custom point %s not in pointIOA after sync", customName)
	}

	// Write to custom point via IOA and verify
	p, err := store.SetValue(ioa, 35.5)
	if err != nil {
		t.Fatalf("SetValue on custom point failed: %v", err)
	}
	if p.Value != 35.5 {
		t.Errorf("custom point value = %f, want 35.5", p.Value)
	}
}

// TestReloadTopology_SyncsStore verifies that ReloadTopology fully syncs the store
// with new points from a modified topology.
func TestReloadTopology_SyncsStore(t *testing.T) {
	// Initial topology: 1 battery
	devs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "ESS-1",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	// Build modified topology: add a device + custom point on bat1
	newDevs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "ESS-1",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100},
			CustomPoints: []CustomPoint{{Name: "Temperature", Type: "AI"}},
		},
		{ID: "pv1", Type: CompPV, Name: "PV-1",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{RatedPowerKW: 100}},
	}
	newTopo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: newDevs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}

	storeBefore := store.TotalCount()

	// ReloadTopology — must sync store AND rebuild index AND regenerate formula
	eng.ReloadTopology(newTopo)

	storeAfter := store.TotalCount()
	if storeAfter <= storeBefore {
		t.Errorf("store not updated by ReloadTopology: before=%d, after=%d", storeBefore, storeAfter)
	}

	// Custom points accessible by Chinese name (no dev.ID alias for custom points)
	if _, ok := eng.pointIOA["储能1_Temperature"]; !ok {
		t.Error("储能1_Temperature missing from pointIOA after ReloadTopology")
	}
	// Standard dev.ID aliases must work
	if _, ok := eng.pointIOA["pv1_Power"]; !ok {
		t.Error("pv1_Power alias missing after ReloadTopology (new device not synced)")
	}

	// Write/read new device
	eng.writePt("pv1_Power", 45.0)
	if v := eng.readPt("pv1_Power"); v != 45.0 {
		t.Errorf("pv1_Power = %f, want 45.0", v)
	}

	// Original device still works
	eng.writePt("bat1_Power", 10.0)
	if v := eng.readPt("bat1_Power"); v != 10.0 {
		t.Errorf("bat1_Power = %f, want 10.0", v)
	}

	// Write to custom point via Chinese name
	eng.writePt("储能1_Temperature", 36.0)
	if v := eng.readPt("储能1_Temperature"); v != 36.0 {
		t.Errorf("储能1_Temperature = %f, want 36.0", v)
	}
}

// TestSyncStoreWithTopology_Idempotent verifies calling syncStoreWithTopology
// multiple times is safe — it should not add duplicate points.
func TestSyncStoreWithTopology_Idempotent(t *testing.T) {
	devs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "ESS-1",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50, RatedPowerKW_B: 100}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}

	// First sync (all points already exist, should be no-op)
	eng.syncStoreWithTopology()
	count1 := store.TotalCount()

	// Second sync
	eng.syncStoreWithTopology()
	count2 := store.TotalCount()

	if count1 != count2 {
		t.Errorf("syncStoreWithTopology not idempotent: %d → %d", count1, count2)
	}
}

// TestSyncStoreWithTopology_NilStoreSafe verifies nil store doesn't panic.
func TestSyncStoreWithTopology_NilStoreSafe(t *testing.T) {
	eng := &Engine{topology: &Topology{Devices: []Device{
		{ID: "bat1", Type: CompBattery, Name: "ESS",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50}},
	}}}
	// Should not panic
	eng.syncStoreWithTopology()
}

// ─── Fix 3: readPt/writePt warn on missing name ───

// TestReadPt_MissingName returns 0 without error for unknown names.
// (The slog.Warn is added; this test verifies the function contract is unchanged.)
func TestReadPt_MissingName(t *testing.T) {
	devs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "ESS",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	// Known name — must return value (0 = initial)
	v := eng.readPt("bat1_Power")
	if v != 0 {
		t.Errorf("bat1_Power initial = %f, want 0", v)
	}

	// Unknown name — must return 0 (was: silently returned 0, now with Warn log)
	v = eng.readPt("nonexistent_point")
	if v != 0 {
		t.Errorf("nonexistent read = %f, want 0", v)
	}
}

// TestWritePt_MissingName does nothing without error for unknown names.
func TestWritePt_MissingName(t *testing.T) {
	devs := []Device{
		{ID: "bat1", Type: CompBattery, Name: "ESS",
			Switch: DeviceSwitch{Closed: true}, Params: DeviceParams{InitSOC: 50}},
	}
	topo := &Topology{BusName: "10kV", BusVoltageKV: 10, Devices: devs, GridMeter: GridMeterConfig{RatedCapacityKW: 500}}
	store := topo.StoreFromTopology()
	eng := &Engine{topology: topo, store: store, history: NewHistoryBuffer(3600)}
	eng.buildPointIndex()

	// Write to nonexistent point — must not panic or add to store
	eng.writePt("does_not_exist", 100.0)

	// Store should only have the original points (6 battery + 6 grid)
	if store.TotalCount() != 12 {
		t.Errorf("store count after write to unknown name = %d, want 12", store.TotalCount())
	}
}

// TestReadPtWritePt_NilStoreSafe verifies nil store handling.
func TestReadPtWritePt_NilStoreSafe(t *testing.T) {
	eng := &Engine{pointIOA: map[string]uint32{"test": 1}}
	// Must not panic
	eng.readPt("test")
	eng.writePt("test", 1.0)
}

// ─── Store.AddPoint ───

// TestStoreAddPoint_ByTypeIndex checks that AddPoint correctly updates the byType index.
func TestStoreAddPoint_ByTypeIndex(t *testing.T) {
	s := library.NewStore([]*config.Point{
		{IOA: 1, Name: "a", PointType: config.TypeAI},
	})
	if err := s.AddPoint(&config.Point{IOA: 2, Name: "b", PointType: config.TypeAI}); err != nil {
		t.Fatal(err)
	}
	if err := s.AddPoint(&config.Point{IOA: 3, Name: "c", PointType: config.TypeDI}); err != nil {
		t.Fatal(err)
	}

	aiPoints := s.GetByType(config.TypeAI)
	if len(aiPoints) != 2 {
		t.Errorf("AI count = %d, want 2", len(aiPoints))
	}
	diPoints := s.GetByType(config.TypeDI)
	if len(diPoints) != 1 {
		t.Errorf("DI count = %d, want 1", len(diPoints))
	}
}

// TestStoreAddPoint_ThreadSafe just verifies basic concurrent safety works.
func TestStoreAddPoint_Duplicate(t *testing.T) {
	s := library.NewStore([]*config.Point{
		{IOA: 1, Name: "original", PointType: config.TypeAI},
	})
	err := s.AddPoint(&config.Point{IOA: 1, Name: "duplicate", PointType: config.TypeAI})
	if err == nil {
		t.Error("expected error for duplicate IOA 1")
	}
}
