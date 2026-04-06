package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/signing-sim/ca"
)

func setupSignTest(t *testing.T) (*ca.CA, *ca.IssuedCertificate) {
	t.Helper()
	c, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}
	cert, err := c.IssueCertificate("Test Signer", "signer-1", 365)
	if err != nil {
		t.Fatalf("IssueCertificate() error: %v", err)
	}
	return c, cert
}

func TestSignHandler_Sign_Success(t *testing.T) {
	c, cert := setupSignTest(t)
	h := NewSignHandler(c)

	doc := []byte("%PDF-1.4 test document")
	docB64 := base64.StdEncoding.EncodeToString(doc)

	body := map[string]string{
		"document_base64": docB64,
		"certificate_pem": cert.CertificatePEM,
		"signature_level": "B_LTA",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp signResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.SignedDocumentBase64 == "" {
		t.Error("signed_document_base64 is empty")
	}

	if resp.SignedDocumentBase64 == docB64 {
		t.Error("signed document should differ from original")
	}

	if resp.PAdESLevel != "B_LTA" {
		t.Errorf("pades_level = %q, want %q", resp.PAdESLevel, "B_LTA")
	}

	if resp.SignatureTimestamp == "" {
		t.Error("signature_timestamp is empty")
	}
}

func TestSignHandler_Sign_MissingDocument(t *testing.T) {
	c, cert := setupSignTest(t)
	h := NewSignHandler(c)

	body := map[string]string{
		"certificate_pem": cert.CertificatePEM,
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSignHandler_Sign_UnknownCertificate(t *testing.T) {
	c, _ := setupSignTest(t)
	h := NewSignHandler(c)

	doc := []byte("%PDF-1.4 test document")
	docB64 := base64.StdEncoding.EncodeToString(doc)

	body := map[string]string{
		"document_base64": docB64,
		"certificate_pem": "-----BEGIN CERTIFICATE-----\nunknown\n-----END CERTIFICATE-----\n",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["error"] != "unknown_certificate" {
		t.Errorf("error = %q, want %q", resp["error"], "unknown_certificate")
	}
}

func TestSignHandler_Sign_InvalidJSON(t *testing.T) {
	c, _ := setupSignTest(t)
	h := NewSignHandler(c)

	req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSignHandler_Sign_MissingCertificate(t *testing.T) {
	c, _ := setupSignTest(t)
	h := NewSignHandler(c)

	doc := []byte("%PDF-1.4 test document")
	docB64 := base64.StdEncoding.EncodeToString(doc)

	body := map[string]string{
		"document_base64": docB64,
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
