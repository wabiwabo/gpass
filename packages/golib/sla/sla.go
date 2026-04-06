// Package sla provides SLA monitoring with error budget tracking.
// It monitors availability, latency, and throughput against defined
// objectives and calculates remaining error budgets.
package sla

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Objective defines an SLA target.
type Objective struct {
	Name             string        `json:"name"`
	Target           float64       `json:"target"`            // Target percentage (e.g., 99.9).
	LatencyThreshold time.Duration `json:"latency_threshold"` // Max acceptable latency.
	Window           time.Duration `json:"window"`            // Measurement window.
}

// Monitor tracks SLA compliance for a service.
type Monitor struct {
	mu         sync.RWMutex
	objectives []Objective
	windows    map[string]*window
}

type window struct {
	total     int64
	successes int64
	failures  int64
	latencies []time.Duration
	startTime time.Time
	duration  time.Duration
}

// NewMonitor creates a new SLA monitor.
func NewMonitor() *Monitor {
	return &Monitor{
		windows: make(map[string]*window),
	}
}

// AddObjective registers an SLA objective.
func (m *Monitor) AddObjective(obj Objective) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if obj.Window <= 0 {
		obj.Window = 30 * 24 * time.Hour // 30 days default.
	}

	m.objectives = append(m.objectives, obj)
	m.windows[obj.Name] = &window{
		startTime: time.Now(),
		duration:  obj.Window,
		latencies: make([]time.Duration, 0, 1000),
	}
}

// RecordSuccess records a successful operation.
func (m *Monitor) RecordSuccess(name string, latency time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.windows[name]
	if !ok {
		return
	}

	w.total++
	w.successes++
	if len(w.latencies) < cap(w.latencies) {
		w.latencies = append(w.latencies, latency)
	} else {
		// Ring buffer behavior — overwrite oldest.
		w.latencies[int(w.total-1)%cap(w.latencies)] = latency
	}
}

// RecordFailure records a failed operation.
func (m *Monitor) RecordFailure(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	w, ok := m.windows[name]
	if !ok {
		return
	}

	w.total++
	w.failures++
}

// Report generates an SLA compliance report.
func (m *Monitor) Report() []ObjectiveReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]ObjectiveReport, 0, len(m.objectives))

	for _, obj := range m.objectives {
		w, ok := m.windows[obj.Name]
		if !ok {
			continue
		}

		report := ObjectiveReport{
			Name:        obj.Name,
			Target:      obj.Target,
			Total:       w.total,
			Successes:   w.successes,
			Failures:    w.failures,
			WindowStart: w.startTime,
		}

		if w.total > 0 {
			report.Availability = float64(w.successes) / float64(w.total) * 100
			report.Compliant = report.Availability >= obj.Target
		}

		// Error budget calculation.
		report.ErrorBudget = m.computeErrorBudget(obj, w)

		// Latency percentiles.
		if len(w.latencies) > 0 {
			report.P50Latency = percentile(w.latencies, 0.50)
			report.P95Latency = percentile(w.latencies, 0.95)
			report.P99Latency = percentile(w.latencies, 0.99)

			if obj.LatencyThreshold > 0 {
				report.LatencyCompliant = report.P99Latency <= obj.LatencyThreshold
			}
		}

		reports = append(reports, report)
	}

	return reports
}

func (m *Monitor) computeErrorBudget(obj Objective, w *window) ErrorBudget {
	if w.total == 0 {
		return ErrorBudget{Remaining: 100}
	}

	// Error budget = (1 - target/100) * total
	// e.g., 99.9% SLA with 10000 requests: budget = 10 errors allowed.
	targetFraction := obj.Target / 100
	budgetTotal := float64(w.total) * (1 - targetFraction)
	budgetUsed := float64(w.failures)
	budgetRemaining := budgetTotal - budgetUsed

	remaining := 100.0
	if budgetTotal > 0 {
		remaining = (budgetRemaining / budgetTotal) * 100
	}

	return ErrorBudget{
		Total:           int64(math.Ceil(budgetTotal)),
		Used:            w.failures,
		Remaining:       remaining,
		BurnRate:        m.computeBurnRate(w),
		ExhaustedAt:     m.estimateExhaustion(budgetRemaining, w),
	}
}

func (m *Monitor) computeBurnRate(w *window) float64 {
	elapsed := time.Since(w.startTime)
	if elapsed <= 0 || w.total == 0 {
		return 0
	}

	// Failures per hour.
	hours := elapsed.Hours()
	if hours <= 0 {
		return 0
	}
	return float64(w.failures) / hours
}

func (m *Monitor) estimateExhaustion(remaining float64, w *window) *time.Time {
	if remaining <= 0 {
		now := time.Now()
		return &now // Already exhausted.
	}

	burnRate := m.computeBurnRate(w)
	if burnRate <= 0 {
		return nil // Not burning — infinite budget.
	}

	hoursLeft := remaining / burnRate
	t := time.Now().Add(time.Duration(hoursLeft * float64(time.Hour)))
	return &t
}

func percentile(data []time.Duration, p float64) time.Duration {
	n := len(data)
	if n == 0 {
		return 0
	}

	sorted := make([]time.Duration, n)
	copy(sorted, data)
	sortDurations(sorted)

	idx := int(math.Ceil(float64(n)*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

func sortDurations(d []time.Duration) {
	for i := 1; i < len(d); i++ {
		key := d[i]
		j := i - 1
		for j >= 0 && d[j] > key {
			d[j+1] = d[j]
			j--
		}
		d[j+1] = key
	}
}

// ObjectiveReport summarizes SLA compliance for one objective.
type ObjectiveReport struct {
	Name             string      `json:"name"`
	Target           float64     `json:"target"`
	Total            int64       `json:"total"`
	Successes        int64       `json:"successes"`
	Failures         int64       `json:"failures"`
	Availability     float64     `json:"availability"`
	Compliant        bool        `json:"compliant"`
	LatencyCompliant bool        `json:"latency_compliant"`
	P50Latency       time.Duration `json:"p50_latency"`
	P95Latency       time.Duration `json:"p95_latency"`
	P99Latency       time.Duration `json:"p99_latency"`
	ErrorBudget      ErrorBudget `json:"error_budget"`
	WindowStart      time.Time   `json:"window_start"`
}

// ErrorBudget tracks remaining error budget.
type ErrorBudget struct {
	Total       int64      `json:"total"`        // Total errors allowed.
	Used        int64      `json:"used"`         // Errors consumed.
	Remaining   float64    `json:"remaining"`    // Remaining percentage.
	BurnRate    float64    `json:"burn_rate"`     // Errors per hour.
	ExhaustedAt *time.Time `json:"exhausted_at"` // Estimated exhaustion time.
}

// String returns a human-readable error budget summary.
func (b ErrorBudget) String() string {
	if b.Remaining <= 0 {
		return "ERROR BUDGET EXHAUSTED"
	}
	return fmt.Sprintf("%.1f%% remaining (%d/%d errors used, %.2f/hr burn rate)",
		b.Remaining, b.Used, b.Total, b.BurnRate)
}
