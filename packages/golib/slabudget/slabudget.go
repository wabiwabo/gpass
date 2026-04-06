// Package slabudget provides SLA error budget tracking with
// burn rate monitoring and exhaustion estimation. Helps teams
// understand how fast they're consuming their error budget
// relative to their SLO targets.
package slabudget

import (
	"math"
	"sync"
	"time"
)

// Config defines the SLO target and monitoring window.
type Config struct {
	// Target is the SLO target (e.g., 0.999 for 99.9%).
	Target float64
	// Window is the SLO measurement window (e.g., 30 days).
	Window time.Duration
}

// Budget tracks error budget consumption.
type Budget struct {
	mu         sync.RWMutex
	cfg        Config
	total      int64
	errors     int64
	startTime  time.Time
}

// NewBudget creates an error budget tracker.
func NewBudget(cfg Config) *Budget {
	if cfg.Target <= 0 || cfg.Target >= 1 {
		cfg.Target = 0.999
	}
	if cfg.Window <= 0 {
		cfg.Window = 30 * 24 * time.Hour
	}
	return &Budget{
		cfg:       cfg,
		startTime: time.Now(),
	}
}

// RecordSuccess records a successful request.
func (b *Budget) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.total++
}

// RecordError records a failed request.
func (b *Budget) RecordError() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.total++
	b.errors++
}

// Record records a batch of requests.
func (b *Budget) Record(total, errors int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.total += total
	b.errors += errors
}

// Status returns the current error budget status.
type Status struct {
	// Target is the SLO target percentage.
	Target float64 `json:"target"`
	// Actual is the current success rate.
	Actual float64 `json:"actual"`
	// TotalRequests is the total observed requests.
	TotalRequests int64 `json:"total_requests"`
	// TotalErrors is the total observed errors.
	TotalErrors int64 `json:"total_errors"`
	// BudgetTotal is the total error budget (allowed errors).
	BudgetTotal int64 `json:"budget_total"`
	// BudgetUsed is the consumed error budget.
	BudgetUsed int64 `json:"budget_used"`
	// BudgetRemaining is the remaining error budget.
	BudgetRemaining int64 `json:"budget_remaining"`
	// BudgetRatio is the percentage of budget consumed (0-1+).
	BudgetRatio float64 `json:"budget_ratio"`
	// BurnRate is the current budget burn rate.
	// 1.0 = consuming at exactly the sustainable rate.
	// >1.0 = consuming faster than sustainable.
	BurnRate float64 `json:"burn_rate"`
	// IsExhausted is true if the error budget is fully consumed.
	IsExhausted bool `json:"is_exhausted"`
	// EstimatedExhaustion is when the budget will be exhausted at current rate.
	EstimatedExhaustion *time.Time `json:"estimated_exhaustion,omitempty"`
	// Elapsed is how much of the window has passed.
	Elapsed time.Duration `json:"elapsed"`
}

// Status returns the current error budget status.
func (b *Budget) Status() Status {
	b.mu.RLock()
	defer b.mu.RUnlock()

	elapsed := time.Since(b.startTime)
	if elapsed > b.cfg.Window {
		elapsed = b.cfg.Window
	}

	errorBudget := float64(b.total) * (1 - b.cfg.Target)
	budgetTotal := int64(math.Ceil(errorBudget))

	remaining := budgetTotal - b.errors
	if remaining < 0 {
		remaining = 0
	}

	var budgetRatio float64
	if budgetTotal > 0 {
		budgetRatio = float64(b.errors) / float64(budgetTotal)
	}

	var actual float64
	if b.total > 0 {
		actual = 1 - float64(b.errors)/float64(b.total)
	} else {
		actual = 1
	}

	// Burn rate: how fast are we consuming relative to window
	var burnRate float64
	windowRatio := elapsed.Seconds() / b.cfg.Window.Seconds()
	if windowRatio > 0 {
		burnRate = budgetRatio / windowRatio
	}

	status := Status{
		Target:          b.cfg.Target,
		Actual:          actual,
		TotalRequests:   b.total,
		TotalErrors:     b.errors,
		BudgetTotal:     budgetTotal,
		BudgetUsed:      b.errors,
		BudgetRemaining: remaining,
		BudgetRatio:     budgetRatio,
		BurnRate:        burnRate,
		IsExhausted:     b.errors >= budgetTotal && budgetTotal > 0,
		Elapsed:         elapsed,
	}

	// Estimate exhaustion
	if burnRate > 0 && !status.IsExhausted && budgetTotal > 0 {
		remainingRatio := 1 - budgetRatio
		if remainingRatio > 0 {
			remainingTime := time.Duration(remainingRatio / burnRate * float64(b.cfg.Window))
			exhaustion := time.Now().Add(remainingTime)
			status.EstimatedExhaustion = &exhaustion
		}
	}

	return status
}

// Reset clears all counters and restarts the window.
func (b *Budget) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.total = 0
	b.errors = 0
	b.startTime = time.Now()
}
