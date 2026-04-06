package safemap

import (
	"sync"
	"testing"
	"time"
)

func TestMap_SetGet(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)

	v, ok := m.Get("a")
	if !ok || v != 1 {
		t.Errorf("get: ok=%v, v=%d", ok, v)
	}
}

func TestMap_GetMissing(t *testing.T) {
	m := New[string, int]()
	_, ok := m.Get("missing")
	if ok {
		t.Error("missing key should return false")
	}
}

func TestMap_SetWithTTL(t *testing.T) {
	m := New[string, string]()
	m.SetWithTTL("key", "value", 20*time.Millisecond)

	v, ok := m.Get("key")
	if !ok || v != "value" {
		t.Error("should be accessible before expiry")
	}

	time.Sleep(30 * time.Millisecond)

	_, ok = m.Get("key")
	if ok {
		t.Error("should expire after TTL")
	}
}

func TestMap_GetOrSet_New(t *testing.T) {
	m := New[string, int]()
	v, existed := m.GetOrSet("key", 42)
	if existed {
		t.Error("should be new")
	}
	if v != 42 {
		t.Errorf("value: got %d", v)
	}
}

func TestMap_GetOrSet_Existing(t *testing.T) {
	m := New[string, int]()
	m.Set("key", 1)

	v, existed := m.GetOrSet("key", 99)
	if !existed {
		t.Error("should exist")
	}
	if v != 1 {
		t.Errorf("should return existing value: got %d", v)
	}
}

func TestMap_GetOrSet_Expired(t *testing.T) {
	m := New[string, int]()
	m.SetWithTTL("key", 1, 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	v, existed := m.GetOrSet("key", 99)
	if existed {
		t.Error("expired entry should not count as existing")
	}
	if v != 99 {
		t.Errorf("should store new value: got %d", v)
	}
}

func TestMap_Delete(t *testing.T) {
	m := New[string, int]()
	m.Set("key", 1)
	m.Delete("key")

	_, ok := m.Get("key")
	if ok {
		t.Error("deleted key should not be found")
	}
}

func TestMap_Has(t *testing.T) {
	m := New[string, int]()
	m.Set("key", 1)

	if !m.Has("key") {
		t.Error("should have key")
	}
	if m.Has("missing") {
		t.Error("should not have missing")
	}
}

func TestMap_Len(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	if m.Len() != 2 {
		t.Errorf("len: got %d", m.Len())
	}
}

func TestMap_Keys(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.SetWithTTL("c", 3, 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	keys := m.Keys()
	if len(keys) != 2 {
		t.Errorf("keys (excluding expired): got %d", len(keys))
	}
}

func TestMap_Values(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 10)
	m.Set("b", 20)

	values := m.Values()
	if len(values) != 2 {
		t.Errorf("values: got %d", len(values))
	}
}

func TestMap_Range(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	sum := 0
	m.Range(func(key string, value int) bool {
		sum += value
		return true
	})
	if sum != 6 {
		t.Errorf("sum: got %d", sum)
	}
}

func TestMap_Range_EarlyStop(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	count := 0
	m.Range(func(key string, value int) bool {
		count++
		return false // Stop after first.
	})
	if count != 1 {
		t.Errorf("should stop after 1: got %d", count)
	}
}

func TestMap_Clear(t *testing.T) {
	m := New[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Clear()

	if m.Len() != 0 {
		t.Errorf("after clear: got %d", m.Len())
	}
}

func TestMap_Cleanup(t *testing.T) {
	m := New[string, int]()
	m.SetWithTTL("expired", 1, 1*time.Millisecond)
	m.Set("permanent", 2)
	time.Sleep(5 * time.Millisecond)

	removed := m.Cleanup()
	if removed != 1 {
		t.Errorf("removed: got %d", removed)
	}
	if m.Len() != 1 {
		t.Errorf("remaining: got %d", m.Len())
	}
}

func TestMap_ConcurrentAccess(t *testing.T) {
	m := New[int, int]()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			m.Set(n, n*10)
		}(i)
		go func(n int) {
			defer wg.Done()
			m.Get(n)
		}(i)
	}
	wg.Wait()

	if m.Len() != 100 {
		t.Errorf("concurrent: got %d", m.Len())
	}
}

func TestMap_IntKeys(t *testing.T) {
	m := New[int, string]()
	m.Set(1, "one")
	m.Set(2, "two")

	v, ok := m.Get(1)
	if !ok || v != "one" {
		t.Error("int key should work")
	}
}
