package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery returns middleware that catches panics, logs the stack trace,
// and returns a 500 Internal Server Error.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()
				reqID := GetRequestID(r.Context())

				attrs := []any{
					slog.Any("panic", err),
					slog.String("stack", string(stack)),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				}
				if reqID != "" {
					attrs = append(attrs, slog.String("request_id", reqID))
				}

				slog.Error("panic recovered", attrs...)

				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
