package healthagg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fixedHandler returns the given status code.
func fixedHandler(code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	})
}

// TestCheckService_StatusBranches pins the four buildReport status outcomes
// (200 → healthy, 503 → unhealthy, other → degraded, dial-fail → unhealthy)
// AND the two report-aggregation paths (critical-down→Unhealthy,
// non-critical-down→Degraded).
func TestCheckService_StatusBranches(t *testing.T) {
	healthy := httptest.NewServer(fixedHandler(http.StatusOK))
	defer healthy.Close()
	unavailable := httptest.NewServer(fixedHandler(http.StatusServiceUnavailable))
	defer unavailable.Close()
	teapot := httptest.NewServer(fixedHandler(http.StatusTeapot))
	defer teapot.Close()

	// All services healthy → overall healthy.
	a := NewAggregator()
	a.SetCacheTTL(0) // disable cache so each Check re-runs
	a.Add(Service{Name: "ok", URL: healthy.URL, Critical: true})
	r := a.Check(context.Background())
	if r.Status != StatusHealthy || r.Healthy != 1 {
		t.Errorf("all-ok: %+v", r)
	}

	// Add a degraded (non-critical 418) → overall degraded.
	a.Add(Service{Name: "teapot", URL: teapot.URL, Critical: false})
	r = a.Check(context.Background())
	if r.Status != StatusDegraded || r.Degraded != 1 {
		t.Errorf("non-critical degrade: %+v", r)
	}

	// Add critical 503 → overall unhealthy.
	a.Add(Service{Name: "down", URL: unavailable.URL, Critical: true})
	r = a.Check(context.Background())
	if r.Status != StatusUnhealthy || r.Unhealthy != 1 {
		t.Errorf("critical down: %+v", r)
	}
}

// TestCheckService_NonCriticalUnhealthyOnlyDegrades pins the branch where
// a non-critical service is Unhealthy → overall stays Degraded (not
// escalated to Unhealthy).
func TestCheckService_NonCriticalUnhealthyOnlyDegrades(t *testing.T) {
	down := httptest.NewServer(fixedHandler(http.StatusServiceUnavailable))
	defer down.Close()
	a := NewAggregator()
	a.SetCacheTTL(0)
	a.Add(Service{Name: "noncrit", URL: down.URL, Critical: false})
	r := a.Check(context.Background())
	if r.Status != StatusDegraded {
		t.Errorf("non-critical 503 should degrade, got %s", r.Status)
	}
}

// TestCheckService_BadURL pins the http.NewRequestWithContext error branch.
func TestCheckService_BadURL(t *testing.T) {
	a := NewAggregator()
	a.SetCacheTTL(0)
	a.Add(Service{Name: "bad", URL: "://malformed", Critical: true})
	r := a.Check(context.Background())
	if r.Status != StatusUnhealthy || r.Services[0].Error == "" {
		t.Errorf("bad URL should be unhealthy with error, got %+v", r.Services[0])
	}
}

// TestCheckService_DialFailure pins the client.Do error branch.
func TestCheckService_DialFailure(t *testing.T) {
	a := NewAggregator()
	a.SetCacheTTL(0)
	a.Add(Service{Name: "dead", URL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond, Critical: true})
	r := a.Check(context.Background())
	if r.Status != StatusUnhealthy {
		t.Errorf("dial fail: %+v", r)
	}
}

// TestHandler_StatusCodes pins the Handler's status-code mapping:
// healthy/degraded → 200, unhealthy → 503.
func TestHandler_StatusCodes(t *testing.T) {
	ok := httptest.NewServer(fixedHandler(http.StatusOK))
	defer ok.Close()
	bad := httptest.NewServer(fixedHandler(http.StatusServiceUnavailable))
	defer bad.Close()

	a := NewAggregator()
	a.SetCacheTTL(0)
	a.Add(Service{Name: "ok", URL: ok.URL, Critical: true})

	rec := httptest.NewRecorder()
	a.Handler()(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != 200 {
		t.Errorf("healthy should be 200, got %d", rec.Code)
	}

	a.Add(Service{Name: "down", URL: bad.URL, Critical: true})
	rec = httptest.NewRecorder()
	a.Handler()(rec, httptest.NewRequest("GET", "/health", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unhealthy should be 503, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q", cc)
	}
}
