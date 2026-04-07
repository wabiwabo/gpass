package middleware

import (
	"net/http"
)

// DefaultMaxBodyBytes is the per-request body ceiling applied by MaxBodyBytes
// when no per-route override is needed. 1 MiB is generous for JSON APIs and
// well below the OOM threshold for any reasonable container.
const DefaultMaxBodyBytes = int64(1 << 20) // 1 MiB

// MaxBodyBytes wraps h so that r.Body is limited to max bytes. Requests
// exceeding the limit return 413 Request Entity Too Large from the wrapped
// handler when it tries to read past the cap. Bounds DoS via huge POST.
//
// Each service can wrap specific routes with a larger or smaller limit.
// The default chain uses DefaultMaxBodyBytes (1 MiB).
func MaxBodyBytes(h http.Handler, max int64) http.Handler {
	if max <= 0 {
		max = DefaultMaxBodyBytes
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, max)
		h.ServeHTTP(w, r)
	})
}
