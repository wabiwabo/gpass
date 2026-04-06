package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Policy defines a data retention policy.
type Policy struct {
	// Name identifies the policy (e.g., "audit_events", "user_sessions").
	Name string `json:"name"`
	// RetentionPeriod is how long data must be kept.
	RetentionPeriod time.Duration `json:"retention_period"`
	// Description explains the regulatory basis.
	Description string `json:"description"`
	// Regulation is the legal requirement (e.g., "PP 71/2019", "UU PDP").
	Regulation string `json:"regulation,omitempty"`
	// Enabled controls whether cleanup runs for this policy.
	Enabled bool `json:"enabled"`
}

// CleanupFunc is called to delete data older than the retention cutoff.
// Returns the number of records deleted.
type CleanupFunc func(ctx context.Context, olderThan time.Time) (int64, error)

// Registration pairs a policy with its cleanup function.
type Registration struct {
	Policy  Policy
	Cleanup CleanupFunc
}

// Report represents the result of a cleanup run.
type Report struct {
	PolicyName string    `json:"policy_name"`
	CutoffDate time.Time `json:"cutoff_date"`
	Deleted    int64     `json:"deleted"`
	Error      string    `json:"error,omitempty"`
	Duration   string    `json:"duration"`
	RunAt      time.Time `json:"run_at"`
}

// ComplianceReport summarizes retention compliance status.
type ComplianceReport struct {
	GeneratedAt    time.Time       `json:"generated_at"`
	Policies       []PolicyStatus  `json:"policies"`
	OverallStatus  string          `json:"overall_status"` // "compliant", "non_compliant", "unknown"
	LastCleanupRun *time.Time      `json:"last_cleanup_run,omitempty"`
}

// PolicyStatus is a single policy's compliance status.
type PolicyStatus struct {
	Name           string    `json:"name"`
	Regulation     string    `json:"regulation,omitempty"`
	RetentionDays  int       `json:"retention_days"`
	Enabled        bool      `json:"enabled"`
	LastCleanup    *Report   `json:"last_cleanup,omitempty"`
	Status         string    `json:"status"` // "compliant", "needs_cleanup", "disabled", "error"
}

// Enforcer manages data retention policies and runs periodic cleanup.
type Enforcer struct {
	mu            sync.RWMutex
	registrations []Registration
	reports       map[string]*Report
	logger        *slog.Logger
	lastRun       *time.Time
}

// NewEnforcer creates a new retention enforcer.
func NewEnforcer(logger *slog.Logger) *Enforcer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Enforcer{
		reports: make(map[string]*Report),
		logger:  logger,
	}
}

// Register adds a retention policy with its cleanup function.
func (e *Enforcer) Register(policy Policy, cleanup CleanupFunc) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.registrations = append(e.registrations, Registration{
		Policy:  policy,
		Cleanup: cleanup,
	})
}

// RunAll executes cleanup for all enabled policies.
func (e *Enforcer) RunAll(ctx context.Context) []Report {
	e.mu.RLock()
	regs := make([]Registration, len(e.registrations))
	copy(regs, e.registrations)
	e.mu.RUnlock()

	now := time.Now()
	var reports []Report

	for _, reg := range regs {
		if !reg.Policy.Enabled {
			continue
		}

		cutoff := now.Add(-reg.Policy.RetentionPeriod)
		start := time.Now()

		report := Report{
			PolicyName: reg.Policy.Name,
			CutoffDate: cutoff,
			RunAt:      now,
		}

		deleted, err := reg.Cleanup(ctx, cutoff)
		report.Duration = time.Since(start).String()
		report.Deleted = deleted

		if err != nil {
			report.Error = err.Error()
			e.logger.Error("retention cleanup failed",
				"policy", reg.Policy.Name,
				"error", err,
			)
		} else {
			e.logger.Info("retention cleanup completed",
				"policy", reg.Policy.Name,
				"deleted", deleted,
				"cutoff", cutoff.Format(time.RFC3339),
			)
		}

		reports = append(reports, report)

		e.mu.Lock()
		e.reports[reg.Policy.Name] = &report
		e.mu.Unlock()
	}

	e.mu.Lock()
	e.lastRun = &now
	e.mu.Unlock()

	return reports
}

// ComplianceStatus generates a compliance report.
func (e *Enforcer) ComplianceStatus() ComplianceReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	report := ComplianceReport{
		GeneratedAt:    time.Now(),
		LastCleanupRun: e.lastRun,
		OverallStatus:  "compliant",
	}

	for _, reg := range e.registrations {
		ps := PolicyStatus{
			Name:          reg.Policy.Name,
			Regulation:    reg.Policy.Regulation,
			RetentionDays: int(reg.Policy.RetentionPeriod.Hours() / 24),
			Enabled:       reg.Policy.Enabled,
		}

		if !reg.Policy.Enabled {
			ps.Status = "disabled"
			report.OverallStatus = "non_compliant"
		} else if r, ok := e.reports[reg.Policy.Name]; ok {
			ps.LastCleanup = r
			if r.Error != "" {
				ps.Status = "error"
				report.OverallStatus = "non_compliant"
			} else {
				ps.Status = "compliant"
			}
		} else {
			ps.Status = "needs_cleanup"
			if report.OverallStatus != "non_compliant" {
				report.OverallStatus = "unknown"
			}
		}

		report.Policies = append(report.Policies, ps)
	}

	return report
}

// Policies returns all registered policies.
func (e *Enforcer) Policies() []Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policies := make([]Policy, len(e.registrations))
	for i, r := range e.registrations {
		policies[i] = r.Policy
	}
	return policies
}

// Handler returns an HTTP handler exposing compliance status.
func (e *Enforcer) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		status := e.ComplianceStatus()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(status)
	}
}

// Predefined policies for Indonesian regulatory compliance.

// PP71AuditRetention returns the PP 71/2019 5-year audit trail retention policy.
func PP71AuditRetention() Policy {
	return Policy{
		Name:            "audit_events",
		RetentionPeriod: 5 * 365 * 24 * time.Hour, // 5 years
		Description:     "Audit event retention per PP 71/2019 on Electronic System Operation",
		Regulation:      "PP 71/2019",
		Enabled:         true,
	}
}

// UUPDPDataRetention returns the UU PDP personal data retention policy.
func UUPDPDataRetention() Policy {
	return Policy{
		Name:            "personal_data",
		RetentionPeriod: 5 * 365 * 24 * time.Hour,
		Description:     "Personal data retention per UU PDP No. 27/2022 — data must be deleted when no longer needed or upon erasure request",
		Regulation:      "UU PDP No. 27/2022",
		Enabled:         true,
	}
}

// SessionRetention returns a session data retention policy (30 days).
func SessionRetention() Policy {
	return Policy{
		Name:            "sessions",
		RetentionPeriod: 30 * 24 * time.Hour,
		Description:     "Expired session cleanup",
		Enabled:         true,
	}
}

// String returns a human-readable description of the policy.
func (p Policy) String() string {
	days := int(p.RetentionPeriod.Hours() / 24)
	return fmt.Sprintf("%s: %d days (%s)", p.Name, days, p.Regulation)
}
