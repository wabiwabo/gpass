package apikey

import (
	"errors"
	"testing"
	"time"
)

// Mock implementations for testing

type mockKeyLookup struct {
	keys map[string]*KeyInfo
}

func (m *mockKeyLookup) LookupByHash(hash string) (*KeyInfo, error) {
	k, ok := m.keys[hash]
	if !ok {
		return nil, errors.New("not found")
	}
	return k, nil
}

type mockAppLookup struct {
	apps map[string]*AppInfo
}

func (m *mockAppLookup) LookupApp(appID string) (*AppInfo, error) {
	a, ok := m.apps[appID]
	if !ok {
		return nil, errors.New("not found")
	}
	return a, nil
}

type mockUsageCounter struct {
	counts map[string]int64
}

func (m *mockUsageCounter) GetDailyCount(appID string) (int64, error) {
	return m.counts[appID], nil
}

func setupValidator(keyHash, appID string, keyStatus string, expiresAt *time.Time, appStatus string, tier string, dailyLimit int, dailyCount int64) *KeyValidator {
	keys := &mockKeyLookup{
		keys: map[string]*KeyInfo{
			keyHash: {
				ID:          "key-1",
				AppID:       appID,
				Status:      keyStatus,
				Environment: "sandbox",
				ExpiresAt:   expiresAt,
			},
		},
	}
	apps := &mockAppLookup{
		apps: map[string]*AppInfo{
			appID: {
				ID:         appID,
				Status:     appStatus,
				Tier:       tier,
				DailyLimit: dailyLimit,
			},
		},
	}
	usage := &mockUsageCounter{
		counts: map[string]int64{
			appID: dailyCount,
		},
	}
	return NewKeyValidator(keys, apps, usage)
}

func TestValidate_ValidKey(t *testing.T) {
	apiKey := "gp_test_validkey123"
	hash := HashKey(apiKey)

	v := setupValidator(hash, "app-1", "ACTIVE", nil, "ACTIVE", "free", 100, 5)

	result, err := v.Validate(apiKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected valid=true, got error: %s", result.Error)
	}
	if result.AppID != "app-1" {
		t.Errorf("expected app-1, got %s", result.AppID)
	}
	if result.Tier != "free" {
		t.Errorf("expected free tier, got %s", result.Tier)
	}
}

func TestValidate_UnknownKey(t *testing.T) {
	v := setupValidator("nothash", "app-1", "ACTIVE", nil, "ACTIVE", "free", 100, 0)

	result, err := v.Validate("gp_test_unknownkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false for unknown key")
	}
	if result.Error != "invalid_key" {
		t.Errorf("expected invalid_key error, got %s", result.Error)
	}
}

func TestValidate_RevokedKey(t *testing.T) {
	apiKey := "gp_test_revokedkey"
	hash := HashKey(apiKey)

	v := setupValidator(hash, "app-1", "REVOKED", nil, "ACTIVE", "free", 100, 0)

	result, err := v.Validate(apiKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false for revoked key")
	}
	if result.Error != "key_revoked" {
		t.Errorf("expected key_revoked, got %s", result.Error)
	}
}

func TestValidate_ExpiredKey(t *testing.T) {
	apiKey := "gp_test_expiredkey"
	hash := HashKey(apiKey)
	past := time.Now().UTC().Add(-1 * time.Hour)

	v := setupValidator(hash, "app-1", "ACTIVE", &past, "ACTIVE", "free", 100, 0)

	result, err := v.Validate(apiKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false for expired key")
	}
	if result.Error != "key_expired" {
		t.Errorf("expected key_expired, got %s", result.Error)
	}
}

func TestValidate_SuspendedApp(t *testing.T) {
	apiKey := "gp_test_suspendedapp"
	hash := HashKey(apiKey)

	v := setupValidator(hash, "app-1", "ACTIVE", nil, "SUSPENDED", "free", 100, 0)

	result, err := v.Validate(apiKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false for suspended app")
	}
	if result.Error != "app_suspended" {
		t.Errorf("expected app_suspended, got %s", result.Error)
	}
}

func TestValidate_RateLimitExceeded(t *testing.T) {
	apiKey := "gp_test_ratelimited"
	hash := HashKey(apiKey)

	v := setupValidator(hash, "app-1", "ACTIVE", nil, "ACTIVE", "free", 100, 100)

	result, err := v.Validate(apiKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false when rate limit exceeded")
	}
	if result.Error != "rate_limit_exceeded" {
		t.Errorf("expected rate_limit_exceeded, got %s", result.Error)
	}
}

func TestValidate_EmptyKey(t *testing.T) {
	v := setupValidator("", "app-1", "ACTIVE", nil, "ACTIVE", "free", 100, 0)

	result, err := v.Validate("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Error("expected valid=false for empty key")
	}
}
