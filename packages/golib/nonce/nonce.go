// Package nonce provides cryptographic nonce generation and
// one-time-use nonce tracking. Prevents replay attacks by ensuring
// each nonce is used exactly once within its TTL.
package nonce

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"
)

// Generate creates a cryptographic nonce of the given byte length.
func Generate(bytes int) (string, error) {
	if bytes < 8 {
		bytes = 16
	}
	b := make([]byte, bytes)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// Generate16 creates a 16-byte (128-bit) nonce.
func Generate16() (string, error) {
	return Generate(16)
}

// Generate32 creates a 32-byte (256-bit) nonce.
func Generate32() (string, error) {
	return Generate(32)
}

type entry struct {
	expiresAt time.Time
}

// Store tracks used nonces for replay prevention.
type Store struct {
	mu    sync.Mutex
	used  map[string]entry
	ttl   time.Duration
}

// NewStore creates a nonce store with the given TTL.
func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &Store{
		used: make(map[string]entry),
		ttl:  ttl,
	}
}

// Use marks a nonce as used. Returns false if already used or expired.
func (s *Store) Use(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.used[nonce]; exists {
		return false // replay detected
	}

	s.used[nonce] = entry{expiresAt: time.Now().Add(s.ttl)}
	return true
}

// IsUsed checks if a nonce has been used.
func (s *Store) IsUsed(nonce string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.used[nonce]
	return exists
}

// Purge removes expired nonces. Returns count removed.
func (s *Store) Purge() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0
	for k, e := range s.used {
		if now.After(e.expiresAt) {
			delete(s.used, k)
			removed++
		}
	}
	return removed
}

// Count returns the number of tracked nonces.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.used)
}
