package retrypolicy

import (
	"errors"
	"testing"
	"time"
)

func TestDefault(t *testing.T) {
	p := Default()
	if p.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d", p.MaxRetries)
	}
	if p.InitialDelay != 100*time.Millisecond {
		t.Errorf("InitialDelay = %v", p.InitialDelay)
	}
	if p.MaxDelay != 10*time.Second {
		t.Errorf("MaxDelay = %v", p.MaxDelay)
	}
	if p.Multiplier != 2.0 {
		t.Errorf("Multiplier = %f", p.Multiplier)
	}
}

func TestNoRetry(t *testing.T) {
	p := NoRetry()
	if p.ShouldRetry(0) {
		t.Error("NoRetry should never retry")
	}
}

func TestShouldRetry(t *testing.T) {
	p := Policy{MaxRetries: 3}
	if !p.ShouldRetry(0) {
		t.Error("attempt 0 should retry")
	}
	if !p.ShouldRetry(2) {
		t.Error("attempt 2 should retry")
	}
	if p.ShouldRetry(3) {
		t.Error("attempt 3 should not retry (max=3)")
	}
	if p.ShouldRetry(5) {
		t.Error("attempt 5 should not retry")
	}
}

func TestDelay_ExponentialBackoff(t *testing.T) {
	p := Policy{
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0, // no jitter for predictable test
	}

	d0 := p.Delay(0)
	d1 := p.Delay(1)
	d2 := p.Delay(2)

	if d0 != 100*time.Millisecond {
		t.Errorf("Delay(0) = %v, want 100ms", d0)
	}
	if d1 != 200*time.Millisecond {
		t.Errorf("Delay(1) = %v, want 200ms", d1)
	}
	if d2 != 400*time.Millisecond {
		t.Errorf("Delay(2) = %v, want 400ms", d2)
	}
}

func TestDelay_CapsAtMaxDelay(t *testing.T) {
	p := Policy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Second,
		Multiplier:   10.0,
		JitterFactor: 0,
	}

	d := p.Delay(3) // 1s * 10^3 = 1000s → capped at 5s
	if d != 5*time.Second {
		t.Errorf("Delay(3) = %v, want 5s (capped)", d)
	}
}

func TestDelay_WithJitter(t *testing.T) {
	p := Policy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.5,
	}

	// Run many times, check that results vary
	delays := make(map[time.Duration]bool)
	for i := 0; i < 100; i++ {
		d := p.Delay(1)
		delays[d] = true

		// Should be in range [base - jitter, base + jitter]
		// base = 2s, jitter = 1s → [1s, 3s]
		if d < 500*time.Millisecond || d > 4*time.Second {
			t.Errorf("Delay(1) = %v, outside expected range", d)
		}
	}

	if len(delays) < 5 {
		t.Error("jitter should produce varied delays")
	}
}

func TestTotalMaxDuration(t *testing.T) {
	p := Policy{
		MaxRetries:   3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0,
	}

	total := p.TotalMaxDuration()
	// 100ms + 200ms + 400ms = 700ms
	if total != 700*time.Millisecond {
		t.Errorf("TotalMaxDuration = %v, want 700ms", total)
	}
}

func TestAlwaysRetry(t *testing.T) {
	if !AlwaysRetry(errors.New("any error")) {
		t.Error("should always return true")
	}
	if !AlwaysRetry(nil) {
		t.Error("should return true even for nil")
	}
}

func TestNeverRetry(t *testing.T) {
	if NeverRetry(errors.New("any error")) {
		t.Error("should always return false")
	}
}
