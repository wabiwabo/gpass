package registry

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInstanceNotFound = errors.New("service instance not found")
	ErrServiceNotFound  = errors.New("service not found")
)

// ServiceInstance represents a registered service instance.
type ServiceInstance struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Host          string            `json:"host"`
	Port          int               `json:"port"`
	Version       string            `json:"version"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	HealthURL     string            `json:"health_url"`
	RegisteredAt  time.Time         `json:"registered_at"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
	Status        string            `json:"status"` // UP, DOWN, DRAINING
}

// Registry manages service instances.
type Registry struct {
	instances map[string][]*ServiceInstance // name -> instances
	mu        sync.RWMutex
}

// New creates a new service registry.
func New() *Registry {
	return &Registry{
		instances: make(map[string][]*ServiceInstance),
	}
}

func copyInstance(inst *ServiceInstance) *ServiceInstance {
	cp := *inst
	if inst.Metadata != nil {
		cp.Metadata = make(map[string]string, len(inst.Metadata))
		for k, v := range inst.Metadata {
			cp.Metadata[k] = v
		}
	}
	return &cp
}

// Register adds a service instance and returns the assigned ID.
func (r *Registry) Register(instance ServiceInstance) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	instance.ID = uuid.New().String()
	instance.RegisteredAt = now
	instance.LastHeartbeat = now
	if instance.Status == "" {
		instance.Status = "UP"
	}

	r.instances[instance.Name] = append(r.instances[instance.Name], copyInstance(&instance))
	return instance.ID
}

// Deregister removes a service instance.
func (r *Registry) Deregister(name, instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, ok := r.instances[name]
	if !ok {
		return ErrServiceNotFound
	}

	for i, inst := range instances {
		if inst.ID == instanceID {
			r.instances[name] = append(instances[:i], instances[i+1:]...)
			if len(r.instances[name]) == 0 {
				delete(r.instances, name)
			}
			return nil
		}
	}
	return ErrInstanceNotFound
}

// Lookup returns healthy (UP) instances of a service.
func (r *Registry) Lookup(name string) []*ServiceInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ServiceInstance
	for _, inst := range r.instances[name] {
		if inst.Status == "UP" {
			result = append(result, copyInstance(inst))
		}
	}
	return result
}

// Heartbeat updates the last heartbeat time for an instance.
func (r *Registry) Heartbeat(name, instanceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, ok := r.instances[name]
	if !ok {
		return ErrServiceNotFound
	}

	for _, inst := range instances {
		if inst.ID == instanceID {
			inst.LastHeartbeat = time.Now().UTC()
			return nil
		}
	}
	return ErrInstanceNotFound
}

// MarkDown marks an instance as DOWN.
func (r *Registry) MarkDown(name, instanceID string) error {
	return r.setStatus(name, instanceID, "DOWN")
}

// MarkDraining marks an instance as DRAINING (won't receive new requests).
func (r *Registry) MarkDraining(name, instanceID string) error {
	return r.setStatus(name, instanceID, "DRAINING")
}

func (r *Registry) setStatus(name, instanceID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instances, ok := r.instances[name]
	if !ok {
		return ErrServiceNotFound
	}

	for _, inst := range instances {
		if inst.ID == instanceID {
			inst.Status = status
			return nil
		}
	}
	return ErrInstanceNotFound
}

// RemoveStale removes instances that haven't sent a heartbeat within maxAge.
func (r *Registry) RemoveStale(maxAge time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().UTC().Add(-maxAge)
	removed := 0

	for name, instances := range r.instances {
		var kept []*ServiceInstance
		for _, inst := range instances {
			if inst.LastHeartbeat.Before(cutoff) {
				removed++
			} else {
				kept = append(kept, inst)
			}
		}
		if len(kept) == 0 {
			delete(r.instances, name)
		} else {
			r.instances[name] = kept
		}
	}

	return removed
}

// All returns all registered services and their instances.
func (r *Registry) All() map[string][]*ServiceInstance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]*ServiceInstance, len(r.instances))
	for name, instances := range r.instances {
		copied := make([]*ServiceInstance, len(instances))
		for i, inst := range instances {
			copied[i] = copyInstance(inst)
		}
		result[name] = copied
	}
	return result
}

// Handler returns HTTP handlers for service registration.
func (r *Registry) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /internal/registry/register", r.handleRegister)
	mux.HandleFunc("POST /internal/registry/heartbeat", r.handleHeartbeat)
	mux.HandleFunc("GET /internal/registry/services", r.handleListServices)
	mux.HandleFunc("GET /internal/registry/services/{name}", r.handleGetService)

	return mux
}

type registerRequest struct {
	Name      string            `json:"name"`
	Host      string            `json:"host"`
	Port      int               `json:"port"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	HealthURL string            `json:"health_url"`
}

type heartbeatRequest struct {
	Name       string `json:"name"`
	InstanceID string `json:"instance_id"`
}

func (r *Registry) handleRegister(w http.ResponseWriter, req *http.Request) {
	var body registerRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if body.Name == "" || body.Host == "" || body.Port == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, host, and port are required"})
		return
	}

	id := r.Register(ServiceInstance{
		Name:      body.Name,
		Host:      body.Host,
		Port:      body.Port,
		Version:   body.Version,
		Metadata:  body.Metadata,
		HealthURL: body.HealthURL,
	})

	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

func (r *Registry) handleHeartbeat(w http.ResponseWriter, req *http.Request) {
	var body heartbeatRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	if err := r.Heartbeat(body.Name, body.InstanceID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Registry) handleListServices(w http.ResponseWriter, req *http.Request) {
	all := r.All()
	writeJSON(w, http.StatusOK, map[string]any{"services": all})
}

func (r *Registry) handleGetService(w http.ResponseWriter, req *http.Request) {
	name := req.PathValue("name")
	instances := r.Lookup(name)
	if instances == nil {
		instances = []*ServiceInstance{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"instances": instances})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
