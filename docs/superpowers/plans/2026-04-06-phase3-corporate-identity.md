# Phase 3: Corporate Identity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build corporate identity features — AHU and OSS simulators for legal entity data, GarudaCorp service for corporate registration with NIK-based officer matching, 3-level role hierarchy, and entity profile management.

**Architecture:** Three new Go services in `services/` — ahu-sim (AHU legal entity simulator, port 4004), oss-sim (OSS business license simulator, port 4005), garudacorp (corporate registration + role management + entity profiles, port 4006). AHU provides legal entity verification; OSS enriches with NIB/KBLI data non-blocking. Officer NIKs are tokenized with the same HMAC key as Identity Service for cross-service matching.

**Tech Stack:** Go 1.22+, stdlib `net/http` with typed routing, `log/slog`, `crypto/hmac` for NIK tokenization, `net/http/httptest` for tests, circuit breaker pattern (same as dukcapil client)

---

## File Structure

```
services/
├── ahu-sim/                                   # AHU API simulator
│   ├── go.mod
│   ├── main.go
│   ├── Dockerfile
│   ├── data/
│   │   └── testdata.go                        # Synthetic company data (10+ companies)
│   └── handler/
│       ├── company.go                         # Company search + officers + shareholders
│       └── company_test.go
│
├── oss-sim/                                   # OSS API simulator
│   ├── go.mod
│   ├── main.go
│   ├── Dockerfile
│   ├── data/
│   │   └── testdata.go                        # Synthetic NIB data linked by NPWP
│   └── handler/
│       ├── nib.go                             # NIB search handler
│       └── nib_test.go
│
└── garudacorp/                                # GarudaCorp service
    ├── go.mod
    ├── main.go
    ├── Dockerfile
    ├── config/
    │   ├── config.go                          # Environment config with validation
    │   └── config_test.go
    ├── ahu/
    │   ├── types.go                           # AHU request/response types
    │   ├── client.go                          # AHU HTTP client + circuit breaker
    │   └── client_test.go
    ├── oss/
    │   ├── types.go                           # OSS request/response types
    │   ├── client.go                          # OSS HTTP client + circuit breaker
    │   └── client_test.go
    ├── store/
    │   ├── entity.go                          # Entity + officer + shareholder store
    │   ├── entity_test.go
    │   ├── role.go                            # Role assignment store
    │   └── role_test.go
    ├── handler/
    │   ├── register.go                        # Corporate registration handler
    │   ├── register_test.go
    │   ├── entity.go                          # Entity profile handler
    │   ├── entity_test.go
    │   ├── role.go                            # Role management handlers
    │   └── role_test.go
    └── Dockerfile

# Also modified:
go.work                                       # Add 3 new service modules
.env.example                                  # Add new env vars
docker-compose.yml                            # Add 3 new services
infrastructure/db/migrations/                  # 4 new SQL migration files
```

---

## Task 1: AHU Simulator — Scaffold, Test Data, Handlers, Server, Dockerfile

**Files:**
- Create: `services/ahu-sim/go.mod`
- Create: `services/ahu-sim/data/testdata.go`
- Create: `services/ahu-sim/handler/company.go`
- Create: `services/ahu-sim/handler/company_test.go`
- Create: `services/ahu-sim/main.go`
- Create: `services/ahu-sim/Dockerfile`

- [ ] **Step 1: Initialize Go module**

Run: `mkdir -p services/ahu-sim && cd services/ahu-sim && go mod init github.com/garudapass/gpass/services/ahu-sim`

- [ ] **Step 2: Write synthetic company test data**

```go
// services/ahu-sim/data/testdata.go
package data

// Officer represents an officer (pengurus) of a legal entity.
type Officer struct {
	NIK             string `json:"nik"`
	Name            string `json:"name"`
	Position        string `json:"position"` // DIREKTUR_UTAMA, DIREKTUR, KOMISARIS_UTAMA, KOMISARIS, PEMBINA, PENGAWAS, KETUA, SEKRETARIS
	AppointmentDate string `json:"appointment_date"` // YYYY-MM-DD
}

// Shareholder represents a shareholder (pemegang saham) of a legal entity.
type Shareholder struct {
	Name       string  `json:"name"`
	ShareType  string  `json:"share_type"` // SAHAM_BIASA, SAHAM_PREFEREN
	Shares     int64   `json:"shares"`
	Percentage float64 `json:"percentage"`
}

// Company represents a legal entity registered with AHU.
type Company struct {
	SKNumber          string        `json:"sk_number"`
	Name              string        `json:"name"`
	EntityType        string        `json:"entity_type"` // PT, CV, YAYASAN, KOPERASI
	NPWP              string        `json:"npwp"`
	Address           string        `json:"address"`
	CapitalAuthorized int64         `json:"capital_authorized"` // IDR
	CapitalPaid       int64         `json:"capital_paid"`       // IDR
	EstablishedDate   string        `json:"established_date"`   // YYYY-MM-DD
	Officers          []Officer     `json:"officers"`
	Shareholders      []Shareholder `json:"shareholders"`
}

// TestCompanies contains synthetic AHU records for development and testing.
// Officer NIKs intentionally match NIKs in dukcapil-sim/data/testdata.go
// so that NIK tokenization produces matching tokens across services.
var TestCompanies = map[string]Company{
	"AHU-0012345.AH.01.01.TAHUN2024": {
		SKNumber:          "AHU-0012345.AH.01.01.TAHUN2024",
		Name:              "PT MAJU JAYA TEKNOLOGI",
		EntityType:        "PT",
		NPWP:              "01.234.567.8-012.000",
		Address:           "JL. SUDIRMAN NO. 25, JAKARTA SELATAN",
		CapitalAuthorized: 5000000000,  // 5 billion IDR
		CapitalPaid:       2500000000,  // 2.5 billion IDR
		EstablishedDate:   "2024-03-15",
		Officers: []Officer{
			{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "DIREKTUR_UTAMA", AppointmentDate: "2024-03-15"},
			{NIK: "3174015506850002", Name: "SITI NURHALIZA", Position: "KOMISARIS_UTAMA", AppointmentDate: "2024-03-15"},
			{NIK: "3507012003950003", Name: "AGUS WIJAYA", Position: "DIREKTUR", AppointmentDate: "2024-03-15"},
		},
		Shareholders: []Shareholder{
			{Name: "BUDI SANTOSO", ShareType: "SAHAM_BIASA", Shares: 2500, Percentage: 50.00},
			{Name: "SITI NURHALIZA", ShareType: "SAHAM_BIASA", Shares: 1500, Percentage: 30.00},
			{Name: "AGUS WIJAYA", ShareType: "SAHAM_BIASA", Shares: 1000, Percentage: 20.00},
		},
	},
	"AHU-0067890.AH.01.01.TAHUN2023": {
		SKNumber:          "AHU-0067890.AH.01.01.TAHUN2023",
		Name:              "PT NUSANTARA DIGITAL",
		EntityType:        "PT",
		NPWP:              "02.345.678.9-023.000",
		Address:           "JL. GATOT SUBROTO NO. 88, JAKARTA PUSAT",
		CapitalAuthorized: 10000000000, // 10 billion IDR
		CapitalPaid:       5000000000,  // 5 billion IDR
		EstablishedDate:   "2023-07-20",
		Officers: []Officer{
			{NIK: "5171014712880004", Name: "NI MADE DEWI", Position: "DIREKTUR_UTAMA", AppointmentDate: "2023-07-20"},
			{NIK: "1271010110750005", Name: "AHMAD LUBIS", Position: "KOMISARIS_UTAMA", AppointmentDate: "2023-07-20"},
		},
		Shareholders: []Shareholder{
			{Name: "NI MADE DEWI", ShareType: "SAHAM_BIASA", Shares: 6000, Percentage: 60.00},
			{Name: "AHMAD LUBIS", ShareType: "SAHAM_BIASA", Shares: 4000, Percentage: 40.00},
		},
	},
	"AHU-0011111.AH.01.01.TAHUN2022": {
		SKNumber:          "AHU-0011111.AH.01.01.TAHUN2022",
		Name:              "CV BERKAH MANDIRI",
		EntityType:        "CV",
		NPWP:              "03.456.789.0-034.000",
		Address:           "JL. BRAGA NO. 10, BANDUNG",
		CapitalAuthorized: 500000000,  // 500 million IDR
		CapitalPaid:       500000000,
		EstablishedDate:   "2022-01-10",
		Officers: []Officer{
			{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "DIREKTUR_UTAMA", AppointmentDate: "2022-01-10"},
			{NIK: "3507012003950003", Name: "AGUS WIJAYA", Position: "DIREKTUR", AppointmentDate: "2022-01-10"},
		},
		Shareholders: []Shareholder{
			{Name: "BUDI SANTOSO", ShareType: "SAHAM_BIASA", Shares: 500, Percentage: 50.00},
			{Name: "AGUS WIJAYA", ShareType: "SAHAM_BIASA", Shares: 500, Percentage: 50.00},
		},
	},
	"AHU-0022222.AH.01.01.TAHUN2021": {
		SKNumber:          "AHU-0022222.AH.01.01.TAHUN2021",
		Name:              "YAYASAN PENDIDIKAN BANGSA",
		EntityType:        "YAYASAN",
		NPWP:              "04.567.890.1-045.000",
		Address:           "JL. DIPONEGORO NO. 5, YOGYAKARTA",
		CapitalAuthorized: 1000000000,
		CapitalPaid:       1000000000,
		EstablishedDate:   "2021-08-17",
		Officers: []Officer{
			{NIK: "1271010110750005", Name: "AHMAD LUBIS", Position: "PEMBINA", AppointmentDate: "2021-08-17"},
			{NIK: "3174015506850002", Name: "SITI NURHALIZA", Position: "KETUA", AppointmentDate: "2021-08-17"},
			{NIK: "5171014712880004", Name: "NI MADE DEWI", Position: "PENGAWAS", AppointmentDate: "2021-08-17"},
		},
		Shareholders: []Shareholder{}, // Yayasan has no shareholders
	},
	"AHU-0033333.AH.01.01.TAHUN2024": {
		SKNumber:          "AHU-0033333.AH.01.01.TAHUN2024",
		Name:              "PT GARUDA SOLUSI INDONESIA",
		EntityType:        "PT",
		NPWP:              "05.678.901.2-056.000",
		Address:           "JL. THAMRIN NO. 50, JAKARTA PUSAT",
		CapitalAuthorized: 25000000000, // 25 billion IDR
		CapitalPaid:       12500000000,
		EstablishedDate:   "2024-01-05",
		Officers: []Officer{
			{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "DIREKTUR_UTAMA", AppointmentDate: "2024-01-05"},
			{NIK: "5171014712880004", Name: "NI MADE DEWI", Position: "DIREKTUR", AppointmentDate: "2024-01-05"},
			{NIK: "1271010110750005", Name: "AHMAD LUBIS", Position: "KOMISARIS_UTAMA", AppointmentDate: "2024-01-05"},
			{NIK: "3174015506850002", Name: "SITI NURHALIZA", Position: "KOMISARIS", AppointmentDate: "2024-01-05"},
		},
		Shareholders: []Shareholder{
			{Name: "BUDI SANTOSO", ShareType: "SAHAM_BIASA", Shares: 5000, Percentage: 40.00},
			{Name: "NI MADE DEWI", ShareType: "SAHAM_BIASA", Shares: 3750, Percentage: 30.00},
			{Name: "AHMAD LUBIS", ShareType: "SAHAM_BIASA", Shares: 2500, Percentage: 20.00},
			{Name: "SITI NURHALIZA", ShareType: "SAHAM_PREFEREN", Shares: 1250, Percentage: 10.00},
		},
	},
	"AHU-0044444.AH.01.01.TAHUN2023": {
		SKNumber:          "AHU-0044444.AH.01.01.TAHUN2023",
		Name:              "PT SENTOSA ABADI",
		EntityType:        "PT",
		NPWP:              "06.789.012.3-067.000",
		Address:           "JL. PEMUDA NO. 15, SURABAYA",
		CapitalAuthorized: 2000000000,
		CapitalPaid:       1000000000,
		EstablishedDate:   "2023-11-01",
		Officers: []Officer{
			{NIK: "3507012003950003", Name: "AGUS WIJAYA", Position: "DIREKTUR_UTAMA", AppointmentDate: "2023-11-01"},
			{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "KOMISARIS", AppointmentDate: "2023-11-01"},
		},
		Shareholders: []Shareholder{
			{Name: "AGUS WIJAYA", ShareType: "SAHAM_BIASA", Shares: 7000, Percentage: 70.00},
			{Name: "BUDI SANTOSO", ShareType: "SAHAM_BIASA", Shares: 3000, Percentage: 30.00},
		},
	},
	"AHU-0055555.AH.01.01.TAHUN2022": {
		SKNumber:          "AHU-0055555.AH.01.01.TAHUN2022",
		Name:              "CV KARYA BERSAMA",
		EntityType:        "CV",
		NPWP:              "07.890.123.4-078.000",
		Address:           "JL. MALIOBORO NO. 30, YOGYAKARTA",
		CapitalAuthorized: 250000000,
		CapitalPaid:       250000000,
		EstablishedDate:   "2022-06-15",
		Officers: []Officer{
			{NIK: "1271010110750005", Name: "AHMAD LUBIS", Position: "DIREKTUR_UTAMA", AppointmentDate: "2022-06-15"},
			{NIK: "3174015506850002", Name: "SITI NURHALIZA", Position: "DIREKTUR", AppointmentDate: "2022-06-15"},
		},
		Shareholders: []Shareholder{
			{Name: "AHMAD LUBIS", ShareType: "SAHAM_BIASA", Shares: 600, Percentage: 60.00},
			{Name: "SITI NURHALIZA", ShareType: "SAHAM_BIASA", Shares: 400, Percentage: 40.00},
		},
	},
	"AHU-0066666.AH.01.01.TAHUN2024": {
		SKNumber:          "AHU-0066666.AH.01.01.TAHUN2024",
		Name:              "YAYASAN KASIH NUSANTARA",
		EntityType:        "YAYASAN",
		NPWP:              "08.901.234.5-089.000",
		Address:           "JL. DIPONEGORO NO. 45, SEMARANG",
		CapitalAuthorized: 500000000,
		CapitalPaid:       500000000,
		EstablishedDate:   "2024-02-14",
		Officers: []Officer{
			{NIK: "3174015506850002", Name: "SITI NURHALIZA", Position: "PEMBINA", AppointmentDate: "2024-02-14"},
			{NIK: "3507012003950003", Name: "AGUS WIJAYA", Position: "KETUA", AppointmentDate: "2024-02-14"},
			{NIK: "5171014712880004", Name: "NI MADE DEWI", Position: "SEKRETARIS", AppointmentDate: "2024-02-14"},
		},
		Shareholders: []Shareholder{}, // Yayasan has no shareholders
	},
	"AHU-0077777.AH.01.01.TAHUN2023": {
		SKNumber:          "AHU-0077777.AH.01.01.TAHUN2023",
		Name:              "PT BINTANG TIMUR SEJAHTERA",
		EntityType:        "PT",
		NPWP:              "09.012.345.6-090.000",
		Address:           "JL. RAYA KUTA NO. 88, BADUNG, BALI",
		CapitalAuthorized: 8000000000,
		CapitalPaid:       4000000000,
		EstablishedDate:   "2023-04-22",
		Officers: []Officer{
			{NIK: "5171014712880004", Name: "NI MADE DEWI", Position: "DIREKTUR_UTAMA", AppointmentDate: "2023-04-22"},
			{NIK: "3507012003950003", Name: "AGUS WIJAYA", Position: "KOMISARIS_UTAMA", AppointmentDate: "2023-04-22"},
			{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "DIREKTUR", AppointmentDate: "2023-04-22"},
		},
		Shareholders: []Shareholder{
			{Name: "NI MADE DEWI", ShareType: "SAHAM_BIASA", Shares: 4000, Percentage: 50.00},
			{Name: "AGUS WIJAYA", ShareType: "SAHAM_BIASA", Shares: 2400, Percentage: 30.00},
			{Name: "BUDI SANTOSO", ShareType: "SAHAM_PREFEREN", Shares: 1600, Percentage: 20.00},
		},
	},
	"AHU-0088888.AH.01.01.TAHUN2024": {
		SKNumber:          "AHU-0088888.AH.01.01.TAHUN2024",
		Name:              "KOPERASI SEJAHTERA BERSAMA",
		EntityType:        "KOPERASI",
		NPWP:              "10.123.456.7-101.000",
		Address:           "JL. VETERAN NO. 12, BANDUNG",
		CapitalAuthorized: 100000000,
		CapitalPaid:       100000000,
		EstablishedDate:   "2024-05-01",
		Officers: []Officer{
			{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "KETUA", AppointmentDate: "2024-05-01"},
			{NIK: "3507012003950003", Name: "AGUS WIJAYA", Position: "SEKRETARIS", AppointmentDate: "2024-05-01"},
			{NIK: "1271010110750005", Name: "AHMAD LUBIS", Position: "PENGAWAS", AppointmentDate: "2024-05-01"},
		},
		Shareholders: []Shareholder{}, // Koperasi uses member shares, not traditional shareholders
	},
	"AHU-0099999.AH.01.01.TAHUN2023": {
		SKNumber:          "AHU-0099999.AH.01.01.TAHUN2023",
		Name:              "PT GLOBAL PRIMA MANDIRI",
		EntityType:        "PT",
		NPWP:              "11.234.567.8-112.000",
		Address:           "JL. IMAM BONJOL NO. 7, MEDAN",
		CapitalAuthorized: 15000000000,
		CapitalPaid:       7500000000,
		EstablishedDate:   "2023-09-10",
		Officers: []Officer{
			{NIK: "1271010110750005", Name: "AHMAD LUBIS", Position: "DIREKTUR_UTAMA", AppointmentDate: "2023-09-10"},
			{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "KOMISARIS_UTAMA", AppointmentDate: "2023-09-10"},
			{NIK: "3174015506850002", Name: "SITI NURHALIZA", Position: "DIREKTUR", AppointmentDate: "2023-09-10"},
			{NIK: "5171014712880004", Name: "NI MADE DEWI", Position: "KOMISARIS", AppointmentDate: "2023-09-10"},
		},
		Shareholders: []Shareholder{
			{Name: "AHMAD LUBIS", ShareType: "SAHAM_BIASA", Shares: 6000, Percentage: 40.00},
			{Name: "BUDI SANTOSO", ShareType: "SAHAM_BIASA", Shares: 4500, Percentage: 30.00},
			{Name: "SITI NURHALIZA", ShareType: "SAHAM_BIASA", Shares: 3000, Percentage: 20.00},
			{Name: "NI MADE DEWI", ShareType: "SAHAM_PREFEREN", Shares: 1500, Percentage: 10.00},
		},
	},
}

// LookupBySK returns the company for the given SK number, or nil if not found.
func LookupBySK(sk string) *Company {
	c, ok := TestCompanies[sk]
	if !ok {
		return nil
	}
	return &c
}

// SearchByName returns companies whose name contains the search term (case-insensitive).
func SearchByName(query string) []Company {
	var results []Company
	for _, c := range TestCompanies {
		if containsIgnoreCase(c.Name, query) {
			results = append(results, c)
		}
	}
	return results
}

func containsIgnoreCase(s, substr string) bool {
	// Simple ASCII case-insensitive contains
	sLower := toLower(s)
	subLower := toLower(substr)
	return len(subLower) > 0 && contains(sLower, subLower)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
```

- [ ] **Step 3: Write handler tests**

```go
// services/ahu-sim/handler/company_test.go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/ahu-sim/handler"
)

func TestGetCompanyBySK(t *testing.T) {
	h := handler.New()

	tests := []struct {
		name       string
		sk         string
		wantStatus int
		wantName   string
	}{
		{
			name:       "valid SK returns company",
			sk:         "AHU-0012345.AH.01.01.TAHUN2024",
			wantStatus: http.StatusOK,
			wantName:   "PT MAJU JAYA TEKNOLOGI",
		},
		{
			name:       "unknown SK returns 404",
			sk:         "AHU-UNKNOWN",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "empty SK returns 400",
			sk:         "",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company?sk_number="+tt.sk, nil)
			w := httptest.NewRecorder()

			h.GetCompany(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp handler.CompanyResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.Name != tt.wantName {
					t.Errorf("expected name %q, got %q", tt.wantName, resp.Name)
				}
			}
		})
	}
}

func TestSearchCompanyByName(t *testing.T) {
	h := handler.New()

	tests := []struct {
		name      string
		query     string
		wantCount int
		wantStatus int
	}{
		{
			name:       "search by partial name",
			query:      "MAJU",
			wantCount:  1,
			wantStatus: http.StatusOK,
		},
		{
			name:       "search case insensitive",
			query:      "nusantara",
			wantCount:  2, // PT NUSANTARA DIGITAL + YAYASAN KASIH NUSANTARA
			wantStatus: http.StatusOK,
		},
		{
			name:       "no results",
			query:      "NONEXISTENT",
			wantCount:  0,
			wantStatus: http.StatusOK,
		},
		{
			name:       "empty query returns 400",
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company/search?name="+tt.query, nil)
			w := httptest.NewRecorder()

			h.SearchCompany(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp handler.SearchResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if len(resp.Companies) != tt.wantCount {
					t.Errorf("expected %d companies, got %d", tt.wantCount, len(resp.Companies))
				}
			}
		})
	}
}

func TestGetOfficers(t *testing.T) {
	h := handler.New()

	tests := []struct {
		name       string
		sk         string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "valid SK returns officers",
			sk:         "AHU-0012345.AH.01.01.TAHUN2024",
			wantStatus: http.StatusOK,
			wantCount:  3,
		},
		{
			name:       "unknown SK returns 404",
			sk:         "AHU-UNKNOWN",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "yayasan has officers",
			sk:         "AHU-0022222.AH.01.01.TAHUN2021",
			wantStatus: http.StatusOK,
			wantCount:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company/"+tt.sk+"/officers", nil)
			req.SetPathValue("sk_number", tt.sk)
			w := httptest.NewRecorder()

			h.GetOfficers(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp handler.OfficersResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if len(resp.Officers) != tt.wantCount {
					t.Errorf("expected %d officers, got %d", tt.wantCount, len(resp.Officers))
				}
				// Verify officers have NIKs (AHU returns plaintext NIKs)
				for _, o := range resp.Officers {
					if o.NIK == "" {
						t.Error("officer NIK should not be empty")
					}
					if len(o.NIK) != 16 {
						t.Errorf("officer NIK should be 16 digits, got %d: %s", len(o.NIK), o.NIK)
					}
				}
			}
		})
	}
}

func TestGetShareholders(t *testing.T) {
	h := handler.New()

	tests := []struct {
		name       string
		sk         string
		wantStatus int
		wantCount  int
	}{
		{
			name:       "PT has shareholders",
			sk:         "AHU-0012345.AH.01.01.TAHUN2024",
			wantStatus: http.StatusOK,
			wantCount:  3,
		},
		{
			name:       "yayasan has no shareholders",
			sk:         "AHU-0022222.AH.01.01.TAHUN2021",
			wantStatus: http.StatusOK,
			wantCount:  0,
		},
		{
			name:       "unknown SK returns 404",
			sk:         "AHU-UNKNOWN",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/ahu/company/"+tt.sk+"/shareholders", nil)
			req.SetPathValue("sk_number", tt.sk)
			w := httptest.NewRecorder()

			h.GetShareholders(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp handler.ShareholdersResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if len(resp.Shareholders) != tt.wantCount {
					t.Errorf("expected %d shareholders, got %d", tt.wantCount, len(resp.Shareholders))
				}
			}
		})
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd services/ahu-sim && go test ./handler/... -v`
Expected: FAIL — handler package does not exist.

- [ ] **Step 5: Write handler implementation**

```go
// services/ahu-sim/handler/company.go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/garudapass/gpass/services/ahu-sim/data"
)

// Handler handles AHU simulator HTTP requests.
type Handler struct{}

// New creates a new Handler.
func New() *Handler { return &Handler{} }

// --- Response types ---

type CompanyResponse struct {
	SKNumber          string `json:"sk_number"`
	Name              string `json:"name"`
	EntityType        string `json:"entity_type"`
	NPWP              string `json:"npwp"`
	Address           string `json:"address"`
	CapitalAuthorized int64  `json:"capital_authorized"`
	CapitalPaid       int64  `json:"capital_paid"`
	EstablishedDate   string `json:"established_date"`
}

type SearchResult struct {
	SKNumber   string `json:"sk_number"`
	Name       string `json:"name"`
	EntityType string `json:"entity_type"`
}

type SearchResponse struct {
	Companies []SearchResult `json:"companies"`
}

type OfficerResponse struct {
	NIK             string `json:"nik"`
	Name            string `json:"name"`
	Position        string `json:"position"`
	AppointmentDate string `json:"appointment_date"`
}

type OfficersResponse struct {
	Officers []OfficerResponse `json:"officers"`
}

type ShareholderResponse struct {
	Name       string  `json:"name"`
	ShareType  string  `json:"share_type"`
	Shares     int64   `json:"shares"`
	Percentage float64 `json:"percentage"`
}

type ShareholdersResponse struct {
	Shareholders []ShareholderResponse `json:"shareholders"`
}

// GetCompany handles GET /api/v1/ahu/company?sk_number=...
func (h *Handler) GetCompany(w http.ResponseWriter, r *http.Request) {
	sk := r.URL.Query().Get("sk_number")
	if sk == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sk_number is required"})
		return
	}

	company := data.LookupBySK(sk)
	if company == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "company not found"})
		return
	}

	writeJSON(w, http.StatusOK, CompanyResponse{
		SKNumber:          company.SKNumber,
		Name:              company.Name,
		EntityType:        company.EntityType,
		NPWP:              company.NPWP,
		Address:           company.Address,
		CapitalAuthorized: company.CapitalAuthorized,
		CapitalPaid:       company.CapitalPaid,
		EstablishedDate:   company.EstablishedDate,
	})
}

// SearchCompany handles GET /api/v1/ahu/company/search?name=...
func (h *Handler) SearchCompany(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("name")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name query is required"})
		return
	}

	companies := data.SearchByName(query)
	results := make([]SearchResult, 0, len(companies))
	for _, c := range companies {
		results = append(results, SearchResult{
			SKNumber:   c.SKNumber,
			Name:       c.Name,
			EntityType: c.EntityType,
		})
	}

	writeJSON(w, http.StatusOK, SearchResponse{Companies: results})
}

// GetOfficers handles GET /api/v1/ahu/company/{sk_number}/officers
func (h *Handler) GetOfficers(w http.ResponseWriter, r *http.Request) {
	sk := r.PathValue("sk_number")
	if sk == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sk_number is required"})
		return
	}

	company := data.LookupBySK(sk)
	if company == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "company not found"})
		return
	}

	officers := make([]OfficerResponse, 0, len(company.Officers))
	for _, o := range company.Officers {
		officers = append(officers, OfficerResponse{
			NIK:             o.NIK,
			Name:            o.Name,
			Position:        o.Position,
			AppointmentDate: o.AppointmentDate,
		})
	}

	writeJSON(w, http.StatusOK, OfficersResponse{Officers: officers})
}

// GetShareholders handles GET /api/v1/ahu/company/{sk_number}/shareholders
func (h *Handler) GetShareholders(w http.ResponseWriter, r *http.Request) {
	sk := r.PathValue("sk_number")
	if sk == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sk_number is required"})
		return
	}

	company := data.LookupBySK(sk)
	if company == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "company not found"})
		return
	}

	shareholders := make([]ShareholderResponse, 0, len(company.Shareholders))
	for _, s := range company.Shareholders {
		shareholders = append(shareholders, ShareholderResponse{
			Name:       s.Name,
			ShareType:  s.ShareType,
			Shares:     s.Shares,
			Percentage: s.Percentage,
		})
	}

	writeJSON(w, http.StatusOK, ShareholdersResponse{Shareholders: shareholders})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd services/ahu-sim && go test ./... -v -count=1`
Expected: All tests PASS.

- [ ] **Step 7: Write main.go**

```go
// services/ahu-sim/main.go
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/garudapass/gpass/services/ahu-sim/handler"
)

func main() {
	port := os.Getenv("AHU_SIM_PORT")
	if port == "" {
		port = "4004"
	}

	h := handler.New()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/ahu/company", h.GetCompany)
	mux.HandleFunc("GET /api/v1/ahu/company/search", h.SearchCompany)
	mux.HandleFunc("GET /api/v1/ahu/company/{sk_number}/officers", h.GetOfficers)
	mux.HandleFunc("GET /api/v1/ahu/company/{sk_number}/shareholders", h.GetShareholders)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"ahu-simulator"}`))
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	slog.Info("ahu simulator listening", "port", port)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 8: Write Dockerfile**

```dockerfile
# services/ahu-sim/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /ahu-sim .

FROM alpine:3.20
RUN addgroup -g 1000 -S app && adduser -u 1000 -S app -G app
COPY --from=builder /ahu-sim /usr/local/bin/ahu-sim
USER app:app
EXPOSE 4004
CMD ["/usr/local/bin/ahu-sim"]
```

- [ ] **Step 9: Verify compilation**

Run: `cd services/ahu-sim && go build -o /dev/null .`
Expected: No errors.

- [ ] **Step 10: Commit**

```bash
git add services/ahu-sim/
git commit -m "feat(ahu-sim): add AHU simulator with synthetic company data, handlers, and tests"
```

---

## Task 2: OSS Simulator — Scaffold, Test Data, Handlers, Server, Dockerfile

**Files:**
- Create: `services/oss-sim/go.mod`
- Create: `services/oss-sim/data/testdata.go`
- Create: `services/oss-sim/handler/nib.go`
- Create: `services/oss-sim/handler/nib_test.go`
- Create: `services/oss-sim/main.go`
- Create: `services/oss-sim/Dockerfile`

- [ ] **Step 1: Initialize Go module**

Run: `mkdir -p services/oss-sim && cd services/oss-sim && go mod init github.com/garudapass/gpass/services/oss-sim`

- [ ] **Step 2: Write synthetic NIB test data**

```go
// services/oss-sim/data/testdata.go
package data

// KBLICode represents a KBLI (Klasifikasi Baku Lapangan Usaha Indonesia) code.
type KBLICode struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// NIBRecord represents an OSS NIB (Nomor Induk Berusaha) record.
type NIBRecord struct {
	NIB          string     `json:"nib"`
	NPWP         string     `json:"npwp"`
	CompanyName  string     `json:"company_name"`
	BusinessType string     `json:"business_type"` // PT, CV, YAYASAN, KOPERASI
	KBLICodes    []KBLICode `json:"kbli_codes"`
	IssuedDate   string     `json:"issued_date"` // YYYY-MM-DD
	Status       string     `json:"status"`       // AKTIF, DICABUT
}

// TestNIBs contains synthetic NIB records linked to AHU test companies by NPWP.
var TestNIBs = map[string]NIBRecord{
	"01.234.567.8-012.000": {
		NIB:          "1234567890123",
		NPWP:         "01.234.567.8-012.000",
		CompanyName:  "PT MAJU JAYA TEKNOLOGI",
		BusinessType: "PT",
		KBLICodes: []KBLICode{
			{Code: "62011", Description: "AKTIVITAS PEMROGRAMAN KOMPUTER"},
			{Code: "62021", Description: "AKTIVITAS KONSULTASI KOMPUTER"},
		},
		IssuedDate: "2024-04-01",
		Status:     "AKTIF",
	},
	"02.345.678.9-023.000": {
		NIB:          "2345678901234",
		NPWP:         "02.345.678.9-023.000",
		CompanyName:  "PT NUSANTARA DIGITAL",
		BusinessType: "PT",
		KBLICodes: []KBLICode{
			{Code: "62011", Description: "AKTIVITAS PEMROGRAMAN KOMPUTER"},
			{Code: "63111", Description: "AKTIVITAS PENGOLAHAN DATA"},
			{Code: "62091", Description: "AKTIVITAS TEKNOLOGI INFORMASI LAINNYA"},
		},
		IssuedDate: "2023-08-15",
		Status:     "AKTIF",
	},
	"03.456.789.0-034.000": {
		NIB:          "3456789012345",
		NPWP:         "03.456.789.0-034.000",
		CompanyName:  "CV BERKAH MANDIRI",
		BusinessType: "CV",
		KBLICodes: []KBLICode{
			{Code: "47192", Description: "PERDAGANGAN ECERAN BERBAGAI MACAM BARANG"},
		},
		IssuedDate: "2022-03-01",
		Status:     "AKTIF",
	},
	"04.567.890.1-045.000": {
		NIB:          "4567890123456",
		NPWP:         "04.567.890.1-045.000",
		CompanyName:  "YAYASAN PENDIDIKAN BANGSA",
		BusinessType: "YAYASAN",
		KBLICodes: []KBLICode{
			{Code: "85311", Description: "AKTIVITAS PENDIDIKAN MENENGAH UMUM"},
			{Code: "85321", Description: "AKTIVITAS PENDIDIKAN MENENGAH KEJURUAN"},
		},
		IssuedDate: "2021-10-01",
		Status:     "AKTIF",
	},
	"05.678.901.2-056.000": {
		NIB:          "5678901234567",
		NPWP:         "05.678.901.2-056.000",
		CompanyName:  "PT GARUDA SOLUSI INDONESIA",
		BusinessType: "PT",
		KBLICodes: []KBLICode{
			{Code: "62011", Description: "AKTIVITAS PEMROGRAMAN KOMPUTER"},
			{Code: "62021", Description: "AKTIVITAS KONSULTASI KOMPUTER"},
			{Code: "62031", Description: "AKTIVITAS PENGELOLAAN FASILITAS KOMPUTER"},
		},
		IssuedDate: "2024-02-01",
		Status:     "AKTIF",
	},
	"06.789.012.3-067.000": {
		NIB:          "6789012345678",
		NPWP:         "06.789.012.3-067.000",
		CompanyName:  "PT SENTOSA ABADI",
		BusinessType: "PT",
		KBLICodes: []KBLICode{
			{Code: "46100", Description: "PERDAGANGAN BESAR ATAS DASAR BALAS JASA (FEE)"},
		},
		IssuedDate: "2023-12-01",
		Status:     "AKTIF",
	},
	"09.012.345.6-090.000": {
		NIB:          "9012345678901",
		NPWP:         "09.012.345.6-090.000",
		CompanyName:  "PT BINTANG TIMUR SEJAHTERA",
		BusinessType: "PT",
		KBLICodes: []KBLICode{
			{Code: "55111", Description: "AKTIVITAS HOTEL BINTANG"},
			{Code: "56101", Description: "AKTIVITAS RESTORAN"},
		},
		IssuedDate: "2023-06-01",
		Status:     "AKTIF",
	},
	"11.234.567.8-112.000": {
		NIB:          "1123456789012",
		NPWP:         "11.234.567.8-112.000",
		CompanyName:  "PT GLOBAL PRIMA MANDIRI",
		BusinessType: "PT",
		KBLICodes: []KBLICode{
			{Code: "64191", Description: "AKTIVITAS PERANTARA MONETER LAINNYA"},
			{Code: "66190", Description: "AKTIVITAS PENUNJANG JASA KEUANGAN LAINNYA"},
		},
		IssuedDate: "2023-10-15",
		Status:     "AKTIF",
	},
}

// LookupByNPWP returns the NIB record for the given NPWP, or nil if not found.
func LookupByNPWP(npwp string) *NIBRecord {
	r, ok := TestNIBs[npwp]
	if !ok {
		return nil
	}
	return &r
}
```

- [ ] **Step 3: Write handler tests**

```go
// services/oss-sim/handler/nib_test.go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/oss-sim/handler"
)

func TestGetNIBByNPWP(t *testing.T) {
	h := handler.New()

	tests := []struct {
		name        string
		npwp        string
		wantStatus  int
		wantNIB     string
		wantCompany string
	}{
		{
			name:        "valid NPWP returns NIB",
			npwp:        "01.234.567.8-012.000",
			wantStatus:  http.StatusOK,
			wantNIB:     "1234567890123",
			wantCompany: "PT MAJU JAYA TEKNOLOGI",
		},
		{
			name:        "another valid NPWP",
			npwp:        "02.345.678.9-023.000",
			wantStatus:  http.StatusOK,
			wantNIB:     "2345678901234",
			wantCompany: "PT NUSANTARA DIGITAL",
		},
		{
			name:       "unknown NPWP returns 404",
			npwp:       "99.999.999.9-999.000",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "empty NPWP returns 400",
			npwp:       "",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/oss/nib?npwp="+tt.npwp, nil)
			w := httptest.NewRecorder()

			h.GetNIB(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tt.wantStatus, w.Code, w.Body.String())
			}

			if tt.wantStatus == http.StatusOK {
				var resp handler.NIBResponse
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("decode response: %v", err)
				}
				if resp.NIB != tt.wantNIB {
					t.Errorf("expected NIB %q, got %q", tt.wantNIB, resp.NIB)
				}
				if resp.CompanyName != tt.wantCompany {
					t.Errorf("expected company %q, got %q", tt.wantCompany, resp.CompanyName)
				}
				if len(resp.KBLICodes) == 0 {
					t.Error("expected at least one KBLI code")
				}
			}
		})
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd services/oss-sim && go test ./handler/... -v`
Expected: FAIL — handler package does not exist.

- [ ] **Step 5: Write handler implementation**

```go
// services/oss-sim/handler/nib.go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/garudapass/gpass/services/oss-sim/data"
)

// Handler handles OSS simulator HTTP requests.
type Handler struct{}

// New creates a new Handler.
func New() *Handler { return &Handler{} }

// --- Response types ---

type KBLICodeResponse struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

type NIBResponse struct {
	NIB          string             `json:"nib"`
	NPWP         string             `json:"npwp"`
	CompanyName  string             `json:"company_name"`
	BusinessType string             `json:"business_type"`
	KBLICodes    []KBLICodeResponse `json:"kbli_codes"`
	IssuedDate   string             `json:"issued_date"`
	Status       string             `json:"status"`
}

// GetNIB handles GET /api/v1/oss/nib?npwp=...
func (h *Handler) GetNIB(w http.ResponseWriter, r *http.Request) {
	npwp := r.URL.Query().Get("npwp")
	if npwp == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "npwp is required"})
		return
	}

	record := data.LookupByNPWP(npwp)
	if record == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "nib not found"})
		return
	}

	codes := make([]KBLICodeResponse, 0, len(record.KBLICodes))
	for _, k := range record.KBLICodes {
		codes = append(codes, KBLICodeResponse{
			Code:        k.Code,
			Description: k.Description,
		})
	}

	writeJSON(w, http.StatusOK, NIBResponse{
		NIB:          record.NIB,
		NPWP:         record.NPWP,
		CompanyName:  record.CompanyName,
		BusinessType: record.BusinessType,
		KBLICodes:    codes,
		IssuedDate:   record.IssuedDate,
		Status:       record.Status,
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd services/oss-sim && go test ./... -v -count=1`
Expected: All tests PASS.

- [ ] **Step 7: Write main.go**

```go
// services/oss-sim/main.go
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/garudapass/gpass/services/oss-sim/handler"
)

func main() {
	port := os.Getenv("OSS_SIM_PORT")
	if port == "" {
		port = "4005"
	}

	h := handler.New()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/oss/nib", h.GetNIB)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"oss-simulator"}`))
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	slog.Info("oss simulator listening", "port", port)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 8: Write Dockerfile**

```dockerfile
# services/oss-sim/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /oss-sim .

FROM alpine:3.20
RUN addgroup -g 1000 -S app && adduser -u 1000 -S app -G app
COPY --from=builder /oss-sim /usr/local/bin/oss-sim
USER app:app
EXPOSE 4005
CMD ["/usr/local/bin/oss-sim"]
```

- [ ] **Step 9: Verify compilation**

Run: `cd services/oss-sim && go build -o /dev/null .`
Expected: No errors.

- [ ] **Step 10: Commit**

```bash
git add services/oss-sim/
git commit -m "feat(oss-sim): add OSS simulator with synthetic NIB data, handlers, and tests"
```

---

## Task 3: GarudaCorp — Config + AHU/OSS Clients

**Files:**
- Create: `services/garudacorp/go.mod`
- Create: `services/garudacorp/config/config.go`
- Create: `services/garudacorp/config/config_test.go`
- Create: `services/garudacorp/ahu/types.go`
- Create: `services/garudacorp/ahu/client.go`
- Create: `services/garudacorp/ahu/client_test.go`
- Create: `services/garudacorp/oss/types.go`
- Create: `services/garudacorp/oss/client.go`
- Create: `services/garudacorp/oss/client_test.go`

- [ ] **Step 1: Initialize Go module**

Run: `mkdir -p services/garudacorp && cd services/garudacorp && go mod init github.com/garudapass/gpass/services/garudacorp`

- [ ] **Step 2: Write config test**

```go
// services/garudacorp/config/config_test.go
package config_test

import (
	"os"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/config"
)

func setTestEnv(t *testing.T) {
	t.Helper()
	envs := map[string]string{
		"GARUDACORP_PORT":         "4006",
		"AHU_MODE":               "simulator",
		"AHU_URL":                "http://localhost:4004",
		"AHU_TIMEOUT":            "10s",
		"OSS_MODE":               "simulator",
		"OSS_URL":                "http://localhost:4005",
		"OSS_TIMEOUT":            "10s",
		"SERVER_NIK_KEY":         "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"CORPORATE_TOKEN_SECRET": "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
	}
	for k, v := range envs {
		t.Setenv(k, v)
	}
}

func TestLoadValid(t *testing.T) {
	setTestEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "4006" {
		t.Errorf("expected port 4006, got %s", cfg.Port)
	}
	if cfg.AHUMode != "simulator" {
		t.Errorf("expected AHU mode simulator, got %s", cfg.AHUMode)
	}
	if cfg.OSSMode != "simulator" {
		t.Errorf("expected OSS mode simulator, got %s", cfg.OSSMode)
	}
	if len(cfg.ServerNIKKey) != 32 {
		t.Errorf("expected 32-byte NIK key, got %d", len(cfg.ServerNIKKey))
	}
	if len(cfg.CorporateTokenSecret) != 32 {
		t.Errorf("expected 32-byte token secret, got %d", len(cfg.CorporateTokenSecret))
	}
}

func TestLoadDefaultPort(t *testing.T) {
	setTestEnv(t)
	os.Unsetenv("GARUDACORP_PORT")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "4006" {
		t.Errorf("expected default port 4006, got %s", cfg.Port)
	}
}

func TestLoadMissingNIKKey(t *testing.T) {
	setTestEnv(t)
	os.Unsetenv("SERVER_NIK_KEY")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing SERVER_NIK_KEY")
	}
}

func TestLoadMissingAHUURL(t *testing.T) {
	setTestEnv(t)
	os.Unsetenv("AHU_URL")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing AHU_URL")
	}
}

func TestLoadInvalidAHUMode(t *testing.T) {
	setTestEnv(t)
	t.Setenv("AHU_MODE", "invalid")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid AHU_MODE")
	}
}

func TestLoadRealModeRequiresAPIKey(t *testing.T) {
	setTestEnv(t)
	t.Setenv("AHU_MODE", "real")
	os.Unsetenv("AHU_API_KEY")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing AHU_API_KEY when mode=real")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd services/garudacorp && go test ./config/... -v`
Expected: FAIL — config package does not exist.

- [ ] **Step 4: Write config implementation**

```go
// services/garudacorp/config/config.go
package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config holds all GarudaCorp service configuration loaded from environment variables.
type Config struct {
	Port                 string
	AHUMode              string        // "simulator" or "real"
	AHUURL               string
	AHUAPIKey            string
	AHUTimeout           time.Duration
	OSSMode              string // "simulator" or "real"
	OSSURL               string
	OSSAPIKey            string
	OSSTimeout           time.Duration
	ServerNIKKey         []byte // 32 bytes decoded from hex
	CorporateTokenSecret []byte // 32 bytes decoded from hex
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	ahuTimeoutStr := getEnv("AHU_TIMEOUT", "10s")
	ahuTimeout, err := time.ParseDuration(ahuTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid AHU_TIMEOUT %q: %w", ahuTimeoutStr, err)
	}

	ossTimeoutStr := getEnv("OSS_TIMEOUT", "10s")
	ossTimeout, err := time.ParseDuration(ossTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid OSS_TIMEOUT %q: %w", ossTimeoutStr, err)
	}

	nikKeyHex := os.Getenv("SERVER_NIK_KEY")
	nikKey, err := decodeHexKey(nikKeyHex, "SERVER_NIK_KEY", 32)
	if err != nil {
		return nil, err
	}

	tokenSecretHex := os.Getenv("CORPORATE_TOKEN_SECRET")
	tokenSecret, err := decodeHexKey(tokenSecretHex, "CORPORATE_TOKEN_SECRET", 32)
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		Port:                 getEnv("GARUDACORP_PORT", "4006"),
		AHUMode:              getEnv("AHU_MODE", "simulator"),
		AHUURL:               os.Getenv("AHU_URL"),
		AHUAPIKey:            os.Getenv("AHU_API_KEY"),
		AHUTimeout:           ahuTimeout,
		OSSMode:              getEnv("OSS_MODE", "simulator"),
		OSSURL:               os.Getenv("OSS_URL"),
		OSSAPIKey:            os.Getenv("OSS_API_KEY"),
		OSSTimeout:           ossTimeout,
		ServerNIKKey:         nikKey,
		CorporateTokenSecret: tokenSecret,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	required := []struct {
		name  string
		value string
	}{
		{"AHU_URL", c.AHUURL},
		{"OSS_URL", c.OSSURL},
	}

	var missing []string
	for _, r := range required {
		if r.value == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}

	// Validate AHU mode
	switch c.AHUMode {
	case "simulator", "real":
	default:
		return fmt.Errorf("invalid AHU_MODE %q: must be \"simulator\" or \"real\"", c.AHUMode)
	}

	// Validate OSS mode
	switch c.OSSMode {
	case "simulator", "real":
	default:
		return fmt.Errorf("invalid OSS_MODE %q: must be \"simulator\" or \"real\"", c.OSSMode)
	}

	// When mode is real, API key is required
	if c.AHUMode == "real" && c.AHUAPIKey == "" {
		return fmt.Errorf("AHU_API_KEY is required when AHU_MODE=real")
	}
	if c.OSSMode == "real" && c.OSSAPIKey == "" {
		return fmt.Errorf("OSS_API_KEY is required when OSS_MODE=real")
	}

	// Validate URLs
	for _, check := range []struct {
		name, val string
	}{
		{"AHU_URL", c.AHUURL},
		{"OSS_URL", c.OSSURL},
	} {
		if _, err := url.ParseRequestURI(check.val); err != nil {
			return fmt.Errorf("invalid URL for %s: %w", check.name, err)
		}
	}

	return nil
}

func decodeHexKey(hexStr, name string, expectedLen int) ([]byte, error) {
	if hexStr == "" {
		return nil, fmt.Errorf("required environment variable not set: %s", name)
	}
	if len(hexStr) != expectedLen*2 {
		return nil, fmt.Errorf("%s must be %d hex characters (got %d)", name, expectedLen*2, len(hexStr))
	}
	key, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex for %s: %w", name, err)
	}
	return key, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 5: Run config tests to verify they pass**

Run: `cd services/garudacorp && go test ./config/... -v -count=1`
Expected: All tests PASS.

- [ ] **Step 6: Write AHU types**

```go
// services/garudacorp/ahu/types.go
package ahu

// CompanyResponse represents the response from AHU company lookup.
type CompanyResponse struct {
	SKNumber          string `json:"sk_number"`
	Name              string `json:"name"`
	EntityType        string `json:"entity_type"`
	NPWP              string `json:"npwp"`
	Address           string `json:"address"`
	CapitalAuthorized int64  `json:"capital_authorized"`
	CapitalPaid       int64  `json:"capital_paid"`
	EstablishedDate   string `json:"established_date"`
}

// OfficerResponse represents an officer returned by AHU.
type OfficerResponse struct {
	NIK             string `json:"nik"`
	Name            string `json:"name"`
	Position        string `json:"position"`
	AppointmentDate string `json:"appointment_date"`
}

// OfficersListResponse wraps the officers list from AHU.
type OfficersListResponse struct {
	Officers []OfficerResponse `json:"officers"`
}

// ShareholderResponse represents a shareholder returned by AHU.
type ShareholderResponse struct {
	Name       string  `json:"name"`
	ShareType  string  `json:"share_type"`
	Shares     int64   `json:"shares"`
	Percentage float64 `json:"percentage"`
}

// ShareholdersListResponse wraps the shareholders list from AHU.
type ShareholdersListResponse struct {
	Shareholders []ShareholderResponse `json:"shareholders"`
}
```

- [ ] **Step 7: Write AHU client test**

```go
// services/garudacorp/ahu/client_test.go
package ahu_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudacorp/ahu"
)

func TestGetCompany(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/ahu/company" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		sk := r.URL.Query().Get("sk_number")
		if sk != "AHU-TEST" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ahu.CompanyResponse{
			SKNumber:   "AHU-TEST",
			Name:       "PT TEST",
			EntityType: "PT",
			NPWP:       "01.234.567.8-012.000",
		})
	}))
	defer server.Close()

	client := ahu.NewClient(server.URL, "", 5*time.Second)

	t.Run("found", func(t *testing.T) {
		resp, err := client.GetCompany(context.Background(), "AHU-TEST")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.Name != "PT TEST" {
			t.Errorf("expected PT TEST, got %s", resp.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := client.GetCompany(context.Background(), "AHU-UNKNOWN")
		if err == nil {
			t.Fatal("expected error for unknown SK")
		}
	})
}

func TestGetOfficers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ahu.OfficersListResponse{
			Officers: []ahu.OfficerResponse{
				{NIK: "3201011501900001", Name: "BUDI", Position: "DIREKTUR_UTAMA", AppointmentDate: "2024-01-01"},
				{NIK: "3174015506850002", Name: "SITI", Position: "KOMISARIS", AppointmentDate: "2024-01-01"},
			},
		})
	}))
	defer server.Close()

	client := ahu.NewClient(server.URL, "", 5*time.Second)
	resp, err := client.GetOfficers(context.Background(), "AHU-TEST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Officers) != 2 {
		t.Errorf("expected 2 officers, got %d", len(resp.Officers))
	}
}

func TestGetShareholders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ahu.ShareholdersListResponse{
			Shareholders: []ahu.ShareholderResponse{
				{Name: "BUDI", ShareType: "SAHAM_BIASA", Shares: 1000, Percentage: 50.00},
			},
		})
	}))
	defer server.Close()

	client := ahu.NewClient(server.URL, "", 5*time.Second)
	resp, err := client.GetShareholders(context.Background(), "AHU-TEST")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Shareholders) != 1 {
		t.Errorf("expected 1 shareholder, got %d", len(resp.Shareholders))
	}
}

func TestCircuitBreaker(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	client := ahu.NewClient(server.URL, "", 5*time.Second)

	// Make 5 failing requests to trip the circuit breaker
	for i := 0; i < 5; i++ {
		client.GetCompany(context.Background(), "AHU-TEST")
	}

	// Next call should fail immediately due to open circuit
	_, err := client.GetCompany(context.Background(), "AHU-TEST")
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	// The server should not have received the 6th call
	if callCount > 5 {
		t.Errorf("expected circuit breaker to block call, but server received %d calls", callCount)
	}
}
```

- [ ] **Step 8: Write AHU client implementation**

```go
// services/garudacorp/ahu/client.go
package ahu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Circuit breaker states.
const (
	stateClosed   = iota
	stateOpen
	stateHalfOpen
)

// circuitBreaker implements a simple circuit breaker pattern.
type circuitBreaker struct {
	mu           sync.Mutex
	state        int
	failureCount int
	threshold    int
	openUntil    time.Time
	cooldown     time.Duration
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:     stateClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		return true
	case stateOpen:
		if time.Now().After(cb.openUntil) {
			cb.state = stateHalfOpen
			return true
		}
		return false
	case stateHalfOpen:
		return true
	}
	return false
}

func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.state = stateClosed
}

func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	if cb.failureCount >= cb.threshold {
		cb.state = stateOpen
		cb.openUntil = time.Now().Add(cb.cooldown)
	}
}

// Client is an HTTP client for the AHU API with circuit breaker.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *circuitBreaker
}

// NewClient creates a new AHU API client.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cb: newCircuitBreaker(5, 30*time.Second),
	}
}

// GetCompany retrieves a company by SK number from AHU.
func (c *Client) GetCompany(ctx context.Context, skNumber string) (*CompanyResponse, error) {
	var resp CompanyResponse
	params := url.Values{"sk_number": {skNumber}}
	if err := c.doGet(ctx, "/api/v1/ahu/company", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetOfficers retrieves the officers list for a company by SK number.
func (c *Client) GetOfficers(ctx context.Context, skNumber string) (*OfficersListResponse, error) {
	var resp OfficersListResponse
	path := fmt.Sprintf("/api/v1/ahu/company/%s/officers", url.PathEscape(skNumber))
	if err := c.doGet(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetShareholders retrieves the shareholders list for a company by SK number.
func (c *Client) GetShareholders(ctx context.Context, skNumber string) (*ShareholdersListResponse, error) {
	var resp ShareholdersListResponse
	path := fmt.Sprintf("/api/v1/ahu/company/%s/shareholders", url.PathEscape(skNumber))
	if err := c.doGet(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doGet(ctx context.Context, path string, params url.Values, result interface{}) error {
	if !c.cb.allow() {
		return fmt.Errorf("circuit breaker open: AHU service unavailable")
	}

	reqURL := c.baseURL + path
	if params != nil {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("AHU request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.cb.recordFailure()
		return fmt.Errorf("AHU returned status %d: %s", resp.StatusCode, string(respBody))
	}

	c.cb.recordSuccess()

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}
```

- [ ] **Step 9: Write OSS types**

```go
// services/garudacorp/oss/types.go
package oss

// KBLICode represents a KBLI classification code.
type KBLICode struct {
	Code        string `json:"code"`
	Description string `json:"description"`
}

// NIBResponse represents the response from OSS NIB lookup.
type NIBResponse struct {
	NIB          string     `json:"nib"`
	NPWP         string     `json:"npwp"`
	CompanyName  string     `json:"company_name"`
	BusinessType string     `json:"business_type"`
	KBLICodes    []KBLICode `json:"kbli_codes"`
	IssuedDate   string     `json:"issued_date"`
	Status       string     `json:"status"`
}
```

- [ ] **Step 10: Write OSS client test**

```go
// services/garudacorp/oss/client_test.go
package oss_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudacorp/oss"
)

func TestGetNIB(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/oss/nib" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		npwp := r.URL.Query().Get("npwp")
		if npwp != "01.234.567.8-012.000" {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(oss.NIBResponse{
			NIB:          "1234567890123",
			NPWP:         "01.234.567.8-012.000",
			CompanyName:  "PT TEST",
			BusinessType: "PT",
			KBLICodes: []oss.KBLICode{
				{Code: "62011", Description: "AKTIVITAS PEMROGRAMAN KOMPUTER"},
			},
			IssuedDate: "2024-01-01",
			Status:     "AKTIF",
		})
	}))
	defer server.Close()

	client := oss.NewClient(server.URL, "", 5*time.Second)

	t.Run("found", func(t *testing.T) {
		resp, err := client.GetNIB(context.Background(), "01.234.567.8-012.000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NIB != "1234567890123" {
			t.Errorf("expected NIB 1234567890123, got %s", resp.NIB)
		}
		if len(resp.KBLICodes) != 1 {
			t.Errorf("expected 1 KBLI code, got %d", len(resp.KBLICodes))
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := client.GetNIB(context.Background(), "99.999.999.9-999.000")
		if err == nil {
			t.Fatal("expected error for unknown NPWP")
		}
	})
}

func TestOSSCircuitBreaker(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	client := oss.NewClient(server.URL, "", 5*time.Second)

	// Make 5 failing requests to trip the circuit breaker
	for i := 0; i < 5; i++ {
		client.GetNIB(context.Background(), "01.234.567.8-012.000")
	}

	// Next call should fail immediately due to open circuit
	_, err := client.GetNIB(context.Background(), "01.234.567.8-012.000")
	if err == nil {
		t.Fatal("expected circuit breaker error")
	}
	if callCount > 5 {
		t.Errorf("expected circuit breaker to block call, but server received %d calls", callCount)
	}
}
```

- [ ] **Step 11: Write OSS client implementation**

```go
// services/garudacorp/oss/client.go
package oss

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Circuit breaker states.
const (
	stateClosed   = iota
	stateOpen
	stateHalfOpen
)

// circuitBreaker implements a simple circuit breaker pattern.
type circuitBreaker struct {
	mu           sync.Mutex
	state        int
	failureCount int
	threshold    int
	openUntil    time.Time
	cooldown     time.Duration
}

func newCircuitBreaker(threshold int, cooldown time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:     stateClosed,
		threshold: threshold,
		cooldown:  cooldown,
	}
}

func (cb *circuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		return true
	case stateOpen:
		if time.Now().After(cb.openUntil) {
			cb.state = stateHalfOpen
			return true
		}
		return false
	case stateHalfOpen:
		return true
	}
	return false
}

func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount = 0
	cb.state = stateClosed
}

func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failureCount++
	if cb.failureCount >= cb.threshold {
		cb.state = stateOpen
		cb.openUntil = time.Now().Add(cb.cooldown)
	}
}

// Client is an HTTP client for the OSS API with circuit breaker.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *circuitBreaker
}

// NewClient creates a new OSS API client.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		cb: newCircuitBreaker(5, 30*time.Second),
	}
}

// GetNIB retrieves NIB data by NPWP from OSS.
func (c *Client) GetNIB(ctx context.Context, npwp string) (*NIBResponse, error) {
	var resp NIBResponse
	params := url.Values{"npwp": {npwp}}
	if err := c.doGet(ctx, "/api/v1/oss/nib", params, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doGet(ctx context.Context, path string, params url.Values, result interface{}) error {
	if !c.cb.allow() {
		return fmt.Errorf("circuit breaker open: OSS service unavailable")
	}

	reqURL := c.baseURL + path
	if params != nil {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("OSS request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		c.cb.recordFailure()
		return fmt.Errorf("OSS returned status %d: %s", resp.StatusCode, string(respBody))
	}

	c.cb.recordSuccess()

	if err := json.Unmarshal(respBody, result); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	return nil
}
```

- [ ] **Step 12: Run all tests**

Run: `cd services/garudacorp && go test ./... -v -count=1`
Expected: All tests PASS.

- [ ] **Step 13: Commit**

```bash
git add services/garudacorp/
git commit -m "feat(garudacorp): add config, AHU client, and OSS client with circuit breakers and tests"
```

---

## Task 4: GarudaCorp — Entity + Role Store

**Files:**
- Create: `services/garudacorp/store/entity.go`
- Create: `services/garudacorp/store/entity_test.go`
- Create: `services/garudacorp/store/role.go`
- Create: `services/garudacorp/store/role_test.go`

- [ ] **Step 1: Write entity store test**

```go
// services/garudacorp/store/entity_test.go
package store_test

import (
	"context"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/store"
)

func TestCreateAndGetEntity(t *testing.T) {
	s := store.NewInMemoryEntityStore()
	ctx := context.Background()

	entity := &store.Entity{
		AHUSKNumber:       "AHU-TEST-001",
		Name:              "PT TEST",
		EntityType:        "PT",
		NPWP:              "01.234.567.8-012.000",
		Address:           "JL. TEST NO. 1",
		CapitalAuthorized: 1000000000,
		CapitalPaid:       500000000,
	}

	err := s.CreateEntity(ctx, entity)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entity.ID == "" {
		t.Fatal("expected entity ID to be set")
	}
	if entity.Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", entity.Status)
	}

	// Retrieve by ID
	got, err := s.GetEntityByID(ctx, entity.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "PT TEST" {
		t.Errorf("expected name PT TEST, got %s", got.Name)
	}

	// Retrieve by SK number
	got, err = s.GetEntityBySK(ctx, "AHU-TEST-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != entity.ID {
		t.Errorf("expected ID %s, got %s", entity.ID, got.ID)
	}
}

func TestDuplicateSKNumber(t *testing.T) {
	s := store.NewInMemoryEntityStore()
	ctx := context.Background()

	entity1 := &store.Entity{AHUSKNumber: "AHU-DUP", Name: "PT A", EntityType: "PT"}
	entity2 := &store.Entity{AHUSKNumber: "AHU-DUP", Name: "PT B", EntityType: "PT"}

	if err := s.CreateEntity(ctx, entity1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err := s.CreateEntity(ctx, entity2)
	if err != store.ErrEntitySKExists {
		t.Fatalf("expected ErrEntitySKExists, got %v", err)
	}
}

func TestEntityNotFound(t *testing.T) {
	s := store.NewInMemoryEntityStore()
	ctx := context.Background()

	_, err := s.GetEntityByID(ctx, "nonexistent")
	if err != store.ErrEntityNotFound {
		t.Fatalf("expected ErrEntityNotFound, got %v", err)
	}

	_, err = s.GetEntityBySK(ctx, "nonexistent")
	if err != store.ErrEntityNotFound {
		t.Fatalf("expected ErrEntityNotFound, got %v", err)
	}
}

func TestCreateAndListOfficers(t *testing.T) {
	s := store.NewInMemoryEntityStore()
	ctx := context.Background()

	entity := &store.Entity{AHUSKNumber: "AHU-OFF", Name: "PT OFF", EntityType: "PT"}
	s.CreateEntity(ctx, entity)

	officers := []*store.Officer{
		{EntityID: entity.ID, NIKToken: "token-1", Name: "BUDI", Position: "DIREKTUR_UTAMA", Source: "AHU"},
		{EntityID: entity.ID, NIKToken: "token-2", Name: "SITI", Position: "KOMISARIS", Source: "AHU"},
	}
	for _, o := range officers {
		if err := s.CreateOfficer(ctx, o); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if o.ID == "" {
			t.Fatal("expected officer ID to be set")
		}
	}

	list, err := s.ListOfficers(ctx, entity.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 officers, got %d", len(list))
	}
}

func TestFindOfficerByNIKToken(t *testing.T) {
	s := store.NewInMemoryEntityStore()
	ctx := context.Background()

	entity := &store.Entity{AHUSKNumber: "AHU-NIK", Name: "PT NIK", EntityType: "PT"}
	s.CreateEntity(ctx, entity)

	officer := &store.Officer{EntityID: entity.ID, NIKToken: "matching-token", Name: "BUDI", Position: "DIREKTUR_UTAMA", Source: "AHU"}
	s.CreateOfficer(ctx, officer)

	found, err := s.FindOfficerByNIKToken(ctx, entity.ID, "matching-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.Name != "BUDI" {
		t.Errorf("expected BUDI, got %s", found.Name)
	}

	_, err = s.FindOfficerByNIKToken(ctx, entity.ID, "nonexistent-token")
	if err != store.ErrOfficerNotFound {
		t.Fatalf("expected ErrOfficerNotFound, got %v", err)
	}
}

func TestCreateAndListShareholders(t *testing.T) {
	s := store.NewInMemoryEntityStore()
	ctx := context.Background()

	entity := &store.Entity{AHUSKNumber: "AHU-SH", Name: "PT SH", EntityType: "PT"}
	s.CreateEntity(ctx, entity)

	shareholders := []*store.Shareholder{
		{EntityID: entity.ID, Name: "BUDI", ShareType: "SAHAM_BIASA", Shares: 1000, Percentage: 50.00, Source: "AHU"},
		{EntityID: entity.ID, Name: "SITI", ShareType: "SAHAM_BIASA", Shares: 1000, Percentage: 50.00, Source: "AHU"},
	}
	for _, sh := range shareholders {
		if err := s.CreateShareholder(ctx, sh); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	list, err := s.ListShareholders(ctx, entity.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 shareholders, got %d", len(list))
	}
}

func TestUpdateEntityOSS(t *testing.T) {
	s := store.NewInMemoryEntityStore()
	ctx := context.Background()

	entity := &store.Entity{AHUSKNumber: "AHU-OSS", Name: "PT OSS", EntityType: "PT"}
	s.CreateEntity(ctx, entity)

	err := s.UpdateEntityOSS(ctx, entity.ID, "1234567890123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := s.GetEntityByID(ctx, entity.ID)
	if got.OSSNIB != "1234567890123" {
		t.Errorf("expected NIB 1234567890123, got %s", got.OSSNIB)
	}
	if got.OSSVerifiedAt == nil {
		t.Error("expected OSSVerifiedAt to be set")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd services/garudacorp && go test ./store/... -v`
Expected: FAIL — store package does not exist.

- [ ] **Step 3: Write entity store implementation**

Run: `go get github.com/google/uuid` (from services/garudacorp directory)

```go
// services/garudacorp/store/entity.go
package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors for entity operations.
var (
	ErrEntityNotFound  = errors.New("entity not found")
	ErrEntitySKExists  = errors.New("entity with this SK number already exists")
	ErrOfficerNotFound = errors.New("officer not found")
)

// Entity represents a registered legal entity.
type Entity struct {
	ID                string
	AHUSKNumber       string
	Name              string
	EntityType        string // PT, CV, YAYASAN, KOPERASI
	Status            string // ACTIVE, SUSPENDED
	NPWP              string
	Address           string
	CapitalAuthorized int64
	CapitalPaid       int64
	AHUVerifiedAt     *time.Time
	OSSNIB            string
	OSSVerifiedAt     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Officer represents an officer (pengurus) of a legal entity.
type Officer struct {
	ID              string
	EntityID        string
	UserID          string // empty until user registers
	NIKToken        string // HMAC-SHA256(nik, SERVER_NIK_KEY)
	Name            string
	Position        string
	AppointmentDate string
	Source          string // AHU, MANUAL
	Verified        bool
	CreatedAt       time.Time
}

// Shareholder represents a shareholder of a legal entity.
type Shareholder struct {
	ID         string
	EntityID   string
	Name       string
	ShareType  string // SAHAM_BIASA, SAHAM_PREFEREN
	Shares     int64
	Percentage float64
	Source     string // AHU, MANUAL
	CreatedAt  time.Time
}

// EntityStore defines the interface for entity persistence.
type EntityStore interface {
	CreateEntity(ctx context.Context, e *Entity) error
	GetEntityByID(ctx context.Context, id string) (*Entity, error)
	GetEntityBySK(ctx context.Context, skNumber string) (*Entity, error)
	UpdateEntityOSS(ctx context.Context, entityID, nib string) error
	CreateOfficer(ctx context.Context, o *Officer) error
	ListOfficers(ctx context.Context, entityID string) ([]*Officer, error)
	FindOfficerByNIKToken(ctx context.Context, entityID, nikToken string) (*Officer, error)
	CreateShareholder(ctx context.Context, s *Shareholder) error
	ListShareholders(ctx context.Context, entityID string) ([]*Shareholder, error)
}

// InMemoryEntityStore is a thread-safe in-memory implementation of EntityStore.
type InMemoryEntityStore struct {
	mu           sync.RWMutex
	entities     map[string]*Entity      // id -> entity
	skIndex      map[string]string       // sk_number -> id
	officers     map[string][]*Officer   // entity_id -> officers
	shareholders map[string][]*Shareholder // entity_id -> shareholders
}

// NewInMemoryEntityStore creates a new in-memory entity store.
func NewInMemoryEntityStore() *InMemoryEntityStore {
	return &InMemoryEntityStore{
		entities:     make(map[string]*Entity),
		skIndex:      make(map[string]string),
		officers:     make(map[string][]*Officer),
		shareholders: make(map[string][]*Shareholder),
	}
}

func copyEntity(e *Entity) *Entity {
	cp := *e
	if e.AHUVerifiedAt != nil {
		t := *e.AHUVerifiedAt
		cp.AHUVerifiedAt = &t
	}
	if e.OSSVerifiedAt != nil {
		t := *e.OSSVerifiedAt
		cp.OSSVerifiedAt = &t
	}
	return &cp
}

func copyOfficer(o *Officer) *Officer {
	cp := *o
	return &cp
}

func copyShareholder(s *Shareholder) *Shareholder {
	cp := *s
	return &cp
}

// CreateEntity stores a new entity, auto-setting ID, Status, and timestamps.
func (s *InMemoryEntityStore) CreateEntity(_ context.Context, e *Entity) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.skIndex[e.AHUSKNumber]; exists {
		return ErrEntitySKExists
	}

	now := time.Now().UTC()
	e.ID = uuid.New().String()
	e.Status = "ACTIVE"
	e.CreatedAt = now
	e.UpdatedAt = now

	s.entities[e.ID] = copyEntity(e)
	s.skIndex[e.AHUSKNumber] = e.ID
	return nil
}

// GetEntityByID retrieves an entity by its ID.
func (s *InMemoryEntityStore) GetEntityByID(_ context.Context, id string) (*Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.entities[id]
	if !ok {
		return nil, ErrEntityNotFound
	}
	return copyEntity(e), nil
}

// GetEntityBySK retrieves an entity by its SK number.
func (s *InMemoryEntityStore) GetEntityBySK(_ context.Context, skNumber string) (*Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.skIndex[skNumber]
	if !ok {
		return nil, ErrEntityNotFound
	}
	e := s.entities[id]
	return copyEntity(e), nil
}

// UpdateEntityOSS updates the OSS NIB and verification timestamp for an entity.
func (s *InMemoryEntityStore) UpdateEntityOSS(_ context.Context, entityID, nib string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entities[entityID]
	if !ok {
		return ErrEntityNotFound
	}

	now := time.Now().UTC()
	e.OSSNIB = nib
	e.OSSVerifiedAt = &now
	e.UpdatedAt = now
	return nil
}

// CreateOfficer stores a new officer, auto-setting ID and CreatedAt.
func (s *InMemoryEntityStore) CreateOfficer(_ context.Context, o *Officer) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	o.ID = uuid.New().String()
	o.CreatedAt = time.Now().UTC()

	s.officers[o.EntityID] = append(s.officers[o.EntityID], copyOfficer(o))
	return nil
}

// ListOfficers returns all officers for a given entity ID.
func (s *InMemoryEntityStore) ListOfficers(_ context.Context, entityID string) ([]*Officer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Officer, 0)
	for _, o := range s.officers[entityID] {
		result = append(result, copyOfficer(o))
	}
	return result, nil
}

// FindOfficerByNIKToken finds an officer with a matching NIK token for a given entity.
func (s *InMemoryEntityStore) FindOfficerByNIKToken(_ context.Context, entityID, nikToken string) (*Officer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, o := range s.officers[entityID] {
		if o.NIKToken == nikToken {
			return copyOfficer(o), nil
		}
	}
	return nil, ErrOfficerNotFound
}

// CreateShareholder stores a new shareholder, auto-setting ID and CreatedAt.
func (s *InMemoryEntityStore) CreateShareholder(_ context.Context, sh *Shareholder) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sh.ID = uuid.New().String()
	sh.CreatedAt = time.Now().UTC()

	s.shareholders[sh.EntityID] = append(s.shareholders[sh.EntityID], copyShareholder(sh))
	return nil
}

// ListShareholders returns all shareholders for a given entity ID.
func (s *InMemoryEntityStore) ListShareholders(_ context.Context, entityID string) ([]*Shareholder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Shareholder, 0)
	for _, sh := range s.shareholders[entityID] {
		result = append(result, copyShareholder(sh))
	}
	return result, nil
}
```

- [ ] **Step 4: Write role store test**

```go
// services/garudacorp/store/role_test.go
package store_test

import (
	"context"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/store"
)

func TestCreateAndGetRole(t *testing.T) {
	s := store.NewInMemoryRoleStore()
	ctx := context.Background()

	role := &store.Role{
		EntityID:      "entity-1",
		UserID:        "user-1",
		Role:          "RO",
		GrantedBy:     "system",
		ServiceAccess: map[string]bool{"garudainfo": true},
	}

	err := s.Create(ctx, role)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role.ID == "" {
		t.Fatal("expected role ID to be set")
	}
	if role.Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", role.Status)
	}

	got, err := s.GetByID(ctx, role.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Role != "RO" {
		t.Errorf("expected role RO, got %s", got.Role)
	}
}

func TestDuplicateActiveRole(t *testing.T) {
	s := store.NewInMemoryRoleStore()
	ctx := context.Background()

	role1 := &store.Role{EntityID: "entity-1", UserID: "user-1", Role: "RO", GrantedBy: "system"}
	role2 := &store.Role{EntityID: "entity-1", UserID: "user-1", Role: "ADMIN", GrantedBy: "user-ro"}

	s.Create(ctx, role1)
	err := s.Create(ctx, role2)
	if err != store.ErrRoleExists {
		t.Fatalf("expected ErrRoleExists, got %v", err)
	}
}

func TestListByEntity(t *testing.T) {
	s := store.NewInMemoryRoleStore()
	ctx := context.Background()

	s.Create(ctx, &store.Role{EntityID: "entity-1", UserID: "user-1", Role: "RO", GrantedBy: "system"})
	s.Create(ctx, &store.Role{EntityID: "entity-1", UserID: "user-2", Role: "ADMIN", GrantedBy: "user-1"})
	s.Create(ctx, &store.Role{EntityID: "entity-2", UserID: "user-3", Role: "RO", GrantedBy: "system"})

	roles, err := s.ListByEntity(ctx, "entity-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}
}

func TestGetActiveRole(t *testing.T) {
	s := store.NewInMemoryRoleStore()
	ctx := context.Background()

	s.Create(ctx, &store.Role{EntityID: "entity-1", UserID: "user-1", Role: "RO", GrantedBy: "system"})

	role, err := s.GetActiveRole(ctx, "entity-1", "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role.Role != "RO" {
		t.Errorf("expected role RO, got %s", role.Role)
	}

	_, err = s.GetActiveRole(ctx, "entity-1", "nonexistent")
	if err != store.ErrRoleNotFound {
		t.Fatalf("expected ErrRoleNotFound, got %v", err)
	}
}

func TestRevokeRole(t *testing.T) {
	s := store.NewInMemoryRoleStore()
	ctx := context.Background()

	role := &store.Role{EntityID: "entity-1", UserID: "user-1", Role: "ADMIN", GrantedBy: "user-ro"}
	s.Create(ctx, role)

	err := s.Revoke(ctx, role.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := s.GetByID(ctx, role.ID)
	if got.Status != "REVOKED" {
		t.Errorf("expected status REVOKED, got %s", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("expected RevokedAt to be set")
	}

	// Double revoke should error
	err = s.Revoke(ctx, role.ID)
	if err != store.ErrRoleAlreadyRevoked {
		t.Fatalf("expected ErrRoleAlreadyRevoked, got %v", err)
	}
}

func TestRevokeAllowsNewRole(t *testing.T) {
	s := store.NewInMemoryRoleStore()
	ctx := context.Background()

	role1 := &store.Role{EntityID: "entity-1", UserID: "user-1", Role: "ADMIN", GrantedBy: "user-ro"}
	s.Create(ctx, role1)
	s.Revoke(ctx, role1.ID)

	// After revocation, can create a new role for same user+entity
	role2 := &store.Role{EntityID: "entity-1", UserID: "user-1", Role: "USER", GrantedBy: "user-ro"}
	err := s.Create(ctx, role2)
	if err != nil {
		t.Fatalf("expected to create new role after revocation, got %v", err)
	}
}

func TestCanAssignRole(t *testing.T) {
	tests := []struct {
		name       string
		actorRole  string
		targetRole string
		want       bool
	}{
		{"RO can assign ADMIN", "RO", "ADMIN", true},
		{"RO can assign USER", "RO", "USER", true},
		{"RO cannot assign RO", "RO", "RO", false},
		{"ADMIN can assign USER", "ADMIN", "USER", true},
		{"ADMIN cannot assign ADMIN", "ADMIN", "ADMIN", false},
		{"ADMIN cannot assign RO", "ADMIN", "RO", false},
		{"USER cannot assign anything", "USER", "USER", false},
		{"USER cannot assign ADMIN", "USER", "ADMIN", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.CanAssignRole(tt.actorRole, tt.targetRole)
			if got != tt.want {
				t.Errorf("CanAssignRole(%s, %s) = %v, want %v", tt.actorRole, tt.targetRole, got, tt.want)
			}
		})
	}
}

func TestCanRevokeRole(t *testing.T) {
	tests := []struct {
		name       string
		actorRole  string
		targetRole string
		want       bool
	}{
		{"RO can revoke ADMIN", "RO", "ADMIN", true},
		{"RO can revoke USER", "RO", "USER", true},
		{"RO cannot revoke RO", "RO", "RO", false},
		{"ADMIN can revoke USER", "ADMIN", "USER", true},
		{"ADMIN cannot revoke ADMIN", "ADMIN", "ADMIN", false},
		{"ADMIN cannot revoke RO", "ADMIN", "RO", false},
		{"USER cannot revoke anything", "USER", "USER", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := store.CanRevokeRole(tt.actorRole, tt.targetRole)
			if got != tt.want {
				t.Errorf("CanRevokeRole(%s, %s) = %v, want %v", tt.actorRole, tt.targetRole, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 5: Write role store implementation**

```go
// services/garudacorp/store/role.go
package store

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Sentinel errors for role operations.
var (
	ErrRoleNotFound      = errors.New("role not found")
	ErrRoleExists        = errors.New("user already has an active role on this entity")
	ErrRoleAlreadyRevoked = errors.New("role already revoked")
)

// Role represents a user's role within a corporate entity.
type Role struct {
	ID            string
	EntityID      string
	UserID        string
	Role          string          // RO, ADMIN, USER
	GrantedBy     string          // user_id who granted this role
	ServiceAccess map[string]bool // {"garudainfo": true, "signing": false}
	Status        string          // ACTIVE, REVOKED
	GrantedAt     time.Time
	RevokedAt     *time.Time
	CreatedAt     time.Time
}

// RoleStore defines the interface for role persistence.
type RoleStore interface {
	Create(ctx context.Context, r *Role) error
	GetByID(ctx context.Context, id string) (*Role, error)
	GetActiveRole(ctx context.Context, entityID, userID string) (*Role, error)
	ListByEntity(ctx context.Context, entityID string) ([]*Role, error)
	Revoke(ctx context.Context, id string) error
}

// roleLevel returns the numeric level of a role for hierarchy comparison.
// Higher numbers = more privileged.
func roleLevel(role string) int {
	switch role {
	case "RO":
		return 3
	case "ADMIN":
		return 2
	case "USER":
		return 1
	default:
		return 0
	}
}

// CanAssignRole checks if an actor with the given role can assign the target role.
// RO can assign ADMIN, USER. ADMIN can assign USER. USER cannot assign.
// Nobody can assign RO (it is auto-assigned via NIK matching).
func CanAssignRole(actorRole, targetRole string) bool {
	if targetRole == "RO" {
		return false // RO is never manually assigned
	}
	return roleLevel(actorRole) > roleLevel(targetRole)
}

// CanRevokeRole checks if an actor with the given role can revoke the target role.
// Same hierarchy as assignment: can only revoke roles below your own.
func CanRevokeRole(actorRole, targetRole string) bool {
	if targetRole == "RO" {
		return false // RO cannot be revoked through role management
	}
	return roleLevel(actorRole) > roleLevel(targetRole)
}

// InMemoryRoleStore is a thread-safe in-memory implementation of RoleStore.
type InMemoryRoleStore struct {
	mu    sync.RWMutex
	roles map[string]*Role // id -> role
}

// NewInMemoryRoleStore creates a new in-memory role store.
func NewInMemoryRoleStore() *InMemoryRoleStore {
	return &InMemoryRoleStore{
		roles: make(map[string]*Role),
	}
}

func copyRole(r *Role) *Role {
	cp := *r
	if r.ServiceAccess != nil {
		cp.ServiceAccess = make(map[string]bool, len(r.ServiceAccess))
		for k, v := range r.ServiceAccess {
			cp.ServiceAccess[k] = v
		}
	}
	if r.RevokedAt != nil {
		t := *r.RevokedAt
		cp.RevokedAt = &t
	}
	return &cp
}

// Create stores a new role, auto-setting ID, Status, GrantedAt, and CreatedAt.
// Returns ErrRoleExists if the user already has an active role on the entity.
func (s *InMemoryRoleStore) Create(_ context.Context, r *Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check for existing active role on same entity+user
	for _, existing := range s.roles {
		if existing.EntityID == r.EntityID && existing.UserID == r.UserID && existing.Status == "ACTIVE" {
			return ErrRoleExists
		}
	}

	now := time.Now().UTC()
	r.ID = uuid.New().String()
	r.Status = "ACTIVE"
	r.GrantedAt = now
	r.CreatedAt = now

	s.roles[r.ID] = copyRole(r)
	return nil
}

// GetByID retrieves a role by its ID.
func (s *InMemoryRoleStore) GetByID(_ context.Context, id string) (*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.roles[id]
	if !ok {
		return nil, ErrRoleNotFound
	}
	return copyRole(r), nil
}

// GetActiveRole retrieves the active role for a user on a given entity.
func (s *InMemoryRoleStore) GetActiveRole(_ context.Context, entityID, userID string) (*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.roles {
		if r.EntityID == entityID && r.UserID == userID && r.Status == "ACTIVE" {
			return copyRole(r), nil
		}
	}
	return nil, ErrRoleNotFound
}

// ListByEntity returns all roles for a given entity ID.
func (s *InMemoryRoleStore) ListByEntity(_ context.Context, entityID string) ([]*Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Role
	for _, r := range s.roles {
		if r.EntityID == entityID {
			result = append(result, copyRole(r))
		}
	}
	return result, nil
}

// Revoke sets a role's status to REVOKED. Returns ErrRoleAlreadyRevoked if already revoked.
func (s *InMemoryRoleStore) Revoke(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.roles[id]
	if !ok {
		return ErrRoleNotFound
	}
	if r.Status == "REVOKED" {
		return ErrRoleAlreadyRevoked
	}

	now := time.Now().UTC()
	r.Status = "REVOKED"
	r.RevokedAt = &now
	return nil
}
```

- [ ] **Step 6: Run all store tests**

Run: `cd services/garudacorp && go test ./store/... -v -count=1`
Expected: All tests PASS.

- [ ] **Step 7: Commit**

```bash
git add services/garudacorp/store/
git commit -m "feat(garudacorp): add in-memory entity and role stores with hierarchy enforcement and tests"
```

---

## Task 5: GarudaCorp — Registration Handler

**Files:**
- Create: `services/garudacorp/handler/register.go`
- Create: `services/garudacorp/handler/register_test.go`

- [ ] **Step 1: Write registration handler test**

```go
// services/garudacorp/handler/register_test.go
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/ahu"
	"github.com/garudapass/gpass/services/garudacorp/handler"
	"github.com/garudapass/gpass/services/garudacorp/oss"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

// --- Mock AHU Client ---

type mockAHUClient struct {
	company      *ahu.CompanyResponse
	officers     *ahu.OfficersListResponse
	shareholders *ahu.ShareholdersListResponse
	err          error
}

func (m *mockAHUClient) GetCompany(_ context.Context, sk string) (*ahu.CompanyResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.company == nil {
		return nil, fmt.Errorf("AHU returned status 404: company not found")
	}
	return m.company, nil
}

func (m *mockAHUClient) GetOfficers(_ context.Context, sk string) (*ahu.OfficersListResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.officers, nil
}

func (m *mockAHUClient) GetShareholders(_ context.Context, sk string) (*ahu.ShareholdersListResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.shareholders, nil
}

// --- Mock OSS Client ---

type mockOSSClient struct {
	nib *oss.NIBResponse
	err error
}

func (m *mockOSSClient) GetNIB(_ context.Context, npwp string) (*oss.NIBResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.nib, nil
}

// --- Helper to create NIK token (same logic as identity service) ---

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func tokenizeNIK(nik string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(nik))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestRegisterSuccess(t *testing.T) {
	nikKey := []byte("01234567890123456789012345678901") // 32 bytes
	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()

	// Caller is BUDI SANTOSO with NIK 3201011501900001
	callerNIKToken := tokenizeNIK("3201011501900001", nikKey)

	ahuClient := &mockAHUClient{
		company: &ahu.CompanyResponse{
			SKNumber:          "AHU-0012345.AH.01.01.TAHUN2024",
			Name:              "PT MAJU JAYA TEKNOLOGI",
			EntityType:        "PT",
			NPWP:              "01.234.567.8-012.000",
			Address:           "JL. SUDIRMAN NO. 25",
			CapitalAuthorized: 5000000000,
			CapitalPaid:       2500000000,
		},
		officers: &ahu.OfficersListResponse{
			Officers: []ahu.OfficerResponse{
				{NIK: "3201011501900001", Name: "BUDI SANTOSO", Position: "DIREKTUR_UTAMA", AppointmentDate: "2024-03-15"},
				{NIK: "3174015506850002", Name: "SITI NURHALIZA", Position: "KOMISARIS", AppointmentDate: "2024-03-15"},
			},
		},
		shareholders: &ahu.ShareholdersListResponse{
			Shareholders: []ahu.ShareholderResponse{
				{Name: "BUDI SANTOSO", ShareType: "SAHAM_BIASA", Shares: 2500, Percentage: 50.00},
				{Name: "SITI NURHALIZA", ShareType: "SAHAM_BIASA", Shares: 2500, Percentage: 50.00},
			},
		},
	}

	ossClient := &mockOSSClient{
		nib: &oss.NIBResponse{
			NIB:         "1234567890123",
			NPWP:        "01.234.567.8-012.000",
			CompanyName: "PT MAJU JAYA TEKNOLOGI",
			Status:      "AKTIF",
		},
	}

	h := handler.NewRegisterHandler(entityStore, roleStore, ahuClient, ossClient, nikKey)

	body, _ := json.Marshal(handler.RegisterRequest{SKNumber: "AHU-0012345.AH.01.01.TAHUN2024"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-budi-123")
	req.Header.Set("X-NIK-Token", callerNIKToken)
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.RegisterResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.EntityID == "" {
		t.Fatal("expected entity_id to be set")
	}
	if resp.EntityName != "PT MAJU JAYA TEKNOLOGI" {
		t.Errorf("expected entity name PT MAJU JAYA TEKNOLOGI, got %s", resp.EntityName)
	}
	if resp.Role != "RO" {
		t.Errorf("expected role RO, got %s", resp.Role)
	}

	// Verify entity was stored
	entity, err := entityStore.GetEntityBySK(context.Background(), "AHU-0012345.AH.01.01.TAHUN2024")
	if err != nil {
		t.Fatalf("entity not found: %v", err)
	}
	if entity.Name != "PT MAJU JAYA TEKNOLOGI" {
		t.Errorf("stored entity name mismatch: %s", entity.Name)
	}

	// Verify officers were stored
	officers, _ := entityStore.ListOfficers(context.Background(), entity.ID)
	if len(officers) != 2 {
		t.Errorf("expected 2 officers, got %d", len(officers))
	}

	// Verify RO role was assigned
	role, err := roleStore.GetActiveRole(context.Background(), entity.ID, "user-budi-123")
	if err != nil {
		t.Fatalf("role not found: %v", err)
	}
	if role.Role != "RO" {
		t.Errorf("expected role RO, got %s", role.Role)
	}
}

func TestRegisterNIKNotOfficer(t *testing.T) {
	nikKey := []byte("01234567890123456789012345678901")
	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()

	// Caller's NIK does NOT match any officer
	callerNIKToken := tokenizeNIK("9999999999999999", nikKey)

	ahuClient := &mockAHUClient{
		company: &ahu.CompanyResponse{
			SKNumber: "AHU-TEST", Name: "PT TEST", EntityType: "PT", NPWP: "01.234.567.8-012.000",
		},
		officers: &ahu.OfficersListResponse{
			Officers: []ahu.OfficerResponse{
				{NIK: "3201011501900001", Name: "BUDI", Position: "DIREKTUR_UTAMA"},
			},
		},
		shareholders: &ahu.ShareholdersListResponse{},
	}

	h := handler.NewRegisterHandler(entityStore, roleStore, ahuClient, &mockOSSClient{}, nikKey)

	body, _ := json.Marshal(handler.RegisterRequest{SKNumber: "AHU-TEST"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-unknown")
	req.Header.Set("X-NIK-Token", callerNIKToken)
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterMissingHeaders(t *testing.T) {
	h := handler.NewRegisterHandler(nil, nil, nil, nil, nil)

	body, _ := json.Marshal(handler.RegisterRequest{SKNumber: "AHU-TEST"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// Missing X-User-ID and X-NIK-Token
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRegisterDuplicateSK(t *testing.T) {
	nikKey := []byte("01234567890123456789012345678901")
	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()

	callerNIKToken := tokenizeNIK("3201011501900001", nikKey)

	ahuClient := &mockAHUClient{
		company: &ahu.CompanyResponse{
			SKNumber: "AHU-DUP", Name: "PT DUP", EntityType: "PT", NPWP: "01.234.567.8-012.000",
		},
		officers: &ahu.OfficersListResponse{
			Officers: []ahu.OfficerResponse{
				{NIK: "3201011501900001", Name: "BUDI", Position: "DIREKTUR_UTAMA"},
			},
		},
		shareholders: &ahu.ShareholdersListResponse{},
	}

	h := handler.NewRegisterHandler(entityStore, roleStore, ahuClient, &mockOSSClient{}, nikKey)

	body, _ := json.Marshal(handler.RegisterRequest{SKNumber: "AHU-DUP"})

	// First registration
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("X-User-ID", "user-1")
	req1.Header.Set("X-NIK-Token", callerNIKToken)
	w1 := httptest.NewRecorder()
	h.Register(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first registration expected 201, got %d", w1.Code)
	}

	// Second registration with same SK
	body2, _ := json.Marshal(handler.RegisterRequest{SKNumber: "AHU-DUP"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/corp/register", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("X-User-ID", "user-2")
	req2.Header.Set("X-NIK-Token", callerNIKToken)
	w2 := httptest.NewRecorder()
	h.Register(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("duplicate registration expected 409, got %d: %s", w2.Code, w2.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd services/garudacorp && go test ./handler/... -v`
Expected: FAIL — handler package does not exist.

- [ ] **Step 3: Write registration handler implementation**

```go
// services/garudacorp/handler/register.go
package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudacorp/ahu"
	"github.com/garudapass/gpass/services/garudacorp/oss"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

// AHUClient defines the interface for AHU API operations.
type AHUClient interface {
	GetCompany(ctx context.Context, skNumber string) (*ahu.CompanyResponse, error)
	GetOfficers(ctx context.Context, skNumber string) (*ahu.OfficersListResponse, error)
	GetShareholders(ctx context.Context, skNumber string) (*ahu.ShareholdersListResponse, error)
}

// OSSClient defines the interface for OSS API operations.
type OSSClient interface {
	GetNIB(ctx context.Context, npwp string) (*oss.NIBResponse, error)
}

// RegisterRequest is the request body for corporate registration.
type RegisterRequest struct {
	SKNumber string `json:"sk_number"`
}

// RegisterResponse is the response for successful corporate registration.
type RegisterResponse struct {
	EntityID   string `json:"entity_id"`
	EntityName string `json:"entity_name"`
	Role       string `json:"role"`
}

// RegisterHandler handles corporate entity registration.
type RegisterHandler struct {
	entityStore store.EntityStore
	roleStore   store.RoleStore
	ahuClient   AHUClient
	ossClient   OSSClient
	nikKey      []byte
}

// NewRegisterHandler creates a new RegisterHandler.
func NewRegisterHandler(
	entityStore store.EntityStore,
	roleStore store.RoleStore,
	ahuClient AHUClient,
	ossClient OSSClient,
	nikKey []byte,
) *RegisterHandler {
	return &RegisterHandler{
		entityStore: entityStore,
		roleStore:   roleStore,
		ahuClient:   ahuClient,
		ossClient:   ossClient,
		nikKey:      nikKey,
	}
}

// Register handles POST /api/v1/corp/register
func (h *RegisterHandler) Register(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	callerNIKToken := r.Header.Get("X-NIK-Token")

	if userID == "" || callerNIKToken == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID and X-NIK-Token headers are required"})
		return
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	if req.SKNumber == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sk_number is required"})
		return
	}

	ctx := r.Context()

	// Step 1: Get company from AHU
	company, err := h.ahuClient.GetCompany(ctx, req.SKNumber)
	if err != nil {
		slog.Error("AHU company lookup failed", "error", err, "sk_number", req.SKNumber)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "company_not_found"})
		return
	}

	// Step 2: Get officers from AHU
	officersResp, err := h.ahuClient.GetOfficers(ctx, req.SKNumber)
	if err != nil {
		slog.Error("AHU officers lookup failed", "error", err, "sk_number", req.SKNumber)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "ahu_unavailable"})
		return
	}

	// Step 3: Tokenize each officer NIK and match against caller
	var matchedOfficer *ahu.OfficerResponse
	type tokenizedOfficer struct {
		officer  ahu.OfficerResponse
		nikToken string
	}
	tokenized := make([]tokenizedOfficer, 0, len(officersResp.Officers))

	for _, o := range officersResp.Officers {
		token := tokenizeNIK(o.NIK, h.nikKey)
		tokenized = append(tokenized, tokenizedOfficer{officer: o, nikToken: token})
		if token == callerNIKToken {
			matched := o
			matchedOfficer = &matched
		}
	}

	if matchedOfficer == nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "nik_not_officer"})
		return
	}

	// Step 4: Get shareholders from AHU
	shareholdersResp, err := h.ahuClient.GetShareholders(ctx, req.SKNumber)
	if err != nil {
		slog.Warn("AHU shareholders lookup failed", "error", err, "sk_number", req.SKNumber)
		// Non-blocking — continue without shareholders
		shareholdersResp = &ahu.ShareholdersListResponse{}
	}

	// Step 5: Create entity
	entity := &store.Entity{
		AHUSKNumber:       company.SKNumber,
		Name:              company.Name,
		EntityType:        company.EntityType,
		NPWP:              company.NPWP,
		Address:           company.Address,
		CapitalAuthorized: company.CapitalAuthorized,
		CapitalPaid:       company.CapitalPaid,
	}

	if err := h.entityStore.CreateEntity(ctx, entity); err != nil {
		if err == store.ErrEntitySKExists {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "already_registered"})
			return
		}
		slog.Error("failed to create entity", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}

	// Step 6: Store officers with tokenized NIKs
	for _, to := range tokenized {
		officer := &store.Officer{
			EntityID:        entity.ID,
			NIKToken:        to.nikToken,
			Name:            to.officer.Name,
			Position:        to.officer.Position,
			AppointmentDate: to.officer.AppointmentDate,
			Source:          "AHU",
			Verified:        to.nikToken == callerNIKToken, // Only the caller is verified
		}
		if to.nikToken == callerNIKToken {
			officer.UserID = userID
		}
		if err := h.entityStore.CreateOfficer(ctx, officer); err != nil {
			slog.Error("failed to create officer", "error", err)
		}
	}

	// Step 7: Store shareholders
	for _, s := range shareholdersResp.Shareholders {
		sh := &store.Shareholder{
			EntityID:   entity.ID,
			Name:       s.Name,
			ShareType:  s.ShareType,
			Shares:     s.Shares,
			Percentage: s.Percentage,
			Source:     "AHU",
		}
		if err := h.entityStore.CreateShareholder(ctx, sh); err != nil {
			slog.Error("failed to create shareholder", "error", err)
		}
	}

	// Step 8: Assign RO role to caller
	role := &store.Role{
		EntityID:  entity.ID,
		UserID:    userID,
		Role:      "RO",
		GrantedBy: "system",
	}
	if err := h.roleStore.Create(ctx, role); err != nil {
		slog.Error("failed to assign RO role", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}

	// Step 9: OSS enrichment (non-blocking — fire and forget in production, inline for simplicity)
	if company.NPWP != "" && h.ossClient != nil {
		go func() {
			nibResp, err := h.ossClient.GetNIB(context.Background(), company.NPWP)
			if err != nil {
				slog.Warn("OSS NIB lookup failed", "error", err, "npwp", company.NPWP)
				return
			}
			if err := h.entityStore.UpdateEntityOSS(context.Background(), entity.ID, nibResp.NIB); err != nil {
				slog.Error("failed to update entity OSS data", "error", err)
			}
		}()
	}

	writeJSON(w, http.StatusCreated, RegisterResponse{
		EntityID:   entity.ID,
		EntityName: entity.Name,
		Role:       "RO",
	})
}

func tokenizeNIK(nik string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(nik))
	return hex.EncodeToString(mac.Sum(nil))
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd services/garudacorp && go test ./handler/... -v -count=1`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/garudacorp/handler/register.go services/garudacorp/handler/register_test.go
git commit -m "feat(garudacorp): add corporate registration handler with NIK tokenization and AHU verification"
```

---

## Task 6: GarudaCorp — Role Management Handlers

**Files:**
- Create: `services/garudacorp/handler/role.go`
- Create: `services/garudacorp/handler/role_test.go`

- [ ] **Step 1: Write role handler tests**

```go
// services/garudacorp/handler/role_test.go
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/handler"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

func setupRoleTest(t *testing.T) (*handler.RoleHandler, *store.InMemoryRoleStore, string) {
	t.Helper()
	roleStore := store.NewInMemoryRoleStore()

	// Pre-create an RO role for user-ro on entity-1
	roRole := &store.Role{
		EntityID:  "entity-1",
		UserID:    "user-ro",
		Role:      "RO",
		GrantedBy: "system",
	}
	roleStore.Create(context.Background(), roRole)

	h := handler.NewRoleHandler(roleStore)
	return h, roleStore, roRole.ID
}

func TestAssignRoleSuccess(t *testing.T) {
	h, roleStore, _ := setupRoleTest(t)

	body, _ := json.Marshal(handler.AssignRoleRequest{
		TargetUserID:  "user-admin",
		Role:          "ADMIN",
		ServiceAccess: map[string]bool{"garudainfo": true},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/entities/entity-1/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-ro")
	req.SetPathValue("id", "entity-1")
	w := httptest.NewRecorder()

	h.AssignRole(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.AssignRoleResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Role != "ADMIN" {
		t.Errorf("expected role ADMIN, got %s", resp.Role)
	}
	if resp.Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", resp.Status)
	}

	// Verify in store
	role, err := roleStore.GetActiveRole(context.Background(), "entity-1", "user-admin")
	if err != nil {
		t.Fatalf("role not found: %v", err)
	}
	if role.GrantedBy != "user-ro" {
		t.Errorf("expected granted_by user-ro, got %s", role.GrantedBy)
	}
}

func TestAssignRoleInsufficientPrivilege(t *testing.T) {
	h, roleStore, _ := setupRoleTest(t)

	// Create an ADMIN role
	roleStore.Create(context.Background(), &store.Role{
		EntityID: "entity-1", UserID: "user-admin", Role: "ADMIN", GrantedBy: "user-ro",
	})

	// ADMIN tries to assign ADMIN — should fail
	body, _ := json.Marshal(handler.AssignRoleRequest{
		TargetUserID: "user-new",
		Role:         "ADMIN",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/entities/entity-1/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-admin")
	req.SetPathValue("id", "entity-1")
	w := httptest.NewRecorder()

	h.AssignRole(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAssignROFails(t *testing.T) {
	h, _, _ := setupRoleTest(t)

	// Try to assign RO — should always fail
	body, _ := json.Marshal(handler.AssignRoleRequest{
		TargetUserID: "user-new",
		Role:         "RO",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/corp/entities/entity-1/roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-ro")
	req.SetPathValue("id", "entity-1")
	w := httptest.NewRecorder()

	h.AssignRole(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for RO assignment, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListRoles(t *testing.T) {
	h, roleStore, _ := setupRoleTest(t)

	roleStore.Create(context.Background(), &store.Role{
		EntityID: "entity-1", UserID: "user-admin", Role: "ADMIN", GrantedBy: "user-ro",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/entity-1/roles", nil)
	req.Header.Set("X-User-ID", "user-ro")
	req.SetPathValue("id", "entity-1")
	w := httptest.NewRecorder()

	h.ListRoles(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.ListRolesResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(resp.Roles))
	}
}

func TestRevokeRole(t *testing.T) {
	h, roleStore, _ := setupRoleTest(t)

	// Create an ADMIN role to revoke
	adminRole := &store.Role{
		EntityID: "entity-1", UserID: "user-admin", Role: "ADMIN", GrantedBy: "user-ro",
	}
	roleStore.Create(context.Background(), adminRole)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/corp/entities/entity-1/roles/"+adminRole.ID, nil)
	req.Header.Set("X-User-ID", "user-ro")
	req.SetPathValue("id", "entity-1")
	req.SetPathValue("role_id", adminRole.ID)
	w := httptest.NewRecorder()

	h.RevokeRole(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.RevokeRoleResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Revoked {
		t.Error("expected revoked=true")
	}
}

func TestRevokeRoleInsufficientPrivilege(t *testing.T) {
	h, roleStore, roRoleID := setupRoleTest(t)

	// Create ADMIN
	roleStore.Create(context.Background(), &store.Role{
		EntityID: "entity-1", UserID: "user-admin", Role: "ADMIN", GrantedBy: "user-ro",
	})

	// ADMIN tries to revoke RO — should fail
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/corp/entities/entity-1/roles/"+roRoleID, nil)
	req.Header.Set("X-User-ID", "user-admin")
	req.SetPathValue("id", "entity-1")
	req.SetPathValue("role_id", roRoleID)
	w := httptest.NewRecorder()

	h.RevokeRole(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListRolesNoRole(t *testing.T) {
	h, _, _ := setupRoleTest(t)

	// User with no role on entity tries to list
	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/entity-1/roles", nil)
	req.Header.Set("X-User-ID", "user-nobody")
	req.SetPathValue("id", "entity-1")
	w := httptest.NewRecorder()

	h.ListRoles(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd services/garudacorp && go test ./handler/... -v -run TestAssign -run TestList -run TestRevoke`
Expected: FAIL — RoleHandler not found.

- [ ] **Step 3: Write role handler implementation**

```go
// services/garudacorp/handler/role.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudacorp/store"
)

// AssignRoleRequest is the request body for role assignment.
type AssignRoleRequest struct {
	TargetUserID  string          `json:"target_user_id"`
	Role          string          `json:"role"`
	ServiceAccess map[string]bool `json:"service_access,omitempty"`
}

// AssignRoleResponse is the response for successful role assignment.
type AssignRoleResponse struct {
	RoleID string `json:"role_id"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// ListRolesResponse wraps the list of roles for an entity.
type ListRolesResponse struct {
	Roles []RoleItem `json:"roles"`
}

// RoleItem represents a role in list responses.
type RoleItem struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	GrantedAt string `json:"granted_at"`
	GrantedBy string `json:"granted_by"`
}

// RevokeRoleResponse is the response for role revocation.
type RevokeRoleResponse struct {
	Revoked bool `json:"revoked"`
}

// RoleHandler handles role management requests.
type RoleHandler struct {
	roleStore store.RoleStore
}

// NewRoleHandler creates a new RoleHandler.
func NewRoleHandler(roleStore store.RoleStore) *RoleHandler {
	return &RoleHandler{roleStore: roleStore}
}

// AssignRole handles POST /api/v1/corp/entities/{id}/roles
func (h *RoleHandler) AssignRole(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	entityID := r.PathValue("id")

	if userID == "" || entityID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required parameters"})
		return
	}

	var req AssignRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	if req.TargetUserID == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target_user_id and role are required"})
		return
	}

	ctx := r.Context()

	// Get actor's role
	actorRole, err := h.roleStore.GetActiveRole(ctx, entityID, userID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_authorized"})
		return
	}

	// Check hierarchy
	if !store.CanAssignRole(actorRole.Role, req.Role) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient_role"})
		return
	}

	// Create role
	role := &store.Role{
		EntityID:      entityID,
		UserID:        req.TargetUserID,
		Role:          req.Role,
		GrantedBy:     userID,
		ServiceAccess: req.ServiceAccess,
	}

	if err := h.roleStore.Create(ctx, role); err != nil {
		if err == store.ErrRoleExists {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "role_exists"})
			return
		}
		slog.Error("failed to create role", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}

	writeJSON(w, http.StatusCreated, AssignRoleResponse{
		RoleID: role.ID,
		Role:   role.Role,
		Status: role.Status,
	})
}

// ListRoles handles GET /api/v1/corp/entities/{id}/roles
func (h *RoleHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	entityID := r.PathValue("id")

	if userID == "" || entityID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required parameters"})
		return
	}

	ctx := r.Context()

	// Verify caller has a role on this entity
	_, err := h.roleStore.GetActiveRole(ctx, entityID, userID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_authorized"})
		return
	}

	roles, err := h.roleStore.ListByEntity(ctx, entityID)
	if err != nil {
		slog.Error("failed to list roles", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}

	items := make([]RoleItem, 0, len(roles))
	for _, r := range roles {
		items = append(items, RoleItem{
			ID:        r.ID,
			UserID:    r.UserID,
			Role:      r.Role,
			Status:    r.Status,
			GrantedAt: r.GrantedAt.Format("2006-01-02T15:04:05Z"),
			GrantedBy: r.GrantedBy,
		})
	}

	writeJSON(w, http.StatusOK, ListRolesResponse{Roles: items})
}

// RevokeRole handles DELETE /api/v1/corp/entities/{id}/roles/{role_id}
func (h *RoleHandler) RevokeRole(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	entityID := r.PathValue("id")
	roleID := r.PathValue("role_id")

	if userID == "" || entityID == "" || roleID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required parameters"})
		return
	}

	ctx := r.Context()

	// Get actor's role
	actorRole, err := h.roleStore.GetActiveRole(ctx, entityID, userID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_authorized"})
		return
	}

	// Get target role
	targetRole, err := h.roleStore.GetByID(ctx, roleID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "role_not_found"})
		return
	}

	// Check hierarchy
	if !store.CanRevokeRole(actorRole.Role, targetRole.Role) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient_role"})
		return
	}

	if err := h.roleStore.Revoke(ctx, roleID); err != nil {
		if err == store.ErrRoleAlreadyRevoked {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "already_revoked"})
			return
		}
		slog.Error("failed to revoke role", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}

	writeJSON(w, http.StatusOK, RevokeRoleResponse{Revoked: true})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd services/garudacorp && go test ./handler/... -v -count=1`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/garudacorp/handler/role.go services/garudacorp/handler/role_test.go
git commit -m "feat(garudacorp): add role management handlers with hierarchy enforcement and tests"
```

---

## Task 7: GarudaCorp — Entity Profile Handler

**Files:**
- Create: `services/garudacorp/handler/entity.go`
- Create: `services/garudacorp/handler/entity_test.go`

- [ ] **Step 1: Write entity profile handler test**

```go
// services/garudacorp/handler/entity_test.go
package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudacorp/handler"
	"github.com/garudapass/gpass/services/garudacorp/oss"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

func setupEntityTest(t *testing.T) (*handler.EntityHandler, *store.InMemoryEntityStore, *store.InMemoryRoleStore, string) {
	t.Helper()
	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()
	ctx := context.Background()

	entity := &store.Entity{
		AHUSKNumber:       "AHU-PROFILE-TEST",
		Name:              "PT PROFILE TEST",
		EntityType:        "PT",
		NPWP:              "01.234.567.8-012.000",
		Address:           "JL. TEST NO. 1",
		CapitalAuthorized: 1000000000,
		CapitalPaid:       500000000,
	}
	entityStore.CreateEntity(ctx, entity)

	entityStore.CreateOfficer(ctx, &store.Officer{
		EntityID: entity.ID, NIKToken: "token-1", Name: "BUDI", Position: "DIREKTUR_UTAMA", Source: "AHU", Verified: true,
	})
	entityStore.CreateOfficer(ctx, &store.Officer{
		EntityID: entity.ID, NIKToken: "token-2", Name: "SITI", Position: "KOMISARIS", Source: "AHU",
	})

	entityStore.CreateShareholder(ctx, &store.Shareholder{
		EntityID: entity.ID, Name: "BUDI", ShareType: "SAHAM_BIASA", Shares: 1000, Percentage: 50.00, Source: "AHU",
	})

	roleStore.Create(ctx, &store.Role{
		EntityID: entity.ID, UserID: "user-ro", Role: "RO", GrantedBy: "system",
	})

	ossClient := &mockOSSClient{
		nib: &oss.NIBResponse{
			NIB:          "1234567890123",
			NPWP:         "01.234.567.8-012.000",
			CompanyName:  "PT PROFILE TEST",
			BusinessType: "PT",
			KBLICodes: []oss.KBLICode{
				{Code: "62011", Description: "AKTIVITAS PEMROGRAMAN KOMPUTER"},
			},
			IssuedDate: "2024-01-01",
			Status:     "AKTIF",
		},
	}

	h := handler.NewEntityHandler(entityStore, roleStore, ossClient)
	return h, entityStore, roleStore, entity.ID
}

func TestGetEntityProfile(t *testing.T) {
	h, _, _, entityID := setupEntityTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/"+entityID, nil)
	req.Header.Set("X-User-ID", "user-ro")
	req.SetPathValue("id", entityID)
	w := httptest.NewRecorder()

	h.GetEntity(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.EntityProfileResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "PT PROFILE TEST" {
		t.Errorf("expected name PT PROFILE TEST, got %s", resp.Name)
	}
	if len(resp.Officers) != 2 {
		t.Errorf("expected 2 officers, got %d", len(resp.Officers))
	}
	if len(resp.Shareholders) != 1 {
		t.Errorf("expected 1 shareholder, got %d", len(resp.Shareholders))
	}
}

func TestGetEntityNotAuthorized(t *testing.T) {
	h, _, _, entityID := setupEntityTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/"+entityID, nil)
	req.Header.Set("X-User-ID", "user-nobody")
	req.SetPathValue("id", entityID)
	w := httptest.NewRecorder()

	h.GetEntity(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetEntityNotFound(t *testing.T) {
	h, _, _, _ := setupEntityTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/corp/entities/nonexistent", nil)
	req.Header.Set("X-User-ID", "user-ro")
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.GetEntity(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd services/garudacorp && go test ./handler/... -v -run TestGetEntity`
Expected: FAIL — EntityHandler not found.

- [ ] **Step 3: Write entity profile handler implementation**

```go
// services/garudacorp/handler/entity.go
package handler

import (
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/garudacorp/store"
)

// EntityProfileResponse is the response for entity profile retrieval.
type EntityProfileResponse struct {
	ID                string                    `json:"id"`
	Name              string                    `json:"name"`
	EntityType        string                    `json:"entity_type"`
	Status            string                    `json:"status"`
	NPWP              string                    `json:"npwp,omitempty"`
	Address           string                    `json:"address,omitempty"`
	CapitalAuthorized int64                     `json:"capital_authorized,omitempty"`
	CapitalPaid       int64                     `json:"capital_paid,omitempty"`
	AHUVerifiedAt     string                    `json:"ahu_verified_at,omitempty"`
	OSSNIB            string                    `json:"oss_nib,omitempty"`
	OSSVerifiedAt     string                    `json:"oss_verified_at,omitempty"`
	Officers          []OfficerProfileResponse  `json:"officers"`
	Shareholders      []ShareholderProfileResponse `json:"shareholders"`
}

// OfficerProfileResponse is an officer in the entity profile.
type OfficerProfileResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position string `json:"position"`
	Verified bool   `json:"verified"`
}

// ShareholderProfileResponse is a shareholder in the entity profile.
type ShareholderProfileResponse struct {
	Name       string  `json:"name"`
	ShareType  string  `json:"share_type"`
	Shares     int64   `json:"shares"`
	Percentage float64 `json:"percentage"`
}

// EntityHandler handles entity profile requests.
type EntityHandler struct {
	entityStore store.EntityStore
	roleStore   store.RoleStore
	ossClient   OSSClient
}

// NewEntityHandler creates a new EntityHandler.
func NewEntityHandler(
	entityStore store.EntityStore,
	roleStore store.RoleStore,
	ossClient OSSClient,
) *EntityHandler {
	return &EntityHandler{
		entityStore: entityStore,
		roleStore:   roleStore,
		ossClient:   ossClient,
	}
}

// GetEntity handles GET /api/v1/corp/entities/{id}
func (h *EntityHandler) GetEntity(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	entityID := r.PathValue("id")

	if userID == "" || entityID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required parameters"})
		return
	}

	ctx := r.Context()

	// Get entity
	entity, err := h.entityStore.GetEntityByID(ctx, entityID)
	if err != nil {
		if err == store.ErrEntityNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "entity_not_found"})
			return
		}
		slog.Error("failed to get entity", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}

	// Verify caller has a role on this entity
	_, err = h.roleStore.GetActiveRole(ctx, entityID, userID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_authorized"})
		return
	}

	// Get officers
	officers, err := h.entityStore.ListOfficers(ctx, entityID)
	if err != nil {
		slog.Error("failed to list officers", "error", err)
		officers = nil
	}

	// Get shareholders
	shareholders, err := h.entityStore.ListShareholders(ctx, entityID)
	if err != nil {
		slog.Error("failed to list shareholders", "error", err)
		shareholders = nil
	}

	// Try OSS enrichment if not yet done
	if entity.OSSNIB == "" && entity.NPWP != "" && h.ossClient != nil {
		nibResp, err := h.ossClient.GetNIB(ctx, entity.NPWP)
		if err == nil {
			_ = h.entityStore.UpdateEntityOSS(ctx, entityID, nibResp.NIB)
			entity.OSSNIB = nibResp.NIB
		} else {
			slog.Warn("OSS enrichment failed", "error", err, "entity_id", entityID)
		}
	}

	// Build response
	officerResponses := make([]OfficerProfileResponse, 0, len(officers))
	for _, o := range officers {
		officerResponses = append(officerResponses, OfficerProfileResponse{
			ID:       o.ID,
			Name:     o.Name,
			Position: o.Position,
			Verified: o.Verified,
		})
	}

	shareholderResponses := make([]ShareholderProfileResponse, 0, len(shareholders))
	for _, s := range shareholders {
		shareholderResponses = append(shareholderResponses, ShareholderProfileResponse{
			Name:       s.Name,
			ShareType:  s.ShareType,
			Shares:     s.Shares,
			Percentage: s.Percentage,
		})
	}

	resp := EntityProfileResponse{
		ID:                entity.ID,
		Name:              entity.Name,
		EntityType:        entity.EntityType,
		Status:            entity.Status,
		NPWP:              entity.NPWP,
		Address:           entity.Address,
		CapitalAuthorized: entity.CapitalAuthorized,
		CapitalPaid:       entity.CapitalPaid,
		OSSNIB:            entity.OSSNIB,
		Officers:          officerResponses,
		Shareholders:      shareholderResponses,
	}

	if entity.AHUVerifiedAt != nil {
		resp.AHUVerifiedAt = entity.AHUVerifiedAt.Format("2006-01-02T15:04:05Z")
	}
	if entity.OSSVerifiedAt != nil {
		resp.OSSVerifiedAt = entity.OSSVerifiedAt.Format("2006-01-02T15:04:05Z")
	}

	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd services/garudacorp && go test ./handler/... -v -count=1`
Expected: All tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/garudacorp/handler/entity.go services/garudacorp/handler/entity_test.go
git commit -m "feat(garudacorp): add entity profile handler with OSS enrichment and tests"
```

---

## Task 8: GarudaCorp — Main Server + Dockerfile

**Files:**
- Create: `services/garudacorp/main.go`
- Create: `services/garudacorp/Dockerfile`

- [ ] **Step 1: Write main.go**

```go
// services/garudacorp/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/garudapass/gpass/services/garudacorp/ahu"
	"github.com/garudapass/gpass/services/garudacorp/config"
	"github.com/garudapass/gpass/services/garudacorp/handler"
	"github.com/garudapass/gpass/services/garudacorp/oss"
	"github.com/garudapass/gpass/services/garudacorp/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize clients
	ahuClient := ahu.NewClient(cfg.AHUURL, cfg.AHUAPIKey, cfg.AHUTimeout)
	ossClient := oss.NewClient(cfg.OSSURL, cfg.OSSAPIKey, cfg.OSSTimeout)

	// Initialize stores
	entityStore := store.NewInMemoryEntityStore()
	roleStore := store.NewInMemoryRoleStore()

	// Initialize handlers
	registerHandler := handler.NewRegisterHandler(entityStore, roleStore, ahuClient, ossClient, cfg.ServerNIKKey)
	entityHandler := handler.NewEntityHandler(entityStore, roleStore, ossClient)
	roleHandler := handler.NewRoleHandler(roleStore)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudacorp"}`)
	})

	// Corporate registration
	mux.HandleFunc("POST /api/v1/corp/register", registerHandler.Register)

	// Entity profile
	mux.HandleFunc("GET /api/v1/corp/entities/{id}", entityHandler.GetEntity)

	// Role management
	mux.HandleFunc("POST /api/v1/corp/entities/{id}/roles", roleHandler.AssignRole)
	mux.HandleFunc("GET /api/v1/corp/entities/{id}/roles", roleHandler.ListRoles)
	mux.HandleFunc("DELETE /api/v1/corp/entities/{id}/roles/{role_id}", roleHandler.RevokeRole)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("garudacorp listening", "addr", server.Addr, "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutdown signal received", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("garudacorp shut down gracefully")
}
```

- [ ] **Step 2: Write Dockerfile**

```dockerfile
# services/garudacorp/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /garudacorp .

FROM alpine:3.20
RUN addgroup -g 1000 -S app && adduser -u 1000 -S app -G app
COPY --from=builder /garudacorp /usr/local/bin/garudacorp
USER app:app
EXPOSE 4006
CMD ["/usr/local/bin/garudacorp"]
```

- [ ] **Step 3: Verify compilation**

Run: `cd services/garudacorp && go build -o /dev/null .`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add services/garudacorp/main.go services/garudacorp/Dockerfile
git commit -m "feat(garudacorp): add main server and Dockerfile"
```

---

## Task 9: Integration — go.work, Docker Compose, Env, Migrations, Final Tests

**Files:**
- Modify: `go.work`
- Modify: `docker-compose.yml`
- Modify: `.env.example`
- Create: `infrastructure/db/migrations/003_create_entities.sql`
- Create: `infrastructure/db/migrations/004_create_entity_officers.sql`
- Create: `infrastructure/db/migrations/005_create_entity_roles.sql`
- Create: `infrastructure/db/migrations/006_create_entity_shareholders.sql`

- [ ] **Step 1: Update go.work**

```
go 1.25.0

use (
	./apps/bff
	./services/dukcapil-sim
	./services/garudainfo
	./services/identity
	./services/garudacorp
	./services/ahu-sim
	./services/oss-sim
)
```

- [ ] **Step 2: Add SQL migrations**

```sql
-- infrastructure/db/migrations/003_create_entities.sql
CREATE TABLE entities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ahu_sk_number VARCHAR(100) UNIQUE NOT NULL,
    name VARCHAR(500) NOT NULL,
    entity_type VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    npwp VARCHAR(20),
    address TEXT,
    capital_authorized BIGINT,
    capital_paid BIGINT,
    ahu_verified_at TIMESTAMPTZ,
    oss_nib VARCHAR(20),
    oss_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entities_ahu_sk ON entities(ahu_sk_number);
CREATE INDEX idx_entities_npwp ON entities(npwp);
CREATE INDEX idx_entities_status ON entities(status);
```

```sql
-- infrastructure/db/migrations/004_create_entity_officers.sql
CREATE TABLE entity_officers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    user_id UUID REFERENCES users(id),
    nik_token VARCHAR(64) NOT NULL,
    name VARCHAR(255) NOT NULL,
    position VARCHAR(50) NOT NULL,
    appointment_date DATE,
    source VARCHAR(20) NOT NULL DEFAULT 'AHU',
    verified BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entity_officers_entity_id ON entity_officers(entity_id);
CREATE INDEX idx_entity_officers_nik_token ON entity_officers(nik_token);
CREATE INDEX idx_entity_officers_user_id ON entity_officers(user_id);
```

```sql
-- infrastructure/db/migrations/005_create_entity_roles.sql
CREATE TABLE entity_roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(20) NOT NULL,
    granted_by UUID REFERENCES users(id),
    service_access JSONB,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entity_roles_entity_id ON entity_roles(entity_id);
CREATE INDEX idx_entity_roles_user_id ON entity_roles(user_id);
CREATE INDEX idx_entity_roles_status ON entity_roles(status);
CREATE UNIQUE INDEX idx_entity_roles_active_unique
    ON entity_roles(entity_id, user_id) WHERE status = 'ACTIVE';
```

```sql
-- infrastructure/db/migrations/006_create_entity_shareholders.sql
CREATE TABLE entity_shareholders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_id UUID NOT NULL REFERENCES entities(id),
    name VARCHAR(255) NOT NULL,
    share_type VARCHAR(50) NOT NULL,
    shares BIGINT NOT NULL,
    percentage DECIMAL(5,2) NOT NULL,
    source VARCHAR(20) NOT NULL DEFAULT 'AHU',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_entity_shareholders_entity_id ON entity_shareholders(entity_id);
```

- [ ] **Step 3: Update docker-compose.yml**

Add the following services after the `garudainfo` service:

```yaml
  ahu-sim:
    build: ./services/ahu-sim
    restart: unless-stopped
    ports:
      - "4004:4004"
    environment:
      AHU_SIM_PORT: "4004"
    deploy:
      resources:
        limits:
          memory: 128M
          cpus: "0.25"
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    networks:
      - gpass-network

  oss-sim:
    build: ./services/oss-sim
    restart: unless-stopped
    ports:
      - "4005:4005"
    environment:
      OSS_SIM_PORT: "4005"
    deploy:
      resources:
        limits:
          memory: 128M
          cpus: "0.25"
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    networks:
      - gpass-network

  garudacorp:
    build: ./services/garudacorp
    restart: unless-stopped
    ports:
      - "4006:4006"
    env_file: .env
    depends_on:
      ahu-sim:
        condition: service_started
      oss-sim:
        condition: service_started
    deploy:
      resources:
        limits:
          memory: 256M
          cpus: "0.5"
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"
    networks:
      - gpass-network
```

- [ ] **Step 4: Update .env.example**

Append the following:

```bash
# GarudaCorp Service
GARUDACORP_PORT=4006
AHU_MODE=simulator
AHU_URL=http://localhost:4004
AHU_API_KEY=
AHU_TIMEOUT=10s
OSS_MODE=simulator
OSS_URL=http://localhost:4005
OSS_API_KEY=
OSS_TIMEOUT=10s
CORPORATE_TOKEN_SECRET=abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789

# AHU Simulator
AHU_SIM_PORT=4004

# OSS Simulator
OSS_SIM_PORT=4005
```

- [ ] **Step 5: Run all tests across all new services**

```bash
cd services/ahu-sim && go test ./... -v -count=1
cd services/oss-sim && go test ./... -v -count=1
cd services/garudacorp && go test ./... -v -count=1
```

Expected: All tests PASS across all 3 services.

- [ ] **Step 6: Verify go.work sync**

Run: `cd /opt/gpass && go work sync`
Expected: No errors.

- [ ] **Step 7: Commit**

```bash
git add go.work docker-compose.yml .env.example infrastructure/db/migrations/
git commit -m "feat: integrate Phase 3 services — update go.work, docker-compose, env, and add SQL migrations"
```

---

## Summary

| Task | Service | What | Files | Tests |
|------|---------|------|-------|-------|
| 1 | ahu-sim | AHU simulator: 11 companies (PT, CV, Yayasan, Koperasi), search/officers/shareholders handlers | 6 files | 10+ test cases |
| 2 | oss-sim | OSS simulator: 8 NIB records linked by NPWP, NIB search handler | 6 files | 4+ test cases |
| 3 | garudacorp | Config + AHU/OSS HTTP clients with circuit breakers | 9 files | 12+ test cases |
| 4 | garudacorp | In-memory EntityStore + RoleStore with hierarchy enforcement | 4 files | 16+ test cases |
| 5 | garudacorp | Corporate registration handler: AHU verify + NIK tokenize + match + create | 2 files | 4+ test cases |
| 6 | garudacorp | Role management: assign (hierarchy enforced), list, revoke | 2 files | 6+ test cases |
| 7 | garudacorp | Entity profile with officers, shareholders, OSS enrichment | 2 files | 3+ test cases |
| 8 | garudacorp | Main server (port 4006) + Dockerfile | 2 files | Build verify |
| 9 | integration | go.work, docker-compose, .env.example, 4 SQL migrations | 7 files | Full suite |
| **Total** | **3 services** | **Phase 3 Corporate Identity** | **~40 files** | **55+ test cases** |
