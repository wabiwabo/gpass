package poolmon

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	// Use a fake driver name — we only need sql.DB.Stats() which works without a real driver.
	// Instead, we'll test with nil DB by checking Monitor plumbing.
	return nil
}

func TestDefaultThresholds(t *testing.T) {
	th := DefaultThresholds()
	if th.WarnUtilization != 0.7 {
		t.Errorf("WarnUtilization: got %f, want 0.7", th.WarnUtilization)
	}
	if th.CritUtilization != 0.9 {
		t.Errorf("CritUtilization: got %f, want 0.9", th.CritUtilization)
	}
}

func TestMonitor_Register(t *testing.T) {
	m := NewMonitor(DefaultThresholds())

	// We can register nil DBs (they'll panic on Stats(), but Register itself should work).
	m.Register("primary", nil)
	m.Register("replica", nil)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(m.pools))
	}
	if m.pools[0].Name != "primary" {
		t.Errorf("first pool name: got %q", m.pools[0].Name)
	}
}

func TestPoolStats_JSONSerialization(t *testing.T) {
	ps := PoolStats{
		Name:    "test-db",
		MaxOpen: 25,
		Open:    10,
		InUse:   5,
		Idle:    5,
		Status:  "healthy",
	}

	data, err := json.Marshal(ps)
	if err != nil {
		t.Fatal(err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["name"] != "test-db" {
		t.Errorf("name: got %v", parsed["name"])
	}
	if parsed["status"] != "healthy" {
		t.Errorf("status: got %v", parsed["status"])
	}
	if parsed["max_open"].(float64) != 25 {
		t.Errorf("max_open: got %v", parsed["max_open"])
	}
}

func TestMonitor_Evaluate_Healthy(t *testing.T) {
	m := NewMonitor(DefaultThresholds())
	status := m.evaluate(sql.DBStats{
		MaxOpenConnections: 100,
		InUse:              10,
	})
	if status != "healthy" {
		t.Errorf("expected healthy, got %s", status)
	}
}

func TestMonitor_Evaluate_Warning(t *testing.T) {
	m := NewMonitor(DefaultThresholds())
	status := m.evaluate(sql.DBStats{
		MaxOpenConnections: 100,
		InUse:              75, // 75% > 70% warn threshold
	})
	if status != "warning" {
		t.Errorf("expected warning, got %s", status)
	}
}

func TestMonitor_Evaluate_Critical(t *testing.T) {
	m := NewMonitor(DefaultThresholds())
	status := m.evaluate(sql.DBStats{
		MaxOpenConnections: 100,
		InUse:              95, // 95% > 90% crit threshold
	})
	if status != "critical" {
		t.Errorf("expected critical, got %s", status)
	}
}

func TestMonitor_Evaluate_NoMaxOpen(t *testing.T) {
	m := NewMonitor(DefaultThresholds())
	// MaxOpenConnections=0 means unlimited.
	status := m.evaluate(sql.DBStats{
		MaxOpenConnections: 0,
		InUse:              1000,
	})
	if status != "healthy" {
		t.Errorf("unlimited pools should always be healthy for utilization, got %s", status)
	}
}

func TestMonitor_IsHealthy_NoPools(t *testing.T) {
	m := NewMonitor(DefaultThresholds())
	if !m.IsHealthy() {
		t.Error("no pools should be considered healthy")
	}
}

func TestMonitor_Handler_Returns_JSON(t *testing.T) {
	m := NewMonitor(DefaultThresholds())
	// No pools registered — should return empty array.
	handler := m.Handler()

	req := httptest.NewRequest(http.MethodGet, "/pools", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type: got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("cache-control: got %q", cc)
	}

	var result []PoolStats
	json.NewDecoder(w.Body).Decode(&result)
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d entries", len(result))
	}
}

func TestThresholds_Custom(t *testing.T) {
	m := NewMonitor(Thresholds{
		WarnUtilization: 0.5,
		CritUtilization: 0.8,
	})

	// 60% > 50% custom warn.
	status := m.evaluate(sql.DBStats{
		MaxOpenConnections: 100,
		InUse:              60,
	})
	if status != "warning" {
		t.Errorf("expected warning at 60%% with 50%% threshold, got %s", status)
	}
}
