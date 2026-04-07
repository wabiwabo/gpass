//go:build integration

package store

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"
)

// openTestDB opens a connection to TEST_DATABASE_URL and skips if not set.
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
		_, _ = db.Exec("DELETE FROM audit_events WHERE service_name = 'integration_test'")
		db.Close()
	})
	return db
}

func TestPostgresAuditStore_AppendQueryGet(t *testing.T) {
	db := openTestDB(t)
	s := NewPostgresAuditStore(db)

	ev := &AuditEvent{
		EventType:   "TEST_EVENT",
		ActorID:     "user-int-1",
		Action:      "TEST",
		ServiceName: "integration_test",
		Metadata:    map[string]string{"k": "v"},
	}
	if err := s.Append(ev); err != nil {
		t.Fatalf("append: %v", err)
	}
	if ev.ID == "" {
		t.Fatal("expected ID populated")
	}

	got, err := s.GetByID(ev.ID)
	if err != nil {
		t.Fatalf("getbyid: %v", err)
	}
	if got.EventType != "TEST_EVENT" || got.Metadata["k"] != "v" {
		t.Errorf("roundtrip mismatch: %+v", got)
	}

	events, err := s.Query(AuditFilter{ActorID: "user-int-1", Limit: 10})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least 1 event")
	}

	count, err := s.Count(AuditFilter{ActorID: "user-int-1"})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count < 1 {
		t.Errorf("count = %d, want >= 1", count)
	}
}

func TestPostgresAuditStore_Validation(t *testing.T) {
	db := openTestDB(t)
	s := NewPostgresAuditStore(db)

	if err := s.Append(&AuditEvent{}); err == nil {
		t.Error("expected validation error for empty event")
	}
}
