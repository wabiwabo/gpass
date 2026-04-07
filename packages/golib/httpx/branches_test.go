package httpx

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// hxFakeDriver is a minimal database/sql driver that can be configured to
// succeed or fail PingContext. It's used to exercise the readiness and
// metrics handlers' DB-aware branches without standing up Postgres.
type hxFakeDriver struct{ pingErr error }

func (d *hxFakeDriver) Open(_ string) (driver.Conn, error) {
	return &hxFakeConn{pingErr: d.pingErr}, nil
}

type hxFakeConn struct{ pingErr error }

func (*hxFakeConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*hxFakeConn) Close() error                        { return nil }
func (*hxFakeConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

// Ping implements driver.Pinger so PingContext on the *sql.DB is routed
// here instead of falling back to BeginTx.
func (c *hxFakeConn) Ping(_ context.Context) error { return c.pingErr }

func init() {
	sql.Register("httpx-ok", &hxFakeDriver{})
	sql.Register("httpx-bad", &hxFakeDriver{pingErr: errors.New("ping refused")})
}

// TestReadiness_DBPingSuccess pins the success path through the DB-backed
// branch of Handler — must return 200 with pool stats.
func TestReadiness_DBPingSuccess(t *testing.T) {
	db, err := sql.Open("httpx-ok", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := NewReadiness("svc", db)
	rec := httptest.NewRecorder()
	r.Handler()(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != 200 {
		t.Errorf("code = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"db":"postgres"`) || !strings.Contains(body, `"open":`) {
		t.Errorf("body missing pool stats: %q", body)
	}
}

// TestReadiness_DBPingFails pins the ping_failed branch — must return
// 503 with an error message.
func TestReadiness_DBPingFails(t *testing.T) {
	db, err := sql.Open("httpx-bad", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	r := NewReadiness("svc", db)
	rec := httptest.NewRecorder()
	r.Handler()(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ping_failed"`) {
		t.Errorf("body missing reason: %q", rec.Body)
	}
}

// TestMetrics_HandlerWithDB pins the db != nil branch in Metrics.Handler
// — must emit db_pool_* gauges in addition to the standard metrics.
func TestMetrics_HandlerWithDB(t *testing.T) {
	db, _ := sql.Open("httpx-ok", "")
	defer db.Close()

	m := NewMetrics("svc-x")
	rec := httptest.NewRecorder()
	m.Handler(db)(rec, httptest.NewRequest("GET", "/metrics", nil))

	body := rec.Body.String()
	for _, want := range []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"http_panics_total",
		"go_goroutines",
		"go_memstats_alloc_bytes",
		"db_pool_open_connections",
		"db_pool_in_use",
		"db_pool_idle",
		"db_pool_wait_count",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("metrics body missing %q", want)
		}
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q", ct)
	}
}
