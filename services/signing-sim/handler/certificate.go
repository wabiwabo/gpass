package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/signing-sim/ca"
)

type issueRequest struct {
	SubjectCN    string `json:"subject_cn"`
	SubjectUID   string `json:"subject_uid"`
	ValidityDays int    `json:"validity_days"`
}

type issueResponse struct {
	SerialNumber      string `json:"serial_number"`
	CertificatePEM    string `json:"certificate_pem"`
	IssuerDN          string `json:"issuer_dn"`
	SubjectDN         string `json:"subject_dn"`
	ValidFrom         string `json:"valid_from"`
	ValidTo           string `json:"valid_to"`
	FingerprintSHA256 string `json:"fingerprint_sha256"`
}

// CertificateHandler handles certificate-related HTTP requests.
type CertificateHandler struct {
	ca *ca.CA
}

// NewCertificateHandler creates a new CertificateHandler.
func NewCertificateHandler(c *ca.CA) *CertificateHandler {
	return &CertificateHandler{ca: c}
}

// Issue handles POST /certificates/issue.
func (h *CertificateHandler) Issue(w http.ResponseWriter, r *http.Request) {
	var req issueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.SubjectCN == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "subject_cn is required")
		return
	}

	if req.ValidityDays <= 0 {
		req.ValidityDays = 365
	}

	cert, err := h.ca.IssueCertificate(req.SubjectCN, req.SubjectUID, req.ValidityDays)
	if err != nil {
		slog.Error("failed to issue certificate", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to issue certificate")
		return
	}

	writeJSON(w, http.StatusOK, issueResponse{
		SerialNumber:      cert.SerialNumber,
		CertificatePEM:    cert.CertificatePEM,
		IssuerDN:          cert.IssuerDN,
		SubjectDN:         cert.SubjectDN,
		ValidFrom:         cert.ValidFrom.UTC().Format("2006-01-02T15:04:05Z"),
		ValidTo:           cert.ValidTo.UTC().Format("2006-01-02T15:04:05Z"),
		FingerprintSHA256: cert.FingerprintSHA256,
	})
}
