package signctx

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithSignals_CancelFunc(t *testing.T) {
	ctx, cancel := WithSignals(context.Background())
	defer cancel()

	// Context should not be done yet.
	select {
	case <-ctx.Done():
		t.Error("context should not be done yet")
	default:
	}

	cancel()

	select {
	case <-ctx.Done():
		// Good.
	case <-time.After(100 * time.Millisecond):
		t.Error("context should be done after cancel")
	}
}

func TestWithTimeout_Timeout(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	select {
	case <-ctx.Done():
		// Good — timed out.
	case <-time.After(200 * time.Millisecond):
		t.Error("context should timeout")
	}
}

func TestWithTimeout_Cancel(t *testing.T) {
	ctx, cancel := WithTimeout(context.Background(), 10*time.Second)

	cancel()

	select {
	case <-ctx.Done():
		// Good — manually canceled.
	case <-time.After(100 * time.Millisecond):
		t.Error("context should be done after cancel")
	}
}

func TestGracefulShutdown_ManualTrigger(t *testing.T) {
	gs := NewGracefulShutdown(1 * time.Second)

	var hookCalled atomic.Bool
	gs.OnShutdown(func(ctx context.Context) {
		hookCalled.Store(true)
	})

	// Trigger shutdown manually.
	go func() {
		time.Sleep(10 * time.Millisecond)
		gs.Trigger()
	}()

	gs.Wait()

	if !hookCalled.Load() {
		t.Error("shutdown hook should have been called")
	}
}

func TestGracefulShutdown_MultipleHooks(t *testing.T) {
	gs := NewGracefulShutdown(1 * time.Second)

	var count atomic.Int64
	gs.OnShutdown(func(ctx context.Context) { count.Add(1) })
	gs.OnShutdown(func(ctx context.Context) { count.Add(1) })
	gs.OnShutdown(func(ctx context.Context) { count.Add(1) })

	go func() {
		time.Sleep(10 * time.Millisecond)
		gs.Trigger()
	}()

	gs.Wait()

	if count.Load() != 3 {
		t.Errorf("hooks: got %d", count.Load())
	}
}

func TestGracefulShutdown_Done(t *testing.T) {
	gs := NewGracefulShutdown(1 * time.Second)

	select {
	case <-gs.Done():
		t.Error("should not be done yet")
	default:
	}

	gs.Trigger()

	select {
	case <-gs.Done():
		// Good.
	case <-time.After(100 * time.Millisecond):
		t.Error("should be done after trigger")
	}
}

func TestGracefulShutdown_Context(t *testing.T) {
	gs := NewGracefulShutdown(1 * time.Second)
	ctx := gs.Context()

	if ctx == nil {
		t.Error("context should not be nil")
	}

	gs.Trigger()

	select {
	case <-ctx.Done():
		// Good.
	case <-time.After(100 * time.Millisecond):
		t.Error("work context should be canceled")
	}
}

func TestGracefulShutdown_HookTimeout(t *testing.T) {
	gs := NewGracefulShutdown(50 * time.Millisecond)

	var hookFinished atomic.Bool
	gs.OnShutdown(func(ctx context.Context) {
		select {
		case <-ctx.Done():
			// Timeout hit.
		case <-time.After(5 * time.Second):
			hookFinished.Store(true)
		}
	})

	gs.Trigger()

	start := time.Now()
	gs.Wait()
	elapsed := time.Since(start)

	// Should complete within ~100ms (50ms timeout + overhead), not 5s.
	if elapsed > 500*time.Millisecond {
		t.Errorf("should timeout: took %v", elapsed)
	}
}
