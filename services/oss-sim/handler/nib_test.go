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

func TestSearchNIB_ByNPWP(t *testing.T) {
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
	if resp.Businesses[0].NIB != "1234500001234" {
		t.Errorf("expected NIB 1234500001234, got %s", resp.Businesses[0].NIB)
	}
	if resp.Businesses[0].CompanyName != "PT GARUDA TEKNOLOGI INDONESIA" {
		t.Errorf("expected PT GARUDA TEKNOLOGI INDONESIA, got %s", resp.Businesses[0].CompanyName)
	}
	if resp.Businesses[0].Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", resp.Businesses[0].Status)
	}
}

func TestSearchNIB_ByNIB(t *testing.T) {
	rec := postJSON(t, SearchNIB, SearchRequest{NIB: "3456700003456"})

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
	if resp.Businesses[0].CompanyName != "PT BALI SEJAHTERA" {
		t.Errorf("expected PT BALI SEJAHTERA, got %s", resp.Businesses[0].CompanyName)
	}
	if resp.Businesses[0].KBLI != "55111" {
		t.Errorf("expected KBLI 55111, got %s", resp.Businesses[0].KBLI)
	}
}

func TestSearchNIB_NotFound(t *testing.T) {
	rec := postJSON(t, SearchNIB, SearchRequest{NIB: "9999999999999"})

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

func TestSearchNIB_MissingParams(t *testing.T) {
	rec := postJSON(t, SearchNIB, SearchRequest{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSearchNIB_RevokedBusiness(t *testing.T) {
	rec := postJSON(t, SearchNIB, SearchRequest{NPWP: "05.678.901.2-056.000"})

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
	if resp.Businesses[0].Status != "REVOKED" {
		t.Errorf("expected status REVOKED, got %s", resp.Businesses[0].Status)
	}
}
