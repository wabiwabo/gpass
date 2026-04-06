package slogattr

import (
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequest(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/users?page=1", nil)
	req.Header.Set("User-Agent", "TestClient/1.0")
	attrs := Request(req)
	if len(attrs) < 3 { t.Errorf("attrs = %d", len(attrs)) }
}

func TestError_Nil(t *testing.T) {
	a := Error(nil)
	if a.Value.String() != "" { t.Error("nil error") }
}

func TestUserID(t *testing.T) {
	a := UserID("u-1")
	if a.Key != "user_id" { t.Error("key") }
}

func TestTenantID(t *testing.T) {
	a := TenantID("t-1")
	if a.Key != "tenant_id" { t.Error("key") }
}

func TestRequestID(t *testing.T) {
	a := RequestID("r-1")
	if a.Key != "request_id" { t.Error("key") }
}

func TestDuration(t *testing.T) {
	a := Duration(5 * time.Second)
	if a.Key != "duration" { t.Error("key") }
}

func TestStatus(t *testing.T) {
	a := Status(200)
	if a.Key != "status" { t.Error("key") }
}

func TestService(t *testing.T) {
	a := Service("identity")
	if a.Key != "service" { t.Error("key") }
}

func TestOperation(t *testing.T) {
	a := Operation("verify_nik")
	if a.Key != "operation" { t.Error("key") }
}

func TestResource(t *testing.T) {
	a := Resource("user", "u-123")
	if a.Key != "resource" { t.Error("key") }
	if a.Value.Kind() != slog.KindGroup { t.Error("should be group") }
}

func TestCount(t *testing.T) {
	a := Count(42)
	if a.Key != "count" { t.Error("key") }
}

func TestTraceID(t *testing.T) {
	a := TraceID("t-abc")
	if a.Key != "trace_id" { t.Error("key") }
}
