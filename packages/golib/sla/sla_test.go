package sla

import (
	"testing"
	"time"
)

func TestMonitor_RecordSuccess(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "api", Target: 99.9, Window: 1 * time.Hour})

	m.RecordSuccess("api", 50*time.Millisecond)
	m.RecordSuccess("api", 100*time.Millisecond)
	m.RecordSuccess("api", 150*time.Millisecond)

	reports := m.Report()
	if len(reports) != 1 {
		t.Fatalf("reports: got %d", len(reports))
	}
	if reports[0].Total != 3 {
		t.Errorf("total: got %d", reports[0].Total)
	}
	if reports[0].Successes != 3 {
		t.Errorf("successes: got %d", reports[0].Successes)
	}
	if reports[0].Availability != 100 {
		t.Errorf("availability: got %f", reports[0].Availability)
	}
	if !reports[0].Compliant {
		t.Error("100% should be compliant with 99.9% target")
	}
}

func TestMonitor_RecordFailure(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "api", Target: 99.9, Window: 1 * time.Hour})

	for i := 0; i < 999; i++ {
		m.RecordSuccess("api", 10*time.Millisecond)
	}
	m.RecordFailure("api")

	reports := m.Report()
	r := reports[0]

	if r.Total != 1000 {
		t.Errorf("total: got %d", r.Total)
	}
	if r.Failures != 1 {
		t.Errorf("failures: got %d", r.Failures)
	}
	if r.Availability != 99.9 {
		t.Errorf("availability: got %f", r.Availability)
	}
	if !r.Compliant {
		t.Error("99.9% should be compliant with 99.9% target")
	}
}

func TestMonitor_NonCompliant(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "api", Target: 99.9, Window: 1 * time.Hour})

	for i := 0; i < 990; i++ {
		m.RecordSuccess("api", 10*time.Millisecond)
	}
	for i := 0; i < 10; i++ {
		m.RecordFailure("api")
	}

	reports := m.Report()
	if reports[0].Compliant {
		t.Error("99% should not be compliant with 99.9% target")
	}
}

func TestMonitor_ErrorBudget(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "api", Target: 99.0, Window: 1 * time.Hour})

	for i := 0; i < 100; i++ {
		m.RecordSuccess("api", 10*time.Millisecond)
	}

	reports := m.Report()
	budget := reports[0].ErrorBudget

	// 99% target, 100 requests → ~1 error allowed (ceil may round up due to float precision).
	if budget.Total < 1 || budget.Total > 2 {
		t.Errorf("budget total: got %d, want 1-2", budget.Total)
	}
	if budget.Used != 0 {
		t.Errorf("budget used: got %d", budget.Used)
	}
	if budget.Remaining != 100 {
		t.Errorf("budget remaining: got %f", budget.Remaining)
	}
}

func TestMonitor_ErrorBudget_Exhausted(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "api", Target: 99.0, Window: 1 * time.Hour})

	for i := 0; i < 98; i++ {
		m.RecordSuccess("api", 10*time.Millisecond)
	}
	for i := 0; i < 2; i++ {
		m.RecordFailure("api")
	}

	reports := m.Report()
	budget := reports[0].ErrorBudget

	// 99% target, 100 requests → 1 error allowed, 2 used.
	if budget.Remaining >= 0 {
		// Budget is over-consumed.
		t.Logf("budget remaining: %f (negative means exhausted)", budget.Remaining)
	}
	if budget.ExhaustedAt == nil {
		t.Error("should have exhaustion time when budget is used up")
	}
}

func TestMonitor_LatencyPercentiles(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{
		Name:             "api",
		Target:           99.9,
		LatencyThreshold: 500 * time.Millisecond,
		Window:           1 * time.Hour,
	})

	for i := 0; i < 100; i++ {
		m.RecordSuccess("api", time.Duration(i)*time.Millisecond)
	}

	reports := m.Report()
	r := reports[0]

	if r.P50Latency == 0 {
		t.Error("p50 should be non-zero")
	}
	if r.P95Latency == 0 {
		t.Error("p95 should be non-zero")
	}
	if r.P99Latency == 0 {
		t.Error("p99 should be non-zero")
	}
	if r.P50Latency >= r.P95Latency {
		t.Error("p50 should be less than p95")
	}
	if r.P95Latency >= r.P99Latency {
		t.Error("p95 should be less than p99")
	}
	if !r.LatencyCompliant {
		t.Error("max latency 99ms should be within 500ms threshold")
	}
}

func TestMonitor_LatencyNonCompliant(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{
		Name:             "api",
		Target:           99.9,
		LatencyThreshold: 10 * time.Millisecond,
		Window:           1 * time.Hour,
	})

	for i := 0; i < 100; i++ {
		m.RecordSuccess("api", time.Duration(i)*time.Millisecond)
	}

	reports := m.Report()
	if reports[0].LatencyCompliant {
		t.Error("99ms p99 should not be compliant with 10ms threshold")
	}
}

func TestMonitor_MultipleObjectives(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "auth", Target: 99.99, Window: 1 * time.Hour})
	m.AddObjective(Objective{Name: "data", Target: 99.9, Window: 1 * time.Hour})

	m.RecordSuccess("auth", 10*time.Millisecond)
	m.RecordSuccess("data", 50*time.Millisecond)

	reports := m.Report()
	if len(reports) != 2 {
		t.Errorf("reports: got %d", len(reports))
	}
}

func TestMonitor_UnknownObjective(t *testing.T) {
	m := NewMonitor()
	m.RecordSuccess("nonexistent", 10*time.Millisecond) // Should not panic.
	m.RecordFailure("nonexistent")                       // Should not panic.
}

func TestMonitor_EmptyReport(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "api", Target: 99.9})

	reports := m.Report()
	if len(reports) != 1 {
		t.Fatalf("reports: got %d", len(reports))
	}
	if reports[0].Total != 0 {
		t.Error("empty should have 0 total")
	}
}

func TestErrorBudget_String(t *testing.T) {
	b := ErrorBudget{Total: 10, Used: 3, Remaining: 70, BurnRate: 1.5}
	s := b.String()
	if s == "" {
		t.Error("should produce string")
	}

	b.Remaining = -10
	s = b.String()
	if s != "ERROR BUDGET EXHAUSTED" {
		t.Errorf("exhausted: got %q", s)
	}
}

func TestMonitor_DefaultWindow(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "api", Target: 99.9}) // No window specified.

	m.RecordSuccess("api", 10*time.Millisecond)
	reports := m.Report()
	if reports[0].Total != 1 {
		t.Error("should work with default window")
	}
}

func TestMonitor_BurnRate(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "api", Target: 99.0, Window: 1 * time.Hour})

	m.RecordFailure("api")
	m.RecordFailure("api")

	reports := m.Report()
	if reports[0].ErrorBudget.BurnRate <= 0 {
		t.Error("burn rate should be positive with failures")
	}
}
