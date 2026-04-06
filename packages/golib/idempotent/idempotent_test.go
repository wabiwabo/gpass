package idempotent

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestMemoryStore_Lock_NewKey(t *testing.T) {
	store := NewMemoryStore()
	locked, err := store.Lock(context.Background(), "key-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !locked {
		t.Error("new key should be locked")
	}
}

func TestMemoryStore_Lock_DuplicateKey(t *testing.T) {
	store := NewMemoryStore()
	store.Lock(context.Background(), "key-1", time.Hour)

	locked, err := store.Lock(context.Background(), "key-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if locked {
		t.Error("duplicate key should not lock")
	}
}

func TestMemoryStore_Lock_RetryAfterFail(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	store.Lock(ctx, "key-1", time.Hour)
	store.Fail(ctx, "key-1")

	// After failure, the same key should be lockable again.
	locked, err := store.Lock(ctx, "key-1", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !locked {
		t.Error("failed key should allow retry")
	}
}

func TestMemoryStore_Complete(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	store.Lock(ctx, "key-1", time.Hour)
	body := json.RawMessage(`{"id":"123"}`)
	headers := map[string]string{"X-Request-Id": "req-1"}

	err := store.Complete(ctx, "key-1", 201, headers, body)
	if err != nil {
		t.Fatal(err)
	}

	entry, _ := store.Get(ctx, "key-1")
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.Status != StatusCompleted {
		t.Errorf("status: got %q, want %q", entry.Status, StatusCompleted)
	}
	if entry.StatusCode != 201 {
		t.Errorf("statusCode: got %d, want 201", entry.StatusCode)
	}
	if entry.CompletedAt == nil {
		t.Error("completedAt should be set")
	}
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	store := NewMemoryStore()
	entry, err := store.Get(context.Background(), "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if entry != nil {
		t.Error("expected nil for missing key")
	}
}

func TestMemoryStore_Count(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	store.Lock(ctx, "a", time.Hour)
	store.Lock(ctx, "b", time.Hour)
	store.Lock(ctx, "c", time.Hour)

	if store.Count() != 3 {
		t.Errorf("count: got %d, want 3", store.Count())
	}
}

func TestMemoryStore_Cleanup(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	store.Lock(ctx, "old", time.Hour)
	// Manually set old created_at.
	store.mu.Lock()
	store.entries["old"].CreatedAt = time.Now().Add(-2 * time.Hour)
	store.mu.Unlock()

	store.Lock(ctx, "new", time.Hour)

	removed := store.Cleanup(time.Hour)
	if removed != 1 {
		t.Errorf("removed: got %d, want 1", removed)
	}
	if store.Count() != 1 {
		t.Errorf("remaining: got %d, want 1", store.Count())
	}
}

func TestHandler_Check_NewRequest(t *testing.T) {
	h := NewHandler(NewMemoryStore(), time.Hour)

	entry, isNew, err := h.Check(context.Background(), "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if !isNew {
		t.Error("should be new request")
	}
	if entry != nil {
		t.Error("entry should be nil for new request")
	}
}

func TestHandler_Check_DuplicateRequest(t *testing.T) {
	store := NewMemoryStore()
	h := NewHandler(store, time.Hour)
	ctx := context.Background()

	// Process first request.
	h.Check(ctx, "idem-1")
	h.Complete(ctx, "idem-1", 200, nil, json.RawMessage(`{"ok":true}`))

	// Duplicate request.
	entry, isNew, err := h.Check(ctx, "idem-1")
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Error("should not be new (duplicate)")
	}
	if entry == nil {
		t.Fatal("expected existing entry")
	}
	if entry.StatusCode != 200 {
		t.Errorf("statusCode: got %d", entry.StatusCode)
	}
}

func TestHandler_Check_EmptyKey(t *testing.T) {
	h := NewHandler(NewMemoryStore(), time.Hour)

	_, isNew, err := h.Check(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !isNew {
		t.Error("empty key should always be new")
	}
}

func TestHandler_Complete_EmptyKey(t *testing.T) {
	h := NewHandler(NewMemoryStore(), time.Hour)
	err := h.Complete(context.Background(), "", 200, nil, nil)
	if err != nil {
		t.Error("empty key complete should be no-op")
	}
}

func TestHandler_Fail_AllowsRetry(t *testing.T) {
	h := NewHandler(NewMemoryStore(), time.Hour)
	ctx := context.Background()

	h.Check(ctx, "key")
	h.Fail(ctx, "key")

	// Should be able to retry.
	_, isNew, err := h.Check(ctx, "key")
	if err != nil {
		t.Fatal(err)
	}
	if !isNew {
		t.Error("failed key should allow retry")
	}
}

func TestHandler_DefaultTTL(t *testing.T) {
	h := NewHandler(NewMemoryStore(), 0) // 0 TTL should default
	if h.ttl != 24*time.Hour {
		t.Errorf("default TTL: got %v, want 24h", h.ttl)
	}
}

func TestHandler_ConcurrentAccess(t *testing.T) {
	h := NewHandler(NewMemoryStore(), time.Hour)
	ctx := context.Background()

	var wg sync.WaitGroup
	results := make([]bool, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, isNew, _ := h.Check(ctx, "shared-key")
			results[idx] = isNew
			if isNew {
				h.Complete(ctx, "shared-key", 200, nil, json.RawMessage(`{}`))
			}
		}(i)
	}
	wg.Wait()

	newCount := 0
	for _, isNew := range results {
		if isNew {
			newCount++
		}
	}

	if newCount != 1 {
		t.Errorf("exactly 1 request should be new, got %d", newCount)
	}
}
