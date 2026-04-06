// Package mwidempotent provides HTTP idempotency middleware.
// Caches responses by idempotency key header, returning cached
// responses for duplicate requests without re-executing handlers.
package mwidempotent

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

type cachedResponse struct {
	status  int
	headers http.Header
	body    []byte
	created time.Time
}

// Config controls idempotency behavior.
type Config struct {
	HeaderName string        // default "Idempotency-Key"
	TTL        time.Duration // how long to cache responses
	Methods    []string      // HTTP methods to apply (default POST, PUT, PATCH)
}

// DefaultConfig returns production defaults.
func DefaultConfig() Config {
	return Config{
		HeaderName: "Idempotency-Key",
		TTL:        24 * time.Hour,
		Methods:    []string{"POST", "PUT", "PATCH"},
	}
}

type responseCapture struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.status = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

// Middleware returns idempotency middleware.
func Middleware(cfg Config) func(http.Handler) http.Handler {
	if cfg.HeaderName == "" {
		cfg.HeaderName = "Idempotency-Key"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}

	methodSet := make(map[string]bool, len(cfg.Methods))
	for _, m := range cfg.Methods {
		methodSet[m] = true
	}
	if len(methodSet) == 0 {
		methodSet["POST"] = true
		methodSet["PUT"] = true
		methodSet["PATCH"] = true
	}

	var mu sync.Mutex
	cache := make(map[string]cachedResponse)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !methodSet[r.Method] {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get(cfg.HeaderName)
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			mu.Lock()
			if cached, ok := cache[key]; ok && time.Since(cached.created) < cfg.TTL {
				mu.Unlock()
				for k, vals := range cached.headers {
					for _, v := range vals {
						w.Header().Add(k, v)
					}
				}
				w.WriteHeader(cached.status)
				w.Write(cached.body)
				return
			}
			mu.Unlock()

			rc := &responseCapture{ResponseWriter: w, status: 200}
			next.ServeHTTP(rc, r)

			mu.Lock()
			cache[key] = cachedResponse{
				status:  rc.status,
				headers: w.Header().Clone(),
				body:    rc.body.Bytes(),
				created: time.Now(),
			}
			mu.Unlock()
		})
	}
}
