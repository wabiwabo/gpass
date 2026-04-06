package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type mockPublisher struct {
	mu       sync.Mutex
	events   []Event
	failNext bool
	failErr  error
}

func (m *mockPublisher) Publish(_ context.Context, event Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		return m.failErr
	}
	m.events = append(m.events, event)
	return nil
}

func (m *mockPublisher) published() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Event, len(m.events))
	copy(result, m.events)
	return result
}

func TestNewEvent(t *testing.T) {
	payload := map[string]string{"nik": "3201120509870001"}
	event, err := NewEvent("evt-1", "user-123", "user.created", payload)
	if err != nil {
		t.Fatal(err)
	}

	if event.ID != "evt-1" {
		t.Errorf("ID: got %q, want %q", event.ID, "evt-1")
	}
	if event.AggregateID != "user-123" {
		t.Errorf("AggregateID: got %q, want %q", event.AggregateID, "user-123")
	}
	if event.EventType != "user.created" {
		t.Errorf("EventType: got %q, want %q", event.EventType, "user.created")
	}
	if event.Status != StatusPending {
		t.Errorf("Status: got %q, want %q", event.Status, StatusPending)
	}

	var p map[string]string
	json.Unmarshal(event.Payload, &p)
	if p["nik"] != "3201120509870001" {
		t.Errorf("payload nik: got %q", p["nik"])
	}
}

func TestNewEvent_InvalidPayload(t *testing.T) {
	_, err := NewEvent("id", "agg", "type", make(chan int))
	if err == nil {
		t.Error("should fail on non-marshalable payload")
	}
}

func TestMemoryStore_SaveAndFetch(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	e1, _ := NewEvent("1", "a", "test", nil)
	e2, _ := NewEvent("2", "b", "test", nil)

	store.Save(ctx, e1)
	store.Save(ctx, e2)

	if store.Count() != 2 {
		t.Errorf("count: got %d, want 2", store.Count())
	}

	pending, err := store.FetchPending(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Errorf("pending: got %d, want 2", len(pending))
	}
}

func TestMemoryStore_FetchPending_Limit(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		e, _ := NewEvent(string(rune('a'+i)), "agg", "test", nil)
		store.Save(ctx, e)
	}

	pending, _ := store.FetchPending(ctx, 3)
	if len(pending) != 3 {
		t.Errorf("pending: got %d, want 3", len(pending))
	}
}

func TestMemoryStore_MarkPublished(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	e, _ := NewEvent("1", "agg", "test", nil)
	store.Save(ctx, e)

	now := time.Now()
	err := store.MarkPublished(ctx, "1", now)
	if err != nil {
		t.Fatal(err)
	}

	got, ok := store.Get("1")
	if !ok {
		t.Fatal("event not found")
	}
	if got.Status != StatusPublished {
		t.Errorf("status: got %q, want %q", got.Status, StatusPublished)
	}
	if got.PublishedAt == nil {
		t.Error("publishedAt should be set")
	}

	// Published events should not appear in FetchPending.
	pending, _ := store.FetchPending(ctx, 10)
	if len(pending) != 0 {
		t.Errorf("pending after publish: got %d, want 0", len(pending))
	}
}

func TestMemoryStore_MarkFailed(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	e, _ := NewEvent("1", "agg", "test", nil)
	store.Save(ctx, e)

	err := store.MarkFailed(ctx, "1", "connection refused", 1)
	if err != nil {
		t.Fatal(err)
	}

	got, _ := store.Get("1")
	if got.Status != StatusFailed {
		t.Errorf("status: got %q, want %q", got.Status, StatusFailed)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts: got %d, want 1", got.Attempts)
	}
	if got.LastError != "connection refused" {
		t.Errorf("lastError: got %q", got.LastError)
	}
}

func TestMemoryStore_MarkPublished_NotFound(t *testing.T) {
	store := NewMemoryStore()
	err := store.MarkPublished(context.Background(), "nonexistent", time.Now())
	if !errors.Is(err, ErrEventNotFound) {
		t.Errorf("expected ErrEventNotFound, got: %v", err)
	}
}

func TestMemoryStore_MarkFailed_NotFound(t *testing.T) {
	store := NewMemoryStore()
	err := store.MarkFailed(context.Background(), "nonexistent", "err", 1)
	if !errors.Is(err, ErrEventNotFound) {
		t.Errorf("expected ErrEventNotFound, got: %v", err)
	}
}

func TestPoller_PublishesEvents(t *testing.T) {
	store := NewMemoryStore()
	pub := &mockPublisher{}

	e1, _ := NewEvent("1", "agg", "user.created", map[string]string{"name": "John"})
	e2, _ := NewEvent("2", "agg", "user.verified", nil)
	store.Save(context.Background(), e1)
	store.Save(context.Background(), e2)

	poller := NewPoller(store, pub, PollerConfig{
		Interval:    50 * time.Millisecond,
		MaxAttempts: 3,
	})

	poller.PollOnce()

	published := pub.published()
	if len(published) != 2 {
		t.Fatalf("published: got %d, want 2", len(published))
	}

	// Verify events were marked as published.
	got, _ := store.Get("1")
	if got.Status != StatusPublished {
		t.Errorf("event 1 status: got %q, want %q", got.Status, StatusPublished)
	}
}

func TestPoller_HandlesPublishFailure(t *testing.T) {
	store := NewMemoryStore()
	pub := &mockPublisher{failNext: true, failErr: errors.New("broker down")}

	e, _ := NewEvent("1", "agg", "test", nil)
	store.Save(context.Background(), e)

	poller := NewPoller(store, pub, PollerConfig{MaxAttempts: 3})
	poller.PollOnce()

	got, _ := store.Get("1")
	if got.Status != StatusFailed {
		t.Errorf("status: got %q, want %q", got.Status, StatusFailed)
	}
	if got.Attempts != 1 {
		t.Errorf("attempts: got %d, want 1", got.Attempts)
	}
}

func TestPoller_RetriesFailedEvents(t *testing.T) {
	store := NewMemoryStore()
	var callCount atomic.Int32
	pub := &mockPublisher{}

	e, _ := NewEvent("1", "agg", "test", nil)
	store.Save(context.Background(), e)

	// First poll: fail.
	pub.failNext = true
	pub.failErr = errors.New("temporary")
	poller := NewPoller(store, pub, PollerConfig{MaxAttempts: 3})
	poller.PollOnce()
	callCount.Add(1)

	// Second poll: succeed.
	pub.failNext = false
	poller.PollOnce()

	got, _ := store.Get("1")
	if got.Status != StatusPublished {
		t.Errorf("status after retry: got %q, want %q", got.Status, StatusPublished)
	}
}

func TestPoller_SkipsMaxAttempts(t *testing.T) {
	store := NewMemoryStore()
	pub := &mockPublisher{failNext: true, failErr: errors.New("permanent")}

	e, _ := NewEvent("1", "agg", "test", nil)
	store.Save(context.Background(), e)

	poller := NewPoller(store, pub, PollerConfig{MaxAttempts: 2})

	// Exhaust attempts.
	poller.PollOnce() // attempt 1
	poller.PollOnce() // attempt 2

	// Should not attempt again.
	pub.failNext = false
	poller.PollOnce()

	published := pub.published()
	if len(published) != 0 {
		t.Errorf("should not publish after max attempts, got %d", len(published))
	}
}

func TestPoller_StartStop(t *testing.T) {
	store := NewMemoryStore()
	pub := &mockPublisher{}

	e, _ := NewEvent("1", "agg", "test", nil)
	store.Save(context.Background(), e)

	poller := NewPoller(store, pub, PollerConfig{
		Interval: 20 * time.Millisecond,
	})

	poller.Start()
	time.Sleep(100 * time.Millisecond)
	poller.Stop()

	published := pub.published()
	if len(published) == 0 {
		t.Error("poller should have published at least one event")
	}
}

func TestPoller_DefaultConfig(t *testing.T) {
	poller := NewPoller(NewMemoryStore(), &mockPublisher{}, PollerConfig{})
	if poller.interval != time.Second {
		t.Errorf("default interval: got %v, want 1s", poller.interval)
	}
	if poller.batchSize != 100 {
		t.Errorf("default batchSize: got %d, want 100", poller.batchSize)
	}
	if poller.maxAttempts != 5 {
		t.Errorf("default maxAttempts: got %d, want 5", poller.maxAttempts)
	}
}

func TestPoller_ConcurrentAccess(t *testing.T) {
	store := NewMemoryStore()
	pub := &mockPublisher{}

	// Add events concurrently while poller is running.
	poller := NewPoller(store, pub, PollerConfig{
		Interval: 10 * time.Millisecond,
	})
	poller.Start()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			e, _ := NewEvent(
				string(rune('A'+n)),
				"agg",
				"test",
				map[string]int{"n": n},
			)
			store.Save(context.Background(), e)
		}(i)
	}
	wg.Wait()

	time.Sleep(100 * time.Millisecond)
	poller.Stop()

	// All events should be published.
	published := pub.published()
	if len(published) < 15 {
		t.Errorf("expected most events published, got %d/20", len(published))
	}
}

func TestMemoryStore_Get_NotFound(t *testing.T) {
	store := NewMemoryStore()
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("should return false for nonexistent event")
	}
}

func TestMemoryStore_FetchPending_IncludesFailed(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	e, _ := NewEvent("1", "agg", "test", nil)
	store.Save(ctx, e)
	store.MarkFailed(ctx, "1", "err", 1)

	pending, _ := store.FetchPending(ctx, 10)
	if len(pending) != 1 {
		t.Errorf("failed events should appear in FetchPending, got %d", len(pending))
	}
}
