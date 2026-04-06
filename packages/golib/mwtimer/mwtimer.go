// Package mwtimer provides middleware that measures and reports
// handler execution time via response headers and structured logging.
package mwtimer

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Config controls timer middleware behavior.
type Config struct {
	// HeaderName is the response header for timing info.
	HeaderName string
	// SlowThreshold logs a warning for slow requests.
	SlowThreshold time.Duration
	// Logger for slow request warnings.
	Logger *slog.Logger
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		HeaderName:    "X-Response-Time",
		SlowThreshold: 3 * time.Second,
	}
}

// Middleware returns HTTP middleware that measures execution time.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "X-Response-Time"
	}
	if cfg.SlowThreshold <= 0 {
		cfg.SlowThreshold = 3 * time.Second
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			next.ServeHTTP(w, r)

			duration := time.Since(start)
			w.Header().Set(cfg.HeaderName, fmt.Sprintf("%.3fms", float64(duration.Microseconds())/1000))

			if duration >= cfg.SlowThreshold {
				logger.Warn("slow request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Duration("duration", duration),
				)
			}
		})
	}
}

// Simple returns a minimal timer middleware with default config.
func Simple(next http.Handler) http.Handler {
	return Middleware(DefaultConfig())(next)
}
