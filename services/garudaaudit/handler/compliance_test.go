package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaaudit/store"
)

func setupComplianceHandler() (*ComplianceHandler, *store.InMemoryAuditStore) {
	s := store.NewInMemoryAuditStore()
	h := NewComplianceHandler(s)
	return h, s
}

func seedConsentEvents(s *store.InMemoryAuditStore) {
	s.Append(&store.AuditEvent{
		EventType: "consent.granted", ActorID: "user-1", Action: "CONSENT_GRANT",
		ServiceName: "garudainfo",
	})
	s.Append(&store.AuditEvent{
		EventType: "consent.granted", ActorID: "user-2", Action: "CONSENT_GRANT",
		ServiceName: "garudainfo",
	})
	s.Append(&store.AuditEvent{
		EventType: "consent.revoked", ActorID: "user-1", Action: "CONSENT_REVOKE",
		ServiceName: "garudainfo",
	})
	s.Append(&store.AuditEvent{
		EventType: "data.exported", ActorID: "user-1", Action: "DATA_EXPORT",
		ServiceName: "garudainfo",
	})
	s.Append(&store.AuditEvent{
		EventType: "data.deleted", ActorID: "user-1", Action: "DATA_DELETE",
		ServiceName: "garudainfo",
	})
	s.Append(&store.AuditEvent{
		EventType: "identity.verified", ActorID: "user-2", Action: "VERIFY",
		ServiceName: "identity",
	})
}

func TestGenerateReport_UUPDP(t *testing.T) {
	h, s := setupComplianceHandler()
	seedConsentEvents(s)

	body := `{
		"report_type": "uu_pdp",
		"from": "2020-01-01T00:00:00Z",
		"to": "2030-12-31T23:59:59Z"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/compliance/report", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.GenerateReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var report complianceReport
	if err := json.NewDecoder(w.Body).Decode(&report); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if report.ReportID == "" {
		t.Error("expected report_id")
	}
	if report.ReportType != "uu_pdp" {
		t.Errorf("expected report_type uu_pdp, got %s", report.ReportType)
	}
	if report.GeneratedAt == "" {
		t.Error("expected generated_at")
	}
	if report.Summary.TotalEvents != 6 {
		t.Errorf("expected 6 total events, got %d", report.Summary.TotalEvents)
	}
	if report.Summary.ConsentGrants != 2 {
		t.Errorf("expected 2 consent grants, got %d", report.Summary.ConsentGrants)
	}
	if report.Summary.ConsentRevocations != 1 {
		t.Errorf("expected 1 consent revocation, got %d", report.Summary.ConsentRevocations)
	}
	if report.Summary.DataDeletions != 1 {
		t.Errorf("expected 1 data deletion, got %d", report.Summary.DataDeletions)
	}
	if report.Summary.DataExports != 1 {
		t.Errorf("expected 1 data export, got %d", report.Summary.DataExports)
	}
	if report.Summary.VerificationCount != 1 {
		t.Errorf("expected 1 verification, got %d", report.Summary.VerificationCount)
	}
	if report.ComplianceStatus != "COMPLIANT" {
		t.Errorf("expected COMPLIANT, got %s", report.ComplianceStatus)
	}
}

func TestGenerateReport_PP71(t *testing.T) {
	h, s := setupComplianceHandler()
	seedConsentEvents(s)

	body := `{
		"report_type": "pp_71_2019",
		"from": "2020-01-01T00:00:00Z",
		"to": "2030-12-31T23:59:59Z"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/compliance/report", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.GenerateReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var report complianceReport
	json.NewDecoder(w.Body).Decode(&report)

	if report.ReportType != "pp_71_2019" {
		t.Errorf("expected report_type pp_71_2019, got %s", report.ReportType)
	}
	if report.Summary.TotalEvents != 6 {
		t.Errorf("expected 6 total events, got %d", report.Summary.TotalEvents)
	}
	if len(report.Findings) == 0 {
		t.Error("expected at least one finding")
	}
}

func TestGenerateReport_DataAccess(t *testing.T) {
	h, s := setupComplianceHandler()
	seedConsentEvents(s)

	body := `{
		"report_type": "data_access",
		"from": "2020-01-01T00:00:00Z",
		"to": "2030-12-31T23:59:59Z"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/compliance/report", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.GenerateReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var report complianceReport
	json.NewDecoder(w.Body).Decode(&report)

	if report.ReportType != "data_access" {
		t.Errorf("expected report_type data_access, got %s", report.ReportType)
	}
	if report.ComplianceStatus != "COMPLIANT" {
		t.Errorf("expected COMPLIANT, got %s", report.ComplianceStatus)
	}
}

func TestGenerateReport_InvalidReportType(t *testing.T) {
	h, _ := setupComplianceHandler()

	body := `{
		"report_type": "invalid_type",
		"from": "2020-01-01T00:00:00Z",
		"to": "2030-12-31T23:59:59Z"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/compliance/report", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.GenerateReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGenerateReport_MissingDateRange(t *testing.T) {
	h, _ := setupComplianceHandler()

	body := `{"report_type": "uu_pdp"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/compliance/report", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.GenerateReport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGenerateReport_EmptyPeriod(t *testing.T) {
	h, _ := setupComplianceHandler()

	body := `{
		"report_type": "uu_pdp",
		"from": "2099-01-01T00:00:00Z",
		"to": "2099-12-31T23:59:59Z"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/audit/compliance/report", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.GenerateReport(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var report complianceReport
	json.NewDecoder(w.Body).Decode(&report)

	if report.Summary.TotalEvents != 0 {
		t.Errorf("expected 0 total events, got %d", report.Summary.TotalEvents)
	}
	if report.Summary.ConsentGrants != 0 {
		t.Errorf("expected 0 consent grants, got %d", report.Summary.ConsentGrants)
	}
	if report.Summary.ConsentRevocations != 0 {
		t.Errorf("expected 0 consent revocations, got %d", report.Summary.ConsentRevocations)
	}
	if report.Summary.DataDeletions != 0 {
		t.Errorf("expected 0 data deletions, got %d", report.Summary.DataDeletions)
	}
	if report.Summary.DataExports != 0 {
		t.Errorf("expected 0 data exports, got %d", report.Summary.DataExports)
	}
	if report.Summary.VerificationCount != 0 {
		t.Errorf("expected 0 verifications, got %d", report.Summary.VerificationCount)
	}
}
