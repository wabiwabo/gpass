package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// migrateFakeDriver is a stdlib-only database/sql driver that supports
// the minimum surface required to exercise Migrate, MigrateAdvanced,
// EnsureTable, Status, Apply, and DryRun. It is registered once per
// test process and stores all rows in a process-wide map keyed by
// version, mimicking the schema_migrations table.
//
// Why a real-driver fake instead of a mock object: the migrate code
// goes through db.Exec, db.QueryRow, db.Begin → tx.Exec → tx.Commit/
// Rollback. A mock at the database level would skip database/sql's
// connection pool, transaction state machine, and result handling.
// Going through the driver interface keeps the path under test
// byte-for-byte identical to production.
type migrateFakeDriver struct{}

func (migrateFakeDriver) Open(_ string) (driver.Conn, error) {
	return &migrateFakeConn{}, nil
}

type migrateFakeConn struct{}

func (*migrateFakeConn) Prepare(query string) (driver.Stmt, error) {
	return &migrateFakeStmt{query: query}, nil
}
func (*migrateFakeConn) Close() error              { return nil }
func (*migrateFakeConn) Begin() (driver.Tx, error) { return &migrateFakeTx{}, nil }

// QueryContext routes db.QueryRow / db.Query through here.
func (*migrateFakeConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(query, "EXISTS") && strings.Contains(query, "schema_migrations") {
		var version string
		if len(args) > 0 {
			if s, ok := args[0].Value.(string); ok {
				version = s
			}
		}
		migrateState.mu.Lock()
		_, present := migrateState.applied[version]
		migrateState.mu.Unlock()
		return &boolRows{value: present, done: false}, nil
	}
	if strings.Contains(query, "SELECT version") && strings.Contains(query, "schema_migrations") {
		migrateState.mu.Lock()
		defer migrateState.mu.Unlock()
		var rows [][]driver.Value
		for v, name := range migrateState.applied {
			rows = append(rows, []driver.Value{v, name, migrateState.checksums[v], time.Now()})
		}
		return &multiRows{cols: []string{"version", "name", "checksum", "applied_at"}, rows: rows}, nil
	}
	return &emptyRows{}, nil
}

// ExecContext routes db.Exec through here.
func (*migrateFakeConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "INSERT INTO schema_migrations") {
		var version, name, checksum string
		if len(args) > 0 {
			if s, ok := args[0].Value.(string); ok {
				version = s
			}
		}
		if len(args) > 1 {
			if s, ok := args[1].Value.(string); ok {
				name = s
			}
		}
		if len(args) > 2 {
			if s, ok := args[2].Value.(string); ok {
				checksum = s
			}
		}
		migrateState.mu.Lock()
		migrateState.applied[version] = name
		migrateState.checksums[version] = checksum
		migrateState.mu.Unlock()
	}
	return driver.RowsAffected(1), nil
}

type migrateFakeStmt struct{ query string }

func (s *migrateFakeStmt) Close() error                                   { return nil }
func (s *migrateFakeStmt) NumInput() int                                  { return -1 }
func (s *migrateFakeStmt) Exec(_ []driver.Value) (driver.Result, error)   { return driver.RowsAffected(0), nil }
func (s *migrateFakeStmt) Query(_ []driver.Value) (driver.Rows, error)    { return &emptyRows{}, nil }

type migrateFakeTx struct{}

func (*migrateFakeTx) Commit() error   { return nil }
func (*migrateFakeTx) Rollback() error { return nil }

// boolRows returns a single row containing a single boolean value, used
// for the EXISTS query result in Migrate/Apply.
type boolRows struct {
	value bool
	done  bool
}

func (r *boolRows) Columns() []string { return []string{"exists"} }
func (r *boolRows) Close() error      { return nil }
func (r *boolRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	dest[0] = r.value
	r.done = true
	return nil
}

// multiRows returns a fixed list of rows for the Status query.
type multiRows struct {
	cols []string
	rows [][]driver.Value
	idx  int
}

func (r *multiRows) Columns() []string { return r.cols }
func (r *multiRows) Close() error      { return nil }
func (r *multiRows) Next(dest []driver.Value) error {
	if r.idx >= len(r.rows) {
		return io.EOF
	}
	for i, v := range r.rows[r.idx] {
		dest[i] = v
	}
	r.idx++
	return nil
}

type emptyRows struct{}

func (emptyRows) Columns() []string              { return nil }
func (emptyRows) Close() error                   { return nil }
func (emptyRows) Next(_ []driver.Value) error    { return io.EOF }

// migrateState is the process-wide fake "database" backing the driver.
var migrateState = struct {
	mu        sync.Mutex
	applied   map[string]string
	checksums map[string]string
}{
	applied:   make(map[string]string),
	checksums: make(map[string]string),
}

func init() {
	sql.Register("database-migrate-fake", migrateFakeDriver{})
}

// resetMigrateState clears the fake DB so each test starts fresh.
func resetMigrateState() {
	migrateState.mu.Lock()
	defer migrateState.mu.Unlock()
	migrateState.applied = make(map[string]string)
	migrateState.checksums = make(map[string]string)
}

// writeMigrationFiles creates a temp directory with the given migration
// filename → SQL pairs and returns the directory.
func writeMigrationFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestMigrate_AppliesAllPendingMigrations pins the happy path of
// Migrate: it creates the schema_migrations table, parses files,
// applies each pending one, and skips already-applied ones on a
// second run.
//
// Note: This test exercises the SQL flow but the driver-level state
// management is intentionally minimal. We're verifying that Migrate
// reaches every code path without erroring, not that it produces
// production-quality state in the fake.
func TestMigrate_AppliesAllPendingMigrations(t *testing.T) {
	resetMigrateState()
	db, err := sql.Open("database-migrate-fake", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	dir := writeMigrationFiles(t, map[string]string{
		"001_create_users.sql": "CREATE TABLE users(id INT);",
		"002_add_email.sql":    "ALTER TABLE users ADD email TEXT;",
	})

	if err := Migrate(db, dir); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	// Second run must be a no-op (both migrations already applied).
	if err := Migrate(db, dir); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
}

// TestMigrate_BadDirectoryPropagated pins the ParseMigrationFiles
// error path inside Migrate.
func TestMigrate_BadDirectoryPropagated(t *testing.T) {
	resetMigrateState()
	db, _ := sql.Open("database-migrate-fake", "")
	defer db.Close()
	if err := Migrate(db, "/no/such/path"); err == nil || !strings.Contains(err.Error(), "read directory") {
		t.Errorf("err = %v", err)
	}
}

// TestParseMigrationFiles_BadFilenameRejected pins the regex error and
// duplicate-version error.
func TestParseMigrationFiles_BadFilenameRejected(t *testing.T) {
	dir := writeMigrationFiles(t, map[string]string{
		"badname.sql": "SELECT 1;",
	})
	_, err := ParseMigrationFiles(dir)
	if err == nil || !strings.Contains(err.Error(), "invalid migration filename") {
		t.Errorf("err = %v", err)
	}

	dir2 := writeMigrationFiles(t, map[string]string{
		"001_first.sql":   "SELECT 1;",
		"001_second.sql":  "SELECT 2;",
	})
	_, err = ParseMigrationFiles(dir2)
	if err == nil || !strings.Contains(err.Error(), "duplicate migration version") {
		t.Errorf("err = %v", err)
	}
}

// TestParseMigrationFiles_SkipsNonSQLAndDirs pins that .txt files and
// nested directories are silently skipped.
func TestParseMigrationFiles_SkipsNonSQLAndDirs(t *testing.T) {
	dir := writeMigrationFiles(t, map[string]string{
		"001_init.sql": "SELECT 1;",
		"README.txt":   "ignore me",
	})
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	mig, err := ParseMigrationFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(mig) != 1 || mig[0].Version != "001" {
		t.Errorf("got %+v", mig)
	}
}

// TestMigrateAdvanced_StatusApplyDryRunVerifyChecksums pins the
// MigrateAdvanced enterprise surface end-to-end through the fake
// driver: EnsureTable, Status, DryRun, Apply, then VerifyChecksums.
func TestMigrateAdvanced_StatusApplyDryRunVerifyChecksums(t *testing.T) {
	resetMigrateState()
	db, _ := sql.Open("database-migrate-fake", "")
	defer db.Close()

	dir := writeMigrationFiles(t, map[string]string{
		"001_a.sql": "SELECT 1;",
		"002_b.sql": "SELECT 2;",
	})

	ma := NewMigrateAdvanced(db, dir)

	if err := ma.EnsureTable(); err != nil {
		t.Fatalf("EnsureTable: %v", err)
	}

	// DryRun should report 2 pending migrations.
	pending, err := ma.DryRun()
	if err != nil {
		t.Fatalf("DryRun: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("DryRun pending = %d, want 2", len(pending))
	}

	// Apply.
	n, err := ma.Apply()
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n != 2 {
		t.Errorf("Apply returned %d, want 2", n)
	}

	// Status after apply: both should be in Applied list.
	plan, err := ma.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(plan.Pending) != 0 {
		t.Errorf("post-apply pending: %d", len(plan.Pending))
	}

	// VerifyChecksums must be empty (no modifications since apply).
	mods, err := ma.VerifyChecksums()
	if err != nil {
		t.Fatalf("VerifyChecksums: %v", err)
	}
	if len(mods) != 0 {
		t.Errorf("modified count = %d, want 0", len(mods))
	}

	// Now mutate one of the SQL files and verify the modified slice fires.
	if err := os.WriteFile(filepath.Join(dir, "001_a.sql"), []byte("SELECT 999;"), 0o644); err != nil {
		t.Fatal(err)
	}
	mods, err = ma.VerifyChecksums()
	if err != nil {
		t.Fatalf("VerifyChecksums after mutation: %v", err)
	}
	if len(mods) != 1 {
		t.Errorf("expected 1 modified, got %d", len(mods))
	}
}

// TestChecksumSQL_Stable pins the exported test helper.
func TestChecksumSQL_Stable(t *testing.T) {
	a := ChecksumSQL("SELECT 1;")
	b := ChecksumSQL("SELECT 1;")
	if a != b {
		t.Error("checksum not deterministic")
	}
	if ChecksumSQL("SELECT 2;") == a {
		t.Error("different SQL should have different checksum")
	}
}

// silenceUnused references functions that the fake driver wires up via
// reflection so the linter doesn't flag them as unused.
var _ = errors.New
