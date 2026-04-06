package graceful

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateStarting, "starting"},
		{StateReady, "ready"},
		{StateDraining, "draining"},
		{StateStopped, "stopped"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.state.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNew_Defaults(t *testing.T) {
	s := New(Config{Handler: http.NotFoundHandler()})

	if s.cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", s.cfg.Addr)
	}
	if s.cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout = %v", s.cfg.ShutdownTimeout)
	}
	if s.cfg.ReadTimeout != 15*time.Second {
		t.Errorf("ReadTimeout = %v", s.cfg.ReadTimeout)
	}
	if s.cfg.WriteTimeout != 15*time.Second {
		t.Errorf("WriteTimeout = %v", s.cfg.WriteTimeout)
	}
	if s.cfg.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v", s.cfg.IdleTimeout)
	}
	if s.State() != StateStarting {
		t.Errorf("State = %v, want Starting", s.State())
	}
}

func TestNew_CustomConfig(t *testing.T) {
	s := New(Config{
		Addr:            ":9090",
		Handler:         http.NotFoundHandler(),
		ShutdownTimeout: 10 * time.Second,
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		IdleTimeout:     30 * time.Second,
	})

	if s.cfg.Addr != ":9090" {
		t.Errorf("Addr = %q", s.cfg.Addr)
	}
	if s.cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("ShutdownTimeout = %v", s.cfg.ShutdownTimeout)
	}
}

func TestNew_NegativeDrainDelay(t *testing.T) {
	s := New(Config{
		Handler:    http.NotFoundHandler(),
		DrainDelay: -1 * time.Second,
	})
	if s.cfg.DrainDelay != 0 {
		t.Errorf("DrainDelay = %v, want 0", s.cfg.DrainDelay)
	}
}

func TestIsReady(t *testing.T) {
	s := New(Config{Handler: http.NotFoundHandler()})

	if s.IsReady() {
		t.Error("should not be ready initially")
	}

	s.state.Store(int32(StateReady))
	if !s.IsReady() {
		t.Error("should be ready after setting state")
	}

	s.state.Store(int32(StateDraining))
	if s.IsReady() {
		t.Error("should not be ready while draining")
	}
}

func TestHealthHandler_Ready(t *testing.T) {
	s := New(Config{Handler: http.NotFoundHandler()})
	s.state.Store(int32(StateReady))

	handler := s.HealthHandler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	handler(w, req)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "ready" {
		t.Errorf("body = %q, want ready", w.Body.String())
	}
}

func TestHealthHandler_Starting(t *testing.T) {
	s := New(Config{Handler: http.NotFoundHandler()})

	handler := s.HealthHandler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	handler(w, req)

	if w.Code != 503 {
		t.Errorf("status = %d, want 503", w.Code)
	}
	if w.Body.String() != "starting" {
		t.Errorf("body = %q, want starting", w.Body.String())
	}
}

func TestHealthHandler_Draining(t *testing.T) {
	s := New(Config{Handler: http.NotFoundHandler()})
	s.state.Store(int32(StateDraining))

	handler := s.HealthHandler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	handler(w, req)

	if w.Code != 503 {
		t.Errorf("status = %d, want 503", w.Code)
	}
	if w.Body.String() != "draining" {
		t.Errorf("body = %q, want draining", w.Body.String())
	}
}

func TestListenAndServe_ShutdownViaSignal(t *testing.T) {
	ready := make(chan struct{})
	s := New(Config{
		Addr:            ":0",
		Handler:         http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }),
		ShutdownTimeout: 5 * time.Second,
		OnReady:         func() { close(ready) },
	})

	// Use injectable notifyCtx to simulate signal
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.listenAndServe(func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
			return ctx, cancel
		})
	}()

	<-ready
	if s.State() != StateReady {
		t.Errorf("State = %v, want Ready", s.State())
	}

	// Simulate signal
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ListenAndServe returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown timed out")
	}

	if s.State() != StateStopped {
		t.Errorf("State = %v, want Stopped", s.State())
	}
}

func TestListenAndServe_OnShutdownCallback(t *testing.T) {
	var shutdownCalled bool
	ready := make(chan struct{})

	s := New(Config{
		Addr:       ":0",
		Handler:    http.NotFoundHandler(),
		OnReady:    func() { close(ready) },
		OnShutdown: func() { shutdownCalled = true },
	})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)

	go func() {
		errCh <- s.listenAndServe(func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
			return ctx, cancel
		})
	}()

	<-ready
	cancel()

	select {
	case <-errCh:
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown timed out")
	}

	if !shutdownCalled {
		t.Error("OnShutdown callback was not called")
	}
}

func TestShutdown_Direct(t *testing.T) {
	s := New(Config{
		Addr:    ":0",
		Handler: http.NotFoundHandler(),
	})

	// Direct shutdown without ListenAndServe — should succeed (server not started)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	// Server not started, Shutdown is a no-op
	_ = s.Shutdown(ctx)
}
