package probe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunChecks_AllOK(t *testing.T) {
	checks := []Check{
		{Name: "db", Fn: func(_ context.Context) error { return nil }, Critical: true},
		{Name: "redis", Fn: func(_ context.Context) error { return nil }, Critical: true},
	}

	result := RunChecks(context.Background(), checks)
	if result.Status != StatusOK {
		t.Errorf("status: got %q, want ok", result.Status)
	}
	if len(result.Checks) != 2 {
		t.Errorf("checks: got %d", len(result.Checks))
	}
}

func TestRunChecks_CriticalFailure(t *testing.T) {
	checks := []Check{
		{Name: "db", Fn: func(_ context.Context) error { return errors.New("down") }, Critical: true},
		{Name: "cache", Fn: func(_ context.Context) error { return nil }},
	}

	result := RunChecks(context.Background(), checks)
	if result.Status != StatusFailed {
		t.Errorf("status: got %q, want failed", result.Status)
	}
}

func TestRunChecks_NonCriticalDegraded(t *testing.T) {
	checks := []Check{
		{Name: "db", Fn: func(_ context.Context) error { return nil }, Critical: true},
		{Name: "cache", Fn: func(_ context.Context) error { return errors.New("slow") }, Critical: false},
	}

	result := RunChecks(context.Background(), checks)
	if result.Status != StatusDegraded {
		t.Errorf("status: got %q, want degraded", result.Status)
	}
}

func TestRunChecks_Empty(t *testing.T) {
	result := RunChecks(context.Background(), nil)
	if result.Status != StatusOK {
		t.Error("empty checks should be OK")
	}
}

func TestRunChecks_Timeout(t *testing.T) {
	checks := []Check{
		{
			Name: "slow",
			Fn: func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
					return nil
				}
			},
			Timeout:  50 * time.Millisecond,
			Critical: true,
		},
	}

	result := RunChecks(context.Background(), checks)
	if result.Status != StatusFailed {
		t.Errorf("timed out check should fail: got %q", result.Status)
	}
}

func TestManager_Liveness(t *testing.T) {
	m := NewManager()
	m.AddLiveness(Check{Name: "goroutines", Fn: func(_ context.Context) error { return nil }})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	m.LivenessHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("liveness: got %d", w.Code)
	}

	var result ProbeResult
	json.NewDecoder(w.Body).Decode(&result)
	if result.Status != StatusOK {
		t.Errorf("status: got %q", result.Status)
	}
}

func TestManager_Readiness_NotReady(t *testing.T) {
	m := NewManager()
	// Don't mark ready.

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	m.ReadinessHandler()(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("not ready: got %d, want 503", w.Code)
	}
}

func TestManager_Readiness_Ready(t *testing.T) {
	m := NewManager()
	m.MarkReady()
	m.AddReadiness(Check{Name: "db", Fn: func(_ context.Context) error { return nil }, Critical: true})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	m.ReadinessHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ready: got %d", w.Code)
	}
}

func TestManager_Startup_NotStarted(t *testing.T) {
	m := NewManager()

	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	w := httptest.NewRecorder()
	m.StartupHandler()(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("not started: got %d, want 503", w.Code)
	}
}

func TestManager_Startup_Started(t *testing.T) {
	m := NewManager()
	m.MarkStarted()

	req := httptest.NewRequest(http.MethodGet, "/startupz", nil)
	w := httptest.NewRecorder()
	m.StartupHandler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("started: got %d", w.Code)
	}
}

func TestManager_MarkReadyToggle(t *testing.T) {
	m := NewManager()

	m.MarkReady()
	if !m.IsReady() {
		t.Error("should be ready")
	}

	m.MarkNotReady()
	if m.IsReady() {
		t.Error("should not be ready")
	}
}

func TestManager_RegisterHandlers(t *testing.T) {
	m := NewManager()
	m.MarkReady()
	m.MarkStarted()

	mux := http.NewServeMux()
	m.RegisterHandlers(mux)

	for _, path := range []string{"/healthz", "/readyz", "/startupz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s: got %d", path, w.Code)
		}
	}
}

func TestRunChecks_ConcurrentSafety(t *testing.T) {
	checks := make([]Check, 20)
	for i := range checks {
		checks[i] = Check{
			Name: "check",
			Fn: func(_ context.Context) error {
				time.Sleep(5 * time.Millisecond)
				return nil
			},
		}
	}

	result := RunChecks(context.Background(), checks)
	if len(result.Checks) != 20 {
		t.Errorf("checks: got %d", len(result.Checks))
	}
}

func TestProbeResult_JSONHeaders(t *testing.T) {
	m := NewManager()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	m.LivenessHandler()(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type: got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}
