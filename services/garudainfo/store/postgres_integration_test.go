//go:build integration

package store

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM consents WHERE client_id = 'integration_test'")
		db.Close()
	})
	return db
}

func TestPostgresConsentStore_Lifecycle(t *testing.T) {
	db := openTestDB(t)
	s := NewPostgresConsentStore(db)
	ctx := context.Background()

	c := &Consent{
		UserID:          "00000000-0000-0000-0000-000000000001",
		ClientID:        "integration_test",
		ClientName:      "Integration Test",
		Purpose:         "testing",
		Fields:          map[string]bool{"name": true, "dob": false},
		DurationSeconds: 3600,
	}
	if err := s.Create(ctx, c); err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.ID == "" {
		t.Fatal("expected ID populated")
	}

	got, err := s.GetByID(ctx, c.ID)
	if err != nil {
		t.Fatalf("getbyid: %v", err)
	}
	if got.Status != "ACTIVE" || !got.Fields["name"] || got.Fields["dob"] {
		t.Errorf("roundtrip mismatch: %+v", got)
	}

	list, err := s.ListActiveByUserAndClient(ctx, c.UserID, c.ClientID)
	if err != nil || len(list) == 0 {
		t.Fatalf("list active: err=%v len=%d", err, len(list))
	}

	if err := s.Revoke(ctx, c.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if err := s.Revoke(ctx, c.ID); err != ErrConsentRevoked {
		t.Errorf("double-revoke: got %v, want ErrConsentRevoked", err)
	}

	got2, _ := s.GetByID(ctx, c.ID)
	if got2.Status != "REVOKED" || got2.RevokedAt == nil {
		t.Errorf("expected REVOKED with timestamp, got %+v", got2)
	}
}
