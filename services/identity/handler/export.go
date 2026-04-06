package handler

import (
	"encoding/json"
	"net/http"
	"time"
)

// ExportHandler handles personal data export requests (UU PDP compliance).
type ExportHandler struct{}

// NewExportHandler creates a new ExportHandler.
func NewExportHandler() *ExportHandler {
	return &ExportHandler{}
}

type exportResponse struct {
	ExportTimestamp time.Time        `json:"export_timestamp"`
	DataCategories []string         `json:"data_categories"`
	PersonalData   personalDataExport `json:"personal_data"`
}

type personalDataExport struct {
	UserID             string          `json:"user_id"`
	MaskedNIK          string          `json:"masked_nik"`
	VerificationStatus string          `json:"verification_status"`
	ConsentList        []consentRecord `json:"consent_list"`
}

type consentRecord struct {
	Purpose   string    `json:"purpose"`
	Granted   bool      `json:"granted"`
	GrantedAt time.Time `json:"granted_at"`
}

// ExportData handles GET /api/v1/identity/export.
// It returns all personal data associated with the user (UU PDP right to access).
func (h *ExportHandler) ExportData(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	// In a full implementation, this would query the database for the user's data.
	// For now, return a structured response with placeholder data.
	resp := exportResponse{
		ExportTimestamp: time.Now().UTC(),
		DataCategories: []string{
			"identity",
			"verification",
			"consent",
		},
		PersonalData: personalDataExport{
			UserID:             userID,
			MaskedNIK:          "3201****0001",
			VerificationStatus: "verified",
			ConsentList: []consentRecord{
				{
					Purpose:   "identity_verification",
					Granted:   true,
					GrantedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
				{
					Purpose:   "data_sharing",
					Granted:   true,
					GrantedAt: time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
