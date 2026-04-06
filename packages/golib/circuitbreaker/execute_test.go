package circuitbreaker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestAdvancedBreaker_Execute_Success(t *testing.T) {
	b := NewAdvanced(Config{Name: "test", Threshold: 3, Cooldown: time.Second})

	err := b.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if b.State() != StateClosed {
		t.Errorf("state: got %q, want closed", b.State())
	}
}

func TestAdvancedBreaker_Execute_OpensAfterThreshold(t *testing.T) {
	b := NewAdvanced(Config{Name: "test", Threshold: 3, Cooldown: time.Second})

	testErr := errors.New("fail")
	for i := 0; i < 3; i++ {
		b.Execute(context.Background(), func(_ context.Context) error {
			return testErr
		})
	}

	if b.State() != StateOpen {
		t.Errorf("state: got %q, want open", b.State())
	}

	// Should return ErrCircuitOpen.
	err := b.Execute(context.Background(), func(_ context.Context) error {
		t.Fatal("should not be called when circuit is open")
		return nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestAdvancedBreaker_Execute_HalfOpenToClose(t *testing.T) {
	b := NewAdvanced(Config{
		Name:             "test",
		Threshold:        2,
		Cooldown:         50 * time.Millisecond,
		SuccessThreshold: 2,
	})

	// Open the circuit.
	for i := 0; i < 2; i++ {
		b.Execute(context.Background(), func(_ context.Context) error {
			return errors.New("fail")
		})
	}

	// Wait for cooldown.
	time.Sleep(60 * time.Millisecond)

	// First success in half-open.
	b.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})

	if b.State() != StateHalfOpen {
		t.Errorf("after 1 success: got %q, want half-open (need 2)", b.State())
	}

	// Second success should close.
	b.Execute(context.Background(), func(_ context.Context) error {
		return nil
	})

	if b.State() != StateClosed {
		t.Errorf("after 2 successes: got %q, want closed", b.State())
	}
}

func TestAdvancedBreaker_Execute_HalfOpenFailure(t *testing.T) {
	b := NewAdvanced(Config{
		Name:      "test",
		Threshold: 2,
		Cooldown:  50 * time.Millisecond,
	})

	// Open.
	for i := 0; i < 2; i++ {
		b.Execute(context.Background(), func(_ context.Context) error {
			return errors.New("fail")
		})
	}

	time.Sleep(60 * time.Millisecond)

	// Failure in half-open → back to open.
	b.Execute(context.Background(), func(_ context.Context) error {
		return errors.New("still failing")
	})

	if b.State() != StateOpen {
		t.Errorf("after half-open failure: got %q, want open", b.State())
	}
}

func TestAdvancedBreaker_OnStateChange(t *testing.T) {
	var transitions []string
	var mu sync.Mutex

	b := NewAdvanced(Config{
		Name:      "test",
		Threshold: 2,
		Cooldown:  50 * time.Millisecond,
		OnStateChange: func(name, from, to string) {
			mu.Lock()
			transitions = append(transitions, from+"→"+to)
			mu.Unlock()
		},
	})

	// Trigger open.
	for i := 0; i < 2; i++ {
		b.Execute(context.Background(), func(_ context.Context) error {
			return errors.New("fail")
		})
	}

	mu.Lock()
	if len(transitions) != 1 || transitions[0] != "closed→open" {
		t.Errorf("transitions: got %v", transitions)
	}
	mu.Unlock()
}

func TestAdvancedBreaker_ExecuteWithResult(t *testing.T) {
	b := NewAdvanced(Config{Name: "result", Threshold: 5})

	result, err := ExecuteWithResult(b, context.Background(), func(_ context.Context) (string, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Errorf("result: got %q", result)
	}
}

func TestAdvancedBreaker_ExecuteWithResult_CircuitOpen(t *testing.T) {
	b := NewAdvanced(Config{Name: "result", Threshold: 1, Cooldown: time.Hour})

	b.Execute(context.Background(), func(_ context.Context) error {
		return errors.New("fail")
	})

	_, err := ExecuteWithResult(b, context.Background(), func(_ context.Context) (int, error) {
		return 42, nil
	})
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestAdvancedBreaker_DefaultConfig(t *testing.T) {
	b := NewAdvanced(Config{Name: "defaults"})
	if b.threshold != 5 {
		t.Errorf("default threshold: got %d", b.threshold)
	}
	if b.successThreshold != 2 {
		t.Errorf("default successThreshold: got %d", b.successThreshold)
	}
}

func TestAdvancedBreaker_Name(t *testing.T) {
	b := NewAdvanced(Config{Name: "dukcapil"})
	if b.Name() != "dukcapil" {
		t.Errorf("name: got %q", b.Name())
	}
}
