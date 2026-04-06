package resilience

import (
	"context"
	"time"
)

// Fallback executes primary function; if it fails or times out, returns fallbackValue.
func Fallback[T any](ctx context.Context, timeout time.Duration, primary func(context.Context) (T, error), fallbackValue T) (T, error) {
	return FallbackFunc(ctx, timeout, primary, func(_ error) (T, error) {
		return fallbackValue, nil
	})
}

// FallbackFunc executes primary; if it fails or times out, executes fallback function.
func FallbackFunc[T any](ctx context.Context, timeout time.Duration, primary func(context.Context) (T, error), fallback func(error) (T, error)) (T, error) {
	type result struct {
		val T
		err error
	}

	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ch := make(chan result, 1)
	go func() {
		v, err := primary(tctx)
		ch <- result{v, err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			return fallback(r.err)
		}
		return r.val, nil
	case <-tctx.Done():
		return fallback(tctx.Err())
	}
}

// Retry executes fn up to maxAttempts times with exponential backoff.
// It stops early if the context is cancelled.
func Retry[T any](ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func(context.Context) (T, error)) (T, error) {
	var lastErr error
	var zero T

	for attempt := range maxAttempts {
		val, err := fn(ctx)
		if err == nil {
			return val, nil
		}
		lastErr = err

		// Don't sleep after the last attempt.
		if attempt < maxAttempts-1 {
			delay := RetryWithBackoff(attempt, baseDelay)
			select {
			case <-ctx.Done():
				return zero, ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return zero, lastErr
}

// RetryWithBackoff returns the delay for attempt n (exponential: baseDelay * 2^n, max 30s).
func RetryWithBackoff(attempt int, baseDelay time.Duration) time.Duration {
	const maxDelay = 30 * time.Second

	delay := baseDelay
	for range attempt {
		delay *= 2
		if delay > maxDelay {
			return maxDelay
		}
	}
	if delay > maxDelay {
		return maxDelay
	}
	return delay
}
