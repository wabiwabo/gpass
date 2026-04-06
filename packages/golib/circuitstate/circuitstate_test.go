package circuitstate

import (
	"encoding/json"
	"testing"
)

func TestMemoryStore_SaveLoad(t *testing.T) {
	store := NewMemoryStore()

	state := State{Name: "db", Status: "closed", Failures: 0}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load("db")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != "closed" {
		t.Errorf("status: got %q", loaded.Status)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("updated_at should be set")
	}
}

func TestMemoryStore_LoadNotFound(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.Load("nonexistent")
	if err == nil {
		t.Error("should fail for missing key")
	}
}

func TestMemoryStore_LoadAll(t *testing.T) {
	store := NewMemoryStore()
	store.Save(State{Name: "a", Status: "closed"})
	store.Save(State{Name: "b", Status: "open"})

	all, err := store.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("count: got %d", len(all))
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore()
	store.Save(State{Name: "a", Status: "closed"})
	store.Delete("a")

	if store.Size() != 0 {
		t.Error("should be empty after delete")
	}
}

func TestMemoryStore_Size(t *testing.T) {
	store := NewMemoryStore()
	store.Save(State{Name: "a", Status: "closed"})
	store.Save(State{Name: "b", Status: "open"})

	if store.Size() != 2 {
		t.Errorf("size: got %d", store.Size())
	}
}

func TestManager_Record(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)

	err := mgr.Record("db-circuit", "open", 5, 0)
	if err != nil {
		t.Fatal(err)
	}

	state, err := mgr.Get("db-circuit")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "open" {
		t.Errorf("status: got %q", state.Status)
	}
	if state.Failures != 5 {
		t.Errorf("failures: got %d", state.Failures)
	}
}

func TestManager_GetFromCache(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)

	mgr.Record("cached", "closed", 0, 10)

	// Should hit cache, not store.
	state, err := mgr.Get("cached")
	if err != nil {
		t.Fatal(err)
	}
	if state.Successes != 10 {
		t.Errorf("successes: got %d", state.Successes)
	}
}

func TestManager_GetFromStore(t *testing.T) {
	store := NewMemoryStore()
	store.Save(State{Name: "stored", Status: "open", Failures: 3})

	mgr := NewManager(store) // Cache is empty.

	state, err := mgr.Get("stored")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != "open" {
		t.Errorf("status: got %q", state.Status)
	}
}

func TestManager_Restore(t *testing.T) {
	store := NewMemoryStore()
	store.Save(State{Name: "a", Status: "closed"})
	store.Save(State{Name: "b", Status: "open"})

	mgr := NewManager(store)
	if err := mgr.Restore(); err != nil {
		t.Fatal(err)
	}

	if mgr.Count() != 2 {
		t.Errorf("count after restore: got %d", mgr.Count())
	}
}

func TestManager_Serialize(t *testing.T) {
	store := NewMemoryStore()
	mgr := NewManager(store)
	mgr.Record("db", "closed", 0, 5)
	mgr.Record("cache", "open", 3, 0)

	data, err := mgr.Serialize()
	if err != nil {
		t.Fatal(err)
	}

	var states []State
	if err := json.Unmarshal(data, &states); err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Errorf("serialized: got %d", len(states))
	}
}

func TestManager_Count(t *testing.T) {
	mgr := NewManager(NewMemoryStore())
	mgr.Record("a", "closed", 0, 0)
	mgr.Record("b", "closed", 0, 0)

	if mgr.Count() != 2 {
		t.Errorf("count: got %d", mgr.Count())
	}
}

func TestManager_GetNotFound(t *testing.T) {
	mgr := NewManager(NewMemoryStore())
	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("should fail for missing circuit")
	}
}

func TestManager_OpenState_SetsTimestamps(t *testing.T) {
	mgr := NewManager(NewMemoryStore())
	mgr.Record("db", "open", 5, 0)

	state, _ := mgr.Get("db")
	if state.OpenedAt.IsZero() {
		t.Error("open state should set opened_at")
	}
	if state.LastFailure.IsZero() {
		t.Error("open state should set last_failure")
	}
}

func TestManager_ClosedState_SetsSuccess(t *testing.T) {
	mgr := NewManager(NewMemoryStore())
	mgr.Record("db", "closed", 0, 5)

	state, _ := mgr.Get("db")
	if state.LastSuccess.IsZero() {
		t.Error("success should set last_success")
	}
}
