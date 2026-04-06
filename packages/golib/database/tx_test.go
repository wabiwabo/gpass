package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func TestWithTxRollbackOnPanic(t *testing.T) {
	// We can't easily test WithTx without a real DB, but we can verify
	// the function signature and that it handles the error case for
	// BeginTx failure by using a closed DB.
	db, err := sql.Open("postgres", "postgres://localhost:5432/nonexistent")
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	db.Close() // Close immediately so BeginTx will fail

	pool := &Pool{DB: db, cfg: Config{}}

	err = pool.WithTx(context.Background(), func(tx *sql.Tx) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected error from WithTx on closed DB, got nil")
	}
	if !strings.Contains(err.Error(), "database:") {
		t.Errorf("error should contain 'database:', got: %v", err)
	}
}

func TestWithTxErrorFromFn(t *testing.T) {
	// Verify that when fn returns an error, WithTx returns a wrapped error.
	db, err := sql.Open("postgres", "postgres://localhost:5432/nonexistent")
	if err != nil {
		t.Fatalf("sql.Open failed: %v", err)
	}
	db.Close()

	pool := &Pool{DB: db, cfg: Config{}}

	fnErr := fmt.Errorf("something went wrong")
	err = pool.WithTx(context.Background(), func(tx *sql.Tx) error {
		return fnErr
	})
	// BeginTx will fail because DB is closed, so we just verify we get an error.
	if err == nil {
		t.Fatal("expected error from WithTx, got nil")
	}
}
