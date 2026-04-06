package audit

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestFromRequest(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("X-Request-Id", "req-123")
	r.Header.Set("X-User-ID", "user-456")
	r.Header.Set("User-Agent", "TestAgent/1.0")
	r.RemoteAddr = "192.168.1.1:12345"

	ac := FromRequest(r, "bff")

	if ac.RequestID != "req-123" {
		t.Errorf("RequestID = %q, want %q", ac.RequestID, "req-123")
	}
	if ac.UserID != "user-456" {
		t.Errorf("UserID = %q, want %q", ac.UserID, "user-456")
	}
	if ac.UserAgent != "TestAgent/1.0" {
		t.Errorf("UserAgent = %q, want %q", ac.UserAgent, "TestAgent/1.0")
	}
	if ac.IPAddress != "192.168.1.1" {
		t.Errorf("IPAddress = %q, want %q", ac.IPAddress, "192.168.1.1")
	}
	if ac.ServiceName != "bff" {
		t.Errorf("ServiceName = %q, want %q", ac.ServiceName, "bff")
	}
	if ac.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestFromRequest_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	r.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2, 10.0.0.3")
	r.RemoteAddr = "192.168.1.1:12345"

	ac := FromRequest(r, "bff")

	if ac.IPAddress != "10.0.0.1" {
		t.Errorf("IPAddress = %q, want %q (first IP from X-Forwarded-For)", ac.IPAddress, "10.0.0.1")
	}
}

func TestFromRequest_NoForwardedHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	r.RemoteAddr = "192.168.1.1:12345"

	ac := FromRequest(r, "bff")

	if ac.IPAddress != "192.168.1.1" {
		t.Errorf("IPAddress = %q, want %q", ac.IPAddress, "192.168.1.1")
	}
}

func TestWithContext_GetContext_Roundtrip(t *testing.T) {
	ac := Context{
		RequestID:   "req-789",
		UserID:      "user-101",
		ServiceName: "auth",
	}

	ctx := WithContext(context.Background(), ac)
	got, ok := GetContext(ctx)

	if !ok {
		t.Fatal("GetContext returned false, want true")
	}
	if got.RequestID != ac.RequestID {
		t.Errorf("RequestID = %q, want %q", got.RequestID, ac.RequestID)
	}
	if got.UserID != ac.UserID {
		t.Errorf("UserID = %q, want %q", got.UserID, ac.UserID)
	}
	if got.ServiceName != ac.ServiceName {
		t.Errorf("ServiceName = %q, want %q", got.ServiceName, ac.ServiceName)
	}
}

func TestGetContext_EmptyContext(t *testing.T) {
	_, ok := GetContext(context.Background())
	if ok {
		t.Error("GetContext on empty context should return false")
	}
}

func TestRealIP_XForwardedFor_MultipleIPs(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.50, 70.41.3.18, 150.172.238.178")
	r.RemoteAddr = "127.0.0.1:8080"

	got := RealIP(r)
	if got != "203.0.113.50" {
		t.Errorf("RealIP = %q, want %q", got, "203.0.113.50")
	}
}

func TestRealIP_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Real-IP", "203.0.113.50")
	r.RemoteAddr = "127.0.0.1:8080"

	got := RealIP(r)
	if got != "203.0.113.50" {
		t.Errorf("RealIP = %q, want %q", got, "203.0.113.50")
	}
}

func TestRealIP_FallbackRemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.100:54321"

	got := RealIP(r)
	if got != "192.168.1.100" {
		t.Errorf("RealIP = %q, want %q", got, "192.168.1.100")
	}
}

func TestRealIP_RemoteAddrNoPort(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.168.1.100"

	got := RealIP(r)
	if got != "192.168.1.100" {
		t.Errorf("RealIP = %q, want %q", got, "192.168.1.100")
	}
}
