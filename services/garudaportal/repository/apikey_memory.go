package repository

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// InMemoryAPIKeyRepository implements APIKeyRepository for testing.
type InMemoryAPIKeyRepository struct {
	mu     sync.RWMutex
	keys   map[string]*APIKeyRecord // keyed by ID
	byHash map[string]string        // keyHash -> ID
	byApp  map[string][]string      // appID -> []keyID
}

// NewInMemoryAPIKeyRepository creates a new in-memory API key repository.
func NewInMemoryAPIKeyRepository() *InMemoryAPIKeyRepository {
	return &InMemoryAPIKeyRepository{
		keys:   make(map[string]*APIKeyRecord),
		byHash: make(map[string]string),
		byApp:  make(map[string][]string),
	}
}

func (r *InMemoryAPIKeyRepository) Create(_ context.Context, key *APIKeyRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byHash[key.KeyHash]; exists {
		return fmt.Errorf("api key hash already exists")
	}

	id, err := generateUUID()
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}

	now := time.Now().UTC()
	key.ID = id
	key.CreatedAt = now

	stored := copyAPIKey(key)
	r.keys[key.ID] = stored
	r.byHash[key.KeyHash] = key.ID
	r.byApp[key.AppID] = append(r.byApp[key.AppID], key.ID)

	return nil
}

func (r *InMemoryAPIKeyRepository) GetByHash(_ context.Context, keyHash string) (*APIKeyRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byHash[keyHash]
	if !ok {
		return nil, ErrNotFound
	}
	k := r.keys[id]
	return copyAPIKey(k), nil
}

func (r *InMemoryAPIKeyRepository) ListByApp(_ context.Context, appID string) ([]*APIKeyRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byApp[appID]
	keys := make([]*APIKeyRecord, 0, len(ids))
	for _, id := range ids {
		if k, ok := r.keys[id]; ok {
			keys = append(keys, copyAPIKey(k))
		}
	}
	return keys, nil
}

func (r *InMemoryAPIKeyRepository) Revoke(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	k, ok := r.keys[id]
	if !ok || k.Status != "active" {
		return ErrNotFound
	}
	now := time.Now().UTC()
	k.Status = "revoked"
	k.RevokedAt = &now
	return nil
}

func (r *InMemoryAPIKeyRepository) UpdateLastUsed(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	k, ok := r.keys[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now().UTC()
	k.LastUsedAt = &now
	return nil
}

func copyAPIKey(k *APIKeyRecord) *APIKeyRecord {
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
