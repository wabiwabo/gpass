// Package reqlog provides structured HTTP request/response logging
// middleware with configurable field selection, PII filtering,
// and latency tracking.
package reqlog

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Config controls what gets logged.
type Config struct {
	// LogRequestHeaders logs request headers (filtered by AllowedHeaders).
	LogRequestHeaders bool
	// LogResponseHeaders logs response headers.
	LogResponseHeaders bool
	// AllowedHeaders are headers safe to log (lowercase). Empty means log none.
	AllowedHeaders map[string]bool
	// SkipPaths are URL paths that skip logging (e.g., /healthz).
	SkipPaths map[string]bool
	// SlowThreshold marks requests slower than this.
	SlowThreshold time.Duration
	// Logger is the slog logger to use. Defaults to slog.Default().
	Logger *slog.Logger
}

// DefaultConfig returns a production-safe config.
func DefaultConfig() Config {
	return Config{
		LogRequestHeaders:  true,
		LogResponseHeaders: false,
		AllowedHeaders: map[string]bool{
			"content-type":     true,
			"accept":           true,
			"x-request-id":     true,
			"x-correlation-id": true,
			"user-agent":       true,
		},
		SkipPaths: map[string]bool{
			"/healthz":  true,
			"/readyz":   true,
			"/startupz": true,
			"/metrics":  true,
		},
		SlowThreshold: 3 * time.Second,
	}
}

// Middleware returns an HTTP middleware that logs requests.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip configured paths.
			if cfg.SkipPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()

			// Wrap response writer to capture status and size.
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r)

			duration := time.Since(start)

			// Build log attributes.
			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.status),
				slog.Int64("bytes", rw.written),
				slog.Duration("duration", duration),
				slog.String("remote_addr", sanitizeIP(r.RemoteAddr)),
			}

			if r.URL.RawQuery != "" {
				attrs = append(attrs, slog.String("query", r.URL.RawQuery))
			}

			if cfg.LogRequestHeaders && len(cfg.AllowedHeaders) > 0 {
				for name, values := range r.Header {
					if cfg.AllowedHeaders[strings.ToLower(name)] {
						attrs = append(attrs, slog.String("req_"+strings.ToLower(name), strings.Join(values, ", ")))
					}
				}
			}

			// Determine log level.
			level := slog.LevelInfo
			if rw.status >= 500 {
				level = slog.LevelError
			} else if rw.status >= 400 {
				level = slog.LevelWarn
			}

			if duration >= cfg.SlowThreshold {
				attrs = append(attrs, slog.Bool("slow", true))
				if level < slog.LevelWarn {
					level = slog.LevelWarn
				}
			}

			// Log as a group.
			args := make([]any, len(attrs))
			for i, a := range attrs {
				args[i] = a
			}
			logger.LogAttrs(r.Context(), level, "http_request", attrs...)
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	written     int64
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// sanitizeIP strips port from remote address.
func sanitizeIP(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
