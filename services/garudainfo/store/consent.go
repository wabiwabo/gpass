package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors for consent operations.
var (
	ErrConsentNotFound = errors.New("consent not found")
	ErrConsentRevoked  = errors.New("consent already revoked")
)

// Consent represents a user's consent grant for field-level data sharing.
type Consent struct {
	ID              string
	UserID          string
	ClientID        string
	ClientName      string
	Purpose         string
	Fields          map[string]bool // e.g. {"name": true, "dob": true, "address": false}
	DurationSeconds int64
	GrantedAt       time.Time
	ExpiresAt       time.Time
	RevokedAt       *time.Time
	Status          string // ACTIVE, EXPIRED, REVOKED
}

// ConsentStore defines the interface for consent persistence.
type ConsentStore interface {
	Create(ctx context.Context, c *Consent) error
	GetByID(ctx context.Context, id string) (*Consent, error)
	ListByUser(ctx context.Context, userID string) ([]*Consent, error)
	ListActiveByUserAndClient(ctx context.Context, userID, clientID string) ([]*Consent, error)
	Revoke(ctx context.Context, id string) error
	ExpireStale(ctx context.Context) (int, error)
}

// InMemoryConsentStore is a thread-safe in-memory implementation of ConsentStore.
type InMemoryConsentStore struct {
	mu       sync.RWMutex
	consents map[string]*Consent
}

// NewInMemoryConsentStore creates a new in-memory consent store.
func NewInMemoryConsentStore() *InMemoryConsentStore {
	return &InMemoryConsentStore{
		consents: make(map[string]*Consent),
	}
}

func copyConsent(c *Consent) *Consent {
	cp := *c
	cp.Fields = make(map[string]bool, len(c.Fields))
	for k, v := range c.Fields {
		cp.Fields[k] = v
	}
	if c.RevokedAt != nil {
		t := *c.RevokedAt
		cp.RevokedAt = &t
	}
	return &cp
}

// Create stores a new consent, auto-setting ID, Status, GrantedAt, and ExpiresAt.
func (s *InMemoryConsentStore) Create(_ context.Context, c *Consent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	c.ID = uuid.New().String()
	c.Status = "ACTIVE"
	c.GrantedAt = now
	c.ExpiresAt = now.Add(time.Duration(c.DurationSeconds) * time.Second)

	s.consents[c.ID] = copyConsent(c)
	return nil
}

// GetByID retrieves a consent by its ID.
func (s *InMemoryConsentStore) GetByID(_ context.Context, id string) (*Consent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c, ok := s.consents[id]
	if !ok {
		return nil, ErrConsentNotFound
	}
	return copyConsent(c), nil
}

// ListByUser returns all consents for a given user ID.
func (s *InMemoryConsentStore) ListByUser(_ context.Context, userID string) ([]*Consent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Consent
	for _, c := range s.consents {
		if c.UserID == userID {
			result = append(result, copyConsent(c))
		}
	}
	return result, nil
}

// ListActiveByUserAndClient returns active consents for a given user and client.
func (s *InMemoryConsentStore) ListActiveByUserAndClient(_ context.Context, userID, clientID string) ([]*Consent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Consent
	for _, c := range s.consents {
		if c.UserID == userID && c.ClientID == clientID && c.Status == "ACTIVE" {
			result = append(result, copyConsent(c))
		}
	}
	return result, nil
}

// Revoke sets a consent's status to REVOKED. Returns ErrConsentRevoked if already revoked.
func (s *InMemoryConsentStore) Revoke(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	c, ok := s.consents[id]
	if !ok {
		return ErrConsentNotFound
	}
	if c.Status == "REVOKED" {
		return ErrConsentRevoked
	}

	now := time.Now().UTC()
	c.Status = "REVOKED"
	c.RevokedAt = &now
	return nil
}

// ExpireStale finds all ACTIVE consents with ExpiresAt < now, sets them to EXPIRED, and returns the count.
func (s *InMemoryConsentStore) ExpireStale(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	count := 0
	for _, c := range s.consents {
		if c.Status == "ACTIVE" && c.ExpiresAt.Before(now) {
			c.Status = "EXPIRED"
			count++
		}
	}
	return count, nil
}

// ExpireConsentForTest is a test helper that forces a consent to EXPIRED status.
func (s *InMemoryConsentStore) ExpireConsentForTest(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.consents[id]; ok {
		c.Status = "EXPIRED"
	}
}
