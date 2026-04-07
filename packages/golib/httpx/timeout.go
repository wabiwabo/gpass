package httpx

import (
	"context"
	"net/http"
	"time"
)

// DefaultRequestTimeout is the per-request deadline applied by Timeout when
// no override is given. Long enough for slow database queries, short enough
// that a stuck handler doesn't hold a connection forever.
const DefaultRequestTimeout = 30 * time.Second

// Timeout wraps h with a per-request context deadline. Handlers that pass
// r.Context() down to database/HTTP calls will see context.DeadlineExceeded
// and return cleanly. Combined with the http.Server WriteTimeout, this
// gives layered protection: cooperative cancellation first, hard kill
// second.
//
// This middleware does NOT use http.TimeoutHandler — that wrapper buffers
// the entire response and breaks streaming, hijacking, and SSE. Setting
// the context deadline is the cooperative path that lets well-written
// handlers shed work voluntarily.
func Timeout(h http.Handler, d time.Duration) http.Handler {
	if d <= 0 {
		d = DefaultRequestTimeout
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
