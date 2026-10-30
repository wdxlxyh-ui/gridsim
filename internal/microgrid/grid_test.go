package microgrid

import (
	"testing"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

func gridTopology() *Topology {
	topo := dispatchTopology()
	topo.GridMeter = GridMeterConfig{RatedCapacityKW: 5000, ImportLimitKW: 100, ExportLimitKW: 60, BreakerPoints: true}
	return topo
}

// 风电/限功率光伏默认按可用功率发电, 需要看纯负荷场景时先把它们关掉
func quietGen(eng *Engine) {
	eng.set("风电1_功率设定", 0)
	eng.set("光伏2_功率设定", 0)
}

func TestGrid_BreakerPointsDefaultClosed(t *testing.T) {
	eng := newDispatchEngine(gridTopology())
	quietGen(eng)
	if eng.readPt("关口表_并网开关遥控") != 1 {
		t.Error("breaker control should default to closed (1)")
	}
	eng.set("负荷4_功率设定", 50)
	eng.tick()
	if eng.readPt("关口表_并网开关状态") != 1 || eng.readPt("关口表_孤岛状态") != 0 {
		t.Errorf("breaker=%v island=%v, want 1/0", eng.readPt("关口表_并网开关状态"), eng.readPt("关口表_孤岛状态"))
	}
	if !near(eng.gridPower, 50, 0.11) {
		t.Errorf("grid power = %v, want 50", eng.gridPower)
	}
}

func TestGrid_ImportExportLimits(t *testing.T) {
	eng := newDispatchEngine(gridTopology())
	quietGen(eng)
	eng.set("负荷4_功率设定", 250)
	eng.tick()
	if !near(eng.gridPower, 100, 0.11) {
		t.Errorf("import must clamp at 100, got %v", eng.gridPower)
	}
	eng.set("负荷4_功率设定", 0)
	eng.set("负荷4_有功功率", 0) // 置数
	eng.set("风电1_风速", 12)
	eng.set("风电1_功率设定", 200)
	eng.tick()
	if !near(eng.gridPower, -60, 0.11) {
		t.Errorf("export must clamp at -60, got %v", eng.gridPower)
	}
}

func TestGrid_BreakerOpenIslands(t *testing.T) {
	eng := newDispatchEngine(gridTopology())
	quietGen(eng)
	eng.set("负荷4_功率设定", 80)
	eng.set("关口表_并网开关遥控", 0)
	eng.tick()
	if eng.gridPower != 0 {
		t.Errorf("islanded grid power = %v, want 0", eng.gridPower)
	}
	if eng.readPt("关口表_孤岛状态") != 1 || eng.readPt("关口表_并网开关状态") != 0 {
		t.Errorf("island=%v breaker=%v, want 1/0", eng.readPt("关口表_孤岛状态"), eng.readPt("关口表_并网开关状态"))
	}
	eng.set("关口表_并网开关遥控", 1)
	eng.tick()
	if !near(eng.gridPower, 80, 0.11) {
		t.Errorf("reconnected grid power = %v, want 80", eng.gridPower)
	}
}

func TestGrid_LegacyStoreWithoutBreakerPointStaysGridConnected(t *testing.T) {
	topo := regressionTopology()
	eng := newDispatchEngine(topo)
	delete(eng.pointIOA, "关口表_并网开关遥控")
	if eng.islandLocked() {
		t.Error("missing breaker point must not island the microgrid")
	}
}

func TestGrid_ReloadAddsBreakerClosed(t *testing.T) {
	topo := regressionTopology()
	topo.GridMeter.BreakerPoints = true
	// 模拟旧实例: 点表里没有新增的并网开关点
	var old []*config.Point
	for _, p := range topo.ExpandPoints() {
		if p.IOA != 1003 && p.IOA != 1011 {
			old = append(old, p)
		}
	}
	eng := regressionEngine(topo, library.NewStore(old))
	eng.buildPointIndex()
	eng.ReloadTopology(topo)
	if eng.readPt("关口表_并网开关遥控") != 1 {
		t.Error("breaker control added on reload must default to closed")
	}
	eng.tick()
	if eng.islandLocked() {
		t.Error("reloaded legacy instance must stay grid-connected")
	}
}

func TestGrid_AutoIOAAvoidsGridBlock(t *testing.T) {
	var devs []Device
	for base := uint32(101); base < 1001; base += 50 {
		devs = append(devs, Device{IOABase: base})
	}
	if next := nextAvailableIOABase(devs); next < 1051 {
		t.Errorf("next base %d overlaps grid meter block 1001-1050", next)
	}
}

func TestGrid_BreakerPointsOffByDefault(t *testing.T) {
	for _, p := range regressionTopology().ExpandPoints() {
		if p.IOA == 1003 || p.IOA == 1011 {
			t.Errorf("breaker point %s must not exist unless enabled", p.Name)
		}
	}
}
