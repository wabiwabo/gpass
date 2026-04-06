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

func TestSignHandler_Sign_EmptyPayload(t *testing.T) {
	c, cert := setupSignTest(t)
	h := NewSignHandler(c)

	// Empty base64 decodes to empty bytes
	body := map[string]string{
		"document_base64": "",
		"certificate_pem": cert.CertificatePEM,
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for empty payload", w.Code, http.StatusBadRequest)
	}
}

func TestSignHandler_Sign_OversizedPayload(t *testing.T) {
	c, cert := setupSignTest(t)
	h := NewSignHandler(c)

	// Create a large document (1MB+)
	largeDoc := make([]byte, 1024*1024+1)
	for i := range largeDoc {
		largeDoc[i] = 'A'
	}
	docB64 := base64.StdEncoding.EncodeToString(largeDoc)

	body := map[string]string{
		"document_base64": docB64,
		"certificate_pem": cert.CertificatePEM,
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	// Large documents should still sign successfully
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d for oversized payload; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp signResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.SignedDocumentBase64 == "" {
		t.Error("signed_document_base64 should not be empty for large document")
	}
}

func TestSignHandler_Sign_InvalidBase64Document(t *testing.T) {
	c, cert := setupSignTest(t)
	h := NewSignHandler(c)

	body := map[string]string{
		"document_base64": "this-is-not-valid-base64!!!",
		"certificate_pem": cert.CertificatePEM,
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for invalid base64", w.Code, http.StatusBadRequest)
	}
}

func TestCertificateHandler_Issue_MissingCommonName(t *testing.T) {
	c, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	h := NewCertificateHandler(c)

	// Empty subject_cn
	body := `{"subject_cn":"","subject_uid":"user-456","validity_days":365}`
	req := httptest.NewRequest(http.MethodPost, "/certificates/issue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d for empty CN", w.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["error"] != "invalid_request" {
		t.Errorf("error = %q, want %q", resp["error"], "invalid_request")
	}
}

func TestSignHandler_Sign_MultipleSequentialOperations(t *testing.T) {
	c, cert := setupSignTest(t)
	h := NewSignHandler(c)

	docs := []string{
		"%PDF-1.4 document one",
		"%PDF-1.4 document two",
		"%PDF-1.4 document three",
	}

	signatures := make(map[string]bool)
	for i, doc := range docs {
		docB64 := base64.StdEncoding.EncodeToString([]byte(doc))
		body := map[string]string{
			"document_base64": docB64,
			"certificate_pem": cert.CertificatePEM,
		}
		bodyJSON, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader(string(bodyJSON)))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.Sign(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("document %d: status = %d, want %d", i, w.Code, http.StatusOK)
		}

		var resp signResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("document %d: failed to decode response: %v", i, err)
		}

		if resp.SignedDocumentBase64 == "" {
			t.Errorf("document %d: signed_document_base64 is empty", i)
		}

		// Each signed document should be unique
		if signatures[resp.SignedDocumentBase64] {
			t.Errorf("document %d: signed document is not unique", i)
		}
		signatures[resp.SignedDocumentBase64] = true
	}
}

func TestSignHandler_Sign_ContentTypeValidation(t *testing.T) {
	c, cert := setupSignTest(t)
	h := NewSignHandler(c)

	doc := []byte("%PDF-1.4 test document")
	docB64 := base64.StdEncoding.EncodeToString(doc)

	body := map[string]string{
		"document_base64": docB64,
		"certificate_pem": cert.CertificatePEM,
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/sign/pades", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Sign(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}
}

func TestCertificateHandler_Issue_ContentTypeValidation(t *testing.T) {
	c, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	h := NewCertificateHandler(c)

	body := `{"subject_cn":"Test User","subject_uid":"user-789","validity_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/certificates/issue", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Issue(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}
}

func TestCertificateHandler_Issue_MultipleCertificates_UniqueSerials(t *testing.T) {
	c, err := ca.NewCA()
	if err != nil {
		t.Fatalf("NewCA() error: %v", err)
	}

	h := NewCertificateHandler(c)

	serials := make(map[string]bool)
	for i := 0; i < 5; i++ {
		body := `{"subject_cn":"User ` + string(rune('A'+i)) + `","subject_uid":"uid-` + string(rune('0'+i)) + `","validity_days":365}`
		req := httptest.NewRequest(http.MethodPost, "/certificates/issue", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		h.Issue(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("certificate %d: status = %d, want %d; body: %s", i, w.Code, http.StatusOK, w.Body.String())
		}

		var resp issueResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("certificate %d: failed to decode response: %v", i, err)
		}

		if serials[resp.SerialNumber] {
			t.Errorf("certificate %d: serial number %s is not unique", i, resp.SerialNumber)
		}
		serials[resp.SerialNumber] = true
	}
}
