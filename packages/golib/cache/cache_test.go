package cache

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSetAndGet(t *testing.T) {
	c := New(5 * time.Minute)
	c.Set("key1", "value1")

	v, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to be found")
	}
	if v != "value1" {
		t.Errorf("got %v, want value1", v)
	}
}

func TestGetExpiredItem(t *testing.T) {
	c := New(1 * time.Millisecond)
	c.Set("key1", "value1")

	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected expired item to return miss")
	}
}

func TestSetWithTTL(t *testing.T) {
	c := New(1 * time.Hour)

	// Set with short TTL
	c.SetWithTTL("short", "value", 1*time.Millisecond)
	// Set with long TTL
	c.SetWithTTL("long", "value", 1*time.Hour)

	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("short")
	if ok {
		t.Error("expected short TTL item to expire")
	}

	_, ok = c.Get("long")
	if !ok {
		t.Error("expected long TTL item to still exist")
	}
}

func TestDelete(t *testing.T) {
	c := New(5 * time.Minute)
	c.Set("key1", "value1")
	c.Delete("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Error("expected deleted key to return miss")
	}
}

func TestClear(t *testing.T) {
	c := New(5 * time.Minute)
	c.Set("key1", "value1")
	c.Set("key2", "value2")
	c.Set("key3", "value3")
	c.Clear()

	if c.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", c.Size())
	}
}

func TestSizeExcludesExpired(t *testing.T) {
	c := New(1 * time.Hour)
	c.Set("live1", "v")
	c.Set("live2", "v")
	c.SetWithTTL("expired", "v", 1*time.Millisecond)

	time.Sleep(5 * time.Millisecond)

	if got := c.Size(); got != 2 {
		t.Errorf("got size %d, want 2", got)
	}
}

func TestGetOrSet_CacheHit(t *testing.T) {
	c := New(5 * time.Minute)
	c.Set("key1", "cached-value")

	called := false
	v, err := c.GetOrSet("key1", func() (interface{}, error) {
		called = true
		return "new-value", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("fn should not be called on cache hit")
	}
	if v != "cached-value" {
		t.Errorf("got %v, want cached-value", v)
	}
}

func TestGetOrSet_CacheMiss(t *testing.T) {
	c := New(5 * time.Minute)

	v, err := c.GetOrSet("key1", func() (interface{}, error) {
		return "computed-value", nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "computed-value" {
		t.Errorf("got %v, want computed-value", v)
	}

	// Verify it was cached
	v2, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected value to be cached after GetOrSet")
	}
	if v2 != "computed-value" {
		t.Errorf("cached value: got %v, want computed-value", v2)
	}
}

func TestGetOrSet_FnError(t *testing.T) {
	c := New(5 * time.Minute)

	errExpected := errors.New("computation failed")
	_, err := c.GetOrSet("key1", func() (interface{}, error) {
		return nil, errExpected
	})

	if !errors.Is(err, errExpected) {
		t.Errorf("got error %v, want %v", err, errExpected)
	}

	// Verify error result was not cached
	_, ok := c.Get("key1")
	if ok {
		t.Error("error result should not be cached")
	}
}

func TestStats(t *testing.T) {
	c := New(5 * time.Minute)
	c.Set("key1", "value1")

	// Hit
	c.Get("key1")
	c.Get("key1")
	// Miss
	c.Get("missing")

	stats := c.Stats()
	if stats.Hits != 2 {
		t.Errorf("got %d hits, want 2", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("got %d misses, want 1", stats.Misses)
	}
	if stats.Size != 1 {
		t.Errorf("got size %d, want 1", stats.Size)
	}
}

func TestCleanup(t *testing.T) {
	c := New(1 * time.Millisecond)
	c.Set("key1", "v1")
	c.Set("key2", "v2")
	c.SetWithTTL("key3", "v3", 1*time.Hour) // This one should survive

	time.Sleep(5 * time.Millisecond)
	c.Cleanup()

	c.mu.RLock()
	count := len(c.items)
	c.mu.RUnlock()

	if count != 1 {
		t.Errorf("expected 1 item after cleanup, got %d", count)
	}

	v, ok := c.Get("key3")
	if !ok || v != "v3" {
		t.Error("key3 should survive cleanup")
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := New(5 * time.Minute)
	var wg sync.WaitGroup

	// Run concurrent reads and writes
	for i := 0; i < 100; i++ {
		wg.Add(3)
		key := "key"
		go func() {
			defer wg.Done()
			c.Set(key, "value")
		}()
		go func() {
			defer wg.Done()
			c.Get(key)
		}()
		go func() {
			defer wg.Done()
			c.Size()
		}()
	}

	wg.Wait()
}
