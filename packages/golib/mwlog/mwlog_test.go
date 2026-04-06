package mwlog

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h), &buf
}

func TestMiddlewareLogsRequest(t *testing.T) {
	logger, buf := newTestLogger()
	handler := Middleware(Config{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	output := buf.String()
	if !strings.Contains(output, "GET") {
		t.Error("log should contain method")
	}
	if !strings.Contains(output, "/api/v1/users") {
		t.Error("log should contain path")
	}
	if !strings.Contains(output, "200") {
		t.Error("log should contain status")
	}
	if !strings.Contains(output, "duration") {
		t.Error("log should contain duration")
	}
}

func TestMiddlewareSkipPaths(t *testing.T) {
	logger, buf := newTestLogger()
	handler := Middleware(Config{
		Logger:    logger,
		SkipPaths: map[string]bool{"/health": true},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rr.Code)
	}
	if buf.Len() > 0 {
		t.Error("health endpoint should not be logged")
	}
}

func TestMiddlewareLogsRequestID(t *testing.T) {
	logger, buf := newTestLogger()
	handler := Middleware(Config{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	req.Header.Set("X-Request-ID", "req-abc-123")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !strings.Contains(buf.String(), "req-abc-123") {
		t.Error("log should contain request ID")
	}
}

func TestMiddleware4xxWarning(t *testing.T) {
	logger, buf := newTestLogger()
	handler := Middleware(Config{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !strings.Contains(buf.String(), "WARN") {
		t.Error("4xx should log at WARN level")
	}
}

func TestMiddleware5xxError(t *testing.T) {
	logger, buf := newTestLogger()
	handler := Middleware(Config{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !strings.Contains(buf.String(), "ERROR") {
		t.Error("5xx should log at ERROR level")
	}
}

func TestMiddlewareLogBody(t *testing.T) {
	logger, buf := newTestLogger()
	handler := Middleware(Config{Logger: logger, LogBody: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api", strings.NewReader("body content"))
	req.ContentLength = 12
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !strings.Contains(buf.String(), "request_bytes") {
		t.Error("should log request body size when LogBody is true")
	}
}

func TestMiddlewareTracksResponseBytes(t *testing.T) {
	logger, buf := newTestLogger()
	handler := Middleware(Config{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("response data here"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !strings.Contains(buf.String(), `"bytes"`) {
		t.Error("should log response bytes")
	}
}

func TestStatusWriter(t *testing.T) {
	rr := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rr, status: http.StatusOK}

	sw.WriteHeader(http.StatusCreated)
	sw.Write([]byte("test"))

	if sw.status != http.StatusCreated {
		t.Errorf("status: got %d, want 201", sw.status)
	}
	if sw.bytes != 4 {
		t.Errorf("bytes: got %d, want 4", sw.bytes)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Logger == nil {
		t.Error("Logger should not be nil")
	}
	if !cfg.SkipPaths["/health"] {
		t.Error("/health should be skipped")
	}
	if !cfg.SkipPaths["/healthz"] {
		t.Error("/healthz should be skipped")
	}
	if !cfg.SkipPaths["/ready"] {
		t.Error("/ready should be skipped")
	}
}

func TestSimple(t *testing.T) {
	var called bool
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("handler should have been called")
	}
}

func TestMiddlewareNilConfig(t *testing.T) {
	handler := Middleware(Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req) // should not panic
	if rr.Code != http.StatusOK {
		t.Errorf("status: got %d", rr.Code)
	}
}
