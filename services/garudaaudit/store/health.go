package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

// ReadinessHandler returns an http.HandlerFunc suitable for Kubernetes
// readinessProbe. If db is nil (in-memory mode) it always returns 200.
// Otherwise it pings Postgres with a short timeout; failure returns 503.
//
// Liveness vs readiness: this is a readiness probe — a failed DB ping
// should remove the pod from the load-balancer rotation but NOT cause
// a restart, because Postgres outages are not the service's fault.
func ReadinessHandler(db *sql.DB, service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if db == nil {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ok","service":%q,"db":"in-memory"}`, service)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unavailable","service":%q,"db":"ping_failed","error":%q}`,
				service, err.Error())
			return
		}
		stats := db.Stats()
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w,
			`{"status":"ok","service":%q,"db":"postgres","open":%d,"in_use":%d,"idle":%d}`,
			service, stats.OpenConnections, stats.InUse, stats.Idle)
	}
}
