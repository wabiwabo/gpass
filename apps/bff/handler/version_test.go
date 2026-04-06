package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

func TestVersionHandler_ReturnsAllFields(t *testing.T) {
	info := DefaultVersionInfo("1.0.0", "abc123def", "2026-04-06T00:00:00Z", "production")
	h := NewVersionHandler(info)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp VersionInfo
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Platform == "" {
		t.Error("Platform should not be empty")
	}
	if resp.Version == "" {
		t.Error("Version should not be empty")
	}
	if resp.APIVersion == "" {
		t.Error("APIVersion should not be empty")
	}
	if resp.GoVersion == "" {
		t.Error("GoVersion should not be empty")
	}
	if resp.Commit == "" {
		t.Error("Commit should not be empty")
	}
	if resp.BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}
	if resp.Services == 0 {
		t.Error("Services should not be zero")
	}
	if resp.Environment == "" {
		t.Error("Environment should not be empty")
	}
}

func TestVersionHandler_ContentType(t *testing.T) {
	info := DefaultVersionInfo("1.0.0", "abc123", "2026-04-06T00:00:00Z", "staging")
	h := NewVersionHandler(info)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
}

func TestVersionHandler_PlatformIsGarudaPass(t *testing.T) {
	info := DefaultVersionInfo("1.0.0", "abc123", "2026-04-06T00:00:00Z", "production")
	h := NewVersionHandler(info)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp VersionInfo
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Platform != "GarudaPass" {
		t.Errorf("Platform = %q, want GarudaPass", resp.Platform)
	}
}

func TestVersionHandler_ServicesCount(t *testing.T) {
	info := DefaultVersionInfo("1.0.0", "abc123", "2026-04-06T00:00:00Z", "production")
	h := NewVersionHandler(info)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp VersionInfo
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Services != 12 {
		t.Errorf("Services = %d, want 12", resp.Services)
	}
}

func TestVersionHandler_GoVersionMatchesRuntime(t *testing.T) {
	info := DefaultVersionInfo("1.0.0", "abc123", "2026-04-06T00:00:00Z", "production")
	h := NewVersionHandler(info)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp VersionInfo
	json.NewDecoder(w.Body).Decode(&resp)

	expected := runtime.Version()
	if resp.GoVersion != expected {
		t.Errorf("GoVersion = %q, want %q", resp.GoVersion, expected)
	}
}

func TestVersionHandler_APIVersionIsV1(t *testing.T) {
	info := DefaultVersionInfo("2.5.0", "def456", "2026-04-06T12:00:00Z", "staging")
	h := NewVersionHandler(info)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp VersionInfo
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.APIVersion != "v1" {
		t.Errorf("APIVersion = %q, want v1", resp.APIVersion)
	}
}

func TestVersionHandler_VersionAndCommitPreserved(t *testing.T) {
	info := DefaultVersionInfo("3.14.159", "deadbeef", "2026-04-06T08:30:00Z", "development")
	h := NewVersionHandler(info)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	var resp VersionInfo
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Version != "3.14.159" {
		t.Errorf("Version = %q, want 3.14.159", resp.Version)
	}
	if resp.Commit != "deadbeef" {
		t.Errorf("Commit = %q, want deadbeef", resp.Commit)
	}
	if resp.BuildTime != "2026-04-06T08:30:00Z" {
		t.Errorf("BuildTime = %q, want 2026-04-06T08:30:00Z", resp.BuildTime)
	}
	if resp.Environment != "development" {
		t.Errorf("Environment = %q, want development", resp.Environment)
	}
}

func TestVersionHandler_CacheControl(t *testing.T) {
	info := DefaultVersionInfo("1.0.0", "abc123", "2026-04-06T00:00:00Z", "production")
	h := NewVersionHandler(info)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/version", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	cc := w.Header().Get("Cache-Control")
	if cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
}

func TestDefaultVersionInfo_Defaults(t *testing.T) {
	info := DefaultVersionInfo("0.0.1", "aaa", "now", "test")

	if info.Platform != "GarudaPass" {
		t.Errorf("Platform = %q, want GarudaPass", info.Platform)
	}
	if info.Services != 12 {
		t.Errorf("Services = %d, want 12", info.Services)
	}
	if info.APIVersion != "v1" {
		t.Errorf("APIVersion = %q, want v1", info.APIVersion)
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("GoVersion = %q, want %q", info.GoVersion, runtime.Version())
	}
}
