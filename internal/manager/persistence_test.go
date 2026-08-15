package manager

import (
	"testing"

	"gridsim/pkg/config"
	"gridsim/pkg/library"
)

func TestPeriodicPersistenceExcludesControlPoints(t *testing.T) {
	points := []*config.Point{
		{IOA: 1, PointType: config.TypeAI},
		{IOA: 2, PointType: config.TypeDI},
		{IOA: 3, PointType: config.TypePI},
		{IOA: 4, PointType: config.TypeAO},
		{IOA: 5, PointType: config.TypeDO},
	}
	samples := periodicPersistenceSamples(points, 123)
	if len(samples) != 3 {
		t.Fatalf("periodic samples=%d, want AI/DI/PI only", len(samples))
	}
	for _, sample := range samples {
		if sample.PointType == "AO" || sample.PointType == "DO" {
			t.Fatalf("control point was periodically persisted: %+v", sample)
		}
		if sample.Timestamp != 123 {
			t.Fatalf("timestamp=%d, want 123", sample.Timestamp)
		}
	}
}

func TestImmediatePersistencePolicy(t *testing.T) {
	cases := []struct {
		name      string
		pointType config.PointType
		changed   bool
		want      bool
	}{
		{"AI unchanged", config.TypeAI, false, false},
		{"AI changed", config.TypeAI, true, true},
		{"DI changed", config.TypeDI, true, true},
		{"PI changed", config.TypePI, true, true},
		{"AO repeated control", config.TypeAO, false, true},
		{"DO repeated control", config.TypeDO, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldPersistImmediately(library.PointChange{
				Point:        config.Point{PointType: tc.pointType},
				ValueChanged: tc.changed,
			})
			if got != tc.want {
				t.Fatalf("shouldPersistImmediately()=%v, want %v", got, tc.want)
			}
		})
	}
}
