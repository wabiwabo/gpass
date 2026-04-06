package drain

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDrainer_AcceptsRequests(t *testing.T) {
	d := New(5 * time.Second)
	handler := d.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("normal: got %d", w.Code)
	}
}

func TestDrainer_RejectsDuringDrain(t *testing.T) {
	d := New(5 * time.Second)
	d.draining.Store(true)

	handler := d.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not be called during drain")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("draining: got %d, want 503", w.Code)
	}
	if w.Header().Get("Retry-After") != "5" {
		t.Error("should set Retry-After")
	}
	if w.Header().Get("Connection") != "close" {
		t.Error("should set Connection: close")
	}
}

func TestDrainer_TracksActive(t *testing.T) {
	d := New(5 * time.Second)

	var wg sync.WaitGroup
	wg.Add(1)
	started := make(chan struct{})

	handler := d.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		wg.Wait() // Block until test releases.
		w.WriteHeader(http.StatusOK)
	}))

	go func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}()

	<-started
	if d.Active() != 1 {
		t.Errorf("active during request: got %d", d.Active())
	}

	wg.Done()
	time.Sleep(10 * time.Millisecond)
	if d.Active() != 0 {
		t.Errorf("active after request: got %d", d.Active())
	}
}

func TestDrainer_DrainWaitsForCompletion(t *testing.T) {
	d := New(5 * time.Second)
	done := make(chan struct{})

	handler := d.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))

	// Start a request.
	go func() {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond) // Let request start.

	// Drain should wait for the in-flight request.
	err := d.Drain(context.Background())
	if err != nil {
		t.Errorf("drain: %v", err)
	}

	<-done // Request should be done.
}

func TestDrainer_DrainTimeout(t *testing.T) {
	d := New(50 * time.Millisecond)

	// Simulate stuck connection.
	d.active.Store(1)

	err := d.Drain(context.Background())
	if err != context.DeadlineExceeded {
		t.Errorf("should timeout: got %v", err)
	}
}

func TestDrainer_IsDraining(t *testing.T) {
	d := New(5 * time.Second)
	if d.IsDraining() {
		t.Error("should not be draining initially")
	}

	d.draining.Store(true)
	if !d.IsDraining() {
		t.Error("should be draining")
	}
}

func TestDrainer_Reset(t *testing.T) {
	d := New(5 * time.Second)
	d.draining.Store(true)
	d.Reset()

	if d.IsDraining() {
		t.Error("should not be draining after reset")
	}
}

func TestDrainer_DefaultTimeout(t *testing.T) {
	d := New(0) // Should default.
	if d.timeout != 30*time.Second {
		t.Errorf("default timeout: got %v", d.timeout)
	}
}
