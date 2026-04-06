package ttlmap

import (
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	m := New(5 * time.Minute)
	if m.Len() != 0 {
		t.Errorf("Len = %d", m.Len())
	}
}

func TestNew_DefaultTTL(t *testing.T) {
	m := New(0)
	if m.ttl != 5*time.Minute {
		t.Errorf("ttl = %v", m.ttl)
	}
}

func TestSet_Get(t *testing.T) {
	m := New(1 * time.Hour)
	m.Set("key", "value")

	v, ok := m.Get("key")
	if !ok || v != "value" {
		t.Errorf("Get = (%q, %v)", v, ok)
	}
}

func TestGet_Missing(t *testing.T) {
	m := New(1 * time.Hour)
	_, ok := m.Get("missing")
	if ok {
		t.Error("should not find missing")
	}
}

func TestGet_Expired(t *testing.T) {
	m := New(1 * time.Millisecond)
	m.Set("key", "value")

	time.Sleep(5 * time.Millisecond)

	_, ok := m.Get("key")
	if ok {
		t.Error("should not find expired")
	}
}

func TestSetWithTTL(t *testing.T) {
	m := New(1 * time.Hour)
	m.SetWithTTL("short", "val", 1*time.Millisecond)

	time.Sleep(5 * time.Millisecond)

	_, ok := m.Get("short")
	if ok {
		t.Error("short TTL should expire")
	}
}

func TestDelete(t *testing.T) {
	m := New(1 * time.Hour)
	m.Set("key", "value")
	m.Delete("key")

	_, ok := m.Get("key")
	if ok {
		t.Error("should be deleted")
	}
}

func TestHas(t *testing.T) {
	m := New(1 * time.Hour)
	m.Set("key", "value")

	if !m.Has("key") {
		t.Error("should have key")
	}
	if m.Has("missing") {
		t.Error("should not have missing")
	}
}

func TestPurge(t *testing.T) {
	m := New(1 * time.Millisecond)
	m.Set("a", "1")
	m.Set("b", "2")

	time.Sleep(5 * time.Millisecond)

	removed := m.Purge()
	if removed != 2 {
		t.Errorf("removed = %d", removed)
	}
}

func TestClear(t *testing.T) {
	m := New(1 * time.Hour)
	m.Set("a", "1")
	m.Set("b", "2")
	m.Clear()

	if m.Len() != 0 {
		t.Errorf("Len = %d after clear", m.Len())
	}
}

func TestConcurrent(t *testing.T) {
	m := New(1 * time.Hour)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			m.Set("key", "value")
		}()
		go func() {
			defer wg.Done()
			m.Get("key")
		}()
		go func() {
			defer wg.Done()
			m.Has("key")
		}()
	}
	wg.Wait()
}
