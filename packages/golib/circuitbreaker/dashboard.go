package circuitbreaker

import (
	"encoding/json"
	"net/http"
	"sync"
)

// Registry tracks all circuit breakers for observability.
type Registry struct {
	breakers map[string]*Breaker
	mu       sync.RWMutex
}

// GlobalRegistry is the default circuit breaker registry.
var GlobalRegistry = NewRegistry()

// NewRegistry creates a new circuit breaker registry.
func NewRegistry() *Registry {
	return &Registry{
		breakers: make(map[string]*Breaker),
	}
}

// Register adds a named circuit breaker to the registry.
func (r *Registry) Register(name string, breaker *Breaker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.breakers[name] = breaker
}

// Get retrieves a circuit breaker by name.
func (r *Registry) Get(name string) (*Breaker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.breakers[name]
	return b, ok
}

// BreakerStatus represents the observable status of a circuit breaker.
type BreakerStatus struct {
	Name         string `json:"name"`
	State        string `json:"state"`
	FailureCount int    `json:"failure_count"`
	Threshold    int    `json:"threshold"`
	LastFailure  string `json:"last_failure,omitempty"`
}

// Status returns the status of all registered circuit breakers.
func (r *Registry) Status() []BreakerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statuses := make([]BreakerStatus, 0, len(r.breakers))
	for name, b := range r.breakers {
		bs := BreakerStatus{
			Name:         name,
			State:        b.State(),
			FailureCount: b.FailureCount(),
			Threshold:    b.Threshold(),
		}
		if opened := b.OpenedAt(); !opened.IsZero() {
			bs.LastFailure = opened.Format("2006-01-02T15:04:05Z07:00")
		}
		statuses = append(statuses, bs)
	}
	return statuses
}

// Handler returns an HTTP handler that exposes circuit breaker status as JSON.
func (r *Registry) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		statuses := r.Status()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statuses)
	}
}
