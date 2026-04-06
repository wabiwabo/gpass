// Package idemkey provides idempotency key management for API
// requests. Ensures that retried requests with the same key
// produce the same response, preventing duplicate operations.
package idemkey

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Result stores a cached response for an idempotent request.
type Result struct {
	StatusCode int             `json:"status_code"`
	Body       json.RawMessage `json:"body"`
	CreatedAt  time.Time       `json:"created_at"`
	ExpiresAt  time.Time       `json:"expires_at"`
}

// IsExpired checks if the cached result has expired.
func (r Result) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

// Store manages idempotency keys and their cached results.
type Store struct {
	mu      sync.Mutex
	results map[string]Result
	ttl     time.Duration
}

// NewStore creates an idempotency store.
func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Store{
		results: make(map[string]Result),
		ttl:     ttl,
	}
}

// Check returns the cached result for a key, if it exists and hasn't expired.
func (s *Store) Check(key string) (Result, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.results[key]
	if !ok {
		return Result{}, false
	}
	if r.IsExpired() {
		delete(s.results, key)
		return Result{}, false
	}
	return r, true
}

// Store saves a result for the given key.
func (s *Store) Save(key string, statusCode int, body []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	s.results[key] = Result{
		StatusCode: statusCode,
		Body:       json.RawMessage(body),
		CreatedAt:  now,
		ExpiresAt:  now.Add(s.ttl),
	}
}

// Delete removes a cached result.
func (s *Store) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.results, key)
}

// Purge removes all expired results. Returns count removed.
func (s *Store) Purge() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0
	for k, r := range s.results {
		if now.After(r.ExpiresAt) {
			delete(s.results, k)
			removed++
		}
	}
	return removed
}

// Count returns the number of stored results.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.results)
}

// GenerateKey creates an idempotency key from request components.
// Uses SHA-256 to create a deterministic key from method, path, and body.
func GenerateKey(method, path string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(method))
	h.Write([]byte(":"))
	h.Write([]byte(path))
	h.Write([]byte(":"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateKey checks if an idempotency key has a valid format.
// Keys must be non-empty and at most 256 characters.
func ValidateKey(key string) error {
	if key == "" {
		return fmt.Errorf("idemkey: empty key")
	}
	if len(key) > 256 {
		return fmt.Errorf("idemkey: key exceeds 256 characters")
	}
	return nil
}
