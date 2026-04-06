package clockx

import (
	"testing"
	"time"
)

func TestReal(t *testing.T) {
	c := Real()
	now := c.Now()
	if now.IsZero() { t.Error("should not be zero") }

	time.Sleep(1 * time.Millisecond)
	since := c.Since(now)
	if since <= 0 { t.Error("since should be positive") }
}

func TestFake_Now(t *testing.T) {
	ts := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	f := NewFake(ts)

	if !f.Now().Equal(ts) { t.Errorf("Now = %v", f.Now()) }
}

func TestFake_Advance(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(ts)

	f.Advance(1 * time.Hour)
	expected := time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC)
	if !f.Now().Equal(expected) { t.Errorf("Now = %v", f.Now()) }
}

func TestFake_Set(t *testing.T) {
	f := NewFake(time.Now())
	newTime := time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC)
	f.Set(newTime)

	if !f.Now().Equal(newTime) { t.Error("Set") }
}

func TestFake_Since(t *testing.T) {
	ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	f := NewFake(ts)

	past := ts.Add(-1 * time.Hour)
	if f.Since(past) != 1*time.Hour { t.Errorf("Since = %v", f.Since(past)) }
}

func TestFake_After(t *testing.T) {
	f := NewFake(time.Now())
	ch := f.After(5 * time.Second)

	select {
	case <-ch:
		// OK - fake After fires immediately
	default:
		t.Error("fake After should fire immediately")
	}
}

func TestReal_After(t *testing.T) {
	c := Real()
	ch := c.After(1 * time.Millisecond)
	select {
	case <-ch:
	case <-time.After(1 * time.Second):
		t.Error("should fire")
	}
}

func TestInterface(t *testing.T) {
	// Both should satisfy Clock interface
	var _ Clock = Real()
	var _ Clock = NewFake(time.Now())
}
