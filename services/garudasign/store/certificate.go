package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// CertificateStore defines the interface for certificate storage operations.
type CertificateStore interface {
	Create(cert *signing.Certificate) (*signing.Certificate, error)
	GetByID(id string) (*signing.Certificate, error)
	GetActiveByUser(userID string) (*signing.Certificate, error)
	ListByUser(userID, statusFilter string) ([]*signing.Certificate, error)
	UpdateStatus(id, status string, revokedAt *time.Time, reason string) error
}

// InMemoryCertificateStore is an in-memory implementation of CertificateStore.
type InMemoryCertificateStore struct {
	mu    sync.RWMutex
	certs map[string]*signing.Certificate
}

// NewInMemoryCertificateStore creates a new in-memory certificate store.
func NewInMemoryCertificateStore() *InMemoryCertificateStore {
	return &InMemoryCertificateStore{
		certs: make(map[string]*signing.Certificate),
	}
}

func (s *InMemoryCertificateStore) Create(cert *signing.Certificate) (*signing.Certificate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert.ID = generateID()
	now := time.Now()
	cert.CreatedAt = now
	cert.UpdatedAt = now

	s.certs[cert.ID] = cert
	return cert, nil
}

func (s *InMemoryCertificateStore) GetByID(id string) (*signing.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cert, ok := s.certs[id]
	if !ok {
		return nil, fmt.Errorf("certificate not found: %s", id)
	}
	return cert, nil
}

func (s *InMemoryCertificateStore) GetActiveByUser(userID string) (*signing.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, cert := range s.certs {
		if cert.UserID == userID && cert.Status == "ACTIVE" {
			return cert, nil
		}
	}
	return nil, fmt.Errorf("no active certificate found for user: %s", userID)
}

func (s *InMemoryCertificateStore) ListByUser(userID, statusFilter string) ([]*signing.Certificate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*signing.Certificate
	for _, cert := range s.certs {
		if cert.UserID != userID {
			continue
		}
		if statusFilter != "" && cert.Status != statusFilter {
			continue
		}
		result = append(result, cert)
	}
	return result, nil
}

func (s *InMemoryCertificateStore) UpdateStatus(id, status string, revokedAt *time.Time, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cert, ok := s.certs[id]
	if !ok {
		return fmt.Errorf("certificate not found: %s", id)
	}

	cert.Status = status
	cert.RevokedAt = revokedAt
	cert.RevocationReason = reason
	cert.UpdatedAt = time.Now()

	return nil
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
