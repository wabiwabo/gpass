package policy

import (
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"*", "anything", true},
		{"admin", "admin", true},
		{"admin", "user", false},
		{"user:*", "user:123", true},
		{"user:*", "admin", false},
		{"api:/v1/sign/*", "api:/v1/sign/certificates", true},
		{"api:/v1/sign/*", "api:/v1/verify/id", false},
		{"data:users:*", "data:users:self", true},
		{"data:users:*", "data:orgs:1", false},
		{"*internal*", "api:/internal/health", true},
		{"exact", "exactplus", false},
		{"pre*suf", "pre-middle-suf", true},
		{"pre*suf", "pre-middle-suffix", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			got := Match(tt.pattern, tt.value)
			if got != tt.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}

func TestEvaluateCondition(t *testing.T) {
	tests := []struct {
		name string
		cond Condition
		ctx  map[string]interface{}
		want bool
	}{
		{
			name: "eq match",
			cond: Condition{Key: "environment", Operator: "eq", Value: "production"},
			ctx:  map[string]interface{}{"environment": "production"},
			want: true,
		},
		{
			name: "eq no match",
			cond: Condition{Key: "environment", Operator: "eq", Value: "production"},
			ctx:  map[string]interface{}{"environment": "staging"},
			want: false,
		},
		{
			name: "neq",
			cond: Condition{Key: "environment", Operator: "neq", Value: "production"},
			ctx:  map[string]interface{}{"environment": "staging"},
			want: true,
		},
		{
			name: "gte match",
			cond: Condition{Key: "auth_level", Operator: "gte", Value: 2},
			ctx:  map[string]interface{}{"auth_level": 3},
			want: true,
		},
		{
			name: "gte exact",
			cond: Condition{Key: "auth_level", Operator: "gte", Value: 2},
			ctx:  map[string]interface{}{"auth_level": 2},
			want: true,
		},
		{
			name: "gte fail",
			cond: Condition{Key: "auth_level", Operator: "gte", Value: 2},
			ctx:  map[string]interface{}{"auth_level": 1},
			want: false,
		},
		{
			name: "gt",
			cond: Condition{Key: "auth_level", Operator: "gt", Value: 2},
			ctx:  map[string]interface{}{"auth_level": 3},
			want: true,
		},
		{
			name: "lt",
			cond: Condition{Key: "auth_level", Operator: "lt", Value: 5},
			ctx:  map[string]interface{}{"auth_level": 3},
			want: true,
		},
		{
			name: "lte",
			cond: Condition{Key: "auth_level", Operator: "lte", Value: 2},
			ctx:  map[string]interface{}{"auth_level": 2},
			want: true,
		},
		{
			name: "in match",
			cond: Condition{Key: "role", Operator: "in", Value: []string{"admin", "superadmin"}},
			ctx:  map[string]interface{}{"role": "admin"},
			want: true,
		},
		{
			name: "in no match",
			cond: Condition{Key: "role", Operator: "in", Value: []string{"admin", "superadmin"}},
			ctx:  map[string]interface{}{"role": "user"},
			want: false,
		},
		{
			name: "contains",
			cond: Condition{Key: "ip", Operator: "contains", Value: "192.168"},
			ctx:  map[string]interface{}{"ip": "192.168.1.1"},
			want: true,
		},
		{
			name: "matches wildcard",
			cond: Condition{Key: "path", Operator: "matches", Value: "/api/v1/*"},
			ctx:  map[string]interface{}{"path": "/api/v1/users"},
			want: true,
		},
		{
			name: "nil context",
			cond: Condition{Key: "auth_level", Operator: "gte", Value: 2},
			ctx:  nil,
			want: false,
		},
		{
			name: "missing key",
			cond: Condition{Key: "auth_level", Operator: "gte", Value: 2},
			ctx:  map[string]interface{}{"other": "val"},
			want: false,
		},
		{
			name: "unknown operator",
			cond: Condition{Key: "x", Operator: "xor", Value: 1},
			ctx:  map[string]interface{}{"x": 1},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCondition(tt.cond, tt.ctx)
			if got != tt.want {
				t.Errorf("EvaluateCondition() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEngine_AdminAllowedAll(t *testing.T) {
	eng := New(DefaultGarudaPassPolicies()...)

	allowed, policy := eng.Evaluate(Request{
		Subject:  "admin",
		Resource: "api:/v1/anything",
		Action:   "read",
		Context:  map[string]interface{}{},
	})
	if !allowed {
		t.Error("admin should be allowed on all resources")
	}
	if policy != "gp-admin-full" {
		t.Errorf("expected gp-admin-full, got %s", policy)
	}
}

func TestEngine_UserOwnData(t *testing.T) {
	eng := New(DefaultGarudaPassPolicies()...)

	allowed, _ := eng.Evaluate(Request{
		Subject:  "user:123",
		Resource: "data:users:self",
		Action:   "read",
		Context:  map[string]interface{}{},
	})
	if !allowed {
		t.Error("user should be allowed to read own data")
	}
}

func TestEngine_UserDeniedOtherData(t *testing.T) {
	eng := New(DefaultGarudaPassPolicies()...)

	allowed, policy := eng.Evaluate(Request{
		Subject:  "user:123",
		Resource: "data:users:other",
		Action:   "read",
		Context:  map[string]interface{}{},
	})
	if allowed {
		t.Error("user should be denied access to other user's data")
	}
	if policy != "gp-user-deny-other" {
		t.Errorf("expected gp-user-deny-other, got %s", policy)
	}
}

func TestEngine_DenyPrecedence(t *testing.T) {
	// Both ALLOW and DENY match; DENY must win.
	eng := New(
		&Policy{
			ID:        "allow-all",
			Effect:    "ALLOW",
			Subjects:  []string{"*"},
			Resources: []string{"*"},
			Actions:   []string{"*"},
		},
		&Policy{
			ID:        "deny-secret",
			Effect:    "DENY",
			Subjects:  []string{"*"},
			Resources: []string{"secret:*"},
			Actions:   []string{"*"},
		},
	)

	allowed, policy := eng.Evaluate(Request{
		Subject:  "user:1",
		Resource: "secret:keys",
		Action:   "read",
	})
	if allowed {
		t.Error("DENY should take precedence over ALLOW")
	}
	if policy != "deny-secret" {
		t.Errorf("expected deny-secret, got %s", policy)
	}
}

func TestEngine_WildcardResources(t *testing.T) {
	eng := New(DefaultGarudaPassPolicies()...)

	allowed, _ := eng.Evaluate(Request{
		Subject:  "admin",
		Resource: "api:/v1/sign/certificates",
		Action:   "sign",
		Context:  map[string]interface{}{},
	})
	if !allowed {
		t.Error("admin should access wildcard resource api:/v1/sign/*")
	}
}

func TestEngine_SigningRequiresAuthLevel(t *testing.T) {
	eng := New(DefaultGarudaPassPolicies()...)

	// Non-admin user with sufficient auth_level.
	allowed, policy := eng.Evaluate(Request{
		Subject:  "user:456",
		Resource: "api:/v1/sign/document",
		Action:   "sign",
		Context:  map[string]interface{}{"auth_level": 2},
	})
	if !allowed {
		t.Error("user with auth_level >= 2 should be allowed to sign")
	}
	if policy != "gp-signing-auth" {
		t.Errorf("expected gp-signing-auth, got %s", policy)
	}

	// Non-admin user with insufficient auth_level.
	allowed, _ = eng.Evaluate(Request{
		Subject:  "user:456",
		Resource: "api:/v1/sign/document",
		Action:   "sign",
		Context:  map[string]interface{}{"auth_level": 1},
	})
	if allowed {
		t.Error("user with auth_level < 2 should not be allowed to sign")
	}
}

func TestEngine_EnvironmentCondition(t *testing.T) {
	eng := New(&Policy{
		ID:        "prod-only",
		Effect:    "ALLOW",
		Subjects:  []string{"*"},
		Resources: []string{"*"},
		Actions:   []string{"deploy"},
		Conditions: []Condition{
			{Key: "environment", Operator: "eq", Value: "production"},
		},
	})

	allowed, _ := eng.Evaluate(Request{
		Subject:  "deployer",
		Resource: "app:web",
		Action:   "deploy",
		Context:  map[string]interface{}{"environment": "production"},
	})
	if !allowed {
		t.Error("deploy should be allowed in production")
	}

	allowed, _ = eng.Evaluate(Request{
		Subject:  "deployer",
		Resource: "app:web",
		Action:   "deploy",
		Context:  map[string]interface{}{"environment": "staging"},
	})
	if allowed {
		t.Error("deploy should not be allowed outside production")
	}
}

func TestEngine_NoMatchDefaultDeny(t *testing.T) {
	eng := New(DefaultGarudaPassPolicies()...)

	allowed, policy := eng.Evaluate(Request{
		Subject:  "unknown",
		Resource: "unknown:resource",
		Action:   "unknown",
		Context:  map[string]interface{}{},
	})
	if allowed {
		t.Error("no matching policy should default to deny")
	}
	if policy != "" {
		t.Errorf("expected empty policy, got %s", policy)
	}
}

func TestEngine_DefaultPoliciesCoverage(t *testing.T) {
	policies := DefaultGarudaPassPolicies()
	if len(policies) < 7 {
		t.Fatalf("expected at least 7 default policies, got %d", len(policies))
	}

	ids := make(map[string]bool)
	for _, p := range policies {
		ids[p.ID] = true
	}

	expected := []string{
		"gp-admin-full",
		"gp-user-own-data",
		"gp-user-deny-other",
		"gp-service-internal",
		"gp-signing-auth",
		"gp-delete-auth",
		"gp-audit-admin",
	}
	for _, id := range expected {
		if !ids[id] {
			t.Errorf("missing default policy: %s", id)
		}
	}
}

func TestEngine_AddPolicy(t *testing.T) {
	eng := New()
	eng.AddPolicy(&Policy{
		ID:        "dynamic",
		Effect:    "ALLOW",
		Subjects:  []string{"*"},
		Resources: []string{"*"},
		Actions:   []string{"ping"},
	})

	allowed, policy := eng.Evaluate(Request{
		Subject:  "anyone",
		Resource: "health",
		Action:   "ping",
	})
	if !allowed || policy != "dynamic" {
		t.Error("dynamically added policy should match")
	}
}

func TestEngine_ServiceInternalAccess(t *testing.T) {
	eng := New(DefaultGarudaPassPolicies()...)

	allowed, policy := eng.Evaluate(Request{
		Subject:  "service:identity",
		Resource: "api:/internal/users",
		Action:   "read",
		Context:  map[string]interface{}{},
	})
	if !allowed {
		t.Error("service account should access internal APIs")
	}
	if policy != "gp-service-internal" {
		t.Errorf("expected gp-service-internal, got %s", policy)
	}
}

func TestEngine_AuditAdminOnly(t *testing.T) {
	eng := New(DefaultGarudaPassPolicies()...)

	allowed, _ := eng.Evaluate(Request{
		Subject:  "admin",
		Resource: "audit:logs",
		Action:   "read",
		Context:  map[string]interface{}{},
	})
	if !allowed {
		t.Error("admin should read audit logs")
	}

	allowed, _ = eng.Evaluate(Request{
		Subject:  "user:123",
		Resource: "audit:logs",
		Action:   "read",
		Context:  map[string]interface{}{},
	})
	if allowed {
		t.Error("non-admin should not read audit logs")
	}
}
