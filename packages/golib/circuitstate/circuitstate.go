// Package circuitstate provides circuit breaker state persistence.
// When a service restarts, it can restore circuit state from storage
// instead of starting all circuits closed and potentially flooding
// a failing downstream.
package circuitstate

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// State represents a persistent circuit breaker state.
type State struct {
	Name          string    `json:"name"`
	Status        string    `json:"status"` // "closed", "open", "half-open"
	Failures      int       `json:"failures"`
	Successes     int       `json:"successes"`
	LastFailure   time.Time `json:"last_failure,omitempty"`
	LastSuccess   time.Time `json:"last_success,omitempty"`
	OpenedAt      time.Time `json:"opened_at,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Store defines the interface for persisting circuit state.
type Store interface {
	Save(state State) error
	Load(name string) (State, error)
	LoadAll() ([]State, error)
	Delete(name string) error
}

// MemoryStore is an in-memory state store for testing.
type MemoryStore struct {
	mu     sync.RWMutex
	states map[string]State
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		states: make(map[string]State),
	}
}

// Save persists a circuit state.
func (m *MemoryStore) Save(state State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	state.UpdatedAt = time.Now()
	m.states[state.Name] = state
	return nil
}

// Load retrieves a circuit state by name.
func (m *MemoryStore) Load(name string) (State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	state, ok := m.states[name]
	if !ok {
		return State{}, fmt.Errorf("circuitstate: %q not found", name)
	}
	return state, nil
}

// LoadAll retrieves all circuit states.
func (m *MemoryStore) LoadAll() ([]State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	states := make([]State, 0, len(m.states))
	for _, s := range m.states {
		states = append(states, s)
	}
	return states, nil
}

// Delete removes a circuit state.
func (m *MemoryStore) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, name)
	return nil
}

// Size returns the number of stored states.
func (m *MemoryStore) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.states)
}

// Manager manages circuit state persistence for multiple circuits.
type Manager struct {
	store Store
	mu    sync.RWMutex
	cache map[string]State
}

// NewManager creates a state manager.
func NewManager(store Store) *Manager {
	return &Manager{
		store: store,
		cache: make(map[string]State),
	}
}

// Record updates the state for a circuit.
func (m *Manager) Record(name, status string, failures, successes int) error {
	state := State{
		Name:      name,
		Status:    status,
		Failures:  failures,
		Successes: successes,
		UpdatedAt: time.Now(),
	}

	if status == "open" {
		state.OpenedAt = time.Now()
		state.LastFailure = time.Now()
	} else if successes > 0 {
		state.LastSuccess = time.Now()
	}

	m.mu.Lock()
	m.cache[name] = state
	m.mu.Unlock()

	return m.store.Save(state)
}

// Get retrieves the current state for a circuit.
func (m *Manager) Get(name string) (State, error) {
	m.mu.RLock()
	if s, ok := m.cache[name]; ok {
		m.mu.RUnlock()
		return s, nil
	}
	m.mu.RUnlock()

	return m.store.Load(name)
}

// Restore loads all states from storage into the cache.
func (m *Manager) Restore() error {
	states, err := m.store.LoadAll()
	if err != nil {
		return fmt.Errorf("circuitstate: restore: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range states {
		m.cache[s.Name] = s
	}
	return nil
}

// Serialize exports all states as JSON.
func (m *Manager) Serialize() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]State, 0, len(m.cache))
	for _, s := range m.cache {
		states = append(states, s)
	}
	return json.Marshal(states)
}

// Count returns the number of tracked circuits.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cache)
}
