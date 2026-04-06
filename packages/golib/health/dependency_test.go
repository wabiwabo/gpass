package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

func TestDependencyGraph_AddServiceAndDependenciesOf(t *testing.T) {
	g := NewDependencyGraph()
	g.AddService("bff", true, "redis", "keycloak")

	deps := g.DependenciesOf("bff")
	if len(deps) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(deps))
	}

	expected := []string{"keycloak", "redis"}
	sort.Strings(deps)
	for i, d := range deps {
		if d != expected[i] {
			t.Errorf("expected dep %s, got %s", expected[i], d)
		}
	}
}

func TestDependencyGraph_ImpactOf_TransitiveDependents(t *testing.T) {
	g := NewDependencyGraph()
	g.AddService("bff", true, "identity")
	g.AddService("identity", true, "postgresql")
	g.AddService("garudainfo", true, "identity")

	// If postgresql goes down, identity is affected, then bff and garudainfo
	impact := g.ImpactOf("postgresql")

	expected := []string{"bff", "garudainfo", "identity"}
	if len(impact) != len(expected) {
		t.Fatalf("expected %d impacted services, got %d: %v", len(expected), len(impact), impact)
	}

	sort.Strings(impact)
	for i, s := range impact {
		if s != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], s)
		}
	}
}

func TestDependencyGraph_ImpactOf_DirectOnly(t *testing.T) {
	g := NewDependencyGraph()
	g.AddService("bff", true, "redis")
	g.AddService("identity", true, "redis")

	impact := g.ImpactOf("redis")
	sort.Strings(impact)

	expected := []string{"bff", "identity"}
	if len(impact) != len(expected) {
		t.Fatalf("expected %d, got %d: %v", len(expected), len(impact), impact)
	}
	for i, s := range impact {
		if s != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], s)
		}
	}
}

func TestDependencyGraph_CriticalPath(t *testing.T) {
	g := NewDependencyGraph()
	g.AddService("bff", true, "redis")
	g.AddService("identity", true, "postgresql")
	g.AddService("garudacorp", false, "postgresql")
	g.AddService("postgresql", true)
	g.AddService("redis", true)

	critical := g.CriticalPath()

	expected := []string{"bff", "identity", "postgresql", "redis"}
	if len(critical) != len(expected) {
		t.Fatalf("expected %d critical services, got %d: %v", len(expected), len(critical), critical)
	}
	for i, s := range critical {
		if s != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], s)
		}
	}
}

func TestDependencyGraph_DefaultGarudaPassGraph(t *testing.T) {
	g := DefaultGarudaPassGraph()

	// Check expected services exist
	expectedServices := []string{
		"bff", "identity", "garudainfo", "garudacorp",
		"garudasign", "garudaportal", "garudaaudit",
		"postgresql", "redis", "keycloak", "dukcapil",
		"ahu", "oss", "signing-backend",
	}

	for _, s := range expectedServices {
		deps := g.DependenciesOf(s)
		if deps == nil && s != "postgresql" && s != "redis" && s != "keycloak" &&
			s != "dukcapil" && s != "ahu" && s != "oss" && s != "signing-backend" {
			t.Errorf("service %s should have dependencies", s)
		}
	}

	// Check critical services
	critical := g.CriticalPath()
	expectedCritical := []string{"bff", "garudainfo", "identity", "postgresql", "redis"}
	if len(critical) != len(expectedCritical) {
		t.Fatalf("expected %d critical services, got %d: %v", len(expectedCritical), len(critical), critical)
	}
	for i, s := range critical {
		if s != expectedCritical[i] {
			t.Errorf("expected critical %s, got %s", expectedCritical[i], s)
		}
	}

	// Check BFF dependencies
	bffDeps := g.DependenciesOf("bff")
	if len(bffDeps) != 7 {
		t.Errorf("expected bff to have 7 dependencies, got %d: %v", len(bffDeps), bffDeps)
	}

	// Check postgresql impact (many services depend on it)
	pgImpact := g.ImpactOf("postgresql")
	if len(pgImpact) < 5 {
		t.Errorf("expected postgresql to impact at least 5 services, got %d: %v", len(pgImpact), pgImpact)
	}
}

func TestDependencyGraph_Handler_ReturnsJSON(t *testing.T) {
	g := NewDependencyGraph()
	g.AddService("bff", true, "redis")
	g.AddService("redis", true)

	handler := g.Handler()

	req := httptest.NewRequest(http.MethodGet, "/dependencies", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("expected JSON content type, got %s", contentType)
	}

	var resp map[string]json.RawMessage
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, ok := resp["services"]; !ok {
		t.Error("missing 'services' field in response")
	}
	if _, ok := resp["critical"]; !ok {
		t.Error("missing 'critical' field in response")
	}

	var services []ServiceNode
	json.Unmarshal(resp["services"], &services)
	if len(services) != 2 {
		t.Errorf("expected 2 services, got %d", len(services))
	}
}

func TestDependencyGraph_UnknownServiceReturnsEmpty(t *testing.T) {
	g := NewDependencyGraph()
	g.AddService("bff", true, "redis")

	deps := g.DependenciesOf("nonexistent")
	if deps != nil {
		t.Errorf("expected nil for unknown service, got %v", deps)
	}

	impact := g.ImpactOf("nonexistent")
	if impact != nil {
		t.Errorf("expected nil for unknown service impact, got %v", impact)
	}
}

func TestDependencyGraph_CircularDependency(t *testing.T) {
	g := NewDependencyGraph()
	g.AddService("a", false, "b")
	g.AddService("b", false, "a")

	// Should not infinite loop; both are transitively impacted
	impact := g.ImpactOf("a")
	sort.Strings(impact)
	expected := []string{"a", "b"}
	if len(impact) != 2 {
		t.Fatalf("expected 2 impacted services, got %d: %v", len(impact), impact)
	}
	for i, s := range impact {
		if s != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], s)
		}
	}
}
