// Package reqscope provides request-scoped value management for
// HTTP handlers. Stores typed values that live for the duration of
// a single request, enabling clean dependency passing without
// global state.
package reqscope

import (
	"context"
	"net/http"
	"sync"
)

type contextKey struct{}

// Scope holds request-scoped values.
type Scope struct {
	mu     sync.RWMutex
	values map[string]interface{}
}

// FromContext retrieves the request scope from context.
func FromContext(ctx context.Context) *Scope {
	s, ok := ctx.Value(contextKey{}).(*Scope)
	if !ok {
		return nil
	}
	return s
}

// Middleware creates a new scope for each request and stores it in context.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope := &Scope{values: make(map[string]interface{})}
		ctx := context.WithValue(r.Context(), contextKey{}, scope)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Set stores a value in the scope.
func (s *Scope) Set(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[key] = value
}

// Get retrieves a value from the scope.
func (s *Scope) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.values[key]
	return v, ok
}

// GetString retrieves a string value.
func (s *Scope) GetString(key string) (string, bool) {
	v, ok := s.Get(key)
	if !ok {
		return "", false
	}
	str, ok := v.(string)
	return str, ok
}

// GetInt retrieves an int value.
func (s *Scope) GetInt(key string) (int, bool) {
	v, ok := s.Get(key)
	if !ok {
		return 0, false
	}
	i, ok := v.(int)
	return i, ok
}

// Has checks if a key exists.
func (s *Scope) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.values[key]
	return ok
}

// Keys returns all keys in the scope.
func (s *Scope) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.values))
	for k := range s.values {
		keys = append(keys, k)
	}
	return keys
}

// Len returns the number of values.
func (s *Scope) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.values)
}
