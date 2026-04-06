package handler

import (
	"encoding/json"
	"net/http"
	"slices"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/store"
)

// ValidRevocationReasons per RFC 5280 Section 5.3.1.
var ValidRevocationReasons = []string{
	"key_compromise",
	"affiliation_changed",
	"superseded",
	"cessation_of_operation",
	"privilege_withdrawn",
}

// RevocationHandler handles certificate revocation requests.
type RevocationHandler struct {
	certStore    store.CertificateStore
	auditEmitter audit.Emitter
}

// NewRevocationHandler creates a new RevocationHandler.
func NewRevocationHandler(certStore store.CertificateStore, auditEmitter audit.Emitter) *RevocationHandler {
	return &RevocationHandler{
		certStore:    certStore,
		auditEmitter: auditEmitter,
	}
}

type revokeRequest struct {
	Reason string `json:"reason"`
}

// RevokeCertificate handles POST /api/v1/sign/certificates/{id}/revoke.
func (h *RevocationHandler) RevokeCertificate(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	certID := r.PathValue("id")
	if certID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "certificate id is required")
		return
	}

	var req revokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if !slices.Contains(ValidRevocationReasons, req.Reason) {
		writeError(w, http.StatusBadRequest, "invalid_reason", "Invalid revocation reason")
		return
	}

	cert, err := h.certStore.GetByID(certID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Certificate not found")
		return
	}

	if cert.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "You do not own this certificate")
		return
	}

	if cert.Status == "REVOKED" {
		writeError(w, http.StatusConflict, "already_revoked", "Certificate is already revoked")
		return
	}

	now := time.Now().UTC()
	if err := h.certStore.UpdateStatus(certID, "REVOKED", &now, req.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, "revoke_failed", "Failed to revoke certificate")
		return
	}

	h.auditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionCertRevoked,
		Metadata: map[string]string{
			"certificate_id":    certID,
			"revocation_reason": req.Reason,
		},
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"id":                certID,
		"status":            "REVOKED",
		"revocation_reason": req.Reason,
		"revoked_at":        now.Format(time.RFC3339),
	})
}
