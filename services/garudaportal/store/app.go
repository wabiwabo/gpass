package store

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAppNotFound  = errors.New("app not found")
	ErrAppNameEmpty = errors.New("app name is required")
)

// App represents a developer application.
type App struct {
	ID            string
	OwnerUserID   string
	Name          string
	Description   string
	Environment   string
	Tier          string
	DailyLimit    int
	CallbackURLs  []string
	OAuthClientID string
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// AppUpdate holds the fields that can be updated on an app.
type AppUpdate struct {
	Name         *string
	Description  *string
	CallbackURLs []string
}

// AppStore defines the interface for app persistence.
type AppStore interface {
	Create(app *App) (*App, error)
	GetByID(id string) (*App, error)
	ListByOwner(ownerUserID string) ([]*App, error)
	Update(id string, updates AppUpdate) (*App, error)
}

// InMemoryAppStore is a thread-safe in-memory implementation of AppStore.
type InMemoryAppStore struct {
	mu   sync.RWMutex
	apps map[string]*App
}

// NewInMemoryAppStore creates a new in-memory app store.
func NewInMemoryAppStore() *InMemoryAppStore {
	return &InMemoryAppStore{
		apps: make(map[string]*App),
	}
}

func copyApp(a *App) *App {
	cp := *a
	cp.CallbackURLs = make([]string, len(a.CallbackURLs))
	copy(cp.CallbackURLs, a.CallbackURLs)
	return &cp
}

// Create stores a new app, auto-setting ID, CreatedAt, and UpdatedAt.
func (s *InMemoryAppStore) Create(app *App) (*App, error) {
	if app.Name == "" {
		return nil, ErrAppNameEmpty
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	app.ID = uuid.New().String()
	app.CreatedAt = now
	app.UpdatedAt = now

	if app.Status == "" {
		app.Status = "ACTIVE"
	}

	s.apps[app.ID] = copyApp(app)
	return copyApp(app), nil
}

// GetByID retrieves an app by its ID.
func (s *InMemoryAppStore) GetByID(id string) (*App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	a, ok := s.apps[id]
	if !ok {
		return nil, ErrAppNotFound
	}
	return copyApp(a), nil
}

// ListByOwner retrieves all apps owned by a user.
func (s *InMemoryAppStore) ListByOwner(ownerUserID string) ([]*App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*App
	for _, a := range s.apps {
		if a.OwnerUserID == ownerUserID {
			result = append(result, copyApp(a))
		}
	}
	return result, nil
}

// Update updates allowed fields on an app.
func (s *InMemoryAppStore) Update(id string, updates AppUpdate) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	a, ok := s.apps[id]
	if !ok {
		return nil, ErrAppNotFound
	}

	if updates.Name != nil {
		a.Name = *updates.Name
	}
	if updates.Description != nil {
		a.Description = *updates.Description
	}
	if updates.CallbackURLs != nil {
		a.CallbackURLs = make([]string, len(updates.CallbackURLs))
		copy(a.CallbackURLs, updates.CallbackURLs)
	}

	a.UpdatedAt = time.Now().UTC()
	return copyApp(a), nil
}
