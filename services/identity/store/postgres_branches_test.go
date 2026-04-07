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

// isFakeDriver is a stdlib-only fake database/sql driver used to pin
// PostgresDeletionStore error branches without a real DB. UU PDP
// right-to-deletion traceability is tested at the seam.
type isFakeDriver struct{ err error }

func (d *isFakeDriver) Open(_ string) (driver.Conn, error) { return &isFakeConn{err: d.err}, nil }

type isFakeConn struct{ err error }

func (c *isFakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, c.err }
func (c *isFakeConn) Close() error                          { return nil }
func (c *isFakeConn) Begin() (driver.Tx, error)             { return nil, c.err }

func (c *isFakeConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &isEmptyRows{}, nil
}

func (c *isFakeConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.err != nil {
		return nil, c.err
	}
	return isFakeResult{}, nil
}

type isFakeResult struct{}

func (isFakeResult) LastInsertId() (int64, error) { return 0, nil }
func (isFakeResult) RowsAffected() (int64, error) { return 0, nil }

type isEmptyRows struct{}

func (r *isEmptyRows) Columns() []string           { return []string{"id"} }
func (r *isEmptyRows) Close() error                { return nil }
func (r *isEmptyRows) Next(_ []driver.Value) error { return io.EOF }

func init() {
	sql.Register("is-fake-ok", &isFakeDriver{})
	sql.Register("is-fake-bad", &isFakeDriver{err: errors.New("boom")})
}

func isValidDeletion() *DeletionRequest {
	return &DeletionRequest{
		UserID: "u1", Reason: "user_request",
	}
}

func TestPostgresDeletionStore_Create_ValidationFail(t *testing.T) {
	db, _ := sql.Open("is-fake-ok", "")
	defer db.Close()
	s := NewPostgresDeletionStore(db)
	if err := s.Create(&DeletionRequest{}); err == nil {
		t.Error("expected validation error")
	}
}

func TestPostgresDeletionStore_Create_DriverError(t *testing.T) {
	db, _ := sql.Open("is-fake-bad", "")
	defer db.Close()
	s := NewPostgresDeletionStore(db)
	if err := s.Create(isValidDeletion()); err == nil {
		t.Error("expected driver error")
	}
}

func TestPostgresDeletionStore_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("is-fake-ok", "")
	defer db.Close()
	s := NewPostgresDeletionStore(db)
	if _, err := s.GetByID("missing"); err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresDeletionStore_ListByUser_DriverError(t *testing.T) {
	db, _ := sql.Open("is-fake-bad", "")
	defer db.Close()
	s := NewPostgresDeletionStore(db)
	if _, err := s.ListByUser("u1"); err == nil {
		t.Error("expected error")
	}
}

func TestPostgresDeletionStore_ListByUser_Empty(t *testing.T) {
	db, _ := sql.Open("is-fake-ok", "")
	defer db.Close()
	s := NewPostgresDeletionStore(db)
	list, err := s.ListByUser("u1")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty, got %d", len(list))
	}
}

func TestPostgresDeletionStore_UpdateStatus_ValidationFail(t *testing.T) {
	db, _ := sql.Open("is-fake-ok", "")
	defer db.Close()
	s := NewPostgresDeletionStore(db)
	if err := s.UpdateStatus("id", "BOGUS", nil, nil); err == nil {
		t.Error("expected validation error")
	}
}

func TestPostgresDeletionStore_UpdateStatus_NotFound(t *testing.T) {
	db, _ := sql.Open("is-fake-ok", "")
	defer db.Close()
	s := NewPostgresDeletionStore(db)
	now := time.Now()
	err := s.UpdateStatus("id", "COMPLETED", &now, []string{"personal_info"})
	if err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresDeletionStore_UpdateStatus_DriverError(t *testing.T) {
	db, _ := sql.Open("is-fake-bad", "")
	defer db.Close()
	s := NewPostgresDeletionStore(db)
	now := time.Now()
	err := s.UpdateStatus("id", "COMPLETED", &now, []string{"personal_info"})
	if err == nil || err == ErrNotFound {
		t.Errorf("err = %v", err)
	}
}
