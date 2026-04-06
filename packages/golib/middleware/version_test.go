package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIVersion_DefaultVersionNoAcceptHeader(t *testing.T) {
	var ctxVersion string
	handler := APIVersion("v1", []string{"v1", "v2"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxVersion = GetAPIVersion(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ctxVersion != "v1" {
		t.Errorf("expected v1, got %q", ctxVersion)
	}
	if v := w.Header().Get("X-API-Version"); v != "v1" {
		t.Errorf("expected X-API-Version v1, got %q", v)
	}
}

func TestAPIVersion_ExtractV1FromAcceptHeader(t *testing.T) {
	var ctxVersion string
	handler := APIVersion("v1", []string{"v1", "v2"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxVersion = GetAPIVersion(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/vnd.garudapass.v1+json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ctxVersion != "v1" {
		t.Errorf("expected v1, got %q", ctxVersion)
	}
}

func TestAPIVersion_ExtractV2FromAcceptHeader(t *testing.T) {
	var ctxVersion string
	handler := APIVersion("v1", []string{"v1", "v2"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxVersion = GetAPIVersion(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/vnd.garudapass.v2+json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ctxVersion != "v2" {
		t.Errorf("expected v2, got %q", ctxVersion)
	}
}

func TestAPIVersion_UnsupportedVersionReturns406(t *testing.T) {
	handler := APIVersion("v1", []string{"v1", "v2"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/vnd.garudapass.v99+json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusNotAcceptable {
		t.Errorf("expected 406, got %d", w.Code)
	}
}

func TestAPIVersion_VersionAvailableInContext(t *testing.T) {
	var ctxVersion string
	handler := APIVersion("v1", []string{"v1"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxVersion = GetAPIVersion(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/vnd.garudapass.v1+json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if ctxVersion != "v1" {
		t.Errorf("expected v1 in context, got %q", ctxVersion)
	}
}

func TestAPIVersion_StandardJSONUsesDefault(t *testing.T) {
	var ctxVersion string
	handler := APIVersion("v1", []string{"v1", "v2"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxVersion = GetAPIVersion(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ctxVersion != "v1" {
		t.Errorf("expected default v1, got %q", ctxVersion)
	}
}

func TestParseAcceptVersion_ValidFormats(t *testing.T) {
	tests := []struct {
		accept  string
		want    string
	}{
		{"application/vnd.garudapass.v1+json", "v1"},
		{"application/vnd.garudapass.v2+json", "v2"},
		{"application/vnd.garudapass.v10+json", "v10"},
		{"application/json", ""},
		{"text/html", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := ParseAcceptVersion(tt.accept)
		if got != tt.want {
			t.Errorf("ParseAcceptVersion(%q) = %q, want %q", tt.accept, got, tt.want)
		}
	}
}

func TestGetAPIVersion_NilContext(t *testing.T) {
	if v := GetAPIVersion(nil); v != "" {
		t.Errorf("expected empty string for nil context, got %q", v)
	}
}
