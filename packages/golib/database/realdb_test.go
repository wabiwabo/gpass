package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestPool_Health_OKAgainstFakeDriver covers the Health success path,
// previously 0% because all existing tests fed Health a closed *sql.DB
// and only verified the failure branch.
func TestPool_Health_OKAgainstFakeDriver(t *testing.T) {
	db, err := openFakePool()
	if err != nil {
		t.Fatalf("openFakePool: %v", err)
	}
	defer db.Close()

	p := &Pool{DB: db, cfg: Config{}.WithDefaults()}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.Health(ctx); err != nil {
		t.Errorf("Health on fake pool returned %v, want nil", err)
	}
}

// TestPool_Health_ErrorAfterClose pins the wrap-error path: closing the
// pool then calling Health must return the "health check failed" wrapped
// error. This is the inverse of the success test and exercises the only
// branch in Health that returns an error.
func TestPool_Health_ErrorAfterClose(t *testing.T) {
	db, err := openFakePool()
	if err != nil {
		t.Fatalf("openFakePool: %v", err)
	}
	p := &Pool{DB: db, cfg: Config{}.WithDefaults()}
	_ = db.Close()

	err = p.Health(context.Background())
	if err == nil {
		t.Fatal("expected error from Health after Close")
	}
	if !strings.Contains(err.Error(), "health check failed") {
		t.Errorf("err = %q, want substring 'health check failed'", err.Error())
	}
}

// TestWithTx_CommitsOnNilError covers the happy path: fn returns nil →
// transaction is committed → WithTx returns nil. Previously 20%.
func TestWithTx_CommitsOnNilError(t *testing.T) {
	db, err := openFakePool()
	if err != nil {
		t.Fatalf("openFakePool: %v", err)
	}
	defer db.Close()
	p := &Pool{DB: db, cfg: Config{}.WithDefaults()}

	called := false
	err = p.WithTx(context.Background(), func(tx *sql.Tx) error {
		called = true
		if tx == nil {
			t.Error("tx should be non-nil")
		}
		return nil
	})
	if err != nil {
		t.Errorf("WithTx returned %v, want nil", err)
	}
	if !called {
		t.Error("fn was never invoked")
	}
}

// TestWithTx_RollsBackOnFnError covers the "fn returned error" branch
// which wraps the error with "transaction failed".
func TestWithTx_RollsBackOnFnError(t *testing.T) {
	db, err := openFakePool()
	if err != nil {
		t.Fatalf("openFakePool: %v", err)
	}
	defer db.Close()
	p := &Pool{DB: db, cfg: Config{}.WithDefaults()}

	sentinel := errors.New("boom")
	err = p.WithTx(context.Background(), func(tx *sql.Tx) error {
		return sentinel
	})
	if err == nil {
		t.Fatal("expected wrapped error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("errors.Is(err, sentinel) = false, want true; err=%v", err)
	}
	if !strings.Contains(err.Error(), "transaction failed") {
		t.Errorf("err = %q, want substring 'transaction failed'", err.Error())
	}
}

// TestWithTx_RecoversFromPanic covers the deferred recover() branch:
// a panicking fn must NOT crash the caller; the recovered value is
// surfaced as a wrapped error and the transaction is rolled back.
func TestWithTx_RecoversFromPanic(t *testing.T) {
	db, err := openFakePool()
	if err != nil {
		t.Fatalf("openFakePool: %v", err)
	}
	defer db.Close()
	p := &Pool{DB: db, cfg: Config{}.WithDefaults()}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("WithTx leaked the panic: %v", r)
		}
	}()

	err = p.WithTx(context.Background(), func(tx *sql.Tx) error {
		panic("kaboom")
	})
	if err == nil {
		t.Fatal("expected error from panic recovery")
	}
	if !strings.Contains(err.Error(), "panic in transaction") {
		t.Errorf("err = %q, want substring 'panic in transaction'", err.Error())
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Errorf("err = %q, want substring 'kaboom'", err.Error())
	}
}
