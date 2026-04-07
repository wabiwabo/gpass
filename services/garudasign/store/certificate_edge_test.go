package store

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// TestCertificateConcurrentCreate pins the enforced "one ACTIVE
// certificate per user" invariant under concurrent creation. Exactly
// one Create must succeed; the remaining n-1 must return
// ErrActiveCertificateExists. This guarantees the store itself is the
// source of truth for the invariant, eliminating the TOCTOU race
// between a handler's pre-check and its subsequent Create call.
func TestCertificateConcurrentCreate(t *testing.T) {
	s := NewInMemoryCertificateStore()
	var wg sync.WaitGroup
	n := 100

	var successes, conflicts int
	var mu sync.Mutex
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
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successes++
			} else if err == ErrActiveCertificateExists {
				conflicts++
			} else {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Errorf("successes = %d, want 1", successes)
	}
	if conflicts != n-1 {
		t.Errorf("conflicts = %d, want %d", conflicts, n-1)
	}
	list, _ := s.ListByUser("user-concurrent", "")
	if len(list) != 1 {
		t.Errorf("active certs: got %d, want 1", len(list))
	}
}

func TestCertificateConcurrentReadWrite(t *testing.T) {
	s := NewInMemoryCertificateStore()

	// Create some certs. Only the first ACTIVE cert per user is
	// permitted; use distinct UserIDs so all 20 Creates succeed.
	ids := make([]string, 20)
	for i := 0; i < 20; i++ {
		cert := &signing.Certificate{UserID: fmt.Sprintf("user-rw-%d", i), Status: "ACTIVE"}
		created, err := s.Create(cert)
		if err != nil {
			t.Fatalf("seed create: %v", err)
		}
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
		cert := &signing.Certificate{UserID: fmt.Sprintf("user-id-%d", i), Status: "ACTIVE"}
		created, err := s.Create(cert)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
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
	// Build a multi-status history for u1 while respecting the
	// "one ACTIVE per user" invariant: seed one cert, revoke it,
	// then create the next ACTIVE.
	first, _ := s.Create(&signing.Certificate{UserID: "u1", Status: "ACTIVE"})
	now := time.Now()
	_ = s.UpdateStatus(first.ID, "REVOKED", &now, "rotated")
	s.Create(&signing.Certificate{UserID: "u1", Status: "ACTIVE"})

	all, _ := s.ListByUser("u1", "")
	if len(all) != 2 {
		t.Errorf("all: got %d", len(all))
	}
	active, _ := s.ListByUser("u1", "ACTIVE")
	if len(active) != 1 {
		t.Errorf("active: got %d", len(active))
	}
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
