package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDeprecated_AddsAllThreeHeaders(t *testing.T) {
	sunset := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	link := "https://api.garudapass.com/v2/docs"

	handler := Deprecated(sunset, link)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Deprecation"); got != "true" {
		t.Errorf("Deprecation header: got %q, want %q", got, "true")
	}

	if got := rec.Header().Get("Sunset"); got == "" {
		t.Error("Sunset header should be set")
	}

	if got := rec.Header().Get("Link"); got == "" {
		t.Error("Link header should be set")
	}
}

func TestDeprecated_SunsetHeaderHTTPDateFormat(t *testing.T) {
	sunset := time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC)
	link := "https://api.example.com/v2"

	handler := Deprecated(sunset, link)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Sunset")
	// HTTP date format per RFC 7231: "Mon, 15 Jun 2026 14:30:00 GMT"
	want := "Mon, 15 Jun 2026 14:30:00 GMT"
	if got != want {
		t.Errorf("Sunset header: got %q, want %q", got, want)
	}
}

func TestDeprecated_LinkHeaderFormat(t *testing.T) {
	sunset := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	link := "https://api.garudapass.com/v2/docs"

	handler := Deprecated(sunset, link)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Link")
	want := `<https://api.garudapass.com/v2/docs>; rel="successor-version"`
	if got != want {
		t.Errorf("Link header: got %q, want %q", got, want)
	}
}

func TestDeprecated_HandlerStillExecutes(t *testing.T) {
	handlerCalled := false
	sunset := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	handler := Deprecated(sunset, "https://example.com/v2")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("still working"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("wrapped handler should still be called")
	}

	if rec.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestDeprecated_ResponseBodyUnchanged(t *testing.T) {
	sunset := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	expectedBody := `{"data":"hello","status":"ok"}`

	handler := Deprecated(sunset, "https://example.com/v2")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expectedBody))
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Body.String(); got != expectedBody {
		t.Errorf("response body: got %q, want %q", got, expectedBody)
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q, want %q", got, "application/json")
	}
}

func TestDeprecated_WorksWithPOST(t *testing.T) {
	sunset := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)

	handler := Deprecated(sunset, "https://example.com/v2")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("POST status: got %d, want %d", rec.Code, http.StatusCreated)
	}

	if got := rec.Header().Get("Deprecation"); got != "true" {
		t.Errorf("Deprecation header on POST: got %q, want %q", got, "true")
	}
}

func TestDeprecated_SunsetConvertsToUTC(t *testing.T) {
	// Use a non-UTC timezone to verify it's converted.
	loc := time.FixedZone("WIB", 7*60*60)
	sunset := time.Date(2026, 6, 15, 21, 30, 0, 0, loc) // 21:30 WIB = 14:30 UTC

	handler := Deprecated(sunset, "https://example.com/v2")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resource", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Sunset")
	want := "Mon, 15 Jun 2026 14:30:00 GMT"
	if got != want {
		t.Errorf("Sunset header (UTC conversion): got %q, want %q", got, want)
	}
}
