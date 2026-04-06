// Package drain provides graceful connection draining for zero-downtime
// deploys. It tracks active connections and waits for them to complete
// before shutting down.
package drain

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Drainer tracks active connections and supports graceful draining.
type Drainer struct {
	active   atomic.Int64
	draining atomic.Bool
	mu       sync.Mutex
	done     chan struct{}
	timeout  time.Duration
}

// New creates a new connection drainer.
func New(timeout time.Duration) *Drainer {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Drainer{
		done:    make(chan struct{}),
		timeout: timeout,
	}
}

// Middleware returns HTTP middleware that tracks active requests
// and rejects new requests during drain.
func (d *Drainer) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.draining.Load() {
			w.Header().Set("Connection", "close")
			w.Header().Set("Retry-After", "5")
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"type":"about:blank","title":"Service Draining","status":503,"detail":"Server is shutting down. Please retry on another instance."}`))
			return
		}

		d.active.Add(1)
		defer d.active.Add(-1)

		next.ServeHTTP(w, r)
	})
}

// Drain begins the drain process. It marks the server as draining
// and waits for active connections to complete or timeout.
func (d *Drainer) Drain(ctx context.Context) error {
	d.draining.Store(true)

	deadline := time.After(d.timeout)
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if d.active.Load() == 0 {
			return nil
		}
		select {
		case <-deadline:
			return context.DeadlineExceeded
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			continue
		}
	}
}

// Active returns the number of in-flight requests.
func (d *Drainer) Active() int64 {
	return d.active.Load()
}

// IsDraining returns whether the drainer is in drain mode.
func (d *Drainer) IsDraining() bool {
	return d.draining.Load()
}

// Reset cancels drain mode (for testing or rollback).
func (d *Drainer) Reset() {
	d.draining.Store(false)
}
