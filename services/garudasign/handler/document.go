package handler

import (
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/hash"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/storage"
	"github.com/garudapass/gpass/services/garudasign/store"
)

// DocumentDeps holds dependencies for the document handler.
type DocumentDeps struct {
	CertStore    store.CertificateStore
	RequestStore store.RequestStore
	DocStore     store.DocumentStore
	FileStorage  *storage.FileStorage
	SignClient   SigningClient
	AuditEmitter audit.Emitter
	MaxSizeMB    int
	RequestTTL   time.Duration
}

// DocumentHandler handles document-related HTTP requests.
type DocumentHandler struct {
	deps DocumentDeps
}

// NewDocumentHandler creates a new DocumentHandler.
func NewDocumentHandler(deps DocumentDeps) *DocumentHandler {
	return &DocumentHandler{deps: deps}
}

// Upload handles POST /api/v1/sign/documents — multipart file upload.
func (h *DocumentHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	maxBytes := int64(h.deps.MaxSizeMB) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Failed to parse multipart form or file too large")
		return
	}

	file, header, err := r.FormFile("document")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "document field is required")
		return
	}
	defer file.Close()

	// Validate PDF extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") {
		writeError(w, http.StatusBadRequest, "invalid_file", "Only PDF files are accepted")
		return
	}

	// Read magic bytes to verify PDF
	magic := make([]byte, 5)
	n, err := file.Read(magic)
	if err != nil || n < 5 || string(magic[:5]) != "%PDF-" {
		writeError(w, http.StatusBadRequest, "invalid_file", "File is not a valid PDF")
		return
	}

	// Seek back to beginning
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process file")
		return
	}

	// Compute hash
	docHash, err := hash.ComputeHash(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to compute hash")
		return
	}

	// Seek back again for storage
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process file")
		return
	}

	// Save file
	path, err := h.deps.FileStorage.Save(header.Filename, file)
	if err != nil {
		slog.Error("failed to save file", "error", err)
		writeError(w, http.StatusInternalServerError, "storage_error", "Failed to save file")
		return
	}

	// Create signing request
	sigReq, err := h.deps.RequestStore.Create(&signing.SigningRequest{
		UserID:       userID,
		DocumentName: header.Filename,
		DocumentSize: header.Size,
		DocumentHash: docHash,
		DocumentPath: path,
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(h.deps.RequestTTL),
	})
	if err != nil {
		slog.Error("failed to create request", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to create signing request")
		return
	}

	h.deps.AuditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionDocUploaded,
		Metadata: map[string]string{
			"request_id":    sigReq.ID,
			"document_name": header.Filename,
		},
	})

	writeJSON(w, http.StatusCreated, sigReq)
}

// Sign handles POST /api/v1/sign/documents/{id}/sign.
func (h *DocumentHandler) Sign(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	requestID := r.PathValue("id")
	if requestID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request ID is required")
		return
	}

	// Get signing request
	sigReq, err := h.deps.RequestStore.GetByID(requestID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Signing request not found")
		return
	}

	// Verify ownership
	if sigReq.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}

	// Check expiry
	if time.Now().After(sigReq.ExpiresAt) {
		writeError(w, http.StatusGone, "expired", "Signing request has expired")
		return
	}

	// Check not already signed
	if sigReq.Status == "COMPLETED" {
		writeError(w, http.StatusConflict, "already_signed", "Document has already been signed")
		return
	}

	// Get active certificate for user
	cert, err := h.deps.CertStore.GetActiveByUser(userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "no_certificate", "No active certificate found")
		return
	}

	// Read the document
	fileReader, err := h.deps.FileStorage.Load(sigReq.DocumentPath)
	if err != nil {
		slog.Error("failed to load document", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load document")
		return
	}
	defer fileReader.Close()

	docBytes, err := io.ReadAll(fileReader)
	if err != nil {
		slog.Error("failed to read document", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to read document")
		return
	}

	// Base64 encode and sign
	docBase64 := base64.StdEncoding.EncodeToString(docBytes)

	signResp, err := h.deps.SignClient.SignDocument(r.Context(), signing.SignRequest{
		DocumentBase64: docBase64,
		CertificatePEM: cert.CertificatePEM,
		SignatureLevel: "PAdES-B-LTA",
	})
	if err != nil {
		slog.Error("failed to sign document", "error", err, "request_id", requestID)
		h.deps.RequestStore.UpdateStatus(requestID, "FAILED", "", err.Error())
		h.deps.AuditEmitter.Emit(audit.Event{
			UserID: userID,
			Action: audit.ActionSignFailed,
			Metadata: map[string]string{
				"request_id": requestID,
				"error":      err.Error(),
			},
		})
		writeError(w, http.StatusInternalServerError, "sign_failed", "Failed to sign document")
		return
	}

	// Decode signed document
	signedBytes, err := base64.StdEncoding.DecodeString(signResp.SignedDocumentBase64)
	if err != nil {
		slog.Error("failed to decode signed document", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to process signed document")
		return
	}

	// Save signed document
	signedPath, err := h.deps.FileStorage.Save("signed_"+sigReq.DocumentName, strings.NewReader(string(signedBytes)))
	if err != nil {
		slog.Error("failed to save signed document", "error", err)
		writeError(w, http.StatusInternalServerError, "storage_error", "Failed to save signed document")
		return
	}

	// Compute signed hash
	signedHash, err := hash.ComputeHash(strings.NewReader(string(signedBytes)))
	if err != nil {
		slog.Error("failed to compute signed hash", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to compute hash")
		return
	}

	sigTimestamp, _ := time.Parse(time.RFC3339, signResp.SignatureTimestamp)

	// Store signed document
	signedDoc, err := h.deps.DocStore.Create(&signing.SignedDocument{
		RequestID:          requestID,
		CertificateID:      cert.ID,
		SignedHash:         signedHash,
		SignedPath:         signedPath,
		SignedSize:         int64(len(signedBytes)),
		PAdESLevel:         signResp.PAdESLevel,
		SignatureTimestamp: sigTimestamp,
	})
	if err != nil {
		slog.Error("failed to store signed document", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to store signed document")
		return
	}

	// Update request status
	h.deps.RequestStore.UpdateStatus(requestID, "COMPLETED", cert.ID, "")

	h.deps.AuditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionDocSigned,
		Metadata: map[string]string{
			"request_id":  requestID,
			"document_id": signedDoc.ID,
			"pades_level": signResp.PAdESLevel,
		},
	})

	writeJSON(w, http.StatusOK, signedDoc)
}

// GetStatus handles GET /api/v1/sign/documents/{id}.
func (h *DocumentHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	requestID := r.PathValue("id")
	if requestID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request ID is required")
		return
	}

	sigReq, err := h.deps.RequestStore.GetByID(requestID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Signing request not found")
		return
	}

	if sigReq.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}

	response := map[string]any{
		"request": sigReq,
	}

	// Include signed document if available
	if sigReq.Status == "COMPLETED" {
		if signedDoc, err := h.deps.DocStore.GetByRequestID(requestID); err == nil {
			response["signed_document"] = signedDoc
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// Download handles GET /api/v1/sign/documents/{id}/download.
func (h *DocumentHandler) Download(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "X-User-ID header is required")
		return
	}

	requestID := r.PathValue("id")
	if requestID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request ID is required")
		return
	}

	sigReq, err := h.deps.RequestStore.GetByID(requestID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Signing request not found")
		return
	}

	if sigReq.UserID != userID {
		writeError(w, http.StatusForbidden, "forbidden", "Access denied")
		return
	}

	if sigReq.Status != "COMPLETED" {
		writeError(w, http.StatusBadRequest, "not_completed", "Document signing is not completed")
		return
	}

	signedDoc, err := h.deps.DocStore.GetByRequestID(requestID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Signed document not found")
		return
	}

	fileReader, err := h.deps.FileStorage.Load(signedDoc.SignedPath)
	if err != nil {
		slog.Error("failed to load signed document", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to load signed document")
		return
	}
	defer fileReader.Close()

	h.deps.AuditEmitter.Emit(audit.Event{
		UserID: userID,
		Action: audit.ActionDocDownloaded,
		Metadata: map[string]string{
			"request_id":  requestID,
			"document_id": signedDoc.ID,
		},
	})

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"signed_%s\"", sigReq.DocumentName))
	io.Copy(w, fileReader)
}
