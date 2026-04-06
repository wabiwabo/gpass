package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudacorp/store"
)

// RoleHandler handles role management endpoints.
type RoleHandler struct {
	roleStore   store.RoleStore
	entityStore store.EntityStore
}

// NewRoleHandler creates a new role handler.
func NewRoleHandler(roleStore store.RoleStore, entityStore store.EntityStore) *RoleHandler {
	return &RoleHandler{
		roleStore:   roleStore,
		entityStore: entityStore,
	}
}

type assignRoleRequest struct {
	UserID        string   `json:"user_id"`
	Role          string   `json:"role"`
	ServiceAccess []string `json:"service_access"`
	CallerUserID  string   `json:"caller_user_id"`
}

type assignRoleResponse struct {
	RoleID string `json:"role_id"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// AssignRole handles POST /api/v1/corp/entities/{entity_id}/roles.
func (h *RoleHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entity_id")
	if entityID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "entity_id is required")
		return
	}

	var req assignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.UserID == "" || req.Role == "" || req.CallerUserID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "user_id, role, and caller_user_id are required")
		return
	}

	ctx := r.Context()

	// Check entity exists
	if _, err := h.entityStore.GetByID(ctx, entityID); err != nil {
		if err == store.ErrEntityNotFound {
			writeError(w, http.StatusNotFound, "entity_not_found", "Entity not found")
			return
		}
		slog.Error("failed to get entity", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve entity")
		return
	}

	// Get caller's role
	callerRole, err := h.roleStore.GetUserRole(ctx, entityID, req.CallerUserID)
	if err != nil {
		if err == store.ErrRoleNotFound {
			writeError(w, http.StatusForbidden, "forbidden", "Caller has no role in this entity")
			return
		}
		slog.Error("failed to get caller role", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to check permissions")
		return
	}

	// Validate role hierarchy
	if err := store.ValidateRoleAssignment(callerRole.Role, req.Role); err != nil {
		writeError(w, http.StatusForbidden, "forbidden", err.Error())
		return
	}

	// Assign the role
	role := &store.EntityRole{
		EntityID:      entityID,
		UserID:        req.UserID,
		Role:          req.Role,
		GrantedBy:     req.CallerUserID,
		ServiceAccess: req.ServiceAccess,
	}
	if err := h.roleStore.Assign(ctx, role); err != nil {
		slog.Error("failed to assign role", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to assign role")
		return
	}

	resp := assignRoleResponse{
		RoleID: role.ID,
		Role:   role.Role,
		Status: role.Status,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// ListRoles handles GET /api/v1/corp/entities/{entity_id}/roles.
func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entity_id")
	if entityID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "entity_id is required")
		return
	}

	roles, err := h.roleStore.ListByEntity(r.Context(), entityID)
	if err != nil {
		slog.Error("failed to list roles", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to list roles")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"roles": roles,
	})
}

// RevokeRole handles DELETE /api/v1/corp/entities/{entity_id}/roles/{role_id}.
func (h *RoleHandler) RevokeRole(w http.ResponseWriter, r *http.Request) {
	entityID := r.PathValue("entity_id")
	roleID := r.PathValue("role_id")

	if entityID == "" || roleID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "entity_id and role_id are required")
		return
	}

	// Verify role belongs to entity
	role, err := h.roleStore.GetByID(r.Context(), roleID)
	if err != nil {
		if err == store.ErrRoleNotFound {
			writeError(w, http.StatusNotFound, "role_not_found", "Role not found")
			return
		}
		slog.Error("failed to get role", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve role")
		return
	}

	if role.EntityID != entityID {
		writeError(w, http.StatusNotFound, "role_not_found", "Role not found in this entity")
		return
	}

	if err := h.roleStore.Revoke(r.Context(), roleID); err != nil {
		if err == store.ErrRoleAlreadyRevoked {
			writeError(w, http.StatusConflict, "already_revoked", "Role is already revoked")
			return
		}
		slog.Error("failed to revoke role", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to revoke role")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "revoked",
	})
}
