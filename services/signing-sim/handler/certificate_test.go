package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/signing-sim/ca"
)

func TestCertificateHandler_Issue_Success(t *testing.T) {
	c, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	h := NewCertificateHandler(c)

	body := `{"subject_cn":"Test User","subject_uid":"user-123","validity_days":365}`
	req := httptest.NewRequest(http.MethodPost, "/certificates/issue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp issueResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.SerialNumber == "" {
		t.Error("serial_number is empty")
	}

	if resp.CertificatePEM == "" {
		t.Error("certificate_pem is empty")
	}

	if resp.IssuerDN == "" {
		t.Error("issuer_dn is empty")
	}

	if resp.SubjectDN == "" {
		t.Error("subject_dn is empty")
	}

	if resp.FingerprintSHA256 == "" {
		t.Error("fingerprint_sha256 is empty")
	}

	if resp.ValidFrom == "" {
		t.Error("valid_from is empty")
	}

	if resp.ValidTo == "" {
		t.Error("valid_to is empty")
	}
}

func TestCertificateHandler_Issue_MissingCN(t *testing.T) {
	c, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	h := NewCertificateHandler(c)

	body := `{"subject_uid":"user-123","validity_days":365}`
	req := httptest.NewRequest(http.MethodPost, "/certificates/issue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["error"] != "invalid_request" {
		t.Errorf("error = %q, want %q", resp["error"], "invalid_request")
	}
}

func TestCertificateHandler_Issue_InvalidJSON(t *testing.T) {
	c, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	h := NewCertificateHandler(c)

	req := httptest.NewRequest(http.MethodPost, "/certificates/issue", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCertificateHandler_Issue_DefaultValidityDays(t *testing.T) {
	c, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	h := NewCertificateHandler(c)

	body := `{"subject_cn":"Test User"}`
	req := httptest.NewRequest(http.MethodPost, "/certificates/issue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}
