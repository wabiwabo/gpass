package bulkhead

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBulkhead_AllowsConcurrentUpToLimit(t *testing.T) {
	b := New(Config{Name: "test", MaxConcurrent: 3})

	var active atomic.Int32
	var maxSeen atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Execute(context.Background(), func(_ context.Context) error {
				cur := active.Add(1)
				// Track max active.
				for {
					old := maxSeen.Load()
					if cur <= old || maxSeen.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				active.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if maxSeen.Load() > 3 {
		t.Errorf("max concurrent exceeded limit: got %d, want <= 3", maxSeen.Load())
	}
}

func TestBulkhead_RejectsWhenFull_NoQueue(t *testing.T) {
	b := New(Config{Name: "reject", MaxConcurrent: 1})

	started := make(chan struct{})
	done := make(chan struct{})

	// Fill the single permit.
	go func() {
		b.Execute(context.Background(), func(_ context.Context) error {
			close(started)
			<-done
			return nil
		})
	}()
	<-started

	// Second call should be rejected immediately.
	err := b.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})
	close(done)

	if !errors.Is(err, ErrBulkheadFull) {
		t.Errorf("expected ErrBulkheadFull, got: %v", err)
	}

	m := b.Metrics()
	if m.TotalRejected != 1 {
		t.Errorf("expected 1 rejected, got %d", m.TotalRejected)
	}
}

func TestBulkhead_QueueWaitsForPermit(t *testing.T) {
	b := New(Config{
		Name:          "queue",
		MaxConcurrent: 1,
		MaxQueue:      5,
		QueueTimeout:  2 * time.Second,
	})

	started := make(chan struct{})
	release := make(chan struct{})

	// Fill the permit.
	go func() {
		b.Execute(context.Background(), func(_ context.Context) error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started

	// Second call should queue and wait.
	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Execute(context.Background(), func(_ context.Context) error {
			return nil
		})
	}()

	// Release the first after a short delay.
	time.Sleep(50 * time.Millisecond)
	close(release)

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("queued request should succeed, got: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("queued request timed out")
	}
}

func TestBulkhead_QueueTimeout(t *testing.T) {
	b := New(Config{
		Name:          "timeout",
		MaxConcurrent: 1,
		MaxQueue:      5,
		QueueTimeout:  50 * time.Millisecond,
	})

	started := make(chan struct{})
	done := make(chan struct{})

	// Fill the permit and hold it.
	go func() {
		b.Execute(context.Background(), func(_ context.Context) error {
			close(started)
			<-done
			return nil
		})
	}()
	<-started

	err := b.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})
	close(done)

	if !errors.Is(err, ErrBulkheadTimeout) {
		t.Errorf("expected ErrBulkheadTimeout, got: %v", err)
	}
}

func TestBulkhead_QueueFull(t *testing.T) {
	b := New(Config{
		Name:          "qfull",
		MaxConcurrent: 1,
		MaxQueue:      1,
		QueueTimeout:  time.Second,
	})

	started := make(chan struct{})
	done := make(chan struct{})

	// Fill the permit.
	go func() {
		b.Execute(context.Background(), func(_ context.Context) error {
			close(started)
			<-done
			return nil
		})
	}()
	<-started

	// Fill the queue.
	queued := make(chan struct{})
	go func() {
		close(queued)
		b.Execute(context.Background(), func(_ context.Context) error {
			return nil
		})
	}()
	<-queued
	time.Sleep(20 * time.Millisecond) // let it enter queue

	// Third call: both permit and queue full.
	err := b.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})
	close(done)

	if !errors.Is(err, ErrBulkheadFull) {
		t.Errorf("expected ErrBulkheadFull, got: %v", err)
	}
}

func TestBulkhead_ContextCancellation(t *testing.T) {
	b := New(Config{
		Name:          "cancel",
		MaxConcurrent: 1,
		MaxQueue:      5,
		QueueTimeout:  5 * time.Second,
	})

	started := make(chan struct{})
	done := make(chan struct{})

	// Fill the permit.
	go func() {
		b.Execute(context.Background(), func(_ context.Context) error {
			close(started)
			<-done
			return nil
		})
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- b.Execute(ctx, func(_ context.Context) error {
			return nil
		})
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("context cancellation not propagated")
	}
	close(done)
}

func TestBulkhead_Metrics(t *testing.T) {
	b := New(Config{Name: "metrics", MaxConcurrent: 2})

	// Successful call.
	b.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})

	// Failed call.
	b.Execute(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	m := b.Metrics()
	if m.TotalAccepted != 2 {
		t.Errorf("accepted: got %d, want 2", m.TotalAccepted)
	}
	if m.TotalCompleted != 1 {
		t.Errorf("completed: got %d, want 1", m.TotalCompleted)
	}
	if m.TotalFailed != 1 {
		t.Errorf("failed: got %d, want 1", m.TotalFailed)
	}
	if m.ActiveCount != 0 {
		t.Errorf("active: got %d, want 0", m.ActiveCount)
	}
	if m.MaxConcurrent != 2 {
		t.Errorf("maxConcurrent: got %d, want 2", m.MaxConcurrent)
	}
}

func TestBulkhead_ExecuteWithResult(t *testing.T) {
	b := New(Config{Name: "result", MaxConcurrent: 5})

	result, err := ExecuteWithResult(b, context.Background(), func(_ context.Context) (string, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello" {
		t.Errorf("got %q, want %q", result, "hello")
	}
}

func TestBulkhead_ExecuteWithResult_Error(t *testing.T) {
	b := New(Config{Name: "err", MaxConcurrent: 5})

	_, err := ExecuteWithResult(b, context.Background(), func(_ context.Context) (int, error) {
		return 0, errors.New("fail")
	})
	if err == nil || err.Error() != "fail" {
		t.Errorf("expected error 'fail', got: %v", err)
	}
}

func TestBulkhead_ExecuteWithResult_BulkheadFull(t *testing.T) {
	b := New(Config{Name: "full-result", MaxConcurrent: 1})

	started := make(chan struct{})
	done := make(chan struct{})

	go func() {
		b.Execute(context.Background(), func(_ context.Context) error {
			close(started)
			<-done
			return nil
		})
	}()
	<-started

	_, err := ExecuteWithResult(b, context.Background(), func(_ context.Context) (string, error) {
		return "x", nil
	})
	close(done)

	if !errors.Is(err, ErrBulkheadFull) {
		t.Errorf("expected ErrBulkheadFull, got: %v", err)
	}
}

func TestBulkhead_DefaultConfig(t *testing.T) {
	b := New(Config{Name: "defaults"})
	if b.maxConc != 10 {
		t.Errorf("default maxConcurrent: got %d, want 10", b.maxConc)
	}
	if b.queueTimeout != time.Second {
		t.Errorf("default queueTimeout: got %v, want 1s", b.queueTimeout)
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()

	b1 := New(Config{Name: "dukcapil", MaxConcurrent: 5})
	b2 := New(Config{Name: "ahu", MaxConcurrent: 10})

	reg.Register(b1)
	reg.Register(b2)

	got := reg.Get("dukcapil")
	if got != b1 {
		t.Error("expected dukcapil bulkhead")
	}

	got = reg.Get("ahu")
	if got != b2 {
		t.Error("expected ahu bulkhead")
	}

	got = reg.Get("nonexistent")
	if got != nil {
		t.Error("expected nil for nonexistent")
	}
}

func TestRegistry_All(t *testing.T) {
	reg := NewRegistry()
	reg.Register(New(Config{Name: "a", MaxConcurrent: 5}))
	reg.Register(New(Config{Name: "b", MaxConcurrent: 10}))

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 bulkheads, got %d", len(all))
	}
	if all["a"].MaxConcurrent != 5 {
		t.Errorf("a maxConcurrent: got %d, want 5", all["a"].MaxConcurrent)
	}
	if all["b"].MaxConcurrent != 10 {
		t.Errorf("b maxConcurrent: got %d, want 10", all["b"].MaxConcurrent)
	}
}

func TestBulkhead_ConcurrentExecutions_RaceDetector(t *testing.T) {
	b := New(Config{Name: "race", MaxConcurrent: 5, MaxQueue: 10, QueueTimeout: time.Second})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Execute(context.Background(), func(_ context.Context) error {
				time.Sleep(10 * time.Millisecond)
				return nil
			})
		}()
	}
	wg.Wait()

	m := b.Metrics()
	total := m.TotalAccepted + m.TotalRejected
	if total != 20 {
		t.Errorf("expected 20 total requests, got %d (accepted=%d, rejected=%d)",
			total, m.TotalAccepted, m.TotalRejected)
	}
}
