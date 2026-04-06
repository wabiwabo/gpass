package errgroup

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	g := New(0)
	errs := g.Wait()
	if len(errs) != 0 {
		t.Errorf("errs = %d", len(errs))
	}
}

func TestGo_Success(t *testing.T) {
	g := New(0)
	var count atomic.Int32

	for i := 0; i < 10; i++ {
		g.Go(func() error {
			count.Add(1)
			return nil
		})
	}

	errs := g.Wait()
	if len(errs) != 0 {
		t.Errorf("errs = %d", len(errs))
	}
	if count.Load() != 10 {
		t.Errorf("count = %d, want 10", count.Load())
	}
}

func TestGo_Errors(t *testing.T) {
	g := New(0)
	g.Go(func() error { return errors.New("err1") })
	g.Go(func() error { return nil })
	g.Go(func() error { return errors.New("err2") })

	errs := g.Wait()
	if len(errs) != 2 {
		t.Errorf("errs = %d, want 2", len(errs))
	}
}

func TestWaitFirst(t *testing.T) {
	g := New(0)
	g.Go(func() error { return errors.New("first") })
	g.Go(func() error { return nil })

	err := g.WaitFirst()
	if err == nil {
		t.Error("should return first error")
	}
}

func TestWaitFirst_NoError(t *testing.T) {
	g := New(0)
	g.Go(func() error { return nil })

	err := g.WaitFirst()
	if err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestConcurrencyLimit(t *testing.T) {
	g := New(2)
	var running atomic.Int32
	var maxRunning atomic.Int32

	for i := 0; i < 20; i++ {
		g.Go(func() error {
			cur := running.Add(1)
			// Track max concurrent
			for {
				old := maxRunning.Load()
				if cur <= old || maxRunning.CompareAndSwap(old, cur) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			running.Add(-1)
			return nil
		})
	}

	g.Wait()
	max := maxRunning.Load()
	if max > 2 {
		t.Errorf("max concurrent = %d, limit was 2", max)
	}
}

func TestWithContext_CancelsOnError(t *testing.T) {
	g, ctx := WithContext(context.Background(), 0)

	g.Go(func() error {
		return errors.New("fail")
	})

	g.Go(func() error {
		// This should be cancelled
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(5 * time.Second):
			return errors.New("should have been cancelled")
		}
	})

	errs := g.Wait()
	// At least the first error
	if len(errs) < 1 {
		t.Errorf("errs = %d, want >= 1", len(errs))
	}
}

func TestErrors_NonBlocking(t *testing.T) {
	g := New(0)
	g.Go(func() error {
		return errors.New("err")
	})

	time.Sleep(10 * time.Millisecond) // let goroutine complete

	errs := g.Errors()
	// May or may not have the error yet, just check it doesn't block
	_ = errs

	g.Wait()
}

func TestNew_NoLimit(t *testing.T) {
	g := New(0)
	var count atomic.Int32

	for i := 0; i < 100; i++ {
		g.Go(func() error {
			count.Add(1)
			return nil
		})
	}

	g.Wait()
	if count.Load() != 100 {
		t.Errorf("count = %d", count.Load())
	}
}
