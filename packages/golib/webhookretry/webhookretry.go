// Package webhookretry provides exponential backoff retry logic for
// webhook delivery. Tracks delivery attempts, computes next retry times,
// and enforces maximum retry limits.
package webhookretry

import (
	"math"
	"time"
)

// Policy defines retry behavior.
type Policy struct {
	MaxRetries    int           // Maximum number of retry attempts.
	InitialDelay  time.Duration // First retry delay.
	MaxDelay      time.Duration // Maximum delay between retries.
	Multiplier    float64       // Backoff multiplier (e.g., 2.0 for doubling).
	JitterPercent float64       // Random jitter as percentage (0.0-1.0).
}

// DefaultPolicy returns a production-suitable retry policy.
func DefaultPolicy() Policy {
	return Policy{
		MaxRetries:    10,
		InitialDelay:  1 * time.Second,
		MaxDelay:      1 * time.Hour,
		Multiplier:    2.0,
		JitterPercent: 0.1,
	}
}

// Delivery tracks the state of a webhook delivery attempt.
type Delivery struct {
	ID         string    `json:"id"`
	WebhookID  string    `json:"webhook_id"`
	EventType  string    `json:"event_type"`
	URL        string    `json:"url"`
	Attempt    int       `json:"attempt"`
	MaxRetries int       `json:"max_retries"`
	Status     Status    `json:"status"`
	LastError  string    `json:"last_error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	NextRetry  time.Time `json:"next_retry,omitempty"`
	LastAttempt time.Time `json:"last_attempt,omitempty"`
}

// Status represents delivery status.
type Status string

const (
	StatusPending   Status = "pending"
	StatusDelivered Status = "delivered"
	StatusRetrying  Status = "retrying"
	StatusFailed    Status = "failed" // Max retries exhausted.
)

// NextDelay calculates the delay for the next retry attempt
// using exponential backoff with optional jitter.
func (p Policy) NextDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return p.InitialDelay
	}

	delay := float64(p.InitialDelay) * math.Pow(p.Multiplier, float64(attempt-1))
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	return time.Duration(delay)
}

// ShouldRetry checks if another retry should be attempted.
func (p Policy) ShouldRetry(attempt int) bool {
	return attempt < p.MaxRetries
}

// RecordSuccess marks a delivery as successful.
func (d *Delivery) RecordSuccess() {
	d.Status = StatusDelivered
	d.LastAttempt = time.Now()
}

// RecordFailure marks a delivery attempt as failed and computes next retry.
func (d *Delivery) RecordFailure(err string, policy Policy) {
	d.Attempt++
	d.LastError = err
	d.LastAttempt = time.Now()

	if policy.ShouldRetry(d.Attempt) {
		d.Status = StatusRetrying
		delay := policy.NextDelay(d.Attempt)
		d.NextRetry = time.Now().Add(delay)
	} else {
		d.Status = StatusFailed
	}
}

// IsTerminal returns true if the delivery has reached a final state.
func (d *Delivery) IsTerminal() bool {
	return d.Status == StatusDelivered || d.Status == StatusFailed
}

// RetrySchedule returns the full schedule of retry delays for a policy.
func (p Policy) RetrySchedule() []time.Duration {
	schedule := make([]time.Duration, p.MaxRetries)
	for i := 0; i < p.MaxRetries; i++ {
		schedule[i] = p.NextDelay(i + 1)
	}
	return schedule
}

// TotalRetryDuration returns the total time a delivery could take
// including all retries.
func (p Policy) TotalRetryDuration() time.Duration {
	var total time.Duration
	for i := 0; i < p.MaxRetries; i++ {
		total += p.NextDelay(i + 1)
	}
	return total
}
