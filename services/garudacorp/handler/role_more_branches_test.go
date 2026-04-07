package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/store"
)

func newRoleSetup2(t *testing.T) (*RoleHandler, *store.Entity) {
	t.Helper()
	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()
	e := &store.Entity{
		AHUSKNumber: "AHU-001", Name: "PT X", EntityType: "PT", Status: "ACTIVE",
	}
	entityStore.Create(context.Background(), e)
	return NewRoleHandler(roleStore, entityStore), e
}

// TestAssignRole_MissingEntityID pins the empty-path guard.
func TestAssignRole_MissingEntityID(t *testing.T) {
	h, _ := newRoleSetup2(t)
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{}`))
	req.SetPathValue("entity_id", "")
	rec := httptest.NewRecorder()
	h.AssignRole(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestAssignRole_BadJSON pins the JSON decode error.
func TestAssignRole_BadJSON(t *testing.T) {
	h, e := newRoleSetup2(t)
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("{not json"))
	req.SetPathValue("entity_id", e.ID)
	rec := httptest.NewRecorder()
	h.AssignRole(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestListRoles_Happy pins the success path + empty-list shape.
func TestListRoles_Happy(t *testing.T) {
	h, e := newRoleSetup2(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.SetPathValue("entity_id", e.ID)
	rec := httptest.NewRecorder()
	h.ListRoles(rec, req)
	if rec.Code != 200 {
		t.Errorf("code = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"roles"`)) {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestRevokeRole_NotFound_Cov pins the GetByID ErrRoleNotFound branch.
func TestRevokeRole_NotFound_Cov(t *testing.T) {
	h, e := newRoleSetup2(t)
	req := httptest.NewRequest("DELETE", "/", nil)
	req.SetPathValue("entity_id", e.ID)
	req.SetPathValue("role_id", "missing")
	rec := httptest.NewRecorder()
	h.RevokeRole(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestRevokeRole_Happy pins the successful revoke path.
func TestRevokeRole_Happy(t *testing.T) {
	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()
	e := &store.Entity{AHUSKNumber: "AHU-RH", Name: "PT X", EntityType: "PT", Status: "ACTIVE"}
	entityStore.Create(context.Background(), e)
	role := &store.EntityRole{
		EntityID: e.ID, UserID: "u1", Role: "ADMIN", GrantedBy: "u0", Status: "ACTIVE",
	}
	roleStore.Assign(context.Background(), role)

	h := NewRoleHandler(roleStore, entityStore)
	req := httptest.NewRequest("DELETE", "/", nil)
	req.SetPathValue("entity_id", e.ID)
	req.SetPathValue("role_id", role.ID)
	rec := httptest.NewRecorder()
	h.RevokeRole(rec, req)
	if rec.Code != 200 {
		t.Errorf("code = %d body=%s", rec.Code, rec.Body)
	}
}
