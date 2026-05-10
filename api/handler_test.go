package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"iec104-sim/config"
	"iec104-sim/library"
)

type mockPublisher struct{}
func (m *mockPublisher) Publish(point *config.Point) {}

type mockStatus struct{}
func (m *mockStatus) ClientConnected() bool                    { return false }
func (m *mockStatus) ClientAddr() string                       { return "" }
func (m *mockStatus) Stats() (interrog, control, spont int64)  { return 0, 0, 0 }
func (m *mockStatus) Uptime() int64                            { return 0 }

func newTestHandler() *Handler {
	points := []*config.Point{
		{IOA: 1001, Name: "AI_01", PointType: config.TypeAI, ValueType: config.VTFloat, Value: 220.0, Efficient: 1.0, BaseValue: 220.0},
		{IOA: 2001, Name: "DI_01", PointType: config.TypeDI, ValueType: config.VTBit, BoolValue: false},
		{IOA: 3001, Name: "PI_01", PointType: config.TypePI, ValueType: config.VTInt, IntValue: 1000},
	}
	store := library.NewStore(points)
	return &Handler{store: store, publisher: &mockPublisher{}, status: &mockStatus{}}
}

func TestListPoints(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/points", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	pts, ok := resp["points"].([]interface{})
	if !ok {
		t.Fatal("expected points array")
	}
	if len(pts) != 3 {
		t.Errorf("expected 3 points, got %d", len(pts))
	}
}

func TestGetPoint(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/points/1001", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var pt config.Point
	if err := json.NewDecoder(rec.Body).Decode(&pt); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if pt.IOA != 1001 || pt.Name != "AI_01" {
		t.Errorf("point mismatch: %+v", pt)
	}
}

func TestGetPoint_NotFound(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/points/9999", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestUpdatePoint_AI(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"value": 235.5}`
	req := httptest.NewRequest(http.MethodPut, "/api/points/1001", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify value was updated
	pt, _ := h.store.Get(1001)
	if pt.Value != 235.5 {
		t.Errorf("expected 235.5, got %f", pt.Value)
	}
}

func TestUpdatePoint_DI(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"bool_value": true}`
	req := httptest.NewRequest(http.MethodPut, "/api/points/2001", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	pt, _ := h.store.Get(2001)
	if !pt.BoolValue {
		t.Error("expected BoolValue=true")
	}
}

func TestUpdatePoint_PI(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"int_value": 9999}`
	req := httptest.NewRequest(http.MethodPut, "/api/points/3001", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	pt, _ := h.store.Get(3001)
	if pt.IntValue != 9999 {
		t.Errorf("expected 9999, got %d", pt.IntValue)
	}
}

func TestBatchUpdate(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"points": [{"ioa": 1001, "value": 150.0}, {"ioa": 2001, "bool_value": true}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/points", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp["updated"].(float64) != 2 {
		t.Errorf("expected 2 updated, got %v", resp["updated"])
	}

	// Verify both points were updated
	p1, _ := h.store.Get(1001)
	if p1.Value != 150.0 {
		t.Errorf("expected 150.0, got %f", p1.Value)
	}
	p2, _ := h.store.Get(2001)
	if !p2.BoolValue {
		t.Error("expected BoolValue=true")
	}
}

func TestUpdateQDS(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	body := `{"invalid": true, "blocked": true}`
	req := httptest.NewRequest(http.MethodPut, "/api/points/1001/qds", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	pt, _ := h.store.Get(1001)
	if !pt.QDS.Invalid || !pt.QDS.Blocked {
		t.Errorf("QDS not set: %+v", pt.QDS)
	}
}

func TestStatus(t *testing.T) {
	h := newTestHandler()
	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var status map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&status); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if status["total_points"].(float64) != 3 {
		t.Errorf("expected 3 total_points, got %v", status["total_points"])
	}
}
