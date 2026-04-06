package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/store"
)

type mockSigningClient struct {
	issueFn func(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error)
	signFn  func(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error)
}

func (m *mockSigningClient) IssueCertificate(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error) {
	return m.issueFn(ctx, req)
}

func (m *mockSigningClient) SignDocument(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
	return m.signFn(ctx, req)
}

func defaultMockClient() *mockSigningClient {
	return &mockSigningClient{
		issueFn: func(ctx context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error) {
			return &signing.CertificateIssueResponse{
				SerialNumber:      "SN-TEST-001",
				CertificatePEM:    "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----",
				IssuerDN:          "CN=Test CA",
				SubjectDN:         "CN=" + req.SubjectCN,
				ValidFrom:         "2025-01-01T00:00:00Z",
				ValidTo:           "2026-01-01T00:00:00Z",
				FingerprintSHA256: "abcdef123456",
			}, nil
		},
		signFn: func(ctx context.Context, req signing.SignRequest) (*signing.SignResponse, error) {
			return &signing.SignResponse{
				SignedDocumentBase64: "c2lnbmVk",
				SignatureTimestamp:   "2025-01-01T00:00:00Z",
				PAdESLevel:          "PAdES-B-LTA",
			}, nil
		},
	}
}

func TestRequestCertificate_Success(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()
	handler := NewCertificateHandler(CertificateDeps{
		CertStore:    certStore,
		SignClient:   defaultMockClient(),
		AuditEmitter: audit.NewLogEmitter(),
		ValidityDays: 365,
	})

	body := `{"subject_cn":"Test User"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sign/certificates/request", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.RequestCertificate(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp signing.Certificate
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.SerialNumber != "SN-TEST-001" {
		t.Errorf("expected SN-TEST-001, got %s", resp.SerialNumber)
	}
}

func TestRequestCertificate_MissingUserID(t *testing.T) {
	handler := NewCertificateHandler(CertificateDeps{
		CertStore:    store.NewInMemoryCertificateStore(),
		SignClient:   defaultMockClient(),
		AuditEmitter: audit.NewLogEmitter(),
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	handler.RequestCertificate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestRequestCertificate_ActiveCertExists(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()
	certStore.Create(&signing.Certificate{
		UserID:       "user-1",
		Status:       "ACTIVE",
		SerialNumber: "SN-OLD",
	})

	handler := NewCertificateHandler(CertificateDeps{
		CertStore:    certStore,
		SignClient:   defaultMockClient(),
		AuditEmitter: audit.NewLogEmitter(),
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"subject_cn":"Test"}`))
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	handler.RequestCertificate(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestListCertificates_Success(t *testing.T) {
	certStore := store.NewInMemoryCertificateStore()
	certStore.Create(&signing.Certificate{
		UserID:       "user-1",
		Status:       "ACTIVE",
		SerialNumber: "SN1",
	})

	handler := NewCertificateHandler(CertificateDeps{
		CertStore:    certStore,
		SignClient:   defaultMockClient(),
		AuditEmitter: audit.NewLogEmitter(),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sign/certificates", nil)
	req.Header.Set("X-User-ID", "user-1")
	w := httptest.NewRecorder()
	handler.ListCertificates(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&resp)
	if _, ok := resp["certificates"]; !ok {
		t.Error("expected certificates field in response")
	}
}
