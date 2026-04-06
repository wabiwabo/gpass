package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery catches panics from downstream handlers and converts them
// to HTTP 500 responses with structured logging. This prevents a single
// panicking request from crashing the entire server process.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := string(debug.Stack())
				slog.Error("panic recovered",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"request_id", GetRequestID(r.Context()),
					"stack", stack,
				)
				http.Error(w, `{"error":"internal_server_error","message":"An unexpected error occurred"}`, http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
