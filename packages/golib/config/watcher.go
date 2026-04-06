package config

import (
	"sync"
	"sync/atomic"
)

// Value holds a configuration value that can be updated at runtime.
// Uses atomic.Value for lock-free reads.
type Value[T any] struct {
	val      atomic.Value
	onChange []func(old, new T)
	mu       sync.Mutex
}

// NewValue creates a config value with an initial value.
func NewValue[T any](initial T) *Value[T] {
	v := &Value[T]{}
	v.val.Store(initial)
	return v
}

// Get returns the current value (lock-free read).
func (v *Value[T]) Get() T {
	return v.val.Load().(T)
}

// Set updates the value and notifies listeners.
func (v *Value[T]) Set(newVal T) {
	v.mu.Lock()
	old := v.val.Load().(T)
	v.val.Store(newVal)
	listeners := make([]func(old, new T), len(v.onChange))
	copy(listeners, v.onChange)
	v.mu.Unlock()

	for _, fn := range listeners {
		fn(old, newVal)
	}
}

// OnChange registers a callback for value changes.
func (v *Value[T]) OnChange(fn func(old, new T)) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onChange = append(v.onChange, fn)
}

// Snapshot holds a point-in-time copy of multiple config values.
type Snapshot struct {
	values map[string]interface{}
	mu     sync.RWMutex
}

// NewSnapshot creates a new empty Snapshot.
func NewSnapshot() *Snapshot {
	return &Snapshot{
		values: make(map[string]interface{}),
	}
}

// Set stores a value in the snapshot.
func (s *Snapshot) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}

// Get retrieves a value from the snapshot.
func (s *Snapshot) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.values[key]
	return v, ok
}

// GetString returns a string value or the default if key is missing or not a string.
func (s *Snapshot) GetString(key string, defaultVal string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.values[key]
	if !ok {
		return defaultVal
	}
	str, ok := v.(string)
	if !ok {
		return defaultVal
	}
	return str
}

// GetInt returns an int value or the default if key is missing or not an int.
func (s *Snapshot) GetInt(key string, defaultVal int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.values[key]
	if !ok {
		return defaultVal
	}
	i, ok := v.(int)
	if !ok {
		return defaultVal
	}
	return i
}

// GetBool returns a bool value or the default if key is missing or not a bool.
func (s *Snapshot) GetBool(key string, defaultVal bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.values[key]
	if !ok {
		return defaultVal
	}
	b, ok := v.(bool)
	if !ok {
		return defaultVal
	}
	return b
}
