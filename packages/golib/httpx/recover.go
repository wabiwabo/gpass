// Package httpx contains shared HTTP middleware used by main.go.
// Stdlib-only, mirrors the conventions of packages/golib/mwrecover.
package httpx

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recover wraps h with a panic recovery handler. On panic it logs the
// panic value + stack trace at ERROR level and writes a 500 to the
// client. Without this every nil-deref or out-of-bounds in any handler
// crash-loops the pod — the most basic enterprise hygiene.
func Recover(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				slog.Error("panic recovered in HTTP handler",
					"panic", fmt.Sprint(rv),
					"method", r.Method,
					"path", r.URL.Path,
					"remote", r.RemoteAddr,
					"stack", string(debug.Stack()),
				)
				// If the handler already started writing, we can't change
				// the status — but we can still avoid crashing.
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":"internal_error","message":"an internal error occurred"}`))
			}
		}()
		h.ServeHTTP(w, r)
	})
}
