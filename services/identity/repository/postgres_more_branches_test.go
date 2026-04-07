package repository

import (
	"context"
	"database/sql"
	"testing"
)

func TestPostgresConsentRepository_ListByUser_Empty(t *testing.T) {
	db, _ := sql.Open("id-fake-ok", "")
	defer db.Close()
	r := NewPostgresConsentRepository(db)
	list, err := r.ListByUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("list = %d", len(list))
	}
}

func TestPostgresConsentRepository_Revoke_DriverError(t *testing.T) {
	db, _ := sql.Open("id-fake-bad", "")
	defer db.Close()
	r := NewPostgresConsentRepository(db)
	if err := r.Revoke(context.Background(), "id"); err == nil {
		t.Error("expected error")
	}
}

func TestPostgresConsentRepository_ExpireStale_Happy(t *testing.T) {
	db, _ := sql.Open("id-fake-ok", "")
	defer db.Close()
	r := NewPostgresConsentRepository(db)
	n, err := r.ExpireStale(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if n != 0 {
		t.Errorf("n = %d", n)
	}
}

func TestConsentColumns(t *testing.T) {
	if consentColumns() == "" {
		t.Error("empty")
	}
}
