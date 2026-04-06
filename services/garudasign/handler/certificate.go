package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/store"
)

// SigningClient defines the interface for the signing backend.
type SigningClient interface {
	IssueCertificate(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error)
	SignDocument(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error)
}

// CertificateDeps holds dependencies for the certificate handler.
type CertificateDeps struct {
	CertStore    store.CertificateStore
	SignClient   SigningClient
	AuditEmitter audit.Emitter
	ValidityDays int
}

// CertificateHandler handles certificate-related HTTP requests.
type CertificateHandler struct {
	deps CertificateDeps
}

// NewCertificateHandler creates a new CertificateHandler.
func NewCertificateHandler(deps CertificateDeps) *CertificateHandler {
	return &CertificateHandler{deps: deps}
}

type certRequest struct {
	SubjectCN string `json:"subject_cn"`
}

// RequestCertificate handles POST certificate requests.
func (h *CertificateHandler) RequestCertificate(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	// Check for existing active certificate
	if _, err := h.deps.CertStore.GetActiveByUser(userID); err == nil {
		writeError(w, http.StatusConflict, "active_cert_exists", "User already has an active certificate")
		return
	}

	var req certRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.SubjectCN == "" {
		req.SubjectCN = userID
	}

	// Issue certificate via backend
	issueResp, err := h.deps.SignClient.IssueCertificate(r.Context(), signing.CertificateIssueRequest{
		SubjectCN:    req.SubjectCN,
		SubjectUID:   userID,
		ValidityDays: h.deps.ValidityDays,
	})
	if err != nil {
		slog.Error("failed to issue certificate", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "issue_failed", "Failed to issue certificate")
		return
	}

	validFrom, _ := time.Parse(time.RFC3339, issueResp.ValidFrom)
	validTo, _ := time.Parse(time.RFC3339, issueResp.ValidTo)

	cert, err := h.deps.CertStore.Create(&signing.Certificate{
		UserID:            userID,
		SerialNumber:      issueResp.SerialNumber,
		IssuerDN:          issueResp.IssuerDN,
		SubjectDN:         issueResp.SubjectDN,
		Status:            "ACTIVE",
		ValidFrom:         validFrom,
		ValidTo:           validTo,
		CertificatePEM:    issueResp.CertificatePEM,
		FingerprintSHA256: issueResp.FingerprintSHA256,
	})
	if err != nil {
		slog.Error("failed to store certificate", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "store_failed", "Failed to store certificate")
		return
	}

	h.deps.AuditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionCertIssued,
		Metadata: map[string]string{
			"certificate_id": cert.ID,
			"serial_number":  cert.SerialNumber,
		},
	})

	writeJSON(w, http.StatusCreated, cert)
}

// ListCertificates handles GET certificate list requests.
func (h *CertificateHandler) ListCertificates(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	statusFilter := r.URL.Query().Get("status")

	certs, err := h.deps.CertStore.ListByUser(userID, statusFilter)
	if err != nil {
		slog.Error("failed to list certificates", "error", err, "user_id", userID)
		writeError(w, http.StatusInternalServerError, "list_failed", "Failed to list certificates")
		return
	}

	if certs == nil {
		certs = []*signing.Certificate{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"certificates": certs,
	})
}
