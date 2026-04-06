package recovery

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestMiddleware_RecoversPanic(t *testing.T) {
	handler := Middleware(Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req) // Should not crash.

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != float64(500) {
		t.Error("body should have RFC 7807 status")
	}
}

func TestMiddleware_NoPanic(t *testing.T) {
	handler := Middleware(Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("no panic: got %d", w.Code)
	}
}

func TestMiddleware_IncludeStack(t *testing.T) {
	handler := Middleware(Config{IncludeStack: true})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("stack test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["stack"] == nil {
		t.Error("should include stack trace")
	}
	if body["panic"] != "stack test" {
		t.Errorf("panic: got %v", body["panic"])
	}
}

func TestMiddleware_NoStackByDefault(t *testing.T) {
	handler := Middleware(Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("no stack")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["stack"] != nil {
		t.Error("should not include stack by default")
	}
}

func TestMiddleware_OnPanicCallback(t *testing.T) {
	var called atomic.Bool
	handler := Middleware(Config{
		OnPanic: func(r *http.Request, err interface{}, stack []byte) {
			called.Store(true)
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("callback test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called.Load() {
		t.Error("OnPanic callback should be called")
	}
}

func TestMiddleware_LogsPanic(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	handler := Middleware(Config{Logger: logger})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("log test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !bytes.Contains(buf.Bytes(), []byte("panic recovered")) {
		t.Error("should log panic")
	}
}

func TestMiddleware_Headers(t *testing.T) {
	handler := Middleware(Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("header test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type: got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q", cc)
	}
}

func TestDefault(t *testing.T) {
	handler := Default()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("default test")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req) // Should not crash.

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d", w.Code)
	}
}
