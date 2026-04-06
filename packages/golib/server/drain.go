package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"
)

// DrainableServer wraps http.Server with connection draining support.
// During drain phase:
// 1. Stop accepting new connections (return 503)
// 2. Wait for in-flight requests to complete (up to timeout)
// 3. Then shut down
type DrainableServer struct {
	server   *http.Server
	draining atomic.Bool
	inFlight atomic.Int64
	drainCh  chan struct{}
}

// NewDrainable creates a new DrainableServer.
func NewDrainable(addr string, handler http.Handler) *DrainableServer {
	ds := &DrainableServer{
		drainCh: make(chan struct{}, 1),
	}
	ds.server = &http.Server{
		Addr:              addr,
		Handler:           ds.DrainMiddleware(handler),
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return ds
}

// ListenAndServe starts the underlying HTTP server.
func (s *DrainableServer) ListenAndServe() error {
	return s.server.ListenAndServe()
}

// Drain initiates connection draining. Returns when all in-flight requests
// complete or timeout is reached.
func (s *DrainableServer) Drain(timeout time.Duration) error {
	s.draining.Store(true)
	slog.Info("drain started, waiting for in-flight requests")

	// Wait for in-flight requests to finish or timeout.
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		if s.inFlight.Load() == 0 {
			break
		}
		select {
		case <-s.drainCh:
			// A request completed; check again.
			if s.inFlight.Load() == 0 {
				break
			}
			continue
		case <-timer.C:
			slog.Warn("drain timeout reached", slog.Int64("remaining", s.inFlight.Load()))
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return s.server.Shutdown(ctx)
		}
		// If we broke out of the inner select via the first case and count is 0, exit loop.
		if s.inFlight.Load() == 0 {
			break
		}
	}

	slog.Info("all in-flight requests completed, shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// IsDraining returns whether the server is in drain mode.
func (s *DrainableServer) IsDraining() bool {
	return s.draining.Load()
}

// InFlight returns the current number of in-flight requests.
func (s *DrainableServer) InFlight() int64 {
	return s.inFlight.Load()
}

// DrainMiddleware returns middleware that tracks in-flight requests
// and returns 503 during drain phase.
func (s *DrainableServer) DrainMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.draining.Load() {
			w.Header().Set("Connection", "close")
			w.Header().Set("Retry-After", "30")
			http.Error(w, "service draining", http.StatusServiceUnavailable)
			return
		}

		s.inFlight.Add(1)
		defer func() {
			if s.inFlight.Add(-1) == 0 && s.draining.Load() {
				// Signal that all in-flight requests are done.
				select {
				case s.drainCh <- struct{}{}:
				default:
				}
			}
		}()

		next.ServeHTTP(w, r)
	})
}
