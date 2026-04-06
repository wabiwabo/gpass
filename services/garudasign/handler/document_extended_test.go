package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// TestUpload_FileTooLarge verifies that uploading a file exceeding the
// configured MaxSizeMB limit returns HTTP 400 (via MaxBytesReader error).
func TestUpload_FileTooLarge(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	deps.MaxSizeMB = 1 // 1MB limit for this test
	handler := NewDocumentHandler(deps)

	// Create content just over 1MB
	bigContent := make([]byte, 1*1024*1024+1024)
	copy(bigContent[:5], []byte("%PDF-"))

	req, _ := createMultipartRequest(t, "large.pdf", bigContent)
	req.Header.Set("X-User-ID", "user-1")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	// MaxBytesReader triggers a 400 because ParseMultipartForm fails
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for oversized file, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_request" {
		t.Errorf("expected error code invalid_request, got %s", errResp["error"])
	}
}

// TestUpload_EmptyFile verifies that uploading a zero-byte file returns 400
// because the PDF magic bytes validation fails.
func TestUpload_EmptyFile(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req, _ := createMultipartRequest(t, "empty.pdf", []byte{})
	req.Header.Set("X-User-ID", "user-1")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty file, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_file" {
		t.Errorf("expected error code invalid_file, got %s", errResp["error"])
	}
}

// TestSign_NonExistentRequest verifies that signing with a non-existent
// request ID returns 404.
func TestSign_NonExistentRequest(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents/nonexistent/sign", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", "nonexistent-id-12345")

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "not_found" {
		t.Errorf("expected not_found error, got %s", errResp["error"])
	}
}

// TestSign_AlreadyCompleted verifies that attempting to sign a document
// that has already been completed returns 409 Conflict.
func TestSign_AlreadyCompleted(t *testing.T) {
	deps, reqStore, certStore := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	// Create a completed signing request
	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:    "user-1",
		Status:    "COMPLETED",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	// Create an active certificate for the user
	certStore.Create(&signing.Certificate{
		UserID:         "user-1",
		Status:         "ACTIVE",
		SerialNumber:   "SN-TEST",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "already_signed" {
		t.Errorf("expected already_signed error, got %s", errResp["error"])
	}
}

// TestDownload_BeforeSigning verifies that downloading a document that
// has not yet been signed returns 400 (not_completed).
func TestDownload_BeforeSigning(t *testing.T) {
	deps, reqStore, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:    "user-1",
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Download(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for download before signing, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "not_completed" {
		t.Errorf("expected not_completed error, got %s", errResp["error"])
	}
}

// TestUpload_NonPDFContentTypeButValidMagicBytes verifies that a file
// with a .pdf extension and valid PDF magic bytes succeeds, regardless
// of the Content-Type header on the multipart part.
func TestUpload_NonPDFContentTypeButValidMagicBytes(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	pdfContent := createPDFContent()

	// Create a multipart form manually with application/octet-stream content type
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("document", "test.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write(pdfContent)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "user-1")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 for valid PDF magic bytes, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpload_MultipleUploadsBySameUser verifies that multiple uploads
// by the same user create separate, independently tracked signing requests.
func TestUpload_MultipleUploadsBySameUser(t *testing.T) {
	deps, reqStore, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	ids := make(map[string]bool)

	for i := 0; i < 5; i++ {
		filename := fmt.Sprintf("document_%d.pdf", i)
		req, _ := createMultipartRequest(t, filename, createPDFContent())
		req.Header.Set("X-User-ID", "user-multi")

		w := httptest.NewRecorder()
		handler.Upload(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("upload %d: expected 201, got %d: %s", i, w.Code, w.Body.String())
		}

		var resp signing.SigningRequest
		json.NewDecoder(w.Body).Decode(&resp)

		if ids[resp.ID] {
			t.Errorf("upload %d: duplicate request ID %s", i, resp.ID)
		}
		ids[resp.ID] = true
	}

	// Verify all 5 requests are stored
	requests, err := reqStore.ListByUser("user-multi")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(requests) != 5 {
		t.Errorf("expected 5 stored requests, got %d", len(requests))
	}

	// Each should be PENDING
	for _, r := range requests {
		if r.Status != "PENDING" {
			t.Errorf("request %s: expected PENDING, got %s", r.ID, r.Status)
		}
	}
}

// TestSign_NoCertificate verifies that signing fails with a helpful error
// when the user has no active certificate.
func TestSign_NoCertificate(t *testing.T) {
	deps, reqStore, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:    "user-no-cert",
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-no-cert")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "no_certificate" {
		t.Errorf("expected no_certificate error, got %s", errResp["error"])
	}
}

// TestSign_MissingUserID verifies that the sign endpoint rejects requests
// without the X-User-ID header.
func TestSign_MissingUserID(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.SetPathValue("id", "some-id")

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "missing_user_id" {
		t.Errorf("expected missing_user_id, got %s", errResp["error"])
	}
}

// TestSign_MissingPathID verifies that the sign endpoint returns 400
// when the path value for ID is empty.
func TestSign_MissingPathID(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	// No path value set

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestDownload_NonExistentRequest verifies that downloading with a
// non-existent request ID returns 404.
func TestDownload_NonExistentRequest(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", "nonexistent-download-id")

	w := httptest.NewRecorder()
	handler.Download(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestDownload_NotOwner verifies that a different user cannot download
// another user's signed document.
func TestDownload_NotOwner(t *testing.T) {
	deps, reqStore, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:    "user-1",
		Status:    "COMPLETED",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-other")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Download(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestGetStatus_CompletedWithSignedDocument verifies that GetStatus includes
// the signed document data when the request is completed.
func TestGetStatus_CompletedWithSignedDocument(t *testing.T) {
	deps, reqStore, certStore := newDocumentDeps(t)

	// Set up signing mock
	signedContent := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 signed content"))
	deps.SignClient = &mockSigningClient{
		issueFn: defaultMockClient().issueFn,
		signFn: func(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
			return &signing.SignResponse{
				SignedDocumentBase64: signedContent,
				SignatureTimestamp:   "2025-01-01T00:00:00Z",
				PAdESLevel:          "PAdES-B-LTA",
			}, nil
		},
	}
	handler := NewDocumentHandler(deps)

	// Save file and create request
	pdfContent := createPDFContent()
	path, _ := deps.FileStorage.Save("test.pdf", bytes.NewReader(pdfContent))

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:       "user-status",
		DocumentName: "test.pdf",
		DocumentSize: int64(len(pdfContent)),
		DocumentHash: "abc123",
		DocumentPath: path,
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	})

	certStore.Create(&signing.Certificate{
		UserID:         "user-status",
		Status:         "ACTIVE",
		SerialNumber:   "SN-STATUS",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
	})

	// Sign the document first
	signReq := httptest.NewRequest(http.MethodPost, "/", nil)
	signReq.Header.Set("X-User-ID", "user-status")
	signReq.SetPathValue("id", sigReq.ID)
	signW := httptest.NewRecorder()
	handler.Sign(signW, signReq)

	if signW.Code != http.StatusOK {
		t.Fatalf("sign failed: %d: %s", signW.Code, signW.Body.String())
	}

	// Now get status
	statusReq := httptest.NewRequest(http.MethodGet, "/", nil)
	statusReq.Header.Set("X-User-ID", "user-status")
	statusReq.SetPathValue("id", sigReq.ID)
	statusW := httptest.NewRecorder()
	handler.GetStatus(statusW, statusReq)

	if statusW.Code != http.StatusOK {
		t.Fatalf("get status: expected 200, got %d: %s", statusW.Code, statusW.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(statusW.Body).Decode(&resp)

	if resp["signed_document"] == nil {
		t.Error("expected signed_document in response for completed request")
	}

	// Verify the request portion shows COMPLETED
	reqData, ok := resp["request"].(map[string]any)
	if !ok {
		t.Fatal("expected request in response")
	}
	if reqData["status"] != "COMPLETED" {
		t.Errorf("expected COMPLETED status, got %v", reqData["status"])
	}
}

// TestUpload_NoDocumentField verifies that omitting the document field
// in the multipart form returns 400.
func TestUpload_NoDocumentField(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	// Create a field that is NOT named "document"
	part, _ := writer.CreateFormFile("wrong_field", "test.pdf")
	part.Write(createPDFContent())
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-User-ID", "user-1")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpload_InvalidPDFMagicBytes verifies that a .pdf file without
// valid PDF magic bytes is rejected.
func TestUpload_InvalidPDFMagicBytes(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	// File has .pdf extension but wrong magic bytes
	content := []byte("THIS IS NOT A PDF FILE AT ALL")
	req, _ := createMultipartRequest(t, "fake.pdf", content)
	req.Header.Set("X-User-ID", "user-1")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_file" {
		t.Errorf("expected invalid_file error, got %s", errResp["error"])
	}
}

// TestGetStatus_MissingUserID verifies GetStatus rejects requests
// without X-User-ID header.
func TestGetStatus_MissingUserID(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("id", "some-id")

	w := httptest.NewRecorder()
	handler.GetStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestGetStatus_NonExistent verifies GetStatus returns 404 for a
// non-existent request ID.
func TestGetStatus_NonExistent(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", "does-not-exist")

	w := httptest.NewRecorder()
	handler.GetStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestDownload_MissingUserID verifies Download rejects requests
// without X-User-ID header.
func TestDownload_MissingUserID(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.SetPathValue("id", "some-id")

	w := httptest.NewRecorder()
	handler.Download(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestUpload_PDFContentHashIsUnique verifies that different PDF contents
// produce different document hashes.
func TestUpload_PDFContentHashIsUnique(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	hashes := make(map[string]bool)

	contents := [][]byte{
		[]byte("%PDF-1.4 content version A"),
		[]byte("%PDF-1.4 content version B"),
		[]byte("%PDF-1.4 content version C"),
	}

	for i, content := range contents {
		req, _ := createMultipartRequest(t, fmt.Sprintf("doc_%d.pdf", i), content)
		req.Header.Set("X-User-ID", "user-hash")

		w := httptest.NewRecorder()
		handler.Upload(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("upload %d: expected 201, got %d", i, w.Code)
		}

		var resp signing.SigningRequest
		json.NewDecoder(w.Body).Decode(&resp)

		if hashes[resp.DocumentHash] {
			t.Errorf("upload %d: duplicate hash %s", i, resp.DocumentHash)
		}
		hashes[resp.DocumentHash] = true
	}

	if len(hashes) != 3 {
		t.Errorf("expected 3 unique hashes, got %d", len(hashes))
	}
}

// TestSign_SigningClientError verifies that a signing client failure
// results in a 500 response and the request status is set to FAILED.
func TestSign_SigningClientError(t *testing.T) {
	deps, reqStore, certStore := newDocumentDeps(t)

	deps.SignClient = &mockSigningClient{
		issueFn: defaultMockClient().issueFn,
		signFn: func(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
			return nil, fmt.Errorf("expired certificate: certificate validity period has ended")
		},
	}
	handler := NewDocumentHandler(deps)

	pdfContent := createPDFContent()
	path, _ := deps.FileStorage.Save("test.pdf", strings.NewReader(string(pdfContent)))

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:       "user-1",
		DocumentName: "test.pdf",
		DocumentSize: int64(len(pdfContent)),
		DocumentHash: "abc123",
		DocumentPath: path,
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	})

	certStore.Create(&signing.Certificate{
		UserID:         "user-1",
		Status:         "ACTIVE",
		SerialNumber:   "SN-EXPIRED",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\nexpired\n-----END CERTIFICATE-----",
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the request was marked as FAILED
	updated, err := reqStore.GetByID(sigReq.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != "FAILED" {
		t.Errorf("expected FAILED status, got %s", updated.Status)
	}
	if updated.ErrorMessage == "" {
		t.Error("expected error message to be recorded")
	}
}
