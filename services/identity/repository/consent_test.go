package repository

import (
	"context"
	"errors"
	"testing"
	"time"
)

func newTestConsent(userID string) *Consent {
	now := time.Now().UTC()
	return &Consent{
		UserID:          userID,
		ClientID:        "app-001",
		ClientName:      "Test App",
		Purpose:         "identity_verification",
		Fields:          []byte(`{"name":true,"dob":true}`),
		DurationSeconds: 86400,
		GrantedAt:       now,
		ExpiresAt:       now.Add(24 * time.Hour),
		Status:          "ACTIVE",
	}
}

func TestInMemoryConsent_GrantAndGetByID(t *testing.T) {
	repo := NewInMemoryConsentRepository()
	ctx := context.Background()
	consent := newTestConsent("user-001")

	if err := repo.Grant(ctx, consent); err != nil {
		t.Fatalf("Grant: unexpected error: %v", err)
	}

	if consent.ID == "" {
		t.Fatal("Grant: expected ID to be set")
	}
	if consent.CreatedAt.IsZero() {
		t.Fatal("Grant: expected CreatedAt to be set")
	}

	got, err := repo.GetByID(ctx, consent.ID)
	if err != nil {
		t.Fatalf("GetByID: unexpected error: %v", err)
	}

	if got.ID != consent.ID {
		t.Errorf("GetByID: ID = %q, want %q", got.ID, consent.ID)
	}
	if got.UserID != "user-001" {
		t.Errorf("GetByID: UserID = %q, want user-001", got.UserID)
	}
	if got.ClientID != "app-001" {
		t.Errorf("GetByID: ClientID = %q, want app-001", got.ClientID)
	}
	if got.Status != "ACTIVE" {
		t.Errorf("GetByID: Status = %q, want ACTIVE", got.Status)
	}
	if string(got.Fields) != `{"name":true,"dob":true}` {
		t.Errorf("GetByID: Fields = %q, want JSON", got.Fields)
	}
}

func TestInMemoryConsent_GetByIDNotFound(t *testing.T) {
	repo := NewInMemoryConsentRepository()
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetByID not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryConsent_ListByUser(t *testing.T) {
	repo := NewInMemoryConsentRepository()
	ctx := context.Background()

	c1 := newTestConsent("user-001")
	c2 := newTestConsent("user-001")
	c2.ClientID = "app-002"
	c3 := newTestConsent("user-002")

	for _, c := range []*Consent{c1, c2, c3} {
		if err := repo.Grant(ctx, c); err != nil {
			t.Fatalf("Grant: unexpected error: %v", err)
		}
	}

	list, err := repo.ListByUser(ctx, "user-001")
	if err != nil {
		t.Fatalf("ListByUser: unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListByUser: got %d consents, want 2", len(list))
	}

	list2, err := repo.ListByUser(ctx, "user-002")
	if err != nil {
		t.Fatalf("ListByUser: unexpected error: %v", err)
	}
	if len(list2) != 1 {
		t.Fatalf("ListByUser: got %d consents, want 1", len(list2))
	}

	// Non-existing user returns empty list, not error.
	list3, err := repo.ListByUser(ctx, "user-999")
	if err != nil {
		t.Fatalf("ListByUser: unexpected error: %v", err)
	}
	if len(list3) != 0 {
		t.Fatalf("ListByUser: got %d consents, want 0", len(list3))
	}
}

func TestInMemoryConsent_Revoke(t *testing.T) {
	repo := NewInMemoryConsentRepository()
	ctx := context.Background()
	consent := newTestConsent("user-001")

	if err := repo.Grant(ctx, consent); err != nil {
		t.Fatalf("Grant: unexpected error: %v", err)
	}

	if err := repo.Revoke(ctx, consent.ID); err != nil {
		t.Fatalf("Revoke: unexpected error: %v", err)
	}

	got, err := repo.GetByID(ctx, consent.ID)
	if err != nil {
		t.Fatalf("GetByID: unexpected error: %v", err)
	}
	if got.Status != "REVOKED" {
		t.Errorf("Status = %q, want REVOKED", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("RevokedAt should be set after revocation")
	}

	// Revoking an already-revoked consent returns ErrNotFound.
	err = repo.Revoke(ctx, consent.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Revoke already revoked: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryConsent_RevokeNotFound(t *testing.T) {
	repo := NewInMemoryConsentRepository()
	ctx := context.Background()

	err := repo.Revoke(ctx, "nonexistent-id")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Revoke not found: got %v, want ErrNotFound", err)
	}
}

func TestInMemoryConsent_ExpireStale(t *testing.T) {
	repo := NewInMemoryConsentRepository()
	ctx := context.Background()

	// Create an already-expired consent.
	expired := newTestConsent("user-001")
	expired.ExpiresAt = time.Now().UTC().Add(-1 * time.Hour)
	if err := repo.Grant(ctx, expired); err != nil {
		t.Fatalf("Grant expired: unexpected error: %v", err)
	}

	// Create a still-active consent.
	active := newTestConsent("user-001")
	active.ExpiresAt = time.Now().UTC().Add(24 * time.Hour)
	if err := repo.Grant(ctx, active); err != nil {
		t.Fatalf("Grant active: unexpected error: %v", err)
	}

	count, err := repo.ExpireStale(ctx)
	if err != nil {
		t.Fatalf("ExpireStale: unexpected error: %v", err)
	}
	if count != 1 {
		t.Errorf("ExpireStale: expired %d, want 1", count)
	}

	got, _ := repo.GetByID(ctx, expired.ID)
	if got.Status != "EXPIRED" {
		t.Errorf("Expired consent status = %q, want EXPIRED", got.Status)
	}

	still, _ := repo.GetByID(ctx, active.ID)
	if still.Status != "ACTIVE" {
		t.Errorf("Active consent status = %q, want ACTIVE", still.Status)
	}

	// Running again should expire 0.
	count2, err := repo.ExpireStale(ctx)
	if err != nil {
		t.Fatalf("ExpireStale second run: unexpected error: %v", err)
	}
	if count2 != 0 {
		t.Errorf("ExpireStale second run: expired %d, want 0", count2)
	}
}
