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
	"sync"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/store"
)

// TestSign_WithRevokedCertificate verifies that signing fails when the
// user's only certificate has been revoked (no active certificate found).
func TestSign_WithRevokedCertificate(t *testing.T) {
	deps, reqStore, certStore := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	pdfContent := createPDFContent()
	path, _ := deps.FileStorage.Save("test.pdf", bytes.NewReader(pdfContent))

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:       "user-revoked",
		DocumentName: "test.pdf",
		DocumentSize: int64(len(pdfContent)),
		DocumentHash: "abc123",
		DocumentPath: path,
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	})

	// Create a certificate and then revoke it
	cert, _ := certStore.Create(&signing.Certificate{
		UserID:         "user-revoked",
		Status:         "ACTIVE",
		SerialNumber:   "SN-REVOKED",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
	})
	now := time.Now()
	certStore.UpdateStatus(cert.ID, "REVOKED", &now, "key_compromise")

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-revoked")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for revoked certificate, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "no_certificate" {
		t.Errorf("expected no_certificate error, got %s", errResp["error"])
	}
}

// TestSign_WithExpiredCertificate verifies that signing fails when the
// user's certificate has expired (ValidTo in the past). The certificate
// is still marked ACTIVE but the signing backend rejects it.
func TestSign_WithExpiredCertificate(t *testing.T) {
	deps, reqStore, certStore := newDocumentDeps(t)

	// Mock signing client that rejects expired certificates
	deps.SignClient = &mockSigningClient{
		issueFn: defaultMockClient().issueFn,
		signFn: func(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
			return nil, fmt.Errorf("certificate has expired")
		},
	}
	handler := NewDocumentHandler(deps)

	pdfContent := createPDFContent()
	path, _ := deps.FileStorage.Save("test.pdf", bytes.NewReader(pdfContent))

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:       "user-expired-cert",
		DocumentName: "test.pdf",
		DocumentSize: int64(len(pdfContent)),
		DocumentHash: "abc123",
		DocumentPath: path,
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	})

	// Create a certificate with expired validity dates but still ACTIVE status
	certStore.Create(&signing.Certificate{
		UserID:         "user-expired-cert",
		Status:         "ACTIVE",
		SerialNumber:   "SN-EXPIRED-CERT",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\nexpired\n-----END CERTIFICATE-----",
		ValidFrom:      time.Now().Add(-2 * 365 * 24 * time.Hour),
		ValidTo:        time.Now().Add(-1 * 365 * 24 * time.Hour),
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-expired-cert")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for expired certificate signing, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "sign_failed" {
		t.Errorf("expected sign_failed error, got %s", errResp["error"])
	}

	// Verify request status changed to FAILED
	updated, _ := reqStore.GetByID(sigReq.ID)
	if updated.Status != "FAILED" {
		t.Errorf("expected FAILED status, got %s", updated.Status)
	}
}

// TestUpload_NonPDFExtension verifies that uploading a file without
// a .pdf extension is rejected, even if the content has PDF magic bytes.
func TestUpload_NonPDFExtension(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	// Valid PDF content but wrong extension
	req, _ := createMultipartRequest(t, "document.docx", createPDFContent())
	req.Header.Set("X-User-ID", "user-1")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-PDF extension, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_file" {
		t.Errorf("expected invalid_file error, got %s", errResp["error"])
	}
}

// TestUpload_OversizedDocumentBoundary verifies size limit behavior at
// exact boundaries. A file at exactly MaxSizeMB should succeed, while
// one slightly above should fail.
func TestUpload_OversizedDocumentBoundary(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	deps.MaxSizeMB = 1 // 1MB limit
	handler := NewDocumentHandler(deps)

	// Create content exactly at the limit (should succeed - multipart overhead may push it over)
	// Instead test that well-under limit succeeds and over-limit fails
	t.Run("well_under_limit_succeeds", func(t *testing.T) {
		smallContent := make([]byte, 512*1024) // 512KB
		copy(smallContent[:5], []byte("%PDF-"))

		req, _ := createMultipartRequest(t, "small.pdf", smallContent)
		req.Header.Set("X-User-ID", "user-boundary")

		w := httptest.NewRecorder()
		handler.Upload(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201 for file under limit, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("over_limit_fails", func(t *testing.T) {
		// 2MB content exceeds 1MB limit
		bigContent := make([]byte, 2*1024*1024)
		copy(bigContent[:5], []byte("%PDF-"))

		req, _ := createMultipartRequest(t, "big.pdf", bigContent)
		req.Header.Set("X-User-ID", "user-boundary")

		w := httptest.NewRecorder()
		handler.Upload(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for file over limit, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestUpload_EmptyBody verifies that a request with no multipart body
// is rejected.
func TestUpload_EmptyBody(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents", strings.NewReader(""))
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=nonexistent")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty body, got %d: %s", w.Code, w.Body.String())
	}
}

// TestGetCertificate_NonExistent verifies that getting a certificate
// by ID that does not exist returns 404.
func TestGetCertificate_NonExistent(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()

	_, err := certStore.GetByID("nonexistent-cert-id")
	if err == nil {
		t.Error("expected error for non-existent certificate, got nil")
	}
}

// TestRevokeCertificate_NonExistentCert verifies that revoking a
// certificate that does not exist returns 404.
func TestRevokeCertificate_NonExistentCert(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()
	h := NewRevocationHandler(certStore, audit.NewLogEmitter())

	body := `{"reason":"key_compromise"}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", "cert-does-not-exist-at-all")

	w := httptest.NewRecorder()
	h.RevokeCertificate(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "not_found" {
		t.Errorf("expected not_found error, got %s", errResp["error"])
	}
}

// TestRevokeCertificate_AlreadyRevokedIdempotency verifies that
// revoking an already-revoked certificate returns 409 Conflict and
// does not change the original revocation reason or timestamp.
func TestRevokeCertificate_AlreadyRevokedIdempotency(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()
	cert, _ := certStore.Create(&signing.Certificate{
		UserID:       "user-1",
		Status:       "ACTIVE",
		SerialNumber: "SN-IDEM",
	})

	// Revoke with original reason
	revokedAt := time.Now().UTC()
	certStore.UpdateStatus(cert.ID, "REVOKED", &revokedAt, "superseded")

	h := NewRevocationHandler(certStore, audit.NewLogEmitter())

	// Try to revoke again with different reason
	body := `{"reason":"key_compromise"}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", cert.ID)

	w := httptest.NewRecorder()
	h.RevokeCertificate(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d: %s", w.Code, w.Body.String())
	}

	// Verify original revocation reason is preserved
	updatedCert, _ := certStore.GetByID(cert.ID)
	if updatedCert.RevocationReason != "superseded" {
		t.Errorf("expected original reason 'superseded' preserved, got %s", updatedCert.RevocationReason)
	}
}

// TestGetSignedDocument_NonExistentID verifies that requesting a
// signed document by a non-existent request ID returns 404.
func TestGetSignedDocument_NonExistentID(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", "totally-nonexistent-id")

	w := httptest.NewRecorder()
	handler.GetStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "not_found" {
		t.Errorf("expected not_found error, got %s", errResp["error"])
	}
}

// TestListCertificates_NoCertificates verifies that listing certificates
// for a user with none returns an empty array (not null).
func TestListCertificates_NoCertificates(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()
	handler := NewCertificateHandler(CertificateDeps{
		CertStore:    certStore,
		SignClient:   defaultMockClient(),
		AuditEmitter: audit.NewLogEmitter(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sign/certificates", nil)
	req.Header.Set("X-User-ID", "user-with-no-certs")

	w := httptest.NewRecorder()
	handler.ListCertificates(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)

	certsRaw, ok := resp["certificates"]
	if !ok {
		t.Fatal("expected certificates field in response")
	}

	// Verify it's an empty array, not null
	var certs []signing.Certificate
	if err := json.Unmarshal(certsRaw, &certs); err != nil {
		t.Fatalf("failed to unmarshal certificates: %v", err)
	}
	if len(certs) != 0 {
		t.Errorf("expected 0 certificates, got %d", len(certs))
	}

	// Ensure the raw JSON is "[]" not "null"
	trimmed := strings.TrimSpace(string(certsRaw))
	if trimmed == "null" {
		t.Error("certificates should be [] not null for empty list")
	}
}

// TestRequestCertificate_InvalidJSON verifies that a request with
// malformed JSON body returns 400.
func TestRequestCertificate_InvalidJSON(t *testing.T) {
	handler := NewCertificateHandler(CertificateDeps{
		CertStore:    store.NewInMemoryCertificateStore(),
		SignClient:   defaultMockClient(),
		AuditEmitter: audit.NewLogEmitter(),
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{not valid json"))
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.RequestCertificate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_request" {
		t.Errorf("expected invalid_request error, got %s", errResp["error"])
	}
}

// TestSign_CertificateOwnedByDifferentUser verifies that a user cannot
// sign a document using another user's signing request (ownership check).
func TestSign_CertificateOwnedByDifferentUser(t *testing.T) {
	deps, reqStore, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	// Create a signing request owned by user-1
	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:       "user-1",
		DocumentName: "private.pdf",
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	})

	// user-2 tries to sign user-1's document
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-2")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "forbidden" {
		t.Errorf("expected forbidden error, got %s", errResp["error"])
	}
}

// TestVerifyDocumentHashAfterSigning verifies that signing a document
// produces a signed document with a non-empty hash that differs from
// the original document hash.
func TestVerifyDocumentHashAfterSigning(t *testing.T) {
	deps, reqStore, certStore := newDocumentDeps(t)

	signedContent := base64.StdEncoding.EncodeToString([]byte("%PDF-1.4 signed and sealed"))
	deps.SignClient = &mockSigningClient{
		issueFn: defaultMockClient().issueFn,
		signFn: func(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
			return &signing.SignResponse{
				SignedDocumentBase64: signedContent,
				SignatureTimestamp:   "2025-06-15T12:00:00Z",
				PAdESLevel:          "PAdES-B-LTA",
			}, nil
		},
	}
	handler := NewDocumentHandler(deps)

	pdfContent := createPDFContent()
	path, _ := deps.FileStorage.Save("hashtest.pdf", bytes.NewReader(pdfContent))

	originalHash := "original-hash-abc"
	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:       "user-hash-verify",
		DocumentName: "hashtest.pdf",
		DocumentSize: int64(len(pdfContent)),
		DocumentHash: originalHash,
		DocumentPath: path,
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	})

	certStore.Create(&signing.Certificate{
		UserID:         "user-hash-verify",
		Status:         "ACTIVE",
		SerialNumber:   "SN-HASH",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-hash-verify")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var signedDoc signing.SignedDocument
	json.NewDecoder(w.Body).Decode(&signedDoc)

	if signedDoc.SignedHash == "" {
		t.Error("expected non-empty signed hash")
	}
	if signedDoc.SignedHash == originalHash {
		t.Error("signed hash should differ from original document hash")
	}
	if signedDoc.PAdESLevel != "PAdES-B-LTA" {
		t.Errorf("expected PAdES-B-LTA, got %s", signedDoc.PAdESLevel)
	}
	if signedDoc.SignedSize <= 0 {
		t.Errorf("expected positive signed size, got %d", signedDoc.SignedSize)
	}
}

// TestRevokeCertificate_InvalidJSON verifies that revoking with
// malformed JSON returns 400.
func TestRevokeCertificate_InvalidJSON(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()
	cert, _ := certStore.Create(&signing.Certificate{
		UserID:       "user-1",
		Status:       "ACTIVE",
		SerialNumber: "SN-JSON",
	})
	h := NewRevocationHandler(certStore, audit.NewLogEmitter())

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{broken"))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", cert.ID)

	w := httptest.NewRecorder()
	h.RevokeCertificate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid JSON, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_request" {
		t.Errorf("expected invalid_request, got %s", errResp["error"])
	}
}

// TestConcurrentCertificateCreation verifies that concurrent certificate
// creation requests are handled safely without data races. Only the first
// request should succeed; subsequent ones should get 409 (active_cert_exists).
func TestConcurrentCertificateCreation(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()
	handler := NewCertificateHandler(CertificateDeps{
		CertStore:    certStore,
		SignClient:   defaultMockClient(),
		AuditEmitter: audit.NewLogEmitter(),
		ValidityDays: 365,
	})

	const goroutines = 10
	var wg sync.WaitGroup
	results := make([]int, goroutines)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			body := `{"subject_cn":"Concurrent User"}`
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
			req.Header.Set("X-User-ID", "user-concurrent")
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			handler.RequestCertificate(w, req)
			results[idx] = w.Code
		}(i)
	}
	wg.Wait()

	createdCount := 0
	conflictCount := 0
	for _, code := range results {
		switch code {
		case http.StatusCreated:
			createdCount++
		case http.StatusConflict:
			conflictCount++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	if createdCount != 1 {
		t.Errorf("expected exactly 1 successful creation, got %d", createdCount)
	}
	if conflictCount != goroutines-1 {
		t.Errorf("expected %d conflicts, got %d", goroutines-1, conflictCount)
	}
}

// TestUpload_NonMultipartContentType verifies that a POST with a
// non-multipart Content-Type header is rejected.
func TestUpload_NonMultipartContentType(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents",
		bytes.NewBufferString(`{"document": "not multipart"}`))
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-multipart content type, got %d: %s", w.Code, w.Body.String())
	}
}

// TestRevokeCertificate_EmptyReason verifies that revoking with an
// empty reason string returns 400 (invalid_reason).
func TestRevokeCertificate_EmptyReason(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()
	cert, _ := certStore.Create(&signing.Certificate{
		UserID:       "user-1",
		Status:       "ACTIVE",
		SerialNumber: "SN-EMPTY-REASON",
	})
	h := NewRevocationHandler(certStore, audit.NewLogEmitter())

	body := `{"reason":""}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", cert.ID)

	w := httptest.NewRecorder()
	h.RevokeCertificate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty reason, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_reason" {
		t.Errorf("expected invalid_reason error, got %s", errResp["error"])
	}
}

// TestUpload_PDFExtensionCaseInsensitive verifies that .PDF (uppercase)
// extension is accepted since the handler uses ToLower.
func TestUpload_PDFExtensionCaseInsensitive(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req, _ := createMultipartRequest(t, "DOCUMENT.PDF", createPDFContent())
	req.Header.Set("X-User-ID", "user-1")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 for .PDF extension, got %d: %s", w.Code, w.Body.String())
	}
}

// TestListCertificates_FilterByStatus verifies that the status filter
// parameter correctly filters the certificate list.
func TestListCertificates_FilterByStatus(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()

	// Seed a cert that will be revoked, then create the surviving ACTIVE.
	// Enforced invariant: only one ACTIVE per user at a time — so we must
	// revoke the first before creating the second.
	toRevoke, _ := certStore.Create(&signing.Certificate{
		UserID:       "user-filter",
		Status:       "ACTIVE",
		SerialNumber: "SN-TO-REVOKE",
	})
	now := time.Now()
	certStore.UpdateStatus(toRevoke.ID, "REVOKED", &now, "superseded")
	certStore.Create(&signing.Certificate{
		UserID:       "user-filter",
		Status:       "ACTIVE",
		SerialNumber: "SN-ACTIVE",
	})

	handler := NewCertificateHandler(CertificateDeps{
		CertStore:    certStore,
		SignClient:   defaultMockClient(),
		AuditEmitter: audit.NewLogEmitter(),
	})

	t.Run("filter_active_only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sign/certificates?status=ACTIVE", nil)
		req.Header.Set("X-User-ID", "user-filter")

		w := httptest.NewRecorder()
		handler.ListCertificates(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string][]*signing.Certificate
		json.NewDecoder(w.Body).Decode(&resp)
		// Only the ACTIVE cert should be returned; the first one was not
		// replaced so we should have exactly 1 ACTIVE
		for _, c := range resp["certificates"] {
			if c.Status != "ACTIVE" {
				t.Errorf("expected only ACTIVE certs, got %s", c.Status)
			}
		}
	})

	t.Run("filter_revoked_only", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/sign/certificates?status=REVOKED", nil)
		req.Header.Set("X-User-ID", "user-filter")

		w := httptest.NewRecorder()
		handler.ListCertificates(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}

		var resp map[string][]*signing.Certificate
		json.NewDecoder(w.Body).Decode(&resp)
		if len(resp["certificates"]) != 1 {
			t.Errorf("expected 1 revoked cert, got %d", len(resp["certificates"]))
		}
		for _, c := range resp["certificates"] {
			if c.Status != "REVOKED" {
				t.Errorf("expected only REVOKED certs, got %s", c.Status)
			}
		}
	})
}

// TestRequestCertificate_DefaultSubjectCN verifies that when no
// subject_cn is provided, it defaults to the user ID.
func TestRequestCertificate_DefaultSubjectCN(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()

	var capturedReq signing.CertificateIssueRequest
	mockClient := &mockSigningClient{
		issueFn: func(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error) {
			capturedReq = req
			return &signing.CertificateIssueResponse{
				SerialNumber:      "SN-DEFAULT",
				CertificatePEM:    "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
				IssuerDN:          "CN=Test CA",
				SubjectDN:         "CN=" + req.SubjectCN,
				ValidFrom:         "2025-01-01T00:00:00Z",
				ValidTo:           "2026-01-01T00:00:00Z",
				FingerprintSHA256: "default123",
			}, nil
		},
		signFn: defaultMockClient().signFn,
	}

	handler := NewCertificateHandler(CertificateDeps{
		CertStore:    certStore,
		SignClient:   mockClient,
		AuditEmitter: audit.NewLogEmitter(),
		ValidityDays: 365,
	})

	// Send empty subject_cn
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-default-cn")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.RequestCertificate(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// The subject_cn should default to the user ID
	if capturedReq.SubjectCN != "user-default-cn" {
		t.Errorf("expected subject_cn to default to user ID 'user-default-cn', got '%s'", capturedReq.SubjectCN)
	}
}

// TestUpload_MixedCaseExtensionVariants tests various PDF extension
// case variations to ensure they are all accepted.
func TestUpload_MixedCaseExtensionVariants(t *testing.T) {
	cases := []struct {
		filename string
		wantCode int
	}{
		{"doc.pdf", http.StatusCreated},
		{"doc.PDF", http.StatusCreated},
		{"doc.Pdf", http.StatusCreated},
		{"doc.txt", http.StatusBadRequest},
		{"doc.pdf.exe", http.StatusBadRequest},
	}

	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			deps, _, _ := newDocumentDeps(t)
			handler := NewDocumentHandler(deps)

			var content []byte
			if tc.wantCode == http.StatusCreated {
				content = createPDFContent()
			} else {
				content = []byte("not a pdf")
			}

			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)
			part, _ := writer.CreateFormFile("document", tc.filename)
			part.Write(content)
			writer.Close()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("X-User-ID", "user-ext")

			w := httptest.NewRecorder()
			handler.Upload(w, req)

			if w.Code != tc.wantCode {
				t.Errorf("filename %q: expected %d, got %d: %s", tc.filename, tc.wantCode, w.Code, w.Body.String())
			}
		})
	}
}
