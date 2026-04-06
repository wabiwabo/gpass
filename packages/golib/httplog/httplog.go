// Package httplog provides HTTP request/response logging middleware.
// Logs method, path, status, duration, and request metadata in
// structured JSON format for observability.
package httplog

import (
	"log/slog"
	"net/http"
	"time"
)

// Config controls HTTP logging behavior.
type Config struct {
	Logger         *slog.Logger
	LogRequestBody bool
	LogHeaders     []string // headers to log
	SkipPaths      []string // paths to skip logging
}

// statusWriter wraps ResponseWriter to capture status code.
type statusWriter struct {
	http.ResponseWriter
	status int
	size   int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += n
	return n, err
}

// Middleware returns HTTP logging middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	skipSet := make(map[string]bool, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skipSet[p] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if skipSet[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: 200}

			next.ServeHTTP(sw, r)

			duration := time.Since(start)

			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Duration("duration", duration),
				slog.Int("size", sw.size),
				slog.String("remote_addr", r.RemoteAddr),
			}

			if r.URL.RawQuery != "" {
				attrs = append(attrs, slog.String("query", r.URL.RawQuery))
			}

			for _, h := range cfg.LogHeaders {
				if v := r.Header.Get(h); v != "" {
					attrs = append(attrs, slog.String("header."+h, v))
				}
			}

			args := make([]any, len(attrs))
			for i, a := range attrs {
				args[i] = a
			}

			if sw.status >= 500 {
				logger.Error("http request", args...)
			} else if sw.status >= 400 {
				logger.Warn("http request", args...)
			} else {
				logger.Info("http request", args...)
			}
		})
	}
}

// Simple returns a minimal logging middleware.
func Simple(next http.Handler) http.Handler {
	return Middleware(Config{})(next)
}
