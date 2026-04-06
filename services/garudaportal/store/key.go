package store

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrKeyNotFound = errors.New("api key not found")
)

// APIKey represents an API key stored in the system.
type APIKey struct {
	ID          string
	AppID       string
	KeyHash     string
	KeyPrefix   string
	Name        string
	Environment string
	Status      string
	LastUsedAt  *time.Time
	RevokedAt   *time.Time
	ExpiresAt   *time.Time
	CreatedAt   time.Time
}

// KeyStore defines the interface for API key persistence.
type KeyStore interface {
	Create(key *APIKey) (*APIKey, error)
	GetByID(id string) (*APIKey, error)
	GetByHash(keyHash string) (*APIKey, error)
	ListByApp(appID string) ([]*APIKey, error)
	Revoke(id string) error
	UpdateLastUsed(id string) error
}

// InMemoryKeyStore is a thread-safe in-memory implementation of KeyStore.
type InMemoryKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*APIKey
}

// NewInMemoryKeyStore creates a new in-memory key store.
func NewInMemoryKeyStore() *InMemoryKeyStore {
	return &InMemoryKeyStore{
		keys: make(map[string]*APIKey),
	}
}

func copyKey(k *APIKey) *APIKey {
	cp := *k
	if k.LastUsedAt != nil {
		t := *k.LastUsedAt
		cp.LastUsedAt = &t
	}
	if k.RevokedAt != nil {
		t := *k.RevokedAt
		cp.RevokedAt = &t
	}
	if k.ExpiresAt != nil {
		t := *k.ExpiresAt
		cp.ExpiresAt = &t
	}
	return &cp
}

// Create stores a new API key.
func (s *InMemoryKeyStore) Create(key *APIKey) (*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key.ID = uuid.New().String()
	key.CreatedAt = time.Now().UTC()
	if key.Status == "" {
		key.Status = "ACTIVE"
	}

	s.keys[key.ID] = copyKey(key)
	return copyKey(key), nil
}

// GetByID retrieves a key by its ID.
func (s *InMemoryKeyStore) GetByID(id string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	k, ok := s.keys[id]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return copyKey(k), nil
}

// GetByHash retrieves a key by its SHA-256 hash.
func (s *InMemoryKeyStore) GetByHash(keyHash string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, k := range s.keys {
		if k.KeyHash == keyHash {
			return copyKey(k), nil
		}
	}
	return nil, ErrKeyNotFound
}

// ListByApp retrieves all keys for a given app.
func (s *InMemoryKeyStore) ListByApp(appID string) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*APIKey
	for _, k := range s.keys {
		if k.AppID == appID {
			result = append(result, copyKey(k))
		}
	}
	return result, nil
}

// Revoke marks a key as revoked.
func (s *InMemoryKeyStore) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	k, ok := s.keys[id]
	if !ok {
		return ErrKeyNotFound
	}

	now := time.Now().UTC()
	k.Status = "REVOKED"
	k.RevokedAt = &now
	return nil
}

// UpdateLastUsed updates the last used timestamp.
func (s *InMemoryKeyStore) UpdateLastUsed(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	k, ok := s.keys[id]
	if !ok {
		return ErrKeyNotFound
	}

	now := time.Now().UTC()
	k.LastUsedAt = &now
	return nil
}
