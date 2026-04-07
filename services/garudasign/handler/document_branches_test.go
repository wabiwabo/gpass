package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/storage"
	"github.com/garudapass/gpass/services/garudasign/store"
)

// TestSign_InvalidBase64InSignResponse pins the base64.DecodeString error
// branch in Sign when the signing backend returns malformed output.
func TestSign_InvalidBase64InSignResponse(t *testing.T) {
	dir := t.TempDir()
	reqStore := store.NewInMemoryRequestStore()
	certStore := store.NewInMemoryCertificateStore()
	docStore := store.NewInMemoryDocumentStore()
	fs := storage.NewFileStorage(dir)

	// Seed cert
	certStore.Create(&signing.Certificate{
		UserID: "user-1", SerialNumber: "SN", Status: "ACTIVE",
		ValidFrom: time.Now().Add(-time.Hour), ValidTo: time.Now().Add(time.Hour),
		CertificatePEM: "pem",
	})
	// Seed request with stored file
	path, _ := fs.Save("d.pdf", bytes.NewReader([]byte("%PDF-1.4 hello")))
	sr, _ := reqStore.Create(&signing.SigningRequest{
		UserID: "user-1", DocumentName: "d.pdf", DocumentPath: path,
		Status: "PENDING", ExpiresAt: time.Now().Add(time.Hour),
	})

	cli := &mockSigningClient{
		signFn: func(_ context.Context, _ signing.SignRequest) (*signing.SignResponse, error) {
			return &signing.SignResponse{
				SignedDocumentBase64: "!!!not base64!!!",
				SignatureTimestamp:   "2025-01-01T00:00:00Z",
				PAdESLevel:           "PAdES-B-LTA",
			}, nil
		},
	}
	h := NewDocumentHandler(DocumentDeps{
		CertStore: certStore, RequestStore: reqStore, DocStore: docStore,
		FileStorage: fs, SignClient: cli, AuditEmitter: audit.NewLogEmitter(),
		MaxSizeMB: 10, RequestTTL: 30 * time.Minute,
	})

	req := httptest.NewRequest("POST", "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sr.ID)
	rec := httptest.NewRecorder()
	h.Sign(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d body=%s", rec.Code, rec.Body)
	}
}

// TestUpload_ParseMultipartFormError pins the ParseMultipartForm error branch
// when the body isn't a valid multipart payload.
func TestUpload_ParseMultipartFormError(t *testing.T) {
	deps, _, _ := newDocumentDeps(t)
	h := NewDocumentHandler(deps)
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("not multipart"))
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	rec := httptest.NewRecorder()
	h.Upload(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}
