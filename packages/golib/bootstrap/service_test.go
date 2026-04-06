package bootstrap

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew_CreatesServiceWithDefaults(t *testing.T) {
	svc := New(ServiceConfig{Name: "test-svc"})

	if svc.Config.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", svc.Config.Port)
	}
	if svc.Config.Environment != "dev" {
		t.Errorf("expected default environment dev, got %s", svc.Config.Environment)
	}
	if svc.Config.LogLevel != "info" {
		t.Errorf("expected default log level info, got %s", svc.Config.LogLevel)
	}
	if svc.Config.LogFormat != "text" {
		t.Errorf("expected default log format text, got %s", svc.Config.LogFormat)
	}
	if svc.Mux == nil {
		t.Error("Mux should not be nil")
	}
	if svc.Metrics != nil {
		t.Error("Metrics should be nil when not enabled")
	}
}

func TestAddRoute_RegistersRoute(t *testing.T) {
	svc := New(ServiceConfig{Name: "test-svc"})

	called := false
	svc.AddRoute("GET", "/api/test", "test endpoint", func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()
	svc.Handler().ServeHTTP(w, req)

	if !called {
		t.Error("route handler was not called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandler_ReturnsWrappedHandler(t *testing.T) {
	svc := New(ServiceConfig{Name: "test-svc"})

	svc.AddRoute("GET", "/hello", "hello endpoint", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello"))
	})

	handler := svc.Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil")
	}

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Body.String() != "hello" {
		t.Errorf("expected body 'hello', got %q", w.Body.String())
	}
}

func TestHealthEndpoint_Responds(t *testing.T) {
	svc := New(ServiceConfig{
		Name:         "test-svc",
		EnableHealth: true,
	})

	handler := svc.Handler()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", resp["status"])
	}
	if resp["service"] != "test-svc" {
		t.Errorf("expected service test-svc, got %v", resp["service"])
	}
}

func TestMetricsEndpoint_Responds(t *testing.T) {
	svc := New(ServiceConfig{
		Name:          "test-svc",
		EnableMetrics: true,
	})

	if svc.Metrics == nil {
		t.Fatal("Metrics should not be nil when enabled")
	}

	handler := svc.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "http_requests_total") {
		t.Error("metrics response should contain http_requests_total")
	}
}

func TestMiddleware_SecurityHeadersPresent(t *testing.T) {
	svc := New(ServiceConfig{Name: "test-svc"})

	svc.AddRoute("GET", "/secure", "secure endpoint", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := svc.Handler()

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	headers := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "0",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	}

	for key, expected := range headers {
		got := w.Header().Get(key)
		if got != expected {
			t.Errorf("header %s: expected %q, got %q", key, expected, got)
		}
	}
}

func TestMiddleware_CorrelationHeadersPresent(t *testing.T) {
	svc := New(ServiceConfig{Name: "test-svc"})

	svc.AddRoute("GET", "/corr", "correlation test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := svc.Handler()

	req := httptest.NewRequest(http.MethodGet, "/corr", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Request-Id") == "" {
		t.Error("X-Request-Id header should be generated")
	}
	if w.Header().Get("X-Correlation-Id") == "" {
		t.Error("X-Correlation-Id header should be generated")
	}
}
