package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExtendedHandler_AllPass(t *testing.T) {
	checks := map[string]func(ctx context.Context) CheckResult{
		"db:connections": func(ctx context.Context) CheckResult {
			return CheckResult{
				ComponentID:   "db-1",
				ComponentType: "datastore",
				Status:        "pass",
				ObservedValue: 10,
				ObservedUnit:  "connections",
				Time:          time.Now().UTC().Format(time.RFC3339),
			}
		},
		"cache:ping": func(ctx context.Context) CheckResult {
			return CheckResult{
				ComponentID:   "cache-1",
				ComponentType: "datastore",
				Status:        "pass",
				Time:          time.Now().UTC().Format(time.RFC3339),
			}
		},
	}

	handler := ExtendedHandler("test-service", "1.0.0", "Test Service", checks)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp ExtendedHealth
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Status != "pass" {
		t.Errorf("expected status 'pass', got '%s'", resp.Status)
	}
}

func TestExtendedHandler_SomeWarn(t *testing.T) {
	checks := map[string]func(ctx context.Context) CheckResult{
		"db:connections": func(ctx context.Context) CheckResult {
			return CheckResult{Status: "pass", Time: time.Now().UTC().Format(time.RFC3339)}
		},
		"memory:usage": func(ctx context.Context) CheckResult {
			return CheckResult{
				Status:        "warn",
				ObservedValue: 85,
				ObservedUnit:  "percent",
				Time:          time.Now().UTC().Format(time.RFC3339),
				Output:        "memory usage above 80%",
			}
		},
	}

	handler := ExtendedHandler("test-service", "1.0.0", "Test Service", checks)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp ExtendedHealth
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Status != "warn" {
		t.Errorf("expected status 'warn', got '%s'", resp.Status)
	}
}

func TestExtendedHandler_AnyFail(t *testing.T) {
	checks := map[string]func(ctx context.Context) CheckResult{
		"db:connections": func(ctx context.Context) CheckResult {
			return CheckResult{Status: "pass", Time: time.Now().UTC().Format(time.RFC3339)}
		},
		"cache:ping": func(ctx context.Context) CheckResult {
			return CheckResult{
				Status: "fail",
				Time:   time.Now().UTC().Format(time.RFC3339),
				Output: "connection refused",
			}
		},
	}

	handler := ExtendedHandler("test-service", "1.0.0", "Test Service", checks)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var resp ExtendedHealth
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Status != "fail" {
		t.Errorf("expected status 'fail', got '%s'", resp.Status)
	}
}

func TestExtendedHandler_ResponseIncludesAllChecks(t *testing.T) {
	checks := map[string]func(ctx context.Context) CheckResult{
		"db:connections": func(ctx context.Context) CheckResult {
			return CheckResult{Status: "pass", Time: time.Now().UTC().Format(time.RFC3339)}
		},
		"cache:ping": func(ctx context.Context) CheckResult {
			return CheckResult{Status: "pass", Time: time.Now().UTC().Format(time.RFC3339)}
		},
		"disk:space": func(ctx context.Context) CheckResult {
			return CheckResult{Status: "pass", Time: time.Now().UTC().Format(time.RFC3339)}
		},
	}

	handler := ExtendedHandler("test-service", "1.0.0", "Test Service", checks)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp ExtendedHealth
	json.NewDecoder(rec.Body).Decode(&resp)

	for name := range checks {
		if _, ok := resp.Checks[name]; !ok {
			t.Errorf("check '%s' missing from response", name)
		}
	}

	if resp.ServiceID != "test-service" {
		t.Errorf("expected serviceId 'test-service', got '%s'", resp.ServiceID)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got '%s'", resp.Version)
	}
	if resp.Description != "Test Service" {
		t.Errorf("expected description 'Test Service', got '%s'", resp.Description)
	}
}

func TestExtendedHandler_ContentType(t *testing.T) {
	checks := map[string]func(ctx context.Context) CheckResult{
		"db": func(ctx context.Context) CheckResult {
			return CheckResult{Status: "pass", Time: time.Now().UTC().Format(time.RFC3339)}
		},
	}

	handler := ExtendedHandler("test-service", "1.0.0", "Test Service", checks)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/health+json" {
		t.Errorf("expected content-type 'application/health+json', got '%s'", ct)
	}
}

func TestExtendedHandler_ConcurrentExecution(t *testing.T) {
	checks := map[string]func(ctx context.Context) CheckResult{
		"slow1": func(ctx context.Context) CheckResult {
			time.Sleep(30 * time.Millisecond)
			return CheckResult{Status: "pass", Time: time.Now().UTC().Format(time.RFC3339)}
		},
		"slow2": func(ctx context.Context) CheckResult {
			time.Sleep(30 * time.Millisecond)
			return CheckResult{Status: "pass", Time: time.Now().UTC().Format(time.RFC3339)}
		},
		"slow3": func(ctx context.Context) CheckResult {
			time.Sleep(30 * time.Millisecond)
			return CheckResult{Status: "pass", Time: time.Now().UTC().Format(time.RFC3339)}
		},
	}

	handler := ExtendedHandler("test-service", "1.0.0", "Test Service", checks)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	// If concurrent, 3 x 30ms should be ~30ms, not ~90ms
	if elapsed > 150*time.Millisecond {
		t.Errorf("checks took %v, expected concurrent execution to be faster", elapsed)
	}
}

func TestExtendedHandler_CheckTimeout(t *testing.T) {
	checks := map[string]func(ctx context.Context) CheckResult{
		"stuck": func(ctx context.Context) CheckResult {
			// Block until context is cancelled
			<-ctx.Done()
			return CheckResult{Status: "fail", Time: time.Now().UTC().Format(time.RFC3339)}
		},
	}

	handler := ExtendedHandler("test-service", "1.0.0", "Test Service", checks)

	// Use a context with a short timeout to trigger the timeout path
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/health", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp ExtendedHealth
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.Status != "fail" {
		t.Errorf("expected status 'fail' on timeout, got '%s'", resp.Status)
	}

	if stuckChecks, ok := resp.Checks["stuck"]; ok {
		if stuckChecks[0].Output != "check timed out" {
			t.Errorf("expected output 'check timed out', got '%s'", stuckChecks[0].Output)
		}
	} else {
		t.Error("expected 'stuck' check in results")
	}
}
