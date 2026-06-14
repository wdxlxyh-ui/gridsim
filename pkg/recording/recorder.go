package recording

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Operation struct {
	Method    string          `json:"method"`
	Path      string          `json:"path"`
	Body      json.RawMessage `json:"body,omitempty"`
	Timestamp int64           `json:"ts"`
	Delay     time.Duration   `json:"delay_ms,omitempty"`
}

type Recording struct {
	Name       string      `json:"name"`
	CreatedAt  int64       `json:"created_at"`
	Operations []Operation `json:"operations"`
}

type Recorder struct {
	mu      sync.Mutex
	dir     string
	active  bool
	name    string
	startTs time.Time
	ops     []Operation
}

func NewRecorder(dir string) *Recorder {
	return &Recorder{dir: dir}
}

func (r *Recorder) Start(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.active {
		return false
	}
	r.active = true
	r.name = name
	r.startTs = time.Now()
	r.ops = make([]Operation, 0)
	return true
}

func (r *Recorder) Stop() (*Recording, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active {
		return nil, nil
	}
	r.active = false
	rec := &Recording{
		Name:       r.name,
		CreatedAt:  r.startTs.Unix(),
		Operations: r.ops,
	}
	r.ops = nil
	if err := os.MkdirAll(r.dir, 0755); err != nil {
		return rec, err
	}
	path := filepath.Join(r.dir, r.name+".json")
	return rec, os.WriteFile(path, mustMarshal(rec), 0644)
}

func (r *Recorder) Record(method, path string, body []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.active {
		return
	}
	var delay time.Duration
	if len(r.ops) > 0 {
		delay = time.Since(r.startTs) - time.Duration(r.ops[len(r.ops)-1].Timestamp-r.startTs.UnixMilli())*time.Millisecond
	}
	r.ops = append(r.ops, Operation{
		Method:    method,
		Path:      path,
		Body:      body,
		Timestamp: time.Since(r.startTs).Milliseconds(),
		Delay:     delay,
	})
}

func (r *Recorder) IsActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active
}

func ListRecordings(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			names = append(names, e.Name()[:len(e.Name())-5])
		}
	}
	return names, nil
}

func LoadRecording(dir, name string) (*Recording, error) {
	data, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return nil, err
	}
	var rec Recording
	return &rec, json.Unmarshal(data, &rec)
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.MarshalIndent(v, "", "  ")
	return b
}
