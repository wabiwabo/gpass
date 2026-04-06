package handler

import (
	"net/http"
)

// PrivacyHandler provides data processing transparency endpoints.
type PrivacyHandler struct{}

// NewPrivacyHandler creates a new PrivacyHandler.
func NewPrivacyHandler() *PrivacyHandler {
	return &PrivacyHandler{}
}

type controllerInfo struct {
	Name     string `json:"name"`
	Contact  string `json:"contact"`
	DPOEmail string `json:"dpo_email"`
}

type processingActivity struct {
	Activity       string   `json:"activity"`
	DataCategories []string `json:"data_categories"`
	Purpose        string   `json:"purpose"`
	LegalBasis     string   `json:"legal_basis"`
	Retention      string   `json:"retention"`
	Recipients     []string `json:"recipients"`
	CrossBorder    bool     `json:"cross_border"`
}

type dataSubjectRights struct {
	Access            string `json:"access"`
	Deletion          string `json:"deletion"`
	ConsentManagement string `json:"consent_management"`
}

type dataProcessingResponse struct {
	Controller           controllerInfo      `json:"controller"`
	ProcessingActivities []processingActivity `json:"processing_activities"`
	DataSubjectRights    dataSubjectRights    `json:"data_subject_rights"`
	LastUpdated          string               `json:"last_updated"`
}

type retentionEntry struct {
	DataCategory string `json:"data_category"`
	Retention    string `json:"retention"`
	LegalBasis   string `json:"legal_basis"`
}

type retentionResponse struct {
	Policies []retentionEntry `json:"policies"`
}

// GetDataProcessingInfo handles GET /api/v1/privacy/processing.
func (h *PrivacyHandler) GetDataProcessingInfo(w http.ResponseWriter, r *http.Request) {
	resp := dataProcessingResponse{
		Controller: controllerInfo{
			Name:     "PT GarudaPass Digital Indonesia",
			Contact:  "privacy@garudapass.id",
			DPOEmail: "dpo@garudapass.id",
		},
		ProcessingActivities: []processingActivity{
			{
				Activity:       "identity_verification",
				DataCategories: []string{"nik", "name", "dob", "biometric"},
				Purpose:        "Verifikasi identitas penduduk Indonesia",
				LegalBasis:     "consent",
				Retention:      "5 years per PP 71/2019",
				Recipients:     []string{"dukcapil (verification only)"},
				CrossBorder:    false,
			},
			{
				Activity:       "consent_management",
				DataCategories: []string{"nik", "name", "consent_records"},
				Purpose:        "Pengelolaan persetujuan penggunaan data pribadi",
				LegalBasis:     "legal_obligation",
				Retention:      "5 years per PP 71/2019",
				Recipients:     []string{"internal (garudainfo)"},
				CrossBorder:    false,
			},
			{
				Activity:       "document_signing",
				DataCategories: []string{"nik", "name", "signature", "certificate"},
				Purpose:        "Tanda tangan elektronik dokumen resmi",
				LegalBasis:     "consent",
				Retention:      "10 years per UU ITE",
				Recipients:     []string{"bsre (certificate authority)"},
				CrossBorder:    false,
			},
			{
				Activity:       "audit_logging",
				DataCategories: []string{"user_id", "ip_address", "user_agent", "action"},
				Purpose:        "Pencatatan jejak audit untuk kepatuhan regulasi",
				LegalBasis:     "legal_obligation",
				Retention:      "5 years per PP 71/2019",
				Recipients:     []string{"internal (garudaaudit)"},
				CrossBorder:    false,
			},
			{
				Activity:       "business_registration",
				DataCategories: []string{"nik", "name", "company_data", "nib"},
				Purpose:        "Pendaftaran dan verifikasi entitas usaha",
				LegalBasis:     "consent",
				Retention:      "5 years per PP 71/2019",
				Recipients:     []string{"oss (bkpm)", "ahu (kemenkumham)"},
				CrossBorder:    false,
			},
		},
		DataSubjectRights: dataSubjectRights{
			Access:            "/api/v1/identity/export",
			Deletion:          "/api/v1/identity/deletion",
			ConsentManagement: "/api/v1/consent",
		},
		LastUpdated: "2026-04-06",
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetRetentionPolicy handles GET /api/v1/privacy/retention.
func (h *PrivacyHandler) GetRetentionPolicy(w http.ResponseWriter, r *http.Request) {
	resp := retentionResponse{
		Policies: []retentionEntry{
			{DataCategory: "nik", Retention: "5 years", LegalBasis: "PP 71/2019"},
			{DataCategory: "name", Retention: "5 years", LegalBasis: "PP 71/2019"},
			{DataCategory: "dob", Retention: "5 years", LegalBasis: "PP 71/2019"},
			{DataCategory: "address", Retention: "5 years", LegalBasis: "PP 71/2019"},
			{DataCategory: "biometric", Retention: "5 years", LegalBasis: "PP 71/2019 + UU PDP Article 25"},
			{DataCategory: "consent_records", Retention: "5 years", LegalBasis: "PP 71/2019"},
			{DataCategory: "audit_logs", Retention: "5 years", LegalBasis: "PP 71/2019"},
			{DataCategory: "signature", Retention: "10 years", LegalBasis: "UU ITE"},
			{DataCategory: "certificate", Retention: "10 years", LegalBasis: "UU ITE"},
		},
	}

	writeJSON(w, http.StatusOK, resp)
}
