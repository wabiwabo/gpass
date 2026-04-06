package limiter

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegistry_Allow(t *testing.T) {
	r := NewRegistry(Config{Rate: 10, Burst: 5, Interval: time.Second})

	for i := 0; i < 5; i++ {
		if !r.Allow("/api/users", "10.0.0.1") {
			t.Errorf("request %d should be allowed", i)
		}
	}

	if r.Allow("/api/users", "10.0.0.1") {
		t.Error("6th request should be rejected")
	}
}

func TestRegistry_PerKeyIsolation(t *testing.T) {
	r := NewRegistry(Config{Rate: 10, Burst: 2, Interval: time.Second})

	r.Allow("/api", "user-a")
	r.Allow("/api", "user-a")
	// User A exhausted.

	if !r.Allow("/api", "user-b") {
		t.Error("different user should have own bucket")
	}
}

func TestRegistry_NamedConfig(t *testing.T) {
	r := NewRegistry(Config{Rate: 10, Burst: 5, Interval: time.Second})
	r.Configure("/api/auth", Config{Rate: 3, Burst: 2, Interval: time.Second})

	// Auth endpoint has stricter limits.
	r.Allow("/api/auth", "ip")
	r.Allow("/api/auth", "ip")
	if r.Allow("/api/auth", "ip") {
		t.Error("auth should be limited at 2 burst")
	}
}

func TestRegistry_FallbackConfig(t *testing.T) {
	r := NewRegistry(Config{Rate: 10, Burst: 3, Interval: time.Second})

	// No specific config for this endpoint — should use fallback.
	for i := 0; i < 3; i++ {
		r.Allow("/api/unknown", "ip")
	}
	if r.Allow("/api/unknown", "ip") {
		t.Error("should use fallback burst of 3")
	}
}

func TestRegistry_Middleware(t *testing.T) {
	r := NewRegistry(Config{Rate: 10, Burst: 1, Interval: time.Second})
	handler := r.Middleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "1.2.3.4:5678"

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Error("first should pass")
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("second should be rate limited: got %d", w.Code)
	}
}

func TestRegistry_Size(t *testing.T) {
	r := NewRegistry(Config{Rate: 10, Burst: 5, Interval: time.Second})
	r.Allow("/a", "ip1")
	r.Allow("/b", "ip2")

	if r.Size() != 2 {
		t.Errorf("size: got %d", r.Size())
	}
}

func TestRegistry_ConfigCount(t *testing.T) {
	r := NewRegistry(Config{Rate: 10, Burst: 5, Interval: time.Second})
	r.Configure("/auth", Config{Rate: 3, Burst: 3, Interval: time.Second})
	r.Configure("/admin", Config{Rate: 5, Burst: 5, Interval: time.Second})

	if r.ConfigCount() != 2 {
		t.Errorf("config count: got %d", r.ConfigCount())
	}
}

func TestRegistry_Defaults(t *testing.T) {
	r := NewRegistry(Config{}) // Should use defaults.
	if !r.Allow("/test", "ip") {
		t.Error("should allow with default config")
	}
}
