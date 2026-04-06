package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/store"
)

func setupRoleTest(t *testing.T) (*RoleHandler, store.EntityStore, store.RoleStore, string) {
	t.Helper()
	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()

	entity := &store.Entity{
		AHUSKNumber: "AHU-12345",
		Name:        "PT Test Corp",
		EntityType:  "PT",
		Status:      "ACTIVE",
	}
	entityStore.Create(context.Background(), entity)

	h := NewRoleHandler(roleStore, entityStore)
	return h, entityStore, roleStore, entity.ID
}

func TestAssignRole_AsRO_Success(t *testing.T) {
	h, _, roleStore, entityID := setupRoleTest(t)
	ctx := context.Background()

	// Create RO role for caller
	roleStore.Assign(ctx, &store.EntityRole{
		EntityID: entityID, UserID: "ro-user", Role: store.RoleRegisteredOfficer, GrantedBy: "system",
	})

	body, _ := json.Marshal(assignRoleRequest{
		UserID:        "new-user",
		Role:          store.RoleAdmin,
		ServiceAccess: []string{"signing"},
		CallerUserID:  "ro-user",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/entities/"+entityID+"/roles", bytes.NewReader(body))
	req.SetPathValue("entity_id", entityID)
	w := httptest.NewRecorder()

	h.AssignRole(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp assignRoleResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Role != store.RoleAdmin {
		t.Errorf("Role = %q, want %q", resp.Role, store.RoleAdmin)
	}
}

func TestAssignRole_AsAdmin_UserSuccess(t *testing.T) {
	h, _, roleStore, entityID := setupRoleTest(t)
	ctx := context.Background()

	// Create ADMIN role for caller
	roleStore.Assign(ctx, &store.EntityRole{
		EntityID: entityID, UserID: "admin-user", Role: store.RoleAdmin, GrantedBy: "ro-user",
	})

	body, _ := json.Marshal(assignRoleRequest{
		UserID:       "new-user",
		Role:         store.RoleUser,
		CallerUserID: "admin-user",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/entities/"+entityID+"/roles", bytes.NewReader(body))
	req.SetPathValue("entity_id", entityID)
	w := httptest.NewRecorder()

	h.AssignRole(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestAssignRole_AsUser_Fail(t *testing.T) {
	h, _, roleStore, entityID := setupRoleTest(t)
	ctx := context.Background()

	// Create USER role for caller
	roleStore.Assign(ctx, &store.EntityRole{
		EntityID: entityID, UserID: "plain-user", Role: store.RoleUser, GrantedBy: "admin-user",
	})

	body, _ := json.Marshal(assignRoleRequest{
		UserID:       "another-user",
		Role:         store.RoleUser,
		CallerUserID: "plain-user",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/entities/"+entityID+"/roles", bytes.NewReader(body))
	req.SetPathValue("entity_id", entityID)
	w := httptest.NewRecorder()

	h.AssignRole(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestAssignRole_AdminCannotAssignAdmin(t *testing.T) {
	h, _, roleStore, entityID := setupRoleTest(t)
	ctx := context.Background()

	roleStore.Assign(ctx, &store.EntityRole{
		EntityID: entityID, UserID: "admin-user", Role: store.RoleAdmin, GrantedBy: "ro-user",
	})

	body, _ := json.Marshal(assignRoleRequest{
		UserID:       "new-user",
		Role:         store.RoleAdmin,
		CallerUserID: "admin-user",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/entities/"+entityID+"/roles", bytes.NewReader(body))
	req.SetPathValue("entity_id", entityID)
	w := httptest.NewRecorder()

	h.AssignRole(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestListRoles(t *testing.T) {
	h, _, roleStore, entityID := setupRoleTest(t)
	ctx := context.Background()

	roleStore.Assign(ctx, &store.EntityRole{EntityID: entityID, UserID: "user-1", Role: store.RoleRegisteredOfficer, GrantedBy: "system"})
	roleStore.Assign(ctx, &store.EntityRole{EntityID: entityID, UserID: "user-2", Role: store.RoleAdmin, GrantedBy: "user-1"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/"+entityID+"/roles", nil)
	req.SetPathValue("entity_id", entityID)
	w := httptest.NewRecorder()

	h.ListRoles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)

	var roles []store.EntityRole
	json.Unmarshal(resp["roles"], &roles)
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}
}

func TestRevokeRole(t *testing.T) {
	h, _, roleStore, entityID := setupRoleTest(t)
	ctx := context.Background()

	r := &store.EntityRole{EntityID: entityID, UserID: "user-1", Role: store.RoleAdmin, GrantedBy: "ro-user"}
	roleStore.Assign(ctx, r)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/corp/entities/"+entityID+"/roles/"+r.ID, nil)
	req.SetPathValue("entity_id", entityID)
	req.SetPathValue("role_id", r.ID)
	w := httptest.NewRecorder()

	h.RevokeRole(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify role is revoked
	got, _ := roleStore.GetByID(ctx, r.ID)
	if got.Status != store.StatusRevoked {
		t.Errorf("Status = %q, want %q", got.Status, store.StatusRevoked)
	}
}

func TestRevokeRole_NotFound(t *testing.T) {
	h, _, _, entityID := setupRoleTest(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/corp/entities/"+entityID+"/roles/nonexistent", nil)
	req.SetPathValue("entity_id", entityID)
	req.SetPathValue("role_id", "nonexistent")
	w := httptest.NewRecorder()

	h.RevokeRole(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}
