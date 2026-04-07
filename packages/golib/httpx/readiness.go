package httpx

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Readiness coordinates the /readyz endpoint with graceful shutdown.
//
// Lifecycle:
//  1. Service starts, ready=true, /readyz returns 200.
//  2. SIGTERM arrives, main calls r.Drain() before server.Shutdown().
//  3. /readyz starts returning 503; kube-proxy notices via the next probe
//     and removes the pod from Endpoints. New requests stop arriving.
//  4. After ShutdownGracePeriod the in-flight requests have completed
//     (or hit Timeout middleware), and main calls server.Shutdown(ctx).
//
// This closes the well-known kube-proxy / Endpoints propagation window
// (typically 5–15s on default settings) where SIGTERM-then-Shutdown
// would otherwise reset live connections.
type Readiness struct {
	service  string
	db       *sql.DB
	draining atomic.Bool
}

// NewReadiness creates a Readiness probe for a service. db may be nil for
// in-memory deployments — the probe will skip the ping.
func NewReadiness(service string, db *sql.DB) *Readiness {
	return &Readiness{service: service, db: db}
}

// Drain marks the service as draining. /readyz will return 503 from now
// on. Idempotent. Call this in your SIGTERM handler BEFORE server.Shutdown.
func (r *Readiness) Drain() {
	r.draining.Store(true)
}

// IsDraining reports whether Drain() has been called.
func (r *Readiness) IsDraining() bool {
	return r.draining.Load()
}

// Handler returns the http.HandlerFunc for the /readyz endpoint. Returns
// 503 with reason "draining" if Drain() has been called, 503 with reason
// "ping_failed" if the DB ping fails, or 200 with pool stats otherwise.
func (r *Readiness) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.draining.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"draining","service":%q}`, r.service)
			return
		}
		if r.db == nil {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ok","service":%q,"db":"in-memory"}`, r.service)
			return
		}
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()
		if err := r.db.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unavailable","service":%q,"db":"ping_failed","error":%q}`,
				r.service, err.Error())
			return
		}
		stats := r.db.Stats()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w,
			`{"status":"ok","service":%q,"db":"postgres","open":%d,"in_use":%d,"idle":%d}`,
			r.service, stats.OpenConnections, stats.InUse, stats.Idle)
	}
}
