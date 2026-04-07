package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
)

// idFakeDriver is a stdlib-only fake database/sql driver used to pin
// error branches in PostgresUserRepository and PostgresConsentRepository
// without provisioning a real PostgreSQL instance. Enterprise 12factor
// Factor VI: backing services are attached resources, and our code
// must tolerate their failure modes at the seam — this fake driver
// exercises exactly those wraps.
type idFakeDriver struct{ err error }

func (d *idFakeDriver) Open(_ string) (driver.Conn, error) { return &idFakeConn{err: d.err}, nil }

type idFakeConn struct{ err error }

func (c *idFakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, c.err }
func (c *idFakeConn) Close() error                          { return nil }
func (c *idFakeConn) Begin() (driver.Tx, error)             { return nil, c.err }

func (c *idFakeConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &idEmptyRows{}, nil
}

func (c *idFakeConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.err != nil {
		return nil, c.err
	}
	return idFakeResult{}, nil
}

type idFakeResult struct{}

func (idFakeResult) LastInsertId() (int64, error) { return 0, nil }
func (idFakeResult) RowsAffected() (int64, error) { return 0, nil }

type idEmptyRows struct{}

func (r *idEmptyRows) Columns() []string           { return []string{"id"} }
func (r *idEmptyRows) Close() error                { return nil }
func (r *idEmptyRows) Next(_ []driver.Value) error { return io.EOF }

func init() {
	sql.Register("id-fake-ok", &idFakeDriver{})
	sql.Register("id-fake-bad", &idFakeDriver{err: errors.New("boom")})
	sql.Register("id-fake-dup", &idFakeDriver{err: errors.New("duplicate key (23505)")})
}

func TestPostgresUserRepository_Create_DriverError(t *testing.T) {
	db, _ := sql.Open("id-fake-bad", "")
	defer db.Close()
	r := NewPostgresUserRepository(db)
	if err := r.Create(context.Background(), &User{}); err == nil {
		t.Error("expected error")
	}
}

func TestPostgresUserRepository_Create_DuplicateNIK(t *testing.T) {
	db, _ := sql.Open("id-fake-dup", "")
	defer db.Close()
	r := NewPostgresUserRepository(db)
	if err := r.Create(context.Background(), &User{}); err != ErrDuplicateNIKToken {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresUserRepository_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("id-fake-ok", "")
	defer db.Close()
	r := NewPostgresUserRepository(db)
	if _, err := r.GetByID(context.Background(), "missing"); err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresUserRepository_GetByID_DriverError(t *testing.T) {
	db, _ := sql.Open("id-fake-bad", "")
	defer db.Close()
	r := NewPostgresUserRepository(db)
	if _, err := r.GetByID(context.Background(), "id"); err == nil || err == ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresUserRepository_GetByNIKToken_NotFound(t *testing.T) {
	db, _ := sql.Open("id-fake-ok", "")
	defer db.Close()
	r := NewPostgresUserRepository(db)
	if _, err := r.GetByNIKToken(context.Background(), "nik"); err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresUserRepository_UpdateVerificationStatus_NotFound(t *testing.T) {
	db, _ := sql.Open("id-fake-ok", "")
	defer db.Close()
	r := NewPostgresUserRepository(db)
	if err := r.UpdateVerificationStatus(context.Background(), "id", "VERIFIED"); err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresUserRepository_UpdateVerificationStatus_DriverError(t *testing.T) {
	db, _ := sql.Open("id-fake-bad", "")
	defer db.Close()
	r := NewPostgresUserRepository(db)
	if err := r.UpdateVerificationStatus(context.Background(), "id", "VERIFIED"); err == nil || err == ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresUserRepository_Exists_DriverError(t *testing.T) {
	db, _ := sql.Open("id-fake-bad", "")
	defer db.Close()
	r := NewPostgresUserRepository(db)
	if _, err := r.Exists(context.Background(), "nik"); err == nil {
		t.Error("expected error")
	}
}

func TestIsUniqueViolation_Identity(t *testing.T) {
	if !isUniqueViolation(errors.New("boom (23505)")) {
		t.Error("should detect 23505")
	}
	if isUniqueViolation(errors.New("other")) {
		t.Error("should not detect")
	}
	if isUniqueViolation(nil) {
		t.Error("nil should not detect")
	}
}

func TestContains_Identity(t *testing.T) {
	if !contains("error (23505)", "23505") {
		t.Error("should find")
	}
	if contains("abc", "abcdef") {
		t.Error("should not find")
	}
}

// TestPostgresConsentRepository_Grant_DriverError pins the Grant wrap.
func TestPostgresConsentRepository_Grant_DriverError(t *testing.T) {
	db, _ := sql.Open("id-fake-bad", "")
	defer db.Close()
	r := NewPostgresConsentRepository(db)
	if err := r.Grant(context.Background(), &Consent{}); err == nil {
		t.Error("expected error")
	}
}

func TestPostgresConsentRepository_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("id-fake-ok", "")
	defer db.Close()
	r := NewPostgresConsentRepository(db)
	if _, err := r.GetByID(context.Background(), "missing"); err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresConsentRepository_ListByUser_DriverError(t *testing.T) {
	db, _ := sql.Open("id-fake-bad", "")
	defer db.Close()
	r := NewPostgresConsentRepository(db)
	if _, err := r.ListByUser(context.Background(), "u1"); err == nil {
		t.Error("expected error")
	}
}

func TestPostgresConsentRepository_Revoke_NotFound(t *testing.T) {
	db, _ := sql.Open("id-fake-ok", "")
	defer db.Close()
	r := NewPostgresConsentRepository(db)
	if err := r.Revoke(context.Background(), "id"); err != ErrNotFound {
		t.Errorf("err = %v", err)
	}
}

func TestPostgresConsentRepository_ExpireStale_DriverError(t *testing.T) {
	db, _ := sql.Open("id-fake-bad", "")
	defer db.Close()
	r := NewPostgresConsentRepository(db)
	if _, err := r.ExpireStale(context.Background()); err == nil {
		t.Error("expected error")
	}
}
