package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestReadinessHandler_AllChecksPass(t *testing.T) {
	h := NewReadinessHandler(
		ReadinessCheck{Name: "redis", Critical: true, Check: func(ctx context.Context) error { return nil }},
		ReadinessCheck{Name: "keycloak", Critical: true, Check: func(ctx context.Context) error { return nil }},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result ReadinessResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Ready {
		t.Error("expected ready to be true")
	}
	if result.Checks["redis"] != "ok" {
		t.Errorf("expected redis check ok, got %s", result.Checks["redis"])
	}
	if result.Checks["keycloak"] != "ok" {
		t.Errorf("expected keycloak check ok, got %s", result.Checks["keycloak"])
	}
}

func TestReadinessHandler_CriticalCheckFails(t *testing.T) {
	h := NewReadinessHandler(
		ReadinessCheck{Name: "redis", Critical: true, Check: func(ctx context.Context) error {
			return errors.New("connection refused")
		}},
		ReadinessCheck{Name: "keycloak", Critical: true, Check: func(ctx context.Context) error { return nil }},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var result ReadinessResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Ready {
		t.Error("expected ready to be false when critical check fails")
	}
}

func TestReadinessHandler_NonCriticalCheckFails(t *testing.T) {
	h := NewReadinessHandler(
		ReadinessCheck{Name: "redis", Critical: true, Check: func(ctx context.Context) error { return nil }},
		ReadinessCheck{Name: "metrics", Critical: false, Check: func(ctx context.Context) error {
			return errors.New("metrics service down")
		}},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 when only non-critical fails, got %d", w.Code)
	}

	var result ReadinessResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !result.Ready {
		t.Error("expected ready to be true when only non-critical check fails")
	}
	if result.Checks["metrics"] == "ok" {
		t.Error("expected metrics check to report error")
	}
}

func TestReadinessHandler_CheckTimeout(t *testing.T) {
	h := NewReadinessHandler(
		ReadinessCheck{Name: "slow-dep", Critical: true, Check: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
				return nil
			}
		}},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for timed-out check, got %d", w.Code)
	}

	var result ReadinessResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Ready {
		t.Error("expected ready to be false when check times out")
	}
	if result.Checks["slow-dep"] == "ok" {
		t.Error("expected slow-dep check to report failure")
	}
}

func TestReadinessHandler_DurationIncluded(t *testing.T) {
	h := NewReadinessHandler(
		ReadinessCheck{Name: "fast", Critical: false, Check: func(ctx context.Context) error { return nil }},
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.ServeHTTP(w, r)

	var result ReadinessResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Duration == "" {
		t.Error("expected duration to be included in response")
	}

	// Verify it's a valid Go duration string
	if _, err := time.ParseDuration(result.Duration); err != nil {
		t.Errorf("expected valid duration string, got %q: %v", result.Duration, err)
	}
}

func TestReadinessHandler_ConcurrentExecution(t *testing.T) {
	var running atomic.Int32
	var maxConcurrent atomic.Int32

	makeCheck := func(name string) ReadinessCheck {
		return ReadinessCheck{
			Name:     name,
			Critical: false,
			Check: func(ctx context.Context) error {
				current := running.Add(1)
				// Track max concurrent
				for {
					old := maxConcurrent.Load()
					if current <= old || maxConcurrent.CompareAndSwap(old, current) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				running.Add(-1)
				return nil
			},
		}
	}

	h := NewReadinessHandler(
		makeCheck("check1"),
		makeCheck("check2"),
		makeCheck("check3"),
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/ready", nil)
	h.ServeHTTP(w, r)

	if maxConcurrent.Load() < 2 {
		t.Errorf("expected checks to run concurrently, max concurrent was %d", maxConcurrent.Load())
	}
}
