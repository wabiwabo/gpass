// Package mwlog provides structured request logging middleware.
// Logs HTTP requests with method, path, status, duration, and
// client IP using slog for consistent JSON log output.
package mwlog

import (
	"log/slog"
	"net/http"
	"time"
)

// Config controls request logging.
type Config struct {
	Logger     *slog.Logger
	SkipPaths  map[string]bool // paths to skip logging (e.g., health checks)
	LogBody    bool            // log request body size
}

// DefaultConfig returns defaults.
func DefaultConfig() Config {
	return Config{
		Logger:    slog.Default(),
		SkipPaths: map[string]bool{"/health": true, "/healthz": true, "/ready": true},
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Middleware returns request logging middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if cfg.SkipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Duration("duration", duration),
				slog.Int("bytes", sw.bytes),
			}

			if r.Header.Get("X-Request-ID") != "" {
				attrs = append(attrs, slog.String("request_id", r.Header.Get("X-Request-ID")))
			}

			if cfg.LogBody && r.ContentLength > 0 {
				attrs = append(attrs, slog.Int64("request_bytes", r.ContentLength))
			}

			args := make([]any, len(attrs))
			for i, a := range attrs {
				args[i] = a
			}

			level := slog.LevelInfo
			if sw.status >= 500 {
				level = slog.LevelError
			} else if sw.status >= 400 {
				level = slog.LevelWarn
			}

			cfg.Logger.Log(r.Context(), level, "http request", args...)
		})
	}
}

// Simple returns middleware with default logging config.
func Simple(next http.Handler) http.Handler {
	return Middleware(DefaultConfig())(next)
}
