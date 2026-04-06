package healthgraph

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
)

func TestAddNode(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("cache", "db")
	g.AddNode("api", "db", "cache")

	status := g.Status()
	if len(status) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(status))
	}
	if status["api"].Status != "healthy" {
		t.Errorf("expected healthy, got %s", status["api"].Status)
	}
	if len(status["api"].Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(status["api"].Dependencies))
	}
	if len(status["db"].Dependencies) != 0 {
		t.Errorf("expected 0 dependencies for db, got %d", len(status["db"].Dependencies))
	}
}

func TestUpdateStatus(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")

	g.UpdateStatus("db", "unhealthy")
	status := g.Status()
	if status["db"].Status != "unhealthy" {
		t.Errorf("expected unhealthy, got %s", status["db"].Status)
	}

	g.UpdateStatus("db", "degraded")
	status = g.Status()
	if status["db"].Status != "degraded" {
		t.Errorf("expected degraded, got %s", status["db"].Status)
	}

	// Updating nonexistent node should not panic.
	g.UpdateStatus("nonexistent", "unhealthy")
}

func TestImpactAnalysis_Direct(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("cache", "db")
	g.AddNode("api", "db")
	g.AddNode("worker", "cache")

	affected := g.ImpactAnalysis("db")
	sort.Strings(affected)

	// db failure should affect cache, api, and worker (worker depends on cache which depends on db).
	expected := []string{"api", "cache", "worker"}
	if len(affected) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, affected)
	}
	for i, name := range expected {
		if affected[i] != name {
			t.Errorf("expected %s at index %d, got %s", name, i, affected[i])
		}
	}
}

func TestImpactAnalysis_Transitive(t *testing.T) {
	// A -> B -> C chain: A fails, B and C should be affected.
	g := NewGraph()
	g.AddNode("A")
	g.AddNode("B", "A")
	g.AddNode("C", "B")

	affected := g.ImpactAnalysis("A")
	sort.Strings(affected)

	expected := []string{"B", "C"}
	if len(affected) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, affected)
	}
	for i, name := range expected {
		if affected[i] != name {
			t.Errorf("expected %s at index %d, got %s", name, i, affected[i])
		}
	}
}

func TestImpactAnalysis_NoImpact(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("cache")
	g.AddNode("api", "db")

	// cache has no dependents.
	affected := g.ImpactAnalysis("cache")
	if len(affected) != 0 {
		t.Errorf("expected no impact, got %v", affected)
	}
}

func TestCriticalPath(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("cache")
	g.AddNode("api", "db")          // single dependency -> db is critical
	g.AddNode("worker", "db", "cache") // two dependencies -> neither is critical from this node

	critical := g.CriticalPath()
	if len(critical) != 1 {
		t.Fatalf("expected 1 critical node, got %v", critical)
	}
	if critical[0] != "db" {
		t.Errorf("expected db as critical, got %s", critical[0])
	}
}

func TestTopologicalOrder(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("cache", "db")
	g.AddNode("api", "db", "cache")

	order := g.TopologicalOrder()
	if len(order) != 3 {
		t.Fatalf("expected 3 nodes in order, got %d", len(order))
	}

	// Build position map.
	pos := make(map[string]int, len(order))
	for i, name := range order {
		pos[name] = i
	}

	// db must come before cache and api.
	if pos["db"] >= pos["cache"] {
		t.Errorf("db should come before cache: %v", order)
	}
	if pos["db"] >= pos["api"] {
		t.Errorf("db should come before api: %v", order)
	}
	// cache must come before api.
	if pos["cache"] >= pos["api"] {
		t.Errorf("cache should come before api: %v", order)
	}
}

func TestOverallStatus_Healthy(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("cache")
	g.AddNode("api", "db", "cache")

	if s := g.OverallStatus(); s != "healthy" {
		t.Errorf("expected healthy, got %s", s)
	}
}

func TestOverallStatus_Degraded(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("cache")
	g.AddNode("api", "db", "cache") // two deps, neither is sole dependency for api

	g.UpdateStatus("cache", "unhealthy")

	if s := g.OverallStatus(); s != "degraded" {
		t.Errorf("expected degraded, got %s", s)
	}
}

func TestOverallStatus_Unhealthy(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("api", "db") // single dependency -> db is critical

	g.UpdateStatus("db", "unhealthy")

	if s := g.OverallStatus(); s != "unhealthy" {
		t.Errorf("expected unhealthy, got %s", s)
	}
}

func TestHandler_JSON(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("api", "db")

	req := httptest.NewRequest(http.MethodGet, "/health/graph", nil)
	rec := httptest.NewRecorder()

	g.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var resp graphResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if resp.OverallStatus != "healthy" {
		t.Errorf("expected healthy, got %s", resp.OverallStatus)
	}
	if len(resp.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(resp.Nodes))
	}

	// Test unhealthy returns 503.
	g.UpdateStatus("db", "unhealthy")
	rec2 := httptest.NewRecorder()
	g.Handler().ServeHTTP(rec2, req)
	if rec2.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec2.Code)
	}
}

func TestGraph_ConcurrentAccess(t *testing.T) {
	g := NewGraph()
	g.AddNode("db")
	g.AddNode("cache", "db")
	g.AddNode("api", "db", "cache")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(5)
		go func() {
			defer wg.Done()
			g.UpdateStatus("db", "unhealthy")
		}()
		go func() {
			defer wg.Done()
			g.UpdateStatus("db", "healthy")
		}()
		go func() {
			defer wg.Done()
			g.ImpactAnalysis("db")
		}()
		go func() {
			defer wg.Done()
			g.OverallStatus()
		}()
		go func() {
			defer wg.Done()
			g.Status()
		}()
	}
	wg.Wait()
}
