// Package maxbody provides HTTP middleware that enforces request body
// size limits to prevent denial-of-service via large payloads.
package maxbody

import (
	"fmt"
	"net/http"
)

// DefaultMaxSize is the default max body size (1MB).
const DefaultMaxSize = 1 * 1024 * 1024

// Middleware returns an HTTP middleware that limits request body size.
// Requests exceeding maxBytes get a 413 Payload Too Large response.
func Middleware(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxSize
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > maxBytes {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				fmt.Fprintf(w, `{"type":"about:blank","title":"Payload Too Large","status":413,"detail":"Request body must not exceed %d bytes"}`, maxBytes)
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// PerRoute returns middleware with different limits per HTTP method.
// Useful for allowing larger bodies on POST/PUT but not GET.
func PerRoute(getLimitBytes, postLimitBytes int64) func(http.Handler) http.Handler {
	if getLimitBytes <= 0 {
		getLimitBytes = 0 // No body expected on GET.
	}
	if postLimitBytes <= 0 {
		postLimitBytes = DefaultMaxSize
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var limit int64
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				limit = getLimitBytes
			default:
				limit = postLimitBytes
			}

			if limit > 0 {
				if r.ContentLength > limit {
					w.Header().Set("Content-Type", "application/problem+json")
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					fmt.Fprintf(w, `{"type":"about:blank","title":"Payload Too Large","status":413,"detail":"Request body must not exceed %d bytes"}`, limit)
					return
				}
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// FormatSize returns a human-readable size string.
func FormatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
