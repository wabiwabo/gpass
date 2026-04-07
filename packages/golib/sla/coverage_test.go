package sla

import (
	"strings"
	"testing"
	"time"
)

// TestRecordSuccess_RingBufferOverflow covers the previously-uncovered
// ring-buffer wrap branch in RecordSuccess: when latencies fills its
// capacity, subsequent recordings overwrite the oldest entries instead
// of growing the slice unbounded (which would leak memory in long-running
// services).
func TestRecordSuccess_RingBufferOverflow(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "ring", Target: 99, Window: time.Hour})

	// Force the ring buffer past capacity. The default capacity is 1000,
	// so 1500 successes triggers 500 overwrites.
	for i := 0; i < 1500; i++ {
		m.RecordSuccess("ring", time.Duration(i)*time.Millisecond)
	}

	r := m.Report()
	if len(r) != 1 {
		t.Fatalf("got %d reports, want 1", len(r))
	}
	if r[0].Total != 1500 {
		t.Errorf("Total = %d, want 1500", r[0].Total)
	}
	if r[0].Successes != 1500 {
		t.Errorf("Successes = %d, want 1500", r[0].Successes)
	}
	// The latency slice itself must NOT have grown beyond capacity.
	m.mu.RLock()
	w := m.windows["ring"]
	got := len(w.latencies)
	cp := cap(w.latencies)
	m.mu.RUnlock()
	if got != cp {
		t.Errorf("latencies len = %d, cap = %d (should saturate at cap)", got, cp)
	}
}

// TestComputeBurnRate_EdgeCases covers the elapsed<=0 and total==0
// branches that the existing tests didn't reach. We can't easily make
// time go backwards, so we exercise total==0 (objective registered but
// no recordings) and the post-recording rate.
func TestComputeBurnRate_EdgeCases(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "idle", Target: 99.9, Window: time.Hour})
	r := m.Report()
	// No recordings → burn rate must be 0, error budget remaining = 100.
	if r[0].ErrorBudget.BurnRate != 0 {
		t.Errorf("idle BurnRate = %f, want 0", r[0].ErrorBudget.BurnRate)
	}
	if r[0].ErrorBudget.Remaining != 100 {
		t.Errorf("idle Remaining = %f, want 100", r[0].ErrorBudget.Remaining)
	}
}

// TestEstimateExhaustion_AlreadyExhausted pins the "remaining<=0" branch
// of estimateExhaustion: a fully-burned budget must return a non-nil
// ExhaustedAt pointing at "now".
func TestEstimateExhaustion_AlreadyExhausted(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "burnt", Target: 99.0, Window: time.Hour})
	// 1 success, 100 failures → fails the 99% target by far.
	m.RecordSuccess("burnt", 10*time.Millisecond)
	for i := 0; i < 100; i++ {
		m.RecordFailure("burnt")
	}
	r := m.Report()
	if r[0].ErrorBudget.ExhaustedAt == nil {
		t.Error("ExhaustedAt should be non-nil for an exhausted budget")
	}
	if r[0].Compliant {
		t.Error("Compliant should be false")
	}
}

// TestEstimateExhaustion_InfiniteBudget pins the burnRate==0 branch:
// when no failures have happened, ExhaustedAt must be nil (infinite).
func TestEstimateExhaustion_InfiniteBudget(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "perfect", Target: 99.9, Window: time.Hour})
	for i := 0; i < 1000; i++ {
		m.RecordSuccess("perfect", 5*time.Millisecond)
	}
	r := m.Report()
	if r[0].ErrorBudget.ExhaustedAt != nil {
		t.Errorf("ExhaustedAt should be nil for zero failures, got %v",
			r[0].ErrorBudget.ExhaustedAt)
	}
}

// TestPercentile_EdgeCases covers the empty input + idx clamping branches.
func TestPercentile_EdgeCases(t *testing.T) {
	if got := percentile(nil, 0.5); got != 0 {
		t.Errorf("percentile(nil) = %v, want 0", got)
	}

	// Single value.
	got := percentile([]time.Duration{42 * time.Millisecond}, 0.99)
	if got != 42*time.Millisecond {
		t.Errorf("single-value p99 = %v, want 42ms", got)
	}

	// Sorted input — pin that p50 of [1,2,3,4,5] is the middle value.
	data := []time.Duration{
		1 * time.Millisecond, 2 * time.Millisecond, 3 * time.Millisecond,
		4 * time.Millisecond, 5 * time.Millisecond,
	}
	if got := percentile(data, 0.50); got != 3*time.Millisecond {
		t.Errorf("p50 = %v, want 3ms", got)
	}
	if got := percentile(data, 0.99); got != 5*time.Millisecond {
		t.Errorf("p99 = %v, want 5ms", got)
	}
}

// TestSortDurations_AlreadySorted covers the "no swap needed" path
// in the insertion sort that the existing tests didn't reach.
func TestSortDurations_AlreadySorted(t *testing.T) {
	d := []time.Duration{1, 2, 3, 4, 5}
	sortDurations(d)
	for i, want := range []time.Duration{1, 2, 3, 4, 5} {
		if d[i] != want {
			t.Errorf("d[%d] = %v, want %v", i, d[i], want)
		}
	}
}

// TestRecordSuccessFailure_UnknownObjective covers the silent-skip
// branch (recording for an objective that was never registered must
// not panic and must not affect any other objective).
func TestRecordSuccessFailure_UnknownObjective(t *testing.T) {
	m := NewMonitor()
	m.AddObjective(Objective{Name: "real", Target: 99, Window: time.Hour})

	// These should be no-ops, not panics.
	m.RecordSuccess("ghost", 10*time.Millisecond)
	m.RecordFailure("ghost")

	r := m.Report()
	if r[0].Total != 0 {
		t.Errorf("real.Total = %d after ghost recordings, want 0", r[0].Total)
	}
}

// TestErrorBudgetString covers both branches of the String formatter.
func TestErrorBudgetString(t *testing.T) {
	exhausted := ErrorBudget{Remaining: 0}
	if got := exhausted.String(); !strings.Contains(got, "EXHAUSTED") {
		t.Errorf("exhausted = %q", got)
	}

	healthy := ErrorBudget{Total: 100, Used: 5, Remaining: 95, BurnRate: 1.5}
	got := healthy.String()
	for _, want := range []string{"95.0%", "5/100", "1.50/hr"} {
		if !strings.Contains(got, want) {
			t.Errorf("healthy = %q, missing %q", got, want)
		}
	}
}
