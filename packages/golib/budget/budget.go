package budget

import (
	"context"
	"sync"
	"time"
)

// Budget enforces time and resource budgets for API requests.
// It tracks remaining time budget across multiple downstream calls,
// preventing a request from spending unlimited time waiting.
type Budget struct {
	deadline time.Time
	spent    time.Duration
	calls    int
	maxCalls int
	mu       sync.Mutex
}

// New creates a budget with the given total time and max downstream calls.
func New(total time.Duration, maxCalls int) *Budget {
	if maxCalls <= 0 {
		maxCalls = 100
	}
	return &Budget{
		deadline: time.Now().Add(total),
		maxCalls: maxCalls,
	}
}

// FromContext creates a budget from a context's deadline.
// If the context has no deadline, uses the fallback duration.
func FromContext(ctx context.Context, fallback time.Duration, maxCalls int) *Budget {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(fallback)
	}
	if maxCalls <= 0 {
		maxCalls = 100
	}
	return &Budget{
		deadline: deadline,
		maxCalls: maxCalls,
	}
}

// Remaining returns the time remaining in the budget.
func (b *Budget) Remaining() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()
	remaining := time.Until(b.deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Exhausted returns true if the budget is expired or max calls reached.
func (b *Budget) Exhausted() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return time.Now().After(b.deadline) || b.calls >= b.maxCalls
}

// RecordCall records a downstream call and its duration.
func (b *Budget) RecordCall(duration time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	b.spent += duration
}

// Context returns a context with the budget's remaining deadline.
func (b *Budget) Context(parent context.Context) (context.Context, context.CancelFunc) {
	remaining := b.Remaining()
	if remaining <= 0 {
		ctx, cancel := context.WithCancel(parent)
		cancel() // immediately cancelled
		return ctx, cancel
	}
	return context.WithTimeout(parent, remaining)
}

// Stats returns budget statistics.
type Stats struct {
	TotalBudget time.Duration `json:"total_budget"`
	Remaining   time.Duration `json:"remaining"`
	Spent       time.Duration `json:"spent"`
	Calls       int           `json:"calls"`
	MaxCalls    int           `json:"max_calls"`
	Exhausted   bool          `json:"exhausted"`
}

// Stats returns current budget stats.
func (b *Budget) Stats() Stats {
	b.mu.Lock()
	defer b.mu.Unlock()

	remaining := time.Until(b.deadline)
	if remaining < 0 {
		remaining = 0
	}

	return Stats{
		Remaining: remaining,
		Spent:     b.spent,
		Calls:     b.calls,
		MaxCalls:  b.maxCalls,
		Exhausted: time.Now().After(b.deadline) || b.calls >= b.maxCalls,
	}
}
