package store

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func newTestService(t *testing.T, retention int, interval time.Duration) *Service {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "gridsim.db"), retention, interval)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestDefaultRetentionIsOneDay(t *testing.T) {
	if DefaultRetentionMinutes != 24*60 {
		t.Fatalf("DefaultRetentionMinutes=%d, want 1440", DefaultRetentionMinutes)
	}
	s, err := Open(filepath.Join(t.TempDir(), "gridsim.db"), DefaultRetentionMinutes, time.Second)
	if err != nil {
		t.Fatalf("Open with default retention: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
}

func TestOpenEnablesWALAndCreatesDedicatedInstanceTable(t *testing.T) {
	s := newTestService(t, 60, time.Second)

	var mode string
	if err := s.db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal mode=%q, want wal", mode)
	}
	if err := s.EnsureInstance("abc123def456"); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='ts_abc123def456'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("instance table count=%d, want 1", count)
	}
}

func TestTableNameRejectsInjection(t *testing.T) {
	for _, id := range []string{"", "ABCDEF", "abc';drop table x;--", "../abc123"} {
		if _, err := tableName(id); err == nil {
			t.Errorf("tableName(%q) accepted invalid id", id)
		}
	}
	if got, err := tableName("abc123def456"); err != nil || got != "ts_abc123def456" {
		t.Fatalf("valid table=%q err=%v", got, err)
	}
}

func TestAllPointTypesPersistAndLatestRoundTrip(t *testing.T) {
	s := newTestService(t, 60, time.Second)
	const id = "abc123def456"
	if err := s.EnsureInstance(id); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	rows := []Sample{
		{IOA: 1, Timestamp: now, Name: "AI", PointType: "AI", Value: 12.5},
		{IOA: 2, Timestamp: now, Name: "DI", PointType: "DI", BoolValue: true},
		{IOA: 3, Timestamp: now, Name: "PI", PointType: "PI", IntValue: 99},
		{IOA: 4, Timestamp: now, Name: "AO", PointType: "AO", Value: -3.25},
		{IOA: 5, Timestamp: now, Name: "DO", PointType: "DO", BoolValue: false, QDS: 3},
	}
	if err := s.insertBatch(id, rows); err != nil {
		t.Fatal(err)
	}
	latest, err := s.Latest(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 5 {
		t.Fatalf("latest count=%d, want 5", len(latest))
	}
	byIOA := make(map[uint32]LatestSample)
	for _, p := range latest {
		byIOA[p.IOA] = p
	}
	if byIOA[1].Value != 12.5 || byIOA[1].PointType != "AI" {
		t.Errorf("AI=%+v", byIOA[1])
	}
	if !byIOA[2].BoolValue || byIOA[2].PointType != "DI" {
		t.Errorf("DI=%+v", byIOA[2])
	}
	if byIOA[3].IntValue != 99 || byIOA[3].PointType != "PI" {
		t.Errorf("PI=%+v", byIOA[3])
	}
	if byIOA[4].Value != -3.25 || byIOA[4].PointType != "AO" {
		t.Errorf("AO=%+v", byIOA[4])
	}
	if byIOA[5].BoolValue || byIOA[5].QDS != 3 || byIOA[5].PointType != "DO" {
		t.Errorf("DO=%+v", byIOA[5])
	}
}

func TestHistoryUsesPointTypeSpecificValueAndNewestLimit(t *testing.T) {
	s := newTestService(t, 60, time.Second)
	const id = "abc123def456"
	if err := s.EnsureInstance(id); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UnixMilli()
	if err := s.insertBatch(id, []Sample{
		{IOA: 1, Timestamp: base + 1, Name: "PI", PointType: "PI", IntValue: 1},
		{IOA: 1, Timestamp: base + 2, Name: "PI", PointType: "PI", IntValue: 2},
		{IOA: 1, Timestamp: base + 3, Name: "PI", PointType: "PI", IntValue: 3},
		{IOA: 2, Timestamp: base + 1, Name: "DI", PointType: "DI", BoolValue: true},
	}); err != nil {
		t.Fatal(err)
	}
	series, truncated, err := s.History(id, []uint32{1, 2}, base, base+10, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !truncated {
		t.Error("expected truncated=true")
	}
	if len(series) != 2 {
		t.Fatalf("series=%d want 2", len(series))
	}
	if got := series[0].Samples; len(got) != 2 || got[0][1] != 2 || got[1][1] != 3 {
		t.Errorf("PI samples=%v", got)
	}
	if got := series[1].Samples; len(got) != 1 || got[0][1] != 1 {
		t.Errorf("DI samples=%v", got)
	}
}

func TestCleanupRemovesOnlyDataOutsideOneHour(t *testing.T) {
	s := newTestService(t, 60, time.Second)
	const id = "abc123def456"
	if err := s.EnsureInstance(id); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	if err := s.insertBatch(id, []Sample{
		{IOA: 1, Timestamp: now - 61*60*1000, Name: "AI", PointType: "AI", Value: 1},
		{IOA: 1, Timestamp: now - 59*60*1000, Name: "AI", PointType: "AI", Value: 2},
	}); err != nil {
		t.Fatal(err)
	}
	s.cleanupAll()
	series, _, err := s.History(id, []uint32{1}, now-2*60*60*1000, now, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 || len(series[0].Samples) != 1 || series[0].Samples[0][1] != 2 {
		t.Fatalf("remaining history=%+v, want only recent sample", series)
	}
}

func TestStartInstanceSamplesImmediatelyAndStops(t *testing.T) {
	s := newTestService(t, 60, 10*time.Millisecond)
	const id = "abc123def456"
	var n atomic.Uint32
	if err := s.StartInstance(id, func() []Sample {
		value := n.Add(1)
		return []Sample{{IOA: 1, Name: "AI", PointType: "AI", Value: float64(value)}}
	}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if s.Stats().Written > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if s.Stats().Written == 0 {
		t.Fatal("initial snapshot not written")
	}
	s.StopInstance(id)
	before := n.Load()
	time.Sleep(40 * time.Millisecond)
	if n.Load() != before {
		t.Fatalf("sampler continued after stop: %d -> %d", before, n.Load())
	}
}

func TestEnqueueWritesImmediateEventSample(t *testing.T) {
	s := newTestService(t, 60, time.Hour)
	const id = "abc123def456"
	if err := s.StartInstance(id, func() []Sample { return nil }); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	s.Enqueue(id, Sample{IOA: 9, Timestamp: now, Name: "AO control", PointType: "AO", Value: 12.5})
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		series, _, err := s.History(id, []uint32{9}, now-1, now+1000, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(series) == 1 && len(series[0].Samples) == 1 {
			if series[0].Samples[0][1] != 12.5 {
				t.Fatalf("value=%v, want 12.5", series[0].Samples)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("immediate event sample was not persisted")
}

func TestDeleteHistoryRemovesOnlySelectedPoints(t *testing.T) {
	s := newTestService(t, 60, time.Second)
	const id = "abc123def456"
	if err := s.EnsureInstance(id); err != nil {
		t.Fatal(err)
	}
	base := time.Now().UnixMilli()
	if err := s.insertBatch(id, []Sample{
		{IOA: 1, Timestamp: base + 1, Name: "AI 1", PointType: "AI", Value: 1},
		{IOA: 1, Timestamp: base + 2, Name: "AI 1", PointType: "AI", Value: 2},
		{IOA: 2, Timestamp: base + 1, Name: "AI 2", PointType: "AI", Value: 3},
	}); err != nil {
		t.Fatal(err)
	}

	deleted, err := s.DeleteHistory(id, []uint32{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("deleted=%d, want 2", deleted)
	}
	if _, err := s.DeleteHistory(id, nil); err == nil {
		t.Fatal("DeleteHistory accepted an empty selection")
	}

	removed, _, err := s.History(id, []uint32{1}, base, base+10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("deleted point still has history: %+v", removed)
	}
	remaining, _, err := s.History(id, []uint32{2}, base, base+10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 1 || len(remaining[0].Samples) != 1 || remaining[0].Samples[0][1] != 3 {
		t.Fatalf("unselected point history=%+v, want one unchanged sample", remaining)
	}
}
