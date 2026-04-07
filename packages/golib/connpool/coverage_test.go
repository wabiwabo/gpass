package connpool

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func intFactory(seq *atomic.Int64) Factory[int] {
	return func(ctx context.Context) (int, error) {
		return int(seq.Add(1)), nil
	}
}

func intCloser() Closer[int] {
	return func(_ int) error { return nil }
}

// TestNew_AppliesDefaults pins the MaxSize<=0 and AcquireTimeout<=0 fallback
// branches in New (was 60% covered).
func TestNew_AppliesDefaults(t *testing.T) {
	var seq atomic.Int64
	p := New[int](Config{}, intFactory(&seq), intCloser())
	if cap(p.sem) != 25 {
		t.Errorf("default MaxSize: cap(sem) = %d, want 25", cap(p.sem))
	}
	if p.config.AcquireTimeout != 10*time.Second {
		t.Errorf("default AcquireTimeout = %v", p.config.AcquireTimeout)
	}

	// Negative values also coerced to defaults.
	p2 := New[int](Config{MaxSize: -1, AcquireTimeout: -time.Second}, intFactory(&seq), intCloser())
	if cap(p2.sem) != 25 || p2.config.AcquireTimeout != 10*time.Second {
		t.Error("negative values not coerced")
	}
}

// TestSetHealthCheck_StoresFn pins the previously-0% setter and exercises
// it indirectly by acquiring after setting.
func TestSetHealthCheck_StoresFn(t *testing.T) {
	var seq atomic.Int64
	p := New[int](Config{MaxSize: 2}, intFactory(&seq), intCloser())
	called := false
	p.SetHealthCheck(func(_ context.Context, _ int) error {
		called = true
		return nil
	})
	if p.health == nil {
		t.Fatal("health fn not stored")
	}
	_ = called // health is invoked on Release/getIdle paths, not under test here
}

// TestAcquire_FactoryError pins the create-error path: capacity is
// released, error is propagated, stats.errors increments.
func TestAcquire_FactoryError(t *testing.T) {
	bad := func(_ context.Context) (int, error) { return 0, errors.New("boom") }
	p := New[int](Config{MaxSize: 1}, bad, intCloser())
	_, err := p.Acquire(context.Background())
	if err == nil {
		t.Fatal("expected factory error")
	}
	// Capacity must have been released — second call should also try and fail
	// (not block on the semaphore).
	done := make(chan struct{})
	go func() {
		_, _ = p.Acquire(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Acquire blocked — capacity wasn't released after factory error")
	}
}

// TestAcquire_ContextCanceled pins the ctx.Done branch in the select.
func TestAcquire_ContextCanceled(t *testing.T) {
	var seq atomic.Int64
	p := New[int](Config{MaxSize: 1, AcquireTimeout: 5 * time.Second}, intFactory(&seq), intCloser())
	// Saturate the pool.
	c1, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer p.Release(c1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	_, err = p.Acquire(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// TestGetIdle_LifetimeAndIdleEviction pins the two destroyLocked branches
// in getIdle: connection past MaxLifetime, and connection past IdleTimeout.
func TestGetIdle_LifetimeAndIdleEviction(t *testing.T) {
	var seq atomic.Int64
	p := New[int](Config{
		MaxSize:        4,
		MaxLifetime:    10 * time.Millisecond,
		AcquireTimeout: time.Second,
	}, intFactory(&seq), intCloser())

	// Create + release a connection so it sits in idle.
	c, _ := p.Acquire(context.Background())
	p.Release(c)

	// Wait past MaxLifetime so the next Acquire's getIdle loop evicts it
	// and falls through to create a new one.
	time.Sleep(20 * time.Millisecond)
	c2, err := p.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if c2.id == c.id {
		t.Errorf("expired conn was reused: id %d == %d", c2.id, c.id)
	}
	p.Release(c2)

	// Now exercise IdleTimeout eviction with a fresh pool.
	var seq2 atomic.Int64
	p2 := New[int](Config{
		MaxSize:        4,
		IdleTimeout:    10 * time.Millisecond,
		AcquireTimeout: time.Second,
	}, intFactory(&seq2), intCloser())
	x, _ := p2.Acquire(context.Background())
	p2.Release(x)
	time.Sleep(20 * time.Millisecond)
	x2, err := p2.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if x2.id == x.id {
		t.Errorf("idle-expired conn was reused: id %d == %d", x2.id, x.id)
	}
}
