package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPAllowlist_AllowedIP(t *testing.T) {
	mw, err := IPAllowlist([]string{"192.168.1.0/24"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestIPAllowlist_BlockedIP(t *testing.T) {
	mw, err := IPAllowlist([]string{"192.168.1.0/24"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["message"] != "IP address not allowed" {
		t.Errorf("unexpected message: %v", body["message"])
	}
}

func TestIPDenylist_DeniedIP(t *testing.T) {
	mw, err := IPDenylist([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "10.5.3.2:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestIPDenylist_AllowedIP(t *testing.T) {
	mw, err := IPDenylist([]string{"10.0.0.0/8"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestIPFilter_CIDRRange(t *testing.T) {
	mw, err := IPAllowlist([]string{"192.168.1.0/24"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// All IPs in 192.168.1.0/24 should match.
	ips := []string{"192.168.1.0", "192.168.1.1", "192.168.1.128", "192.168.1.255"}
	for _, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip + ":12345"
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("IP %s: expected 200, got %d", ip, rec.Code)
		}
	}

	// IP outside the range should be blocked.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.2.1:12345"
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("IP 192.168.2.1: expected 403, got %d", rec.Code)
	}
}

func TestIPFilter_XForwardedFor(t *testing.T) {
	mw, err := IPAllowlist([]string{"203.0.113.0/24"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.1")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with X-Forwarded-For, got %d", rec.Code)
	}
}

func TestIPFilter_InvalidCIDR(t *testing.T) {
	_, err := IPAllowlist([]string{"not-a-cidr"})
	if err == nil {
		t.Fatal("expected error for invalid CIDR, got nil")
	}
}

func TestIPFilter_InvalidMode(t *testing.T) {
	_, err := IPFilter("block", []string{"10.0.0.0/8"})
	if err == nil {
		t.Fatal("expected error for invalid mode, got nil")
	}
}

func TestIPFilter_PrivateNetworkRanges(t *testing.T) {
	// Denylist all private network ranges.
	mw, err := IPDenylist([]string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		ip     string
		status int
	}{
		{"10.x blocked", "10.1.2.3", http.StatusForbidden},
		{"172.16.x blocked", "172.16.5.10", http.StatusForbidden},
		{"172.31.x blocked", "172.31.255.255", http.StatusForbidden},
		{"192.168.x blocked", "192.168.0.1", http.StatusForbidden},
		{"public IP allowed", "8.8.8.8", http.StatusOK},
		{"172.15.x allowed", "172.15.0.1", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.ip + ":12345"
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.status {
				t.Errorf("IP %s: expected %d, got %d", tt.ip, tt.status, rec.Code)
			}
		})
	}
}

func TestIPFilter_XRealIP(t *testing.T) {
	mw, err := IPAllowlist([]string{"203.0.113.0/24"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Real-IP", "203.0.113.10")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 with X-Real-IP, got %d", rec.Code)
	}
}
