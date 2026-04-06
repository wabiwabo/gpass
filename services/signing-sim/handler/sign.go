package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/signing-sim/ca"
	"github.com/garudapass/gpass/services/signing-sim/pades"
)

type signRequest struct {
	DocumentBase64 string `json:"document_base64"`
	CertificatePEM string `json:"certificate_pem"`
	SignatureLevel string `json:"signature_level"`
}

type signResponse struct {
	SignedDocumentBase64 string `json:"signed_document_base64"`
	SignatureTimestamp    string `json:"signature_timestamp"`
	PAdESLevel           string `json:"pades_level"`
}

// SignHandler handles signing-related HTTP requests.
type SignHandler struct {
	ca *ca.CA
}

// NewSignHandler creates a new SignHandler.
func NewSignHandler(c *ca.CA) *SignHandler {
	return &SignHandler{ca: c}
}

// Sign handles POST /sign/pades.
func (h *SignHandler) Sign(w http.ResponseWriter, r *http.Request) {
	var req signRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.DocumentBase64 == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "document_base64 is required")
		return
	}

	if req.CertificatePEM == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "certificate_pem is required")
		return
	}

	// Find the private key by matching the certificate PEM
	var matchedCert *ca.IssuedCertificate
	for _, serial := range h.ca.ListSerials() {
		cert, ok := h.ca.GetCertificate(serial)
		if ok && cert.CertificatePEM == req.CertificatePEM {
			matchedCert = cert
			break
		}
	}

	if matchedCert == nil {
		writeError(w, http.StatusBadRequest, "unknown_certificate", "Certificate not found in issued certificates")
		return
	}

	result, err := pades.SignPAdES(req.DocumentBase64, req.CertificatePEM, matchedCert.PrivateKey)
	if err != nil {
		slog.Error("failed to sign document", "error", err)
		writeError(w, http.StatusBadRequest, "signing_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, signResponse{
		SignedDocumentBase64: result.SignedDocumentBase64,
		SignatureTimestamp:   result.SignatureTimestamp,
		PAdESLevel:          result.PAdESLevel,
	})
}
