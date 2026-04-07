package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// AuditEvent represents an immutable audit log entry.
type AuditEvent struct {
	ID           string            `json:"id"`
	EventType    string            `json:"event_type"`
	ActorID      string            `json:"actor_id"`
	ActorType    string            `json:"actor_type"`
	ResourceID   string            `json:"resource_id"`
	ResourceType string            `json:"resource_type"`
	Action       string            `json:"action"`
	Metadata     map[string]string `json:"metadata"`
	IPAddress    string            `json:"ip_address"`
	UserAgent    string            `json:"user_agent"`
	ServiceName  string            `json:"service_name"`
	RequestID    string            `json:"request_id"`
	Status       string            `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
}

// AuditStore manages immutable audit events.
type AuditStore interface {
	// Append adds an audit event. Events are append-only — no updates or deletes.
	Append(event *AuditEvent) error

	// GetByID retrieves a single event by its ID.
	GetByID(id string) (*AuditEvent, error)

	// Query retrieves events matching the filter criteria.
	Query(filter AuditFilter) ([]*AuditEvent, error)

	// Count returns the number of events matching the filter.
	Count(filter AuditFilter) (int64, error)
}

// AuditFilter for querying audit events.
type AuditFilter struct {
	ActorID      string
	ResourceID   string
	ResourceType string
	EventType    string
	Action       string
	ServiceName  string
	Status       string
	From         time.Time
	To           time.Time
	Limit        int
	Offset       int
}

// InMemoryAuditStore is an in-memory, append-only implementation of AuditStore.
type InMemoryAuditStore struct {
	mu     sync.RWMutex
	events []*AuditEvent
	byID   map[string]*AuditEvent
}

// NewInMemoryAuditStore creates a new in-memory audit store.
func NewInMemoryAuditStore() *InMemoryAuditStore {
	return &InMemoryAuditStore{
		events: make([]*AuditEvent, 0),
		byID:   make(map[string]*AuditEvent),
	}
}

// Append validates and appends an audit event. The event is immutable once appended.
func (s *InMemoryAuditStore) Append(event *AuditEvent) error {
	if err := ValidateEvent(event); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Assign ID and timestamp
	event.ID = generateID()
	event.CreatedAt = time.Now()

	if event.ActorType == "" {
		event.ActorType = "USER"
	}
	if event.Status == "" {
		event.Status = "SUCCESS"
	}
	if event.Metadata == nil {
		event.Metadata = make(map[string]string)
	}

	// Store a copy to ensure immutability
	stored := *event
	storedMeta := make(map[string]string, len(event.Metadata))
	for k, v := range event.Metadata {
		storedMeta[k] = v
	}
	stored.Metadata = storedMeta

	s.events = append(s.events, &stored)
	s.byID[stored.ID] = &stored

	// Write back the assigned ID to the caller's event
	event.ID = stored.ID
	event.CreatedAt = stored.CreatedAt

	return nil
}

// GetByID retrieves a single event by its ID.
func (s *InMemoryAuditStore) GetByID(id string) (*AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.byID[id]
	if !ok {
		return nil, fmt.Errorf("audit event not found: %s", id)
	}

	// Return a copy
	cp := *event
	cpMeta := make(map[string]string, len(event.Metadata))
	for k, v := range event.Metadata {
		cpMeta[k] = v
	}
	cp.Metadata = cpMeta

	return &cp, nil
}

// Query retrieves events matching the filter criteria using AND logic.
func (s *InMemoryAuditStore) Query(filter AuditFilter) ([]*AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	var matched []*AuditEvent
	skipped := 0

	for _, e := range s.events {
		if !matchesFilter(e, filter) {
			continue
		}
		if skipped < filter.Offset {
			skipped++
			continue
		}
		if len(matched) >= limit {
			break
		}

		// Return copies
		cp := *e
		cpMeta := make(map[string]string, len(e.Metadata))
		for k, v := range e.Metadata {
			cpMeta[k] = v
		}
		cp.Metadata = cpMeta
		matched = append(matched, &cp)
	}

	return matched, nil
}

// Count returns the number of events matching the filter.
func (s *InMemoryAuditStore) Count(filter AuditFilter) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, e := range s.events {
		if matchesFilter(e, filter) {
			count++
		}
	}
	return count, nil
}

func matchesFilter(e *AuditEvent, f AuditFilter) bool {
	if f.ActorID != "" && e.ActorID != f.ActorID {
		return false
	}
	if f.ResourceID != "" && e.ResourceID != f.ResourceID {
		return false
	}
	if f.ResourceType != "" && e.ResourceType != f.ResourceType {
		return false
	}
	if f.EventType != "" && e.EventType != f.EventType {
		return false
	}
	if f.Action != "" && e.Action != f.Action {
		return false
	}
	if f.ServiceName != "" && e.ServiceName != f.ServiceName {
		return false
	}
	if f.Status != "" && e.Status != f.Status {
		return false
	}
	if !f.From.IsZero() && e.CreatedAt.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && e.CreatedAt.After(f.To) {
		return false
	}
	return true
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate ID: %v", err))
	}
	return hex.EncodeToString(b)
}
