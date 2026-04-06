package mwratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddleware_AllowsWithinLimit(t *testing.T) {
	cfg := DefaultConfig(10, 5)
	mw := Middleware(cfg)

	var called int
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(200)
	}))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/api", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 200 { t.Errorf("request %d: status = %d", i, w.Code) }
	}
	if called != 5 { t.Errorf("called = %d", called) }
}

func TestMiddleware_RejectsOverLimit(t *testing.T) {
	cfg := DefaultConfig(1, 2) // 1/s, burst 2
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	// First 2 allowed (burst)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/api", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// 3rd should be rate limited
	req := httptest.NewRequest("GET", "/api", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 429 { t.Errorf("status = %d, want 429", w.Code) }
}

func TestMiddleware_SetsHeaders(t *testing.T) {
	cfg := DefaultConfig(10, 5)
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("X-RateLimit-Limit") == "" { t.Error("missing limit header") }
	if w.Header().Get("X-RateLimit-Remaining") == "" { t.Error("missing remaining header") }
}

func TestMiddleware_RetryAfterOnReject(t *testing.T) {
	cfg := DefaultConfig(1, 1) // 1/s, burst 1
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	// Exhaust burst
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "client:1"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Rate limited
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req)

	if w2.Header().Get("Retry-After") == "" { t.Error("missing Retry-After") }
}

func TestMiddleware_PerClientKey(t *testing.T) {
	cfg := DefaultConfig(1, 1)
	mw := Middleware(cfg)

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	// Client 1 exhausted
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "client1:1"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req1)

	// Client 2 still has budget
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "client2:1"
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != 200 { t.Error("different client should have own budget") }
}

func TestMiddleware_CustomKeyFunc(t *testing.T) {
	cfg := Config{
		Rate:  1,
		Burst: 1,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-API-Key")
		},
	}
	mw := Middleware(cfg)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-API-Key", "key-1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != 200 { t.Error("first should pass") }
}
