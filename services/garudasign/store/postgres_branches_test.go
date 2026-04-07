package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudasign/signing"
)

// gsFakeDriver is a stdlib-only fake database/sql driver used to pin
// error branches in the garudasign PostgresCertificate/Request/Document
// stores. 12factor Factor VI — backing services at the seam — is tested
// here without provisioning real Postgres.
type gsFakeDriver struct{ err error }

func (d *gsFakeDriver) Open(_ string) (driver.Conn, error) { return &gsFakeConn{err: d.err}, nil }

type gsFakeConn struct{ err error }

func (c *gsFakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, c.err }
func (c *gsFakeConn) Close() error                          { return nil }
func (c *gsFakeConn) Begin() (driver.Tx, error)             { return nil, c.err }

func (c *gsFakeConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &gsEmptyRows{}, nil
}

func (c *gsFakeConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.err != nil {
		return nil, c.err
	}
	return gsFakeResult{}, nil
}

type gsFakeResult struct{}

func (gsFakeResult) LastInsertId() (int64, error) { return 0, nil }
func (gsFakeResult) RowsAffected() (int64, error) { return 0, nil }

type gsEmptyRows struct{}

func (r *gsEmptyRows) Columns() []string           { return []string{"id"} }
func (r *gsEmptyRows) Close() error                { return nil }
func (r *gsEmptyRows) Next(_ []driver.Value) error { return io.EOF }

func init() {
	sql.Register("gs-fake-ok", &gsFakeDriver{})
	sql.Register("gs-fake-bad", &gsFakeDriver{err: errors.New("boom")})
}

func gsValidCert() *signing.Certificate {
	return &signing.Certificate{
		UserID: "u1", SerialNumber: "SN", IssuerDN: "CN=CA", SubjectDN: "CN=User",
		Status: "ACTIVE", ValidFrom: time.Now(), ValidTo: time.Now().Add(time.Hour),
		CertificatePEM: "pem", FingerprintSHA256: "fp",
	}
}

// TestPostgresCertificateStore_Create_ValidationFail pins the pre-DB guard.
func TestPostgresCertificateStore_Create_ValidationFail(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresCertificateStore(db)
	if _, err := s.Create(&signing.Certificate{}); err == nil {
		t.Error("expected validation error")
	}
}

// TestPostgresCertificateStore_Create_DriverError pins the DB wrap.
func TestPostgresCertificateStore_Create_DriverError(t *testing.T) {
	db, _ := sql.Open("gs-fake-bad", "")
	defer db.Close()
	s := NewPostgresCertificateStore(db)
	if _, err := s.Create(gsValidCert()); err == nil {
		t.Error("expected error")
	}
}

// TestPostgresCertificateStore_GetByID_NotFound pins the sql.ErrNoRows branch.
func TestPostgresCertificateStore_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresCertificateStore(db)
	if _, err := s.GetByID("id"); err == nil {
		t.Error("expected not-found")
	}
}

// TestPostgresCertificateStore_GetActiveByUser_NotFound pins no-active branch.
func TestPostgresCertificateStore_GetActiveByUser_NotFound(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresCertificateStore(db)
	if _, err := s.GetActiveByUser("u1"); err == nil {
		t.Error("expected not-found")
	}
}

// TestPostgresCertificateStore_ListByUser_DriverError pins the list wrap.
func TestPostgresCertificateStore_ListByUser_DriverError(t *testing.T) {
	db, _ := sql.Open("gs-fake-bad", "")
	defer db.Close()
	s := NewPostgresCertificateStore(db)
	if _, err := s.ListByUser("u1", "ACTIVE"); err == nil {
		t.Error("expected error")
	}
}

// TestPostgresCertificateStore_ListByUser_Empty pins the empty-rows path.
func TestPostgresCertificateStore_ListByUser_Empty(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresCertificateStore(db)
	certs, err := s.ListByUser("u1", "")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(certs) != 0 {
		t.Errorf("expected empty, got %d", len(certs))
	}
}

// TestPostgresCertificateStore_UpdateStatus_ValidationFail pins pre-DB guard.
func TestPostgresCertificateStore_UpdateStatus_ValidationFail(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresCertificateStore(db)
	if err := s.UpdateStatus("id", "BOGUS", nil, ""); err == nil {
		t.Error("expected validation error")
	}
}

// TestPostgresCertificateStore_UpdateStatus_NotFound pins 0-rows-affected.
func TestPostgresCertificateStore_UpdateStatus_NotFound(t *testing.T) {
	db, _ := sql.Open("gs-fake-ok", "")
	defer db.Close()
	s := NewPostgresCertificateStore(db)
	now := time.Now()
	if err := s.UpdateStatus("id", "REVOKED", &now, "key_compromise"); err == nil {
		t.Error("expected not-found")
	}
}

// TestPostgresCertificateStore_UpdateStatus_DriverError pins the wrap.
func TestPostgresCertificateStore_UpdateStatus_DriverError(t *testing.T) {
	db, _ := sql.Open("gs-fake-bad", "")
	defer db.Close()
	s := NewPostgresCertificateStore(db)
	now := time.Now()
	if err := s.UpdateStatus("id", "REVOKED", &now, "key_compromise"); err == nil {
		t.Error("expected driver error")
	}
}

// TestNullStrSign_NullTimePtr pins the null helpers.
func TestNullStrSign_NullTimePtr(t *testing.T) {
	if nullStrSign("") != nil {
		t.Error("empty should nil")
	}
	if nullStrSign("x") != "x" {
		t.Error("non-empty passthrough")
	}
	if nullTimePtr(nil) != nil {
		t.Error("nil time should nil")
	}
	now := time.Now()
	if got := nullTimePtr(&now); got == nil {
		t.Error("non-nil time passthrough")
	}
}
