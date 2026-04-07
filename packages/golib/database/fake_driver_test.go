package database

import (
	"database/sql"
	"database/sql/driver"
)

// fakeDriver is an in-process database/sql driver that supports the
// minimum surface required to exercise Pool.Health, Pool.WithTx, and
// related stdlib-only flows. It is registered once per test process via
// init().
//
// Why a real driver instead of a mock object: Pool.WithTx calls
// db.BeginTx → driver.Conn.Begin → driver.Tx.Commit/Rollback through the
// real database/sql machinery, including its connection pool bookkeeping.
// A hand-rolled mock at the Pool layer would skip all of that and prove
// nothing about how WithTx actually behaves under load. With a fake
// driver the path under test is byte-for-byte identical to production
// except for the I/O at the bottom.
type fakeDriver struct{}

func (fakeDriver) Open(name string) (driver.Conn, error) { return &fakeConn{}, nil }

type fakeConn struct{}

func (*fakeConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*fakeConn) Close() error                        { return nil }
func (*fakeConn) Begin() (driver.Tx, error)           { return &fakeTx{}, nil }

type fakeTx struct{}

func (*fakeTx) Commit() error   { return nil }
func (*fakeTx) Rollback() error { return nil }

func init() {
	sql.Register("database-fake", fakeDriver{})
}

// openFakePool returns a real *sql.DB backed by the fake driver.
func openFakePool() (*sql.DB, error) {
	return sql.Open("database-fake", "")
}
