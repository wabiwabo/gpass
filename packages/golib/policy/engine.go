// Package policy provides an Attribute-Based Access Control (ABAC) policy engine
// for fine-grained authorization beyond simple roles.
package policy

import (
	"fmt"
	"strings"
)

// Policy defines an access control policy.
type Policy struct {
	ID         string
	Name       string
	Effect     string // ALLOW, DENY
	Subjects   []string
	Resources  []string
	Actions    []string
	Conditions []Condition
}

// Condition is an additional policy constraint.
type Condition struct {
	Key      string      // e.g., "time", "ip_range", "auth_level", "environment"
	Operator string      // eq, neq, gt, lt, gte, lte, in, contains, matches
	Value    interface{} // expected value for comparison
}

// Request is an authorization request to be evaluated against policies.
type Request struct {
	Subject  string
	Resource string
	Action   string
	Context  map[string]interface{} // time, ip, auth_level, environment, etc.
}

// Engine evaluates policies against authorization requests.
type Engine struct {
	policies []*Policy
}

// New creates a new policy engine with the given policies.
func New(policies ...*Policy) *Engine {
	return &Engine{policies: policies}
}

// AddPolicy adds a policy to the engine.
func (e *Engine) AddPolicy(p *Policy) {
	e.policies = append(e.policies, p)
}

// Evaluate returns whether the request is allowed. DENY takes precedence over ALLOW.
// If no policies match, the request is denied by default.
func (e *Engine) Evaluate(req Request) (allowed bool, matchedPolicy string) {
	var allowMatch string
	hasAllow := false

	for _, p := range e.policies {
		if !policyMatchesRequest(p, req) {
			continue
		}

		// Check all conditions are satisfied.
		conditionsMet := true
		for _, cond := range p.Conditions {
			if !EvaluateCondition(cond, req.Context) {
				conditionsMet = false
				break
			}
		}
		if !conditionsMet {
			continue
		}

		// DENY takes immediate precedence.
		if p.Effect == "DENY" {
			return false, p.ID
		}

		if p.Effect == "ALLOW" && !hasAllow {
			hasAllow = true
			allowMatch = p.ID
		}
	}

	if hasAllow {
		return true, allowMatch
	}

	return false, ""
}

// policyMatchesRequest checks if the policy's subject, resource, and action patterns
// match the request.
func policyMatchesRequest(p *Policy, req Request) bool {
	if !matchAny(p.Subjects, req.Subject) {
		return false
	}
	if !matchAny(p.Resources, req.Resource) {
		return false
	}
	if !matchAny(p.Actions, req.Action) {
		return false
	}
	return true
}

// matchAny returns true if any pattern in the slice matches the value.
func matchAny(patterns []string, value string) bool {
	for _, p := range patterns {
		if Match(p, value) {
			return true
		}
	}
	return false
}

// Match checks if a pattern matches a value. Supports * wildcards:
//   - "*" matches everything
//   - "prefix:*" matches anything starting with "prefix:"
//   - "api:/v1/sign/*" matches "api:/v1/sign/certificates"
func Match(pattern, value string) bool {
	if pattern == "*" {
		return true
	}

	// No wildcard — exact match.
	if !strings.Contains(pattern, "*") {
		return pattern == value
	}

	// Split on * and check that parts appear in order.
	parts := strings.Split(pattern, "*")
	remaining := value
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(remaining, part)
		if idx == -1 {
			return false
		}
		// First part must be a prefix.
		if i == 0 && idx != 0 {
			return false
		}
		remaining = remaining[idx+len(part):]
	}
	// If pattern doesn't end with *, the value must be fully consumed.
	if !strings.HasSuffix(pattern, "*") && remaining != "" {
		return false
	}
	return true
}

// EvaluateCondition checks if a condition is satisfied given the request context.
func EvaluateCondition(cond Condition, ctx map[string]interface{}) bool {
	if ctx == nil {
		return false
	}

	ctxVal, ok := ctx[cond.Key]
	if !ok {
		return false
	}

	switch cond.Operator {
	case "eq":
		return fmt.Sprintf("%v", ctxVal) == fmt.Sprintf("%v", cond.Value)
	case "neq":
		return fmt.Sprintf("%v", ctxVal) != fmt.Sprintf("%v", cond.Value)
	case "gt":
		return toFloat(ctxVal) > toFloat(cond.Value)
	case "lt":
		return toFloat(ctxVal) < toFloat(cond.Value)
	case "gte":
		return toFloat(ctxVal) >= toFloat(cond.Value)
	case "lte":
		return toFloat(ctxVal) <= toFloat(cond.Value)
	case "in":
		return evalIn(ctxVal, cond.Value)
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", ctxVal), fmt.Sprintf("%v", cond.Value))
	case "matches":
		return Match(fmt.Sprintf("%v", cond.Value), fmt.Sprintf("%v", ctxVal))
	default:
		return false
	}
}

// toFloat converts a numeric interface value to float64 for comparison.
func toFloat(v interface{}) float64 {
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	case float32:
		return float64(n)
	default:
		return 0
	}
}

// evalIn checks if ctxVal is contained in the condition value (expected to be a slice).
func evalIn(ctxVal, condVal interface{}) bool {
	s := fmt.Sprintf("%v", ctxVal)
	switch list := condVal.(type) {
	case []string:
		for _, item := range list {
			if item == s {
				return true
			}
		}
	case []interface{}:
		for _, item := range list {
			if fmt.Sprintf("%v", item) == s {
				return true
			}
		}
	}
	return false
}

// DefaultGarudaPassPolicies returns the standard platform policies for GarudaPass.
func DefaultGarudaPassPolicies() []*Policy {
	return []*Policy{
		{
			ID:        "gp-admin-full",
			Name:      "Admin Full Access",
			Effect:    "ALLOW",
			Subjects:  []string{"admin"},
			Resources: []string{"*"},
			Actions:   []string{"*"},
		},
		{
			ID:        "gp-user-own-data",
			Name:      "User Own Data Access",
			Effect:    "ALLOW",
			Subjects:  []string{"user:*"},
			Resources: []string{"data:users:self"},
			Actions:   []string{"read", "write"},
		},
		{
			ID:        "gp-user-deny-other",
			Name:      "Deny Other User Data",
			Effect:    "DENY",
			Subjects:  []string{"user:*"},
			Resources: []string{"data:users:other"},
			Actions:   []string{"*"},
		},
		{
			ID:        "gp-service-internal",
			Name:      "Service Internal API Access",
			Effect:    "ALLOW",
			Subjects:  []string{"service:*"},
			Resources: []string{"api:/internal/*"},
			Actions:   []string{"read", "write"},
		},
		{
			ID:        "gp-signing-auth",
			Name:      "Signing Requires Strong Auth",
			Effect:    "ALLOW",
			Subjects:  []string{"*"},
			Resources: []string{"api:/v1/sign/*"},
			Actions:   []string{"sign"},
			Conditions: []Condition{
				{Key: "auth_level", Operator: "gte", Value: 2},
			},
		},
		{
			ID:        "gp-delete-auth",
			Name:      "Data Deletion Requires Strong Auth",
			Effect:    "ALLOW",
			Subjects:  []string{"*"},
			Resources: []string{"data:*"},
			Actions:   []string{"delete"},
			Conditions: []Condition{
				{Key: "auth_level", Operator: "gte", Value: 2},
			},
		},
		{
			ID:        "gp-audit-admin",
			Name:      "Audit Read Admin Only",
			Effect:    "ALLOW",
			Subjects:  []string{"admin"},
			Resources: []string{"audit:*"},
			Actions:   []string{"read"},
		},
	}
}
