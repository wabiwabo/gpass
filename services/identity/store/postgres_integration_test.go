//go:build integration

package store

import (
	"database/sql"
	"os"
	"testing"
	"time"

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
		_, _ = db.Exec("DELETE FROM deletion_requests WHERE reason = 'user_request' AND user_id = '00000000-0000-0000-0000-000000000099'")
		db.Close()
	})
	return db
}

func TestPostgresDeletionStore_Lifecycle(t *testing.T) {
	db := openTestDB(t)
	s := NewPostgresDeletionStore(db)

	req := &DeletionRequest{
		UserID: "00000000-0000-0000-0000-000000000099",
		Reason: "user_request",
	}
	if err := s.Create(req); err != nil {
		t.Fatalf("create: %v", err)
	}
	if req.ID == "" || req.Status != "PENDING" {
		t.Fatalf("unexpected: %+v", req)
	}

	got, err := s.GetByID(req.ID)
	if err != nil {
		t.Fatalf("getbyid: %v", err)
	}
	if got.Status != "PENDING" {
		t.Errorf("status = %s, want PENDING", got.Status)
	}

	now := time.Now().UTC()
	if err := s.UpdateStatus(req.ID, "COMPLETED", &now, []string{"personal_info", "biometric"}); err != nil {
		t.Fatalf("update: %v", err)
	}

	got2, _ := s.GetByID(req.ID)
	if got2.Status != "COMPLETED" || got2.CompletedAt == nil || len(got2.DeletedData) != 2 {
		t.Errorf("after update: %+v", got2)
	}

	// Invalid reason
	bad := &DeletionRequest{UserID: "x", Reason: "bogus"}
	if err := s.Create(bad); err != ErrInvalidReason {
		t.Errorf("expected ErrInvalidReason, got %v", err)
	}
}
