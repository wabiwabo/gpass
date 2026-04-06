package singleflight

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDo_SingleCall(t *testing.T) {
	var g Group

	val, err, shared := g.Do("key", func() (interface{}, error) {
		return "hello", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hello" {
		t.Fatalf("expected \"hello\", got %v", val)
	}
	if shared {
		t.Fatal("expected shared=false for single call")
	}
}

func TestDo_DuplicateSuppression(t *testing.T) {
	var g Group
	var calls atomic.Int64

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)

	// Gate to ensure all goroutines start before the function completes.
	gate := make(chan struct{})

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			val, err, _ := g.Do("key", func() (interface{}, error) {
				calls.Add(1)
				<-gate
				return 42, nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if val != 42 {
				t.Errorf("expected 42, got %v", val)
			}
		}()
	}

	// Give goroutines time to all call Do.
	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()

	if c := calls.Load(); c != 1 {
		t.Fatalf("expected fn to be called once, got %d", c)
	}
}

func TestDo_DifferentKeys(t *testing.T) {
	var g Group
	var calls atomic.Int64

	var wg sync.WaitGroup
	wg.Add(3)

	for _, key := range []string{"a", "b", "c"} {
		go func(k string) {
			defer wg.Done()
			_, err, _ := g.Do(k, func() (interface{}, error) {
				calls.Add(1)
				time.Sleep(10 * time.Millisecond)
				return k, nil
			})
			if err != nil {
				t.Errorf("unexpected error for key %s: %v", k, err)
			}
		}(key)
	}

	wg.Wait()

	if c := calls.Load(); c != 3 {
		t.Fatalf("expected 3 calls for different keys, got %d", c)
	}
}

func TestDo_ErrorPropagation(t *testing.T) {
	var g Group
	expectedErr := errors.New("something failed")

	const n = 5
	var wg sync.WaitGroup
	wg.Add(n)

	gate := make(chan struct{})

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_, err, _ := g.Do("err-key", func() (interface{}, error) {
				<-gate
				return nil, expectedErr
			})
			if !errors.Is(err, expectedErr) {
				t.Errorf("expected %v, got %v", expectedErr, err)
			}
		}()
	}

	time.Sleep(50 * time.Millisecond)
	close(gate)
	wg.Wait()
}

func TestDo_SharedFlag(t *testing.T) {
	var g Group

	started := make(chan struct{})
	gate := make(chan struct{})

	var firstShared, secondShared bool

	var wg sync.WaitGroup
	wg.Add(2)

	// First caller.
	go func() {
		defer wg.Done()
		_, _, shared := g.Do("key", func() (interface{}, error) {
			close(started)
			<-gate
			return "result", nil
		})
		firstShared = shared
	}()

	// Wait for first caller to start executing.
	<-started

	// Second caller joins the in-flight call.
	go func() {
		defer wg.Done()
		_, _, shared := g.Do("key", func() (interface{}, error) {
			return "result", nil
		})
		secondShared = shared
	}()

	time.Sleep(20 * time.Millisecond)
	close(gate)
	wg.Wait()

	if firstShared {
		t.Fatal("expected first caller shared=false")
	}
	if !secondShared {
		t.Fatal("expected second caller shared=true")
	}
}

func TestDoWithContext_Cancellation(t *testing.T) {
	var g Group

	started := make(chan struct{})
	gate := make(chan struct{})

	// First caller starts a long-running call.
	go func() {
		ctx := context.Background()
		g.DoWithContext(ctx, "key", func(ctx context.Context) (interface{}, error) {
			close(started)
			<-gate
			return "done", nil
		})
	}()

	<-started

	// Second caller with a context that will be cancelled.
	ctx, cancel := context.WithCancel(context.Background())

	var secondErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err, _ := g.DoWithContext(ctx, "key", func(ctx context.Context) (interface{}, error) {
			return "should not run", nil
		})
		secondErr = err
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()
	wg.Wait()

	if !errors.Is(secondErr, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", secondErr)
	}

	// Let the first call finish.
	close(gate)
}

func TestDoWithContext_Success(t *testing.T) {
	var g Group

	ctx := context.Background()
	val, err, shared := g.DoWithContext(ctx, "key", func(ctx context.Context) (interface{}, error) {
		return "success", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "success" {
		t.Fatalf("expected \"success\", got %v", val)
	}
	if shared {
		t.Fatal("expected shared=false for single call")
	}
}

func TestForget(t *testing.T) {
	var g Group
	var calls atomic.Int64

	started := make(chan struct{})
	gate := make(chan struct{})

	// Start first call.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.Do("key", func() (interface{}, error) {
			calls.Add(1)
			close(started)
			<-gate
			return "first", nil
		})
	}()

	<-started

	// Forget the key while first call is still running.
	g.Forget("key")

	// Second call should start a new execution.
	wg.Add(1)
	go func() {
		defer wg.Done()
		val, _, _ := g.Do("key", func() (interface{}, error) {
			calls.Add(1)
			return "second", nil
		})
		if val != "second" {
			t.Errorf("expected \"second\", got %v", val)
		}
	}()

	// Let first call finish.
	close(gate)
	wg.Wait()

	if c := calls.Load(); c != 2 {
		t.Fatalf("expected 2 calls after Forget, got %d", c)
	}
}

func TestStats(t *testing.T) {
	var g Group

	// Initial stats should be zero.
	s := g.Stats()
	if s.TotalCalls != 0 || s.SharedResults != 0 || s.InFlightCount != 0 {
		t.Fatalf("expected zero stats, got %+v", s)
	}

	// Single call.
	g.Do("a", func() (interface{}, error) { return 1, nil })
	s = g.Stats()
	if s.TotalCalls != 1 {
		t.Fatalf("expected TotalCalls=1, got %d", s.TotalCalls)
	}

	// Duplicate suppression should increment shared.
	started := make(chan struct{})
	gate := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(3)

	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			g.Do("b", func() (interface{}, error) {
				close(started)
				<-gate
				return 2, nil
			})
		}()
	}

	<-started
	time.Sleep(20 * time.Millisecond)

	// Check in-flight count.
	s = g.Stats()
	if s.InFlightCount != 1 {
		t.Fatalf("expected InFlightCount=1, got %d", s.InFlightCount)
	}

	close(gate)
	wg.Wait()

	s = g.Stats()
	if s.TotalCalls != 4 { // 1 + 3
		t.Fatalf("expected TotalCalls=4, got %d", s.TotalCalls)
	}
	if s.SharedResults != 2 { // 2 waiters got shared results
		t.Fatalf("expected SharedResults=2, got %d", s.SharedResults)
	}
	if s.InFlightCount != 0 {
		t.Fatalf("expected InFlightCount=0, got %d", s.InFlightCount)
	}
}

func TestDo_ConcurrentRaceDetector(t *testing.T) {
	var g Group

	const goroutines = 100
	const keys = 10

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + (i % keys)))

			val, err, _ := g.Do(key, func() (interface{}, error) {
				time.Sleep(time.Millisecond)
				return i, nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if val == nil {
				t.Error("unexpected nil value")
			}
		}(i)
	}

	wg.Wait()

	// Also exercise Stats, Forget, and DoWithContext concurrently.
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + (i % keys)))

			switch i % 3 {
			case 0:
				g.Do(key, func() (interface{}, error) {
					return i, nil
				})
			case 1:
				g.DoWithContext(context.Background(), key, func(ctx context.Context) (interface{}, error) {
					return i, nil
				})
			case 2:
				g.Forget(key)
				g.Stats()
			}
		}(i)
	}

	wg.Wait()
}
