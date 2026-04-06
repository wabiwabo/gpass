package store

import (
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

func TestDocumentStore_CreateAndGet(t *testing.T) {
	store := NewInMemoryDocumentStore()

	doc := &signing.SignedDocument{
		RequestID:          "req-1",
		CertificateID:      "cert-1",
		SignedHash:         "hash123",
		SignedPath:         "signed/test.pdf",
		SignedSize:         2048,
		PAdESLevel:         "PAdES-B-LTA",
		SignatureTimestamp: time.Now(),
	}

	created, err := store.Create(doc)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.ID == "" {
		t.Error("expected ID to be set")
	}

	got, err := store.GetByRequestID("req-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if got.SignedHash != "hash123" {
		t.Errorf("expected hash123, got %s", got.SignedHash)
	}
}

func TestDocumentStore_NotFound(t *testing.T) {
	store := NewInMemoryDocumentStore()

	_, err := store.GetByRequestID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent request ID")
	}
}

func TestDocumentStore_DuplicateRequestID(t *testing.T) {
	store := NewInMemoryDocumentStore()

	doc := &signing.SignedDocument{
		RequestID:          "req-1",
		CertificateID:      "cert-1",
		SignedHash:         "hash123",
		SignedPath:         "signed/test.pdf",
		SignedSize:         2048,
		PAdESLevel:         "PAdES-B-LTA",
		SignatureTimestamp: time.Now(),
	}

	_, err := store.Create(doc)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}

	dup := &signing.SignedDocument{
		RequestID:          "req-1",
		CertificateID:      "cert-2",
		SignedHash:         "hash456",
		SignedPath:         "signed/test2.pdf",
		SignedSize:         4096,
		PAdESLevel:         "PAdES-B-LTA",
		SignatureTimestamp: time.Now(),
	}

	_, err = store.Create(dup)
	if err == nil {
		t.Error("expected error for duplicate request ID")
	}
}
