// Package par implements Pushed Authorization Requests (PAR) per
// RFC 9126. Required for FAPI 2.0 compliance. Manages request_uri
// lifecycle with expiration and one-time use.
package par

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"
)

// Request represents a pushed authorization request.
type Request struct {
	RequestURI   string            `json:"request_uri"`
	ClientID     string            `json:"client_id"`
	RedirectURI  string            `json:"redirect_uri"`
	ResponseType string            `json:"response_type"`
	Scope        string            `json:"scope"`
	State        string            `json:"state"`
	Nonce        string            `json:"nonce"`
	CodeChallenge       string     `json:"code_challenge,omitempty"`
	CodeChallengeMethod string     `json:"code_challenge_method,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
}

// IsExpired checks if the request has expired.
func (r Request) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

// Store manages PAR request lifecycle.
type Store struct {
	mu       sync.Mutex
	requests map[string]Request
	ttl      time.Duration
}

// NewStore creates a PAR store with the given TTL for requests.
// RFC 9126 recommends short-lived request URIs (60 seconds).
func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &Store{
		requests: make(map[string]Request),
		ttl:      ttl,
	}
}

// Push stores a new authorization request and returns a request_uri.
func (s *Store) Push(req Request) (string, error) {
	uri, err := generateRequestURI()
	if err != nil {
		return "", err
	}

	req.RequestURI = uri
	req.CreatedAt = time.Now().UTC()
	req.ExpiresAt = req.CreatedAt.Add(s.ttl)

	s.mu.Lock()
	s.requests[uri] = req
	s.mu.Unlock()

	return uri, nil
}

// Consume retrieves and deletes a request (one-time use).
// Returns the request and true if found and not expired.
func (s *Store) Consume(requestURI string) (Request, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[requestURI]
	if !ok {
		return Request{}, false
	}

	delete(s.requests, requestURI)

	if req.IsExpired() {
		return Request{}, false
	}

	return req, true
}

// Get retrieves a request without consuming it.
func (s *Store) Get(requestURI string) (Request, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[requestURI]
	if !ok {
		return Request{}, false
	}

	if req.IsExpired() {
		delete(s.requests, requestURI)
		return Request{}, false
	}

	return req, true
}

// Purge removes all expired requests. Returns count removed.
func (s *Store) Purge() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	removed := 0
	for uri, req := range s.requests {
		if req.IsExpired() {
			delete(s.requests, uri)
			removed++
		}
	}
	return removed
}

// Count returns the number of stored requests.
func (s *Store) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func generateRequestURI() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("par: %w", err)
	}
	return "urn:ietf:params:oauth:request_uri:" + hex.EncodeToString(b), nil
}
