package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestServer_StartsAndShutdown(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	s := New("0", handler,
		WithReadTimeout(5*time.Second),
		WithWriteTimeout(10*time.Second),
	)

	if s.srv.ReadTimeout != 5*time.Second {
		t.Errorf("expected ReadTimeout 5s, got %v", s.srv.ReadTimeout)
	}
	if s.srv.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout 10s, got %v", s.srv.WriteTimeout)
	}
}

func TestServer_DefaultTimeouts(t *testing.T) {
	s := New("8080", http.NotFoundHandler())

	if s.srv.ReadTimeout != 15*time.Second {
		t.Errorf("expected default ReadTimeout 15s, got %v", s.srv.ReadTimeout)
	}
	if s.srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("expected default ReadHeaderTimeout 5s, got %v", s.srv.ReadHeaderTimeout)
	}
	if s.srv.WriteTimeout != 30*time.Second {
		t.Errorf("expected default WriteTimeout 30s, got %v", s.srv.WriteTimeout)
	}
	if s.srv.IdleTimeout != 60*time.Second {
		t.Errorf("expected default IdleTimeout 60s, got %v", s.srv.IdleTimeout)
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	s := New("0", handler)

	// Start server in background
	errCh := make(chan error, 1)
	go func() {
		// We won't use Run() here since it blocks on signal.
		// Instead, test Shutdown directly.
		errCh <- s.srv.ListenAndServe()
	}()

	// Give server time to start
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}

	// Server should have stopped
	err := <-errCh
	if err != nil && err != http.ErrServerClosed {
		t.Fatalf("unexpected server error: %v", err)
	}
}
