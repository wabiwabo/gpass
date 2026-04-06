package eventsource

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Event represents a domain event in an event-sourced system.
type Event struct {
	ID            string            `json:"id"`
	AggregateID   string            `json:"aggregate_id"`
	AggregateType string            `json:"aggregate_type"`
	EventType     string            `json:"event_type"`
	Version       int               `json:"version"`
	Payload       json.RawMessage   `json:"payload"`
	Metadata      map[string]string `json:"metadata"`
	Timestamp     time.Time         `json:"timestamp"`
}

// Aggregate represents a domain aggregate that can be reconstituted from events.
type Aggregate interface {
	ID() string
	Type() string
	Version() int
	ApplyEvent(Event) error
}

// EventStore persists and retrieves domain events.
type EventStore interface {
	Save(ctx context.Context, events []Event) error
	Load(ctx context.Context, aggregateID string) ([]Event, error)
	LoadFrom(ctx context.Context, aggregateID string, fromVersion int) ([]Event, error)
}

// Snapshot represents a point-in-time state of an aggregate.
type Snapshot struct {
	AggregateID   string          `json:"aggregate_id"`
	AggregateType string          `json:"aggregate_type"`
	Version       int             `json:"version"`
	Data          json.RawMessage `json:"data"`
	Timestamp     time.Time       `json:"timestamp"`
}

// SnapshotStore persists and retrieves aggregate snapshots.
type SnapshotStore interface {
	SaveSnapshot(ctx context.Context, snapshot Snapshot) error
	LoadSnapshot(ctx context.Context, aggregateID string) (*Snapshot, error)
}

// NewEvent creates a new Event with the given parameters. The payload is
// marshalled to JSON. A timestamp is set to the current time.
func NewEvent(aggregateID, aggregateType, eventType string, version int, payload interface{}) (Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("eventsource: marshal payload: %w", err)
	}

	return Event{
		ID:            fmt.Sprintf("%s-%d", aggregateID, version),
		AggregateID:   aggregateID,
		AggregateType: aggregateType,
		EventType:     eventType,
		Version:       version,
		Payload:       data,
		Metadata:      make(map[string]string),
		Timestamp:     time.Now(),
	}, nil
}

// MemoryEventStore is an in-memory EventStore implementation for testing.
type MemoryEventStore struct {
	mu     sync.RWMutex
	events map[string][]Event // keyed by aggregate ID
}

// NewMemoryEventStore creates a new in-memory event store.
func NewMemoryEventStore() *MemoryEventStore {
	return &MemoryEventStore{
		events: make(map[string][]Event),
	}
}

// Save persists the given events to the in-memory store.
func (s *MemoryEventStore) Save(_ context.Context, events []Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, e := range events {
		s.events[e.AggregateID] = append(s.events[e.AggregateID], e)
	}

	return nil
}

// Load returns all events for the given aggregate ID, ordered by version.
func (s *MemoryEventStore) Load(_ context.Context, aggregateID string) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stored, ok := s.events[aggregateID]
	if !ok {
		return nil, nil
	}

	result := make([]Event, len(stored))
	copy(result, stored)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}

// LoadFrom returns events for the given aggregate ID starting from the
// specified version (inclusive), ordered by version.
func (s *MemoryEventStore) LoadFrom(_ context.Context, aggregateID string, fromVersion int) ([]Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stored, ok := s.events[aggregateID]
	if !ok {
		return nil, nil
	}

	var result []Event
	for _, e := range stored {
		if e.Version >= fromVersion {
			result = append(result, e)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}

// MemorySnapshotStore is an in-memory SnapshotStore implementation for testing.
type MemorySnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[string]Snapshot // keyed by aggregate ID
}

// NewMemorySnapshotStore creates a new in-memory snapshot store.
func NewMemorySnapshotStore() *MemorySnapshotStore {
	return &MemorySnapshotStore{
		snapshots: make(map[string]Snapshot),
	}
}

// SaveSnapshot persists a snapshot to the in-memory store.
func (s *MemorySnapshotStore) SaveSnapshot(_ context.Context, snapshot Snapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots[snapshot.AggregateID] = snapshot
	return nil
}

// LoadSnapshot retrieves a snapshot for the given aggregate ID. Returns nil
// if no snapshot exists.
func (s *MemorySnapshotStore) LoadSnapshot(_ context.Context, aggregateID string) (*Snapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snap, ok := s.snapshots[aggregateID]
	if !ok {
		return nil, nil
	}
	return &snap, nil
}

// Repository is a generic repository that loads and saves aggregates using
// an event store and an optional snapshot store.
type Repository[T Aggregate] struct {
	store   EventStore
	factory func() T
}

// NewRepository creates a new Repository. The factory function is used to
// create new aggregate instances for hydration.
func NewRepository[T Aggregate](store EventStore, factory func() T) *Repository[T] {
	return &Repository[T]{
		store:   store,
		factory: factory,
	}
}

// Load reconstitutes an aggregate from its event history.
func (r *Repository[T]) Load(ctx context.Context, id string) (T, error) {
	agg := r.factory()

	events, err := r.store.Load(ctx, id)
	if err != nil {
		return agg, fmt.Errorf("eventsource: load events: %w", err)
	}

	for _, e := range events {
		if err := agg.ApplyEvent(e); err != nil {
			return agg, fmt.Errorf("eventsource: apply event %s (v%d): %w", e.EventType, e.Version, err)
		}
	}

	return agg, nil
}

// Save persists new events for an aggregate.
func (r *Repository[T]) Save(ctx context.Context, _ T, events []Event) error {
	if len(events) == 0 {
		return nil
	}

	if err := r.store.Save(ctx, events); err != nil {
		return fmt.Errorf("eventsource: save events: %w", err)
	}

	return nil
}
