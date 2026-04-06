package headermap

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	h := New()
	if h.Len() != 0 {
		t.Errorf("Len = %d", h.Len())
	}
}

func TestSetGet(t *testing.T) {
	h := New()
	h.Set("Content-Type", "application/json")
	if h.Get("content-type") != "application/json" {
		t.Error("should be case-insensitive")
	}
	if h.Get("CONTENT-TYPE") != "application/json" {
		t.Error("should be case-insensitive")
	}
}

func TestAdd_GetAll(t *testing.T) {
	h := New()
	h.Add("Accept", "text/html")
	h.Add("Accept", "application/json")

	all := h.GetAll("accept")
	if len(all) != 2 {
		t.Fatalf("len = %d", len(all))
	}
}

func TestHas(t *testing.T) {
	h := New()
	h.Set("X-Custom", "value")
	if !h.Has("x-custom") {
		t.Error("should have header")
	}
	if h.Has("x-missing") {
		t.Error("should not have header")
	}
}

func TestDel(t *testing.T) {
	h := New()
	h.Set("X-Remove", "value")
	h.Del("x-remove")
	if h.Has("x-remove") {
		t.Error("should be deleted")
	}
}

func TestGetInt(t *testing.T) {
	h := New()
	h.Set("Content-Length", "1024")

	n, ok := h.GetInt("content-length")
	if !ok || n != 1024 {
		t.Errorf("GetInt = %d, %v", n, ok)
	}

	_, ok = h.GetInt("missing")
	if ok {
		t.Error("should return false for missing")
	}

	h.Set("Bad", "abc")
	_, ok = h.GetInt("bad")
	if ok {
		t.Error("should return false for non-numeric")
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{"true", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"", false},
	}

	for _, tt := range tests {
		h := New()
		h.Set("X-Flag", tt.value)
		if h.GetBool("x-flag") != tt.want {
			t.Errorf("GetBool(%q) = %v, want %v", tt.value, h.GetBool("x-flag"), tt.want)
		}
	}
}

func TestGetTime(t *testing.T) {
	h := New()
	h.Set("X-Time", "2024-06-15T10:30:00Z")

	ts, ok := h.GetTime("x-time")
	if !ok {
		t.Fatal("should parse time")
	}
	if ts.Year() != 2024 || ts.Month() != time.June {
		t.Errorf("time = %v", ts)
	}

	_, ok = h.GetTime("missing")
	if ok {
		t.Error("missing should return false")
	}
}

func TestKeys(t *testing.T) {
	h := New()
	h.Set("Z-Header", "1")
	h.Set("A-Header", "2")
	h.Set("M-Header", "3")

	keys := h.Keys()
	if len(keys) != 3 {
		t.Fatalf("len = %d", len(keys))
	}
	// Sorted (normalized to lowercase)
	if keys[0] != "a-header" {
		t.Errorf("[0] = %q", keys[0])
	}
}

func TestFromMap(t *testing.T) {
	h := FromMap(map[string]string{
		"X-A": "1",
		"X-B": "2",
	})
	if h.Len() != 2 {
		t.Errorf("Len = %d", h.Len())
	}
	if h.Get("x-a") != "1" {
		t.Error("should have x-a")
	}
}

func TestToMap(t *testing.T) {
	h := New()
	h.Set("X-A", "1")
	h.Set("X-B", "2")

	m := h.ToMap()
	if len(m) != 2 {
		t.Errorf("len = %d", len(m))
	}
}

func TestClone(t *testing.T) {
	h := New()
	h.Set("X-A", "1")
	c := h.Clone()

	c.Set("X-A", "2")
	if h.Get("x-a") != "1" {
		t.Error("clone should not affect original")
	}
}

func TestGet_Missing(t *testing.T) {
	h := New()
	if h.Get("missing") != "" {
		t.Error("missing should return empty string")
	}
}

func TestSet_Overwrites(t *testing.T) {
	h := New()
	h.Set("X-A", "first")
	h.Set("X-A", "second")
	if h.Get("x-a") != "second" {
		t.Error("Set should overwrite")
	}
}
