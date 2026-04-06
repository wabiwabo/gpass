package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/garudapass/gpass/services/garudaportal/apikey"
	"github.com/garudapass/gpass/services/garudaportal/store"
)

// keyStoreLookup adapts store.KeyStore to apikey.KeyLookup.
type keyStoreLookup struct {
	store store.KeyStore
}

func (k *keyStoreLookup) LookupByHash(hash string) (*apikey.KeyInfo, error) {
	key, err := k.store.GetByHash(hash)
	if err != nil {
		return nil, err
	}
	return &apikey.KeyInfo{
		ID:          key.ID,
		AppID:       key.AppID,
		Status:      key.Status,
		Environment: key.Environment,
		ExpiresAt:   key.ExpiresAt,
	}, nil
}

// appStoreLookup adapts store.AppStore to apikey.AppLookup.
type appStoreLookup struct {
	store store.AppStore
}

func (a *appStoreLookup) LookupApp(appID string) (*apikey.AppInfo, error) {
	app, err := a.store.GetByID(appID)
	if err != nil {
		return nil, err
	}
	return &apikey.AppInfo{
		ID:         app.ID,
		Status:     app.Status,
		Tier:       app.Tier,
		DailyLimit: app.DailyLimit,
	}, nil
}

// usageCounterAdapter adapts store.UsageStore to apikey.UsageCounter.
type usageCounterAdapter struct {
	store store.UsageStore
}

func (u *usageCounterAdapter) GetDailyCount(appID string) (int64, error) {
	return u.store.GetDailyUsage(appID, time.Now().UTC())
}

// ValidateHandler handles internal key validation endpoint.
type ValidateHandler struct {
	validator *apikey.KeyValidator
	keyStore  store.KeyStore
}

// NewValidateHandler creates a new validate handler.
func NewValidateHandler(appStore store.AppStore, keyStore store.KeyStore, usageStore store.UsageStore) *ValidateHandler {
	validator := apikey.NewKeyValidator(
		&keyStoreLookup{store: keyStore},
		&appStoreLookup{store: appStore},
		&usageCounterAdapter{store: usageStore},
	)
	return &ValidateHandler{
		validator: validator,
		keyStore:  keyStore,
	}
}

type validateRequest struct {
	APIKey string `json:"api_key"`
}

// ValidateKey handles POST /internal/keys/validate.
func (h *ValidateHandler) ValidateKey(w http.ResponseWriter, r *http.Request) {
	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	if req.APIKey == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(apikey.KeyValidationResult{
			Valid: false,
			Error: "invalid_key",
		})
		return
	}

	result, err := h.validator.Validate(req.APIKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Validation failed")
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if !result.Valid {
		status := http.StatusUnauthorized
		if result.Error == "rate_limit_exceeded" {
			status = http.StatusTooManyRequests
		}
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(result)
		return
	}

	// Update last used (fire and forget)
	hash := apikey.HashKey(req.APIKey)
	key, err := h.keyStore.GetByHash(hash)
	if err == nil {
		h.keyStore.UpdateLastUsed(key.ID)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}
