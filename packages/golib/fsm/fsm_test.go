package fsm

import "testing"

const (
	StatePending  State = "pending"
	StateActive   State = "active"
	StateSuspended State = "suspended"
	StateClosed   State = "closed"

	EventActivate Event = "activate"
	EventSuspend  Event = "suspend"
	EventResume   Event = "resume"
	EventClose    Event = "close"
)

func newTestMachine() *Machine {
	m := New(StatePending)
	m.AddTransition(StatePending, EventActivate, StateActive)
	m.AddTransition(StateActive, EventSuspend, StateSuspended)
	m.AddTransition(StateSuspended, EventResume, StateActive)
	m.AddTransition(StateActive, EventClose, StateClosed)
	m.AddTransition(StateSuspended, EventClose, StateClosed)
	return m
}

func TestNew(t *testing.T) {
	m := New(StatePending)
	if m.Current() != StatePending {
		t.Errorf("Current = %q", m.Current())
	}
}

func TestFire(t *testing.T) {
	m := newTestMachine()

	if err := m.Fire(EventActivate); err != nil {
		t.Fatalf("Fire: %v", err)
	}
	if m.Current() != StateActive {
		t.Errorf("Current = %q", m.Current())
	}
}

func TestFire_InvalidTransition(t *testing.T) {
	m := newTestMachine()
	if err := m.Fire(EventSuspend); err == nil {
		t.Error("should error (can't suspend from pending)")
	}
}

func TestFire_Chain(t *testing.T) {
	m := newTestMachine()
	m.Fire(EventActivate)
	m.Fire(EventSuspend)
	m.Fire(EventResume)

	if m.Current() != StateActive {
		t.Errorf("Current = %q", m.Current())
	}
}

func TestCan(t *testing.T) {
	m := newTestMachine()

	if !m.Can(EventActivate) {
		t.Error("should be able to activate from pending")
	}
	if m.Can(EventSuspend) {
		t.Error("should not be able to suspend from pending")
	}
}

func TestIs(t *testing.T) {
	m := New(StatePending)
	if !m.Is(StatePending) {
		t.Error("should be pending")
	}
	if m.Is(StateActive) {
		t.Error("should not be active")
	}
}

func TestGuardedTransition(t *testing.T) {
	m := New(StatePending)
	allowed := true
	m.AddGuardedTransition(StatePending, EventActivate, StateActive, func() bool {
		return allowed
	})

	if err := m.Fire(EventActivate); err != nil {
		t.Errorf("should pass guard: %v", err)
	}

	// Reset and test with guard rejection
	m2 := New(StatePending)
	m2.AddGuardedTransition(StatePending, EventActivate, StateActive, func() bool {
		return false
	})

	if err := m2.Fire(EventActivate); err == nil {
		t.Error("should reject when guard returns false")
	}
}

func TestOnTransition(t *testing.T) {
	m := newTestMachine()

	var fromState, toState State
	var firedEvent Event

	m.OnTransition(func(from, to State, event Event) {
		fromState = from
		toState = to
		firedEvent = event
	})

	m.Fire(EventActivate)

	if fromState != StatePending || toState != StateActive || firedEvent != EventActivate {
		t.Errorf("callback: %s -> %s on %s", fromState, toState, firedEvent)
	}
}

func TestAvailableEvents(t *testing.T) {
	m := newTestMachine()
	events := m.AvailableEvents()

	if len(events) != 1 || events[0] != EventActivate {
		t.Errorf("events = %v, want [activate]", events)
	}

	m.Fire(EventActivate)
	events = m.AvailableEvents()
	if len(events) != 2 { // suspend, close
		t.Errorf("events = %v, want 2", events)
	}
}

func TestCan_WithGuard(t *testing.T) {
	m := New(StatePending)
	m.AddGuardedTransition(StatePending, EventActivate, StateActive, func() bool {
		return false
	})

	if m.Can(EventActivate) {
		t.Error("should return false when guard rejects")
	}
}
