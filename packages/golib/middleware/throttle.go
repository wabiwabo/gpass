package middleware

import (
	"net/http"
	"sync"
)

// Throttle returns middleware that limits concurrent requests to an endpoint.
// Unlike rate limiting (requests per time), throttling limits simultaneous requests.
// Use this to protect expensive operations like document signing or DB-heavy queries.
// When the limit is exceeded, 429 Too Many Requests is returned.
func Throttle(maxConcurrent int) func(http.Handler) http.Handler {
	sem := make(chan struct{}, maxConcurrent)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			default:
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"too_many_requests","message":"too many concurrent requests"}`))
			}
		})
	}
}

// ThrottleByKey returns middleware that limits concurrent requests per key.
// Key is extracted from the request (e.g., user ID, app ID).
// Each unique key gets its own concurrency limit.
func ThrottleByKey(maxConcurrent int, keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
	var mu sync.Mutex
	semaphores := make(map[string]chan struct{})

	getSem := func(key string) chan struct{} {
		mu.Lock()
		defer mu.Unlock()
		sem, ok := semaphores[key]
		if !ok {
			sem = make(chan struct{}, maxConcurrent)
			semaphores[key] = sem
		}
		return sem
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := keyFunc(r)
			sem := getSem(key)

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			default:
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Retry-After", "1")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error":"too_many_requests","message":"too many concurrent requests"}`))
			}
		})
	}
}
