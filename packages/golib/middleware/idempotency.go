package middleware

import (
	"bytes"
	"net/http"
	"sync"
	"time"
)

// IdempotencyStore stores idempotency keys and their responses.
type IdempotencyStore interface {
	Get(key string) (*IdempotencyEntry, error)
	Set(key string, entry *IdempotencyEntry, ttl time.Duration) error
}

// IdempotencyEntry represents a cached response for an idempotency key.
type IdempotencyEntry struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	CreatedAt  time.Time
}

// InMemoryIdempotencyStore is an in-memory implementation of IdempotencyStore for testing.
type InMemoryIdempotencyStore struct {
	mu      sync.RWMutex
	entries map[string]*idempotencyRecord
}

type idempotencyRecord struct {
	entry     *IdempotencyEntry
	expiresAt time.Time
}

// NewInMemoryIdempotencyStore creates a new in-memory idempotency store.
func NewInMemoryIdempotencyStore() *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{
		entries: make(map[string]*idempotencyRecord),
	}
}

// Get retrieves an idempotency entry by key. Returns nil if not found or expired.
func (s *InMemoryIdempotencyStore) Get(key string) (*IdempotencyEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rec, ok := s.entries[key]
	if !ok {
		return nil, nil
	}
	if time.Now().After(rec.expiresAt) {
		return nil, nil
	}
	return rec.entry, nil
}

// Set stores an idempotency entry with a TTL.
func (s *InMemoryIdempotencyStore) Set(key string, entry *IdempotencyEntry, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[key] = &idempotencyRecord{
		entry:     entry,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

// idempotencyResponseWriter captures the response for caching.
type idempotencyResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
	written    bool
}

func (w *idempotencyResponseWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *idempotencyResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.statusCode = http.StatusOK
		w.written = true
	}
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Idempotency returns middleware that caches responses by Idempotency-Key header.
// - If key seen before and within TTL, return cached response (same status + body)
// - If key not seen, execute handler, cache response, return normally
// - If no Idempotency-Key header, pass through without caching
// - Only applies to POST/PUT/PATCH methods (not GET/DELETE)
func Idempotency(store IdempotencyStore, ttl time.Duration) func(http.Handler) http.Handler {
	// inflight tracks keys currently being processed to prevent concurrent execution.
	var mu sync.Mutex
	inflight := make(map[string]*sync.WaitGroup)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only apply to mutating methods.
			method := r.Method
			if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch {
				next.ServeHTTP(w, r)
				return
			}

			key := r.Header.Get("Idempotency-Key")
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Check for cached response.
			entry, err := store.Get(key)
			if err == nil && entry != nil {
				for k, v := range entry.Headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(entry.StatusCode)
				w.Write(entry.Body)
				return
			}

			// Check if another goroutine is processing this key.
			mu.Lock()
			if wg, ok := inflight[key]; ok {
				mu.Unlock()
				// Wait for the other goroutine to finish.
				wg.Wait()
				// Now the entry should be cached; serve from cache.
				entry, err := store.Get(key)
				if err == nil && entry != nil {
					for k, v := range entry.Headers {
						w.Header().Set(k, v)
					}
					w.WriteHeader(entry.StatusCode)
					w.Write(entry.Body)
					return
				}
				// Fallback: execute handler if cache miss after wait.
				next.ServeHTTP(w, r)
				return
			}

			// Mark this key as in-flight.
			wg := &sync.WaitGroup{}
			wg.Add(1)
			inflight[key] = wg
			mu.Unlock()

			defer func() {
				mu.Lock()
				delete(inflight, key)
				mu.Unlock()
				wg.Done()
			}()

			// Execute the handler and capture the response.
			rec := &idempotencyResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			next.ServeHTTP(rec, r)

			// Cache the response.
			headers := make(map[string]string)
			for k, v := range rec.Header() {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}
			store.Set(key, &IdempotencyEntry{
				StatusCode: rec.statusCode,
				Headers:    headers,
				Body:       rec.body.Bytes(),
				CreatedAt:  time.Now(),
			}, ttl)
		})
	}
}
