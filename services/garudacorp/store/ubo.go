package store

import (
	"errors"
	"sync"

	"github.com/garudapass/gpass/services/garudacorp/ubo"
)

// Sentinel errors for UBO operations.
var (
	ErrUBONotFound = errors.New("ubo analysis not found")
)

// UBOStore defines the interface for UBO analysis result persistence.
type UBOStore interface {
	Save(result *ubo.AnalysisResult) error
	GetByEntityID(entityID string) (*ubo.AnalysisResult, error)
	ListAll() ([]*ubo.AnalysisResult, error)
}

// InMemoryUBOStore is a thread-safe in-memory implementation of UBOStore.
type InMemoryUBOStore struct {
	mu      sync.RWMutex
	results map[string]*ubo.AnalysisResult // keyed by entity ID
}

// NewInMemoryUBOStore creates a new in-memory UBO store.
func NewInMemoryUBOStore() *InMemoryUBOStore {
	return &InMemoryUBOStore{
		results: make(map[string]*ubo.AnalysisResult),
	}
}

func copyAnalysisResult(r *ubo.AnalysisResult) *ubo.AnalysisResult {
	cp := *r
	cp.BeneficialOwners = make([]ubo.BeneficialOwner, len(r.BeneficialOwners))
	copy(cp.BeneficialOwners, r.BeneficialOwners)
	return &cp
}

// Save stores or overwrites a UBO analysis result for an entity.
func (s *InMemoryUBOStore) Save(result *ubo.AnalysisResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.results[result.EntityID] = copyAnalysisResult(result)
	return nil
}

// GetByEntityID retrieves a UBO analysis result by entity ID.
func (s *InMemoryUBOStore) GetByEntityID(entityID string) (*ubo.AnalysisResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.results[entityID]
	if !ok {
		return nil, ErrUBONotFound
	}
	return copyAnalysisResult(r), nil
}

// ListAll returns all stored UBO analysis results.
func (s *InMemoryUBOStore) ListAll() ([]*ubo.AnalysisResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]*ubo.AnalysisResult, 0, len(s.results))
	for _, r := range s.results {
		all = append(all, copyAnalysisResult(r))
	}
	return all, nil
}
