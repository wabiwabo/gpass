package store

import (
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

func TestRequestStore_CreateAndGet(t *testing.T) {
	store := NewInMemoryRequestStore()

	req := &signing.SigningRequest{
		UserID:       "user-1",
		DocumentName: "test.pdf",
		DocumentSize: 1024,
		DocumentHash: "abc123",
		DocumentPath: "files/test.pdf",
		Status:       "PENDING",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	}

	created, err := store.Create(req)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.ID == "" {
		t.Error("expected ID to be set")
	}

	got, err := store.GetByID(created.ID)
	if err != nil {
		t.Fatalf("get by ID: %v", err)
	}

	if got.DocumentName != "test.pdf" {
		t.Errorf("expected test.pdf, got %s", got.DocumentName)
	}
}

func TestRequestStore_UpdateStatus(t *testing.T) {
	store := NewInMemoryRequestStore()

	req, _ := store.Create(&signing.SigningRequest{
		UserID:    "user-1",
		Status:    "PENDING",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	})

	err := store.UpdateStatus(req.ID, "COMPLETED", "cert-123", "")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := store.GetByID(req.ID)
	if got.Status != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %s", got.Status)
	}
	if got.CertificateID != "cert-123" {
		t.Errorf("expected cert-123, got %s", got.CertificateID)
	}
}

func TestRequestStore_ListByUser(t *testing.T) {
	store := NewInMemoryRequestStore()

	store.Create(&signing.SigningRequest{UserID: "user-1", Status: "PENDING", ExpiresAt: time.Now()})
	store.Create(&signing.SigningRequest{UserID: "user-1", Status: "COMPLETED", ExpiresAt: time.Now()})
	store.Create(&signing.SigningRequest{UserID: "user-2", Status: "PENDING", ExpiresAt: time.Now()})

	reqs, err := store.ListByUser("user-1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(reqs) != 2 {
		t.Errorf("expected 2 requests, got %d", len(reqs))
	}
}

func TestRequestStore_NotFound(t *testing.T) {
	store := NewInMemoryRequestStore()

	_, err := store.GetByID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}
