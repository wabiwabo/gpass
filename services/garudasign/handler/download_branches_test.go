package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/storage"
	"github.com/garudapass/gpass/services/garudasign/store"
)

func newDownloadHandler(t *testing.T) (*DocumentHandler, *store.InMemoryRequestStore, *store.InMemoryDocumentStore, *storage.FileStorage) {
	t.Helper()
	dir := t.TempDir()
	reqStore := store.NewInMemoryRequestStore()
	docStore := store.NewInMemoryDocumentStore()
	certStore := store.NewInMemoryCertificateStore()
	fs := storage.NewFileStorage(dir)
	h := NewDocumentHandler(DocumentDeps{
		CertStore: certStore, RequestStore: reqStore, DocStore: docStore,
		FileStorage: fs, SignClient: defaultMockClient(), AuditEmitter: audit.NewLogEmitter(),
		MaxSizeMB: 10, RequestTTL: 30 * time.Minute,
	})
	return h, reqStore, docStore, fs
}

// TestDownload_EmptyRequestID pins the empty PathValue guard.
func TestDownload_EmptyRequestID(t *testing.T) {
	h, _, _, _ := newDownloadHandler(t)
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", "")
	rec := httptest.NewRecorder()
	h.Download(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestDownload_SignedDocMissing pins the GetByRequestID 404 branch — the
// signing request is COMPLETED but no SignedDocument row exists.
func TestDownload_SignedDocMissing(t *testing.T) {
	h, reqStore, _, _ := newDownloadHandler(t)
	sr, _ := reqStore.Create(&signing.SigningRequest{
		UserID: "user-1", DocumentName: "d.pdf", DocumentPath: "/tmp/x",
		Status: "COMPLETED", ExpiresAt: time.Now().Add(time.Hour),
	})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sr.ID)
	rec := httptest.NewRecorder()
	h.Download(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d body=%s", rec.Code, rec.Body)
	}
}

// TestDownload_FileStorageLoadFails pins the FileStorage.Load error branch.
func TestDownload_FileStorageLoadFails(t *testing.T) {
	h, reqStore, docStore, _ := newDownloadHandler(t)
	sr, _ := reqStore.Create(&signing.SigningRequest{
		UserID: "user-1", DocumentName: "d.pdf", DocumentPath: "/tmp/x",
		Status: "COMPLETED", ExpiresAt: time.Now().Add(time.Hour),
	})
	// SignedPath points to a file that doesn't exist in FileStorage.
	docStore.Create(&signing.SignedDocument{
		RequestID: sr.ID, CertificateID: "c1",
		SignedHash: "abc", SignedPath: "does-not-exist.pdf",
		SignedSize: 10, PAdESLevel: "PAdES-B-LTA",
		SignatureTimestamp: time.Now(),
	})

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sr.ID)
	rec := httptest.NewRecorder()
	h.Download(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d body=%s", rec.Code, rec.Body)
	}
}

// TestDownload_NotCompleted pins the "status != COMPLETED" branch.
func TestDownload_NotCompleted(t *testing.T) {
	h, reqStore, _, _ := newDownloadHandler(t)
	sr, _ := reqStore.Create(&signing.SigningRequest{
		UserID: "user-1", DocumentName: "d.pdf", DocumentPath: "/tmp/x",
		Status: "PENDING", ExpiresAt: time.Now().Add(time.Hour),
	})
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", sr.ID)
	rec := httptest.NewRecorder()
	h.Download(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}
