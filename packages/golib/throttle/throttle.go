// Package throttle provides a simple token bucket rate limiter with
// burst support and a middleware for HTTP request throttling.
// Unlike the ratelimit package which offers multiple algorithms,
// this package focuses on per-key throttling suitable for
// user/IP-based rate limiting.
package throttle

import (
	"net/http"
	"sync"
	"time"
)

// Limiter provides per-key rate limiting.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // Tokens per interval.
	burst   int           // Max tokens.
	interval time.Duration // Refill interval.
}

type bucket struct {
	tokens    int
	lastFill  time.Time
}

// New creates a per-key limiter.
// rate: tokens added per interval. burst: maximum tokens.
func New(rate, burst int, interval time.Duration) *Limiter {
	if rate <= 0 {
		rate = 10
	}
	if burst <= 0 {
		burst = rate
	}
	if interval <= 0 {
		interval = time.Second
	}

	return &Limiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		burst:    burst,
		interval: interval,
	}
}

// Allow checks if the key has a token available.
// Returns true if allowed, false if rate limited.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, lastFill: time.Now()}
		l.buckets[key] = b
	}

	// Refill tokens based on elapsed time.
	now := time.Now()
	elapsed := now.Sub(b.lastFill)
	refill := int(elapsed / l.interval) * l.rate
	if refill > 0 {
		b.tokens += refill
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.lastFill = now
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

// Remaining returns the number of tokens left for a key.
func (l *Limiter) Remaining(key string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok {
		return l.burst
	}

	// Calculate current tokens with refill.
	elapsed := time.Since(b.lastFill)
	refill := int(elapsed/l.interval) * l.rate
	tokens := b.tokens + refill
	if tokens > l.burst {
		tokens = l.burst
	}
	return tokens
}

// Reset clears the limiter for a specific key.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.buckets, key)
}

// Cleanup removes stale entries that haven't been accessed recently.
func (l *Limiter) Cleanup(maxAge time.Duration) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	removed := 0
	for key, b := range l.buckets {
		if now.Sub(b.lastFill) > maxAge {
			delete(l.buckets, key)
			removed++
		}
	}
	return removed
}

// Size returns the number of tracked keys.
func (l *Limiter) Size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buckets)
}

// KeyFunc extracts the rate-limiting key from a request.
type KeyFunc func(r *http.Request) string

// IPKeyFunc uses the client IP as the rate-limiting key.
func IPKeyFunc(r *http.Request) string {
	return r.RemoteAddr
}

// Middleware returns an HTTP middleware that applies per-key rate limiting.
func (l *Limiter) Middleware(keyFn KeyFunc) func(http.Handler) http.Handler {
	if keyFn == nil {
		keyFn = IPKeyFunc
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFn(r)

			if !l.Allow(key) {
				w.Header().Set("Retry-After", "1")
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"type":"about:blank","title":"Too Many Requests","status":429,"detail":"Rate limit exceeded. Please retry later."}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
