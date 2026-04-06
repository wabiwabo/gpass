// Package mwratelimit provides HTTP rate limiting middleware.
// Limits requests per client (by IP or header) using token bucket
// algorithm. Returns 429 Too Many Requests with Retry-After header.
package mwratelimit

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	tokens   float64
	lastTime time.Time
}

// Config controls rate limiting behavior.
type Config struct {
	Rate       float64 // requests per second
	Burst      int     // max burst
	KeyFunc    func(*http.Request) string // extracts client key
	StatusCode int     // response code (default 429)
}

// DefaultConfig returns defaults using client IP.
func DefaultConfig(rate float64, burst int) Config {
	return Config{
		Rate:  rate,
		Burst: burst,
		KeyFunc: func(r *http.Request) string {
			return r.RemoteAddr
		},
		StatusCode: http.StatusTooManyRequests,
	}
}

// Middleware returns rate limiting middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.StatusCode == 0 {
		cfg.StatusCode = http.StatusTooManyRequests
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = func(r *http.Request) string { return r.RemoteAddr }
	}

	var mu sync.Mutex
	buckets := make(map[string]*bucket)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := cfg.KeyFunc(r)

			mu.Lock()
			b, ok := buckets[key]
			if !ok {
				b = &bucket{tokens: float64(cfg.Burst), lastTime: time.Now()}
				buckets[key] = b
			}

			// Refill
			now := time.Now()
			elapsed := now.Sub(b.lastTime).Seconds()
			b.tokens += elapsed * cfg.Rate
			if b.tokens > float64(cfg.Burst) {
				b.tokens = float64(cfg.Burst)
			}
			b.lastTime = now

			if b.tokens < 1 {
				retryAfter := (1 - b.tokens) / cfg.Rate
				mu.Unlock()

				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfter))
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", cfg.Rate))
				w.Header().Set("X-RateLimit-Remaining", "0")
				http.Error(w, `{"error":"rate_limited","message":"too many requests"}`, cfg.StatusCode)
				return
			}

			b.tokens--
			remaining := int(b.tokens)
			mu.Unlock()

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%.0f", cfg.Rate))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

			next.ServeHTTP(w, r)
		})
	}
}
