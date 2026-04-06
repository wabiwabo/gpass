package depcheck

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChecker_AllHealthy(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{Name: "postgres", Type: "database", Check: func(_ context.Context) error { return nil }, Required: true})
	c.Add(Dependency{Name: "redis", Type: "cache", Check: func(_ context.Context) error { return nil }, Required: true})

	report := c.Check(context.Background())
	if report.Status != "healthy" {
		t.Errorf("status: got %q", report.Status)
	}
	if len(report.Dependencies) != 2 {
		t.Errorf("deps: got %d", len(report.Dependencies))
	}
}

func TestChecker_RequiredFailure(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{Name: "db", Type: "database", Check: func(_ context.Context) error { return errors.New("down") }, Required: true})
	c.Add(Dependency{Name: "cache", Type: "cache", Check: func(_ context.Context) error { return nil }})

	report := c.Check(context.Background())
	if report.Status != "unhealthy" {
		t.Errorf("required failure should be unhealthy: got %q", report.Status)
	}
}

func TestChecker_OptionalFailure(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{Name: "db", Type: "database", Check: func(_ context.Context) error { return nil }, Required: true})
	c.Add(Dependency{Name: "cache", Type: "cache", Check: func(_ context.Context) error { return errors.New("slow") }, Required: false})

	report := c.Check(context.Background())
	if report.Status != "degraded" {
		t.Errorf("optional failure should be degraded: got %q", report.Status)
	}
}

func TestChecker_Empty(t *testing.T) {
	c := NewChecker()
	report := c.Check(context.Background())
	if report.Status != "healthy" {
		t.Error("no deps should be healthy")
	}
}

func TestChecker_Timeout(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{
		Name:     "slow",
		Type:     "service",
		Timeout:  50 * time.Millisecond,
		Required: true,
		Check: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(5 * time.Second):
				return nil
			}
		},
	})

	report := c.Check(context.Background())
	if report.Status != "unhealthy" {
		t.Errorf("timed out dep should be unhealthy: got %q", report.Status)
	}
	if report.Dependencies[0].Error == "" {
		t.Error("should have error message")
	}
}

func TestChecker_Handler_Healthy(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{Name: "db", Type: "database", Check: func(_ context.Context) error { return nil }, Required: true})

	req := httptest.NewRequest(http.MethodGet, "/health/deps", nil)
	w := httptest.NewRecorder()
	c.Handler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}

	var report Report
	json.NewDecoder(w.Body).Decode(&report)
	if report.Status != "healthy" {
		t.Errorf("report status: got %q", report.Status)
	}
}

func TestChecker_Handler_Unhealthy(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{Name: "db", Type: "database", Check: func(_ context.Context) error { return errors.New("down") }, Required: true})

	req := httptest.NewRequest(http.MethodGet, "/health/deps", nil)
	w := httptest.NewRecorder()
	c.Handler()(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d, want 503", w.Code)
	}
}

func TestChecker_Count(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{Name: "a", Check: func(_ context.Context) error { return nil }})
	c.Add(Dependency{Name: "b", Check: func(_ context.Context) error { return nil }})

	if c.Count() != 2 {
		t.Errorf("count: got %d", c.Count())
	}
}

func TestChecker_Latency(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{
		Name: "slow",
		Type: "service",
		Check: func(_ context.Context) error {
			time.Sleep(20 * time.Millisecond)
			return nil
		},
	})

	report := c.Check(context.Background())
	if report.Dependencies[0].Latency == "" {
		t.Error("latency should be recorded")
	}
	if report.Duration == "" {
		t.Error("overall duration should be recorded")
	}
}

func TestChecker_ErrorMessage(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{Name: "kafka", Type: "queue", Check: func(_ context.Context) error { return errors.New("connection refused") }})

	report := c.Check(context.Background())
	if report.Dependencies[0].Error != "connection refused" {
		t.Errorf("error: got %q", report.Dependencies[0].Error)
	}
}

func TestChecker_DefaultTimeout(t *testing.T) {
	c := NewChecker()
	c.Add(Dependency{Name: "test", Check: func(_ context.Context) error { return nil }})

	// Default timeout is 5s — should not fail.
	report := c.Check(context.Background())
	if !report.Dependencies[0].Healthy {
		t.Error("should be healthy with default timeout")
	}
}
