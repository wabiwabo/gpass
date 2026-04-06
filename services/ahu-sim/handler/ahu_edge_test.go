package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchCompany_InvalidSKFormat(t *testing.T) {
	rec := postJSON(t, SearchCompany, SearchRequest{SKNumber: "INVALID-FORMAT"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Companies) != 0 {
		t.Errorf("expected 0 companies for invalid SK format, got %d", len(resp.Companies))
	}
}

func TestSearchCompany_NonExistentEntity(t *testing.T) {
	rec := postJSON(t, SearchCompany, SearchRequest{SKNumber: "AHU-0000000.AH.01.01"})

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

func TestSearchCompany_EntityTypeValidation(t *testing.T) {
	tests := []struct {
		name       string
		sk         string
		entityType string
	}{
		{"PT entity", "AHU-0001234.AH.01.01", "PT"},
		{"CV entity", "AHU-0002345.AH.01.01", "CV"},
		{"Yayasan entity", "AHU-0004567.AH.01.01", "YAYASAN"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSON(t, SearchCompany, SearchRequest{SKNumber: tc.sk})

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

			if resp.Companies[0].EntityType != tc.entityType {
				t.Errorf("expected entity type %s, got %s", tc.entityType, resp.Companies[0].EntityType)
			}
		})
	}
}

func TestGetOfficers_AndShareholders_Included(t *testing.T) {
	// Test officers
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company/AHU-0006789.AH.01.01/officers", nil)
	req.SetPathValue("sk", "AHU-0006789.AH.01.01")
	rec := httptest.NewRecorder()
	GetOfficers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var officersResp OfficersResponse
	if err := json.NewDecoder(rec.Body).Decode(&officersResp); err != nil {
		t.Fatalf("failed to decode officers response: %v", err)
	}

	if len(officersResp.Officers) != 3 {
		t.Fatalf("expected 3 officers for PT DIGITAL NUSANTARA PRIMA, got %d", len(officersResp.Officers))
	}

	// Verify each officer has required fields
	for _, o := range officersResp.Officers {
		if o.NIK == "" {
			t.Error("officer NIK should not be empty")
		}
		if o.Name == "" {
			t.Error("officer Name should not be empty")
		}
		if o.Position == "" {
			t.Error("officer Position should not be empty")
		}
	}

	// Test shareholders
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company/AHU-0006789.AH.01.01/shareholders", nil)
	req2.SetPathValue("sk", "AHU-0006789.AH.01.01")
	rec2 := httptest.NewRecorder()
	GetShareholders(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec2.Code)
	}

	var shResp ShareholdersResponse
	if err := json.NewDecoder(rec2.Body).Decode(&shResp); err != nil {
		t.Fatalf("failed to decode shareholders response: %v", err)
	}

	if len(shResp.Shareholders) != 3 {
		t.Fatalf("expected 3 shareholders, got %d", len(shResp.Shareholders))
	}

	var totalPct float64
	for _, s := range shResp.Shareholders {
		totalPct += s.Percentage
	}
	if totalPct != 100.0 {
		t.Errorf("expected total percentage 100.0, got %f", totalPct)
	}
}

func TestSearchCompany_EmptyQuery(t *testing.T) {
	rec := postJSON(t, SearchCompany, SearchRequest{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty query, got %d", rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if resp.Error != "bad_request" {
		t.Errorf("expected error 'bad_request', got %q", resp.Error)
	}
}

func TestSearchCompany_ContentTypeHeader(t *testing.T) {
	rec := postJSON(t, SearchCompany, SearchRequest{SKNumber: "AHU-0001234.AH.01.01"})

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type 'application/json; charset=utf-8', got %q", ct)
	}

	xct := rec.Header().Get("X-Content-Type-Options")
	if xct != "nosniff" {
		t.Errorf("expected X-Content-Type-Options 'nosniff', got %q", xct)
	}

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-store, no-cache, must-revalidate, private" {
		t.Errorf("expected Cache-Control 'no-store, no-cache, must-revalidate, private', got %q", cc)
	}
}

func TestSearchCompany_InvalidJSONBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	SearchCompany(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestSearchCompany_DissolvedEntity(t *testing.T) {
	rec := postJSON(t, SearchCompany, SearchRequest{SKNumber: "AHU-0005678.AH.01.01"})

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

	if resp.Companies[0].Status != "DISSOLVED" {
		t.Errorf("expected status DISSOLVED, got %s", resp.Companies[0].Status)
	}
}
