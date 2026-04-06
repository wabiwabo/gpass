package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchNIB_InvalidNIBFormat(t *testing.T) {
	rec := postJSON(t, SearchNIB, SearchRequest{NIB: "ABC"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Businesses) != 0 {
		t.Errorf("expected 0 businesses for invalid NIB, got %d", len(resp.Businesses))
	}
}

func TestSearchNIB_NonExistentNIB(t *testing.T) {
	rec := postJSON(t, SearchNIB, SearchRequest{NIB: "0000000000000"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Businesses) != 0 {
		t.Errorf("expected 0 businesses, got %d", len(resp.Businesses))
	}
}

func TestSearchNIB_NPWPCrossReference(t *testing.T) {
	// Search by NPWP that is shared with AHU simulator data
	rec := postJSON(t, SearchNIB, SearchRequest{NPWP: "01.234.567.8-012.000"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Businesses) != 1 {
		t.Fatalf("expected 1 business, got %d", len(resp.Businesses))
	}

	b := resp.Businesses[0]
	// Verify the NPWP cross-references correctly
	if b.NPWP != "01.234.567.8-012.000" {
		t.Errorf("expected NPWP 01.234.567.8-012.000, got %s", b.NPWP)
	}
	if b.CompanyName != "PT GARUDA TEKNOLOGI INDONESIA" {
		t.Errorf("expected company PT GARUDA TEKNOLOGI INDONESIA, got %s", b.CompanyName)
	}
}

func TestSearchNIB_RequiredFieldsPresent(t *testing.T) {
	rec := postJSON(t, SearchNIB, SearchRequest{NIB: "1234500001234"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Decode into raw map to check all fields
	var raw map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	businesses, ok := raw["businesses"].([]interface{})
	if !ok || len(businesses) == 0 {
		t.Fatal("expected businesses array with at least 1 entry")
	}

	biz, ok := businesses[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected business to be a JSON object")
	}

	requiredFields := []string{"nib", "company_name", "status", "npwp", "kbli", "kbli_desc", "issued_date", "address"}
	for _, field := range requiredFields {
		val, exists := biz[field]
		if !exists {
			t.Errorf("response missing required field %q", field)
			continue
		}
		str, ok := val.(string)
		if !ok || str == "" {
			t.Errorf("field %q should be a non-empty string, got %v", field, val)
		}
	}
}

func TestSearchNIB_ContentTypeHeader(t *testing.T) {
	rec := postJSON(t, SearchNIB, SearchRequest{NIB: "1234500001234"})

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

func TestSearchNIB_SearchByNPWP_Multiple(t *testing.T) {
	// Each NPWP should correspond to exactly one business
	npwps := []struct {
		npwp string
		name string
	}{
		{"01.234.567.8-012.000", "PT GARUDA TEKNOLOGI INDONESIA"},
		{"02.345.678.9-023.000", "CV NUSANTARA DIGITAL"},
		{"03.456.789.0-034.000", "PT BALI SEJAHTERA"},
	}

	for _, tc := range npwps {
		t.Run(tc.name, func(t *testing.T) {
			rec := postJSON(t, SearchNIB, SearchRequest{NPWP: tc.npwp})

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", rec.Code)
			}

			var resp SearchResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if len(resp.Businesses) != 1 {
				t.Fatalf("expected 1 business for NPWP %s, got %d", tc.npwp, len(resp.Businesses))
			}

			if resp.Businesses[0].CompanyName != tc.name {
				t.Errorf("expected %s, got %s", tc.name, resp.Businesses[0].CompanyName)
			}
		})
	}
}

func TestSearchNIB_InvalidJSONBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	SearchNIB(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rec.Code)
	}
}

func TestSearchNIB_NonExistentNPWP(t *testing.T) {
	rec := postJSON(t, SearchNIB, SearchRequest{NPWP: "99.999.999.9-999.000"})

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Businesses) != 0 {
		t.Errorf("expected 0 businesses for non-existent NPWP, got %d", len(resp.Businesses))
	}
}
