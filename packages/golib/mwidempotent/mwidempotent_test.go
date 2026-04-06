package mwidempotent

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_FirstRequest(t *testing.T) {
	mw := Middleware(DefaultConfig())
	var calls int
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(201)
		w.Write([]byte(`{"id":"123"}`))
	}))

	req := httptest.NewRequest("POST", "/api/users", nil)
	req.Header.Set("Idempotency-Key", "key-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 201 { t.Errorf("status = %d", w.Code) }
	if calls != 1 { t.Errorf("calls = %d", calls) }
}

func TestMiddleware_DuplicateRequest(t *testing.T) {
	mw := Middleware(DefaultConfig())
	var calls int
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(201)
		w.Write([]byte(`{"id":"123"}`))
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api/users", nil)
		req.Header.Set("Idempotency-Key", "key-dup")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != 201 { t.Errorf("[%d] status = %d", i, w.Code) }
	}

	if calls != 1 { t.Errorf("handler called %d times, want 1", calls) }
}

func TestMiddleware_NoKey(t *testing.T) {
	mw := Middleware(DefaultConfig())
	var calls int
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
	if calls != 3 { t.Errorf("without key, all calls should execute: %d", calls) }
}

func TestMiddleware_GetSkipped(t *testing.T) {
	mw := Middleware(DefaultConfig())
	var calls int
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/api", nil)
		req.Header.Set("Idempotency-Key", "key-get")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
	if calls != 3 { t.Errorf("GET should not be cached: %d", calls) }
}

func TestMiddleware_DifferentKeys(t *testing.T) {
	mw := Middleware(DefaultConfig())
	var calls int
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api", nil)
		req.Header.Set("Idempotency-Key", string(rune('a'+i)))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
	if calls != 3 { t.Errorf("different keys = different requests: %d", calls) }
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.HeaderName != "Idempotency-Key" { t.Error("header") }
}
