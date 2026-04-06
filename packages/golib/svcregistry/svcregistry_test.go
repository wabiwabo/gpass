package svcregistry

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	r := New(30 * time.Second)
	if r.Count() != 0 {
		t.Errorf("Count = %d", r.Count())
	}
}

func TestNew_DefaultTTL(t *testing.T) {
	r := New(0)
	if r.heartbeatTTL != 30*time.Second {
		t.Errorf("heartbeatTTL = %v", r.heartbeatTTL)
	}
}

func TestRegister_Get(t *testing.T) {
	r := New(30 * time.Second)
	r.Register(Instance{
		ID:      "identity-1",
		Service: "identity",
		Address: "10.0.0.1",
		Port:    4001,
		Status:  StatusHealthy,
		Version: "1.0.0",
	})

	inst, ok := r.Get("identity-1")
	if !ok {
		t.Fatal("should find instance")
	}
	if inst.Service != "identity" {
		t.Errorf("Service = %q", inst.Service)
	}
	if inst.Address != "10.0.0.1" {
		t.Errorf("Address = %q", inst.Address)
	}
	if inst.RegisteredAt.IsZero() {
		t.Error("RegisteredAt should be set")
	}
	if inst.LastHeartbeat.IsZero() {
		t.Error("LastHeartbeat should be set")
	}
}

func TestDeregister(t *testing.T) {
	r := New(30 * time.Second)
	r.Register(Instance{ID: "inst-1", Service: "svc"})

	if !r.Deregister("inst-1") {
		t.Error("should return true")
	}
	if r.Count() != 0 {
		t.Error("should be empty after deregister")
	}
	if r.Deregister("nonexistent") {
		t.Error("should return false for nonexistent")
	}
}

func TestHeartbeat(t *testing.T) {
	r := New(30 * time.Second)
	r.Register(Instance{ID: "inst-1", Service: "svc", Status: StatusHealthy})

	inst1, _ := r.Get("inst-1")
	firstHB := inst1.LastHeartbeat

	time.Sleep(1 * time.Millisecond)

	if !r.Heartbeat("inst-1") {
		t.Error("should return true")
	}

	inst2, _ := r.Get("inst-1")
	if !inst2.LastHeartbeat.After(firstHB) {
		t.Error("heartbeat should update timestamp")
	}
}

func TestHeartbeat_NotFound(t *testing.T) {
	r := New(30 * time.Second)
	if r.Heartbeat("nonexistent") {
		t.Error("should return false")
	}
}

func TestHealthy(t *testing.T) {
	r := New(1 * time.Hour) // long TTL so heartbeats don't expire
	r.Register(Instance{ID: "h1", Service: "identity", Status: StatusHealthy})
	r.Register(Instance{ID: "h2", Service: "identity", Status: StatusHealthy})
	r.Register(Instance{ID: "u1", Service: "identity", Status: StatusUnhealthy})
	r.Register(Instance{ID: "d1", Service: "identity", Status: StatusDraining})
	r.Register(Instance{ID: "o1", Service: "other", Status: StatusHealthy})

	healthy := r.Healthy("identity")
	if len(healthy) != 2 {
		t.Errorf("healthy = %d, want 2", len(healthy))
	}
}

func TestHealthy_StaleHeartbeat(t *testing.T) {
	r := New(1 * time.Millisecond) // very short TTL
	r.Register(Instance{ID: "inst-1", Service: "svc", Status: StatusHealthy})

	time.Sleep(5 * time.Millisecond)

	healthy := r.Healthy("svc")
	if len(healthy) != 0 {
		t.Error("stale heartbeat should not be healthy")
	}
}

func TestAll(t *testing.T) {
	r := New(1 * time.Hour)
	r.Register(Instance{ID: "h1", Service: "identity", Status: StatusHealthy})
	r.Register(Instance{ID: "u1", Service: "identity", Status: StatusUnhealthy})
	r.Register(Instance{ID: "o1", Service: "other", Status: StatusHealthy})

	all := r.All("identity")
	if len(all) != 2 {
		t.Errorf("All = %d, want 2", len(all))
	}
}

func TestServices(t *testing.T) {
	r := New(30 * time.Second)
	r.Register(Instance{ID: "1", Service: "identity"})
	r.Register(Instance{ID: "2", Service: "garudacorp"})
	r.Register(Instance{ID: "3", Service: "identity"}) // duplicate

	services := r.Services()
	if len(services) != 2 {
		t.Fatalf("Services = %v", services)
	}
	// Sorted
	if services[0] != "garudacorp" || services[1] != "identity" {
		t.Errorf("Services = %v", services)
	}
}

func TestPurge(t *testing.T) {
	r := New(1 * time.Millisecond)
	r.Register(Instance{ID: "1", Service: "svc", Status: StatusHealthy})
	r.Register(Instance{ID: "2", Service: "svc", Status: StatusHealthy})

	time.Sleep(5 * time.Millisecond)

	removed := r.Purge()
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	if r.Count() != 0 {
		t.Errorf("Count = %d", r.Count())
	}
}

func TestIsHealthy(t *testing.T) {
	tests := []struct {
		name   string
		inst   Instance
		maxAge time.Duration
		want   bool
	}{
		{"healthy fresh", Instance{Status: StatusHealthy, LastHeartbeat: time.Now()}, 30 * time.Second, true},
		{"unhealthy", Instance{Status: StatusUnhealthy, LastHeartbeat: time.Now()}, 30 * time.Second, false},
		{"draining", Instance{Status: StatusDraining, LastHeartbeat: time.Now()}, 30 * time.Second, false},
		{"stale heartbeat", Instance{Status: StatusHealthy, LastHeartbeat: time.Now().Add(-1 * time.Hour)}, 30 * time.Second, false},
		{"no maxAge check", Instance{Status: StatusHealthy, LastHeartbeat: time.Now().Add(-1 * time.Hour)}, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.inst.IsHealthy(tt.maxAge); got != tt.want {
				t.Errorf("IsHealthy = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConcurrent(t *testing.T) {
	r := New(1 * time.Hour)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			r.Register(Instance{ID: "inst", Service: "svc", Status: StatusHealthy})
		}()
		go func() {
			defer wg.Done()
			r.Healthy("svc")
		}()
		go func() {
			defer wg.Done()
			r.Heartbeat("inst")
		}()
	}
	wg.Wait()
}

func TestRegister_UpdateExisting(t *testing.T) {
	r := New(30 * time.Second)
	r.Register(Instance{ID: "inst-1", Service: "svc", Version: "1.0"})
	r.Register(Instance{ID: "inst-1", Service: "svc", Version: "2.0"})

	inst, _ := r.Get("inst-1")
	if inst.Version != "2.0" {
		t.Errorf("Version = %q, want 2.0", inst.Version)
	}
	if r.Count() != 1 {
		t.Errorf("Count = %d, want 1", r.Count())
	}
}

func TestInstance_Metadata(t *testing.T) {
	r := New(30 * time.Second)
	r.Register(Instance{
		ID:       "inst-1",
		Service:  "svc",
		Metadata: map[string]string{"zone": "ap-southeast-1", "weight": "100"},
	})

	inst, _ := r.Get("inst-1")
	if inst.Metadata["zone"] != "ap-southeast-1" {
		t.Errorf("zone = %q", inst.Metadata["zone"])
	}
}
