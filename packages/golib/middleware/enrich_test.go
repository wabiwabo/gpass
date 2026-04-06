package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnrich_ExtractsClientInfo(t *testing.T) {
	var got ClientInfo
	handler := Enrich(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info, ok := ClientInfoFromContext(r.Context())
		if !ok {
			t.Fatal("expected ClientInfo in context")
		}
		got = info
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept-Language", "id-ID,id;q=0.9,en;q=0.8")
	req.Header.Set("Referer", "https://garudapass.id/dashboard")
	req.Header.Set("X-Forwarded-For", "103.28.12.5, 10.0.0.1")
	req.Header.Set("CF-IPCountry", "ID")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got.IP != "103.28.12.5" {
		t.Errorf("IP: got %q, want %q", got.IP, "103.28.12.5")
	}
	if got.Country != "ID" {
		t.Errorf("Country: got %q, want %q", got.Country, "ID")
	}
	if got.DeviceType != "desktop" {
		t.Errorf("DeviceType: got %q, want %q", got.DeviceType, "desktop")
	}
	if got.AcceptLanguage != "id-ID,id;q=0.9,en;q=0.8" {
		t.Errorf("AcceptLanguage: got %q", got.AcceptLanguage)
	}
	if got.Referer != "https://garudapass.id/dashboard" {
		t.Errorf("Referer: got %q", got.Referer)
	}
}

func TestEnrich_MobileDetection(t *testing.T) {
	tests := []struct {
		ua       string
		expected string
	}{
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X)", "mobile"},
		{"Mozilla/5.0 (Linux; Android 12; Pixel 6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96 Mobile Safari", "mobile"},
		{"Mozilla/5.0 (iPad; CPU OS 15_0 like Mac OS X)", "tablet"},
		{"Mozilla/5.0 (Linux; Android 12; SM-T870) AppleWebKit/537.36", "tablet"},
		{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", "desktop"},
		{"Googlebot/2.1 (+http://www.google.com/bot.html)", "bot"},
		{"curl/7.68.0", "bot"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected+"/"+tt.ua[:min(len(tt.ua), 30)], func(t *testing.T) {
			got := detectDevice(tt.ua)
			if got != tt.expected {
				t.Errorf("detectDevice(%q) = %q, want %q", tt.ua, got, tt.expected)
			}
		})
	}
}

func TestEnrich_IPExtraction_XRealIP(t *testing.T) {
	var got ClientInfo
	handler := Enrich(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = ClientInfoFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Real-IP", "192.168.1.100")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got.IP != "192.168.1.100" {
		t.Errorf("IP from X-Real-IP: got %q", got.IP)
	}
}

func TestEnrich_IPExtraction_RemoteAddr(t *testing.T) {
	var got ClientInfo
	handler := Enrich(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = ClientInfoFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// httptest sets RemoteAddr to 192.0.2.1:1234.
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got.IP != "192.0.2.1" {
		t.Errorf("IP from RemoteAddr: got %q", got.IP)
	}
}

func TestEnrich_CountryFromXCountry(t *testing.T) {
	var got ClientInfo
	handler := Enrich(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = ClientInfoFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Country", "sg")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got.Country != "SG" {
		t.Errorf("Country: got %q, want %q", got.Country, "SG")
	}
}

func TestEnrich_NoCountry(t *testing.T) {
	var got ClientInfo
	handler := Enrich(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = ClientInfoFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got.Country != "" {
		t.Errorf("Country should be empty, got %q", got.Country)
	}
}

func TestClientInfoFromContext_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	_, ok := ClientInfoFromContext(req.Context())
	if ok {
		t.Error("should return false when no ClientInfo in context")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
