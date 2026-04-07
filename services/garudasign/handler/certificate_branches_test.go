package handler

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudasign/audit"
	"github.com/garudapass/gpass/services/garudasign/signing"
	"github.com/garudapass/gpass/services/garudasign/store"
)

func newCertHandler(cli SigningClient) (*CertificateHandler, store.CertificateStore) {
	cs := store.NewInMemoryCertificateStore()
	return NewCertificateHandler(CertificateDeps{
		CertStore: cs, SignClient: cli, AuditEmitter: audit.NewLogEmitter(), ValidityDays: 365,
	}), cs
}

func TestRequestCertificate_BadJSON(t *testing.T) {
	h, _ := newCertHandler(defaultMockClient())
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString("{not json"))
	req.Header.Set("X-User-ID", "user-1")
	rec := httptest.NewRecorder()
	h.RequestCertificate(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

func TestRequestCertificate_IssueFails(t *testing.T) {
	cli := &mockSigningClient{
		issueFn: func(_ context.Context, _ signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error) {
			return nil, errors.New("backend down")
		},
	}
	h, _ := newCertHandler(cli)
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{}`))
	req.Header.Set("X-User-ID", "user-1")
	rec := httptest.NewRecorder()
	h.RequestCertificate(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("code = %d", rec.Code)
	}
}

func TestRequestCertificate_EmptySubjectDefaultsToUserID(t *testing.T) {
	var seenCN string
	cli := &mockSigningClient{
		issueFn: func(_ context.Context, req signing.CertificateIssueRequest) (*signing.CertificateIssueResponse, error) {
			seenCN = req.SubjectCN
			return &signing.CertificateIssueResponse{
				SerialNumber: "SN-X", SubjectDN: "CN=" + req.SubjectCN, IssuerDN: "CN=CA",
				CertificatePEM: "pem", ValidFrom: "2025-01-01T00:00:00Z", ValidTo: "2026-01-01T00:00:00Z",
			}, nil
		},
	}
	h, _ := newCertHandler(cli)
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(`{}`))
	req.Header.Set("X-User-ID", "user-42")
	rec := httptest.NewRecorder()
	h.RequestCertificate(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("code = %d", rec.Code)
	}
	if seenCN != "user-42" {
		t.Errorf("SubjectCN defaulted = %q", seenCN)
	}
}

func TestListCertificates_MissingUserID(t *testing.T) {
	h, _ := newCertHandler(defaultMockClient())
	rec := httptest.NewRecorder()
	h.ListCertificates(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

func TestListCertificates_WithStatusFilter(t *testing.T) {
	h, _ := newCertHandler(defaultMockClient())
	req := httptest.NewRequest("GET", "/?status=ACTIVE", nil)
	req.Header.Set("X-User-ID", "user-1")
	rec := httptest.NewRecorder()
	h.ListCertificates(rec, req)
	if rec.Code != 200 {
		t.Errorf("code = %d", rec.Code)
	}
	// Empty list contract: JSON "[]" not "null"
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"certificates":[]`)) {
		t.Errorf("empty list must serialize as [] not null: %s", rec.Body)
	}
}
