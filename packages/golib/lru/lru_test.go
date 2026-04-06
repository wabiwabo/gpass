package lru

import (
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	c := New[string, int](3)
	if c.Len() != 0 {
		t.Errorf("Len = %d", c.Len())
	}
	if c.Cap() != 3 {
		t.Errorf("Cap = %d", c.Cap())
	}
}

func TestSetGet(t *testing.T) {
	c := New[string, string](10)
	c.Set("key", "value")

	v, ok := c.Get("key")
	if !ok || v != "value" {
		t.Errorf("Get = (%q, %v)", v, ok)
	}
}

func TestGet_Missing(t *testing.T) {
	c := New[string, int](10)
	_, ok := c.Get("missing")
	if ok {
		t.Error("should return false")
	}
}

func TestEviction(t *testing.T) {
	c := New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)
	c.Set("d", 4) // evicts "a"

	if _, ok := c.Get("a"); ok {
		t.Error("a should be evicted")
	}
	if _, ok := c.Get("d"); !ok {
		t.Error("d should exist")
	}
	if c.Len() != 3 {
		t.Errorf("Len = %d", c.Len())
	}
}

func TestLRU_AccessUpdatesRecency(t *testing.T) {
	c := New[string, int](3)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	c.Get("a") // makes "a" most recent

	c.Set("d", 4) // evicts "b" (least recent), not "a"

	if _, ok := c.Get("a"); !ok {
		t.Error("a should still exist (recently accessed)")
	}
	if _, ok := c.Get("b"); ok {
		t.Error("b should be evicted (least recent)")
	}
}

func TestSet_UpdateExisting(t *testing.T) {
	c := New[string, int](3)
	c.Set("key", 1)
	c.Set("key", 2)

	v, _ := c.Get("key")
	if v != 2 {
		t.Errorf("value = %d, want 2", v)
	}
	if c.Len() != 1 {
		t.Errorf("Len = %d, want 1", c.Len())
	}
}

func TestDelete(t *testing.T) {
	c := New[string, int](10)
	c.Set("key", 1)

	if !c.Delete("key") {
		t.Error("should return true")
	}
	if c.Delete("missing") {
		t.Error("should return false for missing")
	}
	if c.Len() != 0 {
		t.Errorf("Len = %d", c.Len())
	}
}

func TestHas(t *testing.T) {
	c := New[string, int](10)
	c.Set("key", 1)

	if !c.Has("key") {
		t.Error("should have key")
	}
	if c.Has("missing") {
		t.Error("should not have missing")
	}
}

func TestClear(t *testing.T) {
	c := New[string, int](10)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()

	if c.Len() != 0 {
		t.Errorf("Len = %d", c.Len())
	}
}

func TestKeys(t *testing.T) {
	c := New[string, int](10)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	keys := c.Keys()
	if len(keys) != 3 {
		t.Fatalf("keys = %d", len(keys))
	}
	// Most recent first
	if keys[0] != "c" {
		t.Errorf("first = %q, want c (most recent)", keys[0])
	}
}

func TestOnEvict(t *testing.T) {
	var evictedKey string
	var evictedVal int

	c := New[string, int](2, WithOnEvict[string, int](func(k string, v int) {
		evictedKey = k
		evictedVal = v
	}))

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // evicts "a"

	if evictedKey != "a" || evictedVal != 1 {
		t.Errorf("evicted = (%q, %d)", evictedKey, evictedVal)
	}
}

func TestConcurrent(t *testing.T) {
	c := New[int, int](100)
	var wg sync.WaitGroup

	for i := 0; i < 200; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			c.Set(n, n*10)
		}(i)
		go func(n int) {
			defer wg.Done()
			c.Get(n)
		}(i)
	}
	wg.Wait()

	if c.Len() > 100 {
		t.Errorf("Len = %d, should not exceed capacity", c.Len())
	}
}

func TestNew_DefaultCapacity(t *testing.T) {
	c := New[string, int](0)
	if c.Cap() != 128 {
		t.Errorf("Cap = %d, want 128 default", c.Cap())
	}
}
