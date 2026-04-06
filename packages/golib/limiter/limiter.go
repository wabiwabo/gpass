// Package limiter provides a concurrent-safe rate limiter registry
// for managing multiple named rate limiters. Useful for applying
// different rate limits to different API endpoints or user tiers.
package limiter

import (
	"net/http"
	"sync"
	"time"
)

// Config defines rate limit parameters.
type Config struct {
	Rate     int           // Tokens per interval.
	Burst    int           // Maximum burst.
	Interval time.Duration // Refill interval.
}

// Registry manages named rate limiters.
type Registry struct {
	mu       sync.RWMutex
	limiters map[string]*bucket
	configs  map[string]Config
	fallback Config
}

type bucket struct {
	mu       sync.Mutex
	tokens   int
	lastFill time.Time
	config   Config
}

// NewRegistry creates a limiter registry with a fallback config.
func NewRegistry(fallback Config) *Registry {
	if fallback.Rate <= 0 {
		fallback.Rate = 60
	}
	if fallback.Burst <= 0 {
		fallback.Burst = fallback.Rate
	}
	if fallback.Interval <= 0 {
		fallback.Interval = time.Minute
	}

	return &Registry{
		limiters: make(map[string]*bucket),
		configs:  make(map[string]Config),
		fallback: fallback,
	}
}

// Configure sets rate limit for a specific name (e.g., endpoint, tier).
func (r *Registry) Configure(name string, cfg Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[name] = cfg
}

// Allow checks if a request is allowed for the given name and key.
func (r *Registry) Allow(name, key string) bool {
	compositeKey := name + ":" + key

	r.mu.RLock()
	b, ok := r.limiters[compositeKey]
	r.mu.RUnlock()

	if !ok {
		cfg := r.configFor(name)
		b = &bucket{
			tokens:   cfg.Burst,
			lastFill: time.Now(),
			config:   cfg,
		}
		r.mu.Lock()
		r.limiters[compositeKey] = b
		r.mu.Unlock()
	}

	return b.allow()
}

func (b *bucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastFill)
	refill := int(elapsed/b.config.Interval) * b.config.Rate
	if refill > 0 {
		b.tokens += refill
		if b.tokens > b.config.Burst {
			b.tokens = b.config.Burst
		}
		b.lastFill = now
	}

	if b.tokens > 0 {
		b.tokens--
		return true
	}
	return false
}

func (r *Registry) configFor(name string) Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if cfg, ok := r.configs[name]; ok {
		return cfg
	}
	return r.fallback
}

// Middleware returns HTTP middleware using path as the limiter name
// and remote addr as the key.
func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !r.Allow(req.URL.Path, req.RemoteAddr) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"type":"about:blank","title":"Too Many Requests","status":429}`))
			return
		}
		next.ServeHTTP(w, req)
	})
}

// Size returns the number of tracked keys.
func (r *Registry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.limiters)
}

// ConfigCount returns the number of named configurations.
func (r *Registry) ConfigCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.configs)
}
