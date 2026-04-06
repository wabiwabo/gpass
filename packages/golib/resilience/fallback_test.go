package resilience

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestFallback_PrimarySucceeds(t *testing.T) {
	result, err := Fallback(context.Background(), time.Second,
		func(_ context.Context) (string, error) {
			return "primary", nil
		},
		"fallback",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "primary" {
		t.Fatalf("expected 'primary', got %q", result)
	}
}

func TestFallback_PrimaryFails_ReturnsFallbackValue(t *testing.T) {
	result, err := Fallback(context.Background(), time.Second,
		func(_ context.Context) (string, error) {
			return "", errors.New("primary failed")
		},
		"fallback",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "fallback" {
		t.Fatalf("expected 'fallback', got %q", result)
	}
}

func TestFallback_PrimaryTimesOut_ReturnsFallbackValue(t *testing.T) {
	result, err := Fallback(context.Background(), 50*time.Millisecond,
		func(ctx context.Context) (string, error) {
			select {
			case <-time.After(5 * time.Second):
				return "slow", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		},
		"timeout-fallback",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "timeout-fallback" {
		t.Fatalf("expected 'timeout-fallback', got %q", result)
	}
}

func TestFallbackFunc_PrimaryFails_FallbackCalledWithError(t *testing.T) {
	primaryErr := errors.New("specific error")

	result, err := FallbackFunc(context.Background(), time.Second,
		func(_ context.Context) (string, error) {
			return "", primaryErr
		},
		func(err error) (string, error) {
			if err != primaryErr {
				t.Fatalf("fallback received wrong error: %v", err)
			}
			return "recovered from: " + err.Error(), nil
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "recovered from: specific error" {
		t.Fatalf("unexpected result: %q", result)
	}
}

func TestFallbackFunc_FallbackCanAlsoFail(t *testing.T) {
	_, err := FallbackFunc(context.Background(), time.Second,
		func(_ context.Context) (string, error) {
			return "", errors.New("primary failed")
		},
		func(_ error) (string, error) {
			return "", errors.New("fallback also failed")
		},
	)

	if err == nil || err.Error() != "fallback also failed" {
		t.Fatalf("expected fallback error, got: %v", err)
	}
}

func TestRetry_SucceedsFirstAttempt(t *testing.T) {
	var attempts atomic.Int32

	result, err := Retry(context.Background(), 3, time.Millisecond,
		func(_ context.Context) (string, error) {
			attempts.Add(1)
			return "ok", nil
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("expected 'ok', got %q", result)
	}
	if attempts.Load() != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts.Load())
	}
}

func TestRetry_SucceedsOnThirdAttempt(t *testing.T) {
	var attempts atomic.Int32

	result, err := Retry(context.Background(), 5, time.Millisecond,
		func(_ context.Context) (string, error) {
			n := attempts.Add(1)
			if n < 3 {
				return "", errors.New("not yet")
			}
			return "finally", nil
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "finally" {
		t.Fatalf("expected 'finally', got %q", result)
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestRetry_AllAttemptsFail_ReturnsLastError(t *testing.T) {
	var attempts atomic.Int32

	_, err := Retry(context.Background(), 3, time.Millisecond,
		func(_ context.Context) (string, error) {
			n := attempts.Add(1)
			return "", errors.New("fail-" + string(rune('0'+n)))
		},
	)

	if err == nil {
		t.Fatal("expected error after all attempts failed")
	}
	if attempts.Load() != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts.Load())
	}
}

func TestRetry_ContextCancelled_StopsRetrying(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var attempts atomic.Int32

	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	_, err := Retry(ctx, 100, 20*time.Millisecond,
		func(_ context.Context) (string, error) {
			attempts.Add(1)
			return "", errors.New("keep failing")
		},
	)

	if err == nil {
		t.Fatal("expected error")
	}
	// Should have stopped well before 100 attempts.
	if attempts.Load() >= 10 {
		t.Fatalf("expected retry to stop early, got %d attempts", attempts.Load())
	}
}

func TestRetryWithBackoff_ExponentialDelays(t *testing.T) {
	base := 100 * time.Millisecond

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1600 * time.Millisecond},
	}

	for _, tt := range tests {
		got := RetryWithBackoff(tt.attempt, base)
		if got != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, got)
		}
	}
}

func TestRetryWithBackoff_CappedAt30s(t *testing.T) {
	got := RetryWithBackoff(100, time.Second)
	if got != 30*time.Second {
		t.Fatalf("expected cap at 30s, got %v", got)
	}
}
