package permission

import (
	"strings"
	"sync"
)

// Scope represents a permission scope in dot-notation: "resource.action".
// Examples: "users.read", "users.write", "admin.*", "*"
type Scope string

// Match checks if this scope matches the required scope.
// Supports wildcards: "admin.*" matches "admin.read", "admin.write".
// "*" matches everything.
func (s Scope) Match(required Scope) bool {
	if s == "*" || required == "*" {
		return true
	}
	if s == required {
		return true
	}

	// Check wildcard patterns.
	sStr := string(s)
	rStr := string(required)

	// "admin.*" matches "admin.read"
	if strings.HasSuffix(sStr, ".*") {
		prefix := strings.TrimSuffix(sStr, ".*")
		return strings.HasPrefix(rStr, prefix+".")
	}

	return false
}

// Checker validates permissions against a set of granted scopes.
type Checker struct {
	mu     sync.RWMutex
	grants map[string][]Scope // subject → scopes
}

// NewChecker creates a new permission checker.
func NewChecker() *Checker {
	return &Checker{
		grants: make(map[string][]Scope),
	}
}

// Grant assigns scopes to a subject (user, role, or service).
func (c *Checker) Grant(subject string, scopes ...Scope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.grants[subject] = append(c.grants[subject], scopes...)
}

// Revoke removes all scopes for a subject.
func (c *Checker) Revoke(subject string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.grants, subject)
}

// RevokeScope removes a specific scope from a subject.
func (c *Checker) RevokeScope(subject string, scope Scope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	scopes := c.grants[subject]
	filtered := make([]Scope, 0, len(scopes))
	for _, s := range scopes {
		if s != scope {
			filtered = append(filtered, s)
		}
	}
	c.grants[subject] = filtered
}

// HasPermission checks if a subject has the required scope.
func (c *Checker) HasPermission(subject string, required Scope) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	scopes := c.grants[subject]
	for _, s := range scopes {
		if s.Match(required) {
			return true
		}
	}
	return false
}

// HasAny checks if a subject has any of the required scopes.
func (c *Checker) HasAny(subject string, required ...Scope) bool {
	for _, r := range required {
		if c.HasPermission(subject, r) {
			return true
		}
	}
	return false
}

// HasAll checks if a subject has all of the required scopes.
func (c *Checker) HasAll(subject string, required ...Scope) bool {
	for _, r := range required {
		if !c.HasPermission(subject, r) {
			return false
		}
	}
	return true
}

// Scopes returns all scopes granted to a subject.
func (c *Checker) Scopes(subject string) []Scope {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Scope, len(c.grants[subject]))
	copy(result, c.grants[subject])
	return result
}

// Subjects returns all subjects with grants.
func (c *Checker) Subjects() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, 0, len(c.grants))
	for k := range c.grants {
		result = append(result, k)
	}
	return result
}

// Role represents a named set of permissions that can be assigned to subjects.
type Role struct {
	Name   string
	Scopes []Scope
}

// RoleHierarchy manages role-based permission inheritance.
type RoleHierarchy struct {
	mu    sync.RWMutex
	roles map[string]*Role
	// parent → children mapping for inheritance.
	parents map[string]string // role → parent role
}

// NewRoleHierarchy creates a new role hierarchy.
func NewRoleHierarchy() *RoleHierarchy {
	return &RoleHierarchy{
		roles:   make(map[string]*Role),
		parents: make(map[string]string),
	}
}

// Define creates a role with the given scopes.
func (h *RoleHierarchy) Define(name string, scopes ...Scope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.roles[name] = &Role{Name: name, Scopes: scopes}
}

// Inherit makes child role inherit all scopes from parent role.
func (h *RoleHierarchy) Inherit(child, parent string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.parents[child] = parent
}

// ResolveScopes returns all scopes for a role including inherited ones.
func (h *RoleHierarchy) ResolveScopes(roleName string) []Scope {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.resolveRecursive(roleName, make(map[string]bool))
}

func (h *RoleHierarchy) resolveRecursive(roleName string, visited map[string]bool) []Scope {
	if visited[roleName] {
		return nil // prevent cycles
	}
	visited[roleName] = true

	var scopes []Scope
	if role, ok := h.roles[roleName]; ok {
		scopes = append(scopes, role.Scopes...)
	}

	if parent, ok := h.parents[roleName]; ok {
		scopes = append(scopes, h.resolveRecursive(parent, visited)...)
	}

	return scopes
}
