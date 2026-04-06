// Package mwrecover provides panic recovery middleware for HTTP
// handlers. Catches panics, logs the stack trace, and returns a
// 500 response instead of crashing the server.
package mwrecover

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Config controls recovery behavior.
type Config struct {
	Logger       *slog.Logger
	LogStack     bool
	ResponseBody string
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		LogStack:     true,
		ResponseBody: `{"error":"internal_error","message":"an unexpected error occurred"}`,
	}
}

// Middleware returns panic recovery middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.ResponseBody == "" {
		cfg.ResponseBody = `{"error":"internal_error","message":"an unexpected error occurred"}`
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					attrs := []any{
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.Any("panic", rec),
					}
					if cfg.LogStack {
						attrs = append(attrs, slog.String("stack", string(debug.Stack())))
					}
					logger.Error("panic recovered", attrs...)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte(cfg.ResponseBody))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// Simple returns a minimal recovery middleware with defaults.
func Simple(next http.Handler) http.Handler {
	return Middleware(DefaultConfig())(next)
}
