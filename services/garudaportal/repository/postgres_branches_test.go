package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
)

// gpFakeDriver is a stdlib-only fake sql driver for pinning error
// branches in PostgresAppRepository/PostgresAPIKeyRepository without
// a live database. 12factor Factor VI — backing services as attached
// resources — means our code must tolerate DB failures at the seam
// with well-wrapped errors. This test verifies those wrap points.
type gpFakeDriver struct{ err error }

func (d *gpFakeDriver) Open(_ string) (driver.Conn, error) { return &gpFakeConn{err: d.err}, nil }

type gpFakeConn struct{ err error }

func (c *gpFakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, c.err }
func (c *gpFakeConn) Close() error                          { return nil }
func (c *gpFakeConn) Begin() (driver.Tx, error)             { return nil, c.err }

func (c *gpFakeConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &gpEmptyRows{}, nil
}

func (c *gpFakeConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.err != nil {
		return nil, c.err
	}
	return gpFakeResult{}, nil
}

type gpFakeResult struct{}

func (gpFakeResult) LastInsertId() (int64, error) { return 0, nil }
func (gpFakeResult) RowsAffected() (int64, error) { return 0, nil }

type gpEmptyRows struct{}

func (r *gpEmptyRows) Columns() []string           { return []string{"id"} }
func (r *gpEmptyRows) Close() error                { return nil }
func (r *gpEmptyRows) Next(_ []driver.Value) error { return io.EOF }

func init() {
	sql.Register("gp-fake-ok", &gpFakeDriver{})
	sql.Register("gp-fake-bad", &gpFakeDriver{err: errors.New("boom")})
	sql.Register("gp-fake-dup", &gpFakeDriver{err: errors.New("pq: duplicate key value violates unique constraint (23505)")})
}

// TestPostgresAppRepository_Create_DriverError pins the create-error wrap.
func TestPostgresAppRepository_Create_DriverError(t *testing.T) {
	db, _ := sql.Open("gp-fake-bad", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	if err := r.Create(context.Background(), &App{}); err == nil {
		t.Error("expected error")
	}
}

// TestPostgresAppRepository_Create_UniqueViolation pins the ErrDuplicateApp
// branch when the driver returns a 23505 SQLSTATE.
func TestPostgresAppRepository_Create_UniqueViolation(t *testing.T) {
	db, _ := sql.Open("gp-fake-dup", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	err := r.Create(context.Background(), &App{})
	if err != ErrDuplicateApp {
		t.Errorf("err = %v, want ErrDuplicateApp", err)
	}
}

// TestPostgresAppRepository_GetByID_NotFound pins the ErrNotFound branch.
func TestPostgresAppRepository_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	_, err := r.GetByID(context.Background(), "missing")
	if err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

// TestPostgresAppRepository_GetByID_DriverError pins the wrap path.
func TestPostgresAppRepository_GetByID_DriverError(t *testing.T) {
	db, _ := sql.Open("gp-fake-bad", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	_, err := r.GetByID(context.Background(), "id")
	if err == nil || err == ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

// TestPostgresAppRepository_ListByOwner_DriverError pins the query wrap.
func TestPostgresAppRepository_ListByOwner_DriverError(t *testing.T) {
	db, _ := sql.Open("gp-fake-bad", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	_, err := r.ListByOwner(context.Background(), "u1")
	if err == nil {
		t.Error("expected error")
	}
}

// TestPostgresAppRepository_ListByOwner_EmptyRows pins the happy-path
// iteration with zero rows.
func TestPostgresAppRepository_ListByOwner_EmptyRows(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	apps, err := r.ListByOwner(context.Background(), "u1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("apps = %d", len(apps))
	}
}

// TestPostgresAppRepository_Update_Empty pins the no-op early return.
func TestPostgresAppRepository_Update_Empty(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	if err := r.Update(context.Background(), "id", nil); err != nil {
		t.Errorf("empty update should be no-op, got %v", err)
	}
}

// TestPostgresAppRepository_Update_BadCallbackURLs pins the type assertion
// failure branch in the callback_urls case.
func TestPostgresAppRepository_Update_BadCallbackURLs(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	err := r.Update(context.Background(), "id", map[string]interface{}{
		"callback_urls": 42, // not []string
	})
	if err == nil {
		t.Error("expected type error")
	}
}

// TestPostgresAppRepository_Update_NotFound pins the 0-rows-affected branch.
func TestPostgresAppRepository_Update_NotFound(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	err := r.Update(context.Background(), "id", map[string]interface{}{
		"name": "new",
	})
	if err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

// TestPostgresAppRepository_UpdateStatus_NotFound pins the 0-rows branch.
func TestPostgresAppRepository_UpdateStatus_NotFound(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	err := r.UpdateStatus(context.Background(), "id", "DISABLED")
	if err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

// TestPostgresAppRepository_UpdateStatus_DriverError pins the wrap path.
func TestPostgresAppRepository_UpdateStatus_DriverError(t *testing.T) {
	db, _ := sql.Open("gp-fake-bad", "")
	defer db.Close()
	r := NewPostgresAppRepository(db)
	err := r.UpdateStatus(context.Background(), "id", "ACTIVE")
	if err == nil || err == ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

// TestIsUniqueViolation pins the SQLSTATE-23505 detection helper.
func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(errors.New("boom (23505)")) {
		t.Error("should detect 23505")
	}
	if isUniqueViolation(errors.New("boom")) {
		t.Error("should not detect")
	}
	if isUniqueViolation(nil) {
		t.Error("nil should not detect")
	}
}

// TestContains pins the lightweight substring helper.
func TestContains(t *testing.T) {
	if !contains("hello world", "world") {
		t.Error("should find")
	}
	if contains("hi", "hello") {
		t.Error("should not find")
	}
	if !contains("abc", "") {
		t.Error("empty substr always matches")
	}
}

// TestPostgresAPIKeyRepository_Revoke_DriverError pins the key revoke wrap.
func TestPostgresAPIKeyRepository_Revoke_DriverError(t *testing.T) {
	db, _ := sql.Open("gp-fake-bad", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	if err := r.Revoke(context.Background(), "id"); err == nil {
		t.Error("expected error")
	}
}

// TestPostgresAPIKeyRepository_Revoke_NotFound pins the 0-rows branch.
func TestPostgresAPIKeyRepository_Revoke_NotFound(t *testing.T) {
	db, _ := sql.Open("gp-fake-ok", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	err := r.Revoke(context.Background(), "id")
	if err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

// TestPostgresAPIKeyRepository_UpdateLastUsed_DriverError pins the wrap.
func TestPostgresAPIKeyRepository_UpdateLastUsed_DriverError(t *testing.T) {
	db, _ := sql.Open("gp-fake-bad", "")
	defer db.Close()
	r := NewPostgresAPIKeyRepository(db)
	if err := r.UpdateLastUsed(context.Background(), "id"); err == nil {
		t.Error("expected error")
	}
}
