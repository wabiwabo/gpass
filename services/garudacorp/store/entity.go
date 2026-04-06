package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors for entity operations.
var (
	ErrEntityNotFound = errors.New("entity not found")
)

// Entity represents a corporate entity verified via AHU.
type Entity struct {
	ID            string
	AHUSKNumber   string
	Name          string
	EntityType    string
	Status        string
	NPWP          string
	Address       string
	CapitalAuth   int64
	CapitalPaid   int64
	AHUVerifiedAt time.Time
	OSSNIB        string
	OSSVerifiedAt *time.Time
	Officers      []EntityOfficer
	Shareholders  []EntityShareholder
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// EntityOfficer represents an officer of a corporate entity.
type EntityOfficer struct {
	ID              string
	EntityID        string
	UserID          string
	NIKToken        string
	Name            string
	Position        string
	AppointmentDate string
	Verified        bool
}

// EntityShareholder represents a shareholder of a corporate entity.
type EntityShareholder struct {
	ID         string
	EntityID   string
	Name       string
	ShareType  string
	Shares     int64
	Percentage float64
}

// EntityStore defines the interface for entity persistence.
type EntityStore interface {
	Create(ctx context.Context, e *Entity) error
	GetByID(ctx context.Context, id string) (*Entity, error)
	GetBySKNumber(ctx context.Context, sk string) (*Entity, error)
	AddOfficers(ctx context.Context, entityID string, officers []EntityOfficer) error
	AddShareholders(ctx context.Context, entityID string, shareholders []EntityShareholder) error
}

// InMemoryEntityStore is a thread-safe in-memory implementation of EntityStore.
type InMemoryEntityStore struct {
	mu       sync.RWMutex
	entities map[string]*Entity
}

// NewInMemoryEntityStore creates a new in-memory entity store.
func NewInMemoryEntityStore() *InMemoryEntityStore {
	return &InMemoryEntityStore{
		entities: make(map[string]*Entity),
	}
}

func copyEntity(e *Entity) *Entity {
	cp := *e
	cp.Officers = make([]EntityOfficer, len(e.Officers))
	copy(cp.Officers, e.Officers)
	cp.Shareholders = make([]EntityShareholder, len(e.Shareholders))
	copy(cp.Shareholders, e.Shareholders)
	if e.OSSVerifiedAt != nil {
		t := *e.OSSVerifiedAt
		cp.OSSVerifiedAt = &t
	}
	return &cp
}

// Create stores a new entity, auto-setting ID, CreatedAt, and UpdatedAt.
func (s *InMemoryEntityStore) Create(_ context.Context, e *Entity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	e.ID = uuid.New().String()
	e.CreatedAt = now
	e.UpdatedAt = now

	s.entities[e.ID] = copyEntity(e)
	return nil
}

// GetByID retrieves an entity by its ID.
func (s *InMemoryEntityStore) GetByID(_ context.Context, id string) (*Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entities[id]
	if !ok {
		return nil, ErrEntityNotFound
	}
	return copyEntity(e), nil
}

// GetBySKNumber retrieves an entity by its AHU SK number.
func (s *InMemoryEntityStore) GetBySKNumber(_ context.Context, sk string) (*Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range s.entities {
		if e.AHUSKNumber == sk {
			return copyEntity(e), nil
		}
	}
	return nil, ErrEntityNotFound
}

// AddOfficers adds officers to an entity.
func (s *InMemoryEntityStore) AddOfficers(_ context.Context, entityID string, officers []EntityOfficer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entities[entityID]
	if !ok {
		return ErrEntityNotFound
	}

	for i := range officers {
		officers[i].ID = uuid.New().String()
		officers[i].EntityID = entityID
	}

	e.Officers = append(e.Officers, officers...)
	e.UpdatedAt = time.Now().UTC()
	return nil
}

// AddShareholders adds shareholders to an entity.
func (s *InMemoryEntityStore) AddShareholders(_ context.Context, entityID string, shareholders []EntityShareholder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entities[entityID]
	if !ok {
		return ErrEntityNotFound
	}

	for i := range shareholders {
		shareholders[i].ID = uuid.New().String()
		shareholders[i].EntityID = entityID
	}

	e.Shareholders = append(e.Shareholders, shareholders...)
	e.UpdatedAt = time.Now().UTC()
	return nil
}
