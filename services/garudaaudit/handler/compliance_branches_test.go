package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaaudit/store"
)

// TestGenerateReport_SigningActivity pins the evaluateSigningActivity
// path (0% → covered) — seed SIGN events and verify findings include
// the signing count.
func TestGenerateReport_SigningActivity(t *testing.T) {
	h, s := setupComplianceHandler()
	s.Append(&store.AuditEvent{EventType: "doc.signed", ActorID: "user-1", Action: "SIGN", ServiceName: "garudasign"})
	s.Append(&store.AuditEvent{EventType: "doc.signed", ActorID: "user-2", Action: "SIGN", ServiceName: "garudasign"})

	body := `{"report_type":"signing_activity","from":"2025-01-01T00:00:00Z","to":"2026-12-31T23:59:59Z"}`
	req := httptest.NewRequest("POST", "/api/v1/audit/compliance/report", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.GenerateReport(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d body=%s", rec.Code, rec.Body)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"signing_activity"`)) {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestGenerateReport_SigningActivity_Empty pins the zero-events
// branch of evaluateSigningActivity.
func TestGenerateReport_SigningActivity_Empty(t *testing.T) {
	h, _ := setupComplianceHandler()
	body := `{"report_type":"signing_activity","from":"2025-01-01T00:00:00Z","to":"2025-12-31T23:59:59Z"}`
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	h.GenerateReport(rec, req)
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("No signing activity")) {
		t.Errorf("body: %s", rec.Body)
	}
}

// TestGenerateReport_RejectionMatrix pins all 4xx branches of
// GenerateReport.
func TestGenerateReport_RejectionMatrix(t *testing.T) {
	h, _ := setupComplianceHandler()
	mk := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.GenerateReport(rec, httptest.NewRequest("POST", "/", bytes.NewBufferString(body)))
		return rec
	}

	if rec := mk("{not json"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad json: %d", rec.Code)
	}
	if rec := mk(`{"report_type":"bogus"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad type: %d", rec.Code)
	}
	if rec := mk(`{"report_type":"uu_pdp"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("missing dates: %d", rec.Code)
	}
	if rec := mk(`{"report_type":"uu_pdp","from":"not-a-date","to":"2025-12-31T23:59:59Z"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad from: %d", rec.Code)
	}
	if rec := mk(`{"report_type":"uu_pdp","from":"2025-01-01T00:00:00Z","to":"not-a-date"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("bad to: %d", rec.Code)
	}
}

// TestGenerateReport_WithEntityIDFilter pins the entity_id filter
// branch, which sets filter.ActorID.
func TestGenerateReport_WithEntityIDFilter(t *testing.T) {
	h, s := setupComplianceHandler()
	s.Append(&store.AuditEvent{EventType: "consent.granted", ActorID: "target-user", Action: "CONSENT_GRANT", ServiceName: "garudainfo"})

	body := `{"report_type":"uu_pdp","entity_id":"target-user","from":"2025-01-01T00:00:00Z","to":"2026-12-31T23:59:59Z"}`
	rec := httptest.NewRecorder()
	h.GenerateReport(rec, httptest.NewRequest("POST", "/", bytes.NewBufferString(body)))
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
}

// TestGenerateReport_DataAccessCritical pins the CRITICAL-severity
// branch: data access events without any consent grants.
func TestGenerateReport_DataAccessCritical(t *testing.T) {
	h, s := setupComplianceHandler()
	s.Append(&store.AuditEvent{EventType: "identity.verified", ActorID: "user-1", Action: "VERIFY", ServiceName: "identity"})

	body := `{"report_type":"data_access","from":"2025-01-01T00:00:00Z","to":"2026-12-31T23:59:59Z"}`
	rec := httptest.NewRecorder()
	h.GenerateReport(rec, httptest.NewRequest("POST", "/", bytes.NewBufferString(body)))
	if rec.Code != 200 {
		t.Fatalf("code = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("NON_COMPLIANT")) {
		t.Errorf("should be NON_COMPLIANT: %s", rec.Body)
	}
}

// TestDetermineStatus_ReviewRequired pins the WARNING-only branch.
func TestDetermineStatus_ReviewRequired(t *testing.T) {
	findings := []complianceFinding{
		{Severity: "WARNING"},
		{Severity: "INFO"},
	}
	if got := determineStatus(findings); got != "REVIEW_REQUIRED" {
		t.Errorf("got %s", got)
	}
}
