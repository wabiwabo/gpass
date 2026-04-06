package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudaaudit/store"
)

// ComplianceHandler generates regulatory compliance reports.
type ComplianceHandler struct {
	store store.AuditStore
}

// NewComplianceHandler creates a new ComplianceHandler.
func NewComplianceHandler(s store.AuditStore) *ComplianceHandler {
	return &ComplianceHandler{store: s}
}

type complianceRequest struct {
	ReportType string `json:"report_type"`
	From       string `json:"from"`
	To         string `json:"to"`
	EntityID   string `json:"entity_id,omitempty"`
}

type compliancePeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type complianceSummary struct {
	TotalEvents       int64 `json:"total_events"`
	ConsentGrants     int64 `json:"consent_grants"`
	ConsentRevocations int64 `json:"consent_revocations"`
	DataDeletions     int64 `json:"data_deletions"`
	DataExports       int64 `json:"data_exports"`
	VerificationCount int64 `json:"verification_count"`
}

type complianceFinding struct {
	Severity       string `json:"severity"`
	Finding        string `json:"finding"`
	Recommendation string `json:"recommendation"`
}

type complianceReport struct {
	ReportID         string              `json:"report_id"`
	ReportType       string              `json:"report_type"`
	GeneratedAt      string              `json:"generated_at"`
	Period           compliancePeriod    `json:"period"`
	Summary          complianceSummary   `json:"summary"`
	ComplianceStatus string              `json:"compliance_status"`
	Findings         []complianceFinding `json:"findings"`
}

var validReportTypes = map[string]bool{
	"uu_pdp":           true,
	"pp_71_2019":       true,
	"data_access":      true,
	"signing_activity": true,
}

// GenerateReport handles POST /api/v1/audit/compliance/report.
func (h *ComplianceHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	var req complianceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	if !validReportTypes[req.ReportType] {
		writeError(w, http.StatusBadRequest, "invalid_report_type",
			fmt.Sprintf("invalid report_type: must be one of uu_pdp, pp_71_2019, data_access, signing_activity"))
		return
	}

	if req.From == "" || req.To == "" {
		writeError(w, http.StatusBadRequest, "missing_date_range", "from and to date fields are required")
		return
	}

	fromTime, err := time.Parse(time.RFC3339, req.From)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", "from must be a valid RFC3339 timestamp")
		return
	}
	toTime, err := time.Parse(time.RFC3339, req.To)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", "to must be a valid RFC3339 timestamp")
		return
	}

	filter := store.AuditFilter{
		From:  fromTime,
		To:    toTime,
		Limit: 10000,
	}
	if req.EntityID != "" {
		filter.ActorID = req.EntityID
	}

	totalEvents, err := h.store.Count(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query_error", err.Error())
		return
	}

	summary := h.buildSummary(filter, totalEvents)
	findings := h.evaluateCompliance(req.ReportType, summary, fromTime, toTime)
	status := determineStatus(findings)

	reportID := generateReportID()

	report := complianceReport{
		ReportID:         reportID,
		ReportType:       req.ReportType,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		Period:           compliancePeriod{From: req.From, To: req.To},
		Summary:          summary,
		ComplianceStatus: status,
		Findings:         findings,
	}

	writeJSON(w, http.StatusOK, report)
}

func (h *ComplianceHandler) buildSummary(baseFilter store.AuditFilter, totalEvents int64) complianceSummary {
	s := complianceSummary{TotalEvents: totalEvents}

	countByAction := func(action string) int64 {
		f := baseFilter
		f.Action = action
		n, _ := h.store.Count(f)
		return n
	}

	s.ConsentGrants = countByAction("CONSENT_GRANT")
	s.ConsentRevocations = countByAction("CONSENT_REVOKE")
	s.DataDeletions = countByAction("DATA_DELETE")
	s.DataExports = countByAction("DATA_EXPORT")
	s.VerificationCount = countByAction("VERIFY")

	return s
}

func (h *ComplianceHandler) evaluateCompliance(reportType string, summary complianceSummary, from, to time.Time) []complianceFinding {
	var findings []complianceFinding

	switch reportType {
	case "uu_pdp":
		findings = h.evaluateUUPDP(summary)
	case "pp_71_2019":
		findings = h.evaluatePP71(summary, from, to)
	case "data_access":
		findings = h.evaluateDataAccess(summary)
	case "signing_activity":
		findings = h.evaluateSigningActivity(summary)
	}

	return findings
}

func (h *ComplianceHandler) evaluateUUPDP(summary complianceSummary) []complianceFinding {
	var findings []complianceFinding

	if summary.ConsentGrants > 0 {
		findings = append(findings, complianceFinding{
			Severity:       "INFO",
			Finding:        fmt.Sprintf("%d consent grants recorded in period", summary.ConsentGrants),
			Recommendation: "Continue recording all consent activities",
		})
	}

	if summary.ConsentRevocations > 0 && summary.DataDeletions == 0 {
		findings = append(findings, complianceFinding{
			Severity:       "WARNING",
			Finding:        "Consent revocations exist but no data deletions recorded",
			Recommendation: "Verify that data deletion follows consent revocation per UU PDP Article 8",
		})
	}

	if summary.DataExports == 0 && summary.TotalEvents > 0 {
		findings = append(findings, complianceFinding{
			Severity:       "WARNING",
			Finding:        "No data export requests recorded in period",
			Recommendation: "Ensure data portability mechanism is available per UU PDP Article 13",
		})
	}

	if summary.TotalEvents == 0 {
		findings = append(findings, complianceFinding{
			Severity:       "INFO",
			Finding:        "No events recorded in the specified period",
			Recommendation: "No compliance issues for empty period",
		})
	}

	return findings
}

func (h *ComplianceHandler) evaluatePP71(summary complianceSummary, from, to time.Time) []complianceFinding {
	var findings []complianceFinding

	periodDuration := to.Sub(from)
	fiveYears := 5 * 365 * 24 * time.Hour

	if periodDuration < fiveYears && summary.TotalEvents > 0 {
		findings = append(findings, complianceFinding{
			Severity:       "INFO",
			Finding:        "Audit period is less than 5 years",
			Recommendation: "Ensure audit logs are retained for at least 5 years per PP 71/2019",
		})
	}

	if summary.TotalEvents == 0 {
		findings = append(findings, complianceFinding{
			Severity:       "WARNING",
			Finding:        "No audit events found in period — potential gap in audit trail",
			Recommendation: "Investigate whether audit logging was active during this period",
		})
	} else {
		findings = append(findings, complianceFinding{
			Severity:       "INFO",
			Finding:        fmt.Sprintf("Audit trail contains %d events in period", summary.TotalEvents),
			Recommendation: "Continue maintaining comprehensive audit logging",
		})
	}

	return findings
}

func (h *ComplianceHandler) evaluateDataAccess(summary complianceSummary) []complianceFinding {
	var findings []complianceFinding

	if summary.VerificationCount > 0 && summary.ConsentGrants == 0 {
		findings = append(findings, complianceFinding{
			Severity:       "CRITICAL",
			Finding:        "Data access events exist without any consent grants",
			Recommendation: "All data access must have a valid consent basis per UU PDP",
		})
	}

	if summary.VerificationCount > 0 && summary.ConsentGrants > 0 {
		findings = append(findings, complianceFinding{
			Severity:       "INFO",
			Finding:        fmt.Sprintf("%d verifications with %d consent grants recorded", summary.VerificationCount, summary.ConsentGrants),
			Recommendation: "Verify each data access event is linked to a valid consent record",
		})
	}

	if summary.TotalEvents == 0 {
		findings = append(findings, complianceFinding{
			Severity:       "INFO",
			Finding:        "No data access events in period",
			Recommendation: "No compliance issues for empty period",
		})
	}

	return findings
}

func (h *ComplianceHandler) evaluateSigningActivity(summary complianceSummary) []complianceFinding {
	var findings []complianceFinding

	// Query for signing events
	f := store.AuditFilter{Action: "SIGN"}
	signCount, _ := h.store.Count(f)

	if signCount > 0 {
		findings = append(findings, complianceFinding{
			Severity:       "INFO",
			Finding:        fmt.Sprintf("%d signing activities recorded", signCount),
			Recommendation: "Ensure all signatures use valid, non-revoked certificates",
		})
	}

	if summary.TotalEvents == 0 {
		findings = append(findings, complianceFinding{
			Severity:       "INFO",
			Finding:        "No signing activity in period",
			Recommendation: "No compliance issues for empty period",
		})
	}

	return findings
}

func determineStatus(findings []complianceFinding) string {
	hasCritical := false
	hasWarning := false

	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			hasCritical = true
		case "WARNING":
			hasWarning = true
		}
	}

	if hasCritical {
		return "NON_COMPLIANT"
	}
	if hasWarning {
		return "REVIEW_REQUIRED"
	}
	return "COMPLIANT"
}

func generateReportID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate report ID: %v", err))
	}
	return hex.EncodeToString(b)
}
