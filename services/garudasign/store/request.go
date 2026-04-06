package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// RequestStore defines the interface for signing request storage operations.
type RequestStore interface {
	Create(req *signing.SigningRequest) (*signing.SigningRequest, error)
	GetByID(id string) (*signing.SigningRequest, error)
	ListByUser(userID string) ([]*signing.SigningRequest, error)
	UpdateStatus(id, status, certificateID, errorMsg string) error
}

// InMemoryRequestStore is an in-memory implementation of RequestStore.
type InMemoryRequestStore struct {
	mu       sync.RWMutex
	requests map[string]*signing.SigningRequest
}

// NewInMemoryRequestStore creates a new in-memory request store.
func NewInMemoryRequestStore() *InMemoryRequestStore {
	return &InMemoryRequestStore{
		requests: make(map[string]*signing.SigningRequest),
	}
}

func (s *InMemoryRequestStore) Create(req *signing.SigningRequest) (*signing.SigningRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req.ID = generateID()
	now := time.Now()
	req.CreatedAt = now
	req.UpdatedAt = now

	s.requests[req.ID] = req
	return req, nil
}

func (s *InMemoryRequestStore) GetByID(id string) (*signing.SigningRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.requests[id]
	if !ok {
		return nil, fmt.Errorf("request not found: %s", id)
	}
	return req, nil
}

func (s *InMemoryRequestStore) ListByUser(userID string) ([]*signing.SigningRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*signing.SigningRequest
	for _, req := range s.requests {
		if req.UserID == userID {
			result = append(result, req)
		}
	}
	return result, nil
}

func (s *InMemoryRequestStore) UpdateStatus(id, status, certificateID, errorMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[id]
	if !ok {
		return fmt.Errorf("request not found: %s", id)
	}

	req.Status = status
	if certificateID != "" {
		req.CertificateID = certificateID
	}
	req.ErrorMessage = errorMsg
	req.UpdatedAt = time.Now()

	return nil
}
