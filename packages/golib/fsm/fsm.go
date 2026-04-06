// Package fsm provides a generic finite state machine.
// Defines states, transitions, guards, and callbacks for
// managing entity lifecycle (e.g., user verification, document approval).
package fsm

import (
	"fmt"
	"sync"
)

// State represents a state in the machine.
type State string

// Event represents a trigger for a transition.
type Event string

// Guard is a function that determines if a transition is allowed.
type Guard func() bool

// Callback is called when a transition occurs.
type Callback func(from, to State, event Event)

// Transition defines a valid state change.
type Transition struct {
	From  State
	To    State
	Event Event
	Guard Guard
}

// Machine is a finite state machine.
type Machine struct {
	mu          sync.RWMutex
	current     State
	transitions []Transition
	callbacks   []Callback
}

// New creates a state machine with an initial state.
func New(initial State) *Machine {
	return &Machine{current: initial}
}

// AddTransition registers a valid transition.
func (m *Machine) AddTransition(from State, event Event, to State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transitions = append(m.transitions, Transition{From: from, To: to, Event: event})
}

// AddGuardedTransition registers a transition with a guard condition.
func (m *Machine) AddGuardedTransition(from State, event Event, to State, guard Guard) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.transitions = append(m.transitions, Transition{From: from, To: to, Event: event, Guard: guard})
}

// OnTransition registers a callback for any transition.
func (m *Machine) OnTransition(cb Callback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, cb)
}

// Fire triggers an event, executing the transition if valid.
func (m *Machine) Fire(event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range m.transitions {
		if t.From == m.current && t.Event == event {
			if t.Guard != nil && !t.Guard() {
				return fmt.Errorf("fsm: guard rejected transition %s -[%s]-> %s", t.From, event, t.To)
			}
			from := m.current
			m.current = t.To
			for _, cb := range m.callbacks {
				cb(from, t.To, event)
			}
			return nil
		}
	}

	return fmt.Errorf("fsm: no transition from %s on event %s", m.current, event)
}

// Can checks if an event can be fired from the current state.
func (m *Machine) Can(event Event) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.transitions {
		if t.From == m.current && t.Event == event {
			if t.Guard != nil {
				return t.Guard()
			}
			return true
		}
	}
	return false
}

// Current returns the current state.
func (m *Machine) Current() State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// Is checks if the machine is in a specific state.
func (m *Machine) Is(state State) bool {
	return m.Current() == state
}

// AvailableEvents returns events that can be fired from current state.
func (m *Machine) AvailableEvents() []Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := make(map[Event]bool)
	var events []Event
	for _, t := range m.transitions {
		if t.From == m.current && !seen[t.Event] {
			seen[t.Event] = true
			events = append(events, t.Event)
		}
	}
	return events
}
