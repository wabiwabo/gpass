// Package tenant provides multi-tenancy support for SaaS deployment.
// Each developer app is isolated via tenant context propagation and middleware.
package tenant

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"sync"
)

// Tenant represents an isolated tenant (developer app).
type Tenant struct {
	ID          string
	Name        string
	Environment string // sandbox, production
	Tier        string
	Config      map[string]string
}

// Store manages tenant data.
type Store struct {
	tenants map[string]*Tenant
	mu      sync.RWMutex
}

// NewStore creates a new tenant store.
func NewStore() *Store {
	return &Store{
		tenants: make(map[string]*Tenant),
	}
}

var (
	// ErrNotFound is returned when a tenant is not found.
	ErrNotFound = errors.New("tenant not found")
	// ErrDuplicate is returned when a tenant with the same ID already exists.
	ErrDuplicate = errors.New("tenant already exists")
	// ErrEmptyID is returned when a tenant ID is empty.
	ErrEmptyID = errors.New("tenant ID must not be empty")
)

// Register adds a new tenant to the store.
func (s *Store) Register(t Tenant) error {
	if t.ID == "" {
		return ErrEmptyID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tenants[t.ID]; exists {
		return ErrDuplicate
	}
	cp := t
	if t.Config != nil {
		cp.Config = make(map[string]string, len(t.Config))
		for k, v := range t.Config {
			cp.Config[k] = v
		}
	}
	s.tenants[t.ID] = &cp
	return nil
}

// Get retrieves a tenant by ID.
func (s *Store) Get(id string) (*Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tenants[id]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

// List returns all registered tenants sorted by ID.
func (s *Store) List() []*Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		result = append(result, t)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// Update modifies a tenant's config entries.
func (s *Store) Update(id string, updates map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tenants[id]
	if !ok {
		return ErrNotFound
	}
	if t.Config == nil {
		t.Config = make(map[string]string)
	}
	for k, v := range updates {
		t.Config[k] = v
	}
	return nil
}

type ctxKey struct{}

// WithContext returns a new context with the tenant attached.
func WithContext(ctx context.Context, t *Tenant) context.Context {
	return context.WithValue(ctx, ctxKey{}, t)
}

// FromContext extracts tenant from context.
func FromContext(ctx context.Context) (*Tenant, bool) {
	t, ok := ctx.Value(ctxKey{}).(*Tenant)
	return t, ok
}

// Middleware extracts tenant ID from the request using the resolver function,
// looks up the tenant in the store, and stores it in the request context.
// If the tenant is not found, it responds with 403 Forbidden.
func Middleware(store *Store, resolver func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := resolver(r)
			if tenantID == "" {
				http.Error(w, "missing tenant identifier", http.StatusForbidden)
				return
			}
			t, err := store.Get(tenantID)
			if err != nil {
				http.Error(w, "unknown tenant", http.StatusForbidden)
				return
			}
			ctx := WithContext(r.Context(), t)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// IsolationMiddleware ensures requests only access their own tenant's data.
// It reads the tenant from context and adds X-Tenant-ID to all downstream requests.
func IsolationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, ok := FromContext(r.Context())
		if !ok {
			http.Error(w, "tenant context required", http.StatusForbidden)
			return
		}
		r.Header.Set("X-Tenant-ID", t.ID)
		w.Header().Set("X-Tenant-ID", t.ID)
		next.ServeHTTP(w, r)
	})
}
