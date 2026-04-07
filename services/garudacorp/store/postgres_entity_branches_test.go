package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
	"time"
)

// gcFakeDriver is a stdlib-only fake database/sql driver that lets us
// exercise the PostgresEntityStore/RoleStore/UBOStore code paths without
// a real database. QueryContext and ExecContext both return an injectable
// error, which pins all the "return nil, fmt.Errorf(...) %w" wrapping
// branches in the postgres_* files. This satisfies 12factor Factor VI
// (backing services as attached resources) while still letting unit tests
// run without provisioning infrastructure.
type gcFakeDriver struct{ err error }

func (d *gcFakeDriver) Open(_ string) (driver.Conn, error) { return &gcFakeConn{err: d.err}, nil }

type gcFakeConn struct{ err error }

func (c *gcFakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, c.err }
func (c *gcFakeConn) Close() error                          { return nil }
func (c *gcFakeConn) Begin() (driver.Tx, error)             { return &gcFakeTx{}, nil }

func (c *gcFakeConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &gcEmptyRows{}, nil
}

func (c *gcFakeConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.err != nil {
		return nil, c.err
	}
	return gcFakeResult{}, nil
}

type gcFakeTx struct{}

func (t *gcFakeTx) Commit() error   { return nil }
func (t *gcFakeTx) Rollback() error { return nil }

type gcFakeResult struct{}

func (gcFakeResult) LastInsertId() (int64, error) { return 0, nil }
func (gcFakeResult) RowsAffected() (int64, error) { return 0, nil }

type gcEmptyRows struct{}

func (r *gcEmptyRows) Columns() []string { return []string{"id"} }
func (r *gcEmptyRows) Close() error      { return nil }
func (r *gcEmptyRows) Next(_ []driver.Value) error {
	return io.EOF
}

func init() {
	sql.Register("gc-fake-ok", &gcFakeDriver{})
	sql.Register("gc-fake-bad", &gcFakeDriver{err: errors.New("boom")})
}

// TestNullStr_Nil pins the empty-string branch.
func TestNullStr_Nil(t *testing.T) {
	if nullStr("") != nil {
		t.Error("empty should be nil")
	}
	if v := nullStr("abc"); v != "abc" {
		t.Errorf("got %v", v)
	}
}

// TestNullTime_Nil pins both nullTime branches.
func TestNullTime_Nil(t *testing.T) {
	if nullTime(nil) != nil {
		t.Error("nil should be nil")
	}
	now := time.Now()
	v := nullTime(&now)
	got, ok := v.(time.Time)
	if !ok || !got.Equal(now) {
		t.Errorf("got %v", v)
	}
}

// TestPostgresEntityStore_Create_ValidationFails pins the validation
// short-circuit before the DB is touched.
func TestPostgresEntityStore_Create_ValidationFails(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	// Zero-value Entity fails ValidateEntity.
	err := s.Create(context.Background(), &Entity{})
	if err == nil {
		t.Error("expected validation error")
	}
}

// TestPostgresEntityStore_Create_Wraps pins the QueryRowContext error path.
func TestPostgresEntityStore_Create_Wraps(t *testing.T) {
	db, _ := sql.Open("gc-fake-bad", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	// Minimal valid Entity: avoid validation rejection so we reach the DB.
	e := &Entity{
		AHUSKNumber: "AHU-001.AH.01.01.TAHUN 2024",
		Name:        "PT Contoh",
		EntityType:  "PT",
		Status:      "ACTIVE",
		AHUVerifiedAt: time.Now(),
	}
	err := s.Create(context.Background(), e)
	if err == nil {
		t.Error("expected driver error to propagate")
	}
}

// TestPostgresEntityStore_GetByID_NotFound pins the empty-rows branch.
func TestPostgresEntityStore_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("gc-fake-ok", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	_, err := s.GetByID(context.Background(), "missing")
	if err == nil {
		t.Error("expected not-found")
	}
}

// TestPostgresRoleStore_Assign_DBError pins the INSERT error wrap.
func TestPostgresRoleStore_Assign_DBError(t *testing.T) {
	db, _ := sql.Open("gc-fake-bad", "")
	defer db.Close()
	s := NewPostgresRoleStore(db)
	err := s.Assign(context.Background(), &EntityRole{
		EntityID: "e1", UserID: "u1", Role: "ADMIN", GrantedBy: "u0", Status: "ACTIVE",
	})
	if err == nil {
		t.Error("expected driver error")
	}
}
