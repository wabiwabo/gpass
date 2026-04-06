package health

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
)

// ServiceNode represents a service and its dependency relationships.
type ServiceNode struct {
	Name       string   `json:"name"`
	DependsOn  []string `json:"depends_on"`
	DependedBy []string `json:"depended_by"`
	Critical   bool     `json:"critical"`
}

// DependencyGraph tracks service dependencies for health and impact analysis.
type DependencyGraph struct {
	services map[string]*ServiceNode
	mu       sync.RWMutex
}

// NewDependencyGraph creates a new empty dependency graph.
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		services: make(map[string]*ServiceNode),
	}
}

// AddService registers a service and its dependencies.
// If a dependency is not yet registered, it is created as a non-critical service.
func (g *DependencyGraph) AddService(name string, critical bool, dependsOn ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	node, exists := g.services[name]
	if !exists {
		node = &ServiceNode{
			Name:       name,
			DependsOn:  make([]string, 0),
			DependedBy: make([]string, 0),
		}
		g.services[name] = node
	}
	node.Critical = critical
	node.DependsOn = append(node.DependsOn[:0], dependsOn...)

	// Ensure all dependencies exist and update their DependedBy
	for _, dep := range dependsOn {
		depNode, ok := g.services[dep]
		if !ok {
			depNode = &ServiceNode{
				Name:       dep,
				DependsOn:  make([]string, 0),
				DependedBy: make([]string, 0),
			}
			g.services[dep] = depNode
		}
		if !contains(depNode.DependedBy, name) {
			depNode.DependedBy = append(depNode.DependedBy, name)
		}
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ImpactOf returns all services transitively affected if the given service goes down.
func (g *DependencyGraph) ImpactOf(serviceName string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.services[serviceName]
	if !ok {
		return nil
	}

	visited := make(map[string]bool)
	var result []string
	g.collectDependents(node, visited, &result)

	sort.Strings(result)
	return result
}

func (g *DependencyGraph) collectDependents(node *ServiceNode, visited map[string]bool, result *[]string) {
	for _, dep := range node.DependedBy {
		if visited[dep] {
			continue
		}
		visited[dep] = true
		*result = append(*result, dep)
		if depNode, ok := g.services[dep]; ok {
			g.collectDependents(depNode, visited, result)
		}
	}
}

// DependenciesOf returns all services that must be healthy for this service.
func (g *DependencyGraph) DependenciesOf(serviceName string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	node, ok := g.services[serviceName]
	if !ok {
		return nil
	}

	result := make([]string, len(node.DependsOn))
	copy(result, node.DependsOn)
	sort.Strings(result)
	return result
}

// CriticalPath returns the list of critical services, sorted by name.
func (g *DependencyGraph) CriticalPath() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var result []string
	for _, node := range g.services {
		if node.Critical {
			result = append(result, node.Name)
		}
	}
	sort.Strings(result)
	return result
}

// Handler returns an HTTP handler that serves the dependency graph as JSON.
func (g *DependencyGraph) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g.mu.RLock()
		nodes := make([]*ServiceNode, 0, len(g.services))
		for _, node := range g.services {
			cp := &ServiceNode{
				Name:       node.Name,
				Critical:   node.Critical,
				DependsOn:  make([]string, len(node.DependsOn)),
				DependedBy: make([]string, len(node.DependedBy)),
			}
			copy(cp.DependsOn, node.DependsOn)
			copy(cp.DependedBy, node.DependedBy)
			nodes = append(nodes, cp)
		}
		g.mu.RUnlock()

		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Name < nodes[j].Name
		})

		resp := map[string]any{
			"services": nodes,
			"critical": g.CriticalPath(),
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}

// DefaultGarudaPassGraph returns the standard GarudaPass dependency graph.
func DefaultGarudaPassGraph() *DependencyGraph {
	g := NewDependencyGraph()

	g.AddService("bff", true, "redis", "keycloak", "identity", "garudainfo", "garudacorp", "garudasign", "garudaportal")
	g.AddService("identity", true, "postgresql", "redis", "keycloak", "dukcapil")
	g.AddService("garudainfo", true, "postgresql", "identity")
	g.AddService("garudacorp", false, "postgresql", "ahu", "oss", "identity")
	g.AddService("garudasign", false, "postgresql", "signing-backend")
	g.AddService("garudaportal", false, "postgresql", "redis")
	g.AddService("garudaaudit", false, "postgresql")
	g.AddService("postgresql", true)
	g.AddService("redis", true)

	return g
}
