package store

import (
	"fmt"
	"sync"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// DocumentStore defines the interface for signed document storage operations.
type DocumentStore interface {
	Create(doc *signing.SignedDocument) (*signing.SignedDocument, error)
	GetByRequestID(requestID string) (*signing.SignedDocument, error)
}

// InMemoryDocumentStore is an in-memory implementation of DocumentStore.
type InMemoryDocumentStore struct {
	mu   sync.RWMutex
	docs map[string]*signing.SignedDocument // keyed by request ID
	byID map[string]*signing.SignedDocument
}

// NewInMemoryDocumentStore creates a new in-memory document store.
func NewInMemoryDocumentStore() *InMemoryDocumentStore {
	return &InMemoryDocumentStore{
		docs: make(map[string]*signing.SignedDocument),
		byID: make(map[string]*signing.SignedDocument),
	}
}

func (s *InMemoryDocumentStore) Create(doc *signing.SignedDocument) (*signing.SignedDocument, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.docs[doc.RequestID]; exists {
		return nil, fmt.Errorf("signed document already exists for request: %s", doc.RequestID)
	}

	doc.ID = generateID()
	doc.CreatedAt = time.Now()

	s.docs[doc.RequestID] = doc
	s.byID[doc.ID] = doc

	return doc, nil
}

func (s *InMemoryDocumentStore) GetByRequestID(requestID string) (*signing.SignedDocument, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	doc, ok := s.docs[requestID]
	if !ok {
		return nil, fmt.Errorf("signed document not found for request: %s", requestID)
	}
	return doc, nil
}
