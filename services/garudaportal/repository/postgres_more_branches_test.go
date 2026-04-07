package repository

import (
	"context"
	"database/sql"
	"testing"
)

func TestPostgresAPIKeyRepository_Create_DriverError(t *testing.T) {
	db, _ := sql.Open("gp-fake-bad", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	if err := r.Create(context.Background(), &APIKeyRecord{}); err == nil {
		t.Error("expected driver error")
	}
}

func TestPostgresAPIKeyRepository_Create_UniqueViolation(t *testing.T) {
	db, _ := sql.Open("gp-fake-dup", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	if err := r.Create(context.Background(), &APIKeyRecord{}); err == nil {
		t.Error("expected duplicate error")
	}
}

func TestPostgresAPIKeyRepository_GetByHash_NotFound(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	if _, err := r.GetByHash(context.Background(), "h"); err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresAPIKeyRepository_GetByHash_DriverError(t *testing.T) {
	db, _ := sql.Open("gp-fake-bad", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	if _, err := r.GetByHash(context.Background(), "h"); err == nil || err == ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresAPIKeyRepository_ListByApp_DriverError(t *testing.T) {
	db, _ := sql.Open("gp-fake-bad", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	if _, err := r.ListByApp(context.Background(), "app"); err == nil {
		t.Error("expected error")
	}
}

func TestPostgresAPIKeyRepository_ListByApp_Empty(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	keys, err := r.ListByApp(context.Background(), "app")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("keys = %d", len(keys))
	}
}

func TestPostgresAPIKeyRepository_UpdateLastUsed_NotFound(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	if err := r.UpdateLastUsed(context.Background(), "id"); err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestAPIKeyColumns(t *testing.T) {
	if apiKeyColumns() == "" {
		t.Error("empty")
	}
}

// Also cover app.go uncovered branches via existing helpers:

func TestPostgresAppRepository_Update_AllFieldKinds(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	// Cover the callback_urls (valid []string) + default (other key) paths.
	err := r.Update(context.Background(), "id", map[string]interface{}{
		"callback_urls": []string{"https://a.test"},
		"name":          "x",
		"description":   "y",
	})
	if err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestSearchString_EdgeCases(t *testing.T) {
	if !searchString("hello", "ello") {
		t.Error("tail match")
	}
	if !searchString("hello", "hello") {
		t.Error("full match")
	}
	if searchString("hi", "bye") {
		t.Error("no match")
	}
}
