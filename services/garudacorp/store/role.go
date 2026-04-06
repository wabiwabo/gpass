package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Role constants.
const (
	RoleRegisteredOfficer = "REGISTERED_OFFICER"
	RoleAdmin             = "ADMIN"
	RoleUser              = "USER"
)

// Role status constants.
const (
	StatusActive  = "ACTIVE"
	StatusRevoked = "REVOKED"
)

// Sentinel errors for role operations.
var (
	ErrRoleNotFound        = errors.New("role not found")
	ErrRoleAlreadyRevoked  = errors.New("role already revoked")
	ErrInsufficientRole    = errors.New("insufficient role to perform assignment")
	ErrCannotAssignRole    = errors.New("caller cannot assign this role level")
)

// EntityRole represents a user's role within a corporate entity.
type EntityRole struct {
	ID            string
	EntityID      string
	UserID        string
	Role          string // REGISTERED_OFFICER, ADMIN, USER
	GrantedBy     string
	ServiceAccess []string // ["signing","garudainfo"]
	Status        string   // ACTIVE, REVOKED
	GrantedAt     time.Time
	RevokedAt     *time.Time
}

// RoleStore defines the interface for role persistence.
type RoleStore interface {
	Assign(ctx context.Context, r *EntityRole) error
	ListByEntity(ctx context.Context, entityID string) ([]*EntityRole, error)
	GetByID(ctx context.Context, id string) (*EntityRole, error)
	Revoke(ctx context.Context, id string) error
	GetUserRole(ctx context.Context, entityID, userID string) (*EntityRole, error)
}

// ValidateRoleAssignment checks if a caller with callerRole can assign targetRole.
// REGISTERED_OFFICER can assign ADMIN or USER.
// ADMIN can assign USER only.
// USER cannot assign any roles.
func ValidateRoleAssignment(callerRole, targetRole string) error {
	callerLevel := roleLevel(callerRole)
	targetLevel := roleLevel(targetRole)

	if callerLevel < 0 || targetLevel < 0 {
		return fmt.Errorf("unknown role: caller=%q target=%q", callerRole, targetRole)
	}

	// Caller must be strictly higher level than target
	if callerLevel <= targetLevel {
		return ErrCannotAssignRole
	}

	return nil
}

// roleLevel returns a numeric level: RO=3, ADMIN=2, USER=1, unknown=-1
func roleLevel(role string) int {
	switch role {
	case RoleRegisteredOfficer:
		return 3
	case RoleAdmin:
		return 2
	case RoleUser:
		return 1
	default:
		return -1
	}
}

// InMemoryRoleStore is a thread-safe in-memory implementation of RoleStore.
type InMemoryRoleStore struct {
	mu    sync.RWMutex
	roles map[string]*EntityRole
}

// NewInMemoryRoleStore creates a new in-memory role store.
func NewInMemoryRoleStore() *InMemoryRoleStore {
	return &InMemoryRoleStore{
		roles: make(map[string]*EntityRole),
	}
}

func copyRole(r *EntityRole) *EntityRole {
	cp := *r
	cp.ServiceAccess = make([]string, len(r.ServiceAccess))
	copy(cp.ServiceAccess, r.ServiceAccess)
	if r.RevokedAt != nil {
		t := *r.RevokedAt
		cp.RevokedAt = &t
	}
	return &cp
}

// Assign stores a new role, auto-setting ID, Status, and GrantedAt.
func (s *InMemoryRoleStore) Assign(_ context.Context, r *EntityRole) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	r.ID = uuid.New().String()
	r.Status = StatusActive
	r.GrantedAt = time.Now().UTC()

	s.roles[r.ID] = copyRole(r)
	return nil
}

// ListByEntity returns all roles for a given entity ID.
func (s *InMemoryRoleStore) ListByEntity(_ context.Context, entityID string) ([]*EntityRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*EntityRole
	for _, r := range s.roles {
		if r.EntityID == entityID {
			result = append(result, copyRole(r))
		}
	}
	return result, nil
}

// GetByID retrieves a role by its ID.
func (s *InMemoryRoleStore) GetByID(_ context.Context, id string) (*EntityRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.roles[id]
	if !ok {
		return nil, ErrRoleNotFound
	}
	return copyRole(r), nil
}

// Revoke sets a role's status to REVOKED.
func (s *InMemoryRoleStore) Revoke(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.roles[id]
	if !ok {
		return ErrRoleNotFound
	}
	if r.Status == StatusRevoked {
		return ErrRoleAlreadyRevoked
	}

	now := time.Now().UTC()
	r.Status = StatusRevoked
	r.RevokedAt = &now
	return nil
}

// GetUserRole retrieves the active role for a user in a specific entity.
func (s *InMemoryRoleStore) GetUserRole(_ context.Context, entityID, userID string) (*EntityRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.roles {
		if r.EntityID == entityID && r.UserID == userID && r.Status == StatusActive {
			return copyRole(r), nil
		}
	}
	return nil, ErrRoleNotFound
}
