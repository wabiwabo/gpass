package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/identity/store"
)

// AuditEmitter emits audit events for compliance logging (PP 71/2019).
type AuditEmitter interface {
	Emit(eventType, userID, resourceID string, metadata map[string]string) error
}

// DeletionHandler handles personal data deletion requests per UU PDP No. 27/2022 Article 8.
type DeletionHandler struct {
	store        store.DeletionStore
	auditEmitter AuditEmitter
}

// NewDeletionHandler creates a new DeletionHandler.
func NewDeletionHandler(s store.DeletionStore, ae AuditEmitter) *DeletionHandler {
	return &DeletionHandler{
		store:        s,
		auditEmitter: ae,
	}
}

type deletionRequestBody struct {
	Reason string `json:"reason"`
}

type deletionResponse struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	RequestedAt time.Time `json:"requested_at"`
}

type deletionStatusResponse struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"`
	RequestedAt time.Time  `json:"requested_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DeletedData []string   `json:"deleted_data,omitempty"`
}

// RequestDeletion handles POST /api/v1/identity/deletion.
// It creates a deletion request, marks user data for deletion, and emits an audit event.
// Returns 202 Accepted as processing is asynchronous.
func (h *DeletionHandler) RequestDeletion(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	var body deletionRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	req := &store.DeletionRequest{
		UserID: userID,
		Reason: body.Reason,
	}

	if err := h.store.Create(req); err != nil {
		if err == store.ErrInvalidReason {
			writeError(w, http.StatusBadRequest, "invalid_reason",
				"Reason must be one of: user_request, consent_revocation, retention_expired")
			return
		}
		slog.Error("failed to create deletion request", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create deletion request")
		return
	}

	// PP 71/2019 requires auditing even the deletion operation itself.
	if err := h.auditEmitter.Emit("data_deletion.requested", userID, req.ID, map[string]string{
		"reason": body.Reason,
	}); err != nil {
		slog.Error("failed to emit audit event for deletion request", "error", err, "request_id", req.ID)
		// Do not fail the request — the deletion was created successfully.
	}

	resp := deletionResponse{
		ID:          req.ID,
		Status:      req.Status,
		RequestedAt: req.RequestedAt,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}

// GetDeletionStatus handles GET /api/v1/identity/deletion/{id}.
// It returns the status of a deletion request. Only the owning user may view it.
func (h *DeletionHandler) GetDeletionStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Deletion request ID is required")
		return
	}

	req, err := h.store.GetByID(id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "Deletion request not found")
			return
		}
		slog.Error("failed to get deletion request", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve deletion request")
		return
	}

	// Only the owning user can view the deletion request.
	if req.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "You do not have access to this deletion request")
		return
	}

	resp := deletionStatusResponse{
		ID:          req.ID,
		UserID:      req.UserID,
		Reason:      req.Reason,
		Status:      req.Status,
		RequestedAt: req.RequestedAt,
		CompletedAt: req.CompletedAt,
		DeletedData: req.DeletedData,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
