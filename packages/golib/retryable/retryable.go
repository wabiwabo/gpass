// Package retryable provides a generic retry mechanism with
// exponential backoff, jitter, and configurable retry conditions.
package retryable

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Config defines retry behavior.
type Config struct {
	MaxRetries int           // Maximum retry attempts.
	Initial    time.Duration // Initial delay.
	Max        time.Duration // Maximum delay.
	Multiplier float64       // Backoff multiplier.
	Jitter     bool          // Add random jitter.
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxRetries: 3,
		Initial:    100 * time.Millisecond,
		Max:        10 * time.Second,
		Multiplier: 2.0,
		Jitter:     true,
	}
}

// Do executes fn with retry logic.
// Returns the result of the first successful call, or the last error.
func Do[T any](ctx context.Context, cfg Config, fn func(ctx context.Context) (T, error)) (T, error) {
	var lastErr error
	var zero T

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}
		lastErr = err

		if attempt == cfg.MaxRetries {
			break
		}

		delay := computeDelay(attempt, cfg)
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}

	return zero, lastErr
}

// DoSimple retries a function that returns only an error.
func DoSimple(ctx context.Context, cfg Config, fn func(ctx context.Context) error) error {
	_, err := Do(ctx, cfg, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, fn(ctx)
	})
	return err
}

func computeDelay(attempt int, cfg Config) time.Duration {
	delay := float64(cfg.Initial) * math.Pow(cfg.Multiplier, float64(attempt))
	if delay > float64(cfg.Max) {
		delay = float64(cfg.Max)
	}

	if cfg.Jitter {
		// Add ±25% jitter.
		jitter := delay * 0.25
		delay = delay - jitter + rand.Float64()*jitter*2
	}

	return time.Duration(delay)
}
