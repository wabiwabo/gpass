package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimit implements a simple in-process token bucket rate limiter
// per IP address. This is defense-in-depth alongside Kong's rate limiting.
//
// For a single BFF instance, this protects against request floods that
// bypass Kong (e.g., direct access). For multiple instances behind a
// load balancer, Kong's rate limiting is the primary protection.
func RateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	limiter := newIPLimiter(requestsPerMinute)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			if !limiter.allow(ip) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, `{"error":"rate_limited","message":"Too many requests, please try again later"}`, http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type ipLimiter struct {
	mu       sync.Mutex
	visitors map[string]*bucket
	rpm      int
}

type bucket struct {
	tokens    int
	lastReset time.Time
}

func newIPLimiter(rpm int) *ipLimiter {
	l := &ipLimiter{
		visitors: make(map[string]*bucket),
		rpm:      rpm,
	}
	// Cleanup stale entries every 5 minutes
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			l.cleanup()
		}
	}()
	return l
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, exists := l.visitors[ip]
	if !exists {
		l.visitors[ip] = &bucket{tokens: l.rpm - 1, lastReset: time.Now()}
		return true
	}

	// Reset tokens if a minute has passed
	if time.Since(b.lastReset) >= time.Minute {
		b.tokens = l.rpm - 1
		b.lastReset = time.Now()
		return true
	}

	if b.tokens <= 0 {
		return false
	}

	b.tokens--
	return true
}

func (l *ipLimiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-5 * time.Minute)
	for ip, b := range l.visitors {
		if b.lastReset.Before(cutoff) {
			delete(l.visitors, ip)
		}
	}
}

func extractIP(r *http.Request) string {
	// Trust X-Forwarded-For from Kong/reverse proxy
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (client IP)
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	// Fall back to RemoteAddr (strip port)
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
