package store

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrWebhookNotFound  = errors.New("webhook subscription not found")
	ErrWebhookURLScheme = errors.New("webhook URL must use https (except localhost for development)")
)

// WebhookSubscription represents a webhook subscription.
type WebhookSubscription struct {
	ID        string
	AppID     string
	URL       string
	Events    []string
	Secret    string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// WebhookStore defines the interface for webhook subscription persistence.
type WebhookStore interface {
	Create(sub *WebhookSubscription) (*WebhookSubscription, error)
	GetByID(id string) (*WebhookSubscription, error)
	ListByApp(appID string) ([]*WebhookSubscription, error)
	ListByEvent(eventType string) ([]*WebhookSubscription, error)
	Disable(id string) error
}

// InMemoryWebhookStore is a thread-safe in-memory implementation of WebhookStore.
type InMemoryWebhookStore struct {
	mu   sync.RWMutex
	subs map[string]*WebhookSubscription
}

// NewInMemoryWebhookStore creates a new in-memory webhook store.
func NewInMemoryWebhookStore() *InMemoryWebhookStore {
	return &InMemoryWebhookStore{
		subs: make(map[string]*WebhookSubscription),
	}
}

func copyWebhook(w *WebhookSubscription) *WebhookSubscription {
	cp := *w
	cp.Events = make([]string, len(w.Events))
	copy(cp.Events, w.Events)
	return &cp
}

// Create stores a new webhook subscription.
func (s *InMemoryWebhookStore) Create(sub *WebhookSubscription) (*WebhookSubscription, error) {
	// Validate URL: must be https unless localhost
	if !isValidWebhookURL(sub.URL) {
		return nil, ErrWebhookURLScheme
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	sub.ID = uuid.New().String()
	sub.CreatedAt = now
	sub.UpdatedAt = now
	if sub.Status == "" {
		sub.Status = "ACTIVE"
	}

	s.subs[sub.ID] = copyWebhook(sub)
	return copyWebhook(sub), nil
}

// GetByID retrieves a subscription by ID.
func (s *InMemoryWebhookStore) GetByID(id string) (*WebhookSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subs[id]
	if !ok {
		return nil, ErrWebhookNotFound
	}
	return copyWebhook(sub), nil
}

// ListByApp retrieves all subscriptions for a given app.
func (s *InMemoryWebhookStore) ListByApp(appID string) ([]*WebhookSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WebhookSubscription
	for _, sub := range s.subs {
		if sub.AppID == appID {
			result = append(result, copyWebhook(sub))
		}
	}
	return result, nil
}

// ListByEvent retrieves all active subscriptions that include the given event type.
func (s *InMemoryWebhookStore) ListByEvent(eventType string) ([]*WebhookSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WebhookSubscription
	for _, sub := range s.subs {
		if sub.Status != "ACTIVE" {
			continue
		}
		for _, ev := range sub.Events {
			if ev == eventType {
				result = append(result, copyWebhook(sub))
				break
			}
		}
	}
	return result, nil
}

// Disable marks a subscription as disabled.
func (s *InMemoryWebhookStore) Disable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subs[id]
	if !ok {
		return ErrWebhookNotFound
	}

	sub.Status = "DISABLED"
	sub.UpdatedAt = time.Now().UTC()
	return nil
}

func isValidWebhookURL(u string) bool {
	if strings.HasPrefix(u, "https://") {
		return true
	}
	// Allow localhost for development
	if strings.HasPrefix(u, "http://localhost") || strings.HasPrefix(u, "http://127.0.0.1") {
		return true
	}
	return false
}
