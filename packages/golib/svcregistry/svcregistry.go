// Package svcregistry provides a service registry for tracking
// microservice instances, their health status, and metadata.
// Used for service discovery and routing in the GarudaPass mesh.
package svcregistry

import (
	"sort"
	"sync"
	"time"
)

// Status represents instance health status.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
	StatusDraining  Status = "draining"
)

// Instance represents a service instance.
type Instance struct {
	ID        string            `json:"id"`
	Service   string            `json:"service"`
	Address   string            `json:"address"`
	Port      int               `json:"port"`
	Status    Status            `json:"status"`
	Version   string            `json:"version,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	RegisteredAt time.Time      `json:"registered_at"`
	LastHeartbeat time.Time     `json:"last_heartbeat"`
}

// IsHealthy returns true if the instance is healthy and heartbeat is fresh.
func (i Instance) IsHealthy(maxAge time.Duration) bool {
	if i.Status != StatusHealthy {
		return false
	}
	if maxAge > 0 && time.Since(i.LastHeartbeat) > maxAge {
		return false
	}
	return true
}

// Registry manages service instances.
type Registry struct {
	mu        sync.RWMutex
	instances map[string]Instance // keyed by ID
	heartbeatTTL time.Duration
}

// New creates a service registry.
func New(heartbeatTTL time.Duration) *Registry {
	if heartbeatTTL <= 0 {
		heartbeatTTL = 30 * time.Second
	}
	return &Registry{
		instances:    make(map[string]Instance),
		heartbeatTTL: heartbeatTTL,
	}
}

// Register adds or updates a service instance.
func (r *Registry) Register(inst Instance) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if inst.RegisteredAt.IsZero() {
		inst.RegisteredAt = time.Now().UTC()
	}
	inst.LastHeartbeat = time.Now().UTC()
	r.instances[inst.ID] = inst
}

// Deregister removes a service instance.
func (r *Registry) Deregister(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.instances[id]; ok {
		delete(r.instances, id)
		return true
	}
	return false
}

// Heartbeat updates the last heartbeat time.
func (r *Registry) Heartbeat(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	inst, ok := r.instances[id]
	if !ok {
		return false
	}
	inst.LastHeartbeat = time.Now().UTC()
	r.instances[id] = inst
	return true
}

// Get returns an instance by ID.
func (r *Registry) Get(id string) (Instance, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	inst, ok := r.instances[id]
	return inst, ok
}

// Healthy returns all healthy instances for a service.
func (r *Registry) Healthy(service string) []Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Instance
	for _, inst := range r.instances {
		if inst.Service == service && inst.IsHealthy(r.heartbeatTTL) {
			result = append(result, inst)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// All returns all instances for a service.
func (r *Registry) All(service string) []Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Instance
	for _, inst := range r.instances {
		if inst.Service == service {
			result = append(result, inst)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// Services returns all unique service names.
func (r *Registry) Services() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	for _, inst := range r.instances {
		seen[inst.Service] = true
	}

	services := make([]string, 0, len(seen))
	for s := range seen {
		services = append(services, s)
	}
	sort.Strings(services)
	return services
}

// Count returns total number of instances.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.instances)
}

// Purge removes unhealthy/stale instances. Returns count removed.
func (r *Registry) Purge() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	removed := 0
	for id, inst := range r.instances {
		if !inst.IsHealthy(r.heartbeatTTL) {
			delete(r.instances, id)
			removed++
		}
	}
	return removed
}
