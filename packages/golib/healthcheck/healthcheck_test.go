package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewChecker(t *testing.T) {
	c := NewChecker()
	if c.Count() != 0 {
		t.Errorf("Count = %d, want 0", c.Count())
	}
}

func TestNewChecker_WithOptions(t *testing.T) {
	c := NewChecker(
		WithVersion("1.2.3"),
		WithTimeout(10*time.Second),
	)
	if c.version != "1.2.3" {
		t.Errorf("version = %q", c.version)
	}
	if c.timeout != 10*time.Second {
		t.Errorf("timeout = %v", c.timeout)
	}
}

func TestAddCheck(t *testing.T) {
	c := NewChecker()
	c.AddCheck("db", func(ctx context.Context) error { return nil }, true)
	c.AddCheck("redis", func(ctx context.Context) error { return nil }, false)

	if c.Count() != 2 {
		t.Errorf("Count = %d, want 2", c.Count())
	}
}

func TestRun_AllHealthy(t *testing.T) {
	c := NewChecker(WithVersion("1.0.0"))
	c.AddCheck("db", func(ctx context.Context) error { return nil }, true)
	c.AddCheck("redis", func(ctx context.Context) error { return nil }, false)

	report := c.Run(context.Background())

	if report.Status != StatusUp {
		t.Errorf("Status = %q, want up", report.Status)
	}
	if len(report.Components) != 2 {
		t.Errorf("Components len = %d, want 2", len(report.Components))
	}
	if report.Version != "1.0.0" {
		t.Errorf("Version = %q", report.Version)
	}
	if report.Uptime == "" {
		t.Error("Uptime should not be empty")
	}

	for _, comp := range report.Components {
		if comp.Status != StatusUp {
			t.Errorf("component %q status = %q", comp.Name, comp.Status)
		}
		if comp.Latency == "" {
			t.Errorf("component %q latency is empty", comp.Name)
		}
	}
}

func TestRun_CriticalDown(t *testing.T) {
	c := NewChecker()
	c.AddCheck("db", func(ctx context.Context) error {
		return errors.New("connection refused")
	}, true)
	c.AddCheck("cache", func(ctx context.Context) error { return nil }, false)

	report := c.Run(context.Background())

	if report.Status != StatusDown {
		t.Errorf("Status = %q, want down", report.Status)
	}
}

func TestRun_NonCriticalDown(t *testing.T) {
	c := NewChecker()
	c.AddCheck("db", func(ctx context.Context) error { return nil }, true)
	c.AddCheck("cache", func(ctx context.Context) error {
		return errors.New("timeout")
	}, false)

	report := c.Run(context.Background())

	if report.Status != StatusDegraded {
		t.Errorf("Status = %q, want degraded", report.Status)
	}
}

func TestRun_ErrorMessage(t *testing.T) {
	c := NewChecker()
	c.AddCheck("db", func(ctx context.Context) error {
		return errors.New("dial tcp: connection refused")
	}, true)

	report := c.Run(context.Background())

	found := false
	for _, comp := range report.Components {
		if comp.Name == "db" {
			found = true
			if comp.Error != "dial tcp: connection refused" {
				t.Errorf("Error = %q", comp.Error)
			}
		}
	}
	if !found {
		t.Error("db component not found")
	}
}

func TestRun_NoChecks(t *testing.T) {
	c := NewChecker()
	report := c.Run(context.Background())
	if report.Status != StatusUp {
		t.Errorf("Status = %q, want up (no checks = healthy)", report.Status)
	}
}

func TestRun_Concurrent(t *testing.T) {
	c := NewChecker()
	c.AddCheck("slow", func(ctx context.Context) error {
		time.Sleep(50 * time.Millisecond)
		return nil
	}, true)
	c.AddCheck("fast", func(ctx context.Context) error {
		return nil
	}, true)

	start := time.Now()
	report := c.Run(context.Background())
	elapsed := time.Since(start)

	if report.Status != StatusUp {
		t.Errorf("Status = %q", report.Status)
	}
	// Both should run concurrently, so total time < 2 * slow
	if elapsed > 200*time.Millisecond {
		t.Errorf("elapsed = %v, checks should run concurrently", elapsed)
	}
}

func TestLivenessHandler(t *testing.T) {
	handler := LivenessHandler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/healthz", nil)
	handler(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q", w.Header().Get("Content-Type"))
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "up" {
		t.Errorf("status = %q", body["status"])
	}
}

func TestReadinessHandler_Healthy(t *testing.T) {
	c := NewChecker(WithVersion("1.0.0"))
	c.AddCheck("db", func(ctx context.Context) error { return nil }, true)

	handler := c.ReadinessHandler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/readyz", nil)
	handler(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}

	var report Report
	json.NewDecoder(w.Body).Decode(&report)
	if report.Status != StatusUp {
		t.Errorf("report status = %q", report.Status)
	}
}

func TestReadinessHandler_Unhealthy(t *testing.T) {
	c := NewChecker()
	c.AddCheck("db", func(ctx context.Context) error {
		return errors.New("down")
	}, true)

	handler := c.ReadinessHandler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/readyz", nil)
	handler(w, req)

	if w.Code != 503 {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestReadinessHandler_Degraded(t *testing.T) {
	c := NewChecker()
	c.AddCheck("db", func(ctx context.Context) error { return nil }, true)
	c.AddCheck("cache", func(ctx context.Context) error {
		return errors.New("timeout")
	}, false)

	handler := c.ReadinessHandler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/readyz", nil)
	handler(w, req)

	// Degraded returns 200 (still serving)
	if w.Code != 200 {
		t.Errorf("status = %d, want 200 for degraded", w.Code)
	}
}

func TestConcurrent_AddAndRun(t *testing.T) {
	c := NewChecker()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			c.AddCheck("check", func(ctx context.Context) error { return nil }, true)
		}()
		go func() {
			defer wg.Done()
			c.Run(context.Background())
		}()
	}
	wg.Wait()
}
