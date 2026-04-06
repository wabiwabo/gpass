package oauth2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestDefaultGarudaPassDiscovery_FAPI2Required(t *testing.T) {
	cfg := DefaultGarudaPassDiscovery("https://id.garudapass.com/realms/garudapass")

	if cfg.Issuer != "https://id.garudapass.com/realms/garudapass" {
		t.Errorf("Issuer = %q, want %q", cfg.Issuer, "https://id.garudapass.com/realms/garudapass")
	}

	// FAPI 2.0: PAR required.
	if !cfg.RequirePushedAuthorizationRequests {
		t.Error("RequirePushedAuthorizationRequests = false, want true (FAPI 2.0)")
	}

	// FAPI 2.0: PKCE S256.
	if !slices.Contains(cfg.CodeChallengeMethodsSupported, "S256") {
		t.Errorf("CodeChallengeMethodsSupported = %v, want to contain S256", cfg.CodeChallengeMethodsSupported)
	}

	// FAPI 2.0: DPoP support.
	if len(cfg.DPoPSigningAlgValues) == 0 {
		t.Error("DPoPSigningAlgValues is empty, want non-empty for FAPI 2.0")
	}

	// FAPI 2.0: response_types must include "code".
	if !slices.Contains(cfg.ResponseTypesSupported, "code") {
		t.Errorf("ResponseTypesSupported = %v, want to contain 'code'", cfg.ResponseTypesSupported)
	}

	// FAPI 2.0: only strong signing algorithms.
	for _, alg := range cfg.IDTokenSigningAlgValues {
		if alg == "RS256" || alg == "none" {
			t.Errorf("IDTokenSigningAlgValues contains weak algorithm %q", alg)
		}
	}
}

func TestDefaultGarudaPassDiscovery_ScopesSupported(t *testing.T) {
	cfg := DefaultGarudaPassDiscovery("https://id.garudapass.com")

	required := []string{"openid", "profile", "email"}
	for _, s := range required {
		if !slices.Contains(cfg.ScopesSupported, s) {
			t.Errorf("ScopesSupported = %v, want to contain %q", cfg.ScopesSupported, s)
		}
	}
}

func TestDefaultGarudaPassDiscovery_TrailingSlashNormalized(t *testing.T) {
	cfg := DefaultGarudaPassDiscovery("https://id.garudapass.com/")

	if cfg.Issuer != "https://id.garudapass.com" {
		t.Errorf("Issuer = %q, trailing slash not trimmed", cfg.Issuer)
	}
}

func TestDiscoveryHandler_ReturnsCorrectJSON(t *testing.T) {
	cfg := DefaultGarudaPassDiscovery("https://id.garudapass.com")
	handler := DiscoveryHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got DiscoveryConfig
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Issuer != cfg.Issuer {
		t.Errorf("Issuer = %q, want %q", got.Issuer, cfg.Issuer)
	}
	if got.RequirePushedAuthorizationRequests != cfg.RequirePushedAuthorizationRequests {
		t.Error("RequirePushedAuthorizationRequests mismatch")
	}
}

func TestDiscoveryHandler_ContentType(t *testing.T) {
	cfg := DefaultGarudaPassDiscovery("https://id.garudapass.com")
	handler := DiscoveryHandler(cfg)

	req := httptest.NewRequest(http.MethodGet, "/.well-known/openid-configuration", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestFetchDiscovery_MockServer(t *testing.T) {
	expected := DefaultGarudaPassDiscovery("https://id.garudapass.com")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			t.Errorf("path = %q, want %q", r.URL.Path, "/.well-known/openid-configuration")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	got, err := FetchDiscovery(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("FetchDiscovery() error = %v", err)
	}

	if got.Issuer != expected.Issuer {
		t.Errorf("Issuer = %q, want %q", got.Issuer, expected.Issuer)
	}
	if got.RequirePushedAuthorizationRequests != expected.RequirePushedAuthorizationRequests {
		t.Error("RequirePushedAuthorizationRequests mismatch")
	}
	if !slices.Equal(got.DPoPSigningAlgValues, expected.DPoPSigningAlgValues) {
		t.Errorf("DPoPSigningAlgValues = %v, want %v", got.DPoPSigningAlgValues, expected.DPoPSigningAlgValues)
	}
}

func TestFetchDiscovery_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := FetchDiscovery(context.Background(), server.URL)
	if err == nil {
		t.Fatal("FetchDiscovery() error = nil, want error for server error")
	}
}
