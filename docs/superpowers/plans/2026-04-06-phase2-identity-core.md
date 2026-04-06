# Phase 2: Identity Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the identity core — Dukcapil integration for NIK verification, Go identity service for user registration with field-level encryption, GarudaInfo consent-based verified data API, and Dukcapil simulator for dev/testing.

**Architecture:** Three new Go services in `services/` — identity (registration + Dukcapil client + OTP + Keycloak Admin API), garudainfo (consent management + person data API), dukcapil-sim (mock Dukcapil for dev/CI). All PII encrypted at field-level with AES-256-GCM envelope encryption. NIK tokenized via HMAC-SHA256, never stored in plaintext.

**Tech Stack:** Go 1.22+, PostgreSQL 16, Redis 7 (OTP store), Kafka (audit events), Keycloak Admin API, miniredis/pgx for testing, httptest for Dukcapil client tests

---

## File Structure

```
services/
├── dukcapil-sim/                          # Dukcapil API simulator
│   ├── go.mod
│   ├── main.go
│   ├── data/
│   │   └── testdata.go                    # Synthetic NIK test records
│   ├── handler/
│   │   ├── verify.go                      # NIK, demographic, biometric handlers
│   │   └── verify_test.go
│   └── Dockerfile
│
├── identity/                              # Identity service
│   ├── go.mod
│   ├── main.go
│   ├── config/
│   │   ├── config.go                      # Environment config with validation
│   │   └── config_test.go
│   ├── crypto/
│   │   ├── nik.go                         # NIK tokenization + masking
│   │   ├── nik_test.go
│   │   ├── field.go                       # Envelope encryption (DEK/KEK)
│   │   └── field_test.go
│   ├── dukcapil/
│   │   ├── types.go                       # Request/response types
│   │   ├── client.go                      # HTTP client + circuit breaker
│   │   └── client_test.go
│   ├── otp/
│   │   ├── service.go                     # OTP gen, storage (Redis), verification
│   │   └── service_test.go
│   ├── keycloak/
│   │   ├── admin.go                       # Keycloak Admin API client
│   │   └── admin_test.go
│   ├── store/
│   │   ├── user.go                        # PostgreSQL user repository
│   │   └── user_test.go
│   ├── handler/
│   │   ├── register.go                    # Registration flow handlers
│   │   ├── register_test.go
│   │   ├── user.go                        # User profile CRUD
│   │   └── user_test.go
│   └── Dockerfile
│
└── garudainfo/                            # GarudaInfo consent + data service
    ├── go.mod
    ├── main.go
    ├── config/
    │   ├── config.go
    │   └── config_test.go
    ├── store/
    │   ├── consent.go                     # PostgreSQL consent repository
    │   └── consent_test.go
    ├── handler/
    │   ├── consent.go                     # Consent CRUD + screen
    │   ├── consent_test.go
    │   ├── person.go                      # Verified data API
    │   └── person_test.go
    ├── audit/
    │   ├── kafka.go                       # Kafka event producer
    │   └── kafka_test.go
    └── Dockerfile

# Also modified:
go.work                                   # Add 3 new service modules
.env.example                              # Add new env vars
docker-compose.yml                        # Add 3 new services
infrastructure/db/migrations/             # SQL migration files
```

---

## Task 1: Dukcapil Simulator — Scaffold and Test Data

**Files:**
- Create: `services/dukcapil-sim/go.mod`
- Create: `services/dukcapil-sim/data/testdata.go`

- [ ] **Step 1: Initialize Go module**

Run: `mkdir -p services/dukcapil-sim && cd services/dukcapil-sim && go mod init github.com/garudapass/gpass/services/dukcapil-sim`

- [ ] **Step 2: Write synthetic test data**

```go
// services/dukcapil-sim/data/testdata.go
package data

type Person struct {
	NIK        string
	Name       string
	DOB        string // YYYY-MM-DD
	Gender     string // "M" or "F"
	Province   string
	City       string
	Address    string
	Alive      bool
	PhotoB64   string // stub base64 for biometric matching
}

// TestPeople contains synthetic NIK records for development and testing.
// NIK format: PPRRDDMMYY####  (Province, Regency, DOB-day, month, year, sequence)
// Female DOBs have day+40 per Dukcapil convention.
var TestPeople = map[string]Person{
	"3174011501900001": {
		NIK: "3174011501900001", Name: "Budi Santoso", DOB: "1990-01-15",
		Gender: "M", Province: "DKI Jakarta", City: "Jakarta Selatan",
		Address: "Jl. Sudirman No. 1", Alive: true, PhotoB64: "stub-photo-budi",
	},
	"3174015501900002": {
		NIK: "3174015501900002", Name: "Siti Rahayu", DOB: "1990-01-15",
		Gender: "F", Province: "DKI Jakarta", City: "Jakarta Selatan",
		Address: "Jl. Thamrin No. 2", Alive: true, PhotoB64: "stub-photo-siti",
	},
	"3273011207850003": {
		NIK: "3273011207850003", Name: "Ahmad Wijaya", DOB: "1985-07-12",
		Gender: "M", Province: "Jawa Barat", City: "Bandung",
		Address: "Jl. Braga No. 10", Alive: true, PhotoB64: "stub-photo-ahmad",
	},
	"3578012312750004": {
		NIK: "3578012312750004", Name: "Dewi Lestari", DOB: "1975-12-23",
		Gender: "F", Province: "Jawa Timur", City: "Surabaya",
		Address: "Jl. Darmo No. 5", Alive: true, PhotoB64: "stub-photo-dewi",
	},
	"1101010101000005": {
		NIK: "1101010101000005", Name: "Rizky Pratama", DOB: "2000-01-01",
		Gender: "M", Province: "Aceh", City: "Banda Aceh",
		Address: "Jl. Sultan Iskandar Muda No. 3", Alive: true, PhotoB64: "stub-photo-rizky",
	},
	// Deceased person — for testing alive checks
	"3174010506600099": {
		NIK: "3174010506600099", Name: "Almarhum Hasan", DOB: "1960-06-05",
		Gender: "M", Province: "DKI Jakarta", City: "Jakarta Pusat",
		Address: "Jl. Merdeka No. 17", Alive: false, PhotoB64: "",
	},
	// Invalid NIK format test cases are handled by the validator, not test data.
}

// LookupByNIK returns the person for a given NIK, or nil if not found.
func LookupByNIK(nik string) *Person {
	if p, ok := TestPeople[nik]; ok {
		return &p
	}
	return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add services/dukcapil-sim/
git commit -m "feat(dukcapil-sim): scaffold module with synthetic NIK test data"
```

---

## Task 2: Dukcapil Simulator — Verify Handlers

**Files:**
- Create: `services/dukcapil-sim/handler/verify.go`
- Create: `services/dukcapil-sim/handler/verify_test.go`

- [ ] **Step 1: Write verify handler tests**

```go
// services/dukcapil-sim/handler/verify_test.go
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/dukcapil-sim/handler"
)

func TestVerifyNIKValid(t *testing.T) {
	h := handler.New()
	body, _ := json.Marshal(map[string]string{"nik": "3174011501900001"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/nik", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyNIK(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp handler.NIKResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Valid {
		t.Error("expected valid=true")
	}
	if !resp.Alive {
		t.Error("expected alive=true")
	}
	if resp.Province != "DKI Jakarta" {
		t.Errorf("expected DKI Jakarta, got %s", resp.Province)
	}
}

func TestVerifyNIKNotFound(t *testing.T) {
	h := handler.New()
	body, _ := json.Marshal(map[string]string{"nik": "0000000000000000"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/nik", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyNIK(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp handler.NIKResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Valid {
		t.Error("expected valid=false for unknown NIK")
	}
}

func TestVerifyNIKDeceased(t *testing.T) {
	h := handler.New()
	body, _ := json.Marshal(map[string]string{"nik": "3174010506600099"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/nik", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyNIK(w, req)

	var resp handler.NIKResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Valid {
		t.Error("NIK exists, expected valid=true")
	}
	if resp.Alive {
		t.Error("person is deceased, expected alive=false")
	}
}

func TestVerifyBiometricMatch(t *testing.T) {
	h := handler.New()
	body, _ := json.Marshal(handler.BiometricRequest{
		NIK:       "3174011501900001",
		SelfieB64: "stub-photo-budi",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/biometric", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyBiometric(w, req)

	var resp handler.BiometricResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Match {
		t.Error("expected match=true for matching selfie")
	}
	if resp.Score < 0.75 {
		t.Errorf("expected score >= 0.75, got %f", resp.Score)
	}
}

func TestVerifyBiometricMismatch(t *testing.T) {
	h := handler.New()
	body, _ := json.Marshal(handler.BiometricRequest{
		NIK:       "3174011501900001",
		SelfieB64: "wrong-photo",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/biometric", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyBiometric(w, req)

	var resp handler.BiometricResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Match {
		t.Error("expected match=false for wrong selfie")
	}
	if resp.Score >= 0.75 {
		t.Errorf("expected score < 0.75, got %f", resp.Score)
	}
}

func TestVerifyDemographic(t *testing.T) {
	h := handler.New()
	body, _ := json.Marshal(handler.DemographicRequest{
		NIK: "3174011501900001", Name: "Budi Santoso", DOB: "1990-01-15", Gender: "M",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/verify/demographic", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.VerifyDemographic(w, req)

	var resp handler.DemographicResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if !resp.Match {
		t.Error("expected match=true")
	}
	if resp.Confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9, got %f", resp.Confidence)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd services/dukcapil-sim && go test ./handler/... -v`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Write verify handler implementation**

```go
// services/dukcapil-sim/handler/verify.go
package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/garudapass/gpass/services/dukcapil-sim/data"
)

type Handler struct{}

func New() *Handler { return &Handler{} }

// --- Request/Response types ---

type NIKRequest struct {
	NIK string `json:"nik"`
}

type NIKResponse struct {
	Valid    bool   `json:"valid"`
	Alive    bool   `json:"alive"`
	Province string `json:"province,omitempty"`
}

type DemographicRequest struct {
	NIK    string `json:"nik"`
	Name   string `json:"name"`
	DOB    string `json:"dob"`
	Gender string `json:"gender"`
}

type DemographicResponse struct {
	Match      bool    `json:"match"`
	Confidence float64 `json:"confidence"`
}

type BiometricRequest struct {
	NIK       string `json:"nik"`
	SelfieB64 string `json:"selfie_base64"`
}

type BiometricResponse struct {
	Match bool    `json:"match"`
	Score float64 `json:"score"`
}

// --- Handlers ---

func (h *Handler) VerifyNIK(w http.ResponseWriter, r *http.Request) {
	var req NIKRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	person := data.LookupByNIK(req.NIK)
	if person == nil {
		writeJSON(w, http.StatusOK, NIKResponse{Valid: false, Alive: false})
		return
	}

	writeJSON(w, http.StatusOK, NIKResponse{
		Valid:    true,
		Alive:    person.Alive,
		Province: person.Province,
	})
}

func (h *Handler) VerifyDemographic(w http.ResponseWriter, r *http.Request) {
	var req DemographicRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	person := data.LookupByNIK(req.NIK)
	if person == nil {
		writeJSON(w, http.StatusOK, DemographicResponse{Match: false, Confidence: 0})
		return
	}

	score := 0.0
	if strings.EqualFold(person.Name, req.Name) {
		score += 0.4
	}
	if person.DOB == req.DOB {
		score += 0.3
	}
	if strings.EqualFold(person.Gender, req.Gender) {
		score += 0.3
	}

	writeJSON(w, http.StatusOK, DemographicResponse{
		Match:      score >= 0.7,
		Confidence: score,
	})
}

func (h *Handler) VerifyBiometric(w http.ResponseWriter, r *http.Request) {
	var req BiometricRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	person := data.LookupByNIK(req.NIK)
	if person == nil {
		writeJSON(w, http.StatusOK, BiometricResponse{Match: false, Score: 0})
		return
	}

	// Simulator: exact match of stub photo = high score, otherwise low
	if person.PhotoB64 != "" && person.PhotoB64 == req.SelfieB64 {
		writeJSON(w, http.StatusOK, BiometricResponse{Match: true, Score: 0.92})
	} else {
		writeJSON(w, http.StatusOK, BiometricResponse{Match: false, Score: 0.21})
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd services/dukcapil-sim && go test ./... -v -count=1`
Expected: 6 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/dukcapil-sim/
git commit -m "feat(dukcapil-sim): add NIK, demographic, and biometric verify handlers with tests"
```

---

## Task 3: Dukcapil Simulator — Main Server + Dockerfile

**Files:**
- Create: `services/dukcapil-sim/main.go`
- Create: `services/dukcapil-sim/Dockerfile`

- [ ] **Step 1: Write main.go**

```go
// services/dukcapil-sim/main.go
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/garudapass/gpass/services/dukcapil-sim/handler"
)

func main() {
	port := os.Getenv("DUKCAPIL_SIM_PORT")
	if port == "" {
		port = "4002"
	}

	h := handler.New()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/verify/nik", h.VerifyNIK)
	mux.HandleFunc("POST /api/v1/verify/demographic", h.VerifyDemographic)
	mux.HandleFunc("POST /api/v1/verify/biometric", h.VerifyBiometric)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"dukcapil-simulator"}`))
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	slog.Info("dukcapil simulator listening", "port", port)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Write Dockerfile**

```dockerfile
# services/dukcapil-sim/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dukcapil-sim .

FROM alpine:3.20
RUN addgroup -g 1000 -S app && adduser -u 1000 -S app -G app
COPY --from=builder /dukcapil-sim /usr/local/bin/dukcapil-sim
USER app:app
EXPOSE 4002
CMD ["/usr/local/bin/dukcapil-sim"]
```

- [ ] **Step 3: Verify compilation**

Run: `cd services/dukcapil-sim && go build -o /dev/null .`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add services/dukcapil-sim/main.go services/dukcapil-sim/Dockerfile
git commit -m "feat(dukcapil-sim): add main server and Dockerfile"
```

---

## Task 4: Identity Service — Scaffold, Config, and NIK Crypto

**Files:**
- Create: `services/identity/go.mod`
- Create: `services/identity/config/config.go`
- Create: `services/identity/config/config_test.go`
- Create: `services/identity/crypto/nik.go`
- Create: `services/identity/crypto/nik_test.go`

- [ ] **Step 1: Initialize Go module**

Run: `mkdir -p services/identity && cd services/identity && go mod init github.com/garudapass/gpass/services/identity`

- [ ] **Step 2: Write config test**

```go
// services/identity/config/config_test.go
package config_test

import (
	"os"
	"testing"

	"github.com/garudapass/gpass/services/identity/config"
)

func setTestEnv(t *testing.T) {
	t.Helper()
	envs := map[string]string{
		"IDENTITY_PORT":         "4001",
		"DUKCAPIL_MODE":         "simulator",
		"DUKCAPIL_URL":          "http://localhost:4002",
		"DUKCAPIL_TIMEOUT":      "10s",
		"SERVER_NIK_KEY":        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"FIELD_ENCRYPTION_KEY":  "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
		"KEYCLOAK_ADMIN_URL":    "http://localhost:8080",
		"KEYCLOAK_ADMIN_USER":   "admin",
		"KEYCLOAK_ADMIN_PASSWORD": "admin",
		"OTP_REDIS_URL":         "redis://localhost:6379",
		"DATABASE_URL":          "postgres://garudapass:garudapass@localhost:5432/garudapass",
	}
	for k, v := range envs {
		os.Setenv(k, v)
	}
	t.Cleanup(func() {
		for k := range envs {
			os.Unsetenv(k)
		}
	})
}

func TestLoadConfig(t *testing.T) {
	setTestEnv(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "4001" {
		t.Errorf("expected port 4001, got %s", cfg.Port)
	}
	if cfg.DukcapilMode != "simulator" {
		t.Errorf("expected simulator mode, got %s", cfg.DukcapilMode)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	os.Clearenv()
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for missing env vars")
	}
}

func TestNIKKeyMustBe32Bytes(t *testing.T) {
	setTestEnv(t)
	os.Setenv("SERVER_NIK_KEY", "tooshort")
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for short NIK key")
	}
}
```

- [ ] **Step 3: Write config implementation**

```go
// services/identity/config/config.go
package config

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port               string
	DatabaseURL        string
	DukcapilMode       string // "simulator" or "real"
	DukcapilURL        string
	DukcapilAPIKey     string
	DukcapilTimeout    time.Duration
	NIKKey             []byte // 32-byte HMAC key for NIK tokenization
	FieldEncryptionKey []byte // 32-byte KEK for field-level encryption
	KeycloakAdminURL   string
	KeycloakAdminUser  string
	KeycloakAdminPass  string
	KeycloakRealm      string
	OTPRedisURL        string
}

func Load() (*Config, error) {
	timeout, err := time.ParseDuration(getEnv("DUKCAPIL_TIMEOUT", "10s"))
	if err != nil {
		return nil, fmt.Errorf("invalid DUKCAPIL_TIMEOUT: %w", err)
	}

	nikKeyHex := os.Getenv("SERVER_NIK_KEY")
	nikKey, err := hex.DecodeString(nikKeyHex)
	if err != nil || len(nikKey) != 32 {
		return nil, fmt.Errorf("SERVER_NIK_KEY must be 64 hex chars (32 bytes), got %d chars", len(nikKeyHex))
	}

	fieldKeyHex := os.Getenv("FIELD_ENCRYPTION_KEY")
	fieldKey, err := hex.DecodeString(fieldKeyHex)
	if err != nil || len(fieldKey) != 32 {
		return nil, fmt.Errorf("FIELD_ENCRYPTION_KEY must be 64 hex chars (32 bytes), got %d chars", len(fieldKeyHex))
	}

	cfg := &Config{
		Port:               getEnv("IDENTITY_PORT", "4001"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		DukcapilMode:       getEnv("DUKCAPIL_MODE", "simulator"),
		DukcapilURL:        os.Getenv("DUKCAPIL_URL"),
		DukcapilAPIKey:     os.Getenv("DUKCAPIL_API_KEY"),
		DukcapilTimeout:    timeout,
		NIKKey:             nikKey,
		FieldEncryptionKey: fieldKey,
		KeycloakAdminURL:   os.Getenv("KEYCLOAK_ADMIN_URL"),
		KeycloakAdminUser:  os.Getenv("KEYCLOAK_ADMIN_USER"),
		KeycloakAdminPass:  os.Getenv("KEYCLOAK_ADMIN_PASSWORD"),
		KeycloakRealm:      getEnv("KEYCLOAK_REALM", "garudapass"),
		OTPRedisURL:        os.Getenv("OTP_REDIS_URL"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	required := []struct{ name, val string }{
		{"DATABASE_URL", c.DatabaseURL},
		{"DUKCAPIL_URL", c.DukcapilURL},
		{"KEYCLOAK_ADMIN_URL", c.KeycloakAdminURL},
		{"KEYCLOAK_ADMIN_USER", c.KeycloakAdminUser},
		{"KEYCLOAK_ADMIN_PASSWORD", c.KeycloakAdminPass},
		{"OTP_REDIS_URL", c.OTPRedisURL},
	}
	var missing []string
	for _, r := range required {
		if r.val == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required env vars not set: %s", strings.Join(missing, ", "))
	}

	for _, check := range []struct{ name, val string }{
		{"DUKCAPIL_URL", c.DukcapilURL},
		{"KEYCLOAK_ADMIN_URL", c.KeycloakAdminURL},
		{"DATABASE_URL", c.DatabaseURL},
	} {
		if _, err := url.ParseRequestURI(check.val); err != nil {
			return fmt.Errorf("invalid URL for %s: %w", check.name, err)
		}
	}

	if c.DukcapilMode != "simulator" && c.DukcapilMode != "real" {
		return fmt.Errorf("DUKCAPIL_MODE must be 'simulator' or 'real', got '%s'", c.DukcapilMode)
	}

	if c.DukcapilMode == "real" && c.DukcapilAPIKey == "" {
		return fmt.Errorf("DUKCAPIL_API_KEY required when DUKCAPIL_MODE=real")
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Run config tests**

Run: `cd services/identity && go test ./config/... -v`
Expected: PASS — 3 tests.

- [ ] **Step 5: Write NIK crypto tests**

```go
// services/identity/crypto/nik_test.go
package crypto_test

import (
	"testing"

	"github.com/garudapass/gpass/services/identity/crypto"
)

func TestValidateNIKFormat(t *testing.T) {
	tests := []struct {
		name    string
		nik     string
		wantErr bool
	}{
		{"valid male", "3174011501900001", false},
		{"valid female (day+40)", "3174015501900002", false},
		{"too short", "317401150190", true},
		{"too long", "31740115019000011", true},
		{"non-numeric", "3174011501abcdef", true},
		{"empty", "", true},
		{"invalid province 00", "0074011501900001", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := crypto.ValidateNIKFormat(tt.nik)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNIKFormat(%q) error=%v, wantErr=%v", tt.nik, err, tt.wantErr)
			}
		})
	}
}

func TestTokenizeNIK(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	token1 := crypto.TokenizeNIK("3174011501900001", key)
	token2 := crypto.TokenizeNIK("3174011501900001", key)
	token3 := crypto.TokenizeNIK("3273011207850003", key)

	if token1 != token2 {
		t.Error("same NIK must produce same token")
	}
	if token1 == token3 {
		t.Error("different NIKs must produce different tokens")
	}
	if len(token1) != 64 {
		t.Errorf("token should be 64 hex chars, got %d", len(token1))
	}
}

func TestMaskNIK(t *testing.T) {
	masked := crypto.MaskNIK("3174011501900001")
	if masked != "************0001" {
		t.Errorf("expected ************0001, got %s", masked)
	}
}

func TestMaskNIKShort(t *testing.T) {
	masked := crypto.MaskNIK("1234")
	if masked != "1234" {
		t.Errorf("short NIK should not be masked, got %s", masked)
	}
}
```

- [ ] **Step 6: Write NIK crypto implementation**

```go
// services/identity/crypto/nik.go
package crypto

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// ValidateNIKFormat validates the 16-digit Indonesian NIK format.
// Format: PPRRDDMMYY#### where PP=province, RR=regency, DD=day (female+40),
// MM=month, YY=year, ####=sequence.
func ValidateNIKFormat(nik string) error {
	if len(nik) != 16 {
		return fmt.Errorf("NIK must be 16 digits, got %d", len(nik))
	}
	for _, c := range nik {
		if c < '0' || c > '9' {
			return fmt.Errorf("NIK must contain only digits")
		}
	}

	province, _ := strconv.Atoi(nik[:2])
	if province < 11 || province > 99 {
		return fmt.Errorf("invalid province code: %02d", province)
	}

	return nil
}

// TokenizeNIK produces a deterministic HMAC-SHA256 token for a given NIK.
// The token is used for database lookups without storing the plaintext NIK.
func TokenizeNIK(nik string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(nik))
	return hex.EncodeToString(mac.Sum(nil))
}

// MaskNIK returns a masked version of the NIK for display purposes.
// Example: "3174011501900001" → "************0001"
func MaskNIK(nik string) string {
	if len(nik) <= 4 {
		return nik
	}
	return strings.Repeat("*", len(nik)-4) + nik[len(nik)-4:]
}
```

- [ ] **Step 7: Run NIK tests**

Run: `cd services/identity && go test ./crypto/... -v`
Expected: PASS — 4 tests (with subtests).

- [ ] **Step 8: Commit**

```bash
git add services/identity/
git commit -m "feat(identity): scaffold with config validation and NIK tokenization/masking"
```

---

## Task 5: Identity Service — Field-Level Envelope Encryption

**Files:**
- Create: `services/identity/crypto/field.go`
- Create: `services/identity/crypto/field_test.go`

- [ ] **Step 1: Write field encryption tests**

```go
// services/identity/crypto/field_test.go
package crypto_test

import (
	"crypto/rand"
	"testing"

	"github.com/garudapass/gpass/services/identity/crypto"
)

func makeKEK(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	rand.Read(key)
	return key
}

func TestFieldEncryptorRoundTrip(t *testing.T) {
	kek := makeKEK(t)
	enc, err := crypto.NewFieldEncryptor(kek)
	if err != nil {
		t.Fatalf("NewFieldEncryptor: %v", err)
	}

	// Generate a DEK for this user
	wrappedDEK, err := enc.GenerateWrappedDEK()
	if err != nil {
		t.Fatalf("GenerateWrappedDEK: %v", err)
	}

	// Encrypt a field
	plaintext := "Budi Santoso"
	ciphertext, err := enc.EncryptField(wrappedDEK, []byte(plaintext))
	if err != nil {
		t.Fatalf("EncryptField: %v", err)
	}
	if string(ciphertext) == plaintext {
		t.Error("ciphertext should not equal plaintext")
	}

	// Decrypt the field
	decrypted, err := enc.DecryptField(wrappedDEK, ciphertext)
	if err != nil {
		t.Fatalf("DecryptField: %v", err)
	}
	if string(decrypted) != plaintext {
		t.Errorf("expected %q, got %q", plaintext, string(decrypted))
	}
}

func TestFieldEncryptorDifferentDEKs(t *testing.T) {
	kek := makeKEK(t)
	enc, _ := crypto.NewFieldEncryptor(kek)

	dek1, _ := enc.GenerateWrappedDEK()
	dek2, _ := enc.GenerateWrappedDEK()

	if string(dek1) == string(dek2) {
		t.Error("different DEKs should be different")
	}
}

func TestFieldEncryptorWrongKEK(t *testing.T) {
	kek1 := makeKEK(t)
	kek2 := makeKEK(t)

	enc1, _ := crypto.NewFieldEncryptor(kek1)
	enc2, _ := crypto.NewFieldEncryptor(kek2)

	wrappedDEK, _ := enc1.GenerateWrappedDEK()
	ciphertext, _ := enc1.EncryptField(wrappedDEK, []byte("secret"))

	// Try to decrypt with wrong KEK (can't unwrap DEK)
	_, err := enc2.DecryptField(wrappedDEK, ciphertext)
	if err == nil {
		t.Error("expected error when decrypting with wrong KEK")
	}
}

func TestFieldEncryptorTamperedCiphertext(t *testing.T) {
	kek := makeKEK(t)
	enc, _ := crypto.NewFieldEncryptor(kek)
	wrappedDEK, _ := enc.GenerateWrappedDEK()

	ciphertext, _ := enc.EncryptField(wrappedDEK, []byte("secret"))

	// Tamper with ciphertext
	tampered := make([]byte, len(ciphertext))
	copy(tampered, ciphertext)
	tampered[len(tampered)-1] ^= 0xFF

	_, err := enc.DecryptField(wrappedDEK, tampered)
	if err == nil {
		t.Error("expected error for tampered ciphertext")
	}
}

func TestFieldEncryptorInvalidKEKLength(t *testing.T) {
	_, err := crypto.NewFieldEncryptor([]byte("short"))
	if err == nil {
		t.Error("expected error for short KEK")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd services/identity && go test ./crypto/... -v -run TestFieldEncryptor`
Expected: FAIL — FieldEncryptor not defined.

- [ ] **Step 3: Write field encryption implementation**

```go
// services/identity/crypto/field.go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// FieldEncryptor implements envelope encryption for PII fields.
// Each user gets a unique DEK (Data Encryption Key) wrapped with the server KEK.
// Fields are encrypted with the user's DEK using AES-256-GCM.
type FieldEncryptor struct {
	kekCipher cipher.AEAD
}

// NewFieldEncryptor creates an encryptor with the given KEK (Key Encryption Key).
// KEK must be exactly 32 bytes for AES-256.
func NewFieldEncryptor(kek []byte) (*FieldEncryptor, error) {
	if len(kek) != 32 {
		return nil, fmt.Errorf("KEK must be 32 bytes, got %d", len(kek))
	}
	block, err := aes.NewCipher(kek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &FieldEncryptor{kekCipher: gcm}, nil
}

// GenerateWrappedDEK generates a random 32-byte DEK and wraps it with the KEK.
// The wrapped DEK is stored alongside the user record.
func (f *FieldEncryptor) GenerateWrappedDEK() ([]byte, error) {
	dek := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, dek); err != nil {
		return nil, err
	}
	return f.wrapDEK(dek)
}

// EncryptField encrypts a plaintext field value using the user's wrapped DEK.
func (f *FieldEncryptor) EncryptField(wrappedDEK, plaintext []byte) ([]byte, error) {
	dek, err := f.unwrapDEK(wrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w", err)
	}
	return encrypt(dek, plaintext)
}

// DecryptField decrypts a ciphertext field value using the user's wrapped DEK.
func (f *FieldEncryptor) DecryptField(wrappedDEK, ciphertext []byte) ([]byte, error) {
	dek, err := f.unwrapDEK(wrappedDEK)
	if err != nil {
		return nil, fmt.Errorf("unwrap DEK: %w", err)
	}
	return decrypt(dek, ciphertext)
}

func (f *FieldEncryptor) wrapDEK(dek []byte) ([]byte, error) {
	nonce := make([]byte, f.kekCipher.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return f.kekCipher.Seal(nonce, nonce, dek, nil), nil
}

func (f *FieldEncryptor) unwrapDEK(wrapped []byte) ([]byte, error) {
	nonceSize := f.kekCipher.NonceSize()
	if len(wrapped) < nonceSize {
		return nil, errors.New("wrapped DEK too short")
	}
	nonce, ciphertext := wrapped[:nonceSize], wrapped[nonceSize:]
	return f.kekCipher.Open(nil, nonce, ciphertext, nil)
}

func encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}
```

- [ ] **Step 4: Run all crypto tests**

Run: `cd services/identity && go test ./crypto/... -v -count=1`
Expected: PASS — all NIK + field encryption tests pass.

- [ ] **Step 5: Commit**

```bash
git add services/identity/crypto/field.go services/identity/crypto/field_test.go
git commit -m "feat(identity): add field-level envelope encryption (AES-256-GCM DEK/KEK)"
```

---

## Task 6: Identity Service — Dukcapil HTTP Client with Circuit Breaker

**Files:**
- Create: `services/identity/dukcapil/types.go`
- Create: `services/identity/dukcapil/client.go`
- Create: `services/identity/dukcapil/client_test.go`

- [ ] **Step 1: Write Dukcapil types**

```go
// services/identity/dukcapil/types.go
package dukcapil

// NIKVerifyRequest is sent to verify a NIK exists.
type NIKVerifyRequest struct {
	NIK string `json:"nik"`
}

// NIKVerifyResponse contains the result of a NIK verification.
type NIKVerifyResponse struct {
	Valid    bool   `json:"valid"`
	Alive    bool   `json:"alive"`
	Province string `json:"province,omitempty"`
}

// DemographicRequest is sent to verify demographic data against Dukcapil records.
type DemographicRequest struct {
	NIK    string `json:"nik"`
	Name   string `json:"name"`
	DOB    string `json:"dob"`
	Gender string `json:"gender"`
}

// DemographicResponse contains demographic verification result.
type DemographicResponse struct {
	Match      bool    `json:"match"`
	Confidence float64 `json:"confidence"`
}

// BiometricRequest is sent to verify a selfie against the Dukcapil biometric database.
type BiometricRequest struct {
	NIK       string `json:"nik"`
	SelfieB64 string `json:"selfie_base64"`
}

// BiometricResponse contains biometric face match result.
type BiometricResponse struct {
	Match bool    `json:"match"`
	Score float64 `json:"score"`
}
```

- [ ] **Step 2: Write client tests**

```go
// services/identity/dukcapil/client_test.go
package dukcapil_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/identity/dukcapil"
)

func TestClientVerifyNIK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/verify/nik" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(dukcapil.NIKVerifyResponse{Valid: true, Alive: true, Province: "DKI Jakarta"})
	}))
	defer server.Close()

	client := dukcapil.NewClient(server.URL, "", 5*time.Second)
	resp, err := client.VerifyNIK(context.Background(), "3174011501900001")
	if err != nil {
		t.Fatalf("VerifyNIK error: %v", err)
	}
	if !resp.Valid {
		t.Error("expected valid=true")
	}
	if resp.Province != "DKI Jakarta" {
		t.Errorf("expected DKI Jakarta, got %s", resp.Province)
	}
}

func TestClientVerifyBiometric(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(dukcapil.BiometricResponse{Match: true, Score: 0.92})
	}))
	defer server.Close()

	client := dukcapil.NewClient(server.URL, "", 5*time.Second)
	resp, err := client.VerifyBiometric(context.Background(), "3174011501900001", "photo-data")
	if err != nil {
		t.Fatalf("VerifyBiometric error: %v", err)
	}
	if !resp.Match {
		t.Error("expected match=true")
	}
}

func TestClientTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(dukcapil.NIKVerifyResponse{Valid: true})
	}))
	defer server.Close()

	client := dukcapil.NewClient(server.URL, "", 100*time.Millisecond)
	_, err := client.VerifyNIK(context.Background(), "3174011501900001")
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestClientCircuitBreaker(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := dukcapil.NewClient(server.URL, "", 5*time.Second)

	// Trip the circuit breaker (5 failures)
	for i := 0; i < 5; i++ {
		client.VerifyNIK(context.Background(), "test")
	}

	countBefore := callCount
	// Next call should be rejected by circuit breaker without hitting server
	_, err := client.VerifyNIK(context.Background(), "test")
	if err == nil {
		t.Error("expected circuit breaker error")
	}
	if callCount != countBefore {
		t.Error("circuit breaker should prevent the request from reaching the server")
	}
}
```

- [ ] **Step 3: Write client implementation**

```go
// services/identity/dukcapil/client.go
package dukcapil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Client communicates with the Dukcapil API (real or simulator).
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	cb         *circuitBreaker
}

// NewClient creates a Dukcapil API client with the given base URL and timeout.
func NewClient(baseURL, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
		cb:         newCircuitBreaker(5, 30*time.Second),
	}
}

func (c *Client) VerifyNIK(ctx context.Context, nik string) (*NIKVerifyResponse, error) {
	var resp NIKVerifyResponse
	err := c.post(ctx, "/api/v1/verify/nik", NIKVerifyRequest{NIK: nik}, &resp)
	return &resp, err
}

func (c *Client) VerifyDemographic(ctx context.Context, req DemographicRequest) (*DemographicResponse, error) {
	var resp DemographicResponse
	err := c.post(ctx, "/api/v1/verify/demographic", req, &resp)
	return &resp, err
}

func (c *Client) VerifyBiometric(ctx context.Context, nik, selfieB64 string) (*BiometricResponse, error) {
	var resp BiometricResponse
	err := c.post(ctx, "/api/v1/verify/biometric", BiometricRequest{NIK: nik, SelfieB64: selfieB64}, &resp)
	return &resp, err
}

func (c *Client) post(ctx context.Context, path string, body, result interface{}) error {
	if err := c.cb.check(); err != nil {
		return err
	}

	b, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.cb.recordFailure()
		return fmt.Errorf("dukcapil request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		c.cb.recordFailure()
		return fmt.Errorf("dukcapil server error: %d", resp.StatusCode)
	}

	c.cb.recordSuccess()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("dukcapil error: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// --- Circuit Breaker ---

type cbState int

const (
	cbClosed cbState = iota
	cbOpen
	cbHalfOpen
)

var ErrCircuitOpen = fmt.Errorf("circuit breaker is open")

type circuitBreaker struct {
	mu           sync.Mutex
	state        cbState
	failures     int
	threshold    int
	resetTimeout time.Duration
	lastFailure  time.Time
}

func newCircuitBreaker(threshold int, resetTimeout time.Duration) *circuitBreaker {
	return &circuitBreaker{
		state:        cbClosed,
		threshold:    threshold,
		resetTimeout: resetTimeout,
	}
}

func (cb *circuitBreaker) check() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case cbOpen:
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = cbHalfOpen
			return nil
		}
		return ErrCircuitOpen
	default:
		return nil
	}
}

func (cb *circuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.threshold {
		cb.state = cbOpen
	}
}

func (cb *circuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = cbClosed
}
```

- [ ] **Step 4: Run client tests**

Run: `cd services/identity && go test ./dukcapil/... -v -count=1`
Expected: PASS — 4 tests.

- [ ] **Step 5: Commit**

```bash
git add services/identity/dukcapil/
git commit -m "feat(identity): add Dukcapil HTTP client with circuit breaker and timeout"
```

---

## Task 7: Identity Service — OTP Service

**Files:**
- Create: `services/identity/otp/service.go`
- Create: `services/identity/otp/service_test.go`

- [ ] **Step 1: Add miniredis dependency**

Run: `cd services/identity && go get github.com/alicebob/miniredis/v2 github.com/redis/go-redis/v9 golang.org/x/crypto`

- [ ] **Step 2: Write OTP service tests**

```go
// services/identity/otp/service_test.go
package otp_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/garudapass/gpass/services/identity/otp"
	"github.com/redis/go-redis/v9"
)

func setupOTP(t *testing.T) (*otp.Service, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return otp.NewService(rdb), mr
}

func TestGenerateAndVerify(t *testing.T) {
	svc, _ := setupOTP(t)
	ctx := context.Background()

	code, err := svc.Generate(ctx, "reg-123", "phone")
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected 6-digit code, got %d chars", len(code))
	}

	// Correct code should pass
	err = svc.Verify(ctx, "reg-123", "phone", code)
	if err != nil {
		t.Errorf("Verify should pass for correct code: %v", err)
	}
}

func TestVerifyWrongCode(t *testing.T) {
	svc, _ := setupOTP(t)
	ctx := context.Background()

	svc.Generate(ctx, "reg-456", "phone")

	err := svc.Verify(ctx, "reg-456", "phone", "000000")
	if err == nil {
		t.Error("expected error for wrong code")
	}
}

func TestVerifyExpired(t *testing.T) {
	svc, mr := setupOTP(t)
	ctx := context.Background()

	svc.Generate(ctx, "reg-789", "email")

	// Fast-forward past expiry
	mr.FastForward(6 * 60 * 1000000000) // 6 minutes in nanoseconds

	err := svc.Verify(ctx, "reg-789", "email", "anything")
	if err == nil {
		t.Error("expected error for expired OTP")
	}
}

func TestVerifyMaxAttempts(t *testing.T) {
	svc, _ := setupOTP(t)
	ctx := context.Background()

	svc.Generate(ctx, "reg-max", "phone")

	// Exhaust attempts
	for i := 0; i < 3; i++ {
		svc.Verify(ctx, "reg-max", "phone", "wrong!")
	}

	// Even correct code should fail after max attempts
	err := svc.Verify(ctx, "reg-max", "phone", "correct")
	if err == nil {
		t.Error("expected error after max attempts exceeded")
	}
}
```

- [ ] **Step 3: Write OTP service implementation**

```go
// services/identity/otp/service.go
package otp

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

const (
	otpExpiry      = 5 * time.Minute
	maxAttempts    = 3
	maxSendsPerDay = 3
)

var (
	ErrOTPExpired     = fmt.Errorf("OTP expired or not found")
	ErrOTPInvalid     = fmt.Errorf("invalid OTP code")
	ErrMaxAttempts    = fmt.Errorf("maximum verification attempts exceeded")
	ErrMaxSends       = fmt.Errorf("maximum OTP sends per day exceeded")
)

// Service handles OTP generation, storage, and verification.
type Service struct {
	rdb *redis.Client
}

// NewService creates an OTP service backed by Redis.
func NewService(rdb *redis.Client) *Service {
	return &Service{rdb: rdb}
}

// Generate creates a 6-digit OTP, bcrypt-hashes it, and stores in Redis.
// Returns the plaintext code (for sending via SMS/email).
func (s *Service) Generate(ctx context.Context, registrationID, channel string) (string, error) {
	// Check send rate limit
	sendKey := fmt.Sprintf("gpass:otp:sends:%s:%s", registrationID, channel)
	sends, _ := s.rdb.Get(ctx, sendKey).Int()
	if sends >= maxSendsPerDay {
		return "", ErrMaxSends
	}

	code, err := generateCode()
	if err != nil {
		return "", err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	key := fmt.Sprintf("gpass:otp:%s:%s", registrationID, channel)
	pipe := s.rdb.Pipeline()
	pipe.Set(ctx, key, string(hash), otpExpiry)
	pipe.Set(ctx, key+":attempts", "0", otpExpiry)
	pipe.Incr(ctx, sendKey)
	pipe.Expire(ctx, sendKey, 24*time.Hour)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", err
	}

	return code, nil
}

// Verify checks the provided code against the stored OTP hash.
func (s *Service) Verify(ctx context.Context, registrationID, channel, code string) error {
	key := fmt.Sprintf("gpass:otp:%s:%s", registrationID, channel)
	attemptsKey := key + ":attempts"

	// Check attempts
	attempts, err := s.rdb.Get(ctx, attemptsKey).Int()
	if err != nil {
		return ErrOTPExpired
	}
	if attempts >= maxAttempts {
		s.rdb.Del(ctx, key, attemptsKey) // Invalidate OTP
		return ErrMaxAttempts
	}

	// Increment attempts
	s.rdb.Incr(ctx, attemptsKey)

	// Get hash
	hash, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return ErrOTPExpired
	}

	// Compare
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)); err != nil {
		return ErrOTPInvalid
	}

	// OTP verified — clean up
	s.rdb.Del(ctx, key, attemptsKey)
	return nil
}

func generateCode() (string, error) {
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
```

- [ ] **Step 4: Run OTP tests**

Run: `cd services/identity && go test ./otp/... -v -count=1`
Expected: PASS — 4 tests.

- [ ] **Step 5: Commit**

```bash
git add services/identity/otp/
git commit -m "feat(identity): add OTP service with bcrypt hashing, rate limiting, and max attempts"
```

---

## Task 8: Identity Service — Keycloak Admin API Client

**Files:**
- Create: `services/identity/keycloak/admin.go`
- Create: `services/identity/keycloak/admin_test.go`

- [ ] **Step 1: Write Keycloak admin client tests**

```go
// services/identity/keycloak/admin_test.go
package keycloak_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/identity/keycloak"
)

func TestCreateUser(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/realms/master/protocol/openid-connect/token":
			// Token endpoint
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "admin-token",
				"expires_in":   300,
			})
		case r.URL.Path == "/admin/realms/garudapass/users" && r.Method == http.MethodPost:
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.Header().Set("Location", "/admin/realms/garudapass/users/new-kc-user-id")
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, "admin", "admin", "garudapass")
	kcID, err := client.CreateUser(context.Background(), keycloak.CreateUserRequest{
		Username:  "3174011501900001",
		Email:     "budi@example.com",
		FirstName: "Budi",
		LastName:  "Santoso",
		Enabled:   true,
		Credentials: []keycloak.Credential{{
			Type:      "password",
			Value:     "securePassword123!",
			Temporary: false,
		}},
		Attributes: map[string][]string{
			"nik_masked":  {"************0001"},
			"auth_level":  {"2"},
			"verified":    {"true"},
		},
	})

	if err != nil {
		t.Fatalf("CreateUser error: %v", err)
	}
	if kcID == "" {
		t.Error("expected non-empty keycloak user ID")
	}
}

func TestCreateUserConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/realms/master/protocol/openid-connect/token" {
			json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "t", "expires_in": 300})
			return
		}
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"errorMessage":"User exists with same username"}`))
	}))
	defer server.Close()

	client := keycloak.NewAdminClient(server.URL, "admin", "admin", "garudapass")
	_, err := client.CreateUser(context.Background(), keycloak.CreateUserRequest{
		Username: "existing-user",
	})
	if err == nil {
		t.Error("expected error for duplicate user")
	}
}
```

- [ ] **Step 2: Write Keycloak admin client implementation**

```go
// services/identity/keycloak/admin.go
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// AdminClient communicates with the Keycloak Admin REST API.
type AdminClient struct {
	baseURL  string
	username string
	password string
	realm    string
	client   *http.Client

	mu          sync.Mutex
	accessToken string
	tokenExpiry time.Time
}

type CreateUserRequest struct {
	Username    string              `json:"username"`
	Email       string              `json:"email,omitempty"`
	FirstName   string              `json:"firstName,omitempty"`
	LastName    string              `json:"lastName,omitempty"`
	Enabled     bool                `json:"enabled"`
	Credentials []Credential        `json:"credentials,omitempty"`
	Attributes  map[string][]string `json:"attributes,omitempty"`
}

type Credential struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Temporary bool   `json:"temporary"`
}

func NewAdminClient(baseURL, username, password, realm string) *AdminClient {
	return &AdminClient{
		baseURL:  baseURL,
		username: username,
		password: password,
		realm:    realm,
		client:   &http.Client{Timeout: 5 * time.Second},
	}
}

// CreateUser creates a new user in the Keycloak realm.
// Returns the Keycloak user ID extracted from the Location header.
func (a *AdminClient) CreateUser(ctx context.Context, req CreateUserRequest) (string, error) {
	token, err := a.getToken(ctx)
	if err != nil {
		return "", fmt.Errorf("get admin token: %w", err)
	}

	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/admin/realms/%s/users", a.baseURL, a.realm),
		bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("create user request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return "", fmt.Errorf("user already exists in Keycloak")
	}
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak error %d: %s", resp.StatusCode, string(respBody))
	}

	// Extract user ID from Location header
	loc := resp.Header.Get("Location")
	parts := strings.Split(loc, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1], nil
	}
	return "", fmt.Errorf("missing Location header in response")
}

func (a *AdminClient) getToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.accessToken != "" && time.Now().Before(a.tokenExpiry) {
		return a.accessToken, nil
	}

	data := url.Values{}
	data.Set("grant_type", "password")
	data.Set("client_id", "admin-cli")
	data.Set("username", a.username)
	data.Set("password", a.password)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", a.baseURL),
		strings.NewReader(data.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	a.accessToken = tokenResp.AccessToken
	// Refresh 30 seconds before expiry
	a.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn-30) * time.Second)
	return a.accessToken, nil
}
```

- [ ] **Step 3: Run Keycloak client tests**

Run: `cd services/identity && go test ./keycloak/... -v -count=1`
Expected: PASS — 2 tests.

- [ ] **Step 4: Commit**

```bash
git add services/identity/keycloak/
git commit -m "feat(identity): add Keycloak Admin API client with token caching"
```

---

## Task 9: Identity Service — Registration Handlers

**Files:**
- Create: `services/identity/handler/register.go`
- Create: `services/identity/handler/register_test.go`

- [ ] **Step 1: Write registration handler tests**

```go
// services/identity/handler/register_test.go
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/identity/dukcapil"
	"github.com/garudapass/gpass/services/identity/handler"
)

// mockDukcapilClient implements the DukcapilVerifier interface for testing.
type mockDukcapilClient struct {
	nikResp       *dukcapil.NIKVerifyResponse
	biometricResp *dukcapil.BiometricResponse
	err           error
}

func (m *mockDukcapilClient) VerifyNIK(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error) {
	return m.nikResp, m.err
}

func (m *mockDukcapilClient) VerifyBiometric(ctx context.Context, nik, selfie string) (*dukcapil.BiometricResponse, error) {
	return m.biometricResp, m.err
}

func (m *mockDukcapilClient) VerifyDemographic(ctx context.Context, req dukcapil.DemographicRequest) (*dukcapil.DemographicResponse, error) {
	return &dukcapil.DemographicResponse{Match: true, Confidence: 0.95}, m.err
}

// mockOTPService implements the OTPGenerator interface for testing.
type mockOTPService struct {
	code string
}

func (m *mockOTPService) Generate(ctx context.Context, regID, channel string) (string, error) {
	m.code = "123456"
	return m.code, nil
}

func (m *mockOTPService) Verify(ctx context.Context, regID, channel, code string) error {
	if code == m.code {
		return nil
	}
	return fmt.Errorf("invalid OTP")
}

// mockKeycloakAdmin implements the KeycloakUserCreator interface for testing.
type mockKeycloakAdmin struct{}

func (m *mockKeycloakAdmin) CreateUser(ctx context.Context, req interface{}) (string, error) {
	return "kc-user-id-123", nil
}

func TestInitiateRegistration(t *testing.T) {
	deps := handler.RegisterDeps{
		Dukcapil: &mockDukcapilClient{
			nikResp: &dukcapil.NIKVerifyResponse{Valid: true, Alive: true},
		},
		OTP:    &mockOTPService{},
		NIKKey: make([]byte, 32),
	}
	h := handler.NewRegisterHandler(deps)

	body, _ := json.Marshal(map[string]string{
		"nik":   "3174011501900001",
		"phone": "+6281234567890",
		"email": "budi@example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["registration_id"] == nil {
		t.Error("expected registration_id in response")
	}
}

func TestInitiateRegistrationInvalidNIK(t *testing.T) {
	deps := handler.RegisterDeps{
		Dukcapil: &mockDukcapilClient{},
		OTP:      &mockOTPService{},
		NIKKey:   make([]byte, 32),
	}
	h := handler.NewRegisterHandler(deps)

	body, _ := json.Marshal(map[string]string{
		"nik":   "invalid",
		"phone": "+6281234567890",
		"email": "test@example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestInitiateRegistrationDeceasedNIK(t *testing.T) {
	deps := handler.RegisterDeps{
		Dukcapil: &mockDukcapilClient{
			nikResp: &dukcapil.NIKVerifyResponse{Valid: true, Alive: false},
		},
		OTP:    &mockOTPService{},
		NIKKey: make([]byte, 32),
	}
	h := handler.NewRegisterHandler(deps)

	body, _ := json.Marshal(map[string]string{
		"nik":   "3174010506600099",
		"phone": "+6281234567890",
		"email": "test@example.com",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/register/initiate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Initiate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for deceased NIK, got %d: %s", w.Code, w.Body.String())
	}
}
```

Note: The `fmt` import will be needed in the test file — add it to the imports:

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/identity/dukcapil"
	"github.com/garudapass/gpass/services/identity/handler"
)
```

- [ ] **Step 2: Write registration handler implementation**

```go
// services/identity/handler/register.go
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/garudapass/gpass/services/identity/crypto"
	"github.com/garudapass/gpass/services/identity/dukcapil"
)

// DukcapilVerifier abstracts Dukcapil API calls for testing.
type DukcapilVerifier interface {
	VerifyNIK(ctx context.Context, nik string) (*dukcapil.NIKVerifyResponse, error)
	VerifyBiometric(ctx context.Context, nik, selfie string) (*dukcapil.BiometricResponse, error)
	VerifyDemographic(ctx context.Context, req dukcapil.DemographicRequest) (*dukcapil.DemographicResponse, error)
}

// OTPGenerator abstracts OTP operations for testing.
type OTPGenerator interface {
	Generate(ctx context.Context, regID, channel string) (string, error)
	Verify(ctx context.Context, regID, channel, code string) error
}

// RegisterDeps holds the dependencies for the registration handler.
type RegisterDeps struct {
	Dukcapil DukcapilVerifier
	OTP      OTPGenerator
	NIKKey   []byte
}

type RegisterHandler struct {
	deps RegisterDeps
}

func NewRegisterHandler(deps RegisterDeps) *RegisterHandler {
	return &RegisterHandler{deps: deps}
}

type InitiateRequest struct {
	NIK   string `json:"nik"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

type InitiateResponse struct {
	RegistrationID string `json:"registration_id"`
	OTPExpiresAt   string `json:"otp_expires_at"`
}

// Initiate starts the registration flow: validate NIK → Dukcapil verify → send OTPs.
func (h *RegisterHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	var req InitiateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	// Validate NIK format
	if err := crypto.ValidateNIKFormat(req.NIK); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_nik", err.Error())
		return
	}

	// Validate phone and email present
	if req.Phone == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "Phone and email are required")
		return
	}

	// Verify NIK with Dukcapil
	nikResp, err := h.deps.Dukcapil.VerifyNIK(r.Context(), req.NIK)
	if err != nil {
		slog.Error("dukcapil verify failed", "error", err)
		writeError(w, http.StatusBadGateway, "dukcapil_unavailable", "Identity verification service unavailable")
		return
	}

	if !nikResp.Valid {
		writeError(w, http.StatusBadRequest, "nik_not_found", "NIK not found in population database")
		return
	}

	if !nikResp.Alive {
		writeError(w, http.StatusBadRequest, "nik_deceased", "NIK is registered as deceased")
		return
	}

	// Generate registration ID
	regID := crypto.TokenizeNIK(req.NIK+req.Phone, h.deps.NIKKey)[:32]

	// Send OTPs
	_, err = h.deps.OTP.Generate(r.Context(), regID, "phone")
	if err != nil {
		slog.Error("phone OTP generation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "otp_error", "Failed to send phone OTP")
		return
	}

	_, err = h.deps.OTP.Generate(r.Context(), regID, "email")
	if err != nil {
		slog.Error("email OTP generation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "otp_error", "Failed to send email OTP")
		return
	}

	writeJSON(w, http.StatusOK, InitiateResponse{
		RegistrationID: regID,
		OTPExpiresAt:   "5m",
	})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q,"message":%q}`, code, message)
}
```

- [ ] **Step 3: Run registration handler tests**

Run: `cd services/identity && go test ./handler/... -v -count=1`
Expected: PASS — 3 tests.

- [ ] **Step 4: Commit**

```bash
git add services/identity/handler/
git commit -m "feat(identity): add registration initiate handler with NIK validation and Dukcapil verification"
```

---

## Task 10: Identity Service — Main Server + Dockerfile

**Files:**
- Create: `services/identity/main.go`
- Create: `services/identity/Dockerfile`

- [ ] **Step 1: Write main.go**

```go
// services/identity/main.go
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/garudapass/gpass/services/identity/config"
	dkclient "github.com/garudapass/gpass/services/identity/dukcapil"
	"github.com/garudapass/gpass/services/identity/handler"
	"github.com/garudapass/gpass/services/identity/otp"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	// Redis for OTPs
	redisOpt, err := redis.ParseURL(cfg.OTPRedisURL)
	if err != nil {
		slog.Error("invalid redis URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(redisOpt)
	defer rdb.Close()

	// Dukcapil client
	dukcapilClient := dkclient.NewClient(cfg.DukcapilURL, cfg.DukcapilAPIKey, cfg.DukcapilTimeout)

	// OTP service
	otpSvc := otp.NewService(rdb)

	// Registration handler
	regHandler := handler.NewRegisterHandler(handler.RegisterDeps{
		Dukcapil: dukcapilClient,
		OTP:      otpSvc,
		NIKKey:   cfg.NIKKey,
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/register/initiate", regHandler.Initiate)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"identity"}`))
	})

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    10 << 20, // 10MB for selfie uploads
	}

	slog.Info("identity service listening", "port", cfg.Port, "dukcapil_mode", cfg.DukcapilMode)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Write Dockerfile**

```dockerfile
# services/identity/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /identity .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 -S app && adduser -u 1000 -S app -G app
COPY --from=builder /identity /usr/local/bin/identity
USER app:app
EXPOSE 4001
CMD ["/usr/local/bin/identity"]
```

- [ ] **Step 3: Tidy and verify compilation**

Run: `cd services/identity && go mod tidy && go build -o /dev/null .`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add services/identity/main.go services/identity/Dockerfile services/identity/go.mod services/identity/go.sum
git commit -m "feat(identity): add main server and Dockerfile"
```

---

## Task 11: GarudaInfo Service — Scaffold, Config, Consent Store

This task creates the GarudaInfo service with consent storage. Since the consent store requires PostgreSQL, we'll use an in-memory implementation for unit tests (same pattern as the BFF session store).

**Files:**
- Create: `services/garudainfo/go.mod`
- Create: `services/garudainfo/config/config.go`
- Create: `services/garudainfo/config/config_test.go`
- Create: `services/garudainfo/store/consent.go`
- Create: `services/garudainfo/store/consent_test.go`

- [ ] **Step 1: Initialize Go module**

Run: `mkdir -p services/garudainfo && cd services/garudainfo && go mod init github.com/garudapass/gpass/services/garudainfo`

- [ ] **Step 2: Write config**

```go
// services/garudainfo/config/config.go
package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port               string
	DatabaseURL        string
	IdentityServiceURL string
	KafkaBrokers       string
	RedisURL           string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("GARUDAINFO_PORT", "4003"),
		DatabaseURL:        os.Getenv("GARUDAINFO_DB_URL"),
		IdentityServiceURL: os.Getenv("IDENTITY_SERVICE_URL"),
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:19092"),
		RedisURL:           os.Getenv("OTP_REDIS_URL"),
	}

	required := []struct{ name, val string }{
		{"GARUDAINFO_DB_URL", cfg.DatabaseURL},
		{"IDENTITY_SERVICE_URL", cfg.IdentityServiceURL},
	}
	var missing []string
	for _, r := range required {
		if r.val == "" {
			missing = append(missing, r.name)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("required env vars not set: %s", strings.Join(missing, ", "))
	}

	if _, err := url.ParseRequestURI(cfg.IdentityServiceURL); err != nil {
		return nil, fmt.Errorf("invalid IDENTITY_SERVICE_URL: %w", err)
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 3: Write consent store interface and in-memory implementation**

```go
// services/garudainfo/store/consent.go
package store

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrConsentNotFound = fmt.Errorf("consent not found")
	ErrConsentRevoked  = fmt.Errorf("consent already revoked")
)

// Consent represents a user's consent for data sharing.
type Consent struct {
	ID              string            `json:"id"`
	UserID          string            `json:"user_id"`
	ClientID        string            `json:"client_id"`
	ClientName      string            `json:"client_name"`
	Purpose         string            `json:"purpose"`
	Fields          map[string]bool   `json:"fields"`
	DurationSeconds int64             `json:"duration_seconds"`
	GrantedAt       time.Time         `json:"granted_at"`
	ExpiresAt       time.Time         `json:"expires_at"`
	RevokedAt       *time.Time        `json:"revoked_at,omitempty"`
	Status          string            `json:"status"` // ACTIVE, EXPIRED, REVOKED
}

// ConsentStore persists consent records.
type ConsentStore interface {
	Create(ctx context.Context, consent *Consent) error
	GetByID(ctx context.Context, id string) (*Consent, error)
	ListByUser(ctx context.Context, userID string) ([]*Consent, error)
	ListActiveByUserAndClient(ctx context.Context, userID, clientID string) ([]*Consent, error)
	Revoke(ctx context.Context, id string) error
	ExpireStale(ctx context.Context) (int, error)
}

// --- In-memory implementation for testing ---

type InMemoryConsentStore struct {
	mu       sync.RWMutex
	consents map[string]*Consent
}

func NewInMemoryConsentStore() *InMemoryConsentStore {
	return &InMemoryConsentStore{consents: make(map[string]*Consent)}
}

func (s *InMemoryConsentStore) Create(_ context.Context, consent *Consent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if consent.ID == "" {
		consent.ID = uuid.New().String()
	}
	consent.Status = "ACTIVE"
	consent.GrantedAt = time.Now()
	consent.ExpiresAt = consent.GrantedAt.Add(time.Duration(consent.DurationSeconds) * time.Second)
	cp := *consent
	s.consents[consent.ID] = &cp
	return nil
}

func (s *InMemoryConsentStore) GetByID(_ context.Context, id string) (*Consent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.consents[id]
	if !ok {
		return nil, ErrConsentNotFound
	}
	cp := *c
	return &cp, nil
}

func (s *InMemoryConsentStore) ListByUser(_ context.Context, userID string) ([]*Consent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Consent
	for _, c := range s.consents {
		if c.UserID == userID {
			cp := *c
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *InMemoryConsentStore) ListActiveByUserAndClient(_ context.Context, userID, clientID string) ([]*Consent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Consent
	for _, c := range s.consents {
		if c.UserID == userID && c.ClientID == clientID && c.Status == "ACTIVE" {
			cp := *c
			result = append(result, &cp)
		}
	}
	return result, nil
}

func (s *InMemoryConsentStore) Revoke(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.consents[id]
	if !ok {
		return ErrConsentNotFound
	}
	if c.Status == "REVOKED" {
		return ErrConsentRevoked
	}
	now := time.Now()
	c.Status = "REVOKED"
	c.RevokedAt = &now
	return nil
}

func (s *InMemoryConsentStore) ExpireStale(_ context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	now := time.Now()
	for _, c := range s.consents {
		if c.Status == "ACTIVE" && now.After(c.ExpiresAt) {
			c.Status = "EXPIRED"
			count++
		}
	}
	return count, nil
}
```

- [ ] **Step 4: Add uuid dependency and write consent store tests**

Run: `cd services/garudainfo && go get github.com/google/uuid`

```go
// services/garudainfo/store/consent_test.go
package store_test

import (
	"context"
	"testing"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

func TestConsentCreateAndGet(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	ctx := context.Background()

	consent := &store.Consent{
		UserID:          "user-1",
		ClientID:        "app-1",
		ClientName:      "Test App",
		Purpose:         "KYC verification",
		Fields:          map[string]bool{"name": true, "dob": true, "address": false},
		DurationSeconds: 86400, // 1 day
	}

	err := s.Create(ctx, consent)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if consent.ID == "" {
		t.Fatal("expected ID to be set")
	}
	if consent.Status != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %s", consent.Status)
	}

	got, err := s.GetByID(ctx, consent.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Purpose != "KYC verification" {
		t.Errorf("expected KYC verification, got %s", got.Purpose)
	}
	if !got.Fields["name"] {
		t.Error("expected name field to be true")
	}
	if got.Fields["address"] {
		t.Error("expected address field to be false")
	}
}

func TestConsentRevoke(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	ctx := context.Background()

	consent := &store.Consent{
		UserID: "user-1", ClientID: "app-1", ClientName: "Test",
		Purpose: "test", Fields: map[string]bool{"name": true}, DurationSeconds: 86400,
	}
	s.Create(ctx, consent)

	err := s.Revoke(ctx, consent.ID)
	if err != nil {
		t.Fatalf("Revoke failed: %v", err)
	}

	got, _ := s.GetByID(ctx, consent.ID)
	if got.Status != "REVOKED" {
		t.Errorf("expected REVOKED, got %s", got.Status)
	}
	if got.RevokedAt == nil {
		t.Error("expected RevokedAt to be set")
	}
}

func TestConsentRevokeAlreadyRevoked(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	ctx := context.Background()

	consent := &store.Consent{
		UserID: "user-1", ClientID: "app-1", ClientName: "Test",
		Purpose: "test", Fields: map[string]bool{"name": true}, DurationSeconds: 86400,
	}
	s.Create(ctx, consent)
	s.Revoke(ctx, consent.ID)

	err := s.Revoke(ctx, consent.ID)
	if err != store.ErrConsentRevoked {
		t.Errorf("expected ErrConsentRevoked, got %v", err)
	}
}

func TestConsentListByUser(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	ctx := context.Background()

	for _, uid := range []string{"user-1", "user-1", "user-2"} {
		s.Create(ctx, &store.Consent{
			UserID: uid, ClientID: "app-1", ClientName: "Test",
			Purpose: "test", Fields: map[string]bool{"name": true}, DurationSeconds: 86400,
		})
	}

	list, _ := s.ListByUser(ctx, "user-1")
	if len(list) != 2 {
		t.Errorf("expected 2 consents for user-1, got %d", len(list))
	}
}

func TestConsentNotFound(t *testing.T) {
	s := store.NewInMemoryConsentStore()
	_, err := s.GetByID(context.Background(), "nonexistent")
	if err != store.ErrConsentNotFound {
		t.Errorf("expected ErrConsentNotFound, got %v", err)
	}
}
```

- [ ] **Step 5: Run consent store tests**

Run: `cd services/garudainfo && go test ./... -v -count=1`
Expected: PASS — 5 tests.

- [ ] **Step 6: Commit**

```bash
git add services/garudainfo/
git commit -m "feat(garudainfo): scaffold with config, consent store interface, and in-memory implementation"
```

---

## Task 12: GarudaInfo Service — Consent and Person Handlers

**Files:**
- Create: `services/garudainfo/handler/consent.go`
- Create: `services/garudainfo/handler/consent_test.go`
- Create: `services/garudainfo/handler/person.go`
- Create: `services/garudainfo/handler/person_test.go`

- [ ] **Step 1: Write consent handler tests**

```go
// services/garudainfo/handler/consent_test.go
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudainfo/handler"
	"github.com/garudapass/gpass/services/garudainfo/store"
)

func TestGrantConsent(t *testing.T) {
	consentStore := store.NewInMemoryConsentStore()
	h := handler.NewConsentHandler(consentStore)

	body, _ := json.Marshal(handler.GrantConsentRequest{
		UserID:       "user-1",
		ClientID:     "app-1",
		ClientName:   "Test App",
		Purpose:      "KYC verification",
		Fields:       []string{"name", "dob", "phone"},
		DurationDays: 30,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/garudainfo/consents", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Grant(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.GrantConsentResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ConsentID == "" {
		t.Error("expected consent_id")
	}
}

func TestListConsents(t *testing.T) {
	consentStore := store.NewInMemoryConsentStore()
	consentStore.Create(nil, &store.Consent{
		UserID: "user-1", ClientID: "app-1", ClientName: "App",
		Purpose: "test", Fields: map[string]bool{"name": true}, DurationSeconds: 86400,
	})

	h := handler.NewConsentHandler(consentStore)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/consents?user_id=user-1", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp handler.ListConsentsResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Consents) != 1 {
		t.Errorf("expected 1 consent, got %d", len(resp.Consents))
	}
}

func TestRevokeConsent(t *testing.T) {
	consentStore := store.NewInMemoryConsentStore()
	consent := &store.Consent{
		UserID: "user-1", ClientID: "app-1", ClientName: "App",
		Purpose: "test", Fields: map[string]bool{"name": true}, DurationSeconds: 86400,
	}
	consentStore.Create(nil, consent)

	h := handler.NewConsentHandler(consentStore)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/garudainfo/consents/"+consent.ID, nil)
	req.SetPathValue("id", consent.ID)
	w := httptest.NewRecorder()

	h.Revoke(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	got, _ := consentStore.GetByID(nil, consent.ID)
	if got.Status != "REVOKED" {
		t.Errorf("expected REVOKED, got %s", got.Status)
	}
}
```

- [ ] **Step 2: Write consent handler implementation**

```go
// services/garudainfo/handler/consent.go
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

type ConsentHandler struct {
	store store.ConsentStore
}

func NewConsentHandler(s store.ConsentStore) *ConsentHandler {
	return &ConsentHandler{store: s}
}

type GrantConsentRequest struct {
	UserID       string   `json:"user_id"`
	ClientID     string   `json:"client_id"`
	ClientName   string   `json:"client_name"`
	Purpose      string   `json:"purpose"`
	Fields       []string `json:"fields"`
	DurationDays int      `json:"duration_days"`
}

type GrantConsentResponse struct {
	ConsentID string `json:"consent_id"`
	ExpiresAt string `json:"expires_at"`
}

type ListConsentsResponse struct {
	Consents []*store.Consent `json:"consents"`
}

func (h *ConsentHandler) Grant(w http.ResponseWriter, r *http.Request) {
	var req GrantConsentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid request body")
		return
	}

	if req.UserID == "" || req.ClientID == "" || len(req.Fields) == 0 {
		writeError(w, http.StatusBadRequest, "missing_fields", "user_id, client_id, and fields are required")
		return
	}

	fields := make(map[string]bool)
	for _, f := range req.Fields {
		fields[f] = true
	}

	consent := &store.Consent{
		UserID:          req.UserID,
		ClientID:        req.ClientID,
		ClientName:      req.ClientName,
		Purpose:         req.Purpose,
		Fields:          fields,
		DurationSeconds: int64(req.DurationDays) * 86400,
	}

	if err := h.store.Create(r.Context(), consent); err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "Failed to create consent")
		return
	}

	writeJSON(w, http.StatusCreated, GrantConsentResponse{
		ConsentID: consent.ID,
		ExpiresAt: consent.ExpiresAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *ConsentHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user_id", "user_id query parameter required")
		return
	}

	consents, err := h.store.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", "Failed to list consents")
		return
	}
	if consents == nil {
		consents = []*store.Consent{}
	}

	writeJSON(w, http.StatusOK, ListConsentsResponse{Consents: consents})
}

func (h *ConsentHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "consent ID required")
		return
	}

	if err := h.store.Revoke(r.Context(), id); err != nil {
		if err == store.ErrConsentNotFound {
			writeError(w, http.StatusNotFound, "not_found", "Consent not found")
			return
		}
		if err == store.ErrConsentRevoked {
			writeError(w, http.StatusConflict, "already_revoked", "Consent already revoked")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", "Failed to revoke consent")
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q,"message":%q}`, code, message)
}
```

- [ ] **Step 3: Write person data handler tests**

```go
// services/garudainfo/handler/person_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudainfo/handler"
	"github.com/garudapass/gpass/services/garudainfo/store"
)

func TestGetPersonFiltersFieldsByConsent(t *testing.T) {
	consentStore := store.NewInMemoryConsentStore()
	consent := &store.Consent{
		UserID: "user-1", ClientID: "app-1", ClientName: "App",
		Purpose:         "kyc",
		Fields:          map[string]bool{"name": true, "dob": true, "address": false},
		DurationSeconds: 86400,
	}
	consentStore.Create(nil, consent)

	// Mock user data provider
	userData := map[string]handler.FieldValue{
		"name":    {Value: "Budi Santoso", Source: "dukcapil", LastVerified: "2026-04-06"},
		"dob":     {Value: "1990-01-15", Source: "dukcapil", LastVerified: "2026-04-06"},
		"address": {Value: "Jl. Sudirman No. 1", Source: "dukcapil", LastVerified: "2026-04-06"},
		"phone":   {Value: "+6281234567890", Source: "user", LastVerified: "2026-04-06"},
	}

	h := handler.NewPersonHandler(consentStore, &mockUserDataProvider{data: userData})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/garudainfo/person?consent_id="+consent.ID, nil)
	w := httptest.NewRecorder()

	h.GetPerson(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp handler.PersonResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if _, ok := resp.Fields["name"]; !ok {
		t.Error("expected name field (consented)")
	}
	if _, ok := resp.Fields["dob"]; !ok {
		t.Error("expected dob field (consented)")
	}
	if _, ok := resp.Fields["address"]; ok {
		t.Error("address should NOT be present (consent=false)")
	}
	if _, ok := resp.Fields["phone"]; ok {
		t.Error("phone should NOT be present (not in consent)")
	}
}

type mockUserDataProvider struct {
	data map[string]handler.FieldValue
}

func (m *mockUserDataProvider) GetUserFields(_ string) map[string]handler.FieldValue {
	return m.data
}
```

- [ ] **Step 4: Write person handler implementation**

```go
// services/garudainfo/handler/person.go
package handler

import (
	"net/http"

	"github.com/garudapass/gpass/services/garudainfo/store"
)

type FieldValue struct {
	Value        string `json:"value"`
	Source       string `json:"source"`
	LastVerified string `json:"last_verified"`
}

type PersonResponse struct {
	Fields map[string]FieldValue `json:"fields"`
}

// UserDataProvider abstracts access to user profile data.
type UserDataProvider interface {
	GetUserFields(userID string) map[string]FieldValue
}

type PersonHandler struct {
	consentStore store.ConsentStore
	userData     UserDataProvider
}

func NewPersonHandler(cs store.ConsentStore, ud UserDataProvider) *PersonHandler {
	return &PersonHandler{consentStore: cs, userData: ud}
}

// GetPerson returns verified user data filtered by the consent's granted fields.
func (h *PersonHandler) GetPerson(w http.ResponseWriter, r *http.Request) {
	consentID := r.URL.Query().Get("consent_id")
	if consentID == "" {
		writeError(w, http.StatusBadRequest, "missing_consent_id", "consent_id query parameter required")
		return
	}

	consent, err := h.consentStore.GetByID(r.Context(), consentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "consent_not_found", "Consent not found")
		return
	}

	if consent.Status != "ACTIVE" {
		writeError(w, http.StatusForbidden, "consent_inactive", "Consent is no longer active")
		return
	}

	allFields := h.userData.GetUserFields(consent.UserID)

	// Filter: only return fields that were explicitly consented
	filtered := make(map[string]FieldValue)
	for fieldName, granted := range consent.Fields {
		if granted {
			if val, ok := allFields[fieldName]; ok {
				filtered[fieldName] = val
			}
		}
	}

	writeJSON(w, http.StatusOK, PersonResponse{Fields: filtered})
}
```

- [ ] **Step 5: Run all GarudaInfo tests**

Run: `cd services/garudainfo && go mod tidy && go test ./... -v -count=1`
Expected: PASS — 6+ tests across store and handler packages.

- [ ] **Step 6: Commit**

```bash
git add services/garudainfo/
git commit -m "feat(garudainfo): add consent and person handlers with field-level consent filtering"
```

---

## Task 13: GarudaInfo Service — Main Server + Dockerfile

**Files:**
- Create: `services/garudainfo/main.go`
- Create: `services/garudainfo/Dockerfile`

- [ ] **Step 1: Write main.go**

```go
// services/garudainfo/main.go
package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/garudapass/gpass/services/garudainfo/config"
	"github.com/garudapass/gpass/services/garudainfo/handler"
	"github.com/garudapass/gpass/services/garudainfo/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	// TODO: Replace with PostgreSQL store in production
	consentStore := store.NewInMemoryConsentStore()

	// TODO: Replace with real identity service client
	var userData handler.UserDataProvider // nil for now — will be wired in integration

	consentHandler := handler.NewConsentHandler(consentStore)
	personHandler := handler.NewPersonHandler(consentStore, userData)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/garudainfo/consents", consentHandler.Grant)
	mux.HandleFunc("GET /api/v1/garudainfo/consents", consentHandler.List)
	mux.HandleFunc("DELETE /api/v1/garudainfo/consents/{id}", consentHandler.Revoke)
	mux.HandleFunc("GET /api/v1/garudainfo/person", personHandler.GetPerson)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"garudainfo"}`))
	})

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	slog.Info("garudainfo service listening", "port", cfg.Port)
	if err := server.ListenAndServe(); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Write Dockerfile**

```dockerfile
# services/garudainfo/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /garudainfo .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -g 1000 -S app && adduser -u 1000 -S app -G app
COPY --from=builder /garudainfo /usr/local/bin/garudainfo
USER app:app
EXPOSE 4003
CMD ["/usr/local/bin/garudainfo"]
```

- [ ] **Step 3: Verify compilation**

Run: `cd services/garudainfo && go mod tidy && go build -o /dev/null .`
Expected: No errors.

- [ ] **Step 4: Commit**

```bash
git add services/garudainfo/main.go services/garudainfo/Dockerfile services/garudainfo/go.mod services/garudainfo/go.sum
git commit -m "feat(garudainfo): add main server and Dockerfile"
```

---

## Task 14: Integration — Go Workspace, Docker Compose, .env

**Files:**
- Modify: `go.work`
- Modify: `docker-compose.yml`
- Modify: `.env.example`
- Create: `infrastructure/db/migrations/001_create_users.sql`
- Create: `infrastructure/db/migrations/002_create_consents.sql`

- [ ] **Step 1: Update go.work**

```
go 1.22.2

use (
    ./apps/bff
    ./services/identity
    ./services/garudainfo
    ./services/dukcapil-sim
)
```

- [ ] **Step 2: Add SQL migrations**

```sql
-- infrastructure/db/migrations/001_create_users.sql
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    keycloak_id UUID UNIQUE NOT NULL,
    nik_token VARCHAR(64) UNIQUE NOT NULL,
    nik_masked VARCHAR(20) NOT NULL,
    name_enc BYTEA NOT NULL,
    dob_enc BYTEA NOT NULL,
    gender VARCHAR(1) NOT NULL,
    phone_hash VARCHAR(64) NOT NULL,
    phone_enc BYTEA NOT NULL,
    email_hash VARCHAR(64) NOT NULL,
    email_enc BYTEA NOT NULL,
    address_enc BYTEA,
    wrapped_dek BYTEA NOT NULL,
    auth_level SMALLINT NOT NULL DEFAULT 0,
    verification_status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    dukcapil_verified_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_nik_token ON users(nik_token);
CREATE INDEX IF NOT EXISTS idx_users_phone_hash ON users(phone_hash);
CREATE INDEX IF NOT EXISTS idx_users_email_hash ON users(email_hash);
CREATE INDEX IF NOT EXISTS idx_users_keycloak_id ON users(keycloak_id);
```

```sql
-- infrastructure/db/migrations/002_create_consents.sql
CREATE TABLE IF NOT EXISTS consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    client_id VARCHAR(100) NOT NULL,
    client_name VARCHAR(255) NOT NULL,
    purpose VARCHAR(255) NOT NULL,
    fields JSONB NOT NULL,
    duration_seconds BIGINT NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_consents_user_id ON consents(user_id);
CREATE INDEX IF NOT EXISTS idx_consents_client_id ON consents(client_id);
CREATE INDEX IF NOT EXISTS idx_consents_status ON consents(status);
CREATE INDEX IF NOT EXISTS idx_consents_expires_at ON consents(expires_at) WHERE status = 'ACTIVE';

CREATE TABLE IF NOT EXISTS consent_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consent_id UUID NOT NULL REFERENCES consents(id),
    action VARCHAR(20) NOT NULL,
    actor_id UUID,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_consent_audit_consent_id ON consent_audit_log(consent_id);
CREATE INDEX IF NOT EXISTS idx_consent_audit_created_at ON consent_audit_log(created_at);
```

- [ ] **Step 3: Add new services to docker-compose.yml**

Append to the `services:` section in `docker-compose.yml`:

```yaml
  dukcapil-sim:
    build: ./services/dukcapil-sim
    restart: unless-stopped
    ports:
      - "4002:4002"
    environment:
      DUKCAPIL_SIM_PORT: "4002"
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

  identity:
    build: ./services/identity
    restart: unless-stopped
    ports:
      - "4001:4001"
    env_file: .env
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      keycloak:
        condition: service_started
      dukcapil-sim:
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

  garudainfo:
    build: ./services/garudainfo
    restart: unless-stopped
    ports:
      - "4003:4003"
    env_file: .env
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
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

- [ ] **Step 4: Update .env.example with new env vars**

Append to `.env.example`:

```bash
# Identity Service
IDENTITY_PORT=4001
DUKCAPIL_MODE=simulator
DUKCAPIL_URL=http://localhost:4002
DUKCAPIL_API_KEY=
DUKCAPIL_TIMEOUT=10s
SERVER_NIK_KEY=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
FIELD_ENCRYPTION_KEY=fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210
KEYCLOAK_ADMIN_URL=http://localhost:8080
KEYCLOAK_ADMIN_USER=admin
KEYCLOAK_ADMIN_PASSWORD=admin

# GarudaInfo Service
GARUDAINFO_PORT=4003
GARUDAINFO_DB_URL=postgres://garudapass:garudapass@localhost:5432/garudapass
IDENTITY_SERVICE_URL=http://localhost:4001

# Dukcapil Simulator
DUKCAPIL_SIM_PORT=4002
```

- [ ] **Step 5: Verify all services compile**

Run: `cd /opt/gpass && go work sync && cd services/dukcapil-sim && go build -o /dev/null . && cd ../identity && go build -o /dev/null . && cd ../garudainfo && go build -o /dev/null . && echo "ALL BUILD OK"`
Expected: `ALL BUILD OK`

- [ ] **Step 6: Verify Docker Compose is valid**

Run: `docker compose config --quiet && echo "Docker Compose: Valid"`

- [ ] **Step 7: Run all tests across all services**

Run: `cd /opt/gpass && cd apps/bff && go test ./... -count=1 && cd ../../services/identity && go test ./... -count=1 && cd ../garudainfo && go test ./... -count=1 && cd ../dukcapil-sim && go test ./... -count=1 && echo "ALL TESTS PASS"`

- [ ] **Step 8: Commit**

```bash
git add go.work docker-compose.yml .env.example infrastructure/db/ services/*/go.mod services/*/go.sum
git commit -m "feat: integrate Phase 2 services into monorepo with Docker Compose and migrations"
```

---

## Summary

| Task | Component | Tests | Status |
|------|-----------|-------|--------|
| 1 | Dukcapil simulator — test data | — | ☐ |
| 2 | Dukcapil simulator — verify handlers | 6 unit tests | ☐ |
| 3 | Dukcapil simulator — server + Dockerfile | Compile check | ☐ |
| 4 | Identity — config + NIK crypto | 7+ unit tests | ☐ |
| 5 | Identity — field encryption (DEK/KEK) | 5 unit tests | ☐ |
| 6 | Identity — Dukcapil client + circuit breaker | 4 unit tests | ☐ |
| 7 | Identity — OTP service | 4 unit tests | ☐ |
| 8 | Identity — Keycloak admin client | 2 unit tests | ☐ |
| 9 | Identity — registration handlers | 3 unit tests | ☐ |
| 10 | Identity — server + Dockerfile | Compile check | ☐ |
| 11 | GarudaInfo — config + consent store | 5 unit tests | ☐ |
| 12 | GarudaInfo — consent + person handlers | 4 unit tests | ☐ |
| 13 | GarudaInfo — server + Dockerfile | Compile check | ☐ |
| 14 | Integration — workspace, compose, migrations | Build + compose check | ☐ |

**Total estimated tests:** ~40+ across 3 services
**Coverage target:** ≥ 80% per package
