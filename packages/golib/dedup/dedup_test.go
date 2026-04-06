package dedup

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestProcessor_FirstProcessing(t *testing.T) {
	store := NewMemoryStore()
	p := NewProcessor(store, 1*time.Hour)

	processed, err := p.Process("evt-1", func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Error("first call should be processed")
	}
}

func TestProcessor_DuplicateRejected(t *testing.T) {
	store := NewMemoryStore()
	p := NewProcessor(store, 1*time.Hour)

	p.Process("evt-1", func() error { return nil })

	processed, err := p.Process("evt-1", func() error {
		t.Error("duplicate should not be processed")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if processed {
		t.Error("duplicate should return false")
	}
}

func TestProcessor_DifferentKeysProcessed(t *testing.T) {
	store := NewMemoryStore()
	p := NewProcessor(store, 1*time.Hour)

	p.Process("evt-1", func() error { return nil })

	processed, err := p.Process("evt-2", func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !processed {
		t.Error("different key should be processed")
	}
}

func TestProcessor_ErrorReturned(t *testing.T) {
	store := NewMemoryStore()
	p := NewProcessor(store, 1*time.Hour)

	testErr := errors.New("processing failed")
	_, err := p.Process("evt-1", func() error { return testErr })
	if !errors.Is(err, testErr) {
		t.Errorf("should return processing error: got %v", err)
	}

	// Key should NOT be marked since processing failed.
	processed, _ := p.Process("evt-1", func() error { return nil })
	if !processed {
		t.Error("should be processable after failure")
	}
}

func TestProcessor_DefaultTTL(t *testing.T) {
	store := NewMemoryStore()
	p := NewProcessor(store, 0) // Should default to 24h.

	p.Process("evt-1", func() error { return nil })
	if store.Size() != 1 {
		t.Error("should mark key")
	}
}

func TestMemoryStore_Expiry(t *testing.T) {
	store := NewMemoryStore()
	store.Mark("key-1", 10*time.Millisecond)

	exists, _ := store.Exists("key-1")
	if !exists {
		t.Error("should exist before expiry")
	}

	time.Sleep(20 * time.Millisecond)

	exists, _ = store.Exists("key-1")
	if exists {
		t.Error("should not exist after expiry")
	}
}

func TestMemoryStore_Remove(t *testing.T) {
	store := NewMemoryStore()
	store.Mark("key-1", 1*time.Hour)

	store.Remove("key-1")

	exists, _ := store.Exists("key-1")
	if exists {
		t.Error("should not exist after remove")
	}
}

func TestMemoryStore_Size(t *testing.T) {
	store := NewMemoryStore()
	store.Mark("a", 1*time.Hour)
	store.Mark("b", 1*time.Hour)

	if store.Size() != 2 {
		t.Errorf("size: got %d", store.Size())
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	store := NewMemoryStore()
	store.Mark("expired", 1*time.Millisecond)
	store.Mark("valid", 1*time.Hour)

	time.Sleep(5 * time.Millisecond)
	removed := store.Cleanup()

	if removed != 1 {
		t.Errorf("removed: got %d", removed)
	}
	if store.Size() != 1 {
		t.Errorf("remaining: got %d", store.Size())
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := ContentKey("test", []byte{byte(n)})
			store.Mark(key, 1*time.Hour)
			store.Exists(key)
		}(i)
	}
	wg.Wait()

	if store.Size() != 100 {
		t.Errorf("size after concurrent: got %d", store.Size())
	}
}

func TestContentKey_Deterministic(t *testing.T) {
	k1 := ContentKey("user.created", []byte(`{"id":"1"}`))
	k2 := ContentKey("user.created", []byte(`{"id":"1"}`))

	if k1 != k2 {
		t.Error("same input should produce same key")
	}

	k3 := ContentKey("user.deleted", []byte(`{"id":"1"}`))
	if k1 == k3 {
		t.Error("different type should produce different key")
	}
}

func TestContentKey_DifferentData(t *testing.T) {
	k1 := ContentKey("test", []byte(`{"a":"1"}`))
	k2 := ContentKey("test", []byte(`{"a":"2"}`))

	if k1 == k2 {
		t.Error("different data should produce different keys")
	}
}

func TestCompositeKey(t *testing.T) {
	k1 := CompositeKey("tenant-1", "user-1", "action-create")
	k2 := CompositeKey("tenant-1", "user-1", "action-create")

	if k1 != k2 {
		t.Error("same parts should produce same key")
	}

	k3 := CompositeKey("tenant-1", "user-2", "action-create")
	if k1 == k3 {
		t.Error("different parts should produce different keys")
	}
}

func TestCompositeKey_Length(t *testing.T) {
	k := CompositeKey("a", "b", "c")
	if len(k) != 64 { // SHA-256 hex = 64 chars.
		t.Errorf("key length: got %d", len(k))
	}
}
