package store

import (
	"sync"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

func TestCertificateConcurrentCreate(t *testing.T) {
	s := NewInMemoryCertificateStore()
	var wg sync.WaitGroup
	n := 100

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cert := &signing.Certificate{
				UserID:            "user-concurrent",
				Status:            "ACTIVE",
				SubjectDN:         "CN=Test User",
				FingerprintSHA256: "abc123",
			}
			_, err := s.Create(cert)
			if err != nil {
				t.Errorf("Create: %v", err)
			}
		}()
	}
	wg.Wait()

	list, _ := s.ListByUser("user-concurrent", "")
	if len(list) != n {
		t.Errorf("certs: got %d, want %d", len(list), n)
	}
}

func TestCertificateConcurrentReadWrite(t *testing.T) {
	s := NewInMemoryCertificateStore()

	// Create some certs
	ids := make([]string, 20)
	for i := 0; i < 20; i++ {
		cert := &signing.Certificate{UserID: "user-rw", Status: "ACTIVE"}
		created, _ := s.Create(cert)
		ids[i] = created.ID
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _ = s.GetByID(ids[idx%20])
		}(i)
	}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.ListByUser("user-rw", "")
		}()
	}
	wg.Wait()
}

func TestCertificateUniqueIDs(t *testing.T) {
	s := NewInMemoryCertificateStore()
	ids := make(map[string]bool)

	for i := 0; i < 200; i++ {
		cert := &signing.Certificate{UserID: "user-id", Status: "ACTIVE"}
		created, _ := s.Create(cert)
		if ids[created.ID] {
			t.Fatalf("duplicate ID: %s", created.ID)
		}
		ids[created.ID] = true
	}
}

func TestGetActiveByUser(t *testing.T) {
	s := NewInMemoryCertificateStore()

	// Create active cert
	cert1, _ := s.Create(&signing.Certificate{UserID: "user-1", Status: "ACTIVE"})
	// Create revoked cert
	cert2, _ := s.Create(&signing.Certificate{UserID: "user-1", Status: "REVOKED"})

	got, err := s.GetActiveByUser("user-1")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got.ID != cert1.ID {
		t.Errorf("should return active cert, got ID=%s (revoked=%s)", got.ID, cert2.ID)
	}
}

func TestGetActiveByUserNotFound(t *testing.T) {
	s := NewInMemoryCertificateStore()
	_, err := s.GetActiveByUser("nobody")
	if err == nil {
		t.Error("expected error for no active cert")
	}
}

func TestGetActiveByUserAllRevoked(t *testing.T) {
	s := NewInMemoryCertificateStore()
	cert, _ := s.Create(&signing.Certificate{UserID: "user-1", Status: "ACTIVE"})
	now := time.Now()
	_ = s.UpdateStatus(cert.ID, "REVOKED", &now, "key_compromise")

	_, err := s.GetActiveByUser("user-1")
	if err == nil {
		t.Error("expected error when all certs are revoked")
	}
}

func TestListByUserStatusFilter(t *testing.T) {
	s := NewInMemoryCertificateStore()
	s.Create(&signing.Certificate{UserID: "u1", Status: "ACTIVE"})
	s.Create(&signing.Certificate{UserID: "u1", Status: "REVOKED"})
	s.Create(&signing.Certificate{UserID: "u1", Status: "ACTIVE"})

	// No filter
	all, _ := s.ListByUser("u1", "")
	if len(all) != 3 {
		t.Errorf("all: got %d", len(all))
	}

	// Active only
	active, _ := s.ListByUser("u1", "ACTIVE")
	if len(active) != 2 {
		t.Errorf("active: got %d", len(active))
	}

	// Revoked only
	revoked, _ := s.ListByUser("u1", "REVOKED")
	if len(revoked) != 1 {
		t.Errorf("revoked: got %d", len(revoked))
	}
}

func TestListByUserEmpty(t *testing.T) {
	s := NewInMemoryCertificateStore()
	list, _ := s.ListByUser("nobody", "")
	if len(list) != 0 {
		t.Errorf("got %d, want 0", len(list))
	}
}

func TestUpdateStatus(t *testing.T) {
	s := NewInMemoryCertificateStore()
	cert, _ := s.Create(&signing.Certificate{UserID: "u1", Status: "ACTIVE"})

	now := time.Now()
	err := s.UpdateStatus(cert.ID, "REVOKED", &now, "key_compromise")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	got, _ := s.GetByID(cert.ID)
	if got.Status != "REVOKED" {
		t.Errorf("status: got %q", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("RevokedAt should be set")
	}
	if got.RevocationReason != "key_compromise" {
		t.Errorf("reason: got %q", got.RevocationReason)
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	s := NewInMemoryCertificateStore()
	err := s.UpdateStatus("bad-id", "REVOKED", nil, "")
	if err == nil {
		t.Error("expected error")
	}
}

func TestGetByIDNotFound(t *testing.T) {
	s := NewInMemoryCertificateStore()
	_, err := s.GetByID("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestCertificateTimestamps(t *testing.T) {
	s := NewInMemoryCertificateStore()
	before := time.Now().Add(-time.Second)
	cert, _ := s.Create(&signing.Certificate{UserID: "u1", Status: "ACTIVE"})
	after := time.Now().Add(time.Second)

	if cert.CreatedAt.Before(before) || cert.CreatedAt.After(after) {
		t.Errorf("CreatedAt out of range: %v", cert.CreatedAt)
	}
	if cert.UpdatedAt.Before(before) || cert.UpdatedAt.After(after) {
		t.Errorf("UpdatedAt out of range: %v", cert.UpdatedAt)
	}
}
