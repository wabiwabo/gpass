package store

import (
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

func TestCertificateStore_CreateAndGet(t *testing.T) {
	store := NewInMemoryCertificateStore()

	cert := &signing.Certificate{
		UserID:       "user-1",
		SerialNumber: "SN001",
		IssuerDN:     "CN=Test CA",
		SubjectDN:    "CN=Test User",
		Status:       "ACTIVE",
		ValidFrom:    time.Now(),
		ValidTo:      time.Now().Add(365 * 24 * time.Hour),
	}

	created, err := store.Create(cert)
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

	if got.SerialNumber != "SN001" {
		t.Errorf("expected SN001, got %s", got.SerialNumber)
	}
}

func TestCertificateStore_ListByUser(t *testing.T) {
	store := NewInMemoryCertificateStore()

	store.Create(&signing.Certificate{UserID: "user-1", Status: "ACTIVE", SerialNumber: "SN1"})
	store.Create(&signing.Certificate{UserID: "user-1", Status: "REVOKED", SerialNumber: "SN2"})
	store.Create(&signing.Certificate{UserID: "user-2", Status: "ACTIVE", SerialNumber: "SN3"})

	certs, err := store.ListByUser("user-1", "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(certs) != 2 {
		t.Errorf("expected 2 certs, got %d", len(certs))
	}
}

func TestCertificateStore_ListByUserWithStatusFilter(t *testing.T) {
	store := NewInMemoryCertificateStore()

	store.Create(&signing.Certificate{UserID: "user-1", Status: "ACTIVE", SerialNumber: "SN1"})
	store.Create(&signing.Certificate{UserID: "user-1", Status: "REVOKED", SerialNumber: "SN2"})

	certs, err := store.ListByUser("user-1", "ACTIVE")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(certs) != 1 {
		t.Errorf("expected 1 cert, got %d", len(certs))
	}
}

func TestCertificateStore_UpdateStatus(t *testing.T) {
	store := NewInMemoryCertificateStore()

	cert, _ := store.Create(&signing.Certificate{
		UserID:       "user-1",
		Status:       "ACTIVE",
		SerialNumber: "SN1",
	})

	now := time.Now()
	err := store.UpdateStatus(cert.ID, "REVOKED", &now, "key_compromise")
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := store.GetByID(cert.ID)
	if got.Status != "REVOKED" {
		t.Errorf("expected REVOKED, got %s", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("expected RevokedAt to be set")
	}
	if got.RevocationReason != "key_compromise" {
		t.Errorf("expected key_compromise, got %s", got.RevocationReason)
	}
}

func TestCertificateStore_GetByIDNotFound(t *testing.T) {
	store := NewInMemoryCertificateStore()

	_, err := store.GetByID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestCertificateStore_GetActiveByUser(t *testing.T) {
	store := NewInMemoryCertificateStore()

	store.Create(&signing.Certificate{UserID: "user-1", Status: "ACTIVE", SerialNumber: "SN1"})

	cert, err := store.GetActiveByUser("user-1")
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if cert.Status != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %s", cert.Status)
	}

	_, err = store.GetActiveByUser("user-nonexistent")
	if err == nil {
		t.Error("expected error for user with no active cert")
	}
}
