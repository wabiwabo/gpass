package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestIdempotency_PostWithKey_CachesResponse(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	var callCount atomic.Int32

	handler := Idempotency(store, 1*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"123"}`))
	}))

	// First request.
	req1 := httptest.NewRequest(http.MethodPost, "/sign", nil)
	req1.Header.Set("Idempotency-Key", "key-1")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec1.Code)
	}
	if rec1.Body.String() != `{"id":"123"}` {
		t.Fatalf("unexpected body: %s", rec1.Body.String())
	}
	if callCount.Load() != 1 {
		t.Fatalf("expected handler called once, got %d", callCount.Load())
	}

	// Second request with same key — should return cached.
	req2 := httptest.NewRequest(http.MethodPost, "/sign", nil)
	req2.Header.Set("Idempotency-Key", "key-1")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusCreated {
		t.Fatalf("expected cached 201, got %d", rec2.Code)
	}
	if rec2.Body.String() != `{"id":"123"}` {
		t.Fatalf("unexpected cached body: %s", rec2.Body.String())
	}
	if callCount.Load() != 1 {
		t.Fatalf("expected handler called only once, got %d", callCount.Load())
	}
}

func TestIdempotency_PostWithoutKey_PassesThrough(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	var callCount atomic.Int32

	handler := Idempotency(store, 1*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/sign", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if callCount.Load() != 1 {
		t.Fatalf("expected handler called once, got %d", callCount.Load())
	}

	// Second call without key should also execute.
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/sign", nil))

	if callCount.Load() != 2 {
		t.Fatalf("expected handler called twice, got %d", callCount.Load())
	}
}

func TestIdempotency_GetRequests_PassThrough(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	var callCount atomic.Int32

	handler := Idempotency(store, 1*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Write([]byte("data"))
	}))

	for _, method := range []string{http.MethodGet, http.MethodDelete} {
		req := httptest.NewRequest(method, "/resource", nil)
		req.Header.Set("Idempotency-Key", "key-get")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	if callCount.Load() != 2 {
		t.Fatalf("expected handler called twice for GET/DELETE, got %d", callCount.Load())
	}
}

func TestIdempotency_DifferentKeys_SeparateResponses(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	var callCount atomic.Int32

	handler := Idempotency(store, 1*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("response-%d", n)))
	}))

	// Key A
	reqA := httptest.NewRequest(http.MethodPost, "/", nil)
	reqA.Header.Set("Idempotency-Key", "key-a")
	recA := httptest.NewRecorder()
	handler.ServeHTTP(recA, reqA)

	// Key B
	reqB := httptest.NewRequest(http.MethodPost, "/", nil)
	reqB.Header.Set("Idempotency-Key", "key-b")
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)

	if recA.Body.String() == recB.Body.String() {
		t.Fatal("different keys should have different responses")
	}
	if callCount.Load() != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount.Load())
	}
}

func TestIdempotency_ExpiredEntry_ReExecutes(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	var callCount atomic.Int32

	handler := Idempotency(store, 1*time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Idempotency-Key", "expire-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Wait for expiry.
	time.Sleep(5 * time.Millisecond)

	req2 := httptest.NewRequest(http.MethodPost, "/", nil)
	req2.Header.Set("Idempotency-Key", "expire-key")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if callCount.Load() != 2 {
		t.Fatalf("expected handler re-executed after expiry, got %d calls", callCount.Load())
	}
}

func TestIdempotency_ConcurrentSameKey_OnlyOneExecutes(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	var callCount atomic.Int32

	handler := Idempotency(store, 1*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		// Simulate slow handler.
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))

	const n = 5
	var wg sync.WaitGroup
	wg.Add(n)

	for range n {
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/", nil)
			req.Header.Set("Idempotency-Key", "concurrent-key")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Errorf("expected 201, got %d", rec.Code)
			}
		}()
	}

	wg.Wait()

	if callCount.Load() != 1 {
		t.Fatalf("expected handler called once for concurrent requests, got %d", callCount.Load())
	}
}

func TestIdempotency_PutAndPatch_Supported(t *testing.T) {
	store := NewInMemoryIdempotencyStore()
	var callCount atomic.Int32

	handler := Idempotency(store, 1*time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	for _, method := range []string{http.MethodPut, http.MethodPatch} {
		req := httptest.NewRequest(method, "/", nil)
		req.Header.Set("Idempotency-Key", "key-"+method)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// Second call with same key should be cached.
		rec2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(method, "/", nil)
		req2.Header.Set("Idempotency-Key", "key-"+method)
		handler.ServeHTTP(rec2, req2)
	}

	// 2 methods, each called once (second is cached).
	if callCount.Load() != 2 {
		t.Fatalf("expected 2 handler calls (one per method), got %d", callCount.Load())
	}
}
