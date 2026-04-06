package cachettl

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	c := New[string, int](5 * time.Minute)
	if c.Len() != 0 {
		t.Errorf("Len = %d", c.Len())
	}
}

func TestSetGet(t *testing.T) {
	c := New[string, string](1 * time.Hour)
	c.Set("key", "value")

	v, ok := c.Get("key")
	if !ok || v != "value" {
		t.Errorf("Get = (%q, %v)", v, ok)
	}
}

func TestGet_Missing(t *testing.T) {
	c := New[string, int](1 * time.Hour)
	_, ok := c.Get("missing")
	if ok {
		t.Error("should return false for missing")
	}
}

func TestGet_Expired(t *testing.T) {
	c := New[string, string](1 * time.Millisecond)
	c.Set("key", "value")

	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("key")
	if ok {
		t.Error("should return false for expired")
	}
}

func TestSetWithTTL(t *testing.T) {
	c := New[string, string](1 * time.Hour)
	c.SetWithTTL("short", "val", 1*time.Millisecond)
	c.SetWithTTL("long", "val", 1*time.Hour)

	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("short")
	if ok {
		t.Error("short TTL should be expired")
	}
	_, ok = c.Get("long")
	if !ok {
		t.Error("long TTL should still be valid")
	}
}

func TestGetOrSet(t *testing.T) {
	c := New[string, int](1 * time.Hour)
	callCount := 0

	v := c.GetOrSet("key", func() int {
		callCount++
		return 42
	})
	if v != 42 {
		t.Errorf("v = %d", v)
	}

	v2 := c.GetOrSet("key", func() int {
		callCount++
		return 99
	})
	if v2 != 42 {
		t.Error("should return cached value")
	}
	if callCount != 1 {
		t.Errorf("fn called %d times", callCount)
	}
}

func TestDelete(t *testing.T) {
	c := New[string, string](1 * time.Hour)
	c.Set("key", "val")
	c.Delete("key")

	_, ok := c.Get("key")
	if ok {
		t.Error("should be deleted")
	}
}

func TestHas(t *testing.T) {
	c := New[string, int](1 * time.Hour)
	c.Set("key", 1)

	if !c.Has("key") {
		t.Error("should have key")
	}
	if c.Has("missing") {
		t.Error("should not have missing")
	}
}

func TestPurge(t *testing.T) {
	c := New[string, string](1 * time.Millisecond)
	c.Set("a", "1")
	c.Set("b", "2")
	c.Set("c", "3")

	time.Sleep(5 * time.Millisecond)

	removed := c.Purge()
	if removed != 3 {
		t.Errorf("removed = %d", removed)
	}
}

func TestClear(t *testing.T) {
	c := New[string, int](1 * time.Hour)
	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()

	if c.Len() != 0 {
		t.Errorf("Len = %d after clear", c.Len())
	}
}

func TestKeys(t *testing.T) {
	c := New[string, int](1 * time.Hour)
	c.Set("a", 1)
	c.Set("b", 2)

	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("keys = %d", len(keys))
	}
}

func TestKeys_ExcludesExpired(t *testing.T) {
	c := New[string, int](1 * time.Millisecond)
	c.Set("expired", 1)
	c.SetWithTTL("fresh", 2, 1*time.Hour)

	time.Sleep(5 * time.Millisecond)

	keys := c.Keys()
	if len(keys) != 1 {
		t.Errorf("keys = %d, want 1", len(keys))
	}
}

func TestConcurrent(t *testing.T) {
	c := New[string, int](1 * time.Hour)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func(n int) {
			defer wg.Done()
			c.Set("key", n)
		}(i)
		go func() {
			defer wg.Done()
			c.Get("key")
		}()
		go func() {
			defer wg.Done()
			c.Has("key")
		}()
	}
	wg.Wait()
}

func TestCache_IntKeys(t *testing.T) {
	c := New[int, string](1 * time.Hour)
	c.Set(1, "one")
	c.Set(2, "two")

	v, ok := c.Get(1)
	if !ok || v != "one" {
		t.Error("should work with int keys")
	}
}
