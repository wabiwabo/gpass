// Package circuitmanager manages multiple circuit breakers for
// different services/endpoints. Provides a centralized view of
// circuit states and bulk operations.
package circuitmanager

import (
	"sort"
	"sync"
	"time"
)

// State represents a circuit state.
type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

// Circuit represents a managed circuit breaker's metadata.
type Circuit struct {
	Name         string    `json:"name"`
	State        State     `json:"state"`
	Failures     int       `json:"failures"`
	Successes    int       `json:"successes"`
	LastFailure  time.Time `json:"last_failure,omitempty"`
	LastSuccess  time.Time `json:"last_success,omitempty"`
	StateChanged time.Time `json:"state_changed"`
}

// Manager manages a collection of circuit breakers.
type Manager struct {
	mu       sync.RWMutex
	circuits map[string]*Circuit
}

// New creates a circuit manager.
func New() *Manager {
	return &Manager{
		circuits: make(map[string]*Circuit),
	}
}

// Register adds a new circuit breaker.
func (m *Manager) Register(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.circuits[name] = &Circuit{
		Name:         name,
		State:        StateClosed,
		StateChanged: time.Now().UTC(),
	}
}

// RecordSuccess records a successful call for a circuit.
func (m *Manager) RecordSuccess(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.circuits[name]
	if !ok {
		return
	}
	c.Successes++
	c.LastSuccess = time.Now().UTC()
}

// RecordFailure records a failed call for a circuit.
func (m *Manager) RecordFailure(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.circuits[name]
	if !ok {
		return
	}
	c.Failures++
	c.LastFailure = time.Now().UTC()
}

// SetState updates a circuit's state.
func (m *Manager) SetState(name string, state State) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.circuits[name]
	if !ok {
		return
	}
	if c.State != state {
		c.State = state
		c.StateChanged = time.Now().UTC()
	}
}

// Get returns a circuit's status.
func (m *Manager) Get(name string) (Circuit, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.circuits[name]
	if !ok {
		return Circuit{}, false
	}
	return *c, true
}

// All returns all circuits sorted by name.
func (m *Manager) All() []Circuit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Circuit, 0, len(m.circuits))
	for _, c := range m.circuits {
		result = append(result, *c)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Open returns all circuits in the open state.
func (m *Manager) Open() []Circuit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Circuit
	for _, c := range m.circuits {
		if c.State == StateOpen {
			result = append(result, *c)
		}
	}
	return result
}

// Count returns total and open circuit counts.
func (m *Manager) Count() (total, open int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total = len(m.circuits)
	for _, c := range m.circuits {
		if c.State == StateOpen {
			open++
		}
	}
	return
}

// Reset resets a circuit's counters and sets it to closed.
func (m *Manager) Reset(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.circuits[name]
	if !ok {
		return false
	}
	c.Failures = 0
	c.Successes = 0
	c.State = StateClosed
	c.StateChanged = time.Now().UTC()
	return true
}

// ResetAll resets all circuits.
func (m *Manager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, c := range m.circuits {
		c.Failures = 0
		c.Successes = 0
		c.State = StateClosed
		c.StateChanged = time.Now().UTC()
	}
}
