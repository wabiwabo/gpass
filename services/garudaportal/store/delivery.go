package store

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrDeliveryNotFound = errors.New("webhook delivery not found")
)

// WebhookDelivery represents a webhook delivery attempt.
type WebhookDelivery struct {
	ID               string
	SubscriptionID   string
	EventType        string
	Payload          string
	Status           string
	Attempts         int
	LastResponseCode int
	LastResponseBody string
	NextRetryAt      *time.Time
	DeliveredAt      *time.Time
	CreatedAt        time.Time
}

// DeliveryStore defines the interface for webhook delivery persistence.
type DeliveryStore interface {
	Create(delivery *WebhookDelivery) (*WebhookDelivery, error)
	GetByID(id string) (*WebhookDelivery, error)
	UpdateStatus(id, status string, responseCode int, responseBody string, nextRetryAt *time.Time) error
	ListPending() ([]*WebhookDelivery, error)
}

// InMemoryDeliveryStore is a thread-safe in-memory implementation of DeliveryStore.
type InMemoryDeliveryStore struct {
	mu         sync.RWMutex
	deliveries map[string]*WebhookDelivery
}

// NewInMemoryDeliveryStore creates a new in-memory delivery store.
func NewInMemoryDeliveryStore() *InMemoryDeliveryStore {
	return &InMemoryDeliveryStore{
		deliveries: make(map[string]*WebhookDelivery),
	}
}

func copyDelivery(d *WebhookDelivery) *WebhookDelivery {
	cp := *d
	if d.NextRetryAt != nil {
		t := *d.NextRetryAt
		cp.NextRetryAt = &t
	}
	if d.DeliveredAt != nil {
		t := *d.DeliveredAt
		cp.DeliveredAt = &t
	}
	return &cp
}

// Create stores a new webhook delivery.
func (s *InMemoryDeliveryStore) Create(delivery *WebhookDelivery) (*WebhookDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delivery.ID = uuid.New().String()
	delivery.CreatedAt = time.Now().UTC()
	if delivery.Status == "" {
		delivery.Status = "PENDING"
	}

	s.deliveries[delivery.ID] = copyDelivery(delivery)
	return copyDelivery(delivery), nil
}

// GetByID retrieves a delivery by ID.
func (s *InMemoryDeliveryStore) GetByID(id string) (*WebhookDelivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.deliveries[id]
	if !ok {
		return nil, ErrDeliveryNotFound
	}
	return copyDelivery(d), nil
}

// UpdateStatus updates the status of a delivery.
func (s *InMemoryDeliveryStore) UpdateStatus(id, status string, responseCode int, responseBody string, nextRetryAt *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deliveries[id]
	if !ok {
		return ErrDeliveryNotFound
	}

	d.Status = status
	d.LastResponseCode = responseCode
	d.LastResponseBody = responseBody
	d.Attempts++

	if nextRetryAt != nil {
		t := *nextRetryAt
		d.NextRetryAt = &t
	} else {
		d.NextRetryAt = nil
	}

	if status == "DELIVERED" {
		now := time.Now().UTC()
		d.DeliveredAt = &now
	}

	return nil
}

// ListPending returns all deliveries with status PENDING and next_retry_at <= now.
func (s *InMemoryDeliveryStore) ListPending() ([]*WebhookDelivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now().UTC()
	var result []*WebhookDelivery
	for _, d := range s.deliveries {
		if d.Status != "PENDING" {
			continue
		}
		// Include if no retry time set (immediate) or retry time has passed
		if d.NextRetryAt == nil || !d.NextRetryAt.After(now) {
			result = append(result, copyDelivery(d))
		}
	}
	return result, nil
}
