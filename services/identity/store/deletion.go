package store

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("record not found")

// ErrInvalidReason is returned when the deletion reason is not valid.
var ErrInvalidReason = errors.New("invalid deletion reason")

// ValidReasons are the accepted reasons for a deletion request.
var ValidReasons = map[string]bool{
	"user_request":       true,
	"consent_revocation": true,
	"retention_expired":  true,
}

// DeletionRequest represents a data deletion request per UU PDP No. 27/2022.
type DeletionRequest struct {
	ID          string
	UserID      string
	Reason      string // user_request, consent_revocation, retention_expired
	Status      string // PENDING, PROCESSING, COMPLETED, FAILED
	RequestedAt time.Time
	CompletedAt *time.Time
	DeletedData []string // categories deleted: ["personal_info", "biometric", "contact"]
}

// DeletionStore provides deletion request persistence operations.
type DeletionStore interface {
	Create(req *DeletionRequest) error
	GetByID(id string) (*DeletionRequest, error)
	ListByUser(userID string) ([]*DeletionRequest, error)
	UpdateStatus(id, status string, completedAt *time.Time, deletedData []string) error
}

// InMemoryDeletionStore implements DeletionStore for testing.
type InMemoryDeletionStore struct {
	mu       sync.RWMutex
	requests map[string]*DeletionRequest // keyed by ID
}

// NewInMemoryDeletionStore creates a new in-memory deletion store.
func NewInMemoryDeletionStore() *InMemoryDeletionStore {
	return &InMemoryDeletionStore{
		requests: make(map[string]*DeletionRequest),
	}
}

// Create stores a new deletion request after validating the reason.
func (s *InMemoryDeletionStore) Create(req *DeletionRequest) error {
	if err := ValidateDeletionRequest(req); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id, err := generateID()
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}

	req.ID = id
	req.Status = "PENDING"
	req.RequestedAt = time.Now().UTC()

	s.requests[req.ID] = copyDeletionRequest(req)
	return nil
}

// GetByID retrieves a deletion request by its ID.
func (s *InMemoryDeletionStore) GetByID(id string) (*DeletionRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.requests[id]
	if !ok {
		return nil, ErrNotFound
	}
	return copyDeletionRequest(r), nil
}

// ListByUser retrieves all deletion requests for a given user.
func (s *InMemoryDeletionStore) ListByUser(userID string) ([]*DeletionRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*DeletionRequest
	for _, r := range s.requests {
		if r.UserID == userID {
			result = append(result, copyDeletionRequest(r))
		}
	}
	return result, nil
}

// UpdateStatus updates the status, completion time, and deleted data categories.
func (s *InMemoryDeletionStore) UpdateStatus(id, status string, completedAt *time.Time, deletedData []string) error {
	if err := ValidateStatusUpdate(status, deletedData); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.requests[id]
	if !ok {
		return ErrNotFound
	}

	r.Status = status
	if completedAt != nil {
		t := *completedAt
		r.CompletedAt = &t
	}
	if deletedData != nil {
		r.DeletedData = make([]string, len(deletedData))
		copy(r.DeletedData, deletedData)
	}
	return nil
}

func copyDeletionRequest(r *DeletionRequest) *DeletionRequest {
	cp := *r
	if r.CompletedAt != nil {
		t := *r.CompletedAt
		cp.CompletedAt = &t
	}
	if r.DeletedData != nil {
		cp.DeletedData = make([]string, len(r.DeletedData))
		copy(cp.DeletedData, r.DeletedData)
	}
	return &cp
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Format as UUID v4.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	), nil
}
