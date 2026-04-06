package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/store"
)

func setupRevocationTest(t *testing.T) (*RevocationHandler, *store.InMemoryCertificateStore, *signing.Certificate) {
	t.Helper()
	certStore := store.NewInMemoryCertificateStore()
	cert, err := certStore.Create(&signing.Certificate{
		UserID:       "user-1",
		Status:       "ACTIVE",
		SerialNumber: "SN-001",
	})
	if err != nil {
		t.Fatalf("failed to create test cert: %v", err)
	}
	h := NewRevocationHandler(certStore, audit.NewLogEmitter())
	return h, certStore, cert
}

func TestRevokeCertificate_Success(t *testing.T) {
	h, _, cert := setupRevocationTest(t)

	body := `{"reason":"key_compromise"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/"+cert.ID+"/revoke", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", cert.ID)
	w := httptest.NewRecorder()

	h.RevokeCertificate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "REVOKED" {
		t.Errorf("expected REVOKED, got %v", resp["status"])
	}
	if resp["revocation_reason"] != "key_compromise" {
		t.Errorf("expected key_compromise, got %v", resp["revocation_reason"])
	}
	if resp["revoked_at"] == nil {
		t.Error("expected revoked_at to be set")
	}
}

func TestRevokeCertificate_MissingUserID(t *testing.T) {
	h, _, cert := setupRevocationTest(t)

	body := `{"reason":"key_compromise"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/"+cert.ID+"/revoke", bytes.NewBufferString(body))
	req.SetPathValue("id", cert.ID)
	w := httptest.NewRecorder()

	h.RevokeCertificate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRevokeCertificate_InvalidReason(t *testing.T) {
	h, _, cert := setupRevocationTest(t)

	body := `{"reason":"bored_of_certificate"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/"+cert.ID+"/revoke", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", cert.ID)
	w := httptest.NewRecorder()

	h.RevokeCertificate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRevokeCertificate_NotFound(t *testing.T) {
	h, _, _ := setupRevocationTest(t)

	body := `{"reason":"key_compromise"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/nonexistent/revoke", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.RevokeCertificate(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRevokeCertificate_NotOwner(t *testing.T) {
	h, _, cert := setupRevocationTest(t)

	body := `{"reason":"key_compromise"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/"+cert.ID+"/revoke", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-2")
	req.SetPathValue("id", cert.ID)
	w := httptest.NewRecorder()

	h.RevokeCertificate(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRevokeCertificate_AlreadyRevoked(t *testing.T) {
	h, certStore, cert := setupRevocationTest(t)

	// Revoke it first
	now := cert.CreatedAt
	certStore.UpdateStatus(cert.ID, "REVOKED", &now, "superseded")

	body := `{"reason":"key_compromise"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/"+cert.ID+"/revoke", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("id", cert.ID)
	w := httptest.NewRecorder()

	h.RevokeCertificate(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}
