package retention

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPP71AuditRetention(t *testing.T) {
	p := PP71AuditRetention()
	if p.Name != "audit_events" {
		t.Errorf("name: got %q", p.Name)
	}
	days := int(p.RetentionPeriod.Hours() / 24)
	if days != 1825 {
		t.Errorf("retention days: got %d, want 1825 (5 years)", days)
	}
	if p.Regulation != "PP 71/2019" {
		t.Errorf("regulation: got %q", p.Regulation)
	}
	if !p.Enabled {
		t.Error("should be enabled by default")
	}
}

func TestUUPDPDataRetention(t *testing.T) {
	p := UUPDPDataRetention()
	if p.Regulation != "UU PDP No. 27/2022" {
		t.Errorf("regulation: got %q", p.Regulation)
	}
}

func TestSessionRetention(t *testing.T) {
	p := SessionRetention()
	days := int(p.RetentionPeriod.Hours() / 24)
	if days != 30 {
		t.Errorf("retention days: got %d, want 30", days)
	}
}

func TestPolicy_String(t *testing.T) {
	p := PP71AuditRetention()
	s := p.String()
	if s == "" {
		t.Error("String() should not be empty")
	}
}

func TestEnforcer_Register(t *testing.T) {
	e := NewEnforcer(nil)
	e.Register(PP71AuditRetention(), func(_ context.Context, _ time.Time) (int64, error) {
		return 0, nil
	})

	policies := e.Policies()
	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
	if policies[0].Name != "audit_events" {
		t.Errorf("policy name: got %q", policies[0].Name)
	}
}

func TestEnforcer_RunAll_Success(t *testing.T) {
	e := NewEnforcer(nil)

	var deletedCount atomic.Int64
	e.Register(PP71AuditRetention(), func(_ context.Context, olderThan time.Time) (int64, error) {
		if olderThan.IsZero() {
			t.Error("cutoff should not be zero")
		}
		deletedCount.Add(42)
		return 42, nil
	})

	reports := e.RunAll(context.Background())
	if len(reports) != 1 {
		t.Fatalf("expected 1 report, got %d", len(reports))
	}

	r := reports[0]
	if r.PolicyName != "audit_events" {
		t.Errorf("policy name: got %q", r.PolicyName)
	}
	if r.Deleted != 42 {
		t.Errorf("deleted: got %d, want 42", r.Deleted)
	}
	if r.Error != "" {
		t.Errorf("unexpected error: %s", r.Error)
	}
}

func TestEnforcer_RunAll_Error(t *testing.T) {
	e := NewEnforcer(nil)
	e.Register(PP71AuditRetention(), func(_ context.Context, _ time.Time) (int64, error) {
		return 0, errors.New("database unavailable")
	})

	reports := e.RunAll(context.Background())
	if reports[0].Error != "database unavailable" {
		t.Errorf("error: got %q", reports[0].Error)
	}
}

func TestEnforcer_RunAll_SkipsDisabled(t *testing.T) {
	e := NewEnforcer(nil)
	p := PP71AuditRetention()
	p.Enabled = false
	e.Register(p, func(_ context.Context, _ time.Time) (int64, error) {
		t.Fatal("should not be called for disabled policy")
		return 0, nil
	})

	reports := e.RunAll(context.Background())
	if len(reports) != 0 {
		t.Errorf("expected 0 reports for disabled, got %d", len(reports))
	}
}

func TestEnforcer_ComplianceStatus_AllCompliant(t *testing.T) {
	e := NewEnforcer(nil)
	e.Register(PP71AuditRetention(), func(_ context.Context, _ time.Time) (int64, error) {
		return 10, nil
	})

	e.RunAll(context.Background())
	status := e.ComplianceStatus()

	if status.OverallStatus != "compliant" {
		t.Errorf("overall: got %q, want compliant", status.OverallStatus)
	}
	if len(status.Policies) != 1 {
		t.Fatalf("expected 1 policy status, got %d", len(status.Policies))
	}
	if status.Policies[0].Status != "compliant" {
		t.Errorf("policy status: got %q", status.Policies[0].Status)
	}
}

func TestEnforcer_ComplianceStatus_NonCompliant(t *testing.T) {
	e := NewEnforcer(nil)
	p := PP71AuditRetention()
	p.Enabled = false
	e.Register(p, func(_ context.Context, _ time.Time) (int64, error) {
		return 0, nil
	})

	status := e.ComplianceStatus()
	if status.OverallStatus != "non_compliant" {
		t.Errorf("disabled policy should make overall non_compliant, got %q", status.OverallStatus)
	}
}

func TestEnforcer_ComplianceStatus_NeedsCleanup(t *testing.T) {
	e := NewEnforcer(nil)
	e.Register(PP71AuditRetention(), func(_ context.Context, _ time.Time) (int64, error) {
		return 0, nil
	})

	// Don't run cleanup — status should show needs_cleanup.
	status := e.ComplianceStatus()
	if status.Policies[0].Status != "needs_cleanup" {
		t.Errorf("before cleanup: got %q, want needs_cleanup", status.Policies[0].Status)
	}
}

func TestEnforcer_ComplianceStatus_Error(t *testing.T) {
	e := NewEnforcer(nil)
	e.Register(PP71AuditRetention(), func(_ context.Context, _ time.Time) (int64, error) {
		return 0, errors.New("fail")
	})

	e.RunAll(context.Background())
	status := e.ComplianceStatus()

	if status.OverallStatus != "non_compliant" {
		t.Errorf("error should make non_compliant, got %q", status.OverallStatus)
	}
	if status.Policies[0].Status != "error" {
		t.Errorf("policy status: got %q, want error", status.Policies[0].Status)
	}
}

func TestEnforcer_Handler_JSON(t *testing.T) {
	e := NewEnforcer(nil)
	e.Register(PP71AuditRetention(), func(_ context.Context, _ time.Time) (int64, error) {
		return 5, nil
	})
	e.RunAll(context.Background())

	req := httptest.NewRequest(http.MethodGet, "/retention", nil)
	w := httptest.NewRecorder()
	e.Handler()(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}

	var report ComplianceReport
	json.NewDecoder(w.Body).Decode(&report)
	if report.OverallStatus != "compliant" {
		t.Errorf("overall: got %q", report.OverallStatus)
	}
}

func TestEnforcer_MultiplePolicies(t *testing.T) {
	e := NewEnforcer(nil)
	e.Register(PP71AuditRetention(), func(_ context.Context, _ time.Time) (int64, error) {
		return 100, nil
	})
	e.Register(SessionRetention(), func(_ context.Context, _ time.Time) (int64, error) {
		return 50, nil
	})
	e.Register(UUPDPDataRetention(), func(_ context.Context, _ time.Time) (int64, error) {
		return 0, nil
	})

	reports := e.RunAll(context.Background())
	if len(reports) != 3 {
		t.Fatalf("expected 3 reports, got %d", len(reports))
	}

	total := int64(0)
	for _, r := range reports {
		total += r.Deleted
	}
	if total != 150 {
		t.Errorf("total deleted: got %d, want 150", total)
	}
}
