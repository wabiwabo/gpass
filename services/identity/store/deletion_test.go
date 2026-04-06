package store

import (
	"errors"
	"testing"
	"time"
)

func TestInMemoryDeletionStore_CreateAndGetByID(t *testing.T) {
	s := NewInMemoryDeletionStore()

	req := &DeletionRequest{
		UserID: "user-001",
		Reason: "user_request",
	}

	if err := s.Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if req.ID == "" {
		t.Fatal("Create() did not set ID")
	}
	if req.Status != "PENDING" {
		t.Errorf("Create() status = %q, want %q", req.Status, "PENDING")
	}
	if req.RequestedAt.IsZero() {
		t.Error("Create() did not set RequestedAt")
	}

	got, err := s.GetByID(req.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.UserID != "user-001" {
		t.Errorf("GetByID() UserID = %q, want %q", got.UserID, "user-001")
	}
	if got.Reason != "user_request" {
		t.Errorf("GetByID() Reason = %q, want %q", got.Reason, "user_request")
	}
}

func TestInMemoryDeletionStore_ListByUser(t *testing.T) {
	s := NewInMemoryDeletionStore()

	for _, reason := range []string{"user_request", "consent_revocation"} {
		if err := s.Create(&DeletionRequest{UserID: "user-001", Reason: reason}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}
	if err := s.Create(&DeletionRequest{UserID: "user-002", Reason: "retention_expired"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	list, err := s.ListByUser("user-001")
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("ListByUser() returned %d items, want 2", len(list))
	}

	list2, err := s.ListByUser("user-002")
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if len(list2) != 1 {
		t.Errorf("ListByUser() returned %d items, want 1", len(list2))
	}
}

func TestInMemoryDeletionStore_UpdateStatus(t *testing.T) {
	s := NewInMemoryDeletionStore()

	req := &DeletionRequest{UserID: "user-001", Reason: "user_request"}
	if err := s.Create(req); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	now := time.Now().UTC()
	deletedData := []string{"personal_info", "biometric", "contact"}
	if err := s.UpdateStatus(req.ID, "COMPLETED", &now, deletedData); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	got, err := s.GetByID(req.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Status != "COMPLETED" {
		t.Errorf("Status = %q, want %q", got.Status, "COMPLETED")
	}
	if got.CompletedAt == nil {
		t.Fatal("CompletedAt is nil after update")
	}
	if len(got.DeletedData) != 3 {
		t.Errorf("DeletedData length = %d, want 3", len(got.DeletedData))
	}
}

func TestInMemoryDeletionStore_GetByID_NotFound(t *testing.T) {
	s := NewInMemoryDeletionStore()

	_, err := s.GetByID("nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByID() error = %v, want %v", err, ErrNotFound)
	}
}

func TestInMemoryDeletionStore_UpdateStatus_NotFound(t *testing.T) {
	s := NewInMemoryDeletionStore()

	err := s.UpdateStatus("nonexistent", "COMPLETED", nil, nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateStatus() error = %v, want %v", err, ErrNotFound)
	}
}

func TestInMemoryDeletionStore_InvalidReason(t *testing.T) {
	s := NewInMemoryDeletionStore()

	req := &DeletionRequest{UserID: "user-001", Reason: "invalid_reason"}
	err := s.Create(req)
	if !errors.Is(err, ErrInvalidReason) {
		t.Errorf("Create() error = %v, want %v", err, ErrInvalidReason)
	}
}

func TestInMemoryDeletionStore_ValidReasons(t *testing.T) {
	reasons := []string{"user_request", "consent_revocation", "retention_expired"}
	for _, reason := range reasons {
		t.Run(reason, func(t *testing.T) {
			s := NewInMemoryDeletionStore()
			req := &DeletionRequest{UserID: "user-001", Reason: reason}
			if err := s.Create(req); err != nil {
				t.Errorf("Create() with reason %q error = %v", reason, err)
			}
		})
	}
}

func TestInMemoryDeletionStore_ListByUser_Empty(t *testing.T) {
	s := NewInMemoryDeletionStore()

	list, err := s.ListByUser("nonexistent")
	if err != nil {
		t.Fatalf("ListByUser() error = %v", err)
	}
	if list != nil {
		t.Errorf("ListByUser() returned %v, want nil", list)
	}
}
