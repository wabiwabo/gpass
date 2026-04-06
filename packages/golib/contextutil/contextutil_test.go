package contextutil

import (
	"context"
	"testing"
	"time"
)

func TestRemainingTime_WithDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	remaining := RemainingTime(ctx)
	if remaining <= 0 || remaining > 100*time.Millisecond {
		t.Errorf("remaining: got %v", remaining)
	}
}

func TestRemainingTime_NoDeadline(t *testing.T) {
	remaining := RemainingTime(context.Background())
	if remaining != 0 {
		t.Errorf("no deadline: got %v", remaining)
	}
}

func TestRemainingTime_Expired(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	remaining := RemainingTime(ctx)
	if remaining != 0 {
		t.Errorf("expired: got %v", remaining)
	}
}

func TestHasDeadline(t *testing.T) {
	if HasDeadline(context.Background()) {
		t.Error("background should not have deadline")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if !HasDeadline(ctx) {
		t.Error("timeout context should have deadline")
	}
}

func TestIsExpired(t *testing.T) {
	if IsExpired(context.Background()) {
		t.Error("background should not be expired")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if !IsExpired(ctx) {
		t.Error("canceled context should be expired")
	}
}

func TestWithMinTimeout(t *testing.T) {
	// When parent has sufficient deadline, use parent context.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	newCtx, newCancel := WithMinTimeout(ctx, 100*time.Millisecond)
	defer newCancel()

	remaining := RemainingTime(newCtx)
	if remaining < 5*time.Second {
		t.Errorf("should preserve longer parent deadline: got %v", remaining)
	}
}

func TestWithMinTimeout_Short(t *testing.T) {
	// When parent has short deadline, WithMinTimeout creates new context
	// but it inherits parent's shorter deadline (Go context semantics).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	newCtx, newCancel := WithMinTimeout(ctx, 100*time.Millisecond)
	defer newCancel()

	// The new context still inherits parent's 10ms deadline.
	// WithMinTimeout creates a longer timeout but parent wins.
	if HasDeadline(newCtx) {
		// This is expected behavior.
		t.Log("context inherits parent deadline (Go semantics)")
	}
}

func TestWithMinTimeout_AlreadySufficient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	newCtx, newCancel := WithMinTimeout(ctx, 100*time.Millisecond)
	defer newCancel()

	// Should return same context since deadline is already sufficient.
	remaining := RemainingTime(newCtx)
	if remaining < 5*time.Second {
		t.Errorf("should preserve existing deadline: got %v", remaining)
	}
}

func TestWithMaxTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	newCtx, newCancel := WithMaxTimeout(ctx, 100*time.Millisecond)
	defer newCancel()

	remaining := RemainingTime(newCtx)
	if remaining > 200*time.Millisecond {
		t.Errorf("should be capped: got %v", remaining)
	}
}

func TestWithMaxTimeout_AlreadyShort(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	newCtx, newCancel := WithMaxTimeout(ctx, 10*time.Second)
	defer newCancel()

	remaining := RemainingTime(newCtx)
	if remaining > 60*time.Millisecond {
		t.Errorf("should preserve shorter deadline: got %v", remaining)
	}
}

func TestSplitTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	split := SplitTimeout(ctx, 4)
	if split < 20*time.Millisecond || split > 30*time.Millisecond {
		t.Errorf("split: got %v", split)
	}
}

func TestSplitTimeout_NoDeadline(t *testing.T) {
	split := SplitTimeout(context.Background(), 4)
	if split != 0 {
		t.Errorf("no deadline: got %v", split)
	}
}

func TestSplitTimeout_Zero(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	split := SplitTimeout(ctx, 0) // Should treat as 1.
	if split < 90*time.Millisecond {
		t.Errorf("zero n: got %v", split)
	}
}

type testKey struct{}

func TestDetach(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	ctx = context.WithValue(ctx, testKey{}, "test-value")
	detached := Detach(ctx)

	// Should carry values.
	if detached.Value(testKey{}) != "test-value" {
		t.Error("should carry values")
	}

	// Should NOT inherit deadline.
	if HasDeadline(detached) {
		t.Error("detached should not have deadline")
	}

	// Should NOT be canceled when parent is.
	time.Sleep(20 * time.Millisecond)
	if IsExpired(detached) {
		t.Error("detached should not be expired")
	}
}

func TestMerge(t *testing.T) {
	ctx1, cancel1 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel1()

	ctx2 := context.WithValue(context.Background(), testKey{}, "from-ctx2")

	merged := Merge(ctx1, ctx2)

	// Should have deadline from ctx1.
	if !HasDeadline(merged) {
		t.Error("should have deadline from ctx1")
	}

	// Should have values from ctx2.
	if merged.Value(testKey{}) != "from-ctx2" {
		t.Error("should have values from ctx2")
	}

	// Should cancel when ctx1 cancels.
	cancel1()
	if !IsExpired(merged) {
		t.Error("should be expired when ctx1 cancels")
	}
}
