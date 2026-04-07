package store

import (
	"context"
	"testing"
	"time"
)

func TestCreateAndGet(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	c := &Consent{
		UserID:          "user-1",
		ClientID:        "client-1", ClientName:      "Test App",
		Purpose:         "KYC verification",
		Fields:          map[string]bool{"name": true, "dob": true},
		DurationSeconds: 3600,
	}

	if err := s.Create(ctx, c); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if c.ID == "" {
		t.Fatal("expected ID to be set after Create")
	}
	if c.Status != "ACTIVE" {
		t.Errorf("Status = %q, want ACTIVE", c.Status)
	}
	if c.GrantedAt.IsZero() {
		t.Error("GrantedAt should not be zero")
	}
	if c.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should not be zero")
	}

	got, err := s.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.UserID != "user-1" {
		t.Errorf("UserID = %q, want user-1", got.UserID)
	}
	if !got.Fields["name"] || !got.Fields["dob"] {
		t.Errorf("Fields mismatch: %v", got.Fields)
	}

	// Verify copy safety: mutating returned consent should not affect store
	got.Fields["name"] = false
	got2, _ := s.GetByID(ctx, c.ID)
	if !got2.Fields["name"] {
		t.Error("store returned pointer to internal data, not a copy")
	}
}

func TestRevoke(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	c := &Consent{
		UserID:          "user-1",
		ClientID:        "client-1", ClientName: "Test Client", Purpose: "test",
		Fields:          map[string]bool{"name": true},
		DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)

	if err := s.Revoke(ctx, c.ID); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	got, _ := s.GetByID(ctx, c.ID)
	if got.Status != "REVOKED" {
		t.Errorf("Status = %q, want REVOKED", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("RevokedAt should be set after revocation")
	}
}

func TestRevokeAlreadyRevoked(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	c := &Consent{
		UserID:          "user-1",
		ClientID:        "client-1", ClientName: "Test Client", Purpose: "test",
		Fields:          map[string]bool{"name": true},
		DurationSeconds: 3600,
	}
	_ = s.Create(ctx, c)
	_ = s.Revoke(ctx, c.ID)

	err := s.Revoke(ctx, c.ID)
	if err != ErrConsentRevoked {
		t.Errorf("err = %v, want ErrConsentRevoked", err)
	}
}

func TestListByUser(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_ = s.Create(ctx, &Consent{
			UserID:          "user-1",
			ClientID:        "client-1", ClientName: "Test Client", Purpose: "test",
			Fields:          map[string]bool{"name": true},
			DurationSeconds: 3600,
		})
	}
	_ = s.Create(ctx, &Consent{
		UserID:          "user-2",
		ClientID:        "client-1", ClientName: "Test Client", Purpose: "test",
		Fields:          map[string]bool{"name": true},
		DurationSeconds: 3600,
	})

	list, err := s.ListByUser(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("got %d consents, want 3", len(list))
	}
}

func TestGetByID_NotFound(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	_, err := s.GetByID(ctx, "nonexistent")
	if err != ErrConsentNotFound {
		t.Errorf("err = %v, want ErrConsentNotFound", err)
	}
}

func TestExpireStale(t *testing.T) {
	s := NewInMemoryConsentStore()
	ctx := context.Background()

	// Create a consent with a very short duration
	c := &Consent{
		UserID:          "user-1",
		ClientID:        "client-1", ClientName: "Test Client", Purpose: "test",
		Fields:          map[string]bool{"name": true},
		DurationSeconds: 1, // 1 second
	}
	_ = s.Create(ctx, c)

	// Manually set ExpiresAt to the past to simulate expiry
	s.mu.Lock()
	s.consents[c.ID].ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)
	s.mu.Unlock()

	count, err := s.ExpireStale(ctx)
	if err != nil {
		t.Fatalf("ExpireStale: %v", err)
	}
	if count != 1 {
		t.Errorf("expired count = %d, want 1", count)
	}

	got, _ := s.GetByID(ctx, c.ID)
	if got.Status != "EXPIRED" {
		t.Errorf("Status = %q, want EXPIRED", got.Status)
	}

	// Running again should return 0
	count2, _ := s.ExpireStale(ctx)
	if count2 != 0 {
		t.Errorf("second expire count = %d, want 0", count2)
	}
}
