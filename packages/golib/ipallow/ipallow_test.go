package ipallow

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestList_AllowList_SingleIP(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("10.0.0.1")

	if !l.IsAllowed("10.0.0.1") {
		t.Error("listed IP should be allowed")
	}
	if l.IsAllowed("10.0.0.2") {
		t.Error("unlisted IP should be blocked")
	}
}

func TestList_AllowList_CIDR(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("10.0.0.0/24")

	if !l.IsAllowed("10.0.0.1") {
		t.Error("IP in CIDR should be allowed")
	}
	if !l.IsAllowed("10.0.0.255") {
		t.Error("last IP in CIDR should be allowed")
	}
	if l.IsAllowed("10.0.1.1") {
		t.Error("IP outside CIDR should be blocked")
	}
}

func TestList_BlockList(t *testing.T) {
	l := New(ModeBlockList)
	l.Block("192.168.1.100")

	if l.IsAllowed("192.168.1.100") {
		t.Error("blocked IP should not be allowed")
	}
	if !l.IsAllowed("192.168.1.101") {
		t.Error("unblocked IP should be allowed")
	}
}

func TestList_BlockOverridesAllow(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("10.0.0.0/24")
	l.Block("10.0.0.5")

	if l.IsAllowed("10.0.0.5") {
		t.Error("blocked IP should override allowlist")
	}
	if !l.IsAllowed("10.0.0.6") {
		t.Error("non-blocked IP should be allowed")
	}
}

func TestList_EmptyAllowList(t *testing.T) {
	l := New(ModeAllowList)
	// Empty allowlist = allow all.
	if !l.IsAllowed("1.2.3.4") {
		t.Error("empty allowlist should allow all")
	}
}

func TestList_InvalidIP(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("10.0.0.1")

	if l.IsAllowed("not-an-ip") {
		t.Error("invalid IP should not be allowed")
	}
}

func TestList_InvalidCIDR(t *testing.T) {
	l := New(ModeAllowList)
	err := l.Allow("not-a-cidr")
	if err == nil {
		t.Error("invalid CIDR should fail")
	}
}

func TestList_Count(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("10.0.0.1")
	l.Allow("10.0.0.2")
	l.Block("192.168.1.1")

	allowed, blocked := l.Count()
	if allowed != 2 {
		t.Errorf("allowed: got %d", allowed)
	}
	if blocked != 1 {
		t.Errorf("blocked: got %d", blocked)
	}
}

func TestMiddleware_Allowed(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("192.0.2.1")

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("allowed IP: got %d", w.Code)
	}
}

func TestMiddleware_Blocked(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("10.0.0.1")

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("blocked IP: got %d", w.Code)
	}
}

func TestMiddleware_XForwardedFor(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("10.0.0.1")

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("XFF: got %d", w.Code)
	}
}

func TestMiddleware_XRealIP(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("10.0.0.1")

	handler := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "10.0.0.1")
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("X-Real-IP: got %d", w.Code)
	}
}

func TestList_IPv6(t *testing.T) {
	l := New(ModeAllowList)
	l.Allow("::1")

	if !l.IsAllowed("::1") {
		t.Error("IPv6 loopback should be allowed")
	}
}
