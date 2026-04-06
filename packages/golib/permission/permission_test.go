package permission

import (
	"sync"
	"testing"
)

func TestScope_ExactMatch(t *testing.T) {
	if !Scope("users.read").Match("users.read") {
		t.Error("exact match should succeed")
	}
	if Scope("users.read").Match("users.write") {
		t.Error("different scope should not match")
	}
}

func TestScope_WildcardAll(t *testing.T) {
	if !Scope("*").Match("anything.here") {
		t.Error("* should match everything")
	}
}

func TestScope_WildcardSuffix(t *testing.T) {
	s := Scope("admin.*")
	if !s.Match("admin.read") {
		t.Error("admin.* should match admin.read")
	}
	if !s.Match("admin.write") {
		t.Error("admin.* should match admin.write")
	}
	if s.Match("user.read") {
		t.Error("admin.* should not match user.read")
	}
}

func TestScope_NoPartialMatch(t *testing.T) {
	if Scope("users").Match("users.read") {
		t.Error("users should not match users.read without wildcard")
	}
}

func TestChecker_Grant_HasPermission(t *testing.T) {
	c := NewChecker()
	c.Grant("user-1", "users.read", "users.write")

	if !c.HasPermission("user-1", "users.read") {
		t.Error("should have users.read")
	}
	if c.HasPermission("user-1", "admin.delete") {
		t.Error("should not have admin.delete")
	}
}

func TestChecker_HasPermission_UnknownSubject(t *testing.T) {
	c := NewChecker()
	if c.HasPermission("nobody", "users.read") {
		t.Error("unknown subject should have no permissions")
	}
}

func TestChecker_WildcardGrant(t *testing.T) {
	c := NewChecker()
	c.Grant("admin", "*")

	if !c.HasPermission("admin", "anything.at.all") {
		t.Error("* grant should match everything")
	}
}

func TestChecker_ScopedWildcard(t *testing.T) {
	c := NewChecker()
	c.Grant("manager", "reports.*")

	if !c.HasPermission("manager", "reports.generate") {
		t.Error("reports.* should match reports.generate")
	}
	if c.HasPermission("manager", "users.delete") {
		t.Error("reports.* should not match users.delete")
	}
}

func TestChecker_Revoke(t *testing.T) {
	c := NewChecker()
	c.Grant("user", "read", "write")
	c.Revoke("user")

	if c.HasPermission("user", "read") {
		t.Error("revoked subject should have no permissions")
	}
}

func TestChecker_RevokeScope(t *testing.T) {
	c := NewChecker()
	c.Grant("user", "read", "write", "delete")
	c.RevokeScope("user", "delete")

	if c.HasPermission("user", "delete") {
		t.Error("revoked scope should be removed")
	}
	if !c.HasPermission("user", "read") {
		t.Error("other scopes should remain")
	}
}

func TestChecker_HasAny(t *testing.T) {
	c := NewChecker()
	c.Grant("user", "reports.read")

	if !c.HasAny("user", "reports.read", "reports.write") {
		t.Error("HasAny should return true if any match")
	}
	if c.HasAny("user", "admin.read", "admin.write") {
		t.Error("HasAny should return false if none match")
	}
}

func TestChecker_HasAll(t *testing.T) {
	c := NewChecker()
	c.Grant("user", "read", "write")

	if !c.HasAll("user", "read", "write") {
		t.Error("HasAll should return true when all present")
	}
	if c.HasAll("user", "read", "delete") {
		t.Error("HasAll should return false when any missing")
	}
}

func TestChecker_Scopes(t *testing.T) {
	c := NewChecker()
	c.Grant("user", "a", "b", "c")

	scopes := c.Scopes("user")
	if len(scopes) != 3 {
		t.Errorf("scopes: got %d, want 3", len(scopes))
	}
}

func TestChecker_Subjects(t *testing.T) {
	c := NewChecker()
	c.Grant("alice", "read")
	c.Grant("bob", "write")

	subjects := c.Subjects()
	if len(subjects) != 2 {
		t.Errorf("subjects: got %d", len(subjects))
	}
}

func TestRoleHierarchy_Define(t *testing.T) {
	h := NewRoleHierarchy()
	h.Define("user", "read")

	scopes := h.ResolveScopes("user")
	if len(scopes) != 1 || scopes[0] != "read" {
		t.Errorf("scopes: got %v", scopes)
	}
}

func TestRoleHierarchy_Inheritance(t *testing.T) {
	h := NewRoleHierarchy()
	h.Define("user", "read")
	h.Define("admin", "write", "delete")
	h.Inherit("admin", "user")

	scopes := h.ResolveScopes("admin")
	if len(scopes) != 3 {
		t.Errorf("admin scopes: got %d, want 3 (write, delete, read)", len(scopes))
	}

	// User should not inherit admin scopes.
	userScopes := h.ResolveScopes("user")
	if len(userScopes) != 1 {
		t.Errorf("user scopes: got %d, want 1", len(userScopes))
	}
}

func TestRoleHierarchy_MultiLevel(t *testing.T) {
	h := NewRoleHierarchy()
	h.Define("viewer", "read")
	h.Define("editor", "write")
	h.Define("admin", "delete")
	h.Inherit("editor", "viewer")  // editor inherits viewer
	h.Inherit("admin", "editor")   // admin inherits editor (which inherits viewer)

	scopes := h.ResolveScopes("admin")
	if len(scopes) != 3 {
		t.Errorf("admin should have 3 scopes (delete, write, read), got %d: %v", len(scopes), scopes)
	}
}

func TestRoleHierarchy_CyclePrevention(t *testing.T) {
	h := NewRoleHierarchy()
	h.Define("a", "scope_a")
	h.Define("b", "scope_b")
	h.Inherit("a", "b")
	h.Inherit("b", "a") // cycle!

	// Should not infinite loop.
	scopes := h.ResolveScopes("a")
	if len(scopes) == 0 {
		t.Error("should resolve some scopes despite cycle")
	}
}

func TestRoleHierarchy_UndefinedRole(t *testing.T) {
	h := NewRoleHierarchy()
	scopes := h.ResolveScopes("nonexistent")
	if len(scopes) != 0 {
		t.Error("undefined role should have no scopes")
	}
}

func TestChecker_ConcurrentAccess(t *testing.T) {
	c := NewChecker()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			subject := string(rune('A' + (n % 26)))
			c.Grant(subject, Scope("scope_"+subject))
			c.HasPermission(subject, Scope("scope_"+subject))
			c.HasAny(subject, Scope("scope_"+subject), "other")
			c.Scopes(subject)
		}(i)
	}
	wg.Wait()
}
