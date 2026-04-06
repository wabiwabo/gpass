package handler

import (
	"errors"
	"net/http"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

// FieldValue represents a single verified data field.
type FieldValue struct {
	Value        string `json:"value"`
	Source       string `json:"source"`
	LastVerified string `json:"last_verified"`
}

// UserDataProvider provides user field data from upstream sources.
type UserDataProvider interface {
	GetUserFields(userID string) map[string]FieldValue
}

// PersonResponse is the API response for person data queries.
type PersonResponse struct {
	Fields map[string]FieldValue `json:"fields"`
}

// PersonHandler handles HTTP requests for person data retrieval.
type PersonHandler struct {
	consentStore store.ConsentStore
	userData     UserDataProvider
}

// NewPersonHandler creates a new PersonHandler.
func NewPersonHandler(consentStore store.ConsentStore, userData UserDataProvider) *PersonHandler {
	return &PersonHandler{
		consentStore: consentStore,
		userData:     userData,
	}
}

// GetPerson handles GET requests to retrieve person data filtered by consent.
func (h *PersonHandler) GetPerson(w http.ResponseWriter, r *http.Request) {
	consentID := r.URL.Query().Get("consent_id")
	if consentID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "consent_id query parameter is required")
		return
	}

	consent, err := h.consentStore.GetByID(r.Context(), consentID)
	if err != nil {
		if errors.Is(err, store.ErrConsentNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "consent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to look up consent")
		return
	}

	if consent.Status != "ACTIVE" {
		writeError(w, http.StatusForbidden, "consent_inactive", "consent is not active")
		return
	}

	allFields := h.userData.GetUserFields(consent.UserID)

	// Filter to only consented fields
	filtered := make(map[string]FieldValue)
	for field, granted := range consent.Fields {
		if granted {
			if val, ok := allFields[field]; ok {
				filtered[field] = val
			}
		}
	}

	writeJSON(w, http.StatusOK, PersonResponse{Fields: filtered})
}
