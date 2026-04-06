package reqlog

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddleware_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := DefaultConfig()
	cfg.Logger = logger

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/users?page=1", nil)
	req.Header.Set("X-Request-Id", "req-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "http_request") {
		t.Error("should log http_request message")
	}
	if !strings.Contains(output, "/api/users") {
		t.Error("should log path")
	}
	if !strings.Contains(output, "GET") {
		t.Error("should log method")
	}
}

func TestMiddleware_SkipPaths(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := DefaultConfig()
	cfg.Logger = logger

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if buf.Len() > 0 {
		t.Error("health check paths should not be logged")
	}
}

func TestMiddleware_SkipPaths_ReadyAndStartup(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := DefaultConfig()
	cfg.Logger = logger

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, path := range []string{"/readyz", "/startupz", "/metrics"} {
		buf.Reset()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if buf.Len() > 0 {
			t.Errorf("%s should be skipped", path)
		}
	}
}

func TestMiddleware_StatusCapture(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := DefaultConfig()
	cfg.Logger = logger

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "404") {
		t.Error("should log 404 status")
	}
}

func TestMiddleware_ServerError_LogLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := DefaultConfig()
	cfg.Logger = logger

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "ERROR") {
		t.Error("500 should log at ERROR level")
	}
}

func TestMiddleware_SlowRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := DefaultConfig()
	cfg.Logger = logger
	cfg.SlowThreshold = 10 * time.Millisecond

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "slow") {
		t.Error("slow request should be marked")
	}
}

func TestMiddleware_DefaultLogger(t *testing.T) {
	cfg := Config{} // No logger set.
	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req) // Should not panic.
}

func TestMiddleware_HeaderFilter(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	cfg := DefaultConfig()
	cfg.Logger = logger
	cfg.LogRequestHeaders = true

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Request-Id", "test-123")
	req.Header.Set("Authorization", "Bearer secret") // Should NOT be logged.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	output := buf.String()
	if strings.Contains(output, "secret") {
		t.Error("authorization header should not be logged")
	}
}

func TestResponseWriter_DefaultStatus(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}

	rw.Write([]byte("hello"))

	if rw.status != http.StatusOK {
		t.Errorf("default status: got %d", rw.status)
	}
	if rw.written != 5 {
		t.Errorf("written: got %d", rw.written)
	}
}

func TestResponseWriter_DoubleWriteHeader(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}

	rw.WriteHeader(http.StatusCreated)
	rw.WriteHeader(http.StatusNotFound) // Should be ignored.

	if rw.status != http.StatusCreated {
		t.Errorf("status: got %d, want 201", rw.status)
	}
}

func TestSanitizeIP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"192.168.1.1:8080", "192.168.1.1"},
		{"10.0.0.1", "10.0.0.1"},
		{"[::1]:8080", "[::1]"},
	}

	for _, tt := range tests {
		got := sanitizeIP(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeIP(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.LogRequestHeaders {
		t.Error("should log request headers by default")
	}
	if cfg.LogResponseHeaders {
		t.Error("should not log response headers by default")
	}
	if cfg.SlowThreshold != 3*time.Second {
		t.Errorf("slow threshold: got %v", cfg.SlowThreshold)
	}
	if !cfg.SkipPaths["/healthz"] {
		t.Error("should skip healthz")
	}
	if !cfg.AllowedHeaders["x-request-id"] {
		t.Error("should allow x-request-id")
	}
}
