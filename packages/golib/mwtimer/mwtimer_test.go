package mwtimer

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMiddleware_SetsResponseTimeHeader(t *testing.T) {
	cfg := DefaultConfig()
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	header := w.Header().Get("X-Response-Time")
	if header == "" {
		t.Fatal("X-Response-Time header not set")
	}
	if !strings.HasSuffix(header, "ms") {
		t.Errorf("header = %q, should end with 'ms'", header)
	}
}

func TestMiddleware_CustomHeaderName(t *testing.T) {
	cfg := Config{
		HeaderName:    "X-Duration",
		SlowThreshold: 10 * time.Second,
	}
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Duration") == "" {
		t.Error("custom header X-Duration not set")
	}
	if w.Header().Get("X-Response-Time") != "" {
		t.Error("default header should not be set when custom is configured")
	}
}

func TestMiddleware_EmptyHeaderNameUsesDefault(t *testing.T) {
	cfg := Config{HeaderName: "", SlowThreshold: 10 * time.Second}
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Response-Time") == "" {
		t.Error("default header should be used when HeaderName is empty")
	}
}

func TestMiddleware_NegativeThresholdUsesDefault(t *testing.T) {
	cfg := Config{SlowThreshold: -1 * time.Second}
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-Response-Time") == "" {
		t.Error("header should still be set with default threshold")
	}
}

func TestMiddleware_SlowRequestLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	cfg := Config{
		SlowThreshold: 1 * time.Nanosecond, // Everything is slow
		Logger:        logger,
	}
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Millisecond) // Guarantee we exceed 1ns
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/slow", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "slow request") {
		t.Errorf("expected slow request log, got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "POST") {
		t.Errorf("log should contain method, got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "/api/slow") {
		t.Errorf("log should contain path, got: %q", logOutput)
	}
}

func TestMiddleware_FastRequestNotLogged(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	cfg := Config{
		SlowThreshold: 1 * time.Hour, // Nothing is slow
		Logger:        logger,
	}
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/fast", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if buf.Len() > 0 {
		t.Errorf("fast request should not be logged, got: %q", buf.String())
	}
}

func TestMiddleware_NilLoggerUsesDefault(t *testing.T) {
	cfg := Config{Logger: nil, SlowThreshold: 10 * time.Second}
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	// Should not panic
	handler.ServeHTTP(w, req)
}

func TestMiddleware_PassesThroughHandler(t *testing.T) {
	cfg := DefaultConfig()
	mw := Middleware(cfg)

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("inner handler was not called")
	}
	if w.Header().Get("X-Custom") != "value" {
		t.Error("inner handler's headers should be preserved")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.HeaderName != "X-Response-Time" {
		t.Errorf("HeaderName = %q, want X-Response-Time", cfg.HeaderName)
	}
	if cfg.SlowThreshold != 3*time.Second {
		t.Errorf("SlowThreshold = %v, want 3s", cfg.SlowThreshold)
	}
	if cfg.Logger != nil {
		t.Error("Logger should be nil by default")
	}
}

func TestSimple(t *testing.T) {
	var called bool
	handler := Simple(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/simple", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("inner handler not called")
	}
	if w.Header().Get("X-Response-Time") == "" {
		t.Error("X-Response-Time header not set by Simple()")
	}
}

func TestMiddleware_TimingFormat(t *testing.T) {
	cfg := DefaultConfig()
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	header := w.Header().Get("X-Response-Time")
	// Should match pattern like "0.123ms"
	if !strings.Contains(header, ".") || !strings.HasSuffix(header, "ms") {
		t.Errorf("timing format unexpected: %q", header)
	}
}
