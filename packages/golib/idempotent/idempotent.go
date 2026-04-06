package idempotent

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

// Status represents the state of an idempotent request.
type Status string

const (
	StatusProcessing Status = "processing"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// Entry stores the result of a previously processed request.
type Entry struct {
	Key         string          `json:"key"`
	Status      Status          `json:"status"`
	StatusCode  int             `json:"status_code"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        json.RawMessage `json:"body,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// Store persists idempotency entries.
type Store interface {
	// Get retrieves an entry by key. Returns nil if not found.
	Get(ctx context.Context, key string) (*Entry, error)
	// Lock creates a processing entry and returns true if the key was not already present.
	// Returns false if the key already exists (duplicate request).
	Lock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// Complete stores the final result for a key.
	Complete(ctx context.Context, key string, statusCode int, headers map[string]string, body json.RawMessage) error
	// Fail marks an entry as failed, allowing retry.
	Fail(ctx context.Context, key string) error
}

// MemoryStore is an in-memory idempotency store for testing.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]*Entry
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		entries: make(map[string]*Entry),
	}
}

// Get retrieves an entry by key.
func (s *MemoryStore) Get(_ context.Context, key string) (*Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entries[key]
	if !ok {
		return nil, nil
	}
	copy := *e
	return &copy, nil
}

// Lock creates a processing entry. Returns true if successfully locked (new key).
func (s *MemoryStore) Lock(_ context.Context, key string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.entries[key]; ok {
		// Allow retry if previous attempt failed.
		if existing.Status == StatusFailed {
			existing.Status = StatusProcessing
			existing.CreatedAt = time.Now()
			return true, nil
		}
		return false, nil
	}

	s.entries[key] = &Entry{
		Key:       key,
		Status:    StatusProcessing,
		CreatedAt: time.Now(),
	}
	return true, nil
}

// Complete stores the final result.
func (s *MemoryStore) Complete(_ context.Context, key string, statusCode int, headers map[string]string, body json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[key]
	if !ok {
		return nil
	}

	now := time.Now()
	e.Status = StatusCompleted
	e.StatusCode = statusCode
	e.Headers = headers
	e.Body = body
	e.CompletedAt = &now
	return nil
}

// Fail marks an entry as failed.
func (s *MemoryStore) Fail(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if e, ok := s.entries[key]; ok {
		e.Status = StatusFailed
	}
	return nil
}

// Count returns the number of entries.
func (s *MemoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Cleanup removes entries older than maxAge.
func (s *MemoryStore) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for key, e := range s.entries {
		if e.CreatedAt.Before(cutoff) {
			delete(s.entries, key)
			removed++
		}
	}
	return removed
}

// Handler provides HTTP-level idempotency support.
type Handler struct {
	store Store
	ttl   time.Duration
}

// NewHandler creates an idempotency handler.
func NewHandler(store Store, ttl time.Duration) *Handler {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &Handler{store: store, ttl: ttl}
}

// Check attempts to lock the idempotency key.
// Returns (existing entry, is new request, error).
// If is_new is true, the caller should process the request.
// If is_new is false, the caller should replay the existing entry.
func (h *Handler) Check(ctx context.Context, key string) (*Entry, bool, error) {
	if key == "" {
		return nil, true, nil // No idempotency key = always new
	}

	locked, err := h.store.Lock(ctx, key, h.ttl)
	if err != nil {
		return nil, false, err
	}

	if locked {
		return nil, true, nil // New request, proceed
	}

	// Key exists, get the existing entry.
	entry, err := h.store.Get(ctx, key)
	if err != nil {
		return nil, false, err
	}

	return entry, false, nil
}

// Complete records the result of a processed request.
func (h *Handler) Complete(ctx context.Context, key string, statusCode int, headers map[string]string, body json.RawMessage) error {
	if key == "" {
		return nil
	}
	return h.store.Complete(ctx, key, statusCode, headers, body)
}

// Fail marks a request as failed, allowing retry with the same key.
func (h *Handler) Fail(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}
	return h.store.Fail(ctx, key)
}
