package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/storage"
	"github.com/garudapass/gpass/services/garudasign/store"
)

func createPDFContent() []byte {
	return []byte("%PDF-1.4 test content for signing")
}

func createMultipartRequest(t *testing.T, filename string, content []byte) (*http.Request, string) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("document", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	part.Write(content)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, writer.FormDataContentType()
}

func newDocumentDeps(t *testing.T) (DocumentDeps, *store.InMemoryRequestStore, *store.InMemoryCertificateStore) {
	t.Helper()
	dir := t.TempDir()
	certStore := store.NewInMemoryCertificateStore()
	reqStore := store.NewInMemoryRequestStore()
	docStore := store.NewInMemoryDocumentStore()
	fileStorage := storage.NewFileStorage(dir)

	deps := DocumentDeps{
		CertStore:    certStore,
		RequestStore: reqStore,
		DocStore:     docStore,
		FileStorage:  fileStorage,
		SignClient:   defaultMockClient(),
		AuditEmitter: audit.NewLogEmitter(),
		MaxSizeMB:    10,
		RequestTTL:   30 * time.Minute,
	}
	return deps, reqStore, certStore
}

func TestUpload_Success(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req, _ := createMultipartRequest(t, "test.pdf", createPDFContent())
	req.Header.Set("X-User-ID", "user-1")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp signing.SigningRequest
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.DocumentName != "test.pdf" {
		t.Errorf("expected test.pdf, got %s", resp.DocumentName)
	}
	if resp.Status != "PENDING" {
		t.Errorf("expected PENDING, got %s", resp.Status)
	}
}

func TestUpload_MissingUserID(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req, _ := createMultipartRequest(t, "test.pdf", createPDFContent())
	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpload_NotPDF(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	req, _ := createMultipartRequest(t, "test.txt", []byte("not a pdf"))
	req.Header.Set("X-User-ID", "user-1")

	w := httptest.NewRecorder()
	handler.Upload(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestSign_Success(t *testing.T) {
	deps, reqStore, certStore := newDocumentDeps(t)

	// Create a mock that returns base64 encoded signed content
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

	// Save a PDF file first
	pdfContent := createPDFContent()
	path, _ := deps.FileStorage.Save("test.pdf", bytes.NewReader(pdfContent))

	// Create a signing request
	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:       "user-1",
		DocumentName: "test.pdf",
		DocumentSize: int64(len(pdfContent)),
		DocumentHash: "abc123",
		DocumentPath: path,
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	})

	// Create an active certificate
	certStore.Create(&signing.Certificate{
		UserID:         "user-1",
		Status:         "ACTIVE",
		SerialNumber:   "SN-TEST",
		CertificatePEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/documents/"+sigReq.ID+"/sign", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSign_NotOwner(t *testing.T) {
	deps, reqStore, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:    "user-1",
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-other")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestSign_Expired(t *testing.T) {
	deps, reqStore, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:    "user-1",
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.Sign(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("expected 410, got %d", w.Code)
	}
}

func TestGetStatus_Success(t *testing.T) {
	deps, reqStore, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:       "user-1",
		DocumentName: "test.pdf",
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.GetStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetStatus_NotOwner(t *testing.T) {
	deps, reqStore, _ := newDocumentDeps(t)
	handler := NewDocumentHandler(deps)

	sigReq, _ := reqStore.Create(&signing.SigningRequest{
		UserID:    "user-1",
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-User-ID", "user-other")
	req.SetPathValue("id", sigReq.ID)

	w := httptest.NewRecorder()
	handler.GetStatus(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}
