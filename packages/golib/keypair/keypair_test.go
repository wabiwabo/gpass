package keypair

import "testing"

func TestNew(t *testing.T) {
	p := New("a", "1", "b", "2", "c", "3")
	if p.Len() != 3 { t.Errorf("Len = %d", p.Len()) }
	if p.Get("a") != "1" { t.Error("a") }
	if p.Get("b") != "2" { t.Error("b") }
}

func TestNew_OddArgs(t *testing.T) {
	p := New("a", "1", "b")
	if p.Len() != 1 { t.Errorf("Len = %d (odd arg dropped)", p.Len()) }
}

func TestFromMap(t *testing.T) {
	p := FromMap(map[string]string{"b": "2", "a": "1"})
	if p.Len() != 2 { t.Errorf("Len = %d", p.Len()) }
	// Should be sorted
	if p[0].Key != "a" { t.Errorf("first = %q", p[0].Key) }
}

func TestAdd(t *testing.T) {
	var p Pairs
	p.Add("key", "value")
	if p.Len() != 1 { t.Error("Add") }
}

func TestGet(t *testing.T) {
	p := New("x", "1", "y", "2")
	if p.Get("x") != "1" { t.Error("x") }
	if p.Get("missing") != "" { t.Error("missing") }
}

func TestHas(t *testing.T) {
	p := New("x", "1")
	if !p.Has("x") { t.Error("should have x") }
	if p.Has("y") { t.Error("should not have y") }
}

func TestCanonical(t *testing.T) {
	p := New("c", "3", "a", "1", "b", "2")
	// Sorted: a=1&b=2&c=3
	got := p.Canonical("&", "=")
	if got != "a=1&b=2&c=3" {
		t.Errorf("Canonical = %q", got)
	}
}

func TestCanonical_CustomSep(t *testing.T) {
	p := New("x", "1", "y", "2")
	got := p.Canonical("\n", ":")
	if got != "x:1\ny:2" {
		t.Errorf("Canonical = %q", got)
	}
}

func TestQueryString(t *testing.T) {
	p := New("name", "John Doe", "age", "30")
	qs := p.QueryString()
	if qs == "" { t.Error("should not be empty") }
}

func TestToMap(t *testing.T) {
	p := New("a", "1", "b", "2")
	m := p.ToMap()
	if m["a"] != "1" || m["b"] != "2" {
		t.Errorf("ToMap = %v", m)
	}
}

func TestKeys_Values(t *testing.T) {
	p := New("a", "1", "b", "2")
	keys := p.Keys()
	vals := p.Values()
	if len(keys) != 2 || keys[0] != "a" { t.Error("keys") }
	if len(vals) != 2 || vals[0] != "1" { t.Error("vals") }
}

func TestSort(t *testing.T) {
	p := New("c", "3", "a", "1", "b", "2")
	p.Sort()
	if p[0].Key != "a" || p[1].Key != "b" || p[2].Key != "c" {
		t.Errorf("Sort = %v", p)
	}
}

func TestDuplicateKeys(t *testing.T) {
	p := New("a", "1", "a", "2")
	// Get returns first
	if p.Get("a") != "1" { t.Error("should return first") }
	// ToMap returns last
	m := p.ToMap()
	if m["a"] != "2" { t.Error("map should have last value") }
}
