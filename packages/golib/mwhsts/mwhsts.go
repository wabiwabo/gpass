// Package mwhsts provides HTTP Strict Transport Security middleware.
// Sets HSTS header with configurable max-age, includeSubDomains,
// and preload directives per OWASP best practices.
package mwhsts

import (
	"fmt"
	"net/http"
)

// Config controls HSTS header values.
type Config struct {
	MaxAge            int  // max-age in seconds
	IncludeSubDomains bool
	Preload           bool
}

// DefaultConfig returns HSTS with 2 year max-age, subdomains, preload.
func DefaultConfig() Config {
	return Config{
		MaxAge:            63072000, // 2 years
		IncludeSubDomains: true,
		Preload:           true,
	}
}

// Middleware returns HSTS middleware with the given config.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = 63072000
	}

	value := fmt.Sprintf("max-age=%d", cfg.MaxAge)
	if cfg.IncludeSubDomains {
		value += "; includeSubDomains"
	}
	if cfg.Preload {
		value += "; preload"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Strict-Transport-Security", value)
			next.ServeHTTP(w, r)
		})
	}
}

// Simple returns middleware with default 2-year preload config.
func Simple(next http.Handler) http.Handler {
	return Middleware(DefaultConfig())(next)
}
