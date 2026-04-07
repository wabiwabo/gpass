package poolmon

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeDriver is a no-op database/sql driver registered once per test
// process. It exists solely so we can construct a real *sql.DB whose
// Stats() bookkeeping reflects pool activity (InUse, Open, etc.) without
// requiring an actual database. The driver's I/O methods are never
// reached because we only borrow connections via db.Conn().
type fakeDriver struct{}

func (fakeDriver) Open(name string) (driver.Conn, error) { return &fakeConn{}, nil }

type fakeConn struct{}

func (*fakeConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*fakeConn) Close() error                        { return nil }
func (*fakeConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func init() {
	sql.Register("poolmon-fake", fakeDriver{})
}

// openFakePool returns a real *sql.DB backed by the fake driver, with the
// given max-open setting. Caller must Close.
func openFakePool(t *testing.T, maxOpen int) *sql.DB {
	t.Helper()
	db, err := sql.Open("poolmon-fake", "")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	db.SetMaxOpenConns(maxOpen)
	return db
}

// TestMonitor_Stats_PopulatesFieldsFromRealDB exercises the previously-
// uncovered Stats() loop body — including the per-pool DBStats read,
// the WaitDurationStr formatting, and the per-pool status evaluation —
// against a real *sql.DB so the bookkeeping is honest.
func TestMonitor_Stats_PopulatesFieldsFromRealDB(t *testing.T) {
	db := openFakePool(t, 4)
	defer db.Close()

	m := NewMonitor(DefaultThresholds())
	m.Register("primary", db)

	stats := m.Stats()
	if len(stats) != 1 {
		t.Fatalf("got %d entries, want 1", len(stats))
	}
	s := stats[0]
	if s.Name != "primary" {
		t.Errorf("Name = %q", s.Name)
	}
	if s.MaxOpen != 4 {
		t.Errorf("MaxOpen = %d, want 4", s.MaxOpen)
	}
	if s.Status != "healthy" {
		t.Errorf("idle pool should be healthy, got %q", s.Status)
	}
	// WaitDurationStr is the formatted version of WaitDuration; for an
	// idle pool this is "0s".
	if s.WaitDurationStr == "" {
		t.Error("WaitDurationStr should be populated")
	}
}

// TestMonitor_IsHealthy_FalseWhenCritical drives Stats() through a real
// pool that has been pushed into the "critical" band by saturating its
// connection budget. This covers the IsHealthy false-return branch
// (50%→100%) which has no other safe way to be exercised.
func TestMonitor_IsHealthy_FalseWhenCritical(t *testing.T) {
	db := openFakePool(t, 1)
	defer db.Close()

	// Borrow the only connection so InUse=1, MaxOpen=1 → 100% utilization
	// which is >= 90% critical threshold.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("Conn: %v", err)
	}
	defer conn.Close()

	m := NewMonitor(DefaultThresholds())
	m.Register("hot", db)

	if m.IsHealthy() {
		t.Error("IsHealthy should be false when a pool is at 100% utilization")
	}

	// Sanity check: Handler also reflects the critical state in JSON.
	w := httptest.NewRecorder()
	m.Handler()(w, httptest.NewRequest(http.MethodGet, "/", nil))
	var out []PoolStats
	if err := json.NewDecoder(w.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out) != 1 || out[0].Status != "critical" {
		t.Errorf("Handler status = %+v, want one critical entry", out)
	}
}

// TestEvaluate_WaitDurationWarning covers the wait-time warning branch
// in evaluate (waitCount > 0 && avg >= WarnWaitDuration), which the
// existing tests didn't reach because they only varied utilization.
func TestEvaluate_WaitDurationWarning(t *testing.T) {
	m := NewMonitor(Thresholds{
		WarnUtilization:  0.7,
		CritUtilization:  0.9,
		WarnWaitDuration: 10 * time.Millisecond,
	})
	status := m.evaluate(sql.DBStats{
		MaxOpenConnections: 100,
		InUse:              10,             // utilization 10% — healthy
		WaitCount:          5,              // 5 waits
		WaitDuration:       100 * time.Millisecond, // avg 20ms > 10ms threshold
	})
	if status != "warning" {
		t.Errorf("expected warning from wait duration, got %q", status)
	}
}
