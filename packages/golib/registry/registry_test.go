package registry

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegistry_RegisterAndLookup(t *testing.T) {
	r := New()
	id := r.Register(ServiceInstance{
		Name:    "auth-service",
		Host:    "10.0.0.1",
		Port:    8080,
		Version: "1.0.0",
	})

	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	instances := r.Lookup("auth-service")
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}
	if instances[0].ID != id {
		t.Errorf("expected ID %s, got %s", id, instances[0].ID)
	}
	if instances[0].Status != "UP" {
		t.Errorf("expected status UP, got %s", instances[0].Status)
	}
}

func TestRegistry_MultipleInstancesSameService(t *testing.T) {
	r := New()
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.2", Port: 8080})
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.3", Port: 8080})

	instances := r.Lookup("api")
	if len(instances) != 3 {
		t.Fatalf("expected 3 instances, got %d", len(instances))
	}
}

func TestRegistry_Deregister(t *testing.T) {
	r := New()
	id := r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})

	err := r.Deregister("api", id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instances := r.Lookup("api")
	if len(instances) != 0 {
		t.Errorf("expected 0 instances, got %d", len(instances))
	}
}

func TestRegistry_Deregister_NotFound(t *testing.T) {
	r := New()
	err := r.Deregister("nonexistent", "id-1")
	if err != ErrServiceNotFound {
		t.Errorf("expected ErrServiceNotFound, got %v", err)
	}

	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})
	err = r.Deregister("api", "nonexistent-id")
	if err != ErrInstanceNotFound {
		t.Errorf("expected ErrInstanceNotFound, got %v", err)
	}
}

func TestRegistry_Heartbeat(t *testing.T) {
	r := New()
	id := r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})

	// Wait a tiny bit to ensure time difference
	time.Sleep(time.Millisecond)

	err := r.Heartbeat("api", id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instances := r.Lookup("api")
	if len(instances) != 1 {
		t.Fatalf("expected 1 instance, got %d", len(instances))
	}

	if !instances[0].LastHeartbeat.After(instances[0].RegisteredAt) {
		t.Error("expected last heartbeat to be after registered time")
	}
}

func TestRegistry_MarkDown_ExcludesFromLookup(t *testing.T) {
	r := New()
	id := r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})

	err := r.MarkDown("api", id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instances := r.Lookup("api")
	if len(instances) != 0 {
		t.Errorf("expected 0 UP instances, got %d", len(instances))
	}
}

func TestRegistry_MarkDraining_ExcludesFromLookup(t *testing.T) {
	r := New()
	id := r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})

	err := r.MarkDraining("api", id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instances := r.Lookup("api")
	if len(instances) != 0 {
		t.Errorf("expected 0 UP instances, got %d", len(instances))
	}
}

func TestRegistry_RemoveStale(t *testing.T) {
	r := New()
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})

	// Manually set the heartbeat to the past
	r.mu.Lock()
	r.instances["api"][0].LastHeartbeat = time.Now().UTC().Add(-2 * time.Hour)
	r.mu.Unlock()

	removed := r.RemoveStale(1 * time.Hour)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}

	instances := r.Lookup("api")
	if len(instances) != 0 {
		t.Errorf("expected 0 instances, got %d", len(instances))
	}
}

func TestRegistry_RemoveStale_KeepsFresh(t *testing.T) {
	r := New()
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})

	removed := r.RemoveStale(1 * time.Hour)
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}

	instances := r.Lookup("api")
	if len(instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(instances))
	}
}

func TestRegistry_LookupReturnsOnlyUP(t *testing.T) {
	r := New()
	id1 := r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.2", Port: 8080})
	id3 := r.Register(ServiceInstance{Name: "api", Host: "10.0.0.3", Port: 8080})

	r.MarkDown("api", id1)
	r.MarkDraining("api", id3)

	instances := r.Lookup("api")
	if len(instances) != 1 {
		t.Fatalf("expected 1 UP instance, got %d", len(instances))
	}
	if instances[0].Host != "10.0.0.2" {
		t.Errorf("expected host 10.0.0.2, got %s", instances[0].Host)
	}
}

func TestRegistry_All(t *testing.T) {
	r := New()
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.2", Port: 8080})
	r.Register(ServiceInstance{Name: "auth", Host: "10.0.0.3", Port: 9090})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 services, got %d", len(all))
	}
	if len(all["api"]) != 2 {
		t.Errorf("expected 2 api instances, got %d", len(all["api"]))
	}
	if len(all["auth"]) != 1 {
		t.Errorf("expected 1 auth instance, got %d", len(all["auth"]))
	}
}

func TestRegistry_Handler_Register(t *testing.T) {
	r := New()
	handler := r.Handler()

	body, _ := json.Marshal(registerRequest{
		Name:    "api",
		Host:    "10.0.0.1",
		Port:    8080,
		Version: "1.0.0",
	})

	req := httptest.NewRequest(http.MethodPost, "/internal/registry/register", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("expected non-empty id")
	}

	// Verify it was registered
	instances := r.Lookup("api")
	if len(instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(instances))
	}
}

func TestRegistry_Handler_Heartbeat(t *testing.T) {
	r := New()
	id := r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})
	handler := r.Handler()

	body, _ := json.Marshal(heartbeatRequest{Name: "api", InstanceID: id})
	req := httptest.NewRequest(http.MethodPost, "/internal/registry/heartbeat", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegistry_Handler_ListServices(t *testing.T) {
	r := New()
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})
	r.Register(ServiceInstance{Name: "auth", Host: "10.0.0.2", Port: 9090})
	handler := r.Handler()

	req := httptest.NewRequest(http.MethodGet, "/internal/registry/services", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Services map[string][]ServiceInstance `json:"services"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(resp.Services))
	}
}

func TestRegistry_Handler_GetService(t *testing.T) {
	r := New()
	r.Register(ServiceInstance{Name: "api", Host: "10.0.0.1", Port: 8080})
	handler := r.Handler()

	req := httptest.NewRequest(http.MethodGet, "/internal/registry/services/api", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Instances []ServiceInstance `json:"instances"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Instances) != 1 {
		t.Errorf("expected 1 instance, got %d", len(resp.Instances))
	}
}
