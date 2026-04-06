package healthgraph

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Node represents a service in the dependency graph.
type Node struct {
	Name         string   `json:"name"`
	Status       string   `json:"status"`
	Dependencies []string `json:"dependencies,omitempty"`
	LastCheck    time.Time `json:"last_check"`
}

// Graph models service dependencies as a directed graph for cascade failure analysis.
// All methods are safe for concurrent use.
type Graph struct {
	mu    sync.RWMutex
	nodes map[string]Node
}

// NewGraph creates an empty dependency graph.
func NewGraph() *Graph {
	return &Graph{
		nodes: make(map[string]Node),
	}
}

// AddNode registers a service node with its dependencies.
// The node starts with "healthy" status.
func (g *Graph) AddNode(name string, deps ...string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.nodes[name] = Node{
		Name:         name,
		Status:       "healthy",
		Dependencies: deps,
		LastCheck:    time.Now(),
	}
}

// UpdateStatus sets the health status for a named node.
// Valid statuses are "healthy", "degraded", and "unhealthy".
func (g *Graph) UpdateStatus(name string, status string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if n, ok := g.nodes[name]; ok {
		n.Status = status
		n.LastCheck = time.Now()
		g.nodes[name] = n
	}
}

// ImpactAnalysis returns all nodes transitively affected by the failure of the
// given node. A node is affected if it directly or indirectly depends on the
// failed node.
func (g *Graph) ImpactAnalysis(failedNode string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	affected := make(map[string]bool)
	g.findDependents(failedNode, affected)

	result := make([]string, 0, len(affected))
	for name := range affected {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

// findDependents recursively finds all nodes that depend on the target node.
func (g *Graph) findDependents(target string, affected map[string]bool) {
	for name, node := range g.nodes {
		if affected[name] {
			continue
		}
		for _, dep := range node.Dependencies {
			if dep == target || affected[dep] {
				affected[name] = true
				g.findDependents(name, affected)
				break
			}
		}
	}
}

// CriticalPath returns nodes that are single points of failure — nodes that,
// if they fail, would leave at least one dependent with no healthy alternative
// dependency.
func (g *Graph) CriticalPath() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	candidates := make(map[string]bool)

	for _, node := range g.nodes {
		if len(node.Dependencies) == 1 {
			// This node has a single dependency — that dependency is critical.
			candidates[node.Dependencies[0]] = true
		}
	}

	// Only include candidates that actually exist as nodes.
	result := make([]string, 0, len(candidates))
	for name := range candidates {
		if _, ok := g.nodes[name]; ok {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

// TopologicalOrder returns a valid startup order that respects dependencies.
// Dependencies appear before the nodes that depend on them.
func (g *Graph) TopologicalOrder() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	var order []string

	// Sort node names for deterministic output.
	names := make([]string, 0, len(g.nodes))
	for name := range g.nodes {
		names = append(names, name)
	}
	sort.Strings(names)

	var visit func(name string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true

		node, ok := g.nodes[name]
		if !ok {
			return
		}
		// Visit dependencies first.
		deps := make([]string, len(node.Dependencies))
		copy(deps, node.Dependencies)
		sort.Strings(deps)
		for _, dep := range deps {
			visit(dep)
		}
		order = append(order, name)
	}

	for _, name := range names {
		visit(name)
	}
	return order
}

// Status returns a snapshot of all nodes in the graph.
func (g *Graph) Status() map[string]Node {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]Node, len(g.nodes))
	for k, v := range g.nodes {
		result[k] = v
	}
	return result
}

// OverallStatus returns the aggregate health of the graph:
//   - "healthy" if all nodes are healthy
//   - "unhealthy" if any node on the critical path is not healthy
//   - "degraded" if some nodes are unhealthy but no critical path is affected
func (g *Graph) OverallStatus() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	allHealthy := true
	statusMap := make(map[string]string, len(g.nodes))

	for name, node := range g.nodes {
		statusMap[name] = node.Status
		if node.Status != "healthy" {
			allHealthy = false
		}
	}

	if allHealthy {
		return "healthy"
	}

	// Check if any critical path node is unhealthy.
	// Build critical set (nodes that are sole dependencies).
	for _, node := range g.nodes {
		if len(node.Dependencies) == 1 {
			dep := node.Dependencies[0]
			if s, ok := statusMap[dep]; ok && s != "healthy" {
				return "unhealthy"
			}
		}
	}

	return "degraded"
}

type graphResponse struct {
	OverallStatus string          `json:"overall_status"`
	Nodes         map[string]Node `json:"nodes"`
	CriticalPath  []string        `json:"critical_path"`
}

// Handler returns an http.HandlerFunc that serves the graph status as JSON.
func (g *Graph) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := graphResponse{
			OverallStatus: g.OverallStatus(),
			Nodes:         g.Status(),
			CriticalPath:  g.CriticalPath(),
		}

		w.Header().Set("Content-Type", "application/json")
		if resp.OverallStatus != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(resp)
	}
}
