package detail

import (
	"os"
	"testing"

	"gridsim/internal/model"
	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

// 注意：mockPublisher 在 csv_replay_test.go 中已定义，此处复用

// BenchmarkHandleAOFollow_Optimized 测试优化后的 AO Follow 性能
func BenchmarkHandleAOFollow_Optimized(b *testing.B) {
	// 创建测试 Store
	points := []*config.Point{
		{IOA: 100, Name: "AO_Test", PointType: config.TypeAO, ValueType: config.VTFloat, Value: 50.0},
	}
	for i := 1; i <= 100; i++ {
		points = append(points, &config.Point{
			IOA:       uint32(1000 + i),
			Name:      "AI_Follow",
			PointType: config.TypeAI,
			ValueType: config.VTFloat,
			Value:     0.0,
		})
	}
	store := library.NewStore(points)

	// 创建 mock publisher
	pub := &mockPublisher{}

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "engine_test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建 AutoChangeStore
	acStore := NewAutoChangeStore(tmpDir)

	// 创建 Engine
	engine := NewEngine("test", store, pub, acStore, tmpDir, nil)

	// 创建 100 个 AO Follow 配置，全部关联到 AO 100
	for i := 1; i <= 100; i++ {
		cfg := &model.AutoChangeConfig{
			PointIOA: uint32(1000 + i),
			Strategy: model.StrategyAOFollow,
			Enabled:  true,
			Params: model.StrategyParams{
				FollowAOIOA: 100,
			},
		}
		if err := engine.StartOrUpdate(cfg); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.Run("Optimized", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			engine.HandleAOFollow(100)
		}
	})
}

// BenchmarkHandleAOFollow_Original 测试原始的线性扫描性能（对比用）
func BenchmarkHandleAOFollow_Original(b *testing.B) {
	// 创建测试 Store
	points := []*config.Point{
		{IOA: 100, Name: "AO_Test", PointType: config.TypeAO, ValueType: config.VTFloat, Value: 50.0},
	}
	for i := 1; i <= 100; i++ {
		points = append(points, &config.Point{
			IOA:       uint32(1000 + i),
			Name:      "AI_Follow",
			PointType: config.TypeAI,
			ValueType: config.VTFloat,
			Value:     0.0,
		})
	}
	store := library.NewStore(points)

	// 创建 mock publisher
	pub := &mockPublisher{}

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "engine_test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建 AutoChangeStore
	acStore := NewAutoChangeStore(tmpDir)

	// 创建 Engine
	engine := NewEngine("test", store, pub, acStore, tmpDir, nil)

	// 创建 100 个配置
	for i := 1; i <= 100; i++ {
		cfg := &model.AutoChangeConfig{
			PointIOA: uint32(1000 + i),
			Strategy: model.StrategyAOFollow,
			Enabled:  true,
			Params: model.StrategyParams{
				FollowAOIOA: 100,
			},
		}
		if err := engine.StartOrUpdate(cfg); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.Run("Original_Linear_Scan", func(b *testing.B) {
		// 模拟原始的线性扫描实现
		for i := 0; i < b.N; i++ {
			for _, cfg := range engine.acStore.All("test") {
				if cfg.Strategy == model.StrategyAOFollow && cfg.Enabled && cfg.Params.FollowAOIOA == 100 {
					store.SetValue(cfg.PointIOA, 50.0)
				}
			}
		}
	})
}

// BenchmarkHandleAOFollow_10Configs 测试 10 个关联点的性能
func BenchmarkHandleAOFollow_10Configs(b *testing.B) {
	benchmarkAOFollowN(b, 10)
}

// BenchmarkHandleAOFollow_50Configs 测试 50 个关联点的性能
func BenchmarkHandleAOFollow_50Configs(b *testing.B) {
	benchmarkAOFollowN(b, 50)
}

// BenchmarkHandleAOFollow_200Configs 测试 200 个关联点的性能
func BenchmarkHandleAOFollow_200Configs(b *testing.B) {
	benchmarkAOFollowN(b, 200)
}

func benchmarkAOFollowN(b *testing.B, n int) {
	// 创建测试 Store
	points := []*config.Point{
		{IOA: 100, Name: "AO_Test", PointType: config.TypeAO, ValueType: config.VTFloat, Value: 50.0},
	}
	for i := 1; i <= n; i++ {
		points = append(points, &config.Point{
			IOA:       uint32(1000 + i),
			Name:      "AI_Follow",
			PointType: config.TypeAI,
			ValueType: config.VTFloat,
			Value:     0.0,
		})
	}
	store := library.NewStore(points)

	// 创建 mock publisher
	pub := &mockPublisher{}

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "engine_test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建 AutoChangeStore
	acStore := NewAutoChangeStore(tmpDir)

	// 创建 Engine
	engine := NewEngine("test", store, pub, acStore, tmpDir, nil)

	// 创建 n 个 AO Follow 配置
	for i := 1; i <= n; i++ {
		cfg := &model.AutoChangeConfig{
			PointIOA: uint32(1000 + i),
			Strategy: model.StrategyAOFollow,
			Enabled:  true,
			Params: model.StrategyParams{
				FollowAOIOA: 100,
			},
		}
		if err := engine.StartOrUpdate(cfg); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.HandleAOFollow(100)
	}
}
