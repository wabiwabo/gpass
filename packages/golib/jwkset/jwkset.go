// Package jwkset provides JSON Web Key Set (JWKS) management
// for key rotation, kid-based key lookup, and JWK formatting.
// Supports the JWKS endpoint pattern for OpenID Connect.
package jwkset

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Key represents a JSON Web Key.
type Key struct {
	KID       string   `json:"kid"`
	KTY       string   `json:"kty"`
	Use       string   `json:"use,omitempty"`
	Alg       string   `json:"alg,omitempty"`
	N         string   `json:"n,omitempty"`         // RSA modulus
	E         string   `json:"e,omitempty"`         // RSA exponent
	Crv       string   `json:"crv,omitempty"`       // EC curve
	X         string   `json:"x,omitempty"`         // EC x coordinate
	Y         string   `json:"y,omitempty"`         // EC y coordinate
	ExpiresAt time.Time `json:"-"`                  // rotation expiry
}

// IsExpired checks if the key has passed its rotation expiry.
func (k Key) IsExpired() bool {
	if k.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(k.ExpiresAt)
}

// Set is a JSON Web Key Set.
type Set struct {
	mu   sync.RWMutex
	keys []Key
}

// New creates an empty JWKS.
func New() *Set {
	return &Set{}
}

// Add adds a key to the set.
func (s *Set) Add(key Key) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = append(s.keys, key)
}

// Remove removes a key by KID.
func (s *Set) Remove(kid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, k := range s.keys {
		if k.KID == kid {
			s.keys = append(s.keys[:i], s.keys[i+1:]...)
			return true
		}
	}
	return false
}

// Get returns a key by KID.
func (s *Set) Get(kid string) (Key, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, k := range s.keys {
		if k.KID == kid {
			return k, true
		}
	}
	return Key{}, false
}

// Current returns the most recently added non-expired key.
func (s *Set) Current() (Key, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.keys) - 1; i >= 0; i-- {
		if !s.keys[i].IsExpired() {
			return s.keys[i], true
		}
	}
	return Key{}, false
}

// Keys returns all non-expired keys.
func (s *Set) Keys() []Key {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Key, 0, len(s.keys))
	for _, k := range s.keys {
		if !k.IsExpired() {
			result = append(result, k)
		}
	}
	return result
}

// AllKeys returns all keys including expired ones.
func (s *Set) AllKeys() []Key {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Key, len(s.keys))
	copy(result, s.keys)
	return result
}

// Len returns the total number of keys.
func (s *Set) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.keys)
}

// Purge removes all expired keys.
func (s *Set) Purge() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]Key, 0, len(s.keys))
	removed := 0
	for _, k := range s.keys {
		if k.IsExpired() {
			removed++
		} else {
			kept = append(kept, k)
		}
	}
	s.keys = kept
	return removed
}

// KIDs returns all key IDs sorted alphabetically.
func (s *Set) KIDs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	kids := make([]string, len(s.keys))
	for i, k := range s.keys {
		kids[i] = k.KID
	}
	sort.Strings(kids)
	return kids
}

// jwksResponse is the JWKS JSON format.
type jwksResponse struct {
	Keys []Key `json:"keys"`
}

// Handler returns an HTTP handler serving the JWKS endpoint.
func (s *Set) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=900")
		json.NewEncoder(w).Encode(jwksResponse{Keys: s.Keys()})
	}
}

// JSON returns the JWKS as JSON bytes.
func (s *Set) JSON() ([]byte, error) {
	return json.Marshal(jwksResponse{Keys: s.Keys()})
}
