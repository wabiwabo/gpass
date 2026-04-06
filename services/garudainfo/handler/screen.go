package handler

import (
	"net/http"
	"strings"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

// FieldInfo defines the display metadata for consent fields.
type FieldInfo struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// FieldMetadata defines the display metadata for consent fields.
var FieldMetadata = map[string]FieldInfo{
	"name":    {Label: "Nama Lengkap", Description: "Nama sesuai KTP", Category: "identity"},
	"dob":     {Label: "Tanggal Lahir", Description: "Tanggal lahir sesuai KTP", Category: "identity"},
	"gender":  {Label: "Jenis Kelamin", Description: "Jenis kelamin", Category: "identity"},
	"address": {Label: "Alamat", Description: "Alamat sesuai KTP", Category: "contact"},
	"phone":   {Label: "Nomor Telepon", Description: "Nomor telepon terdaftar", Category: "contact"},
	"email":   {Label: "Email", Description: "Alamat email terdaftar", Category: "contact"},
	"nik":     {Label: "NIK (Masked)", Description: "NIK yang disamarkan", Category: "identity"},
	"marital": {Label: "Status Perkawinan", Description: "Status perkawinan", Category: "identity"},
}

// PurposeLabels maps purpose codes to Indonesian labels.
var PurposeLabels = map[string]string{
	"kyc_verification":  "Verifikasi Identitas (KYC)",
	"account_opening":   "Pembukaan Rekening",
	"loan_application":  "Pengajuan Pinjaman",
	"insurance_claim":   "Klaim Asuransi",
	"employment_check":  "Verifikasi Ketenagakerjaan",
	"background_check":  "Pemeriksaan Latar Belakang",
}

// requestedField represents a field in the consent screen response.
type requestedField struct {
	Field       string `json:"field"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// appInfo represents app metadata in the consent screen response.
type appInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// consentScreenResponse is the response for GET /api/v1/consent/screen.
type consentScreenResponse struct {
	App             appInfo          `json:"app"`
	RequestedFields []requestedField `json:"requested_fields"`
	Purpose         string           `json:"purpose"`
	PurposeLabel    string           `json:"purpose_label"`
	ExpiresIn       string           `json:"expires_in"`
}

// ScreenHandler provides consent screen data for the OAuth2 authorization flow.
type ScreenHandler struct {
	consentStore store.ConsentStore
}

// NewScreenHandler creates a new ScreenHandler.
func NewScreenHandler(s store.ConsentStore) *ScreenHandler {
	return &ScreenHandler{consentStore: s}
}

// GetConsentScreen handles GET /api/v1/consent/screen.
// Query: ?app_id=<uuid>&scope=name,dob,address&purpose=kyc_verification
func (h *ScreenHandler) GetConsentScreen(w http.ResponseWriter, r *http.Request) {
	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "app_id query parameter is required")
		return
	}

	scopeParam := r.URL.Query().Get("scope")
	if scopeParam == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "scope query parameter is required")
		return
	}

	purpose := r.URL.Query().Get("purpose")

	scopes := strings.Split(scopeParam, ",")

	// Build requested fields from known metadata; skip unknown fields.
	var fields []requestedField
	for _, s := range scopes {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		meta, ok := FieldMetadata[s]
		if !ok {
			continue
		}
		fields = append(fields, requestedField{
			Field:       s,
			Label:       meta.Label,
			Description: meta.Description,
			Required:    meta.Category == "identity",
		})
	}

	// Resolve purpose label.
	purposeLabel := purpose
	if label, ok := PurposeLabels[purpose]; ok {
		purposeLabel = label
	}

	resp := consentScreenResponse{
		App: appInfo{
			Name:        "Application " + appID,
			Description: "Third-party application requesting data access",
		},
		RequestedFields: fields,
		Purpose:         purpose,
		PurposeLabel:    purposeLabel,
		ExpiresIn:       "365d",
	}

	writeJSON(w, http.StatusOK, resp)
}
