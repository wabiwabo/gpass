package trie

import "testing"

func TestSetGet(t *testing.T) {
	tr := New[int]()
	tr.Set("hello", 1)
	tr.Set("world", 2)

	v, ok := tr.Get("hello")
	if !ok || v != 1 {
		t.Errorf("Get(hello) = (%d, %v)", v, ok)
	}
	v, ok = tr.Get("world")
	if !ok || v != 2 {
		t.Errorf("Get(world) = (%d, %v)", v, ok)
	}
}

func TestGet_Missing(t *testing.T) {
	tr := New[string]()
	_, ok := tr.Get("missing")
	if ok {
		t.Error("should not find missing")
	}
}

func TestGet_PrefixNotFound(t *testing.T) {
	tr := New[int]()
	tr.Set("hello", 1)

	_, ok := tr.Get("hel") // prefix exists but has no value
	if ok {
		t.Error("prefix without value should not match")
	}
}

func TestHas(t *testing.T) {
	tr := New[int]()
	tr.Set("api/users", 1)

	if !tr.Has("api/users") {
		t.Error("should have exact key")
	}
	if tr.Has("api") {
		t.Error("should not have prefix-only")
	}
}

func TestHasPrefix(t *testing.T) {
	tr := New[int]()
	tr.Set("api/users", 1)
	tr.Set("api/posts", 2)

	if !tr.HasPrefix("api") {
		t.Error("should have prefix 'api'")
	}
	if !tr.HasPrefix("api/users") {
		t.Error("should have prefix of exact key")
	}
	if tr.HasPrefix("web") {
		t.Error("should not have 'web' prefix")
	}
}

func TestDelete(t *testing.T) {
	tr := New[int]()
	tr.Set("key", 42)

	if !tr.Delete("key") {
		t.Error("should return true")
	}
	if tr.Has("key") {
		t.Error("key should be deleted")
	}
	if tr.Len() != 0 {
		t.Errorf("Len = %d", tr.Len())
	}

	if tr.Delete("missing") {
		t.Error("should return false for missing")
	}
}

func TestLongestPrefix(t *testing.T) {
	tr := New[string]()
	tr.Set("/", "root")
	tr.Set("/api", "api")
	tr.Set("/api/v1", "v1")

	key, val, ok := tr.LongestPrefix("/api/v1/users")
	if !ok || key != "/api/v1" || val != "v1" {
		t.Errorf("LongestPrefix = (%q, %q, %v)", key, val, ok)
	}

	key, val, ok = tr.LongestPrefix("/api/v2/items")
	if !ok || key != "/api" || val != "api" {
		t.Errorf("LongestPrefix = (%q, %q, %v)", key, val, ok)
	}

	key, val, ok = tr.LongestPrefix("/web/page")
	if !ok || key != "/" || val != "root" {
		t.Errorf("LongestPrefix = (%q, %q, %v)", key, val, ok)
	}
}

func TestLongestPrefix_NoMatch(t *testing.T) {
	tr := New[int]()
	tr.Set("abc", 1)

	_, _, ok := tr.LongestPrefix("xyz")
	if ok {
		t.Error("should not match")
	}
}

func TestLen(t *testing.T) {
	tr := New[int]()
	tr.Set("a", 1)
	tr.Set("ab", 2)
	tr.Set("abc", 3)

	if tr.Len() != 3 {
		t.Errorf("Len = %d", tr.Len())
	}
}

func TestSet_Overwrite(t *testing.T) {
	tr := New[string]()
	tr.Set("key", "old")
	tr.Set("key", "new")

	v, _ := tr.Get("key")
	if v != "new" {
		t.Errorf("value = %q", v)
	}
	if tr.Len() != 1 {
		t.Errorf("Len = %d, overwrite should not increase count", tr.Len())
	}
}

func TestLongestPrefix_ExactMatch(t *testing.T) {
	tr := New[int]()
	tr.Set("exact", 42)

	key, val, ok := tr.LongestPrefix("exact")
	if !ok || key != "exact" || val != 42 {
		t.Errorf("exact = (%q, %d, %v)", key, val, ok)
	}
}
