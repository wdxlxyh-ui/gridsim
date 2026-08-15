// Package store provides SQLite-backed rolling persistence for GridSim points.
// Each running instance has an independent time-series table and keeps only
// the most recent retention window (60 minutes by default).
package store

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

const (
	DefaultRetentionMinutes = 60
	DefaultSampleInterval   = time.Second
	defaultQueueSize        = 8192
	defaultBatchSize        = 2000
)

var instanceIDPattern = regexp.MustCompile(`^[0-9a-f]{6,32}$`)

// Sample is a snapshot of one AI, DI, PI, AO or DO point.
type Sample struct {
	IOA       uint32
	Timestamp int64
	Name      string
	PointType string
	Value     float64
	BoolValue bool
	IntValue  int32
	QDS       uint8
}

// LatestSample is the newest persisted record for a point.
type LatestSample struct {
	IOA       uint32  `json:"ioa"`
	Timestamp int64   `json:"timestamp"`
	Name      string  `json:"name"`
	PointType string  `json:"point_type"`
	Value     float64 `json:"value"`
	BoolValue bool    `json:"bool_value"`
	IntValue  int32   `json:"int_value"`
	QDS       uint8   `json:"qds"`
}

// Series is a time ordered series suitable for ECharts time axes.
type Series struct {
	IOA       uint32       `json:"ioa"`
	Name      string       `json:"name"`
	PointType string       `json:"point_type"`
	Samples   [][2]float64 `json:"samples"`
}

// Stats exposes operational information without exposing a SQL handle.
type Stats struct {
	Enabled          bool  `json:"enabled"`
	SizeBytes        int64 `json:"size_bytes"`
	RetentionMinutes int   `json:"retention_minutes"`
	SampleIntervalMs int   `json:"sample_interval_ms"`
	Written          int64 `json:"written"`
	Dropped          int64 `json:"dropped"`
	Instances        int   `json:"instances"`
}

type queuedSample struct {
	instanceID string
	sample     Sample
}

type sampler struct {
	stop chan struct{}
	done chan struct{}
}

// Service owns the SQLite connection, asynchronous writer and per-instance samplers.
type Service struct {
	db               *sql.DB
	path             string
	retentionMinutes int
	sampleInterval   time.Duration
	queue            chan queuedSample
	stopWriter       chan struct{}
	writerDone       chan struct{}

	mu       sync.Mutex
	samplers map[string]*sampler
	closed   bool
	written  atomic.Int64
	dropped  atomic.Int64
}

func tableName(instanceID string) (string, error) {
	if !instanceIDPattern.MatchString(instanceID) {
		return "", fmt.Errorf("invalid instance id %q", instanceID)
	}
	return "ts_" + instanceID, nil
}

// Open creates or opens a database. It is safe to call before any instances exist.
func Open(path string, retentionMinutes int, sampleInterval time.Duration) (*Service, error) {
	if retentionMinutes <= 0 || retentionMinutes > 240 {
		return nil, fmt.Errorf("retention minutes must be within 1..240, got %d", retentionMinutes)
	}
	if sampleInterval <= 0 {
		sampleInterval = DefaultSampleInterval
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite has a single writer; serialize DB access intentionally.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Service{
		db:               db,
		path:             path,
		retentionMinutes: retentionMinutes,
		sampleInterval:   sampleInterval,
		queue:            make(chan queuedSample, defaultQueueSize),
		stopWriter:       make(chan struct{}),
		writerDone:       make(chan struct{}),
		samplers:         make(map[string]*sampler),
	}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	go s.writerLoop()
	return s, nil
}

func (s *Service) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY,
		applied_at INTEGER NOT NULL
	)`)
	if err != nil {
		return fmt.Errorf("create schema version: %w", err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM schema_version WHERE version = 1`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		if _, err := s.db.Exec(`INSERT INTO schema_version(version, applied_at) VALUES(1, ?)`, time.Now().UnixMilli()); err != nil {
			return err
		}
	}
	return nil
}

// EnsureInstance creates a dedicated time-series table. It is idempotent.
func (s *Service) EnsureInstance(instanceID string) error {
	table, err := tableName(instanceID)
	if err != nil {
		return err
	}
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			ioa INTEGER NOT NULL,
			ts INTEGER NOT NULL,
			name TEXT NOT NULL,
			point_type TEXT NOT NULL,
			value REAL,
			bool_value INTEGER,
			int_value INTEGER,
			qds INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (ioa, ts)
		) WITHOUT ROWID`, table),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_%s_ts ON %s(ts)`, table, table),
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("create %s: %w", table, err)
		}
	}
	return nil
}

// StartInstance begins immediate then periodic snapshots. Calling it twice replaces the old sampler.
func (s *Service) StartInstance(instanceID string, snapshot func() []Sample) error {
	if err := s.EnsureInstance(instanceID); err != nil {
		return err
	}
	s.StopInstance(instanceID)

	p := &sampler{stop: make(chan struct{}), done: make(chan struct{})}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return fmt.Errorf("persistence service is closed")
	}
	s.samplers[instanceID] = p
	s.mu.Unlock()

	go func() {
		defer close(p.done)
		// Capture initial values at start so opening the UI after a restart has data immediately.
		s.enqueueSnapshot(instanceID, snapshot())
		ticker := time.NewTicker(s.sampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-p.stop:
				return
			case <-ticker.C:
				s.enqueueSnapshot(instanceID, snapshot())
			}
		}
	}()
	return nil
}

// StopInstance stops a sampler and waits for it. Existing samples remain queryable.
func (s *Service) StopInstance(instanceID string) {
	s.mu.Lock()
	p, ok := s.samplers[instanceID]
	if ok {
		delete(s.samplers, instanceID)
	}
	s.mu.Unlock()
	if ok {
		close(p.stop)
		<-p.done
	}
}

// Enqueue immediately persists event-driven samples for an active instance without
// blocking the caller. Periodic snapshots and control/change events share one writer.
func (s *Service) Enqueue(instanceID string, samples ...Sample) {
	if len(samples) == 0 {
		return
	}
	s.mu.Lock()
	active := !s.closed && s.samplers[instanceID] != nil
	s.mu.Unlock()
	if active {
		s.enqueueSnapshot(instanceID, samples)
	}
}

func (s *Service) enqueueSnapshot(instanceID string, samples []Sample) {
	for _, sample := range samples {
		if sample.Timestamp == 0 {
			sample.Timestamp = time.Now().UnixMilli()
		}
		select {
		case s.queue <- queuedSample{instanceID: instanceID, sample: sample}:
		default:
			// Never block a protocol/engine goroutine for database I/O.
			s.dropped.Add(1)
		}
	}
}

func (s *Service) writerLoop() {
	defer close(s.writerDone)
	flushTicker := time.NewTicker(250 * time.Millisecond)
	cleanupTicker := time.NewTicker(time.Minute)
	defer flushTicker.Stop()
	defer cleanupTicker.Stop()
	batch := make(map[string][]Sample)
	count := 0

	flush := func() {
		if count == 0 {
			return
		}
		for id, rows := range batch {
			if err := s.insertBatch(id, rows); err != nil {
				slog.Error("SQLite 测点数据写入失败", "instance", id, "rows", len(rows), "error", err)
				continue
			}
			s.written.Add(int64(len(rows)))
		}
		batch = make(map[string][]Sample)
		count = 0
	}

	for {
		select {
		case q := <-s.queue:
			batch[q.instanceID] = append(batch[q.instanceID], q.sample)
			count++
			if count >= defaultBatchSize {
				flush()
			}
		case <-flushTicker.C:
			flush()
		case <-cleanupTicker.C:
			flush()
			s.cleanupAll()
		case <-s.stopWriter:
			for {
				select {
				case q := <-s.queue:
					batch[q.instanceID] = append(batch[q.instanceID], q.sample)
					count++
				default:
					flush()
					return
				}
			}
		}
	}
}

func (s *Service) insertBatch(instanceID string, rows []Sample) error {
	table, err := tableName(instanceID)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(fmt.Sprintf(`INSERT OR REPLACE INTO %s
		(ioa, ts, name, point_type, value, bool_value, int_value, qds)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, table))
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, row := range rows {
		b := 0
		if row.BoolValue {
			b = 1
		}
		if _, err := stmt.Exec(row.IOA, row.Timestamp, row.Name, row.PointType, row.Value, b, row.IntValue, row.QDS); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) cleanupAll() {
	rows, err := s.db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'ts_%'`)
	if err != nil {
		slog.Error("SQLite 枚举时序表失败", "error", err)
		return
	}
	// 先读完并关闭 rows。连接数被有意限定为 1，rows 未关闭时不能再 Exec。
	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err == nil && strings.HasPrefix(table, "ts_") && instanceIDPattern.MatchString(strings.TrimPrefix(table, "ts_")) {
			tables = append(tables, table)
		}
	}
	rows.Close()

	cutoff := time.Now().Add(-time.Duration(s.retentionMinutes) * time.Minute).UnixMilli()
	for _, table := range tables {
		id := strings.TrimPrefix(table, "ts_")
		if _, err := s.db.Exec(fmt.Sprintf(`DELETE FROM %s WHERE ts < ?`, table), cutoff); err != nil {
			slog.Error("SQLite 超期数据清理失败", "instance", id, "error", err)
		}
	}
	// Freed pages are reused by future inserts. Do not VACUUM on a rolling workload.
	_, _ = s.db.Exec(`PRAGMA wal_checkpoint(PASSIVE)`)
}

// Latest returns the newest persisted value of every point in an instance.
func (s *Service) Latest(instanceID string) ([]LatestSample, error) {
	table, err := tableName(instanceID)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`SELECT t.ioa, t.ts, t.name, t.point_type, t.value, t.bool_value, t.int_value, t.qds
		FROM %s t JOIN (SELECT ioa, MAX(ts) max_ts FROM %s GROUP BY ioa) latest
		ON t.ioa = latest.ioa AND t.ts = latest.max_ts ORDER BY t.ioa`, table, table)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]LatestSample, 0)
	for rows.Next() {
		var p LatestSample
		var boolValue int
		var qds int
		if err := rows.Scan(&p.IOA, &p.Timestamp, &p.Name, &p.PointType, &p.Value, &boolValue, &p.IntValue, &qds); err != nil {
			return nil, err
		}
		p.BoolValue = boolValue != 0
		p.QDS = uint8(qds)
		result = append(result, p)
	}
	return result, rows.Err()
}

// History returns time ordered series. limit applies per IOA and retains newest samples.
func (s *Service) History(instanceID string, ioas []uint32, from, to int64, limit int) ([]Series, bool, error) {
	table, err := tableName(instanceID)
	if err != nil {
		return nil, false, err
	}
	if len(ioas) == 0 {
		return nil, false, nil
	}
	if limit <= 0 || limit > 10000 {
		limit = 5000
	}
	truncated := false
	result := make([]Series, 0, len(ioas))
	for _, ioa := range ioas {
		query := fmt.Sprintf(`SELECT ts, name, point_type, value, bool_value, int_value
			FROM %s WHERE ioa=? AND ts BETWEEN ? AND ? ORDER BY ts DESC LIMIT ?`, table)
		rows, err := s.db.Query(query, ioa, from, to, limit+1)
		if err != nil {
			return nil, false, err
		}
		var series Series
		series.IOA = ioa
		for rows.Next() {
			var ts int64
			var boolValue int
			var value float64
			var intValue int32
			var name, pointType string
			if err := rows.Scan(&ts, &name, &pointType, &value, &boolValue, &intValue); err != nil {
				rows.Close()
				return nil, false, err
			}
			if series.Name == "" {
				series.Name, series.PointType = name, pointType
			}
			pointValue := value
			if pointType == "DI" || pointType == "DO" {
				pointValue = float64(boolValue)
			} else if pointType == "PI" {
				pointValue = float64(intValue)
			}
			series.Samples = append(series.Samples, [2]float64{float64(ts), pointValue})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, false, err
		}
		rows.Close()
		if len(series.Samples) > limit {
			series.Samples = series.Samples[:limit]
			truncated = true
		}
		for left, right := 0, len(series.Samples)-1; left < right; left, right = left+1, right-1 {
			series.Samples[left], series.Samples[right] = series.Samples[right], series.Samples[left]
		}
		if len(series.Samples) > 0 {
			result = append(result, series)
		}
	}
	return result, truncated, nil
}

// Stats reports current service state.
func (s *Service) Stats() Stats {
	stat := Stats{Enabled: true, RetentionMinutes: s.retentionMinutes, SampleIntervalMs: int(s.sampleInterval / time.Millisecond), Written: s.written.Load(), Dropped: s.dropped.Load()}
	if fi, err := os.Stat(s.path); err == nil {
		stat.SizeBytes = fi.Size()
	}
	s.mu.Lock()
	stat.Instances = len(s.samplers)
	s.mu.Unlock()
	return stat
}

// Close stops all samplers, flushes queued rows and closes SQLite.
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	ids := make([]string, 0, len(s.samplers))
	for id := range s.samplers {
		ids = append(ids, id)
	}
	s.mu.Unlock()
	for _, id := range ids {
		s.StopInstance(id)
	}
	close(s.stopWriter)
	<-s.writerDone
	_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return s.db.Close()
}

// DropInstance removes persisted samples when an instance configuration is deleted.
func (s *Service) DropInstance(instanceID string) error {
	s.StopInstance(instanceID)
	table, err := tableName(instanceID)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %s`, table))
	return err
}

// SortedLatest makes deterministic API responses convenient for callers that hold a map.
func SortedLatest(points []LatestSample) []LatestSample {
	sort.Slice(points, func(i, j int) bool { return points[i].IOA < points[j].IOA })
	return points
}
