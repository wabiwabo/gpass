package eventsource

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
)

// testAggregate is a simple aggregate used for testing.
type testAggregate struct {
	id      string
	version int
	name    string
}

func (a *testAggregate) ID() string   { return a.id }
func (a *testAggregate) Type() string { return "test" }
func (a *testAggregate) Version() int { return a.version }

func (a *testAggregate) ApplyEvent(e Event) error {
	a.id = e.AggregateID
	a.version = e.Version

	if e.EventType == "named" {
		var payload struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(e.Payload, &payload); err != nil {
			return err
		}
		a.name = payload.Name
	}
	return nil
}

func TestNewEvent(t *testing.T) {
	payload := map[string]string{"name": "alice"}
	ev, err := NewEvent("agg-1", "user", "created", 1, payload)
	if err != nil {
		t.Fatalf("NewEvent returned error: %v", err)
	}

	if ev.AggregateID != "agg-1" {
		t.Errorf("AggregateID = %q, want %q", ev.AggregateID, "agg-1")
	}
	if ev.AggregateType != "user" {
		t.Errorf("AggregateType = %q, want %q", ev.AggregateType, "user")
	}
	if ev.EventType != "created" {
		t.Errorf("EventType = %q, want %q", ev.EventType, "created")
	}
	if ev.Version != 1 {
		t.Errorf("Version = %d, want %d", ev.Version, 1)
	}
	if ev.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}

	var decoded map[string]string
	if err := json.Unmarshal(ev.Payload, &decoded); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if decoded["name"] != "alice" {
		t.Errorf("payload name = %q, want %q", decoded["name"], "alice")
	}
}

func TestMemoryEventStore_SaveAndLoad(t *testing.T) {
	store := NewMemoryEventStore()
	ctx := context.Background()

	events := []Event{
		{AggregateID: "agg-1", EventType: "created", Version: 1, Payload: json.RawMessage(`{}`)},
		{AggregateID: "agg-1", EventType: "updated", Version: 2, Payload: json.RawMessage(`{}`)},
	}

	if err := store.Save(ctx, events); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("loaded %d events, want 2", len(loaded))
	}
	if loaded[0].EventType != "created" {
		t.Errorf("first event type = %q, want %q", loaded[0].EventType, "created")
	}
	if loaded[1].EventType != "updated" {
		t.Errorf("second event type = %q, want %q", loaded[1].EventType, "updated")
	}
}

func TestMemoryEventStore_LoadFrom(t *testing.T) {
	store := NewMemoryEventStore()
	ctx := context.Background()

	events := []Event{
		{AggregateID: "agg-1", EventType: "e1", Version: 1, Payload: json.RawMessage(`{}`)},
		{AggregateID: "agg-1", EventType: "e2", Version: 2, Payload: json.RawMessage(`{}`)},
		{AggregateID: "agg-1", EventType: "e3", Version: 3, Payload: json.RawMessage(`{}`)},
	}

	if err := store.Save(ctx, events); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := store.LoadFrom(ctx, "agg-1", 2)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("loaded %d events, want 2", len(loaded))
	}
	if loaded[0].Version != 2 {
		t.Errorf("first event version = %d, want 2", loaded[0].Version)
	}
	if loaded[1].Version != 3 {
		t.Errorf("second event version = %d, want 3", loaded[1].Version)
	}
}

func TestMemoryEventStore_EmptyAggregate(t *testing.T) {
	store := NewMemoryEventStore()
	ctx := context.Background()

	loaded, err := store.Load(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("loaded %d events, want 0", len(loaded))
	}

	loadedFrom, err := store.LoadFrom(ctx, "nonexistent", 1)
	if err != nil {
		t.Fatalf("LoadFrom returned error: %v", err)
	}
	if len(loadedFrom) != 0 {
		t.Errorf("loaded %d events from LoadFrom, want 0", len(loadedFrom))
	}
}

func TestMemorySnapshotStore_SaveAndLoad(t *testing.T) {
	store := NewMemorySnapshotStore()
	ctx := context.Background()

	snap := Snapshot{
		AggregateID:   "agg-1",
		AggregateType: "user",
		Version:       5,
		Data:          json.RawMessage(`{"name":"alice"}`),
	}

	if err := store.SaveSnapshot(ctx, snap); err != nil {
		t.Fatalf("SaveSnapshot returned error: %v", err)
	}

	loaded, err := store.LoadSnapshot(ctx, "agg-1")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadSnapshot returned nil")
	}
	if loaded.AggregateID != "agg-1" {
		t.Errorf("AggregateID = %q, want %q", loaded.AggregateID, "agg-1")
	}
	if loaded.Version != 5 {
		t.Errorf("Version = %d, want 5", loaded.Version)
	}

	// Loading a nonexistent snapshot should return nil.
	missing, err := store.LoadSnapshot(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("LoadSnapshot returned error: %v", err)
	}
	if missing != nil {
		t.Errorf("expected nil snapshot, got %+v", missing)
	}
}

func TestRepository_Load(t *testing.T) {
	store := NewMemoryEventStore()
	ctx := context.Background()

	e1, _ := NewEvent("agg-1", "test", "named", 1, map[string]string{"name": "alice"})
	e2, _ := NewEvent("agg-1", "test", "named", 2, map[string]string{"name": "bob"})

	if err := store.Save(ctx, []Event{e1, e2}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	repo := NewRepository[*testAggregate](store, func() *testAggregate {
		return &testAggregate{}
	})

	agg, err := repo.Load(ctx, "agg-1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if agg.ID() != "agg-1" {
		t.Errorf("ID = %q, want %q", agg.ID(), "agg-1")
	}
	if agg.Version() != 2 {
		t.Errorf("Version = %d, want 2", agg.Version())
	}
	if agg.name != "bob" {
		t.Errorf("name = %q, want %q", agg.name, "bob")
	}
}

func TestRepository_Save(t *testing.T) {
	store := NewMemoryEventStore()
	ctx := context.Background()

	repo := NewRepository[*testAggregate](store, func() *testAggregate {
		return &testAggregate{}
	})

	agg := &testAggregate{id: "agg-1"}
	e1, _ := NewEvent("agg-1", "test", "named", 1, map[string]string{"name": "alice"})

	if err := repo.Save(ctx, agg, []Event{e1}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d events, want 1", len(loaded))
	}
	if loaded[0].EventType != "named" {
		t.Errorf("EventType = %q, want %q", loaded[0].EventType, "named")
	}
}

func TestRepository_ConcurrentAccess(t *testing.T) {
	store := NewMemoryEventStore()
	ctx := context.Background()

	repo := NewRepository[*testAggregate](store, func() *testAggregate {
		return &testAggregate{}
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			ev, _ := NewEvent("agg-concurrent", "test", "named", n, map[string]string{"name": "test"})
			_ = repo.Save(ctx, &testAggregate{id: "agg-concurrent"}, []Event{ev})
			_, _ = repo.Load(ctx, "agg-concurrent")
		}(i)
	}
	wg.Wait()

	loaded, err := store.Load(ctx, "agg-concurrent")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if len(loaded) != 50 {
		t.Errorf("loaded %d events, want 50", len(loaded))
	}
}

func TestEvent_Ordering(t *testing.T) {
	store := NewMemoryEventStore()
	ctx := context.Background()

	// Save events out of order.
	events := []Event{
		{AggregateID: "agg-1", EventType: "e3", Version: 3, Payload: json.RawMessage(`{}`)},
		{AggregateID: "agg-1", EventType: "e1", Version: 1, Payload: json.RawMessage(`{}`)},
		{AggregateID: "agg-1", EventType: "e2", Version: 2, Payload: json.RawMessage(`{}`)},
	}

	if err := store.Save(ctx, events); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	for i, e := range loaded {
		expected := i + 1
		if e.Version != expected {
			t.Errorf("event[%d].Version = %d, want %d", i, e.Version, expected)
		}
	}
}
