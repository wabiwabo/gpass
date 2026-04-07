package store

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDeletionConcurrentCreate(t *testing.T) {
	s := NewInMemoryDeletionStore()
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := &DeletionRequest{UserID: "user-concurrent", Reason: "user_request"}
			if err := s.Create(req); err != nil {
				t.Errorf("Create: %v", err)
			}
		}()
	}
	wg.Wait()

	list, _ := s.ListByUser("user-concurrent")
	if len(list) != n {
		t.Errorf("got %d, want %d", len(list), n)
	}
}

func TestDeletionConcurrentUpdate(t *testing.T) {
	s := NewInMemoryDeletionStore()

	// Create some requests
	ids := make([]string, 50)
	for i := 0; i < 50; i++ {
		req := &DeletionRequest{UserID: "user-update", Reason: "user_request"}
		_ = s.Create(req)
		ids[i] = req.ID
	}

	var wg sync.WaitGroup
	var success atomic.Int32
	now := time.Now()

	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			err := s.UpdateStatus(id, "COMPLETED", &now, []string{"personal_info"})
			if err == nil {
				success.Add(1)
			}
		}(id)
	}
	wg.Wait()

	if success.Load() != 50 {
		t.Errorf("updates: got %d, want 50", success.Load())
	}
}

func TestDeletionUniqueIDs(t *testing.T) {
	s := NewInMemoryDeletionStore()
	ids := make(map[string]bool)

	for i := 0; i < 200; i++ {
		req := &DeletionRequest{UserID: "user-id", Reason: "user_request"}
		_ = s.Create(req)
		if ids[req.ID] {
			t.Fatalf("duplicate ID: %s", req.ID)
		}
		ids[req.ID] = true
	}
}

func TestDeletionInvalidReason(t *testing.T) {
	s := NewInMemoryDeletionStore()

	tests := []string{"unknown", "spam", "", "USER_REQUEST"}
	for _, reason := range tests {
		t.Run(reason, func(t *testing.T) {
			req := &DeletionRequest{UserID: "u1", Reason: reason}
			err := s.Create(req)
			if err != ErrInvalidReason {
				t.Errorf("got %v, want ErrInvalidReason", err)
			}
		})
	}
}

func TestDeletionAllValidReasons(t *testing.T) {
	s := NewInMemoryDeletionStore()
	for reason := range ValidReasons {
		req := &DeletionRequest{UserID: "u1", Reason: reason}
		if err := s.Create(req); err != nil {
			t.Errorf("reason %q: %v", reason, err)
		}
	}
}

func TestDeletionGetByIDNotFound(t *testing.T) {
	s := NewInMemoryDeletionStore()
	_, err := s.GetByID("nonexistent")
	if err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestDeletionUpdateStatusNotFound(t *testing.T) {
	s := NewInMemoryDeletionStore()
	err := s.UpdateStatus("bad-id", "COMPLETED", nil, nil)
	if err != ErrNotFound {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestDeletionCopyIsolation(t *testing.T) {
	s := NewInMemoryDeletionStore()

	req := &DeletionRequest{UserID: "u1", Reason: "user_request"}
	_ = s.Create(req)
	now := time.Now()
	_ = s.UpdateStatus(req.ID, "COMPLETED", &now, []string{"personal_info", "biometric"})

	got, _ := s.GetByID(req.ID)
	got.DeletedData[0] = "TAMPERED"
	got.Status = "TAMPERED"

	got2, _ := s.GetByID(req.ID)
	if got2.Status == "TAMPERED" {
		t.Error("status mutated")
	}
	if got2.DeletedData[0] == "TAMPERED" {
		t.Error("deleted data mutated")
	}
}

func TestDeletionLifecycle(t *testing.T) {
	s := NewInMemoryDeletionStore()

	// Create
	req := &DeletionRequest{UserID: "u1", Reason: "user_request"}
	_ = s.Create(req)
	if req.Status != "PENDING" {
		t.Errorf("initial status: %q", req.Status)
	}
	if req.RequestedAt.IsZero() {
		t.Error("RequestedAt should be set")
	}

	// Update to PROCESSING
	_ = s.UpdateStatus(req.ID, "PROCESSING", nil, nil)
	got, _ := s.GetByID(req.ID)
	if got.Status != "PROCESSING" {
		t.Errorf("status: %q", got.Status)
	}

	// Update to COMPLETED with data categories
	now := time.Now()
	_ = s.UpdateStatus(req.ID, "COMPLETED", &now, []string{"personal_info", "biometric"})

	got, _ = s.GetByID(req.ID)
	if got.Status != "COMPLETED" {
		t.Errorf("final status: %q", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
	if len(got.DeletedData) != 2 {
		t.Errorf("deleted data: got %d", len(got.DeletedData))
	}
}

func TestDeletionListByUserIsolation(t *testing.T) {
	s := NewInMemoryDeletionStore()

	for i := 0; i < 5; i++ {
		_ = s.Create(&DeletionRequest{UserID: "alice", Reason: "user_request"})
	}
	for i := 0; i < 3; i++ {
		_ = s.Create(&DeletionRequest{UserID: "bob", Reason: "consent_revocation"})
	}

	alice, _ := s.ListByUser("alice")
	bob, _ := s.ListByUser("bob")

	if len(alice) != 5 {
		t.Errorf("alice: got %d, want 5", len(alice))
	}
	if len(bob) != 3 {
		t.Errorf("bob: got %d, want 3", len(bob))
	}
}

func TestDeletionListEmpty(t *testing.T) {
	s := NewInMemoryDeletionStore()
	list, _ := s.ListByUser("nobody")
	if len(list) != 0 {
		t.Errorf("got %d", len(list))
	}
}
