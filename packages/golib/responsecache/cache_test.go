package responsecache

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func newTestHandler(body string, statusCode int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write([]byte(body))
	})
}

func TestCache_CachesGETResponse(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})

	c := New(Config{DefaultTTL: 1 * time.Minute, MaxEntries: 100})
	wrapped := c.Middleware()(handler)

	// First request: miss.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "hello" {
		t.Fatalf("expected 'hello', got %q", rec.Body.String())
	}
	if callCount != 1 {
		t.Fatalf("expected handler called once, got %d", callCount)
	}

	// Second request: hit.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}
	if rec2.Body.String() != "hello" {
		t.Fatalf("expected 'hello', got %q", rec2.Body.String())
	}
	if callCount != 1 {
		t.Fatalf("expected handler called once (cached), got %d", callCount)
	}
}

func TestCache_SkipsPOST(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("ok"))
	})

	c := New(Config{DefaultTTL: 1 * time.Minute, MaxEntries: 100})
	wrapped := c.Middleware()(handler)

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	if callCount != 3 {
		t.Fatalf("expected handler called 3 times for POST, got %d", callCount)
	}
}

func TestCache_SkipsPaths(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("ok"))
	})

	c := New(Config{
		DefaultTTL: 1 * time.Minute,
		MaxEntries: 100,
		SkipPaths:  []string{"/health", "/metrics"},
	})
	wrapped := c.Middleware()(handler)

	// Request to skipped path twice.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	if callCount != 2 {
		t.Fatalf("expected handler called 2 times for skipped path, got %d", callCount)
	}

	// Request to non-skipped path should cache.
	callCount = 0
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	if callCount != 1 {
		t.Fatalf("expected handler called once for cacheable path, got %d", callCount)
	}
}

func TestCache_ETagConditional(t *testing.T) {
	handler := newTestHandler("hello", http.StatusOK)

	c := New(Config{DefaultTTL: 1 * time.Minute, MaxEntries: 100})
	wrapped := c.Middleware()(handler)

	// Prime the cache.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	etag := rec.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header on first response")
	}

	// Conditional request with matching ETag.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("If-None-Match", etag)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusNotModified {
		t.Fatalf("expected 304, got %d", rec2.Code)
	}

	if rec2.Body.Len() != 0 {
		t.Fatalf("expected empty body for 304, got %q", rec2.Body.String())
	}
}

func TestCache_CacheHeaders(t *testing.T) {
	handler := newTestHandler("hello", http.StatusOK)

	c := New(Config{DefaultTTL: 30 * time.Second, MaxEntries: 100})
	wrapped := c.Middleware()(handler)

	// First request: MISS.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected X-Cache: MISS, got %q", rec.Header().Get("X-Cache"))
	}
	if rec.Header().Get("ETag") == "" {
		t.Fatal("expected ETag header on miss")
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Fatal("expected Cache-Control header on miss")
	}

	// Second request: HIT.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if rec2.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected X-Cache: HIT, got %q", rec2.Header().Get("X-Cache"))
	}
	if rec2.Header().Get("ETag") == "" {
		t.Fatal("expected ETag header on hit")
	}
	if rec2.Header().Get("Cache-Control") == "" {
		t.Fatal("expected Cache-Control header on hit")
	}
}

func TestCache_TTLExpiration(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("hello"))
	})

	c := New(Config{DefaultTTL: 50 * time.Millisecond, MaxEntries: 100})
	wrapped := c.Middleware()(handler)

	// Prime the cache.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// Wait for expiration.
	time.Sleep(100 * time.Millisecond)

	// Should be a miss now.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if callCount != 2 {
		t.Fatalf("expected 2 calls after TTL expiry, got %d", callCount)
	}
}

func TestCache_Invalidate(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("hello"))
	})

	c := New(Config{DefaultTTL: 1 * time.Minute, MaxEntries: 100})
	wrapped := c.Middleware()(handler)

	// Prime.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	// Invalidate.
	c.Invalidate("/test")

	// Should be a miss.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if callCount != 2 {
		t.Fatalf("expected 2 calls after invalidation, got %d", callCount)
	}
}

func TestCache_InvalidatePrefix(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("hello"))
	})

	c := New(Config{DefaultTTL: 1 * time.Minute, MaxEntries: 100})
	wrapped := c.Middleware()(handler)

	// Prime multiple paths.
	paths := []string{"/api/users", "/api/users/1", "/api/orders"}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	if callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", callCount)
	}

	// Invalidate /api/users prefix.
	c.InvalidatePrefix("/api/users")

	// /api/users and /api/users/1 should miss, /api/orders should hit.
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	// 3 original + 2 misses for /api/users and /api/users/1.
	if callCount != 5 {
		t.Fatalf("expected 5 calls after prefix invalidation, got %d", callCount)
	}
}

func TestCache_Flush(t *testing.T) {
	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Write([]byte("hello"))
	})

	c := New(Config{DefaultTTL: 1 * time.Minute, MaxEntries: 100})
	wrapped := c.Middleware()(handler)

	// Prime.
	for _, p := range []string{"/a", "/b", "/c"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	if callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", callCount)
	}

	// Flush.
	c.Flush()

	stats := c.Stats()
	if stats.Entries != 0 {
		t.Fatalf("expected 0 entries after flush, got %d", stats.Entries)
	}

	// All should miss now.
	for _, p := range []string{"/a", "/b", "/c"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
	}

	if callCount != 6 {
		t.Fatalf("expected 6 calls after flush, got %d", callCount)
	}
}

func TestCache_Stats(t *testing.T) {
	handler := newTestHandler("hello", http.StatusOK)

	c := New(Config{DefaultTTL: 1 * time.Minute, MaxEntries: 100})
	wrapped := c.Middleware()(handler)

	// Miss.
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	// Hit.
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	// Another hit.
	req3 := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec3 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec3, req3)

	stats := c.Stats()
	if stats.Misses != 1 {
		t.Fatalf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Hits != 2 {
		t.Fatalf("expected 2 hits, got %d", stats.Hits)
	}
	if stats.Entries != 1 {
		t.Fatalf("expected 1 entry, got %d", stats.Entries)
	}
}

func TestCache_MaxEntries(t *testing.T) {
	handler := newTestHandler("hello", http.StatusOK)

	c := New(Config{DefaultTTL: 1 * time.Minute, MaxEntries: 3})
	wrapped := c.Middleware()(handler)

	// Fill cache to capacity.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/path/%d", i), nil)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		// Small sleep to ensure distinct CreatedAt times for deterministic eviction.
		time.Sleep(5 * time.Millisecond)
	}

	stats := c.Stats()
	if stats.Entries != 3 {
		t.Fatalf("expected 3 entries, got %d", stats.Entries)
	}

	// Add one more, should trigger eviction of the oldest (/path/0).
	req := httptest.NewRequest(http.MethodGet, "/path/3", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	stats = c.Stats()
	if stats.Entries != 3 {
		t.Fatalf("expected 3 entries after eviction, got %d", stats.Entries)
	}
	if stats.Evictions != 1 {
		t.Fatalf("expected 1 eviction, got %d", stats.Evictions)
	}

	// Verify /path/0 was evicted (should be a miss).
	c.mu.RLock()
	_, exists := c.entries["/path/0"]
	c.mu.RUnlock()
	if exists {
		t.Fatal("expected /path/0 to be evicted")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	handler := newTestHandler("hello", http.StatusOK)

	c := New(Config{DefaultTTL: 1 * time.Minute, MaxEntries: 1000})
	wrapped := c.Middleware()(handler)

	var wg sync.WaitGroup
	const goroutines = 50
	const requestsPerGoroutine = 20

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				path := fmt.Sprintf("/path/%d", j%10)
				req := httptest.NewRequest(http.MethodGet, path, nil)
				rec := httptest.NewRecorder()
				wrapped.ServeHTTP(rec, req)

				if rec.Code != http.StatusOK {
					t.Errorf("goroutine %d: expected 200, got %d", id, rec.Code)
				}
			}

			// Also exercise invalidation and stats concurrently.
			c.Invalidate("/path/0")
			c.InvalidatePrefix("/path/1")
			_ = c.Stats()
		}(i)
	}

	wg.Wait()

	stats := c.Stats()
	total := stats.Hits + stats.Misses
	if total == 0 {
		t.Fatal("expected some hits or misses")
	}
}
