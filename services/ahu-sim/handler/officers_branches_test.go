package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestGetOfficers_RejectionMatrix pins the empty-sk and not-found
// branches via Go 1.22+ method-routed mux so PathValue works.
func TestGetOfficers_RejectionMatrix(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ahu/company/{sk}/officers", GetOfficers)

	// Empty sk: hit a sibling pattern that produces an empty value.
	// Under method routing, /api/v1/ahu/company//officers would not
	// match {sk}, so test the not-found branch instead which exercises
	// the same handler past the empty guard.
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/ahu/company/AHU-NONEXISTENT/officers", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("not-found: %d", rec.Code)
	}
}

// TestGetOfficers_EmptySKDirect pins the empty-sk branch by calling the
// handler directly without registering it on a mux.
func TestGetOfficers_EmptySKDirect(t *testing.T) {
	rec := httptest.NewRecorder()
	GetOfficers(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty sk: %d, want 400", rec.Code)
	}
}

// TestGetShareholders_RejectionMatrix mirrors GetOfficers.
func TestGetShareholders_RejectionMatrix(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ahu/company/{sk}/shareholders", GetShareholders)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/ahu/company/AHU-NONEXISTENT/shareholders", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("not-found: %d", rec.Code)
	}
}

func TestGetShareholders_EmptySKDirect(t *testing.T) {
	rec := httptest.NewRecorder()
	GetShareholders(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("empty sk: %d, want 400", rec.Code)
	}
}
