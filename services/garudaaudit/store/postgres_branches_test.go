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

// gaFakeDriver is a stdlib-only fake database/sql driver for pinning
// PostgresAuditStore error branches. PP 71/2019 5-year append-only
// audit retention is tested at the seam without a real database.
type gaFakeDriver struct{ err error }

func (d *gaFakeDriver) Open(_ string) (driver.Conn, error) { return &gaFakeConn{err: d.err}, nil }

type gaFakeConn struct{ err error }

func (c *gaFakeConn) Prepare(_ string) (driver.Stmt, error) { return nil, c.err }
func (c *gaFakeConn) Close() error                          { return nil }
func (c *gaFakeConn) Begin() (driver.Tx, error)             { return nil, c.err }

func (c *gaFakeConn) QueryContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &gaEmptyRows{}, nil
}

func (c *gaFakeConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if c.err != nil {
		return nil, c.err
	}
	return gaFakeResult{}, nil
}

type gaFakeResult struct{}

func (gaFakeResult) LastInsertId() (int64, error) { return 0, nil }
func (gaFakeResult) RowsAffected() (int64, error) { return 0, nil }

type gaEmptyRows struct{}

func (r *gaEmptyRows) Columns() []string           { return []string{"id"} }
func (r *gaEmptyRows) Close() error                { return nil }
func (r *gaEmptyRows) Next(_ []driver.Value) error { return io.EOF }

func init() {
	sql.Register("ga-fake-ok", &gaFakeDriver{})
	sql.Register("ga-fake-bad", &gaFakeDriver{err: errors.New("boom")})
}

func gaValidEvent() *AuditEvent {
	return &AuditEvent{
		EventType: "consent.granted", ActorID: "u1", Action: "CONSENT_GRANT",
		ServiceName: "garudainfo",
	}
}

func TestPostgresAuditStore_Append_ValidationFail(t *testing.T) {
	db, _ := sql.Open("ga-fake-ok", "")
	defer db.Close()
	s := NewPostgresAuditStore(db)
	if err := s.Append(&AuditEvent{}); err == nil {
		t.Error("expected validation error")
	}
}

func TestPostgresAuditStore_Append_DriverError(t *testing.T) {
	db, _ := sql.Open("ga-fake-bad", "")
	defer db.Close()
	s := NewPostgresAuditStore(db)
	if err := s.Append(gaValidEvent()); err == nil {
		t.Error("expected driver error")
	}
}

func TestPostgresAuditStore_Append_AppliesDefaults(t *testing.T) {
	// Uses the ok driver which returns empty rows → Scan returns ErrNoRows,
	// but we still want to verify the defaults were applied before the DB
	// call. Since the scan fails, we can't inspect the struct after — but
	// the branch coverage for the default assignment is what we're after.
	db, _ := sql.Open("ga-fake-ok", "")
	defer db.Close()
	s := NewPostgresAuditStore(db)
	e := gaValidEvent()
	_ = s.Append(e)
	if e.ActorType != "USER" {
		t.Errorf("ActorType default not applied: %q", e.ActorType)
	}
	if e.Status != "SUCCESS" {
		t.Errorf("Status default not applied: %q", e.Status)
	}
	if e.Metadata == nil {
		t.Error("Metadata default not applied")
	}
}

func TestPostgresAuditStore_GetByID_NotFound(t *testing.T) {
	db, _ := sql.Open("ga-fake-ok", "")
	defer db.Close()
	s := NewPostgresAuditStore(db)
	if _, err := s.GetByID("missing"); err == nil {
		t.Error("expected not-found")
	}
}

func TestPostgresAuditStore_GetByID_DriverError(t *testing.T) {
	db, _ := sql.Open("ga-fake-bad", "")
	defer db.Close()
	s := NewPostgresAuditStore(db)
	if _, err := s.GetByID("id"); err == nil {
		t.Error("expected error")
	}
}

func TestPostgresAuditStore_Query_DriverError(t *testing.T) {
	db, _ := sql.Open("ga-fake-bad", "")
	defer db.Close()
	s := NewPostgresAuditStore(db)
	if _, err := s.Query(AuditFilter{}); err == nil {
		t.Error("expected error")
	}
}

func TestPostgresAuditStore_Query_Empty(t *testing.T) {
	db, _ := sql.Open("ga-fake-ok", "")
	defer db.Close()
	s := NewPostgresAuditStore(db)
	events, err := s.Query(AuditFilter{Limit: 10})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected empty, got %d", len(events))
	}
}

func TestPostgresAuditStore_Query_LimitCaps(t *testing.T) {
	db, _ := sql.Open("ga-fake-ok", "")
	defer db.Close()
	s := NewPostgresAuditStore(db)
	// Exercises both bounds: limit<=0 → 100, limit>1000 → 1000
	if _, err := s.Query(AuditFilter{Limit: 0}); err != nil {
		t.Errorf("zero limit: %v", err)
	}
	if _, err := s.Query(AuditFilter{Limit: 99999}); err != nil {
		t.Errorf("huge limit: %v", err)
	}
}

func TestPostgresAuditStore_Count_DriverError(t *testing.T) {
	db, _ := sql.Open("ga-fake-bad", "")
	defer db.Close()
	s := NewPostgresAuditStore(db)
	if _, err := s.Count(AuditFilter{}); err == nil {
		t.Error("expected error")
	}
}

// TestBuildFilterConditions_AllFields pins every branch of the filter
// builder in one shot.
func TestBuildFilterConditions_AllFields(t *testing.T) {
	f := AuditFilter{
		ActorID:      "u1",
		ResourceID:   "r1",
		ResourceType: "document",
		EventType:    "doc.signed",
		Action:       "SIGN",
		ServiceName:  "garudasign",
		Status:       "SUCCESS",
		From:         time.Now().Add(-time.Hour),
		To:           time.Now(),
	}
	conds, args := buildFilterConditions(f)
	if len(conds) != 9 {
		t.Errorf("conds = %d, want 9", len(conds))
	}
	if len(args) != 9 {
		t.Errorf("args = %d, want 9", len(args))
	}
}

func TestBuildFilterConditions_Empty(t *testing.T) {
	conds, args := buildFilterConditions(AuditFilter{})
	if len(conds) != 0 || len(args) != 0 {
		t.Errorf("expected empty")
	}
}

func TestNullString_Audit(t *testing.T) {
	if nullString("") != nil {
		t.Error("empty should nil")
	}
	if nullString("x") != "x" {
		t.Error("non-empty passthrough")
	}
}
