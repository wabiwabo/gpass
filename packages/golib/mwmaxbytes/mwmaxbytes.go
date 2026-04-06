// Package mwmaxbytes provides request body size limiting middleware.
// Prevents memory exhaustion from oversized payloads by enforcing
// a maximum request body size with early rejection.
package mwmaxbytes

import (
	"fmt"
	"net/http"
)

// Config controls body size limiting.
type Config struct {
	MaxBytes int64  // maximum body size
	Message  string // error message
}

// DefaultConfig returns defaults with 1MB limit.
func DefaultConfig() Config {
	return Config{
		MaxBytes: 1 << 20, // 1 MB
		Message:  "request body too large",
	}
}

// Middleware returns body size limiting middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 1 << 20
	}
	if cfg.Message == "" {
		cfg.Message = "request body too large"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > cfg.MaxBytes {
				http.Error(w,
					fmt.Sprintf(`{"error":"payload_too_large","message":"%s","max_bytes":%d}`, cfg.Message, cfg.MaxBytes),
					http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// Simple returns middleware with default 1MB limit.
func Simple(next http.Handler) http.Handler {
	return Middleware(DefaultConfig())(next)
}
