package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHandler_AllHealthy(t *testing.T) {
	h := Handler("test-service",
		Check{Name: "db", Fn: func(ctx context.Context) error { return nil }},
		Check{Name: "redis", Fn: func(ctx context.Context) error { return nil }},
	)

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", resp["status"])
	}
	if resp["service"] != "test-service" {
		t.Errorf("expected service 'test-service', got %v", resp["service"])
	}
}

func TestHandler_OneDegraded(t *testing.T) {
	h := Handler("test-service",
		Check{Name: "db", Fn: func(ctx context.Context) error { return nil }},
		Check{Name: "redis", Fn: func(ctx context.Context) error { return errors.New("connection refused") }},
	)

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for degraded, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] != "degraded" {
		t.Errorf("expected status 'degraded', got %v", resp["status"])
	}

	checks := resp["checks"].(map[string]interface{})
	if checks["db"] != "ok" {
		t.Errorf("expected db=ok, got %v", checks["db"])
	}
	if checks["redis"] != "connection refused" {
		t.Errorf("expected redis='connection refused', got %v", checks["redis"])
	}
}

func TestHandler_AllUnhealthy(t *testing.T) {
	h := Handler("test-service",
		Check{Name: "db", Fn: func(ctx context.Context) error { return errors.New("down") }},
		Check{Name: "redis", Fn: func(ctx context.Context) error { return errors.New("down") }},
	)

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] != "unhealthy" {
		t.Errorf("expected status 'unhealthy', got %v", resp["status"])
	}
}

func TestHandler_CheckTimeout(t *testing.T) {
	h := Handler("test-service",
		Check{Name: "slow", Fn: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
				return nil
			}
		}},
	)

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] == "ok" {
		t.Error("expected non-ok status for timed-out check")
	}
}

func TestHandler_NoChecks(t *testing.T) {
	h := Handler("test-service")

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] != "ok" {
		t.Errorf("expected 'ok' with no checks, got %v", resp["status"])
	}
}
