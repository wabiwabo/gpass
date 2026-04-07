package mwidempotent

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestMiddleware_TTLExpiry covers the cache-expiry branch: when a cached
// entry is older than TTL, the next request must re-execute the handler
// and overwrite the cache rather than returning a stale response.
func TestMiddleware_TTLExpiry(t *testing.T) {
	mw := Middleware(Config{TTL: 50 * time.Millisecond, Methods: []string{"POST"}})
	var calls int
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(200)
		w.Write([]byte("v"))
	}))

	doReq := func() {
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("Idempotency-Key", "k-ttl")
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}

	doReq() // miss → handler runs, calls=1
	doReq() // hit → calls still 1
	if calls != 1 {
		t.Fatalf("after hit calls = %d, want 1", calls)
	}

	time.Sleep(80 * time.Millisecond) // exceed TTL
	doReq() // expired → handler runs again, calls=2
	if calls != 2 {
		t.Errorf("after TTL expiry calls = %d, want 2 (cache should have been invalidated)", calls)
	}
}

// TestMiddleware_CachedResponseReplaysHeadersAndBody pins that the
// cached path replays the original headers, status, and body byte-for-byte.
// Without the headers loop the cached response would lose Content-Type,
// which clients downstream depend on for content negotiation.
func TestMiddleware_CachedResponseReplaysHeadersAndBody(t *testing.T) {
	mw := Middleware(DefaultConfig())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom", "trace-42")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"abc"}`))
	}))

	// First call: populate cache.
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Idempotency-Key", "k-replay")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first status = %d", w1.Code)
	}

	// Second call: hit cache. Headers + body must match exactly.
	req2 := httptest.NewRequest("POST", "/", nil)
	req2.Header.Set("Idempotency-Key", "k-replay")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Errorf("cached status = %d, want 201", w2.Code)
	}
	if got := w2.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("cached Content-Type = %q, want application/json", got)
	}
	if got := w2.Header().Get("X-Custom"); got != "trace-42" {
		t.Errorf("cached X-Custom = %q, want trace-42", got)
	}
	if !strings.Contains(w2.Body.String(), `"id":"abc"`) {
		t.Errorf("cached body = %q", w2.Body.String())
	}
}

// TestMiddleware_CustomHeaderName covers the cfg.HeaderName override
// branch — Stripe-style services use "X-Idempotency-Key" instead.
func TestMiddleware_CustomHeaderName(t *testing.T) {
	mw := Middleware(Config{HeaderName: "X-Request-Id", Methods: []string{"POST"}})
	var calls int
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/", nil)
		req.Header.Set("X-Request-Id", "uuid-1")
		handler.ServeHTTP(httptest.NewRecorder(), req)
	}
	if calls != 1 {
		t.Errorf("custom header dedup failed: calls = %d, want 1", calls)
	}

	// The default header name must NOT be honored when a custom one is set.
	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("Idempotency-Key", "uuid-1")
	handler.ServeHTTP(httptest.NewRecorder(), req)
	if calls != 2 {
		t.Errorf("default header should be ignored when custom is set: calls = %d, want 2", calls)
	}
}
