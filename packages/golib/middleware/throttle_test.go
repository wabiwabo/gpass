package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestThrottle_AllowsUpToMaxConcurrent(t *testing.T) {
	const maxConcurrent = 3
	var active atomic.Int32

	started := make(chan struct{}, maxConcurrent)
	release := make(chan struct{})

	handler := Throttle(maxConcurrent)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		active.Add(1)
		started <- struct{}{}
		<-release
		active.Add(-1)
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	for i := 0; i < maxConcurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
		}()
	}

	// Wait for all to start
	for i := 0; i < maxConcurrent; i++ {
		<-started
	}

	if got := active.Load(); got != int32(maxConcurrent) {
		t.Errorf("expected %d active, got %d", maxConcurrent, got)
	}

	close(release)
	wg.Wait()
}

func TestThrottle_Returns429WhenExceeded(t *testing.T) {
	const maxConcurrent = 1

	started := make(chan struct{})
	release := make(chan struct{})

	handler := Throttle(maxConcurrent)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	// First request: occupies the slot
	go func() {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	}()

	<-started

	// Second request: should be rejected
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
	if ra := w.Header().Get("Retry-After"); ra != "1" {
		t.Errorf("expected Retry-After header, got %q", ra)
	}

	close(release)
}

func TestThrottle_ReleasesSlotWhenRequestCompletes(t *testing.T) {
	const maxConcurrent = 1

	handler := Throttle(maxConcurrent)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request completes
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	// Second request should succeed since slot was released
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 on second request, got %d", w2.Code)
	}
}

func TestThrottleByKey_DifferentKeysHaveIndependentLimits(t *testing.T) {
	const maxConcurrent = 1

	startedA := make(chan struct{})
	release := make(chan struct{})

	keyFunc := func(r *http.Request) string {
		return r.Header.Get("X-User-ID")
	}

	handler := ThrottleByKey(maxConcurrent, keyFunc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-ID") == "userA" {
			close(startedA)
			<-release
		}
		w.WriteHeader(http.StatusOK)
	}))

	// User A occupies its slot
	go func() {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-User-ID", "userA")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	}()

	<-startedA

	// User B should still be allowed (different key)
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-User-ID", "userB")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for userB, got %d", w.Code)
	}

	close(release)
	// Give goroutine time to finish
	time.Sleep(50 * time.Millisecond)
}

func TestThrottleByKey_SameKeySharesLimit(t *testing.T) {
	const maxConcurrent = 1

	started := make(chan struct{})
	release := make(chan struct{})

	keyFunc := func(r *http.Request) string {
		return r.Header.Get("X-User-ID")
	}

	handler := ThrottleByKey(maxConcurrent, keyFunc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	}))

	// First request from userA occupies the slot
	go func() {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-User-ID", "userA")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
	}()

	<-started

	// Second request from same userA should be rejected
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-User-ID", "userA")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for same key, got %d", w.Code)
	}

	close(release)
	time.Sleep(50 * time.Millisecond)
}
