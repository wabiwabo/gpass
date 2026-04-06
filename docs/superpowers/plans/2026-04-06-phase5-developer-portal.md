# Phase 5: Developer Portal & API Gateway — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a developer portal service (GarudaPortal) to GarudaPass — enabling app registration, API key lifecycle, usage metering, and webhook delivery for third-party integrations.

**Architecture:** GarudaPortal (Go) manages developer apps, API keys (SHA-256 hashed, plaintext shown once), webhook subscriptions with HMAC-SHA256 signatures, and per-app usage counters. Keys are validated via an internal endpoint for Kong gateway integration. All stores are in-memory for MVP.

**Tech Stack:** Go 1.25, crypto/rand, crypto/sha256, crypto/hmac, encoding/base64, net/http, log/slog, httptest (test)

---

## File Structure

```
services/
└── garudaportal/
    ├── go.mod
    ├── main.go
    ├── Dockerfile
    ├── config/
    │   ├── config.go
    │   └── config_test.go
    ├── apikey/
    │   ├── generator.go          # Key generation (crypto/rand + base62 + prefix)
    │   ├── generator_test.go
    │   ├── validator.go           # Key validation (hash lookup + status + rate limit)
    │   └── validator_test.go
    ├── store/
    │   ├── app.go                 # AppStore interface + InMemory
    │   ├── app_test.go
    │   ├── key.go                 # KeyStore interface + InMemory
    │   ├── key_test.go
    │   ├── webhook.go             # WebhookStore interface + InMemory
    │   ├── webhook_test.go
    │   ├── delivery.go            # DeliveryStore interface + InMemory
    │   ├── delivery_test.go
    │   ├── usage.go               # UsageStore interface + InMemory
    │   └── usage_test.go
    ├── webhook/
    │   ├── signer.go              # HMAC-SHA256 signature computation
    │   ├── signer_test.go
    │   ├── dispatcher.go          # HTTP delivery with retry scheduling
    │   └── dispatcher_test.go
    ├── handler/
    │   ├── app.go                 # App CRUD handlers
    │   ├── app_test.go
    │   ├── key.go                 # Key management handlers
    │   ├── key_test.go
    │   ├── webhook.go             # Webhook subscription handlers
    │   ├── webhook_test.go
    │   ├── usage.go               # Usage stats handlers
    │   ├── usage_test.go
    │   ├── validate.go            # Internal key validation handler
    │   └── validate_test.go

infrastructure/db/migrations/
├── 010_create_developer_apps.sql
├── 011_create_api_keys.sql
├── 012_create_webhook_subscriptions.sql
├── 013_create_webhook_deliveries.sql
└── 014_create_api_usage.sql
```

---

### Task 1: Config + API Key Generator

**Files:**
- Create: `services/garudaportal/go.mod`
- Create: `services/garudaportal/config/config.go`
- Create: `services/garudaportal/config/config_test.go`
- Create: `services/garudaportal/apikey/generator.go`
- Create: `services/garudaportal/apikey/generator_test.go`

- [ ] **Step 1: Initialize go module**

```bash
cd services/garudaportal
go mod init github.com/garudapass/gpass/services/garudaportal
```

Set `go 1.25.0` in go.mod.

- [ ] **Step 2: Write failing config tests**

Create `services/garudaportal/config/config_test.go`:

```go
package config

import (
	"os"
	"testing"
)

func clearEnv() {
	for _, key := range []string{
		"GARUDAPORTAL_PORT", "GARUDAPORTAL_DB_URL",
		"IDENTITY_SERVICE_URL", "KEYCLOAK_ADMIN_URL",
		"KEYCLOAK_ADMIN_USER", "KEYCLOAK_ADMIN_PASSWORD",
		"KAFKA_BROKERS", "REDIS_URL",
		"WEBHOOK_TIMEOUT", "WEBHOOK_MAX_RETRIES",
	} {
		os.Unsetenv(key)
	}
}

func setRequiredEnv() {
	os.Setenv("GARUDAPORTAL_DB_URL", "postgres://user:pass@localhost/db")
	os.Setenv("IDENTITY_SERVICE_URL", "http://localhost:4001")
	os.Setenv("REDIS_URL", "redis://localhost:6379")
}

func TestLoad_Success(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != "4009" {
		t.Errorf("expected port 4009, got %s", cfg.Port)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost/db" {
		t.Errorf("unexpected database URL: %s", cfg.DatabaseURL)
	}
	if cfg.WebhookTimeout.Seconds() != 10 {
		t.Errorf("expected webhook timeout 10s, got %v", cfg.WebhookTimeout)
	}
	if cfg.WebhookMaxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", cfg.WebhookMaxRetries)
	}
	if cfg.KafkaBrokers != "localhost:19092" {
		t.Errorf("expected default kafka brokers, got %s", cfg.KafkaBrokers)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("GARUDAPORTAL_PORT", "5009")
	defer clearEnv()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Port != "5009" {
		t.Errorf("expected port 5009, got %s", cfg.Port)
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Unsetenv("GARUDAPORTAL_DB_URL")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing GARUDAPORTAL_DB_URL")
	}
}

func TestLoad_MissingIdentityServiceURL(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Unsetenv("IDENTITY_SERVICE_URL")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing IDENTITY_SERVICE_URL")
	}
}

func TestLoad_MissingRedisURL(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Unsetenv("REDIS_URL")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing REDIS_URL")
	}
}

func TestLoad_InvalidDatabaseURL(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("GARUDAPORTAL_DB_URL", "://invalid")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid database URL")
	}
}

func TestLoad_InvalidIdentityServiceURL(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("IDENTITY_SERVICE_URL", "://invalid")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid identity service URL")
	}
}

func TestLoad_InvalidWebhookTimeout(t *testing.T) {
	clearEnv()
	setRequiredEnv()
	os.Setenv("WEBHOOK_TIMEOUT", "not-a-duration")
	defer clearEnv()

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid WEBHOOK_TIMEOUT")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd services/garudaportal && go test ./config/... -v -count=1
```

Expected: FAIL — `config` package doesn't exist yet.

- [ ] **Step 4: Implement config module**

Create `services/garudaportal/config/config.go`:

```go
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all GarudaPortal service configuration loaded from environment variables.
type Config struct {
	Port               string
	DatabaseURL        string
	IdentityServiceURL string
	KeycloakAdminURL   string
	KeycloakAdminUser  string
	KeycloakAdminPass  string
	KafkaBrokers       string
	RedisURL           string
	WebhookTimeout     time.Duration
	WebhookMaxRetries  int
}

// Load reads configuration from environment variables and validates it.
func Load() (*Config, error) {
	webhookTimeoutStr := getEnv("WEBHOOK_TIMEOUT", "10s")
	webhookTimeout, err := time.ParseDuration(webhookTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("invalid WEBHOOK_TIMEOUT %q: %w", webhookTimeoutStr, err)
	}

	maxRetriesStr := getEnv("WEBHOOK_MAX_RETRIES", "5")
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		return nil, fmt.Errorf("invalid WEBHOOK_MAX_RETRIES %q: %w", maxRetriesStr, err)
	}

	cfg := &Config{
		Port:               getEnv("GARUDAPORTAL_PORT", "4009"),
		DatabaseURL:        os.Getenv("GARUDAPORTAL_DB_URL"),
		IdentityServiceURL: os.Getenv("IDENTITY_SERVICE_URL"),
		KeycloakAdminURL:   getEnv("KEYCLOAK_ADMIN_URL", "http://localhost:8080"),
		KeycloakAdminUser:  getEnv("KEYCLOAK_ADMIN_USER", "admin"),
		KeycloakAdminPass:  getEnv("KEYCLOAK_ADMIN_PASSWORD", "admin"),
		KafkaBrokers:       getEnv("KAFKA_BROKERS", "localhost:19092"),
		RedisURL:           os.Getenv("REDIS_URL"),
		WebhookTimeout:     webhookTimeout,
		WebhookMaxRetries:  maxRetries,
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
		{"GARUDAPORTAL_DB_URL", c.DatabaseURL},
		{"IDENTITY_SERVICE_URL", c.IdentityServiceURL},
		{"REDIS_URL", c.RedisURL},
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

	// Validate URLs
	for _, check := range []struct {
		name, val string
	}{
		{"GARUDAPORTAL_DB_URL", c.DatabaseURL},
		{"IDENTITY_SERVICE_URL", c.IdentityServiceURL},
	} {
		if _, err := url.ParseRequestURI(check.val); err != nil {
			return fmt.Errorf("invalid URL for %s: %w", check.name, err)
		}
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

- [ ] **Step 5: Run config tests to verify they pass**

```bash
cd services/garudaportal && go test ./config/... -v -count=1
```

Expected: All 8 tests PASS.

- [ ] **Step 6: Write failing API key generator tests**

Create `services/garudaportal/apikey/generator_test.go`:

```go
package apikey

import (
	"strings"
	"testing"
)

func TestGenerate_SandboxPrefix(t *testing.T) {
	key, err := Generate("sandbox")
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.HasPrefix(key, "gp_test_") {
		t.Errorf("sandbox key should start with gp_test_, got prefix: %s", key[:8])
	}

	// gp_test_ (8) + 64 base62 chars = 72 total
	if len(key) != 72 {
		t.Errorf("expected key length 72, got %d", len(key))
	}
}

func TestGenerate_ProductionPrefix(t *testing.T) {
	key, err := Generate("production")
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if !strings.HasPrefix(key, "gp_live_") {
		t.Errorf("production key should start with gp_live_, got prefix: %s", key[:8])
	}

	if len(key) != 72 {
		t.Errorf("expected key length 72, got %d", len(key))
	}
}

func TestGenerate_InvalidEnvironment(t *testing.T) {
	_, err := Generate("invalid")
	if err == nil {
		t.Error("expected error for invalid environment")
	}
}

func TestGenerate_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := Generate("sandbox")
		if err != nil {
			t.Fatalf("Generate() error on iteration %d: %v", i, err)
		}
		if keys[key] {
			t.Fatalf("duplicate key generated on iteration %d", i)
		}
		keys[key] = true
	}
}

func TestGenerate_Base62Characters(t *testing.T) {
	key, err := Generate("sandbox")
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	// Remove prefix
	randomPart := key[8:]
	for _, c := range randomPart {
		if !isBase62(c) {
			t.Errorf("non-base62 character in key: %c", c)
		}
	}
}

func isBase62(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func TestExtractPrefix(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"sandbox key", "gp_test_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789aBcDeFgHiJkLmNoPqRsTuVwXy", "gp_test_aBcDeFgH"},
		{"production key", "gp_live_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789aBcDeFgHiJkLmNoPqRsTuVwXy", "gp_live_aBcDeFgH"},
		{"short key", "gp_test_abc", "gp_test_abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := ExtractPrefix(tt.key)
			if prefix != tt.expected {
				t.Errorf("ExtractPrefix(%q) = %q, want %q", tt.key, prefix, tt.expected)
			}
		})
	}
}

func TestHashKey(t *testing.T) {
	hash1 := HashKey("gp_test_abc123")
	hash2 := HashKey("gp_test_abc123")
	hash3 := HashKey("gp_test_different")

	if hash1 != hash2 {
		t.Error("same key should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("different keys should produce different hashes")
	}
	if len(hash1) != 64 {
		t.Errorf("expected SHA-256 hex length 64, got %d", len(hash1))
	}
}
```

- [ ] **Step 7: Run tests to verify they fail**

```bash
cd services/garudaportal && go test ./apikey/... -v -count=1
```

Expected: FAIL — `apikey` package doesn't exist yet.

- [ ] **Step 8: Implement API key generator**

Create `services/garudaportal/apikey/generator.go`:

```go
package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

const (
	// base62Chars is the alphabet for base62 encoding.
	base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	// randomBytes is the number of random bytes to generate.
	randomBytes = 48

	// base62Len is the length of the base62-encoded random part.
	base62Len = 64

	// prefixLen is the number of chars to store as key_prefix (prefix + first 8 random chars).
	prefixLen = 16

	// PrefixSandbox is the key prefix for sandbox environment.
	PrefixSandbox = "gp_test_"

	// PrefixProduction is the key prefix for production environment.
	PrefixProduction = "gp_live_"
)

// Generate creates a new API key for the given environment.
// Returns the full plaintext key (shown once to the developer).
func Generate(environment string) (string, error) {
	prefix, err := prefixForEnvironment(environment)
	if err != nil {
		return "", err
	}

	randomPart, err := generateBase62(base62Len)
	if err != nil {
		return "", fmt.Errorf("generate random: %w", err)
	}

	return prefix + randomPart, nil
}

// HashKey computes the SHA-256 hash of a plaintext API key.
// Returns the hex-encoded hash string (64 chars).
func HashKey(plaintextKey string) string {
	h := sha256.Sum256([]byte(plaintextKey))
	return hex.EncodeToString(h[:])
}

// ExtractPrefix extracts the display prefix from a full key.
// Returns the first 16 characters (prefix + first 8 random chars).
func ExtractPrefix(key string) string {
	if len(key) <= prefixLen {
		return key
	}
	return key[:prefixLen]
}

func prefixForEnvironment(env string) (string, error) {
	switch env {
	case "sandbox":
		return PrefixSandbox, nil
	case "production":
		return PrefixProduction, nil
	default:
		return "", fmt.Errorf("invalid environment %q: must be 'sandbox' or 'production'", env)
	}
}

func generateBase62(length int) (string, error) {
	base := big.NewInt(int64(len(base62Chars)))
	result := make([]byte, length)

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, base)
		if err != nil {
			return "", fmt.Errorf("random int: %w", err)
		}
		result[i] = base62Chars[n.Int64()]
	}

	return string(result), nil
}
```

- [ ] **Step 9: Run tests to verify they pass**

```bash
cd services/garudaportal && go test ./apikey/... -v -count=1
```

Expected: All 6 tests PASS.

- [ ] **Step 10: Commit**

```bash
git add services/garudaportal/go.mod services/garudaportal/config/ services/garudaportal/apikey/generator.go services/garudaportal/apikey/generator_test.go
git commit -m "feat(garudaportal): add config loader and API key generator with base62 encoding"
```

---

### Task 2: API Key Validator

**Files:**
- Create: `services/garudaportal/apikey/validator.go`
- Create: `services/garudaportal/apikey/validator_test.go`

- [ ] **Step 1: Write failing validator tests**

Create `services/garudaportal/apikey/validator_test.go`:

```go
package apikey

import (
	"testing"
	"time"
)

// MockKeyLookup implements KeyLookup for testing.
type MockKeyLookup struct {
	keys map[string]*KeyInfo // hash -> KeyInfo
}

func NewMockKeyLookup() *MockKeyLookup {
	return &MockKeyLookup{keys: make(map[string]*KeyInfo)}
}

func (m *MockKeyLookup) GetKeyByHash(hash string) (*KeyInfo, error) {
	info, ok := m.keys[hash]
	if !ok {
		return nil, nil
	}
	return info, nil
}

func (m *MockKeyLookup) GetAppByID(appID string) (*AppInfo, error) {
	return &AppInfo{
		ID:          appID,
		Status:      "ACTIVE",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	}, nil
}

func (m *MockKeyLookup) GetDailyUsage(appID string, date time.Time) (int64, error) {
	return 0, nil
}

func (m *MockKeyLookup) IncrementUsage(appID, endpoint string, date time.Time) error {
	return nil
}

// MockKeyLookup with usage tracking
type MockKeyLookupWithUsage struct {
	MockKeyLookup
	usage int64
	app   *AppInfo
}

func NewMockKeyLookupWithUsage(usage int64, app *AppInfo) *MockKeyLookupWithUsage {
	return &MockKeyLookupWithUsage{
		MockKeyLookup: MockKeyLookup{keys: make(map[string]*KeyInfo)},
		usage:         usage,
		app:           app,
	}
}

func (m *MockKeyLookupWithUsage) GetAppByID(appID string) (*AppInfo, error) {
	if m.app != nil {
		return m.app, nil
	}
	return &AppInfo{
		ID:          appID,
		Status:      "ACTIVE",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	}, nil
}

func (m *MockKeyLookupWithUsage) GetDailyUsage(appID string, date time.Time) (int64, error) {
	return m.usage, nil
}

func TestValidate_Success(t *testing.T) {
	lookup := NewMockKeyLookup()
	plaintext := "gp_test_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789aBcDeFgHiJkLmNoPqRsTuVwXy"
	hash := HashKey(plaintext)
	lookup.keys[hash] = &KeyInfo{
		ID:          "key-1",
		AppID:       "app-1",
		Environment: "sandbox",
		Status:      "ACTIVE",
	}

	v := NewValidator(lookup)
	result, err := v.Validate(plaintext)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if !result.Valid {
		t.Errorf("expected valid=true, got false: %s", result.Error)
	}
	if result.AppID != "app-1" {
		t.Errorf("expected app-1, got %s", result.AppID)
	}
	if result.Environment != "sandbox" {
		t.Errorf("expected sandbox, got %s", result.Environment)
	}
}

func TestValidate_InvalidKey(t *testing.T) {
	lookup := NewMockKeyLookup()
	v := NewValidator(lookup)

	result, err := v.Validate("gp_test_nonexistent")
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for unknown key")
	}
	if result.Error != "invalid_key" {
		t.Errorf("expected error 'invalid_key', got %q", result.Error)
	}
}

func TestValidate_RevokedKey(t *testing.T) {
	lookup := NewMockKeyLookup()
	plaintext := "gp_test_revokedKeyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	hash := HashKey(plaintext)
	lookup.keys[hash] = &KeyInfo{
		ID:          "key-2",
		AppID:       "app-1",
		Environment: "sandbox",
		Status:      "REVOKED",
	}

	v := NewValidator(lookup)
	result, err := v.Validate(plaintext)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for revoked key")
	}
	if result.Error != "key_revoked" {
		t.Errorf("expected error 'key_revoked', got %q", result.Error)
	}
}

func TestValidate_ExpiredKey(t *testing.T) {
	lookup := NewMockKeyLookup()
	plaintext := "gp_test_expiredKeyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	hash := HashKey(plaintext)
	expired := time.Now().Add(-1 * time.Hour)
	lookup.keys[hash] = &KeyInfo{
		ID:          "key-3",
		AppID:       "app-1",
		Environment: "sandbox",
		Status:      "ACTIVE",
		ExpiresAt:   &expired,
	}

	v := NewValidator(lookup)
	result, err := v.Validate(plaintext)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for expired key")
	}
	if result.Error != "key_expired" {
		t.Errorf("expected error 'key_expired', got %q", result.Error)
	}
}

func TestValidate_RateLimitExceeded(t *testing.T) {
	app := &AppInfo{
		ID:          "app-1",
		Status:      "ACTIVE",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	}
	lookup := NewMockKeyLookupWithUsage(100, app)
	plaintext := "gp_test_rateLimitKeyXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	hash := HashKey(plaintext)
	lookup.keys[hash] = &KeyInfo{
		ID:          "key-4",
		AppID:       "app-1",
		Environment: "sandbox",
		Status:      "ACTIVE",
	}

	v := NewValidator(lookup)
	result, err := v.Validate(plaintext)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false when rate limit exceeded")
	}
	if result.Error != "rate_limit_exceeded" {
		t.Errorf("expected error 'rate_limit_exceeded', got %q", result.Error)
	}
}

func TestValidate_SuspendedApp(t *testing.T) {
	app := &AppInfo{
		ID:          "app-1",
		Status:      "SUSPENDED",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	}
	lookup := NewMockKeyLookupWithUsage(0, app)
	plaintext := "gp_test_suspendedAppXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"
	hash := HashKey(plaintext)
	lookup.keys[hash] = &KeyInfo{
		ID:          "key-5",
		AppID:       "app-1",
		Environment: "sandbox",
		Status:      "ACTIVE",
	}

	v := NewValidator(lookup)
	result, err := v.Validate(plaintext)
	if err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false for suspended app")
	}
	if result.Error != "app_suspended" {
		t.Errorf("expected error 'app_suspended', got %q", result.Error)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd services/garudaportal && go test ./apikey/... -v -count=1
```

Expected: FAIL — `Validator`, `KeyInfo`, `AppInfo` types don't exist yet.

- [ ] **Step 3: Implement API key validator**

Create `services/garudaportal/apikey/validator.go`:

```go
package apikey

import (
	"time"
)

// KeyInfo holds the stored metadata for an API key.
type KeyInfo struct {
	ID          string
	AppID       string
	Environment string
	Status      string // ACTIVE, REVOKED
	ExpiresAt   *time.Time
	LastUsedAt  *time.Time
}

// AppInfo holds the metadata for a developer app.
type AppInfo struct {
	ID          string
	Status      string // ACTIVE, SUSPENDED, DELETED
	Environment string
	Tier        string
	DailyLimit  int
}

// ValidationResult holds the outcome of a key validation.
type ValidationResult struct {
	Valid       bool   `json:"valid"`
	AppID       string `json:"app_id,omitempty"`
	Environment string `json:"environment,omitempty"`
	Tier        string `json:"tier,omitempty"`
	DailyLimit  int    `json:"daily_limit,omitempty"`
	Error       string `json:"error,omitempty"`
}

// KeyLookup abstracts the data access needed for key validation.
type KeyLookup interface {
	GetKeyByHash(hash string) (*KeyInfo, error)
	GetAppByID(appID string) (*AppInfo, error)
	GetDailyUsage(appID string, date time.Time) (int64, error)
	IncrementUsage(appID, endpoint string, date time.Time) error
}

// Validator validates API keys against stored hashes and checks rate limits.
type Validator struct {
	lookup KeyLookup
}

// NewValidator creates a new key validator.
func NewValidator(lookup KeyLookup) *Validator {
	return &Validator{lookup: lookup}
}

// Validate checks if a plaintext API key is valid.
// It computes the SHA-256 hash, looks up the key, verifies status, expiry, and rate limits.
func (v *Validator) Validate(plaintextKey string) (*ValidationResult, error) {
	hash := HashKey(plaintextKey)

	keyInfo, err := v.lookup.GetKeyByHash(hash)
	if err != nil {
		return nil, err
	}
	if keyInfo == nil {
		return &ValidationResult{Valid: false, Error: "invalid_key"}, nil
	}

	// Check key status
	if keyInfo.Status == "REVOKED" {
		return &ValidationResult{Valid: false, Error: "key_revoked"}, nil
	}

	// Check expiry
	if keyInfo.ExpiresAt != nil && time.Now().After(*keyInfo.ExpiresAt) {
		return &ValidationResult{Valid: false, Error: "key_expired"}, nil
	}

	// Look up the app
	appInfo, err := v.lookup.GetAppByID(keyInfo.AppID)
	if err != nil {
		return nil, err
	}
	if appInfo == nil {
		return &ValidationResult{Valid: false, Error: "app_not_found"}, nil
	}

	// Check app status
	if appInfo.Status != "ACTIVE" {
		return &ValidationResult{Valid: false, Error: "app_suspended"}, nil
	}

	// Check rate limit
	today := time.Now().UTC().Truncate(24 * time.Hour)
	usage, err := v.lookup.GetDailyUsage(keyInfo.AppID, today)
	if err != nil {
		return nil, err
	}
	if usage >= int64(appInfo.DailyLimit) {
		return &ValidationResult{Valid: false, Error: "rate_limit_exceeded"}, nil
	}

	return &ValidationResult{
		Valid:       true,
		AppID:       keyInfo.AppID,
		Environment: appInfo.Environment,
		Tier:        appInfo.Tier,
		DailyLimit:  appInfo.DailyLimit,
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd services/garudaportal && go test ./apikey/... -v -count=1
```

Expected: All 12 tests PASS (6 generator + 6 validator).

- [ ] **Step 5: Commit**

```bash
git add services/garudaportal/apikey/validator.go services/garudaportal/apikey/validator_test.go
git commit -m "feat(garudaportal): add API key validator with SHA-256 hash lookup and rate limit checks"
```

---

### Task 3: Webhook Signer

**Files:**
- Create: `services/garudaportal/webhook/signer.go`
- Create: `services/garudaportal/webhook/signer_test.go`

- [ ] **Step 1: Write failing signer tests**

Create `services/garudaportal/webhook/signer_test.go`:

```go
package webhook

import (
	"strings"
	"testing"
	"time"
)

func TestSign(t *testing.T) {
	secret := "whsec_abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678"
	payload := `{"event":"identity.verified","data":{"user_id":"user-123"}}`
	timestamp := time.Unix(1712400000, 0)

	signature := Sign(payload, secret, timestamp)

	if signature == "" {
		t.Fatal("signature should not be empty")
	}

	// Format: t=<timestamp>,v1=<hex>
	if !strings.HasPrefix(signature, "t=1712400000,v1=") {
		t.Errorf("unexpected signature format: %s", signature)
	}

	parts := strings.SplitN(signature, ",v1=", 2)
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts separated by ',v1=', got %d", len(parts))
	}

	hexPart := parts[1]
	if len(hexPart) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(hexPart))
	}
}

func TestSign_Deterministic(t *testing.T) {
	secret := "whsec_test_secret_123"
	payload := `{"event":"test"}`
	timestamp := time.Unix(1000000, 0)

	sig1 := Sign(payload, secret, timestamp)
	sig2 := Sign(payload, secret, timestamp)

	if sig1 != sig2 {
		t.Error("same inputs should produce same signature")
	}
}

func TestSign_DifferentPayloads(t *testing.T) {
	secret := "whsec_test_secret_123"
	timestamp := time.Unix(1000000, 0)

	sig1 := Sign(`{"a":1}`, secret, timestamp)
	sig2 := Sign(`{"a":2}`, secret, timestamp)

	if sig1 == sig2 {
		t.Error("different payloads should produce different signatures")
	}
}

func TestSign_DifferentTimestamps(t *testing.T) {
	secret := "whsec_test_secret_123"
	payload := `{"event":"test"}`

	sig1 := Sign(payload, secret, time.Unix(1000000, 0))
	sig2 := Sign(payload, secret, time.Unix(2000000, 0))

	if sig1 == sig2 {
		t.Error("different timestamps should produce different signatures")
	}
}

func TestSign_DifferentSecrets(t *testing.T) {
	payload := `{"event":"test"}`
	timestamp := time.Unix(1000000, 0)

	sig1 := Sign(payload, "secret1", timestamp)
	sig2 := Sign(payload, "secret2", timestamp)

	if sig1 == sig2 {
		t.Error("different secrets should produce different signatures")
	}
}

func TestVerify_Valid(t *testing.T) {
	secret := "whsec_verify_test"
	payload := `{"event":"identity.verified"}`
	timestamp := time.Now()

	signature := Sign(payload, secret, timestamp)

	if !Verify(payload, secret, signature, 5*time.Minute) {
		t.Error("Verify should return true for valid signature")
	}
}

func TestVerify_InvalidSignature(t *testing.T) {
	secret := "whsec_verify_test"
	payload := `{"event":"identity.verified"}`

	if Verify(payload, secret, "t=1712400000,v1=0000000000000000000000000000000000000000000000000000000000000000", 5*time.Minute) {
		t.Error("Verify should return false for wrong signature")
	}
}

func TestVerify_ExpiredTimestamp(t *testing.T) {
	secret := "whsec_verify_test"
	payload := `{"event":"identity.verified"}`
	oldTimestamp := time.Now().Add(-10 * time.Minute)

	signature := Sign(payload, secret, oldTimestamp)

	if Verify(payload, secret, signature, 5*time.Minute) {
		t.Error("Verify should return false for expired timestamp")
	}
}

func TestVerify_MalformedSignature(t *testing.T) {
	if Verify("payload", "secret", "malformed", 5*time.Minute) {
		t.Error("Verify should return false for malformed signature")
	}

	if Verify("payload", "secret", "t=notanumber,v1=abc", 5*time.Minute) {
		t.Error("Verify should return false for non-numeric timestamp")
	}

	if Verify("payload", "secret", "", 5*time.Minute) {
		t.Error("Verify should return false for empty signature")
	}
}

func TestGenerateSecret(t *testing.T) {
	secret1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error: %v", err)
	}

	if !strings.HasPrefix(secret1, "whsec_") {
		t.Errorf("secret should start with whsec_, got: %s", secret1[:6])
	}

	// whsec_ (6) + 64 hex chars = 70 total
	if len(secret1) != 70 {
		t.Errorf("expected secret length 70, got %d", len(secret1))
	}

	secret2, _ := GenerateSecret()
	if secret1 == secret2 {
		t.Error("secrets should be unique")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd services/garudaportal && go test ./webhook/... -v -count=1
```

Expected: FAIL — `webhook` package doesn't exist yet.

- [ ] **Step 3: Implement webhook signer**

Create `services/garudaportal/webhook/signer.go`:

```go
package webhook

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Sign computes an HMAC-SHA256 signature for a webhook payload.
// Format: t=<unix_timestamp>,v1=<hex_hmac>
// Signed message: "<timestamp>.<payload>"
func Sign(payload, secret string, timestamp time.Time) string {
	ts := strconv.FormatInt(timestamp.Unix(), 10)
	message := ts + "." + payload

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	sig := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("t=%s,v1=%s", ts, sig)
}

// Verify checks a webhook signature against a payload.
// Returns false if the signature is invalid or the timestamp is too old.
func Verify(payload, secret, signature string, tolerance time.Duration) bool {
	if signature == "" {
		return false
	}

	// Parse t=<ts>,v1=<sig>
	parts := strings.SplitN(signature, ",v1=", 2)
	if len(parts) != 2 {
		return false
	}

	tsPart := strings.TrimPrefix(parts[0], "t=")
	ts, err := strconv.ParseInt(tsPart, 10, 64)
	if err != nil {
		return false
	}

	// Check timestamp freshness
	sigTime := time.Unix(ts, 0)
	if time.Since(sigTime) > tolerance {
		return false
	}

	// Recompute expected signature
	expected := Sign(payload, secret, sigTime)

	// Constant-time comparison
	return subtle.ConstantTimeCompare([]byte(signature), []byte(expected)) == 1
}

// GenerateSecret generates a random webhook signing secret.
// Format: whsec_ + 32 random bytes hex-encoded (64 chars).
func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate webhook secret: %w", err)
	}
	return "whsec_" + hex.EncodeToString(b), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd services/garudaportal && go test ./webhook/... -v -count=1
```

Expected: All 10 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add services/garudaportal/webhook/signer.go services/garudaportal/webhook/signer_test.go
git commit -m "feat(garudaportal): add HMAC-SHA256 webhook signer with timestamp verification"
```

---

### Task 4: Webhook Dispatcher

**Files:**
- Create: `services/garudaportal/webhook/dispatcher.go`
- Create: `services/garudaportal/webhook/dispatcher_test.go`

- [ ] **Step 1: Write failing dispatcher tests**

Create `services/garudaportal/webhook/dispatcher_test.go`:

```go
package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatcher_DeliverSuccess(t *testing.T) {
	var received atomic.Bool
	var receivedSig string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(true)
		receivedSig = r.Header.Get("X-GarudaPass-Signature")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	d := NewDispatcher(10*time.Second, 5)

	result := d.Deliver(DeliveryRequest{
		URL:       srv.URL,
		Secret:    "whsec_test123",
		EventType: "identity.verified",
		Payload:   json.RawMessage(`{"user_id":"u-1"}`),
	})

	if !received.Load() {
		t.Fatal("webhook was not received by server")
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
	if !result.Success {
		t.Errorf("expected success=true, got false: %s", result.Error)
	}
	if receivedSig == "" {
		t.Error("X-GarudaPass-Signature header should be set")
	}
	if result.ResponseBody == "" {
		t.Error("response body should not be empty")
	}
}

func TestDispatcher_DeliverServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal"}`))
	}))
	defer srv.Close()

	d := NewDispatcher(10*time.Second, 5)

	result := d.Deliver(DeliveryRequest{
		URL:       srv.URL,
		Secret:    "whsec_test123",
		EventType: "identity.verified",
		Payload:   json.RawMessage(`{"user_id":"u-1"}`),
	})

	if result.Success {
		t.Error("expected success=false for 500 response")
	}
	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", result.StatusCode)
	}
}

func TestDispatcher_DeliverTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(100*time.Millisecond, 5)

	result := d.Deliver(DeliveryRequest{
		URL:       srv.URL,
		Secret:    "whsec_test123",
		EventType: "identity.verified",
		Payload:   json.RawMessage(`{"user_id":"u-1"}`),
	})

	if result.Success {
		t.Error("expected success=false for timeout")
	}
	if result.Error == "" {
		t.Error("expected error message for timeout")
	}
}

func TestDispatcher_DeliverInvalidURL(t *testing.T) {
	d := NewDispatcher(10*time.Second, 5)

	result := d.Deliver(DeliveryRequest{
		URL:       "http://invalid.localhost:99999/webhook",
		Secret:    "whsec_test123",
		EventType: "identity.verified",
		Payload:   json.RawMessage(`{"user_id":"u-1"}`),
	})

	if result.Success {
		t.Error("expected success=false for invalid URL")
	}
}

func TestDispatcher_ResponseBodyTruncation(t *testing.T) {
	longBody := make([]byte, 2048)
	for i := range longBody {
		longBody[i] = 'x'
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(longBody)
	}))
	defer srv.Close()

	d := NewDispatcher(10*time.Second, 5)

	result := d.Deliver(DeliveryRequest{
		URL:       srv.URL,
		Secret:    "whsec_test123",
		EventType: "identity.verified",
		Payload:   json.RawMessage(`{}`),
	})

	if len(result.ResponseBody) > 1024 {
		t.Errorf("response body should be truncated to 1024, got %d", len(result.ResponseBody))
	}
}

func TestRetryDelays(t *testing.T) {
	delays := RetryDelays()

	expected := []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		24 * time.Hour,
	}

	if len(delays) != len(expected) {
		t.Fatalf("expected %d delays, got %d", len(expected), len(delays))
	}

	for i, d := range delays {
		if d != expected[i] {
			t.Errorf("delay[%d] = %v, want %v", i, d, expected[i])
		}
	}
}

func TestNextRetryAt(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		wantNil  bool
	}{
		{"first retry", 1, false},
		{"second retry", 2, false},
		{"third retry", 3, false},
		{"fourth retry", 4, false},
		{"fifth retry", 5, false},
		{"exhausted", 6, true},
		{"beyond max", 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NextRetryAt(tt.attempt, 5)
			if tt.wantNil && result != nil {
				t.Errorf("expected nil for attempt %d, got %v", tt.attempt, result)
			}
			if !tt.wantNil && result == nil {
				t.Errorf("expected non-nil for attempt %d", tt.attempt)
			}
		})
	}
}

func TestDispatcher_SignatureHeaders(t *testing.T) {
	var headers http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewDispatcher(10*time.Second, 5)
	d.Deliver(DeliveryRequest{
		URL:       srv.URL,
		Secret:    "whsec_test123",
		EventType: "identity.verified",
		Payload:   json.RawMessage(`{"user_id":"u-1"}`),
	})

	if headers.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", headers.Get("Content-Type"))
	}
	if headers.Get("X-GarudaPass-Event") != "identity.verified" {
		t.Errorf("expected X-GarudaPass-Event identity.verified, got %s", headers.Get("X-GarudaPass-Event"))
	}
	if headers.Get("X-GarudaPass-Signature") == "" {
		t.Error("X-GarudaPass-Signature should be set")
	}
	if headers.Get("User-Agent") != "GarudaPass-Webhook/1.0" {
		t.Errorf("expected User-Agent GarudaPass-Webhook/1.0, got %s", headers.Get("User-Agent"))
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd services/garudaportal && go test ./webhook/... -v -count=1
```

Expected: FAIL — `Dispatcher`, `DeliveryRequest` types don't exist yet.

- [ ] **Step 3: Implement webhook dispatcher**

Create `services/garudaportal/webhook/dispatcher.go`:

```go
package webhook

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxResponseBodySize = 1024
)

// DeliveryRequest holds the data needed to deliver a webhook.
type DeliveryRequest struct {
	URL       string
	Secret    string
	EventType string
	Payload   json.RawMessage
}

// DeliveryResult holds the outcome of a delivery attempt.
type DeliveryResult struct {
	Success      bool
	StatusCode   int
	ResponseBody string
	Error        string
}

// Dispatcher delivers webhooks via HTTP with signature headers.
type Dispatcher struct {
	httpClient *http.Client
	maxRetries int
}

// NewDispatcher creates a new webhook dispatcher.
func NewDispatcher(timeout time.Duration, maxRetries int) *Dispatcher {
	return &Dispatcher{
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxRetries: maxRetries,
	}
}

// Deliver sends a webhook to the target URL with HMAC-SHA256 signature.
func (d *Dispatcher) Deliver(req DeliveryRequest) *DeliveryResult {
	payloadBytes, err := json.Marshal(req.Payload)
	if err != nil {
		return &DeliveryResult{Success: false, Error: fmt.Sprintf("marshal payload: %v", err)}
	}

	timestamp := time.Now()
	signature := Sign(string(payloadBytes), req.Secret, timestamp)

	httpReq, err := http.NewRequest(http.MethodPost, req.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		return &DeliveryResult{Success: false, Error: fmt.Sprintf("create request: %v", err)}
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("User-Agent", "GarudaPass-Webhook/1.0")
	httpReq.Header.Set("X-GarudaPass-Event", req.EventType)
	httpReq.Header.Set("X-GarudaPass-Signature", signature)

	resp, err := d.httpClient.Do(httpReq)
	if err != nil {
		return &DeliveryResult{Success: false, Error: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	// Read response body (truncated)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBodySize+1))
	respBody := string(bodyBytes)
	if len(respBody) > maxResponseBodySize {
		respBody = respBody[:maxResponseBodySize]
	}

	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	return &DeliveryResult{
		Success:      success,
		StatusCode:   resp.StatusCode,
		ResponseBody: respBody,
	}
}

// RetryDelays returns the exponential backoff schedule for webhook retries.
func RetryDelays() []time.Duration {
	return []time.Duration{
		1 * time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		2 * time.Hour,
		24 * time.Hour,
	}
}

// NextRetryAt computes the next retry time based on the current attempt.
// Returns nil if retries are exhausted.
func NextRetryAt(attempt, maxRetries int) *time.Time {
	delays := RetryDelays()
	if attempt < 1 || attempt > maxRetries || attempt > len(delays) {
		return nil
	}
	t := time.Now().Add(delays[attempt-1])
	return &t
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd services/garudaportal && go test ./webhook/... -v -count=1
```

Expected: All 18 tests PASS (10 signer + 8 dispatcher).

- [ ] **Step 5: Commit**

```bash
git add services/garudaportal/webhook/dispatcher.go services/garudaportal/webhook/dispatcher_test.go
git commit -m "feat(garudaportal): add webhook dispatcher with HTTP delivery, retry scheduling, and body truncation"
```

---

### Task 5: Stores (App, Key, Webhook, Delivery, Usage)

**Files:**
- Create: `services/garudaportal/store/app.go`
- Create: `services/garudaportal/store/app_test.go`
- Create: `services/garudaportal/store/key.go`
- Create: `services/garudaportal/store/key_test.go`
- Create: `services/garudaportal/store/webhook.go`
- Create: `services/garudaportal/store/webhook_test.go`
- Create: `services/garudaportal/store/delivery.go`
- Create: `services/garudaportal/store/delivery_test.go`
- Create: `services/garudaportal/store/usage.go`
- Create: `services/garudaportal/store/usage_test.go`

- [ ] **Step 1: Write failing app store tests**

Create `services/garudaportal/store/app_test.go`:

```go
package store

import (
	"testing"
)

func TestAppStore_Create(t *testing.T) {
	s := NewInMemoryAppStore()

	app := &App{
		OwnerUserID:  "user-1",
		Name:         "My App",
		Description:  "Test application",
		Environment:  "sandbox",
		Tier:         "free",
		DailyLimit:   100,
		CallbackURLs: []string{"https://example.com/callback"},
		Status:       "ACTIVE",
	}

	created, err := s.Create(app)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if created.ID == "" {
		t.Error("ID should be generated")
	}
	if created.Name != "My App" {
		t.Errorf("expected name 'My App', got %q", created.Name)
	}
	if created.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestAppStore_GetByID(t *testing.T) {
	s := NewInMemoryAppStore()

	app, _ := s.Create(&App{
		OwnerUserID: "user-1",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
		Status:      "ACTIVE",
	})

	found, err := s.GetByID(app.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if found.Name != "Test App" {
		t.Errorf("expected 'Test App', got %q", found.Name)
	}
}

func TestAppStore_GetByID_NotFound(t *testing.T) {
	s := NewInMemoryAppStore()

	_, err := s.GetByID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestAppStore_ListByOwner(t *testing.T) {
	s := NewInMemoryAppStore()

	s.Create(&App{OwnerUserID: "user-1", Name: "App 1", Status: "ACTIVE"})
	s.Create(&App{OwnerUserID: "user-1", Name: "App 2", Status: "ACTIVE"})
	s.Create(&App{OwnerUserID: "user-2", Name: "App 3", Status: "ACTIVE"})

	apps, err := s.ListByOwner("user-1")
	if err != nil {
		t.Fatalf("ListByOwner() error: %v", err)
	}
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestAppStore_Update(t *testing.T) {
	s := NewInMemoryAppStore()

	app, _ := s.Create(&App{
		OwnerUserID: "user-1",
		Name:        "Original",
		Description: "Desc",
		Status:      "ACTIVE",
	})

	updated, err := s.Update(app.ID, &AppUpdate{
		Name:        strPtr("Updated Name"),
		Description: strPtr("New desc"),
	})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("expected 'Updated Name', got %q", updated.Name)
	}
	if updated.Description != "New desc" {
		t.Errorf("expected 'New desc', got %q", updated.Description)
	}
}

func TestAppStore_Update_NotFound(t *testing.T) {
	s := NewInMemoryAppStore()

	_, err := s.Update("nonexistent", &AppUpdate{Name: strPtr("X")})
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}

func TestAppStore_UpdateStatus(t *testing.T) {
	s := NewInMemoryAppStore()

	app, _ := s.Create(&App{
		OwnerUserID: "user-1",
		Name:        "App",
		Status:      "ACTIVE",
	})

	err := s.UpdateStatus(app.ID, "SUSPENDED")
	if err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	found, _ := s.GetByID(app.ID)
	if found.Status != "SUSPENDED" {
		t.Errorf("expected SUSPENDED, got %s", found.Status)
	}
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Write failing key store tests**

Create `services/garudaportal/store/key_test.go`:

```go
package store

import (
	"testing"
	"time"
)

func TestKeyStore_Create(t *testing.T) {
	s := NewInMemoryKeyStore()

	key := &APIKey{
		AppID:       "app-1",
		KeyHash:     "abc123hash",
		KeyPrefix:   "gp_test_aBcDeFgH",
		Name:        "Production Key",
		Environment: "sandbox",
		Status:      "ACTIVE",
	}

	created, err := s.Create(key)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if created.ID == "" {
		t.Error("ID should be generated")
	}
	if created.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestKeyStore_GetByHash(t *testing.T) {
	s := NewInMemoryKeyStore()

	s.Create(&APIKey{
		AppID:       "app-1",
		KeyHash:     "unique-hash-123",
		KeyPrefix:   "gp_test_prefix",
		Name:        "Key 1",
		Environment: "sandbox",
		Status:      "ACTIVE",
	})

	found, err := s.GetByHash("unique-hash-123")
	if err != nil {
		t.Fatalf("GetByHash() error: %v", err)
	}
	if found.Name != "Key 1" {
		t.Errorf("expected 'Key 1', got %q", found.Name)
	}
}

func TestKeyStore_GetByHash_NotFound(t *testing.T) {
	s := NewInMemoryKeyStore()

	found, err := s.GetByHash("nonexistent")
	if err != nil {
		t.Fatalf("GetByHash() unexpected error: %v", err)
	}
	if found != nil {
		t.Error("expected nil for nonexistent hash")
	}
}

func TestKeyStore_ListByApp(t *testing.T) {
	s := NewInMemoryKeyStore()

	s.Create(&APIKey{AppID: "app-1", KeyHash: "h1", KeyPrefix: "p1", Name: "K1", Status: "ACTIVE"})
	s.Create(&APIKey{AppID: "app-1", KeyHash: "h2", KeyPrefix: "p2", Name: "K2", Status: "ACTIVE"})
	s.Create(&APIKey{AppID: "app-2", KeyHash: "h3", KeyPrefix: "p3", Name: "K3", Status: "ACTIVE"})

	keys, err := s.ListByApp("app-1")
	if err != nil {
		t.Fatalf("ListByApp() error: %v", err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestKeyStore_Revoke(t *testing.T) {
	s := NewInMemoryKeyStore()

	key, _ := s.Create(&APIKey{
		AppID:   "app-1",
		KeyHash: "revoke-hash",
		Name:    "To Revoke",
		Status:  "ACTIVE",
	})

	err := s.Revoke(key.ID)
	if err != nil {
		t.Fatalf("Revoke() error: %v", err)
	}

	found, _ := s.GetByHash("revoke-hash")
	if found.Status != "REVOKED" {
		t.Errorf("expected REVOKED, got %s", found.Status)
	}
	if found.RevokedAt == nil {
		t.Error("RevokedAt should be set")
	}
}

func TestKeyStore_UpdateLastUsed(t *testing.T) {
	s := NewInMemoryKeyStore()

	key, _ := s.Create(&APIKey{
		AppID:   "app-1",
		KeyHash: "used-hash",
		Name:    "Used Key",
		Status:  "ACTIVE",
	})

	now := time.Now()
	err := s.UpdateLastUsed(key.ID, now)
	if err != nil {
		t.Fatalf("UpdateLastUsed() error: %v", err)
	}

	found, _ := s.GetByHash("used-hash")
	if found.LastUsedAt == nil {
		t.Error("LastUsedAt should be set")
	}
}

func TestKeyStore_GetByID(t *testing.T) {
	s := NewInMemoryKeyStore()

	key, _ := s.Create(&APIKey{
		AppID:   "app-1",
		KeyHash: "byid-hash",
		Name:    "By ID",
		Status:  "ACTIVE",
	})

	found, err := s.GetByID(key.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if found.Name != "By ID" {
		t.Errorf("expected 'By ID', got %q", found.Name)
	}
}
```

- [ ] **Step 3: Write failing webhook store tests**

Create `services/garudaportal/store/webhook_test.go`:

```go
package store

import (
	"testing"
)

func TestWebhookStore_Create(t *testing.T) {
	s := NewInMemoryWebhookStore()

	wh := &WebhookSubscription{
		AppID:  "app-1",
		URL:    "https://example.com/webhook",
		Events: []string{"identity.verified", "document.signed"},
		Secret: "whsec_test123",
		Status: "ACTIVE",
	}

	created, err := s.Create(wh)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if created.ID == "" {
		t.Error("ID should be generated")
	}
	if len(created.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(created.Events))
	}
}

func TestWebhookStore_ListByApp(t *testing.T) {
	s := NewInMemoryWebhookStore()

	s.Create(&WebhookSubscription{AppID: "app-1", URL: "https://a.com/wh", Events: []string{"e1"}, Secret: "s1", Status: "ACTIVE"})
	s.Create(&WebhookSubscription{AppID: "app-1", URL: "https://b.com/wh", Events: []string{"e2"}, Secret: "s2", Status: "ACTIVE"})
	s.Create(&WebhookSubscription{AppID: "app-2", URL: "https://c.com/wh", Events: []string{"e3"}, Secret: "s3", Status: "ACTIVE"})

	hooks, err := s.ListByApp("app-1")
	if err != nil {
		t.Fatalf("ListByApp() error: %v", err)
	}
	if len(hooks) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(hooks))
	}
}

func TestWebhookStore_ListByAppAndEvent(t *testing.T) {
	s := NewInMemoryWebhookStore()

	s.Create(&WebhookSubscription{AppID: "app-1", URL: "https://a.com/wh", Events: []string{"identity.verified", "document.signed"}, Secret: "s1", Status: "ACTIVE"})
	s.Create(&WebhookSubscription{AppID: "app-1", URL: "https://b.com/wh", Events: []string{"document.signed"}, Secret: "s2", Status: "ACTIVE"})
	s.Create(&WebhookSubscription{AppID: "app-1", URL: "https://c.com/wh", Events: []string{"identity.verified"}, Secret: "s3", Status: "DISABLED"})

	hooks, err := s.ListByAppAndEvent("app-1", "identity.verified")
	if err != nil {
		t.Fatalf("ListByAppAndEvent() error: %v", err)
	}
	// Only 1 active subscription with identity.verified
	if len(hooks) != 1 {
		t.Errorf("expected 1 webhook, got %d", len(hooks))
	}
}

func TestWebhookStore_Disable(t *testing.T) {
	s := NewInMemoryWebhookStore()

	wh, _ := s.Create(&WebhookSubscription{
		AppID:  "app-1",
		URL:    "https://a.com/wh",
		Events: []string{"e1"},
		Secret: "s1",
		Status: "ACTIVE",
	})

	err := s.Disable(wh.ID)
	if err != nil {
		t.Fatalf("Disable() error: %v", err)
	}

	found, _ := s.GetByID(wh.ID)
	if found.Status != "DISABLED" {
		t.Errorf("expected DISABLED, got %s", found.Status)
	}
}

func TestWebhookStore_GetByID(t *testing.T) {
	s := NewInMemoryWebhookStore()

	wh, _ := s.Create(&WebhookSubscription{
		AppID:  "app-1",
		URL:    "https://a.com/wh",
		Events: []string{"e1"},
		Secret: "s1",
		Status: "ACTIVE",
	})

	found, err := s.GetByID(wh.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if found.URL != "https://a.com/wh" {
		t.Errorf("expected URL 'https://a.com/wh', got %q", found.URL)
	}
}

func TestWebhookStore_GetByID_NotFound(t *testing.T) {
	s := NewInMemoryWebhookStore()

	_, err := s.GetByID("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ID")
	}
}
```

- [ ] **Step 4: Write failing delivery store tests**

Create `services/garudaportal/store/delivery_test.go`:

```go
package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeliveryStore_Create(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	d := &WebhookDelivery{
		SubscriptionID: "sub-1",
		EventType:      "identity.verified",
		Payload:        json.RawMessage(`{"user_id":"u-1"}`),
		Status:         "PENDING",
		Attempts:       0,
	}

	created, err := s.Create(d)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if created.ID == "" {
		t.Error("ID should be generated")
	}
	if created.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestDeliveryStore_GetByID(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	d, _ := s.Create(&WebhookDelivery{
		SubscriptionID: "sub-1",
		EventType:      "identity.verified",
		Payload:        json.RawMessage(`{}`),
		Status:         "PENDING",
	})

	found, err := s.GetByID(d.ID)
	if err != nil {
		t.Fatalf("GetByID() error: %v", err)
	}
	if found.EventType != "identity.verified" {
		t.Errorf("expected event_type 'identity.verified', got %q", found.EventType)
	}
}

func TestDeliveryStore_ListBySubscription(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	s.Create(&WebhookDelivery{SubscriptionID: "sub-1", EventType: "e1", Payload: json.RawMessage(`{}`), Status: "PENDING"})
	s.Create(&WebhookDelivery{SubscriptionID: "sub-1", EventType: "e2", Payload: json.RawMessage(`{}`), Status: "DELIVERED"})
	s.Create(&WebhookDelivery{SubscriptionID: "sub-2", EventType: "e3", Payload: json.RawMessage(`{}`), Status: "PENDING"})

	deliveries, err := s.ListBySubscription("sub-1")
	if err != nil {
		t.Fatalf("ListBySubscription() error: %v", err)
	}
	if len(deliveries) != 2 {
		t.Errorf("expected 2 deliveries, got %d", len(deliveries))
	}
}

func TestDeliveryStore_UpdateStatus(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	d, _ := s.Create(&WebhookDelivery{
		SubscriptionID: "sub-1",
		EventType:      "e1",
		Payload:        json.RawMessage(`{}`),
		Status:         "PENDING",
	})

	now := time.Now()
	err := s.UpdateStatus(d.ID, &DeliveryUpdate{
		Status:           "DELIVERED",
		Attempts:         1,
		LastResponseCode: intPtr(200),
		LastResponseBody: strPtr(`{"ok":true}`),
		DeliveredAt:      &now,
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error: %v", err)
	}

	found, _ := s.GetByID(d.ID)
	if found.Status != "DELIVERED" {
		t.Errorf("expected DELIVERED, got %s", found.Status)
	}
	if found.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", found.Attempts)
	}
	if found.DeliveredAt == nil {
		t.Error("DeliveredAt should be set")
	}
}

func TestDeliveryStore_ListPendingRetries(t *testing.T) {
	s := NewInMemoryDeliveryStore()

	past := time.Now().Add(-1 * time.Minute)
	future := time.Now().Add(1 * time.Hour)

	d1, _ := s.Create(&WebhookDelivery{SubscriptionID: "sub-1", EventType: "e1", Payload: json.RawMessage(`{}`), Status: "PENDING"})
	d2, _ := s.Create(&WebhookDelivery{SubscriptionID: "sub-1", EventType: "e2", Payload: json.RawMessage(`{}`), Status: "PENDING"})
	s.Create(&WebhookDelivery{SubscriptionID: "sub-1", EventType: "e3", Payload: json.RawMessage(`{}`), Status: "DELIVERED"})

	s.UpdateStatus(d1.ID, &DeliveryUpdate{Status: "PENDING", Attempts: 1, NextRetryAt: &past})
	s.UpdateStatus(d2.ID, &DeliveryUpdate{Status: "PENDING", Attempts: 1, NextRetryAt: &future})

	pending, err := s.ListPendingRetries(time.Now())
	if err != nil {
		t.Fatalf("ListPendingRetries() error: %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("expected 1 pending retry, got %d", len(pending))
	}
}

func intPtr(i int) *int { return &i }
```

- [ ] **Step 5: Write failing usage store tests**

Create `services/garudaportal/store/usage_test.go`:

```go
package store

import (
	"testing"
	"time"
)

func TestUsageStore_Increment(t *testing.T) {
	s := NewInMemoryUsageStore()

	date := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	err := s.Increment("app-1", "/api/v1/identity/verify", date)
	if err != nil {
		t.Fatalf("Increment() error: %v", err)
	}

	err = s.Increment("app-1", "/api/v1/identity/verify", date)
	if err != nil {
		t.Fatalf("Increment() error: %v", err)
	}

	usage, err := s.GetDailyTotal("app-1", date)
	if err != nil {
		t.Fatalf("GetDailyTotal() error: %v", err)
	}
	if usage != 2 {
		t.Errorf("expected 2, got %d", usage)
	}
}

func TestUsageStore_IncrementError(t *testing.T) {
	s := NewInMemoryUsageStore()

	date := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	err := s.IncrementError("app-1", "/api/v1/identity/verify", date)
	if err != nil {
		t.Fatalf("IncrementError() error: %v", err)
	}

	records, err := s.GetByAppAndDateRange("app-1", date, date)
	if err != nil {
		t.Fatalf("GetByAppAndDateRange() error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].ErrorCount != 1 {
		t.Errorf("expected error_count 1, got %d", records[0].ErrorCount)
	}
}

func TestUsageStore_GetDailyTotal_Empty(t *testing.T) {
	s := NewInMemoryUsageStore()

	date := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	usage, err := s.GetDailyTotal("app-1", date)
	if err != nil {
		t.Fatalf("GetDailyTotal() error: %v", err)
	}
	if usage != 0 {
		t.Errorf("expected 0, got %d", usage)
	}
}

func TestUsageStore_GetByAppAndDateRange(t *testing.T) {
	s := NewInMemoryUsageStore()

	d1 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	s.Increment("app-1", "/api/v1/identity/verify", d1)
	s.Increment("app-1", "/api/v1/identity/verify", d1)
	s.Increment("app-1", "/api/v1/corp/entity", d2)
	s.Increment("app-1", "/api/v1/identity/verify", d3)

	records, err := s.GetByAppAndDateRange("app-1", d1, d3)
	if err != nil {
		t.Fatalf("GetByAppAndDateRange() error: %v", err)
	}
	if len(records) != 3 {
		t.Errorf("expected 3 records, got %d", len(records))
	}
}

func TestUsageStore_GetByAppAndDateRange_FilterByDate(t *testing.T) {
	s := NewInMemoryUsageStore()

	d1 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 4, 3, 0, 0, 0, 0, time.UTC)

	s.Increment("app-1", "/api/v1/identity/verify", d1)
	s.Increment("app-1", "/api/v1/identity/verify", d2)
	s.Increment("app-1", "/api/v1/identity/verify", d3)

	records, err := s.GetByAppAndDateRange("app-1", d1, d2)
	if err != nil {
		t.Fatalf("GetByAppAndDateRange() error: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records (d1 and d2 only), got %d", len(records))
	}
}

func TestUsageStore_MultipleApps(t *testing.T) {
	s := NewInMemoryUsageStore()

	date := time.Date(2026, 4, 6, 0, 0, 0, 0, time.UTC)
	s.Increment("app-1", "/api/v1/identity/verify", date)
	s.Increment("app-2", "/api/v1/identity/verify", date)

	u1, _ := s.GetDailyTotal("app-1", date)
	u2, _ := s.GetDailyTotal("app-2", date)

	if u1 != 1 || u2 != 1 {
		t.Errorf("expected 1 and 1, got %d and %d", u1, u2)
	}
}
```

- [ ] **Step 6: Run tests to verify they fail**

```bash
cd services/garudaportal && go test ./store/... -v -count=1
```

Expected: FAIL — `store` package doesn't exist yet.

- [ ] **Step 7: Implement app store**

Create `services/garudaportal/store/app.go`:

```go
package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// App represents a developer application.
type App struct {
	ID           string    `json:"id"`
	OwnerUserID  string    `json:"owner_user_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description,omitempty"`
	Environment  string    `json:"environment"`
	Tier         string    `json:"tier"`
	DailyLimit   int       `json:"daily_limit"`
	CallbackURLs []string  `json:"callback_urls,omitempty"`
	OAuthClientID string   `json:"oauth_client_id,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AppUpdate holds optional fields for updating an app.
type AppUpdate struct {
	Name         *string
	Description  *string
	CallbackURLs []string
}

// AppStore defines the interface for app storage operations.
type AppStore interface {
	Create(app *App) (*App, error)
	GetByID(id string) (*App, error)
	ListByOwner(ownerUserID string) ([]*App, error)
	Update(id string, update *AppUpdate) (*App, error)
	UpdateStatus(id, status string) error
}

// InMemoryAppStore is an in-memory implementation of AppStore.
type InMemoryAppStore struct {
	mu   sync.RWMutex
	apps map[string]*App
}

// NewInMemoryAppStore creates a new in-memory app store.
func NewInMemoryAppStore() *InMemoryAppStore {
	return &InMemoryAppStore{
		apps: make(map[string]*App),
	}
}

func (s *InMemoryAppStore) Create(app *App) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app.ID = generateID()
	now := time.Now()
	app.CreatedAt = now
	app.UpdatedAt = now

	s.apps[app.ID] = app
	return app, nil
}

func (s *InMemoryAppStore) GetByID(id string) (*App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.apps[id]
	if !ok {
		return nil, fmt.Errorf("app not found: %s", id)
	}
	return app, nil
}

func (s *InMemoryAppStore) ListByOwner(ownerUserID string) ([]*App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*App
	for _, app := range s.apps {
		if app.OwnerUserID == ownerUserID {
			result = append(result, app)
		}
	}
	return result, nil
}

func (s *InMemoryAppStore) Update(id string, update *AppUpdate) (*App, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.apps[id]
	if !ok {
		return nil, fmt.Errorf("app not found: %s", id)
	}

	if update.Name != nil {
		app.Name = *update.Name
	}
	if update.Description != nil {
		app.Description = *update.Description
	}
	if update.CallbackURLs != nil {
		app.CallbackURLs = update.CallbackURLs
	}
	app.UpdatedAt = time.Now()

	return app, nil
}

func (s *InMemoryAppStore) UpdateStatus(id, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.apps[id]
	if !ok {
		return fmt.Errorf("app not found: %s", id)
	}

	app.Status = status
	app.UpdatedAt = time.Now()
	return nil
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 8: Implement key store**

Create `services/garudaportal/store/key.go`:

```go
package store

import (
	"fmt"
	"sync"
	"time"
)

// APIKey represents a stored API key (hash only, no plaintext).
type APIKey struct {
	ID          string     `json:"id"`
	AppID       string     `json:"app_id"`
	KeyHash     string     `json:"-"`
	KeyPrefix   string     `json:"key_prefix"`
	Name        string     `json:"name"`
	Environment string     `json:"environment"`
	Status      string     `json:"status"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	RevokedAt   *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// KeyStore defines the interface for API key storage operations.
type KeyStore interface {
	Create(key *APIKey) (*APIKey, error)
	GetByID(id string) (*APIKey, error)
	GetByHash(hash string) (*APIKey, error)
	ListByApp(appID string) ([]*APIKey, error)
	Revoke(id string) error
	UpdateLastUsed(id string, t time.Time) error
}

// InMemoryKeyStore is an in-memory implementation of KeyStore.
type InMemoryKeyStore struct {
	mu   sync.RWMutex
	keys map[string]*APIKey // id -> key
}

// NewInMemoryKeyStore creates a new in-memory key store.
func NewInMemoryKeyStore() *InMemoryKeyStore {
	return &InMemoryKeyStore{
		keys: make(map[string]*APIKey),
	}
}

func (s *InMemoryKeyStore) Create(key *APIKey) (*APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key.ID = generateID()
	key.CreatedAt = time.Now()

	s.keys[key.ID] = key
	return key, nil
}

func (s *InMemoryKeyStore) GetByID(id string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.keys[id]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", id)
	}
	return key, nil
}

func (s *InMemoryKeyStore) GetByHash(hash string) (*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, key := range s.keys {
		if key.KeyHash == hash {
			return key, nil
		}
	}
	return nil, nil // not found is not an error
}

func (s *InMemoryKeyStore) ListByApp(appID string) ([]*APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*APIKey
	for _, key := range s.keys {
		if key.AppID == appID {
			result = append(result, key)
		}
	}
	return result, nil
}

func (s *InMemoryKeyStore) Revoke(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.keys[id]
	if !ok {
		return fmt.Errorf("key not found: %s", id)
	}

	key.Status = "REVOKED"
	now := time.Now()
	key.RevokedAt = &now
	return nil
}

func (s *InMemoryKeyStore) UpdateLastUsed(id string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.keys[id]
	if !ok {
		return fmt.Errorf("key not found: %s", id)
	}

	key.LastUsedAt = &t
	return nil
}
```

- [ ] **Step 9: Implement webhook store**

Create `services/garudaportal/store/webhook.go`:

```go
package store

import (
	"fmt"
	"sync"
	"time"
)

// WebhookSubscription represents a webhook subscription.
type WebhookSubscription struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Secret    string    `json:"-"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebhookStore defines the interface for webhook subscription storage.
type WebhookStore interface {
	Create(wh *WebhookSubscription) (*WebhookSubscription, error)
	GetByID(id string) (*WebhookSubscription, error)
	ListByApp(appID string) ([]*WebhookSubscription, error)
	ListByAppAndEvent(appID, eventType string) ([]*WebhookSubscription, error)
	Disable(id string) error
}

// InMemoryWebhookStore is an in-memory implementation of WebhookStore.
type InMemoryWebhookStore struct {
	mu       sync.RWMutex
	webhooks map[string]*WebhookSubscription
}

// NewInMemoryWebhookStore creates a new in-memory webhook store.
func NewInMemoryWebhookStore() *InMemoryWebhookStore {
	return &InMemoryWebhookStore{
		webhooks: make(map[string]*WebhookSubscription),
	}
}

func (s *InMemoryWebhookStore) Create(wh *WebhookSubscription) (*WebhookSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	wh.ID = generateID()
	now := time.Now()
	wh.CreatedAt = now
	wh.UpdatedAt = now

	s.webhooks[wh.ID] = wh
	return wh, nil
}

func (s *InMemoryWebhookStore) GetByID(id string) (*WebhookSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wh, ok := s.webhooks[id]
	if !ok {
		return nil, fmt.Errorf("webhook not found: %s", id)
	}
	return wh, nil
}

func (s *InMemoryWebhookStore) ListByApp(appID string) ([]*WebhookSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WebhookSubscription
	for _, wh := range s.webhooks {
		if wh.AppID == appID {
			result = append(result, wh)
		}
	}
	return result, nil
}

func (s *InMemoryWebhookStore) ListByAppAndEvent(appID, eventType string) ([]*WebhookSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WebhookSubscription
	for _, wh := range s.webhooks {
		if wh.AppID != appID || wh.Status != "ACTIVE" {
			continue
		}
		for _, e := range wh.Events {
			if e == eventType {
				result = append(result, wh)
				break
			}
		}
	}
	return result, nil
}

func (s *InMemoryWebhookStore) Disable(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wh, ok := s.webhooks[id]
	if !ok {
		return fmt.Errorf("webhook not found: %s", id)
	}

	wh.Status = "DISABLED"
	wh.UpdatedAt = time.Now()
	return nil
}
```

- [ ] **Step 10: Implement delivery store**

Create `services/garudaportal/store/delivery.go`:

```go
package store

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// WebhookDelivery represents a webhook delivery attempt record.
type WebhookDelivery struct {
	ID               string          `json:"id"`
	SubscriptionID   string          `json:"subscription_id"`
	EventType        string          `json:"event_type"`
	Payload          json.RawMessage `json:"payload"`
	Status           string          `json:"status"` // PENDING, DELIVERED, FAILED
	Attempts         int             `json:"attempts"`
	LastResponseCode *int            `json:"last_response_code,omitempty"`
	LastResponseBody *string         `json:"last_response_body,omitempty"`
	NextRetryAt      *time.Time      `json:"next_retry_at,omitempty"`
	DeliveredAt      *time.Time      `json:"delivered_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

// DeliveryUpdate holds fields for updating a delivery record.
type DeliveryUpdate struct {
	Status           string
	Attempts         int
	LastResponseCode *int
	LastResponseBody *string
	NextRetryAt      *time.Time
	DeliveredAt      *time.Time
}

// DeliveryStore defines the interface for webhook delivery storage.
type DeliveryStore interface {
	Create(d *WebhookDelivery) (*WebhookDelivery, error)
	GetByID(id string) (*WebhookDelivery, error)
	ListBySubscription(subscriptionID string) ([]*WebhookDelivery, error)
	UpdateStatus(id string, update *DeliveryUpdate) error
	ListPendingRetries(before time.Time) ([]*WebhookDelivery, error)
}

// InMemoryDeliveryStore is an in-memory implementation of DeliveryStore.
type InMemoryDeliveryStore struct {
	mu         sync.RWMutex
	deliveries map[string]*WebhookDelivery
}

// NewInMemoryDeliveryStore creates a new in-memory delivery store.
func NewInMemoryDeliveryStore() *InMemoryDeliveryStore {
	return &InMemoryDeliveryStore{
		deliveries: make(map[string]*WebhookDelivery),
	}
}

func (s *InMemoryDeliveryStore) Create(d *WebhookDelivery) (*WebhookDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d.ID = generateID()
	d.CreatedAt = time.Now()

	s.deliveries[d.ID] = d
	return d, nil
}

func (s *InMemoryDeliveryStore) GetByID(id string) (*WebhookDelivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d, ok := s.deliveries[id]
	if !ok {
		return nil, fmt.Errorf("delivery not found: %s", id)
	}
	return d, nil
}

func (s *InMemoryDeliveryStore) ListBySubscription(subscriptionID string) ([]*WebhookDelivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WebhookDelivery
	for _, d := range s.deliveries {
		if d.SubscriptionID == subscriptionID {
			result = append(result, d)
		}
	}
	return result, nil
}

func (s *InMemoryDeliveryStore) UpdateStatus(id string, update *DeliveryUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, ok := s.deliveries[id]
	if !ok {
		return fmt.Errorf("delivery not found: %s", id)
	}

	d.Status = update.Status
	d.Attempts = update.Attempts
	if update.LastResponseCode != nil {
		d.LastResponseCode = update.LastResponseCode
	}
	if update.LastResponseBody != nil {
		d.LastResponseBody = update.LastResponseBody
	}
	if update.NextRetryAt != nil {
		d.NextRetryAt = update.NextRetryAt
	}
	if update.DeliveredAt != nil {
		d.DeliveredAt = update.DeliveredAt
	}

	return nil
}

func (s *InMemoryDeliveryStore) ListPendingRetries(before time.Time) ([]*WebhookDelivery, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*WebhookDelivery
	for _, d := range s.deliveries {
		if d.Status == "PENDING" && d.NextRetryAt != nil && d.NextRetryAt.Before(before) {
			result = append(result, d)
		}
	}
	return result, nil
}
```

- [ ] **Step 11: Implement usage store**

Create `services/garudaportal/store/usage.go`:

```go
package store

import (
	"fmt"
	"sync"
	"time"
)

// UsageRecord represents API usage for a specific app/date/endpoint.
type UsageRecord struct {
	ID         string    `json:"id"`
	AppID      string    `json:"app_id"`
	Date       time.Time `json:"date"`
	Endpoint   string    `json:"endpoint"`
	CallCount  int64     `json:"call_count"`
	ErrorCount int64     `json:"error_count"`
}

// UsageStore defines the interface for API usage storage.
type UsageStore interface {
	Increment(appID, endpoint string, date time.Time) error
	IncrementError(appID, endpoint string, date time.Time) error
	GetDailyTotal(appID string, date time.Time) (int64, error)
	GetByAppAndDateRange(appID string, from, to time.Time) ([]*UsageRecord, error)
}

// InMemoryUsageStore is an in-memory implementation of UsageStore.
type InMemoryUsageStore struct {
	mu      sync.RWMutex
	records map[string]*UsageRecord // "appID|date|endpoint" -> record
}

// NewInMemoryUsageStore creates a new in-memory usage store.
func NewInMemoryUsageStore() *InMemoryUsageStore {
	return &InMemoryUsageStore{
		records: make(map[string]*UsageRecord),
	}
}

func (s *InMemoryUsageStore) Increment(appID, endpoint string, date time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := usageKey(appID, date, endpoint)
	rec, ok := s.records[key]
	if !ok {
		rec = &UsageRecord{
			ID:       generateID(),
			AppID:    appID,
			Date:     date,
			Endpoint: endpoint,
		}
		s.records[key] = rec
	}
	rec.CallCount++
	return nil
}

func (s *InMemoryUsageStore) IncrementError(appID, endpoint string, date time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := usageKey(appID, date, endpoint)
	rec, ok := s.records[key]
	if !ok {
		rec = &UsageRecord{
			ID:       generateID(),
			AppID:    appID,
			Date:     date,
			Endpoint: endpoint,
		}
		s.records[key] = rec
	}
	rec.ErrorCount++
	return nil
}

func (s *InMemoryUsageStore) GetDailyTotal(appID string, date time.Time) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var total int64
	dateStr := date.Format("2006-01-02")
	for _, rec := range s.records {
		if rec.AppID == appID && rec.Date.Format("2006-01-02") == dateStr {
			total += rec.CallCount
		}
	}
	return total, nil
}

func (s *InMemoryUsageStore) GetByAppAndDateRange(appID string, from, to time.Time) ([]*UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fromStr := from.Format("2006-01-02")
	toStr := to.Format("2006-01-02")

	var result []*UsageRecord
	for _, rec := range s.records {
		if rec.AppID != appID {
			continue
		}
		recStr := rec.Date.Format("2006-01-02")
		if recStr >= fromStr && recStr <= toStr {
			result = append(result, rec)
		}
	}
	return result, nil
}

func usageKey(appID string, date time.Time, endpoint string) string {
	return fmt.Sprintf("%s|%s|%s", appID, date.Format("2006-01-02"), endpoint)
}
```

- [ ] **Step 12: Run tests to verify they pass**

```bash
cd services/garudaportal && go test ./store/... -v -count=1
```

Expected: All store tests PASS.

- [ ] **Step 13: Commit**

```bash
git add services/garudaportal/store/
git commit -m "feat(garudaportal): add in-memory stores for apps, keys, webhooks, deliveries, and usage"
```

---

### Task 6: App + Key Handlers

**Files:**
- Create: `services/garudaportal/handler/app.go`
- Create: `services/garudaportal/handler/app_test.go`
- Create: `services/garudaportal/handler/key.go`
- Create: `services/garudaportal/handler/key_test.go`

- [ ] **Step 1: Write failing app handler tests**

Create `services/garudaportal/handler/app_test.go`:

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func setupAppHandler() *AppHandler {
	appStore := store.NewInMemoryAppStore()
	return NewAppHandler(appStore)
}

func TestCreateApp_Success(t *testing.T) {
	h := setupAppHandler()

	body := `{"name":"My Test App","description":"A test application","callback_urls":["https://example.com/callback"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["name"] != "My Test App" {
		t.Errorf("expected name 'My Test App', got %v", resp["name"])
	}
	if resp["environment"] != "sandbox" {
		t.Errorf("expected environment 'sandbox', got %v", resp["environment"])
	}
	if resp["tier"] != "free" {
		t.Errorf("expected tier 'free', got %v", resp["tier"])
	}
	if resp["status"] != "ACTIVE" {
		t.Errorf("expected status 'ACTIVE', got %v", resp["status"])
	}
}

func TestCreateApp_MissingName(t *testing.T) {
	h := setupAppHandler()

	body := `{"description":"No name"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateApp_MissingUserID(t *testing.T) {
	h := setupAppHandler()

	body := `{"name":"App"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateApp_InvalidJSON(t *testing.T) {
	h := setupAppHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString("{invalid"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateApp_InvalidCallbackURL(t *testing.T) {
	h := setupAppHandler()

	body := `{"name":"App","callback_urls":["not-a-url"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListApps_Success(t *testing.T) {
	h := setupAppHandler()

	// Create two apps
	for _, name := range []string{"App 1", "App 2"} {
		body, _ := json.Marshal(map[string]string{"name": name})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "user-123")
		w := httptest.NewRecorder()
		h.Create(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps", nil)
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	apps := resp["apps"].([]interface{})
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
	}
}

func TestGetApp_Success(t *testing.T) {
	h := setupAppHandler()

	body := `{"name":"Get Me"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-User-ID", "user-123")
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	var created map[string]interface{}
	json.Unmarshal(createW.Body.Bytes(), &created)
	appID := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+appID, nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("id", appID)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetApp_NotOwner(t *testing.T) {
	h := setupAppHandler()

	body := `{"name":"Owned App"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-User-ID", "user-123")
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	var created map[string]interface{}
	json.Unmarshal(createW.Body.Bytes(), &created)
	appID := created["id"].(string)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+appID, nil)
	req.Header.Set("X-User-ID", "other-user")
	req.SetPathValue("id", appID)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestUpdateApp_Success(t *testing.T) {
	h := setupAppHandler()

	body := `{"name":"Original"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-User-ID", "user-123")
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	var created map[string]interface{}
	json.Unmarshal(createW.Body.Bytes(), &created)
	appID := created["id"].(string)

	updateBody := `{"name":"Updated"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/portal/apps/"+appID, bytes.NewBufferString(updateBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("id", appID)
	w := httptest.NewRecorder()

	h.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "Updated" {
		t.Errorf("expected name 'Updated', got %v", resp["name"])
	}
}
```

- [ ] **Step 2: Write failing key handler tests**

Create `services/garudaportal/handler/key_test.go`:

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func setupKeyHandler() (*KeyHandler, *store.InMemoryAppStore, string) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()

	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-123",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
		Status:      "ACTIVE",
	})

	h := NewKeyHandler(keyStore, appStore)
	return h, appStore, app.ID
}

func TestCreateKey_Success(t *testing.T) {
	h, _, appID := setupKeyHandler()

	body := `{"name":"My API Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["plaintext_key"] == nil || resp["plaintext_key"] == "" {
		t.Error("plaintext_key should be present in creation response")
	}
	if resp["key_prefix"] == nil || resp["key_prefix"] == "" {
		t.Error("key_prefix should be present")
	}
	if resp["name"] != "My API Key" {
		t.Errorf("expected name 'My API Key', got %v", resp["name"])
	}
}

func TestCreateKey_DefaultName(t *testing.T) {
	h, _, appID := setupKeyHandler()

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "Default" {
		t.Errorf("expected name 'Default', got %v", resp["name"])
	}
}

func TestCreateKey_NotOwner(t *testing.T) {
	h, _, appID := setupKeyHandler()

	body := `{"name":"Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "other-user")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestListKeys_Success(t *testing.T) {
	h, _, appID := setupKeyHandler()

	// Create two keys
	for _, name := range []string{"Key 1", "Key 2"} {
		body, _ := json.Marshal(map[string]string{"name": name})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/keys", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User-ID", "user-123")
		req.SetPathValue("app_id", appID)
		w := httptest.NewRecorder()
		h.Create(w, req)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+appID+"/keys", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	keys := resp["keys"].([]interface{})
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}

	// Verify plaintext_key is NOT in list response
	for _, k := range keys {
		km := k.(map[string]interface{})
		if _, ok := km["plaintext_key"]; ok {
			t.Error("plaintext_key should NOT be in list response")
		}
	}
}

func TestRevokeKey_Success(t *testing.T) {
	h, _, appID := setupKeyHandler()

	// Create a key
	body := `{"name":"To Revoke"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/keys", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-User-ID", "user-123")
	createReq.SetPathValue("app_id", appID)
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	var created map[string]interface{}
	json.Unmarshal(createW.Body.Bytes(), &created)
	keyID := created["id"].(string)

	// Revoke it
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/portal/apps/"+appID+"/keys/"+keyID, nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	req.SetPathValue("key_id", keyID)
	w := httptest.NewRecorder()

	h.Revoke(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "REVOKED" {
		t.Errorf("expected status REVOKED, got %v", resp["status"])
	}
}

func TestCreateKey_WithExpiry(t *testing.T) {
	h, _, appID := setupKeyHandler()

	body := `{"name":"Expiring Key","expires_in_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["expires_at"] == nil {
		t.Error("expires_at should be set for expiring keys")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd services/garudaportal && go test ./handler/... -v -count=1
```

Expected: FAIL — `handler` package doesn't exist yet.

- [ ] **Step 4: Implement app handler**

Create `services/garudaportal/handler/app.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

type createAppRequest struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	CallbackURLs []string `json:"callback_urls"`
}

type updateAppRequest struct {
	Name         *string  `json:"name"`
	Description  *string  `json:"description"`
	CallbackURLs []string `json:"callback_urls"`
}

// AppHandler handles developer app CRUD operations.
type AppHandler struct {
	appStore store.AppStore
}

// NewAppHandler creates a new AppHandler.
func NewAppHandler(appStore store.AppStore) *AppHandler {
	return &AppHandler{appStore: appStore}
}

// Create handles POST /api/v1/portal/apps.
func (h *AppHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	var req createAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	// Validate callback URLs
	for _, u := range req.CallbackURLs {
		parsed, err := url.ParseRequestURI(u)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid callback URL: " + u})
			return
		}
	}

	app := &store.App{
		OwnerUserID:  userID,
		Name:         req.Name,
		Description:  req.Description,
		Environment:  "sandbox",
		Tier:         "free",
		DailyLimit:   100,
		CallbackURLs: req.CallbackURLs,
		Status:       "ACTIVE",
	}

	created, err := h.appStore.Create(app)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create app"})
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// List handles GET /api/v1/portal/apps.
func (h *AppHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	apps, err := h.appStore.ListByOwner(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list apps"})
		return
	}

	if apps == nil {
		apps = []*store.App{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"apps": apps})
}

// Get handles GET /api/v1/portal/apps/{id}.
func (h *AppHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	appID := r.PathValue("id")
	if appID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app ID is required"})
		return
	}

	app, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	if app.OwnerUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	writeJSON(w, http.StatusOK, app)
}

// Update handles PATCH /api/v1/portal/apps/{id}.
func (h *AppHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	appID := r.PathValue("id")
	if appID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app ID is required"})
		return
	}

	app, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}

	if app.OwnerUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	var req updateAppRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	// Validate callback URLs if provided
	for _, u := range req.CallbackURLs {
		parsed, err := url.ParseRequestURI(u)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid callback URL: " + u})
			return
		}
	}

	updated, err := h.appStore.Update(appID, &store.AppUpdate{
		Name:         req.Name,
		Description:  req.Description,
		CallbackURLs: req.CallbackURLs,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update app"})
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 5: Implement key handler**

Create `services/garudaportal/handler/key.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/apikey"
	"github.com/garudapass/gpass/services/garudaportal/store"
)

type createKeyRequest struct {
	Name          string `json:"name"`
	ExpiresInDays int    `json:"expires_in_days"`
}

type createKeyResponse struct {
	ID           string     `json:"id"`
	KeyPrefix    string     `json:"key_prefix"`
	Name         string     `json:"name"`
	Environment  string     `json:"environment"`
	Status       string     `json:"status"`
	PlaintextKey string     `json:"plaintext_key"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

type revokeKeyResponse struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	RevokedAt time.Time `json:"revoked_at"`
}

// KeyHandler handles API key management operations.
type KeyHandler struct {
	keyStore store.KeyStore
	appStore store.AppStore
}

// NewKeyHandler creates a new KeyHandler.
func NewKeyHandler(keyStore store.KeyStore, appStore store.AppStore) *KeyHandler {
	return &KeyHandler{
		keyStore: keyStore,
		appStore: appStore,
	}
}

// Create handles POST /api/v1/portal/apps/{app_id}/keys.
func (h *KeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	appID := r.PathValue("app_id")
	if appID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app_id is required"})
		return
	}

	// Verify app ownership
	app, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if app.OwnerUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	var req createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body
		req = createKeyRequest{}
	}

	if req.Name == "" {
		req.Name = "Default"
	}

	// Generate plaintext key
	plaintext, err := apikey.Generate(app.Environment)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate key"})
		return
	}

	keyHash := apikey.HashKey(plaintext)
	keyPrefix := apikey.ExtractPrefix(plaintext)

	var expiresAt *time.Time
	if req.ExpiresInDays > 0 {
		exp := time.Now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &exp
	}

	created, err := h.keyStore.Create(&store.APIKey{
		AppID:       appID,
		KeyHash:     keyHash,
		KeyPrefix:   keyPrefix,
		Name:        req.Name,
		Environment: app.Environment,
		Status:      "ACTIVE",
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create key"})
		return
	}

	writeJSON(w, http.StatusCreated, createKeyResponse{
		ID:           created.ID,
		KeyPrefix:    created.KeyPrefix,
		Name:         created.Name,
		Environment:  created.Environment,
		Status:       created.Status,
		PlaintextKey: plaintext,
		ExpiresAt:    expiresAt,
		CreatedAt:    created.CreatedAt,
	})
}

// List handles GET /api/v1/portal/apps/{app_id}/keys.
func (h *KeyHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	appID := r.PathValue("app_id")
	if appID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app_id is required"})
		return
	}

	app, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if app.OwnerUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	keys, err := h.keyStore.ListByApp(appID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list keys"})
		return
	}

	if keys == nil {
		keys = []*store.APIKey{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"keys": keys})
}

// Revoke handles DELETE /api/v1/portal/apps/{app_id}/keys/{key_id}.
func (h *KeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	appID := r.PathValue("app_id")
	keyID := r.PathValue("key_id")
	if appID == "" || keyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app_id and key_id are required"})
		return
	}

	app, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if app.OwnerUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	// Verify key belongs to app
	key, err := h.keyStore.GetByID(keyID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "key not found"})
		return
	}
	if key.AppID != appID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "key not found"})
		return
	}

	if err := h.keyStore.Revoke(keyID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to revoke key"})
		return
	}

	writeJSON(w, http.StatusOK, revokeKeyResponse{
		ID:        keyID,
		Status:    "REVOKED",
		RevokedAt: time.Now(),
	})
}
```

- [ ] **Step 6: Run tests to verify they pass**

```bash
cd services/garudaportal && go test ./handler/... -v -count=1
```

Expected: All handler tests PASS.

- [ ] **Step 7: Commit**

```bash
git add services/garudaportal/handler/app.go services/garudaportal/handler/app_test.go services/garudaportal/handler/key.go services/garudaportal/handler/key_test.go
git commit -m "feat(garudaportal): add app CRUD and key management handlers with ownership enforcement"
```

---

### Task 7: Webhook + Usage + Validate Handlers

**Files:**
- Create: `services/garudaportal/handler/webhook.go`
- Create: `services/garudaportal/handler/webhook_test.go`
- Create: `services/garudaportal/handler/usage.go`
- Create: `services/garudaportal/handler/usage_test.go`
- Create: `services/garudaportal/handler/validate.go`
- Create: `services/garudaportal/handler/validate_test.go`

- [ ] **Step 1: Write failing webhook handler tests**

Create `services/garudaportal/handler/webhook_test.go`:

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func setupWebhookHandler() (*WebhookHandler, string) {
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()

	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-123",
		Name:        "Test App",
		Environment: "sandbox",
		Status:      "ACTIVE",
	})

	h := NewWebhookHandler(webhookStore, appStore)
	return h, app.ID
}

func TestCreateWebhook_Success(t *testing.T) {
	h, appID := setupWebhookHandler()

	body := `{"url":"https://example.com/webhook","events":["identity.verified","document.signed"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/webhooks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["url"] != "https://example.com/webhook" {
		t.Errorf("expected URL, got %v", resp["url"])
	}
	if resp["secret"] == nil || resp["secret"] == "" {
		t.Error("secret should be present in creation response")
	}
	if resp["status"] != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %v", resp["status"])
	}
}

func TestCreateWebhook_MissingURL(t *testing.T) {
	h, appID := setupWebhookHandler()

	body := `{"events":["identity.verified"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/webhooks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCreateWebhook_NonHTTPS(t *testing.T) {
	h, appID := setupWebhookHandler()

	body := `{"url":"http://example.com/webhook","events":["identity.verified"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/webhooks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-HTTPS URL, got %d", w.Code)
	}
}

func TestCreateWebhook_EmptyEvents(t *testing.T) {
	h, appID := setupWebhookHandler()

	body := `{"url":"https://example.com/webhook","events":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/webhooks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty events, got %d", w.Code)
	}
}

func TestListWebhooks_Success(t *testing.T) {
	h, appID := setupWebhookHandler()

	// Create a webhook
	body := `{"url":"https://example.com/webhook","events":["identity.verified"]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/webhooks", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-User-ID", "user-123")
	createReq.SetPathValue("app_id", appID)
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+appID+"/webhooks", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	webhooks := resp["webhooks"].([]interface{})
	if len(webhooks) != 1 {
		t.Errorf("expected 1 webhook, got %d", len(webhooks))
	}
}

func TestDeleteWebhook_Success(t *testing.T) {
	h, appID := setupWebhookHandler()

	body := `{"url":"https://example.com/webhook","events":["identity.verified"]}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+appID+"/webhooks", bytes.NewBufferString(body))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("X-User-ID", "user-123")
	createReq.SetPathValue("app_id", appID)
	createW := httptest.NewRecorder()
	h.Create(createW, createReq)

	var created map[string]interface{}
	json.Unmarshal(createW.Body.Bytes(), &created)
	webhookID := created["id"].(string)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/portal/apps/"+appID+"/webhooks/"+webhookID, nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	req.SetPathValue("webhook_id", webhookID)
	w := httptest.NewRecorder()

	h.Delete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "DISABLED" {
		t.Errorf("expected DISABLED, got %v", resp["status"])
	}
}
```

- [ ] **Step 2: Write failing usage handler tests**

Create `services/garudaportal/handler/usage_test.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func setupUsageHandler() (*UsageHandler, string) {
	appStore := store.NewInMemoryAppStore()
	usageStore := store.NewInMemoryUsageStore()

	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-123",
		Name:        "Test App",
		Status:      "ACTIVE",
	})

	// Add some usage data
	d1 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC)

	usageStore.Increment(app.ID, "/api/v1/identity/verify", d1)
	usageStore.Increment(app.ID, "/api/v1/identity/verify", d1)
	usageStore.Increment(app.ID, "/api/v1/corp/entity", d2)
	usageStore.IncrementError(app.ID, "/api/v1/identity/verify", d1)

	h := NewUsageHandler(usageStore, appStore)
	return h, app.ID
}

func TestGetUsage_Success(t *testing.T) {
	h, appID := setupUsageHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+appID+"/usage?from=2026-04-01&to=2026-04-06", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	totalCalls := resp["total_calls"].(float64)
	if totalCalls != 3 {
		t.Errorf("expected 3 total calls, got %v", totalCalls)
	}

	totalErrors := resp["total_errors"].(float64)
	if totalErrors != 1 {
		t.Errorf("expected 1 total error, got %v", totalErrors)
	}
}

func TestGetUsage_MissingDates(t *testing.T) {
	h, appID := setupUsageHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+appID+"/usage", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetUsage_NotOwner(t *testing.T) {
	h, appID := setupUsageHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+appID+"/usage?from=2026-04-01&to=2026-04-06", nil)
	req.Header.Set("X-User-ID", "other-user")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestGetUsage_InvalidDateFormat(t *testing.T) {
	h, appID := setupUsageHandler()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+appID+"/usage?from=invalid&to=2026-04-06", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.SetPathValue("app_id", appID)
	w := httptest.NewRecorder()

	h.Get(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
```

- [ ] **Step 3: Write failing validate handler tests**

Create `services/garudaportal/handler/validate_test.go`:

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apikeyPkg "github.com/garudapass/gpass/services/garudaportal/apikey"
	"github.com/garudapass/gpass/services/garudaportal/store"
)

func setupValidateHandler() (*ValidateHandler, string) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	usageStore := store.NewInMemoryUsageStore()

	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-123",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
		Status:      "ACTIVE",
	})

	// Create a valid key
	plaintext, _ := apikeyPkg.Generate("sandbox")
	keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     apikeyPkg.HashKey(plaintext),
		KeyPrefix:   apikeyPkg.ExtractPrefix(plaintext),
		Name:        "Valid Key",
		Environment: "sandbox",
		Status:      "ACTIVE",
	})

	h := NewValidateHandler(keyStore, appStore, usageStore)
	return h, plaintext
}

func TestValidate_Success(t *testing.T) {
	h, plaintext := setupValidateHandler()

	body, _ := json.Marshal(map[string]string{"api_key": plaintext})
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Validate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["valid"] != true {
		t.Errorf("expected valid=true, got %v", resp["valid"])
	}
	if resp["environment"] != "sandbox" {
		t.Errorf("expected sandbox, got %v", resp["environment"])
	}
	if resp["tier"] != "free" {
		t.Errorf("expected free, got %v", resp["tier"])
	}
}

func TestValidate_InvalidKey(t *testing.T) {
	h, _ := setupValidateHandler()

	body, _ := json.Marshal(map[string]string{"api_key": "gp_test_nonexistent_key_here"})
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Validate(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp["valid"] != false {
		t.Errorf("expected valid=false, got %v", resp["valid"])
	}
}

func TestValidate_MissingKey(t *testing.T) {
	h, _ := setupValidateHandler()

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Validate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestValidate_RateLimited(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	usageStore := store.NewInMemoryUsageStore()

	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-123",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  1, // Very low limit
		Status:      "ACTIVE",
	})

	plaintext, _ := apikeyPkg.Generate("sandbox")
	keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     apikeyPkg.HashKey(plaintext),
		KeyPrefix:   apikeyPkg.ExtractPrefix(plaintext),
		Name:        "Limited Key",
		Environment: "sandbox",
		Status:      "ACTIVE",
	})

	// Use up the quota
	today := time.Now().UTC().Truncate(24 * time.Hour)
	usageStore.Increment(app.ID, "/test", today)

	h := NewValidateHandler(keyStore, appStore, usageStore)

	body, _ := json.Marshal(map[string]string{"api_key": plaintext})
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Validate(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 4: Run tests to verify they fail**

```bash
cd services/garudaportal && go test ./handler/... -v -count=1
```

Expected: FAIL — `WebhookHandler`, `UsageHandler`, `ValidateHandler` don't exist yet.

- [ ] **Step 5: Implement webhook handler**

Create `services/garudaportal/handler/webhook.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"net/url"

	webhookPkg "github.com/garudapass/gpass/services/garudaportal/webhook"
	"github.com/garudapass/gpass/services/garudaportal/store"
)

type createWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

type createWebhookResponse struct {
	ID     string   `json:"id"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Status string   `json:"status"`
	Secret string   `json:"secret"`
}

// WebhookHandler handles webhook subscription operations.
type WebhookHandler struct {
	webhookStore store.WebhookStore
	appStore     store.AppStore
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(webhookStore store.WebhookStore, appStore store.AppStore) *WebhookHandler {
	return &WebhookHandler{
		webhookStore: webhookStore,
		appStore:     appStore,
	}
}

// Create handles POST /api/v1/portal/apps/{app_id}/webhooks.
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	appID := r.PathValue("app_id")
	if appID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app_id is required"})
		return
	}

	app, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if app.OwnerUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	var req createWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	// Validate HTTPS
	parsed, err := url.ParseRequestURI(req.URL)
	if err != nil || parsed.Scheme != "https" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "webhook URL must use HTTPS"})
		return
	}

	if len(req.Events) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one event is required"})
		return
	}

	secret, err := webhookPkg.GenerateSecret()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate secret"})
		return
	}

	created, err := h.webhookStore.Create(&store.WebhookSubscription{
		AppID:  appID,
		URL:    req.URL,
		Events: req.Events,
		Secret: secret,
		Status: "ACTIVE",
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create webhook"})
		return
	}

	writeJSON(w, http.StatusCreated, createWebhookResponse{
		ID:     created.ID,
		URL:    created.URL,
		Events: created.Events,
		Status: created.Status,
		Secret: secret,
	})
}

// List handles GET /api/v1/portal/apps/{app_id}/webhooks.
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	appID := r.PathValue("app_id")
	app, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if app.OwnerUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	webhooks, err := h.webhookStore.ListByApp(appID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list webhooks"})
		return
	}

	if webhooks == nil {
		webhooks = []*store.WebhookSubscription{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"webhooks": webhooks})
}

// Delete handles DELETE /api/v1/portal/apps/{app_id}/webhooks/{webhook_id}.
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	appID := r.PathValue("app_id")
	webhookID := r.PathValue("webhook_id")

	app, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if app.OwnerUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	wh, err := h.webhookStore.GetByID(webhookID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
		return
	}
	if wh.AppID != appID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "webhook not found"})
		return
	}

	if err := h.webhookStore.Disable(webhookID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to disable webhook"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":     webhookID,
		"status": "DISABLED",
	})
}
```

- [ ] **Step 6: Implement usage handler**

Create `services/garudaportal/handler/usage.go`:

```go
package handler

import (
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

type usageResponse struct {
	TotalCalls  int64              `json:"total_calls"`
	TotalErrors int64              `json:"total_errors"`
	Daily       []dailyUsage       `json:"daily"`
	ByEndpoint  []endpointUsage    `json:"by_endpoint"`
}

type dailyUsage struct {
	Date   string `json:"date"`
	Calls  int64  `json:"calls"`
	Errors int64  `json:"errors"`
}

type endpointUsage struct {
	Endpoint string `json:"endpoint"`
	Calls    int64  `json:"calls"`
	Errors   int64  `json:"errors"`
}

// UsageHandler handles usage stats operations.
type UsageHandler struct {
	usageStore store.UsageStore
	appStore   store.AppStore
}

// NewUsageHandler creates a new UsageHandler.
func NewUsageHandler(usageStore store.UsageStore, appStore store.AppStore) *UsageHandler {
	return &UsageHandler{
		usageStore: usageStore,
		appStore:   appStore,
	}
}

// Get handles GET /api/v1/portal/apps/{app_id}/usage.
func (h *UsageHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-User-ID header is required"})
		return
	}

	appID := r.PathValue("app_id")
	if appID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app_id is required"})
		return
	}

	app, err := h.appStore.GetByID(appID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	if app.OwnerUserID != userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "not_owner"})
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from and to query parameters are required (format: 2006-01-02)"})
		return
	}

	from, err := time.Parse("2006-01-02", fromStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid from date format, expected 2006-01-02"})
		return
	}

	to, err := time.Parse("2006-01-02", toStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid to date format, expected 2006-01-02"})
		return
	}

	records, err := h.usageStore.GetByAppAndDateRange(appID, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get usage"})
		return
	}

	// Aggregate
	var totalCalls, totalErrors int64
	dailyMap := make(map[string]*dailyUsage)
	endpointMap := make(map[string]*endpointUsage)

	for _, rec := range records {
		totalCalls += rec.CallCount
		totalErrors += rec.ErrorCount

		dateKey := rec.Date.Format("2006-01-02")
		if d, ok := dailyMap[dateKey]; ok {
			d.Calls += rec.CallCount
			d.Errors += rec.ErrorCount
		} else {
			dailyMap[dateKey] = &dailyUsage{
				Date:   dateKey,
				Calls:  rec.CallCount,
				Errors: rec.ErrorCount,
			}
		}

		if e, ok := endpointMap[rec.Endpoint]; ok {
			e.Calls += rec.CallCount
			e.Errors += rec.ErrorCount
		} else {
			endpointMap[rec.Endpoint] = &endpointUsage{
				Endpoint: rec.Endpoint,
				Calls:    rec.CallCount,
				Errors:   rec.ErrorCount,
			}
		}
	}

	var daily []dailyUsage
	for _, d := range dailyMap {
		daily = append(daily, *d)
	}

	var byEndpoint []endpointUsage
	for _, e := range endpointMap {
		byEndpoint = append(byEndpoint, *e)
	}

	if daily == nil {
		daily = []dailyUsage{}
	}
	if byEndpoint == nil {
		byEndpoint = []endpointUsage{}
	}

	writeJSON(w, http.StatusOK, usageResponse{
		TotalCalls:  totalCalls,
		TotalErrors: totalErrors,
		Daily:       daily,
		ByEndpoint:  byEndpoint,
	})
}
```

- [ ] **Step 7: Implement validate handler**

Create `services/garudaportal/handler/validate.go`:

```go
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/apikey"
	"github.com/garudapass/gpass/services/garudaportal/store"
)

type validateRequest struct {
	APIKey string `json:"api_key"`
}

// ValidateHandler handles internal key validation requests from Kong.
type ValidateHandler struct {
	keyStore   store.KeyStore
	appStore   store.AppStore
	usageStore store.UsageStore
}

// NewValidateHandler creates a new ValidateHandler.
func NewValidateHandler(keyStore store.KeyStore, appStore store.AppStore, usageStore store.UsageStore) *ValidateHandler {
	return &ValidateHandler{
		keyStore:   keyStore,
		appStore:   appStore,
		usageStore: usageStore,
	}
}

// Validate handles POST /internal/keys/validate.
func (h *ValidateHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.APIKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"valid": false,
			"error": "api_key is required",
		})
		return
	}

	// Hash and lookup
	hash := apikey.HashKey(req.APIKey)
	keyInfo, err := h.keyStore.GetByHash(hash)
	if err != nil || keyInfo == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"valid": false,
			"error": "invalid_key",
		})
		return
	}

	// Check key status
	if keyInfo.Status == "REVOKED" {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"valid": false,
			"error": "key_revoked",
		})
		return
	}

	// Check expiry
	if keyInfo.ExpiresAt != nil && time.Now().After(*keyInfo.ExpiresAt) {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"valid": false,
			"error": "key_expired",
		})
		return
	}

	// Check app status
	app, err := h.appStore.GetByID(keyInfo.AppID)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"valid": false,
			"error": "app_not_found",
		})
		return
	}
	if app.Status != "ACTIVE" {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"valid": false,
			"error": "app_suspended",
		})
		return
	}

	// Check rate limit
	today := time.Now().UTC().Truncate(24 * time.Hour)
	usage, err := h.usageStore.GetDailyTotal(keyInfo.AppID, today)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "usage_check_failed"})
		return
	}
	if usage >= int64(app.DailyLimit) {
		writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
			"valid": false,
			"error": "rate_limit_exceeded",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":       true,
		"app_id":      app.ID,
		"environment": app.Environment,
		"tier":        app.Tier,
		"daily_limit": app.DailyLimit,
	})
}
```

- [ ] **Step 8: Run tests to verify they pass**

```bash
cd services/garudaportal && go test ./handler/... -v -count=1
```

Expected: All handler tests PASS.

- [ ] **Step 9: Commit**

```bash
git add services/garudaportal/handler/webhook.go services/garudaportal/handler/webhook_test.go services/garudaportal/handler/usage.go services/garudaportal/handler/usage_test.go services/garudaportal/handler/validate.go services/garudaportal/handler/validate_test.go
git commit -m "feat(garudaportal): add webhook, usage, and key validation handlers"
```

---

### Task 8: Main + Dockerfile + Migrations

**Files:**
- Create: `services/garudaportal/main.go`
- Create: `services/garudaportal/Dockerfile`
- Create: `infrastructure/db/migrations/010_create_developer_apps.sql`
- Create: `infrastructure/db/migrations/011_create_api_keys.sql`
- Create: `infrastructure/db/migrations/012_create_webhook_subscriptions.sql`
- Create: `infrastructure/db/migrations/013_create_webhook_deliveries.sql`
- Create: `infrastructure/db/migrations/014_create_api_usage.sql`

- [ ] **Step 1: Create main.go**

Create `services/garudaportal/main.go`:

```go
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

	"github.com/garudapass/gpass/services/garudaportal/config"
	"github.com/garudapass/gpass/services/garudaportal/handler"
	"github.com/garudapass/gpass/services/garudaportal/store"
	"github.com/garudapass/gpass/services/garudaportal/webhook"
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

	slog.Info("starting GarudaPortal service",
		"port", cfg.Port,
		"webhook_timeout", cfg.WebhookTimeout,
		"webhook_max_retries", cfg.WebhookMaxRetries,
	)

	// Stores (in-memory for MVP)
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	webhookStore := store.NewInMemoryWebhookStore()
	deliveryStore := store.NewInMemoryDeliveryStore()
	usageStore := store.NewInMemoryUsageStore()

	// Webhook dispatcher
	_ = webhook.NewDispatcher(cfg.WebhookTimeout, cfg.WebhookMaxRetries)

	// Handlers
	appHandler := handler.NewAppHandler(appStore)
	keyHandler := handler.NewKeyHandler(keyStore, appStore)
	webhookHandler := handler.NewWebhookHandler(webhookStore, appStore)
	usageHandler := handler.NewUsageHandler(usageStore, appStore)
	validateHandler := handler.NewValidateHandler(keyStore, appStore, usageStore)

	// Suppress unused variable
	_ = deliveryStore

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok","service":"garudaportal"}`)
	})

	// App APIs
	mux.HandleFunc("POST /api/v1/portal/apps", appHandler.Create)
	mux.HandleFunc("GET /api/v1/portal/apps", appHandler.List)
	mux.HandleFunc("GET /api/v1/portal/apps/{id}", appHandler.Get)
	mux.HandleFunc("PATCH /api/v1/portal/apps/{id}", appHandler.Update)

	// Key APIs
	mux.HandleFunc("POST /api/v1/portal/apps/{app_id}/keys", keyHandler.Create)
	mux.HandleFunc("GET /api/v1/portal/apps/{app_id}/keys", keyHandler.List)
	mux.HandleFunc("DELETE /api/v1/portal/apps/{app_id}/keys/{key_id}", keyHandler.Revoke)

	// Webhook APIs
	mux.HandleFunc("POST /api/v1/portal/apps/{app_id}/webhooks", webhookHandler.Create)
	mux.HandleFunc("GET /api/v1/portal/apps/{app_id}/webhooks", webhookHandler.List)
	mux.HandleFunc("DELETE /api/v1/portal/apps/{app_id}/webhooks/{webhook_id}", webhookHandler.Delete)

	// Usage APIs
	mux.HandleFunc("GET /api/v1/portal/apps/{app_id}/usage", usageHandler.Get)

	// Internal APIs (Kong → Portal)
	mux.HandleFunc("POST /internal/keys/validate", validateHandler.Validate)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("GarudaPortal service listening", "addr", server.Addr)
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
	slog.Info("GarudaPortal service shut down gracefully")
}
```

- [ ] **Step 2: Create Dockerfile**

Create `services/garudaportal/Dockerfile`:

```dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o garudaportal .

FROM alpine:3.20

RUN apk --no-cache add ca-certificates \
    && adduser -D -u 1000 appuser

WORKDIR /app

COPY --from=builder /build/garudaportal .

USER appuser

EXPOSE 4009

ENTRYPOINT ["./garudaportal"]
```

- [ ] **Step 3: Create database migrations**

Create `infrastructure/db/migrations/010_create_developer_apps.sql`:

```sql
CREATE TABLE IF NOT EXISTS developer_apps (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    environment VARCHAR(10) NOT NULL DEFAULT 'sandbox',
    tier VARCHAR(20) NOT NULL DEFAULT 'free',
    daily_limit INT NOT NULL DEFAULT 100,
    callback_urls TEXT[] NOT NULL DEFAULT '{}',
    oauth_client_id VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_dev_apps_owner ON developer_apps(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_dev_apps_status ON developer_apps(status);
CREATE INDEX IF NOT EXISTS idx_dev_apps_oauth_client ON developer_apps(oauth_client_id);
```

Create `infrastructure/db/migrations/011_create_api_keys.sql`:

```sql
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES developer_apps(id),
    key_hash VARCHAR(64) NOT NULL,
    key_prefix VARCHAR(16) NOT NULL,
    name VARCHAR(100) NOT NULL DEFAULT 'Default',
    environment VARCHAR(10) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_app ON api_keys(app_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_prefix ON api_keys(key_prefix);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
```

Create `infrastructure/db/migrations/012_create_webhook_subscriptions.sql`:

```sql
CREATE TABLE IF NOT EXISTS webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES developer_apps(id),
    url VARCHAR(2048) NOT NULL,
    events TEXT[] NOT NULL,
    secret VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhooks_app ON webhook_subscriptions(app_id);
CREATE INDEX IF NOT EXISTS idx_webhooks_status ON webhook_subscriptions(status);
```

Create `infrastructure/db/migrations/013_create_webhook_deliveries.sql`:

```sql
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id),
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    attempts INT NOT NULL DEFAULT 0,
    last_response_code INT,
    last_response_body TEXT,
    next_retry_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_deliveries_sub ON webhook_deliveries(subscription_id);
CREATE INDEX IF NOT EXISTS idx_deliveries_status ON webhook_deliveries(status);
CREATE INDEX IF NOT EXISTS idx_deliveries_retry ON webhook_deliveries(next_retry_at) WHERE status = 'PENDING';
```

Create `infrastructure/db/migrations/014_create_api_usage.sql`:

```sql
CREATE TABLE IF NOT EXISTS api_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id UUID NOT NULL REFERENCES developer_apps(id),
    date DATE NOT NULL,
    endpoint VARCHAR(200) NOT NULL,
    call_count BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    UNIQUE(app_id, date, endpoint)
);

CREATE INDEX IF NOT EXISTS idx_usage_app_date ON api_usage(app_id, date);
```

- [ ] **Step 4: Build to verify compilation**

```bash
cd services/garudaportal && go build -o /dev/null .
```

Expected: Successful build.

- [ ] **Step 5: Run all tests**

```bash
cd services/garudaportal && go test ./... -v -count=1
```

Expected: All tests PASS across all packages.

- [ ] **Step 6: Commit**

```bash
git add services/garudaportal/main.go services/garudaportal/Dockerfile infrastructure/db/migrations/010_create_developer_apps.sql infrastructure/db/migrations/011_create_api_keys.sql infrastructure/db/migrations/012_create_webhook_subscriptions.sql infrastructure/db/migrations/013_create_webhook_deliveries.sql infrastructure/db/migrations/014_create_api_usage.sql
git commit -m "feat(garudaportal): add main entrypoint, Dockerfile, and database migrations"
```

---

### Task 9: Infrastructure Integration

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.env.example`
- Modify: `go.work`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Update go.work**

Add to `go.work`:

```
use (
    ...existing...
    ./services/garudaportal
)
```

- [ ] **Step 2: Update docker-compose.yml**

Add `garudaportal` service:

```yaml
  garudaportal:
    build: ./services/garudaportal
    restart: unless-stopped
    ports:
      - "4009:4009"
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

- [ ] **Step 3: Update .env.example**

Add all Phase 5 env vars:

```bash
# GarudaPortal Service
GARUDAPORTAL_PORT=4009
GARUDAPORTAL_DB_URL=postgres://garudapass:garudapass@localhost:5432/garudapass?sslmode=disable
WEBHOOK_TIMEOUT=10s
WEBHOOK_MAX_RETRIES=5
```

- [ ] **Step 4: Update CI workflow**

Add `garudaportal` to test matrix, security scan, and Docker build:

```yaml
- { dir: "services/garudaportal", name: "garudaportal" }
```

Add to security scan and Docker build matrices:

```yaml
- "services/garudaportal"
```

Add to integration test loop:

```bash
services/garudaportal
```

- [ ] **Step 5: Run all tests across all 10 services**

```bash
for svc in apps/bff services/identity services/garudainfo services/dukcapil-sim services/ahu-sim services/oss-sim services/garudacorp services/signing-sim services/garudasign services/garudaportal; do
  echo "=== $svc ==="
  cd /opt/gpass/$svc && go test ./... -count=1
done
```

Expected: All tests PASS across all 10 services.

- [ ] **Step 6: Commit**

```bash
git add go.work docker-compose.yml .env.example .github/workflows/ci.yml
git commit -m "feat: integrate Phase 5 developer portal infrastructure"
```
