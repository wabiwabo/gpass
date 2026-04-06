package store

import (
	"context"
	"testing"
)

func TestRoleStore_AssignAndGetByID(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	r := &EntityRole{
		EntityID:      "entity-1",
		UserID:        "user-1",
		Role:          RoleRegisteredOfficer,
		GrantedBy:     "system",
		ServiceAccess: []string{"signing", "garudainfo"},
	}

	if err := s.Assign(ctx, r); err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if r.ID == "" {
		t.Fatal("expected ID to be set")
	}
	if r.Status != StatusActive {
		t.Errorf("Status = %q, want %q", r.Status, StatusActive)
	}

	got, err := s.GetByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Role != RoleRegisteredOfficer {
		t.Errorf("Role = %q, want %q", got.Role, RoleRegisteredOfficer)
	}
}

func TestRoleStore_ListByEntity(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	s.Assign(ctx, &EntityRole{EntityID: "entity-1", UserID: "user-1", Role: RoleRegisteredOfficer, GrantedBy: "system"})
	s.Assign(ctx, &EntityRole{EntityID: "entity-1", UserID: "user-2", Role: RoleAdmin, GrantedBy: "user-1"})
	s.Assign(ctx, &EntityRole{EntityID: "entity-2", UserID: "user-3", Role: RoleUser, GrantedBy: "user-1"})

	roles, err := s.ListByEntity(ctx, "entity-1")
	if err != nil {
		t.Fatalf("ListByEntity: %v", err)
	}
	if len(roles) != 2 {
		t.Fatalf("expected 2 roles, got %d", len(roles))
	}
}

func TestRoleStore_Revoke(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	r := &EntityRole{EntityID: "entity-1", UserID: "user-1", Role: RoleAdmin, GrantedBy: "user-0"}
	s.Assign(ctx, r)

	if err := s.Revoke(ctx, r.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	got, err := s.GetByID(ctx, r.ID)
	if err != nil {
		t.Fatalf("GetByID after revoke: %v", err)
	}
	if got.Status != StatusRevoked {
		t.Errorf("Status = %q, want %q", got.Status, StatusRevoked)
	}
	if got.RevokedAt == nil {
		t.Error("expected RevokedAt to be set")
	}
}

func TestRoleStore_Revoke_AlreadyRevoked(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	r := &EntityRole{EntityID: "entity-1", UserID: "user-1", Role: RoleAdmin, GrantedBy: "user-0"}
	s.Assign(ctx, r)
	s.Revoke(ctx, r.ID)

	err := s.Revoke(ctx, r.ID)
	if err != ErrRoleAlreadyRevoked {
		t.Errorf("expected ErrRoleAlreadyRevoked, got %v", err)
	}
}

func TestRoleStore_Revoke_NotFound(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	err := s.Revoke(ctx, "nonexistent")
	if err != ErrRoleNotFound {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestRoleStore_GetUserRole(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	s.Assign(ctx, &EntityRole{EntityID: "entity-1", UserID: "user-1", Role: RoleRegisteredOfficer, GrantedBy: "system"})

	got, err := s.GetUserRole(ctx, "entity-1", "user-1")
	if err != nil {
		t.Fatalf("GetUserRole: %v", err)
	}
	if got.Role != RoleRegisteredOfficer {
		t.Errorf("Role = %q, want %q", got.Role, RoleRegisteredOfficer)
	}
}

func TestRoleStore_GetUserRole_NotFound(t *testing.T) {
	s := NewInMemoryRoleStore()
	ctx := context.Background()

	_, err := s.GetUserRole(ctx, "entity-1", "user-1")
	if err != ErrRoleNotFound {
		t.Errorf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestValidateRoleAssignment_ROCanAssignAdmin(t *testing.T) {
	err := ValidateRoleAssignment(RoleRegisteredOfficer, RoleAdmin)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateRoleAssignment_ROCanAssignUser(t *testing.T) {
	err := ValidateRoleAssignment(RoleRegisteredOfficer, RoleUser)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateRoleAssignment_AdminCanAssignUser(t *testing.T) {
	err := ValidateRoleAssignment(RoleAdmin, RoleUser)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateRoleAssignment_AdminCannotAssignAdmin(t *testing.T) {
	err := ValidateRoleAssignment(RoleAdmin, RoleAdmin)
	if err != ErrCannotAssignRole {
		t.Errorf("expected ErrCannotAssignRole, got %v", err)
	}
}

func TestValidateRoleAssignment_UserCannotAssignAnything(t *testing.T) {
	err := ValidateRoleAssignment(RoleUser, RoleUser)
	if err != ErrCannotAssignRole {
		t.Errorf("expected ErrCannotAssignRole, got %v", err)
	}

	err = ValidateRoleAssignment(RoleUser, RoleAdmin)
	if err != ErrCannotAssignRole {
		t.Errorf("expected ErrCannotAssignRole, got %v", err)
	}
}
