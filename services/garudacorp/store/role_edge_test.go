package store

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

func TestValidateRoleAssignment(t *testing.T) {
	tests := []struct {
		name   string
		caller string
		target string
		ok     bool
	}{
		{"ro_assigns_admin", RoleRegisteredOfficer, RoleAdmin, true},
		{"ro_assigns_user", RoleRegisteredOfficer, RoleUser, true},
		{"admin_assigns_user", RoleAdmin, RoleUser, true},
		{"admin_cannot_assign_admin", RoleAdmin, RoleAdmin, false},
		{"admin_cannot_assign_ro", RoleAdmin, RoleRegisteredOfficer, false},
		{"user_cannot_assign_user", RoleUser, RoleUser, false},
		{"user_cannot_assign_admin", RoleUser, RoleAdmin, false},
		{"ro_cannot_assign_ro", RoleRegisteredOfficer, RoleRegisteredOfficer, false},
		{"unknown_caller", "UNKNOWN", RoleUser, false},
		{"unknown_target", RoleAdmin, "UNKNOWN", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoleAssignment(tt.caller, tt.target)
			if tt.ok && err != nil {
				t.Errorf("expected nil error, got %v", err)
			}
			if !tt.ok && err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestConcurrentRoleAssign(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			r := &EntityRole{
				EntityID:      "entity-1",
				UserID:        "user-concurrent",
				Role:          RoleUser,
				GrantedBy:     "admin",
				ServiceAccess: []string{"signing"},
			}
			_ = s.Assign(ctx, r)
		}(i)
	}
	wg.Wait()

	list, _ := s.ListByEntity(ctx, "entity-1")
	if len(list) != 100 {
		t.Errorf("roles: got %d, want 100", len(list))
	}
}

func TestConcurrentRoleRevoke(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	r := &EntityRole{
		EntityID: "entity-1", UserID: "user-1", Role: RoleAdmin,
		GrantedBy: "ro", ServiceAccess: []string{"signing"},
	}
	_ = s.Assign(ctx, r)
	id := r.ID

	var wg sync.WaitGroup
	var successCount atomic.Int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Revoke(ctx, id); err == nil {
				successCount.Add(1)
			}
		}()
	}
	wg.Wait()

	if successCount.Load() != 1 {
		t.Errorf("exactly 1 revoke should succeed, got %d", successCount.Load())
	}
}

func TestRoleCopyIsolation(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	r := &EntityRole{
		EntityID: "entity-1", UserID: "user-1", Role: RoleAdmin,
		GrantedBy: "ro", ServiceAccess: []string{"signing", "garudainfo"},
	}
	_ = s.Assign(ctx, r)

	got, _ := s.GetByID(ctx, r.ID)
	got.ServiceAccess[0] = "TAMPERED"
	got.Role = "TAMPERED"

	got2, _ := s.GetByID(ctx, r.ID)
	if got2.Role == "TAMPERED" {
		t.Error("role should not be mutated")
	}
	if got2.ServiceAccess[0] == "TAMPERED" {
		t.Error("service access should not be mutated")
	}
}

func TestGetUserRole(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	// Assign active role
	r := &EntityRole{
		EntityID: "entity-1", UserID: "user-1", Role: RoleAdmin,
		GrantedBy: "ro", ServiceAccess: []string{"signing"},
	}
	_ = s.Assign(ctx, r)

	got, err := s.GetUserRole(ctx, "entity-1", "user-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.Role != RoleAdmin {
		t.Errorf("role: got %q", got.Role)
	}
}

func TestGetUserRoleSkipsRevoked(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	r := &EntityRole{
		EntityID: "entity-1", UserID: "user-1", Role: RoleAdmin,
		GrantedBy: "ro",
	}
	_ = s.Assign(ctx, r)
	_ = s.Revoke(ctx, r.ID)

	_, err := s.GetUserRole(ctx, "entity-1", "user-1")
	if err != ErrRoleNotFound {
		t.Errorf("got %v, want ErrRoleNotFound (revoked role should not be returned)", err)
	}
}

func TestGetUserRoleNotFound(t *testing.T) {
	s := NewInMemoryRoleStore()
	_, err := s.GetUserRole(context.Background(), "entity-x", "user-x")
	if err != ErrRoleNotFound {
		t.Errorf("got %v, want ErrRoleNotFound", err)
	}
}

func TestRevokeNotFoundRole(t *testing.T) {
	s := NewInMemoryRoleStore()
	err := s.Revoke(context.Background(), "bad-id")
	if err != ErrRoleNotFound {
		t.Errorf("got %v, want ErrRoleNotFound", err)
	}
}

func TestRevokeAlreadyRevoked(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	r := &EntityRole{EntityID: "e1", UserID: "u1", Role: RoleUser, GrantedBy: "admin"}
	_ = s.Assign(ctx, r)
	_ = s.Revoke(ctx, r.ID)

	err := s.Revoke(ctx, r.ID)
	if err != ErrRoleAlreadyRevoked {
		t.Errorf("got %v, want ErrRoleAlreadyRevoked", err)
	}
}

func TestListByEntityIncludesAllStatuses(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	r1 := &EntityRole{EntityID: "e1", UserID: "u1", Role: RoleAdmin, GrantedBy: "ro"}
	r2 := &EntityRole{EntityID: "e1", UserID: "u2", Role: RoleUser, GrantedBy: "admin"}
	_ = s.Assign(ctx, r1)
	_ = s.Assign(ctx, r2)
	_ = s.Revoke(ctx, r2.ID)

	list, _ := s.ListByEntity(ctx, "e1")
	if len(list) != 2 {
		t.Errorf("should include both active and revoked: got %d", len(list))
	}
}

func TestListByEntityIsolation(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	_ = s.Assign(ctx, &EntityRole{EntityID: "e1", UserID: "u1", Role: RoleAdmin, GrantedBy: "ro"})
	_ = s.Assign(ctx, &EntityRole{EntityID: "e2", UserID: "u2", Role: RoleUser, GrantedBy: "admin"})

	list, _ := s.ListByEntity(ctx, "e1")
	if len(list) != 1 {
		t.Errorf("entity isolation: got %d, want 1", len(list))
	}
}

func TestRoleAssignSetsDefaults(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	r := &EntityRole{EntityID: "e1", UserID: "u1", Role: RoleUser, GrantedBy: "admin"}
	_ = s.Assign(ctx, r)

	if r.ID == "" {
		t.Error("ID should be assigned")
	}
	if r.Status != StatusActive {
		t.Errorf("Status: got %q", r.Status)
	}
	if r.GrantedAt.IsZero() {
		t.Error("GrantedAt should be set")
	}
}

func TestRevokeAndReassign(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	// Assign
	r1 := &EntityRole{EntityID: "e1", UserID: "u1", Role: RoleUser, GrantedBy: "admin"}
	_ = s.Assign(ctx, r1)
	_ = s.Revoke(ctx, r1.ID)

	// Reassign same user
	r2 := &EntityRole{EntityID: "e1", UserID: "u1", Role: RoleAdmin, GrantedBy: "ro"}
	_ = s.Assign(ctx, r2)

	got, err := s.GetUserRole(ctx, "e1", "u1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.Role != RoleAdmin {
		t.Errorf("reassigned role: got %q", got.Role)
	}
}
