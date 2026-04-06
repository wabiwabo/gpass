package circuitbreaker

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrCircuitOpen is returned when the circuit breaker is open.
	ErrCircuitOpen = errors.New("circuit breaker is open")
)

// Config holds advanced circuit breaker configuration.
type Config struct {
	// Name identifies this circuit breaker.
	Name string
	// Threshold is consecutive failures before opening.
	Threshold int
	// Cooldown is how long to wait before probing.
	Cooldown time.Duration
	// HalfOpenMaxProbes limits concurrent probe requests in half-open state.
	HalfOpenMaxProbes int
	// SuccessThreshold is successful probes needed to fully close.
	SuccessThreshold int
	// OnStateChange is called when the circuit breaker changes state.
	OnStateChange func(name, from, to string)
}

// AdvancedBreaker extends Breaker with Execute support and configurable probing.
type AdvancedBreaker struct {
	Breaker
	name              string
	halfOpenSuccess   int
	halfOpenMax       int
	successThreshold  int
	onStateChange     func(string, string, string)
}

// NewAdvanced creates an advanced circuit breaker with full configuration.
func NewAdvanced(cfg Config) *AdvancedBreaker {
	if cfg.Threshold <= 0 {
		cfg.Threshold = 5
	}
	if cfg.Cooldown <= 0 {
		cfg.Cooldown = 30 * time.Second
	}
	if cfg.HalfOpenMaxProbes <= 0 {
		cfg.HalfOpenMaxProbes = 1
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = 2
	}

	return &AdvancedBreaker{
		Breaker:          *New(cfg.Threshold, cfg.Cooldown),
		name:             cfg.Name,
		halfOpenMax:      cfg.HalfOpenMaxProbes,
		successThreshold: cfg.SuccessThreshold,
		onStateChange:    cfg.OnStateChange,
	}
}

// Execute runs fn through the circuit breaker.
// Returns ErrCircuitOpen if the circuit is open.
// Automatically records success/failure based on fn's return value.
func (b *AdvancedBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if !b.Allow() {
		return ErrCircuitOpen
	}

	err := fn(ctx)
	if err != nil {
		b.RecordAdvancedFailure()
		return err
	}

	b.RecordAdvancedSuccess()
	return nil
}

// ExecuteWithResult runs fn and returns a result through the circuit breaker.
func ExecuteWithResult[T any](b *AdvancedBreaker, ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if !b.Allow() {
		return zero, ErrCircuitOpen
	}

	result, err := fn(ctx)
	if err != nil {
		b.RecordAdvancedFailure()
		return zero, err
	}

	b.RecordAdvancedSuccess()
	return result, nil
}

// RecordAdvancedSuccess records success with half-open awareness.
func (b *AdvancedBreaker) RecordAdvancedSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	prevState := b.state
	if b.state == StateHalfOpen {
		b.halfOpenSuccess++
		if b.halfOpenSuccess >= b.successThreshold {
			b.state = StateClosed
			b.failures = 0
			b.halfOpenSuccess = 0
		}
	} else {
		b.failures = 0
		b.state = StateClosed
	}

	if b.onStateChange != nil && prevState != b.state {
		b.onStateChange(b.name, prevState, b.state)
	}
}

// RecordAdvancedFailure records failure with state change notification.
func (b *AdvancedBreaker) RecordAdvancedFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	prevState := b.state

	if b.state == StateHalfOpen {
		// Any failure in half-open goes straight back to open.
		b.state = StateOpen
		b.openedAt = time.Now()
		b.halfOpenSuccess = 0
	} else {
		b.failures++
		if b.failures >= b.threshold {
			b.state = StateOpen
			b.openedAt = time.Now()
		}
	}

	if b.onStateChange != nil && prevState != b.state {
		b.onStateChange(b.name, prevState, b.state)
	}
}

// Name returns the breaker's name.
func (b *AdvancedBreaker) Name() string { return b.name }
