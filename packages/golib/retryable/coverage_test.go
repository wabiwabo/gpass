package retryable

import (
	"testing"
	"time"
)

// TestComputeDelay_CapsAtMax pins that exponential growth is bounded by
// cfg.Max. Without the cap, attempt=20 with multiplier=2 and initial=1ms
// would compute a 2^20 ms ≈ 17-minute delay, which would silently break
// every caller relying on a "max 10s" bound.
func TestComputeDelay_CapsAtMax(t *testing.T) {
	cfg := Config{
		Initial:    1 * time.Millisecond,
		Max:        100 * time.Millisecond,
		Multiplier: 2.0,
		Jitter:     false,
	}
	// attempt=10 → 1024ms uncapped, must clamp to 100ms.
	got := computeDelay(10, cfg)
	if got != 100*time.Millisecond {
		t.Errorf("attempt=10 delay = %v, want 100ms (capped)", got)
	}
	// attempt=20 → ~1MB ms uncapped, still capped.
	got = computeDelay(20, cfg)
	if got != 100*time.Millisecond {
		t.Errorf("attempt=20 delay = %v, want 100ms (capped)", got)
	}
}

// TestComputeDelay_ExponentialGrowthBeforeCap pins the unclamped growth
// schedule: with multiplier=2 and initial=10ms, attempts 0..3 should
// produce 10, 20, 40, 80 ms.
func TestComputeDelay_ExponentialGrowthBeforeCap(t *testing.T) {
	cfg := Config{
		Initial:    10 * time.Millisecond,
		Max:        10 * time.Second,
		Multiplier: 2.0,
		Jitter:     false,
	}
	want := []time.Duration{10, 20, 40, 80}
	for i, w := range want {
		got := computeDelay(i, cfg)
		if got != w*time.Millisecond {
			t.Errorf("attempt=%d delay = %v, want %vms", i, got, w)
		}
	}
}

// TestComputeDelay_JitterBoundedRange pins that jitter stays within
// ±25% of the base delay. Without bounds, a buggy jitter could produce
// negative durations (which would either panic or never block).
func TestComputeDelay_JitterBoundedRange(t *testing.T) {
	cfg := Config{
		Initial:    100 * time.Millisecond,
		Max:        time.Second,
		Multiplier: 2.0,
		Jitter:     true,
	}
	// attempt=0 → base 100ms, jitter range [75ms, 125ms]
	for i := 0; i < 100; i++ {
		got := computeDelay(0, cfg)
		if got < 75*time.Millisecond || got > 125*time.Millisecond {
			t.Errorf("jittered delay %v outside [75ms, 125ms]", got)
		}
	}
}
