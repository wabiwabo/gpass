package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func postJSON(t *testing.T, handler http.HandlerFunc, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

func TestSearchCompany_BySK(t *testing.T) {
	rec := postJSON(t, SearchCompany, SearchRequest{SKNumber: "AHU-0001234.AH.01.01"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Companies) != 1 {
		t.Fatalf("expected 1 company, got %d", len(resp.Companies))
	}
	if resp.Companies[0].Name != "PT GARUDA TEKNOLOGI INDONESIA" {
		t.Errorf("expected PT GARUDA TEKNOLOGI INDONESIA, got %s", resp.Companies[0].Name)
	}
	if resp.Companies[0].EntityType != "PT" {
		t.Errorf("expected entity type PT, got %s", resp.Companies[0].EntityType)
	}
}

func TestSearchCompany_ByName(t *testing.T) {
	rec := postJSON(t, SearchCompany, SearchRequest{Name: "GARUDA"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Companies) < 1 {
		t.Fatal("expected at least 1 company matching 'GARUDA'")
	}

	found := false
	for _, c := range resp.Companies {
		if c.SKNumber == "AHU-0001234.AH.01.01" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find PT GARUDA TEKNOLOGI INDONESIA in results")
	}
}

func TestSearchCompany_NotFound(t *testing.T) {
	rec := postJSON(t, SearchCompany, SearchRequest{SKNumber: "AHU-9999999.AH.01.01"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Companies) != 0 {
		t.Errorf("expected 0 companies, got %d", len(resp.Companies))
	}
}

func TestSearchCompany_MissingParams(t *testing.T) {
	rec := postJSON(t, SearchCompany, SearchRequest{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetOfficers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company/AHU-0001234.AH.01.01/officers", nil)
	req.SetPathValue("sk", "AHU-0001234.AH.01.01")
	rec := httptest.NewRecorder()
	GetOfficers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp OfficersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Officers) != 2 {
		t.Fatalf("expected 2 officers, got %d", len(resp.Officers))
	}

	// Verify Budi is Direktur Utama
	found := false
	for _, o := range resp.Officers {
		if o.NIK == "3201011501900001" && o.Position == "DIREKTUR_UTAMA" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected BUDI SANTOSO as DIREKTUR_UTAMA")
	}
}

func TestGetOfficers_NotFound(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company/AHU-9999999.AH.01.01/officers", nil)
	req.SetPathValue("sk", "AHU-9999999.AH.01.01")
	rec := httptest.NewRecorder()
	GetOfficers(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetShareholders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company/AHU-0001234.AH.01.01/shareholders", nil)
	req.SetPathValue("sk", "AHU-0001234.AH.01.01")
	rec := httptest.NewRecorder()
	GetShareholders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp ShareholdersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Shareholders) != 2 {
		t.Fatalf("expected 2 shareholders, got %d", len(resp.Shareholders))
	}

	// Verify total percentage is 100%
	var total float64
	for _, s := range resp.Shareholders {
		total += s.Percentage
	}
	if total != 100.0 {
		t.Errorf("expected total percentage 100.0, got %f", total)
	}
}

func TestGetShareholders_Yayasan(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company/AHU-0004567.AH.01.01/shareholders", nil)
	req.SetPathValue("sk", "AHU-0004567.AH.01.01")
	rec := httptest.NewRecorder()
	GetShareholders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp ShareholdersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Shareholders) != 0 {
		t.Errorf("expected 0 shareholders for yayasan, got %d", len(resp.Shareholders))
	}
}
