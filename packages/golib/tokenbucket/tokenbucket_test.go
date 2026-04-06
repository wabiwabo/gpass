package tokenbucket

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	b := New(10, 5)
	s := b.Stats()
	if s.Capacity != 10 {
		t.Errorf("Capacity = %f", s.Capacity)
	}
	if s.Rate != 5 {
		t.Errorf("Rate = %f", s.Rate)
	}
}

func TestAllow(t *testing.T) {
	b := New(3, 1)
	if !b.Allow() { t.Error("1st") }
	if !b.Allow() { t.Error("2nd") }
	if !b.Allow() { t.Error("3rd") }
	if b.Allow() { t.Error("4th should be denied") }
}

func TestAllowN(t *testing.T) {
	b := New(10, 1)
	if !b.AllowN(5) { t.Error("5 should be allowed") }
	if !b.AllowN(5) { t.Error("another 5") }
	if b.AllowN(1) { t.Error("should be denied") }
}

func TestStats(t *testing.T) {
	b := New(10, 1)
	b.Allow()
	b.Allow()
	b.AllowN(100) // rejected

	s := b.Stats()
	if s.Allowed != 2 {
		t.Errorf("Allowed = %d", s.Allowed)
	}
	if s.Rejected != 1 {
		t.Errorf("Rejected = %d", s.Rejected)
	}
	if s.RejectRate < 0.3 || s.RejectRate > 0.4 {
		t.Errorf("RejectRate = %f", s.RejectRate)
	}
	if s.Utilization < 0 || s.Utilization > 1 {
		t.Errorf("Utilization = %f", s.Utilization)
	}
}

func TestWaitDuration(t *testing.T) {
	b := New(10, 10) // 10/s
	b.AllowN(10) // drain

	d := b.WaitDuration(5) // need 5 at 10/s = 0.5s
	if d < 400*time.Millisecond || d > 600*time.Millisecond {
		t.Errorf("WaitDuration = %v", d)
	}
}

func TestWaitDuration_Available(t *testing.T) {
	b := New(10, 1)
	if d := b.WaitDuration(1); d != 0 {
		t.Errorf("WaitDuration = %v, want 0", d)
	}
}

func TestReset(t *testing.T) {
	b := New(10, 1)
	b.AllowN(10)
	b.Allow() // rejected

	b.Reset()
	s := b.Stats()
	if s.Allowed != 0 || s.Rejected != 0 {
		t.Error("metrics should be reset")
	}
	if !b.Allow() {
		t.Error("should allow after reset")
	}
}

func TestRefill(t *testing.T) {
	b := New(5, 1000) // 1000/s
	b.AllowN(5)
	time.Sleep(10 * time.Millisecond) // ~10 tokens

	tokens := b.Tokens()
	if tokens > 5 {
		t.Error("should cap at capacity")
	}
}

func TestStats_ZeroTotal(t *testing.T) {
	b := New(10, 1)
	s := b.Stats()
	if s.RejectRate != 0 {
		t.Errorf("RejectRate = %f with no requests", s.RejectRate)
	}
}
