package server

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDrainMiddleware_NormalRequestPassesThrough(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	ds := NewDrainable(":0", handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	ds.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Errorf("expected body 'ok', got %q", rec.Body.String())
	}
}

func TestDrainMiddleware_DrainingReturns503(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ds := NewDrainable(":0", handler)
	ds.draining.Store(true)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	ds.server.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") != "30" {
		t.Errorf("expected Retry-After header")
	}
}

func TestDrainMiddleware_InFlightRequestsComplete(t *testing.T) {
	started := make(chan struct{})
	finish := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-finish
		w.WriteHeader(http.StatusOK)
	})

	ds := NewDrainable(":0", handler)

	// Start an in-flight request.
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		ds.server.Handler.ServeHTTP(rec, req)
	}()

	<-started

	if ds.InFlight() != 1 {
		t.Errorf("expected 1 in-flight, got %d", ds.InFlight())
	}

	// Start draining.
	ds.draining.Store(true)

	// New request should get 503.
	req := httptest.NewRequest(http.MethodGet, "/new", nil)
	rec := httptest.NewRecorder()
	ds.server.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 during drain, got %d", rec.Code)
	}

	// Let in-flight request complete.
	close(finish)

	// Give time for the goroutine to finish.
	time.Sleep(50 * time.Millisecond)

	if ds.InFlight() != 0 {
		t.Errorf("expected 0 in-flight after completion, got %d", ds.InFlight())
	}
}

func TestDrainMiddleware_InFlightCounterTracks(t *testing.T) {
	started := make(chan struct{}, 3)
	finish := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		<-finish
		w.WriteHeader(http.StatusOK)
	})

	ds := NewDrainable(":0", handler)

	// Start 3 concurrent requests.
	for i := 0; i < 3; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			ds.server.Handler.ServeHTTP(rec, req)
		}()
	}

	// Wait for all to start.
	for i := 0; i < 3; i++ {
		<-started
	}

	if ds.InFlight() != 3 {
		t.Errorf("expected 3 in-flight, got %d", ds.InFlight())
	}

	close(finish)
	time.Sleep(50 * time.Millisecond)

	if ds.InFlight() != 0 {
		t.Errorf("expected 0 in-flight, got %d", ds.InFlight())
	}
}

func TestDrain_ReturnsAfterAllComplete(t *testing.T) {
	started := make(chan struct{})
	finish := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-finish
		w.WriteHeader(http.StatusOK)
	})

	ds := NewDrainable(":0", handler)

	// We need to start the server to test Drain properly via middleware only.
	// Use the handler directly for simplicity.
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		ds.server.Handler.ServeHTTP(rec, req)
	}()

	<-started

	var drainErr error
	drainDone := make(chan struct{})
	go func() {
		drainErr = ds.Drain(5 * time.Second)
		close(drainDone)
	}()

	// Let the in-flight request finish after a small delay.
	time.Sleep(50 * time.Millisecond)
	close(finish)

	select {
	case <-drainDone:
		if drainErr != nil {
			t.Errorf("drain returned error: %v", drainErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("drain did not complete in time")
	}
}

func TestIsDraining_ReflectsState(t *testing.T) {
	ds := NewDrainable(":0", http.NotFoundHandler())

	if ds.IsDraining() {
		t.Error("should not be draining initially")
	}

	ds.draining.Store(true)

	if !ds.IsDraining() {
		t.Error("should be draining after setting flag")
	}
}

func TestDrainMiddleware_ConcurrentAccess(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ds := NewDrainable(":0", handler)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			ds.server.Handler.ServeHTTP(rec, req)
		}()
	}
	wg.Wait()

	if ds.InFlight() != 0 {
		t.Errorf("expected 0 in-flight after all complete, got %d", ds.InFlight())
	}
}
