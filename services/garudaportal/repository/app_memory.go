package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// InMemoryAppRepository implements AppRepository for testing.
type InMemoryAppRepository struct {
	mu      sync.RWMutex
	apps    map[string]*App  // keyed by ID
	byOwner map[string][]string // ownerUserID -> []appID
}

// NewInMemoryAppRepository creates a new in-memory app repository.
func NewInMemoryAppRepository() *InMemoryAppRepository {
	return &InMemoryAppRepository{
		apps:    make(map[string]*App),
		byOwner: make(map[string][]string),
	}
}

func (r *InMemoryAppRepository) Create(_ context.Context, app *App) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check for duplicate name under the same owner.
	for _, existing := range r.apps {
		if existing.OwnerUserID == app.OwnerUserID && existing.Name == app.Name {
			return ErrDuplicateApp
		}
	}

	id, err := generateUUID()
	if err != nil {
		return fmt.Errorf("generate id: %w", err)
	}

	now := time.Now().UTC()
	app.ID = id
	app.CreatedAt = now
	app.UpdatedAt = now

	stored := copyApp(app)
	r.apps[app.ID] = stored
	r.byOwner[app.OwnerUserID] = append(r.byOwner[app.OwnerUserID], app.ID)

	return nil
}

func (r *InMemoryAppRepository) GetByID(_ context.Context, id string) (*App, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.apps[id]
	if !ok {
		return nil, ErrNotFound
	}
	return copyApp(a), nil
}

func (r *InMemoryAppRepository) ListByOwner(_ context.Context, ownerUserID string) ([]*App, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byOwner[ownerUserID]
	apps := make([]*App, 0, len(ids))
	for _, id := range ids {
		if a, ok := r.apps[id]; ok {
			apps = append(apps, copyApp(a))
		}
	}
	return apps, nil
}

func (r *InMemoryAppRepository) Update(_ context.Context, id string, updates map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.apps[id]
	if !ok {
		return ErrNotFound
	}

	for key, val := range updates {
		switch key {
		case "name":
			if v, ok := val.(string); ok {
				a.Name = v
			}
		case "description":
			if v, ok := val.(string); ok {
				a.Description = v
			}
		case "environment":
			if v, ok := val.(string); ok {
				a.Environment = v
			}
		case "tier":
			if v, ok := val.(string); ok {
				a.Tier = v
			}
		case "daily_limit":
			if v, ok := val.(int); ok {
				a.DailyLimit = v
			}
		case "callback_urls":
			if v, ok := val.([]string); ok {
				a.CallbackURLs = copyStrings(v)
			}
		case "status":
			if v, ok := val.(string); ok {
				a.Status = v
			}
		}
	}
	a.UpdatedAt = time.Now().UTC()
	return nil
}

func (r *InMemoryAppRepository) UpdateStatus(_ context.Context, id, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	a, ok := r.apps[id]
	if !ok {
		return ErrNotFound
	}
	a.Status = status
	a.UpdatedAt = time.Now().UTC()
	return nil
}

func copyApp(a *App) *App {
	cp := *a
	cp.CallbackURLs = copyStrings(a.CallbackURLs)
	return &cp
}

func copyStrings(s []string) []string {
	if s == nil {
		return nil
	}
	cp := make([]string, len(s))
	copy(cp, s)
	return cp
}

func generateUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Set version 4 and variant bits.
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	), nil
}
