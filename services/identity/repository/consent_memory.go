package repository

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// InMemoryConsentRepository implements ConsentRepository for testing.
type InMemoryConsentRepository struct {
	mu       sync.RWMutex
	consents map[string]*Consent // keyed by ID
}

// NewInMemoryConsentRepository creates a new in-memory consent repository.
func NewInMemoryConsentRepository() *InMemoryConsentRepository {
	return &InMemoryConsentRepository{
		consents: make(map[string]*Consent),
	}
}

func (r *InMemoryConsentRepository) Grant(_ context.Context, consent *Consent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, err := generateUUID()
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}

	now := time.Now().UTC()
	consent.ID = id
	consent.CreatedAt = now

	r.consents[consent.ID] = copyConsent(consent)
	return nil
}

func (r *InMemoryConsentRepository) GetByID(_ context.Context, id string) (*Consent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.consents[id]
	if !ok {
		return nil, ErrNotFound
	}
	return copyConsent(c), nil
}

func (r *InMemoryConsentRepository) ListByUser(_ context.Context, userID string) ([]*Consent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*Consent
	for _, c := range r.consents {
		if c.UserID == userID {
			result = append(result, copyConsent(c))
		}
	}
	return result, nil
}

func (r *InMemoryConsentRepository) Revoke(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.consents[id]
	if !ok {
		return ErrNotFound
	}
	if c.Status != "ACTIVE" {
		return ErrNotFound
	}

	now := time.Now().UTC()
	c.Status = "REVOKED"
	c.RevokedAt = &now
	return nil
}

func (r *InMemoryConsentRepository) ExpireStale(_ context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	count := 0
	for _, c := range r.consents {
		if c.Status == "ACTIVE" && c.ExpiresAt.Before(now) {
			c.Status = "EXPIRED"
			count++
		}
	}
	return count, nil
}

func copyConsent(c *Consent) *Consent {
	cp := *c
	cp.Fields = copyBytes(c.Fields)
	if c.RevokedAt != nil {
		t := *c.RevokedAt
		cp.RevokedAt = &t
	}
	return &cp
}
