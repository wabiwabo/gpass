package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/store"
)

// reqWithPath builds a request with PathValue populated for Go 1.22+
// method routing. We register the route on a tiny mux so PathValue works.
func reqWithRoles(method, pattern, path string, body string, h http.HandlerFunc) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc(pattern, h)
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// TestAssignRole_MissingFields pins each input-validation rejection.
func TestAssignRole_MissingFields(t *testing.T) {
	h, _, _, entityID := setupRoleTest(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"missing user_id", `{"role":"USER","caller_user_id":"c"}`, "user_id"},
		{"missing role", `{"user_id":"u","caller_user_id":"c"}`, "role"},
		{"missing caller", `{"user_id":"u","role":"USER"}`, "caller_user_id"},
		{"bad json", `{not json`, "Invalid JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := reqWithRoles("POST", "POST /api/v1/corp/entities/{entity_id}/roles",
				"/api/v1/corp/entities/"+entityID+"/roles", tc.body, h.AssignRole)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("code = %d, want 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("body missing %q: %s", tc.want, rec.Body)
			}
		})
	}
}

// TestAssignRole_EntityNotFound pins the GetByID → ErrEntityNotFound branch.
func TestAssignRole_EntityNotFound(t *testing.T) {
	h, _, _, _ := setupRoleTest(t)
	body := `{"user_id":"u","role":"USER","caller_user_id":"c"}`
	rec := reqWithRoles("POST", "POST /api/v1/corp/entities/{entity_id}/roles",
		"/api/v1/corp/entities/missing-entity/roles", body, h.AssignRole)
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "entity_not_found") {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestAssignRole_CallerHasNoRole pins the GetUserRole → ErrRoleNotFound
// branch — caller without a role in the entity must get 403.
func TestAssignRole_CallerHasNoRole(t *testing.T) {
	h, _, _, entityID := setupRoleTest(t)
	body := `{"user_id":"u","role":"USER","caller_user_id":"unknown-caller"}`
	rec := reqWithRoles("POST", "POST /api/v1/corp/entities/{entity_id}/roles",
		"/api/v1/corp/entities/"+entityID+"/roles", body, h.AssignRole)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

// TestListRoles_MissingEntityID pins the empty entity_id branch.
func TestListRoles_MissingEntityID(t *testing.T) {
	h, _, _, _ := setupRoleTest(t)
	// Register a pattern that captures empty {entity_id} via direct call.
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ListRoles(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d, want 400", rec.Code)
	}
}

// TestRevokeRole_MissingPathValues pins the entity_id+role_id empty
// branch.
func TestRevokeRole_MissingPathValues(t *testing.T) {
	h, _, _, _ := setupRoleTest(t)
	req := httptest.NewRequest("DELETE", "/", nil)
	rec := httptest.NewRecorder()
	h.RevokeRole(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestRevokeRole_RoleBelongsToDifferentEntity pins the cross-entity
// rejection branch (role exists but in another entity).
func TestRevokeRole_RoleBelongsToDifferentEntity(t *testing.T) {
	h, _, roleStore, entityID := setupRoleTest(t)
	ctx := context.Background()

	// Create a role in entity A.
	r := &store.EntityRole{
		EntityID: entityID,
		UserID:   "u",
		Role:     "USER",
	}
	roleStore.Assign(ctx, r)

	// Try to revoke it under a different entity ID.
	rec := reqWithRoles("DELETE", "DELETE /api/v1/corp/entities/{entity_id}/roles/{role_id}",
		"/api/v1/corp/entities/wrong-entity/roles/"+r.ID, "", h.RevokeRole)
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d, want 404 (cross-entity revoke)", rec.Code)
	}
}

// TestRevokeRole_AlreadyRevoked pins the ErrRoleAlreadyRevoked branch.
func TestRevokeRole_AlreadyRevoked(t *testing.T) {
	h, _, roleStore, entityID := setupRoleTest(t)
	ctx := context.Background()

	r := &store.EntityRole{EntityID: entityID, UserID: "u", Role: "USER"}
	roleStore.Assign(ctx, r)
	roleStore.Revoke(ctx, r.ID) // first revoke

	// Second revoke must return 409.
	rec := reqWithRoles("DELETE", "DELETE /api/v1/corp/entities/{entity_id}/roles/{role_id}",
		"/api/v1/corp/entities/"+entityID+"/roles/"+r.ID, "", h.RevokeRole)
	if rec.Code != http.StatusConflict {
		t.Errorf("code = %d, want 409", rec.Code)
	}
}
