package store

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrentAppend(t *testing.T) {
	s := NewInMemoryAuditStore()
	var wg sync.WaitGroup
	n := 200

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			e := &AuditEvent{
				EventType: "LOGIN",
				ActorID:   "user-concurrent",
				Action:    "authenticate",
			}
			if err := s.Append(e); err != nil {
				t.Errorf("Append %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	count, _ := s.Count(AuditFilter{ActorID: "user-concurrent"})
	if count != int64(n) {
		t.Errorf("count: got %d, want %d", count, n)
	}
}

func TestConcurrentAppendAndQuery(t *testing.T) {
	s := NewInMemoryAuditStore()
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := &AuditEvent{
				EventType: "DATA_ACCESS",
				ActorID:   "user-rw",
				Action:    "read",
			}
			_ = s.Append(e)
		}()
	}

	// Readers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Query(AuditFilter{ActorID: "user-rw"})
		}()
	}

	// Counters
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Count(AuditFilter{ActorID: "user-rw"})
		}()
	}

	wg.Wait()
}

func TestConcurrentGetByID(t *testing.T) {
	s := NewInMemoryAuditStore()

	// Create some events
	ids := make([]string, 50)
	for i := 0; i < 50; i++ {
		e := &AuditEvent{
			EventType: "AUDIT",
			ActorID:   "user-get",
			Action:    "create",
		}
		_ = s.Append(e)
		ids[i] = e.ID
	}

	var wg sync.WaitGroup
	var notFound atomic.Int32

	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := s.GetByID(ids[idx%50])
			if err != nil {
				notFound.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if notFound.Load() != 0 {
		t.Errorf("unexpected not-found errors: %d", notFound.Load())
	}
}

func TestAppendUniqueIDs(t *testing.T) {
	s := NewInMemoryAuditStore()
	ids := make(map[string]bool)

	for i := 0; i < 500; i++ {
		e := &AuditEvent{
			EventType: "TEST",
			ActorID:   "user-unique",
			Action:    "test",
		}
		_ = s.Append(e)
		if ids[e.ID] {
			t.Fatalf("duplicate ID: %s at iteration %d", e.ID, i)
		}
		ids[e.ID] = true
	}
}

func TestAppendImmutability(t *testing.T) {
	s := NewInMemoryAuditStore()
	e := &AuditEvent{
		EventType: "LOGIN",
		ActorID:   "user-1",
		Action:    "authenticate",
		Metadata:  map[string]string{"ip": "1.2.3.4"},
	}
	_ = s.Append(e)

	// Mutate the original
	e.Metadata["ip"] = "TAMPERED"
	e.Action = "TAMPERED"

	// Stored version should be unaffected
	stored, _ := s.GetByID(e.ID)
	if stored.Metadata["ip"] != "1.2.3.4" {
		t.Error("store metadata should be immutable")
	}
	if stored.Action != "authenticate" {
		t.Error("store action should be immutable")
	}
}

func TestQueryImmutability(t *testing.T) {
	s := NewInMemoryAuditStore()
	e := &AuditEvent{
		EventType: "LOGIN",
		ActorID:   "user-qi",
		Action:    "authenticate",
		Metadata:  map[string]string{"key": "original"},
	}
	_ = s.Append(e)

	results, _ := s.Query(AuditFilter{ActorID: "user-qi"})
	results[0].Metadata["key"] = "TAMPERED"

	// Re-query should still have original
	results2, _ := s.Query(AuditFilter{ActorID: "user-qi"})
	if results2[0].Metadata["key"] != "original" {
		t.Error("query results should be copies")
	}
}

func TestGetByIDImmutability(t *testing.T) {
	s := NewInMemoryAuditStore()
	e := &AuditEvent{
		EventType: "LOGIN",
		ActorID:   "user-gi",
		Action:    "auth",
		Metadata:  map[string]string{"k": "v"},
	}
	_ = s.Append(e)

	got, _ := s.GetByID(e.ID)
	got.Metadata["k"] = "TAMPERED"

	got2, _ := s.GetByID(e.ID)
	if got2.Metadata["k"] != "v" {
		t.Error("GetByID should return copies")
	}
}
