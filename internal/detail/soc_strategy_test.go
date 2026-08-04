package detail

import (
	"math"
	"testing"

	"gridsim/internal/model"
	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

func setupSOCEnv(t *testing.T, powerValue float64) (*strategyRunner, *model.AutoChangeConfig, *strategyState, *library.Store) {
	t.Helper()

	points := []*config.Point{
		{IOA: 16385, Name: "Power", ValueType: config.VTFloat, PointType: config.TypeAI, Value: powerValue, Efficient: 1, BaseValue: 0},
		{IOA: 16386, Name: "SOC", ValueType: config.VTFloat, PointType: config.TypeAI, Value: 50, Efficient: 1, BaseValue: 0},
	}
	store := library.NewStore(points)
	pub := &mockPublisher{}
	runner := newStrategyRunner(store, pub, t.TempDir(), "test-inst", nil)

	cfg := &model.AutoChangeConfig{
		PointIOA: 16386,
		Strategy: model.StrategySOC,
		Enabled:  true,
		Params: model.StrategyParams{
			InitSOC:    50,
			RatedCap:   1000,
			PowerIOA:   16385,
			IntegralMs: 1000,
			PeriodMs:   0,
		},
	}
	state := &strategyState{currentSOC: cfg.Params.InitSOC}

	return runner, cfg, state, store
}

func socValue(t *testing.T, store *library.Store) float64 {
	t.Helper()
	p, ok := store.Get(16386)
	if !ok {
		t.Fatal("SOC point not found")
	}
	return p.Value
}

// 物理基准：1000kW 对 1000kWh 电池充电 1 秒应增加 0.02778% SOC，
// delta = P × T(h) / Cap(kWh) × 100 = 1000 × (1/3600) / 1000 × 100
const (
	powerkW   = 1000.0
	ratedCap  = 1000.0
	perSecond = 1000.0 * (1.0 / 3600.0) / ratedCap * 100.0
)

func TestSOCChargePhysics(t *testing.T) {
	runner, cfg, state, store := setupSOCEnv(t, powerkW)

	for i := 0; i < 10; i++ {
		runner.doSOC(cfg, state)
	}

	got := socValue(t, store)
	want := 50 + perSecond
	if math.Abs(got-want) > 0.01 {
		t.Errorf("充电 1 秒后 SOC = %.4f%%，期望 ~%.4f%%（缓慢积分，不应瞬间满）", got, want)
	}
	if got >= 99.9 {
		t.Fatalf("SOC 被瞬间钳制到 100%%：复现了 bug（got %.2f%%）", got)
	}
}

func TestSOCDischargePhysics(t *testing.T) {
	runner, cfg, state, store := setupSOCEnv(t, -powerkW)

	for i := 0; i < 10; i++ {
		runner.doSOC(cfg, state)
	}

	got := socValue(t, store)
	want := 50 - perSecond
	if math.Abs(got-want) > 0.01 {
		t.Errorf("放电 1 秒后 SOC = %.4f%%，期望 ~%.4f%%", got, want)
	}
	if got <= 0.1 {
		t.Fatalf("SOC 被瞬间钳制到 0%%：复现了 bug（got %.2f%%）", got)
	}
}

func TestSOCRateIndependentOfPeriod(t *testing.T) {
	run := func(periodMs, ticks int) float64 {
		runner, cfg, state, store := setupSOCEnv(t, powerkW)
		cfg.Params.PeriodMs = periodMs
		for i := 0; i < ticks; i++ {
			runner.doSOC(cfg, state)
		}
		return socValue(t, store)
	}

	fast := run(100, 100)
	slow := run(1000, 10)

	if math.Abs(fast-slow) > 0.01 {
		t.Errorf("SOC 积分速率依赖 tick 周期：100ms×100 = %.4f%%，1000ms×10 = %.4f%%（应相同）", fast, slow)
	}
	want := 50 + perSecond*10
	if math.Abs(fast-want) > 0.01 {
		t.Errorf("10 秒充电 SOC = %.4f%%，期望 ~%.4f%%", fast, want)
	}
}

func TestSOCClampBounds(t *testing.T) {
	runner, cfg, state, store := setupSOCEnv(t, powerkW*1000)
	for i := 0; i < 100; i++ {
		runner.doSOC(cfg, state)
	}
	if got := socValue(t, store); math.Abs(got-100) > 0.01 {
		t.Errorf("超大功率充电后 SOC = %.2f%%，应钳制为 100%%", got)
	}

	runner2, cfg2, state2, store2 := setupSOCEnv(t, -powerkW*1000)
	for i := 0; i < 100; i++ {
		runner2.doSOC(cfg2, state2)
	}
	if got := socValue(t, store2); math.Abs(got-0) > 0.01 {
		t.Errorf("超大功率放电后 SOC = %.2f%%，应钳制为 0%%", got)
	}
}
