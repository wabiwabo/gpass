package apikey

import (
	"time"
)

// KeyInfo represents the stored key data needed for validation.
type KeyInfo struct {
	ID          string
	AppID       string
	Status      string
	Environment string
	ExpiresAt   *time.Time
}

// AppInfo represents the app data needed for validation.
type AppInfo struct {
	ID         string
	Status     string
	Tier       string
	DailyLimit int
}

// KeyLookup looks up a key by its hash.
type KeyLookup interface {
	LookupByHash(hash string) (*KeyInfo, error)
}

// AppLookup looks up an app by its ID.
type AppLookup interface {
	LookupApp(appID string) (*AppInfo, error)
}

// UsageCounter retrieves the daily usage count for an app.
type UsageCounter interface {
	GetDailyCount(appID string) (int64, error)
}

// KeyValidationResult contains the result of an API key validation.
type KeyValidationResult struct {
	Valid       bool   `json:"valid"`
	AppID       string `json:"app_id,omitempty"`
	Environment string `json:"environment,omitempty"`
	Tier        string `json:"tier,omitempty"`
	DailyLimit  int    `json:"daily_limit,omitempty"`
	Error       string `json:"error,omitempty"`
}

// KeyValidator validates API keys.
type KeyValidator struct {
	keys  KeyLookup
	apps  AppLookup
	usage UsageCounter
}

// NewKeyValidator creates a new key validator.
func NewKeyValidator(keys KeyLookup, apps AppLookup, usage UsageCounter) *KeyValidator {
	return &KeyValidator{
		keys:  keys,
		apps:  apps,
		usage: usage,
	}
}

// Validate validates an API key and returns the result.
func (v *KeyValidator) Validate(apiKey string) (KeyValidationResult, error) {
	if apiKey == "" {
		return KeyValidationResult{Valid: false, Error: "invalid_key"}, nil
	}

	// Compute hash
	hash := HashKey(apiKey)

	// Lookup key
	keyInfo, err := v.keys.LookupByHash(hash)
	if err != nil {
		return KeyValidationResult{Valid: false, Error: "invalid_key"}, nil
	}

	// Check status
	if keyInfo.Status != "ACTIVE" {
		return KeyValidationResult{Valid: false, Error: "key_revoked"}, nil
	}

	// Check expiry
	if keyInfo.ExpiresAt != nil && keyInfo.ExpiresAt.Before(time.Now().UTC()) {
		return KeyValidationResult{Valid: false, Error: "key_expired"}, nil
	}

	// Lookup app
	appInfo, err := v.apps.LookupApp(keyInfo.AppID)
	if err != nil {
		return KeyValidationResult{Valid: false, Error: "invalid_key"}, nil
	}

	// Check app status
	if appInfo.Status != "ACTIVE" {
		return KeyValidationResult{Valid: false, Error: "app_suspended"}, nil
	}

	// Check daily usage
	dailyCount, err := v.usage.GetDailyCount(keyInfo.AppID)
	if err != nil {
		return KeyValidationResult{}, err
	}

	if dailyCount >= int64(appInfo.DailyLimit) {
		return KeyValidationResult{Valid: false, Error: "rate_limit_exceeded"}, nil
	}

	return KeyValidationResult{
		Valid:       true,
		AppID:       keyInfo.AppID,
		Environment: keyInfo.Environment,
		Tier:        appInfo.Tier,
		DailyLimit:  appInfo.DailyLimit,
	}, nil
}
