package retryable

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	result, err := Do(context.Background(), DefaultConfig(), func(ctx context.Context) (string, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Errorf("result: got %q", result)
	}
}

func TestDo_RetryThenSuccess(t *testing.T) {
	var attempts atomic.Int64
	result, err := Do(context.Background(), Config{
		MaxRetries: 3,
		Initial:    time.Millisecond,
		Max:        10 * time.Millisecond,
		Multiplier: 2.0,
	}, func(ctx context.Context) (string, error) {
		if attempts.Add(1) < 3 {
			return "", errors.New("not yet")
		}
		return "done", nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if result != "done" {
		t.Errorf("result: got %q", result)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts: got %d", attempts.Load())
	}
}

func TestDo_AllFail(t *testing.T) {
	cfg := Config{MaxRetries: 2, Initial: time.Millisecond, Max: 5 * time.Millisecond, Multiplier: 2.0}

	_, err := Do(context.Background(), cfg, func(ctx context.Context) (int, error) {
		return 0, errors.New("always fail")
	})

	if err == nil {
		t.Error("should return error after all retries")
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	cfg := Config{MaxRetries: 100, Initial: 100 * time.Millisecond, Max: time.Second, Multiplier: 2.0}

	_, err := Do(ctx, cfg, func(ctx context.Context) (int, error) {
		return 0, errors.New("fail")
	})

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("should cancel: got %v", err)
	}
}

func TestDoSimple_Success(t *testing.T) {
	err := DoSimple(context.Background(), DefaultConfig(), func(ctx context.Context) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDoSimple_RetryThenSuccess(t *testing.T) {
	var attempts atomic.Int64
	err := DoSimple(context.Background(), Config{
		MaxRetries: 3,
		Initial:    time.Millisecond,
		Max:        5 * time.Millisecond,
		Multiplier: 2.0,
	}, func(ctx context.Context) error {
		if attempts.Add(1) < 2 {
			return errors.New("fail")
		}
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("max retries: got %d", cfg.MaxRetries)
	}
	if cfg.Initial != 100*time.Millisecond {
		t.Errorf("initial: got %v", cfg.Initial)
	}
	if !cfg.Jitter {
		t.Error("jitter should be on by default")
	}
}
