package adaptive

import (
	"testing"
	"time"
)

func TestThrottle_AllowsWithinRate(t *testing.T) {
	th := New(Config{MaxRate: 100, Window: time.Second})

	allowed := 0
	for i := 0; i < 100; i++ {
		if th.Allow() {
			allowed++
		}
	}

	if allowed != 100 {
		t.Errorf("allowed: got %d, want 100", allowed)
	}
}

func TestThrottle_DeniesOverRate(t *testing.T) {
	th := New(Config{MaxRate: 5, Window: time.Second})

	for i := 0; i < 5; i++ {
		th.Allow()
	}

	if th.Allow() {
		t.Error("should deny when over rate")
	}

	stats := th.Stats()
	if stats.Denied == 0 {
		t.Error("denied count should be > 0")
	}
}

func TestThrottle_BacksOffOnErrors(t *testing.T) {
	th := New(Config{
		MaxRate:           100,
		MinRate:           10,
		BackoffMultiplier: 0.5,
		ErrorThreshold:    0.1,
		Window:            50 * time.Millisecond,
	})

	// Generate requests with high error rate.
	for i := 0; i < 10; i++ {
		th.Allow()
		th.RecordError()
	}

	// Wait for window to expire.
	time.Sleep(60 * time.Millisecond)
	th.Allow() // trigger adjustment

	rate := th.CurrentRate()
	if rate >= 100 {
		t.Errorf("rate should have decreased from 100, got %f", rate)
	}
}

func TestThrottle_RecoversWhenHealthy(t *testing.T) {
	th := New(Config{
		MaxRate:           100,
		MinRate:           10,
		BackoffMultiplier: 0.5,
		RecoverMultiplier: 2.0,
		ErrorThreshold:    0.5,
		Window:            50 * time.Millisecond,
	})

	// Force backoff.
	for i := 0; i < 10; i++ {
		th.Allow()
		th.RecordError()
	}
	time.Sleep(60 * time.Millisecond)
	th.Allow()

	backedOffRate := th.CurrentRate()

	// Healthy window — no errors.
	for i := 0; i < 5; i++ {
		th.Allow()
	}
	time.Sleep(60 * time.Millisecond)
	th.Allow()

	recoveredRate := th.CurrentRate()
	if recoveredRate <= backedOffRate {
		t.Errorf("rate should recover: backed=%f, recovered=%f", backedOffRate, recoveredRate)
	}
}

func TestThrottle_MinRateFloor(t *testing.T) {
	th := New(Config{
		MaxRate:           100,
		MinRate:           10,
		BackoffMultiplier: 0.01, // aggressive backoff
		ErrorThreshold:    0.01,
		Window:            10 * time.Millisecond,
	})

	// Many error windows.
	for round := 0; round < 5; round++ {
		for i := 0; i < 5; i++ {
			th.Allow()
			th.RecordError()
		}
		time.Sleep(15 * time.Millisecond)
		th.Allow()
	}

	if th.CurrentRate() < 10 {
		t.Errorf("rate should not go below minRate 10, got %f", th.CurrentRate())
	}
}

func TestThrottle_MaxRateCeiling(t *testing.T) {
	th := New(Config{
		MaxRate:           50,
		RecoverMultiplier: 10.0, // aggressive recovery
		Window:            10 * time.Millisecond,
	})

	// Several healthy windows.
	for round := 0; round < 5; round++ {
		for i := 0; i < 3; i++ {
			th.Allow()
		}
		time.Sleep(15 * time.Millisecond)
		th.Allow()
	}

	if th.CurrentRate() > 50 {
		t.Errorf("rate should not exceed maxRate 50, got %f", th.CurrentRate())
	}
}

func TestThrottle_DefaultConfig(t *testing.T) {
	th := New(Config{MaxRate: 100})
	if th.minRate != 1 {
		t.Errorf("default minRate: got %f", th.minRate)
	}
	if th.backoffRate != 0.5 {
		t.Errorf("default backoffRate: got %f", th.backoffRate)
	}
	if th.recoverRate != 1.1 {
		t.Errorf("default recoverRate: got %f", th.recoverRate)
	}
	if th.errorThreshold != 0.1 {
		t.Errorf("default errorThreshold: got %f", th.errorThreshold)
	}
}

func TestThrottle_Stats(t *testing.T) {
	th := New(Config{MaxRate: 10, Window: time.Second})

	for i := 0; i < 10; i++ {
		th.Allow()
	}
	th.Allow() // should be denied

	stats := th.Stats()
	if stats.Allowed != 10 {
		t.Errorf("allowed: got %d", stats.Allowed)
	}
	if stats.Denied != 1 {
		t.Errorf("denied: got %d", stats.Denied)
	}
	if stats.MaxRate != 10 {
		t.Errorf("maxRate: got %f", stats.MaxRate)
	}
}

func TestThrottle_ConcurrentAccess(t *testing.T) {
	th := New(Config{MaxRate: 1000, Window: time.Second})

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				if th.Allow() {
					th.RecordSuccess()
				} else {
					th.RecordError()
				}
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	stats := th.Stats()
	total := stats.Allowed + stats.Denied
	if total != 1000 {
		t.Errorf("total: got %d, want 1000", total)
	}
}
