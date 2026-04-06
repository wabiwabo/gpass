package middleware

import (
	"fmt"
	"net/http"
	"time"
)

// RateLimitHeaders returns middleware that adds standard rate limit headers to responses.
// Headers (per IETF draft-ietf-httpapi-ratelimit-headers-07):
//
//	RateLimit-Limit: <max requests per window>
//	RateLimit-Remaining: <remaining requests in current window>
//	RateLimit-Reset: <seconds until window resets>
//	Retry-After: <seconds> (only on 429 responses)
func RateLimitHeaders(limiter *TieredRateLimiter, keyFunc func(*http.Request) string, tierFunc func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			tier := tierFunc(r)

			allowed, remaining, resetAt := limiter.Allow(key, tier)

			// Look up tier config for the limit value.
			config, ok := limiter.tiers[tier]
			if !ok {
				config = limiter.tiers["free"]
			}

			// Calculate seconds until reset.
			now := limiter.now()
			resetSeconds := int(resetAt.Sub(now).Seconds())
			if resetSeconds < 0 {
				resetSeconds = 0
			}

			// Set standard rate limit headers.
			w.Header().Set("RateLimit-Limit", fmt.Sprintf("%d", config.DailyLimit))
			w.Header().Set("RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("RateLimit-Reset", fmt.Sprintf("%d", resetSeconds))

			if !allowed {
				if resetSeconds == 0 {
					resetSeconds = 1
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", resetSeconds))
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// rateLimitNow is a helper for tests to override time in the limiter.
func rateLimitNow(l *TieredRateLimiter) time.Time {
	return l.now()
}
