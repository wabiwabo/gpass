package worker

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// TestNew_CoercesBadInputs pins the workers<1 and bufferSize<0 guards
// in New (was 60% covered).
func TestNew_CoercesBadInputs(t *testing.T) {
	noop := func(_ context.Context, _ Task) error { return nil }
	p := New(0, -5, noop)
	if p.workers != 1 {
		t.Errorf("workers = %d, want 1 (coerced)", p.workers)
	}
	if cap(p.taskCh) != 0 {
		t.Errorf("taskCh cap = %d, want 0 (coerced)", cap(p.taskCh))
	}

	p2 := New(-3, 5, noop)
	if p2.workers != 1 {
		t.Errorf("negative workers = %d, want 1", p2.workers)
	}
}

// TestStart_DoubleStartIsNoOp pins the running.Load early-return guard
// in Start. Calling Start twice must NOT spawn extra worker goroutines.
func TestStart_DoubleStartIsNoOp(t *testing.T) {
	var calls atomic.Int64
	h := func(_ context.Context, _ Task) error {
		calls.Add(1)
		return nil
	}
	p := New(2, 4, h)
	p.Start(context.Background())
	p.Start(context.Background()) // must be no-op
	defer p.Stop()

	for i := 0; i < 4; i++ {
		p.Submit(Task{ID: "t"})
	}
	time.Sleep(50 * time.Millisecond)
	if calls.Load() != 4 {
		t.Errorf("processed = %d, want 4", calls.Load())
	}
	if !p.Stats().Running {
		t.Error("Stats().Running should be true")
	}
}

// TestStop_DoubleStopIsNoOp pins the CompareAndSwap guard in Stop.
func TestStop_DoubleStopIsNoOp(t *testing.T) {
	p := New(1, 1, func(_ context.Context, _ Task) error { return nil })
	p.Start(context.Background())
	p.Stop()
	p.Stop() // must not panic on double-close
	if p.Stats().Running {
		t.Error("after Stop, Running should be false")
	}
}

// TestSubmit_RejectsAfterStop pins the !running.Load branch in Submit.
func TestSubmit_RejectsAfterStop(t *testing.T) {
	p := New(1, 1, func(_ context.Context, _ Task) error { return nil })
	p.Start(context.Background())
	p.Stop()
	if p.Submit(Task{ID: "x"}) {
		t.Error("Submit after Stop should return false")
	}
}

// TestSubmit_RejectsAfterBufferFull pins the buffer-full default branch.
func TestSubmit_RejectsAfterBufferFull(t *testing.T) {
	block := make(chan struct{})
	h := func(_ context.Context, _ Task) error {
		<-block
		return nil
	}
	p := New(1, 1, h)
	p.Start(context.Background())
	defer p.Stop()
	defer close(block) // deferred AFTER Stop registration → runs FIRST: unblock workers so Stop can drain

	// First fills the worker, second fills the buffer, third must be rejected.
	p.Submit(Task{ID: "1"})
	time.Sleep(20 * time.Millisecond) // let worker pick up
	p.Submit(Task{ID: "2"})
	if p.Submit(Task{ID: "3"}) {
		t.Error("Submit on full buffer should return false")
	}
}

// TestWorker_FailedTaskIncrementsFailed pins the err != nil branch in
// the worker loop.
func TestWorker_FailedTaskIncrementsFailed(t *testing.T) {
	h := func(_ context.Context, _ Task) error { return errors.New("boom") }
	p := New(1, 4, h)
	p.Start(context.Background())
	defer p.Stop()
	for i := 0; i < 3; i++ {
		p.Submit(Task{ID: "x"})
	}
	time.Sleep(50 * time.Millisecond)
	s := p.Stats()
	if s.Failed != 3 {
		t.Errorf("Failed = %d, want 3", s.Failed)
	}
}

// TestWorker_ContextCancelExits pins the ctx.Done branch in worker.
func TestWorker_ContextCancelExits(t *testing.T) {
	p := New(2, 1, func(ctx context.Context, _ Task) error {
		<-ctx.Done()
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)
	p.Submit(Task{ID: "blocking"})
	cancel()
	// Without ctx.Done branch, p.Stop() would deadlock waiting on wg.
	done := make(chan struct{})
	go func() { p.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return after ctx cancel")
	}
}
