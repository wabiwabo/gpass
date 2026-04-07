package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/ubo"
)

// gcTxDriver is a smarter fake driver that returns exists=true for the
// EXISTS probe and success for subsequent INSERT/UPDATE/DELETE statements,
// letting us drive AddOfficers/AddShareholders/Save to their happy-path
// commit. Complementary to gcFakeDriver which only toggles error/empty.
type gcTxDriver struct{}

func (*gcTxDriver) Open(_ string) (driver.Conn, error) { return &gcTxConn{}, nil }

type gcTxConn struct{}

func (*gcTxConn) Prepare(_ string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*gcTxConn) Close() error                          { return nil }
func (*gcTxConn) Begin() (driver.Tx, error)             { return &gcTxTx{}, nil }

func (c *gcTxConn) QueryContext(_ context.Context, q string, _ []driver.NamedValue) (driver.Rows, error) {
	// EXISTS probe returns bool=true
	if strings.Contains(q, "EXISTS") {
		return &gcBoolRows{}, nil
	}
	// INSERT ... RETURNING id returns a synthetic id string
	if strings.Contains(q, "RETURNING id") {
		return &gcIDRows{}, nil
	}
	return &gcEmptyRows2{}, nil
}

func (c *gcTxConn) ExecContext(_ context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	return gcTxResult{}, nil
}

type gcTxTx struct{}

func (gcTxTx) Commit() error   { return nil }
func (gcTxTx) Rollback() error { return nil }

type gcTxResult struct{}

func (gcTxResult) LastInsertId() (int64, error) { return 0, nil }
func (gcTxResult) RowsAffected() (int64, error) { return 1, nil }

type gcBoolRows struct{ done bool }

func (r *gcBoolRows) Columns() []string { return []string{"exists"} }
func (r *gcBoolRows) Close() error      { return nil }
func (r *gcBoolRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = true
	return nil
}

type gcIDRows struct{ done bool }

func (r *gcIDRows) Columns() []string { return []string{"id"} }
func (r *gcIDRows) Close() error      { return nil }
func (r *gcIDRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	r.done = true
	dest[0] = "synthetic-id"
	return nil
}

type gcEmptyRows2 struct{}

func (r *gcEmptyRows2) Columns() []string           { return []string{"id"} }
func (r *gcEmptyRows2) Close() error                { return nil }
func (r *gcEmptyRows2) Next(_ []driver.Value) error { return io.EOF }

func init() {
	sql.Register("gc-tx", &gcTxDriver{})
}

// TestPostgresEntityStore_AddOfficers_Happy pins the full tx happy path
// — validate, begin, EXISTS, per-officer INSERT, UPDATE updated_at, commit.
func TestPostgresEntityStore_AddOfficers_Happy(t *testing.T) {
	db, _ := sql.Open("gc-tx", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	err := s.AddOfficers(context.Background(), "e1", []EntityOfficer{
		{NIKToken: "t1", Name: "Budi", Position: "DIRECTOR", AppointmentDate: "2024-01-01"},
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestPostgresEntityStore_AddShareholders_Happy(t *testing.T) {
	db, _ := sql.Open("gc-tx", "")
	defer db.Close()
	s := NewPostgresEntityStore(db)
	err := s.AddShareholders(context.Background(), "e1", []EntityShareholder{
		{Name: "Budi", ShareType: "INDIVIDUAL", Shares: 100, Percentage: 25.5},
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestPostgresUBOStore_Save_Happy(t *testing.T) {
	db, _ := sql.Open("gc-tx", "")
	defer db.Close()
	s := NewPostgresUBOStore(db)
	err := s.Save(&ubo.AnalysisResult{
		EntityID:   "e1",
		EntityName: "PT X",
		Criteria:   "ownership_25",
		Status:     "IDENTIFIED",
		BeneficialOwners: []ubo.BeneficialOwner{
			{Name: "Budi", NIKToken: "t1", OwnershipType: "DIRECT_SHARES", Percentage: 30, Source: "AHU"},
		},
	})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
}
