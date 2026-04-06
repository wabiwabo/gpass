// Package retrypolicy provides configurable retry policies with
// exponential backoff, jitter, and circuit-aware retry decisions.
// Separates retry policy logic from execution.
package retrypolicy

import (
	"math"
	"math/rand"
	"time"
)

// Policy defines retry behavior.
type Policy struct {
	MaxRetries   int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	JitterFactor float64 // 0-1, fraction of delay to jitter
}

// Default returns a production-ready retry policy.
func Default() Policy {
	return Policy{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.1,
	}
}

// NoRetry returns a policy that never retries.
func NoRetry() Policy {
	return Policy{MaxRetries: 0}
}

// ShouldRetry returns true if another attempt should be made.
func (p Policy) ShouldRetry(attempt int) bool {
	return attempt < p.MaxRetries
}

// Delay returns the delay before the next retry attempt.
func (p Policy) Delay(attempt int) time.Duration {
	if attempt <= 0 {
		return p.InitialDelay
	}

	delay := float64(p.InitialDelay) * math.Pow(p.Multiplier, float64(attempt))

	maxDelay := float64(p.MaxDelay)
	if delay > maxDelay {
		delay = maxDelay
	}

	if p.JitterFactor > 0 {
		jitter := delay * p.JitterFactor
		delay = delay - jitter + (rand.Float64() * 2 * jitter)
	}

	return time.Duration(delay)
}

// TotalMaxDuration returns the maximum total time all retries could take.
func (p Policy) TotalMaxDuration() time.Duration {
	var total time.Duration
	for i := 0; i < p.MaxRetries; i++ {
		d := float64(p.InitialDelay) * math.Pow(p.Multiplier, float64(i))
		maxD := float64(p.MaxDelay)
		if d > maxD {
			d = maxD
		}
		// Include max jitter
		d += d * p.JitterFactor
		total += time.Duration(d)
	}
	return total
}

// Classifier determines if an error is retryable.
type Classifier func(err error) bool

// AlwaysRetry classifies all errors as retryable.
func AlwaysRetry(_ error) bool {
	return true
}

// NeverRetry classifies all errors as non-retryable.
func NeverRetry(_ error) bool {
	return false
}
