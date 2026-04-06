package healthagg

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func healthyServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
}

func unhealthyServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
}

func TestAggregator_AllHealthy(t *testing.T) {
	s1 := healthyServer()
	s2 := healthyServer()
	defer s1.Close()
	defer s2.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(0) // Disable cache for tests.
	agg.Add(Service{Name: "svc-a", URL: s1.URL, Critical: true})
	agg.Add(Service{Name: "svc-b", URL: s2.URL, Critical: true})

	report := agg.Check(context.Background())

	if report.Status != StatusHealthy {
		t.Errorf("status: got %q", report.Status)
	}
	if report.Healthy != 2 {
		t.Errorf("healthy: got %d", report.Healthy)
	}
	if len(report.Services) != 2 {
		t.Errorf("services: got %d", len(report.Services))
	}
}

func TestAggregator_CriticalUnhealthy(t *testing.T) {
	s1 := healthyServer()
	s2 := unhealthyServer()
	defer s1.Close()
	defer s2.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(0)
	agg.Add(Service{Name: "svc-a", URL: s1.URL, Critical: false})
	agg.Add(Service{Name: "svc-b", URL: s2.URL, Critical: true})

	report := agg.Check(context.Background())

	if report.Status != StatusUnhealthy {
		t.Errorf("critical failure should be unhealthy: got %q", report.Status)
	}
	if report.Unhealthy != 1 {
		t.Errorf("unhealthy: got %d", report.Unhealthy)
	}
}

func TestAggregator_NonCriticalDegraded(t *testing.T) {
	s1 := healthyServer()
	s2 := unhealthyServer()
	defer s1.Close()
	defer s2.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(0)
	agg.Add(Service{Name: "svc-a", URL: s1.URL, Critical: true})
	agg.Add(Service{Name: "svc-b", URL: s2.URL, Critical: false})

	report := agg.Check(context.Background())

	if report.Status != StatusDegraded {
		t.Errorf("non-critical failure should be degraded: got %q", report.Status)
	}
}

func TestAggregator_UnreachableService(t *testing.T) {
	s1 := healthyServer()
	defer s1.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(0)
	agg.Add(Service{Name: "svc-a", URL: s1.URL, Critical: false})
	agg.Add(Service{Name: "svc-b", URL: "http://127.0.0.1:1", Timeout: 100 * time.Millisecond, Critical: true})

	report := agg.Check(context.Background())

	if report.Status != StatusUnhealthy {
		t.Errorf("unreachable critical should be unhealthy: got %q", report.Status)
	}

	for _, svc := range report.Services {
		if svc.Name == "svc-b" && svc.Error == "" {
			t.Error("unreachable service should have error")
		}
	}
}

func TestAggregator_EmptyServices(t *testing.T) {
	agg := NewAggregator()
	report := agg.Check(context.Background())

	if report.Status != StatusHealthy {
		t.Errorf("empty should be healthy: got %q", report.Status)
	}
	if len(report.Services) != 0 {
		t.Error("should have no services")
	}
}

func TestAggregator_Count(t *testing.T) {
	agg := NewAggregator()
	agg.Add(Service{Name: "a", URL: "http://a"})
	agg.Add(Service{Name: "b", URL: "http://b"})

	if agg.Count() != 2 {
		t.Errorf("count: got %d", agg.Count())
	}
}

func TestAggregator_Handler_Healthy(t *testing.T) {
	s := healthyServer()
	defer s.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(0)
	agg.Add(Service{Name: "svc", URL: s.URL, Critical: true})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	agg.Handler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}

	var report Report
	json.NewDecoder(w.Body).Decode(&report)
	if report.Status != StatusHealthy {
		t.Errorf("body status: got %q", report.Status)
	}
}

func TestAggregator_Handler_Unhealthy(t *testing.T) {
	s := unhealthyServer()
	defer s.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(0)
	agg.Add(Service{Name: "svc", URL: s.URL, Critical: true})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	agg.Handler()(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", w.Code)
	}
}

func TestAggregator_Latency(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(0)
	agg.Add(Service{Name: "slow", URL: s.URL})

	report := agg.Check(context.Background())
	if report.Services[0].Latency < 5*time.Millisecond {
		t.Errorf("latency should be at least 5ms: got %v", report.Services[0].Latency)
	}
	if report.Duration == 0 {
		t.Error("duration should be recorded")
	}
}

func TestAggregator_Cache(t *testing.T) {
	var callCount atomic.Int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(1 * time.Second)
	agg.Add(Service{Name: "svc", URL: s.URL})

	agg.Check(context.Background())
	agg.Check(context.Background())
	agg.Check(context.Background())

	if callCount.Load() != 1 {
		t.Errorf("cache should prevent repeated checks: got %d calls", callCount.Load())
	}
}

func TestAggregator_InvalidateCache(t *testing.T) {
	var callCount atomic.Int64
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer s.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(1 * time.Hour)
	agg.Add(Service{Name: "svc", URL: s.URL})

	agg.Check(context.Background())
	agg.InvalidateCache()
	agg.Check(context.Background())

	if callCount.Load() != 2 {
		t.Errorf("invalidate should force recheck: got %d calls", callCount.Load())
	}
}

func TestAggregator_DefaultTimeout(t *testing.T) {
	agg := NewAggregator()
	agg.Add(Service{Name: "svc", URL: "http://example.com"}) // No timeout set.

	// Just verify it doesn't panic and sets default.
	if agg.Count() != 1 {
		t.Error("should add service")
	}
}

func TestAggregator_SortedOutput(t *testing.T) {
	s := healthyServer()
	defer s.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(0)
	agg.Add(Service{Name: "zulu", URL: s.URL})
	agg.Add(Service{Name: "alpha", URL: s.URL})
	agg.Add(Service{Name: "mike", URL: s.URL})

	report := agg.Check(context.Background())
	if report.Services[0].Name != "alpha" {
		t.Errorf("should be sorted: first is %q", report.Services[0].Name)
	}
	if report.Services[2].Name != "zulu" {
		t.Errorf("should be sorted: last is %q", report.Services[2].Name)
	}
}

func TestAggregator_Headers(t *testing.T) {
	s := healthyServer()
	defer s.Close()

	agg := NewAggregator()
	agg.SetCacheTTL(0)
	agg.Add(Service{Name: "svc", URL: s.URL})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	agg.Handler()(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}
