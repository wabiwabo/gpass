package circuitmanager

import (
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	m := New()
	total, open := m.Count()
	if total != 0 || open != 0 {
		t.Errorf("Count = (%d, %d)", total, open)
	}
}

func TestRegister(t *testing.T) {
	m := New()
	m.Register("identity")
	m.Register("garudacorp")

	total, _ := m.Count()
	if total != 2 {
		t.Errorf("total = %d", total)
	}
}

func TestRecordSuccess(t *testing.T) {
	m := New()
	m.Register("svc")
	m.RecordSuccess("svc")
	m.RecordSuccess("svc")

	c, _ := m.Get("svc")
	if c.Successes != 2 {
		t.Errorf("Successes = %d", c.Successes)
	}
	if c.LastSuccess.IsZero() {
		t.Error("LastSuccess should be set")
	}
}

func TestRecordFailure(t *testing.T) {
	m := New()
	m.Register("svc")
	m.RecordFailure("svc")

	c, _ := m.Get("svc")
	if c.Failures != 1 {
		t.Errorf("Failures = %d", c.Failures)
	}
	if c.LastFailure.IsZero() {
		t.Error("LastFailure should be set")
	}
}

func TestSetState(t *testing.T) {
	m := New()
	m.Register("svc")

	m.SetState("svc", StateOpen)
	c, _ := m.Get("svc")
	if c.State != StateOpen {
		t.Errorf("State = %q", c.State)
	}

	m.SetState("svc", StateHalfOpen)
	c, _ = m.Get("svc")
	if c.State != StateHalfOpen {
		t.Errorf("State = %q", c.State)
	}
}

func TestSetState_SameState(t *testing.T) {
	m := New()
	m.Register("svc")

	c1, _ := m.Get("svc")
	firstChanged := c1.StateChanged

	m.SetState("svc", StateClosed) // same state
	c2, _ := m.Get("svc")

	if c2.StateChanged != firstChanged {
		t.Error("StateChanged should not update when state doesn't change")
	}
}

func TestGet_NotFound(t *testing.T) {
	m := New()
	_, ok := m.Get("nonexistent")
	if ok {
		t.Error("should return false")
	}
}

func TestAll(t *testing.T) {
	m := New()
	m.Register("c")
	m.Register("a")
	m.Register("b")

	all := m.All()
	if len(all) != 3 {
		t.Fatalf("len = %d", len(all))
	}
	if all[0].Name != "a" || all[1].Name != "b" || all[2].Name != "c" {
		t.Errorf("not sorted: %v", all)
	}
}

func TestOpen(t *testing.T) {
	m := New()
	m.Register("svc-1")
	m.Register("svc-2")
	m.Register("svc-3")

	m.SetState("svc-2", StateOpen)

	open := m.Open()
	if len(open) != 1 {
		t.Fatalf("open = %d", len(open))
	}
	if open[0].Name != "svc-2" {
		t.Errorf("Name = %q", open[0].Name)
	}
}

func TestCount(t *testing.T) {
	m := New()
	m.Register("svc-1")
	m.Register("svc-2")
	m.SetState("svc-2", StateOpen)

	total, open := m.Count()
	if total != 2 {
		t.Errorf("total = %d", total)
	}
	if open != 1 {
		t.Errorf("open = %d", open)
	}
}

func TestReset(t *testing.T) {
	m := New()
	m.Register("svc")
	m.RecordFailure("svc")
	m.RecordSuccess("svc")
	m.SetState("svc", StateOpen)

	if !m.Reset("svc") {
		t.Error("should return true")
	}

	c, _ := m.Get("svc")
	if c.Failures != 0 || c.Successes != 0 {
		t.Error("counters should be reset")
	}
	if c.State != StateClosed {
		t.Errorf("State = %q, want closed", c.State)
	}
}

func TestReset_NotFound(t *testing.T) {
	m := New()
	if m.Reset("nonexistent") {
		t.Error("should return false")
	}
}

func TestResetAll(t *testing.T) {
	m := New()
	m.Register("a")
	m.Register("b")
	m.RecordFailure("a")
	m.SetState("a", StateOpen)
	m.RecordFailure("b")

	m.ResetAll()

	for _, c := range m.All() {
		if c.State != StateClosed {
			t.Errorf("%s State = %q", c.Name, c.State)
		}
		if c.Failures != 0 {
			t.Errorf("%s Failures = %d", c.Name, c.Failures)
		}
	}
}

func TestRecordOnNonexistent(t *testing.T) {
	m := New()
	// Should not panic
	m.RecordSuccess("nonexistent")
	m.RecordFailure("nonexistent")
	m.SetState("nonexistent", StateOpen)
}

func TestConcurrent(t *testing.T) {
	m := New()
	m.Register("svc")
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			m.RecordSuccess("svc")
		}()
		go func() {
			defer wg.Done()
			m.RecordFailure("svc")
		}()
		go func() {
			defer wg.Done()
			m.All()
		}()
	}
	wg.Wait()

	c, _ := m.Get("svc")
	if c.Successes+c.Failures != 200 {
		t.Errorf("total = %d, want 200", c.Successes+c.Failures)
	}
}
