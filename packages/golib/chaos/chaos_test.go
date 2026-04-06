package chaos

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
}

func TestDisabledByDefault_RequestsPassThrough(t *testing.T) {
	fi := New()
	handler := fi.Middleware(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestEnabled100PercentErrorRate_AllRequestsFail(t *testing.T) {
	fi := New()
	fi.Enable()
	fi.SetErrorRate(100)

	handler := fi.Middleware(okHandler())

	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("request %d: expected 500, got %d", i, w.Code)
		}
	}
}

func TestEnabled0PercentErrorRate_AllRequestsPass(t *testing.T) {
	fi := New()
	fi.Enable()
	fi.SetErrorRate(0)

	handler := fi.Middleware(okHandler())

	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i, w.Code)
		}
	}
}

func TestEnabled50PercentErrorRate_RoughlyHalfFail(t *testing.T) {
	fi := New()
	fi.Enable()
	fi.SetErrorRate(50)

	handler := fi.Middleware(okHandler())

	failures := 0
	total := 1000
	for i := 0; i < total; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			failures++
		}
	}

	ratio := float64(failures) / float64(total)
	if ratio < 0.35 || ratio > 0.65 {
		t.Errorf("expected ~50%% failure rate, got %.1f%% (%d/%d)", ratio*100, failures, total)
	}
}

func TestLatencyInjection_AddsDelay(t *testing.T) {
	fi := New()
	fi.Enable()
	fi.SetLatency(50) // 50ms
	fi.SetErrorRate(0) // no errors, just latency

	handler := fi.Middleware(okHandler())

	start := time.Now()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	elapsed := time.Since(start)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("expected at least 40ms delay, got %v", elapsed)
	}
}

func TestCustomStatusCode_Returned(t *testing.T) {
	fi := New()
	fi.Enable()
	fi.SetErrorRate(100)
	fi.SetStatusCode(http.StatusServiceUnavailable)

	handler := fi.Middleware(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestDisableAfterEnable_RequestsPassAgain(t *testing.T) {
	fi := New()
	fi.Enable()
	fi.SetErrorRate(100)

	handler := fi.Middleware(okHandler())

	// Verify it fails first.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 while enabled, got %d", w.Code)
	}

	// Disable.
	fi.Disable()

	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 after disable, got %d", w.Code)
	}
}

func TestIsEnabled(t *testing.T) {
	fi := New()
	if fi.IsEnabled() {
		t.Error("expected disabled by default")
	}
	fi.Enable()
	if !fi.IsEnabled() {
		t.Error("expected enabled after Enable()")
	}
	fi.Disable()
	if fi.IsEnabled() {
		t.Error("expected disabled after Disable()")
	}
}

func TestHandler_EnableEndpoint(t *testing.T) {
	fi := New()
	h := fi.Handler()

	req := httptest.NewRequest(http.MethodPost, "/internal/chaos/enable", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !fi.IsEnabled() {
		t.Error("expected fault injector to be enabled")
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["enabled"] != true {
		t.Error("expected enabled=true in response")
	}
}

func TestHandler_DisableEndpoint(t *testing.T) {
	fi := New()
	fi.Enable()
	h := fi.Handler()

	req := httptest.NewRequest(http.MethodPost, "/internal/chaos/disable", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if fi.IsEnabled() {
		t.Error("expected fault injector to be disabled")
	}
}

func TestHandler_ConfigEndpoint(t *testing.T) {
	fi := New()
	h := fi.Handler()

	body := `{"error_rate": 75, "latency_ms": 200, "status_code": 503}`
	req := httptest.NewRequest(http.MethodPost, "/internal/chaos/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	// JSON numbers decode as float64.
	if int(resp["error_rate"].(float64)) != 75 {
		t.Errorf("expected error_rate=75, got %v", resp["error_rate"])
	}
	if int(resp["latency_ms"].(float64)) != 200 {
		t.Errorf("expected latency_ms=200, got %v", resp["latency_ms"])
	}
	if int(resp["status_code"].(float64)) != 503 {
		t.Errorf("expected status_code=503, got %v", resp["status_code"])
	}
}

func TestHandler_StatusEndpoint(t *testing.T) {
	fi := New()
	fi.Enable()
	fi.SetErrorRate(42)
	fi.SetLatency(100)
	fi.SetStatusCode(502)

	h := fi.Handler()

	req := httptest.NewRequest(http.MethodGet, "/internal/chaos/status", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", resp["enabled"])
	}
	if int(resp["error_rate"].(float64)) != 42 {
		t.Errorf("expected error_rate=42, got %v", resp["error_rate"])
	}
	if int(resp["latency_ms"].(float64)) != 100 {
		t.Errorf("expected latency_ms=100, got %v", resp["latency_ms"])
	}
	if int(resp["status_code"].(float64)) != 502 {
		t.Errorf("expected status_code=502, got %v", resp["status_code"])
	}
}

func TestHandler_ConfigInvalidJSON(t *testing.T) {
	fi := New()
	h := fi.Handler()

	req := httptest.NewRequest(http.MethodPost, "/internal/chaos/config", bytes.NewBufferString("not json"))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSetErrorRate_ClampsValues(t *testing.T) {
	fi := New()
	fi.SetErrorRate(-10)
	if fi.errorRate.Load() != 0 {
		t.Errorf("expected 0, got %d", fi.errorRate.Load())
	}
	fi.SetErrorRate(150)
	if fi.errorRate.Load() != 100 {
		t.Errorf("expected 100, got %d", fi.errorRate.Load())
	}
}

func TestSetLatency_ClampsNegative(t *testing.T) {
	fi := New()
	fi.SetLatency(-50)
	if fi.latencyMs.Load() != 0 {
		t.Errorf("expected 0, got %d", fi.latencyMs.Load())
	}
}
