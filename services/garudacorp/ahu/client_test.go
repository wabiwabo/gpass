package ahu

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchCompany_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/company/search" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("missing or wrong API key")
		}

		var req CompanySearchRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.SKNumber != "AHU-12345" {
			t.Errorf("SKNumber = %q, want %q", req.SKNumber, "AHU-12345")
		}

		resp := CompanySearchResponse{
			Found:       true,
			SKNumber:    "AHU-12345",
			Name:        "PT Test Corp",
			EntityType:  "PT",
			Status:      "ACTIVE",
			NPWP:        "01.234.567.8-901.000",
			Address:     "Jakarta",
			CapitalAuth: 1000000000,
			CapitalPaid: 500000000,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-key", 5*time.Second)
	resp, err := client.SearchCompany(context.Background(), "AHU-12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Found {
		t.Error("expected Found to be true")
	}
	if resp.Name != "PT Test Corp" {
		t.Errorf("Name = %q, want %q", resp.Name, "PT Test Corp")
	}
}

func TestSearchCompany_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CompanySearchResponse{Found: false, Message: "not found"})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", 5*time.Second)
	resp, err := client.SearchCompany(context.Background(), "AHU-99999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Found {
		t.Error("expected Found to be false")
	}
}

func TestGetOfficers_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/company/AHU-12345/officers" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}

		resp := OfficersResponse{
			SKNumber: "AHU-12345",
			Officers: []Officer{
				{NIK: "3201010101010001", Name: "John Doe", Position: "DIREKTUR_UTAMA", AppointmentDate: "2020-01-01"},
				{NIK: "3201010101010002", Name: "Jane Doe", Position: "KOMISARIS", AppointmentDate: "2020-01-01"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", 5*time.Second)
	officers, err := client.GetOfficers(context.Background(), "AHU-12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(officers) != 2 {
		t.Fatalf("expected 2 officers, got %d", len(officers))
	}
	if officers[0].Position != "DIREKTUR_UTAMA" {
		t.Errorf("Position = %q, want %q", officers[0].Position, "DIREKTUR_UTAMA")
	}
}

func TestGetShareholders_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/company/AHU-12345/shareholders" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		resp := ShareholdersResponse{
			SKNumber: "AHU-12345",
			Shareholders: []Shareholder{
				{Name: "John Doe", ShareType: "INDIVIDUAL", Shares: 500, Percentage: 50.0},
				{Name: "PT Holding", ShareType: "CORPORATE", Shares: 500, Percentage: 50.0},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", 5*time.Second)
	shareholders, err := client.GetShareholders(context.Background(), "AHU-12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shareholders) != 2 {
		t.Fatalf("expected 2 shareholders, got %d", len(shareholders))
	}
}

func TestClient_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", 5*time.Second)
	_, err := client.SearchCompany(context.Background(), "AHU-12345")
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestClient_CircuitBreaker(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "", 5*time.Second)

	// Trigger 5 failures to open circuit breaker
	for i := 0; i < 5; i++ {
		client.SearchCompany(context.Background(), "AHU-12345")
	}

	// Next call should fail with circuit breaker open
	_, err := client.SearchCompany(context.Background(), "AHU-12345")
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	if callCount != 5 {
		t.Errorf("expected 5 server calls, got %d", callCount)
	}
}
