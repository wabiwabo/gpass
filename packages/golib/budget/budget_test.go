package budget

import (
	"context"
	"testing"
	"time"
)

func TestBudget_Remaining(t *testing.T) {
	b := New(5*time.Second, 10)
	remaining := b.Remaining()
	if remaining < 4*time.Second || remaining > 5*time.Second {
		t.Errorf("remaining: got %v, want ~5s", remaining)
	}
}

func TestBudget_Exhausted_ByTime(t *testing.T) {
	b := New(50*time.Millisecond, 100)
	if b.Exhausted() {
		t.Error("should not be exhausted immediately")
	}

	time.Sleep(60 * time.Millisecond)
	if !b.Exhausted() {
		t.Error("should be exhausted after timeout")
	}
}

func TestBudget_Exhausted_ByCallCount(t *testing.T) {
	b := New(time.Hour, 3)
	b.RecordCall(time.Millisecond)
	b.RecordCall(time.Millisecond)
	if b.Exhausted() {
		t.Error("should not be exhausted at 2/3 calls")
	}

	b.RecordCall(time.Millisecond)
	if !b.Exhausted() {
		t.Error("should be exhausted at 3/3 calls")
	}
}

func TestBudget_RecordCall(t *testing.T) {
	b := New(time.Second, 10)
	b.RecordCall(100 * time.Millisecond)
	b.RecordCall(200 * time.Millisecond)

	stats := b.Stats()
	if stats.Calls != 2 {
		t.Errorf("calls: got %d", stats.Calls)
	}
	if stats.Spent != 300*time.Millisecond {
		t.Errorf("spent: got %v", stats.Spent)
	}
}

func TestBudget_Context(t *testing.T) {
	b := New(500*time.Millisecond, 10)
	ctx, cancel := b.Context(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context should have deadline")
	}
	if time.Until(deadline) > 500*time.Millisecond {
		t.Error("context deadline should be within budget")
	}
}

func TestBudget_Context_Exhausted(t *testing.T) {
	b := New(10*time.Millisecond, 10)
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := b.Context(context.Background())
	defer cancel()

	if ctx.Err() == nil {
		t.Error("exhausted budget should return cancelled context")
	}
}

func TestBudget_FromContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	b := FromContext(ctx, 10*time.Second, 5)
	remaining := b.Remaining()
	if remaining > 2*time.Second {
		t.Errorf("should inherit context deadline, remaining: %v", remaining)
	}
}

func TestBudget_FromContext_NoDeadline(t *testing.T) {
	b := FromContext(context.Background(), 3*time.Second, 5)
	remaining := b.Remaining()
	if remaining < 2*time.Second || remaining > 3*time.Second {
		t.Errorf("should use fallback duration, remaining: %v", remaining)
	}
}

func TestBudget_Stats(t *testing.T) {
	b := New(time.Second, 5)
	b.RecordCall(50 * time.Millisecond)

	stats := b.Stats()
	if stats.Calls != 1 {
		t.Errorf("calls: got %d", stats.Calls)
	}
	if stats.MaxCalls != 5 {
		t.Errorf("maxCalls: got %d", stats.MaxCalls)
	}
	if stats.Exhausted {
		t.Error("should not be exhausted")
	}
}

func TestBudget_DefaultMaxCalls(t *testing.T) {
	b := New(time.Second, 0)
	stats := b.Stats()
	if stats.MaxCalls != 100 {
		t.Errorf("default maxCalls: got %d, want 100", stats.MaxCalls)
	}
}
