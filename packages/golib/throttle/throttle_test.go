package throttle

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestLimiter_AllowWithinBurst(t *testing.T) {
	l := New(10, 5, time.Second)

	for i := 0; i < 5; i++ {
		if !l.Allow("key") {
			t.Errorf("request %d should be allowed within burst", i)
		}
	}
}

func TestLimiter_RejectOverBurst(t *testing.T) {
	l := New(10, 3, time.Second)

	for i := 0; i < 3; i++ {
		l.Allow("key")
	}

	if l.Allow("key") {
		t.Error("should reject after burst exhausted")
	}
}

func TestLimiter_Refill(t *testing.T) {
	l := New(10, 2, 50*time.Millisecond)

	l.Allow("key")
	l.Allow("key")

	if l.Allow("key") {
		t.Error("should be empty")
	}

	time.Sleep(60 * time.Millisecond) // Wait for refill.

	if !l.Allow("key") {
		t.Error("should have refilled tokens")
	}
}

func TestLimiter_PerKeyIsolation(t *testing.T) {
	l := New(10, 2, time.Second)

	l.Allow("user-a")
	l.Allow("user-a")

	// User B should still have tokens.
	if !l.Allow("user-b") {
		t.Error("different keys should have separate buckets")
	}
}

func TestLimiter_Remaining(t *testing.T) {
	l := New(10, 5, time.Second)

	if r := l.Remaining("new-key"); r != 5 {
		t.Errorf("new key remaining: got %d", r)
	}

	l.Allow("key")
	l.Allow("key")

	r := l.Remaining("key")
	if r != 3 {
		t.Errorf("after 2 uses: got %d", r)
	}
}

func TestLimiter_Reset(t *testing.T) {
	l := New(10, 3, time.Second)

	l.Allow("key")
	l.Allow("key")
	l.Allow("key")
	l.Reset("key")

	if !l.Allow("key") {
		t.Error("should allow after reset")
	}
}

func TestLimiter_Size(t *testing.T) {
	l := New(10, 5, time.Second)

	l.Allow("a")
	l.Allow("b")
	l.Allow("c")

	if l.Size() != 3 {
		t.Errorf("size: got %d", l.Size())
	}
}

func TestLimiter_Cleanup(t *testing.T) {
	l := New(10, 5, time.Second)

	l.Allow("stale")
	time.Sleep(20 * time.Millisecond)
	l.Allow("fresh")

	removed := l.Cleanup(10 * time.Millisecond)
	if removed != 1 {
		t.Errorf("removed: got %d", removed)
	}
	if l.Size() != 1 {
		t.Errorf("remaining: got %d", l.Size())
	}
}

func TestLimiter_ConcurrentAccess(t *testing.T) {
	l := New(100, 50, time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			l.Allow("shared")
		}(i)
	}
	wg.Wait()
	// Just verify no panic.
}

func TestLimiter_Defaults(t *testing.T) {
	l := New(0, 0, 0) // Should use defaults.
	if !l.Allow("key") {
		t.Error("should allow with default config")
	}
}

func TestMiddleware_Allows(t *testing.T) {
	l := New(10, 5, time.Second)
	handler := l.Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestMiddleware_Rejects(t *testing.T) {
	l := New(10, 1, time.Second)
	handler := l.Middleware(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "1.2.3.4:5678"

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Error("first should pass")
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("second should be rate limited: got %d", w.Code)
	}
	if w.Header().Get("Retry-After") != "1" {
		t.Error("should set Retry-After")
	}
}

func TestMiddleware_CustomKeyFunc(t *testing.T) {
	l := New(10, 1, time.Second)
	keyFn := func(r *http.Request) string {
		return r.Header.Get("X-API-Key")
	}

	handler := l.Middleware(keyFn)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First key.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "key-a")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Second key should still pass (different key).
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-API-Key", "key-b")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Error("different API key should have its own bucket")
	}
}

func TestIPKeyFunc(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"

	key := IPKeyFunc(req)
	if key != "10.0.0.1:12345" {
		t.Errorf("key: got %q", key)
	}
}
