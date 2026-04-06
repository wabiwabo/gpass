package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apk "github.com/garudapass/gpass/services/garudaportal/apikey"
	"github.com/garudapass/gpass/services/garudaportal/store"
)

func setupValidateHandler() (*ValidateHandler, *store.InMemoryAppStore, *store.InMemoryKeyStore, *store.InMemoryUsageStore) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	usageStore := store.NewInMemoryUsageStore()

	h := NewValidateHandler(appStore, keyStore, usageStore)
	return h, appStore, keyStore, usageStore
}

func TestValidateHandler_ValidKey(t *testing.T) {
	h, appStore, keyStore, _ := setupValidateHandler()

	// Create app and key
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	plaintextKey := "gp_test_validtestkey123456"
	hash := apk.HashKey(plaintextKey)
	keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     hash,
		KeyPrefix:   plaintextKey[:16],
		Name:        "Test Key",
		Environment: "sandbox",
	})

	body := `{"api_key":"` + plaintextKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ValidateKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result apk.KeyValidationResult
	json.NewDecoder(w.Body).Decode(&result)

	if !result.Valid {
		t.Errorf("expected valid=true, got error: %s", result.Error)
	}
	if result.AppID != app.ID {
		t.Errorf("expected app_id %s, got %s", app.ID, result.AppID)
	}
}

func TestValidateHandler_InvalidKey(t *testing.T) {
	h, _, _, _ := setupValidateHandler()

	body := `{"api_key":"gp_test_nonexistent"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ValidateKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidateHandler_RateLimited(t *testing.T) {
	h, appStore, keyStore, usageStore := setupValidateHandler()

	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  5, // Low limit
	})

	plaintextKey := "gp_test_ratelimitedkey123"
	hash := apk.HashKey(plaintextKey)
	keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     hash,
		KeyPrefix:   plaintextKey[:16],
		Name:        "Test Key",
		Environment: "sandbox",
	})

	// Exhaust daily limit
	for i := 0; i < 5; i++ {
		usageStore.Increment(app.ID, "/test", false)
	}

	body := `{"api_key":"` + plaintextKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ValidateKey(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d: %s", w.Code, w.Body.String())
	}
}

func TestValidateHandler_EmptyKey(t *testing.T) {
	h, _, _, _ := setupValidateHandler()

	body := `{"api_key":""}`
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ValidateKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestValidateHandler_ExpiredKey(t *testing.T) {
	h, appStore, keyStore, _ := setupValidateHandler()

	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	plaintextKey := "gp_test_expiredtestkey123"
	hash := apk.HashKey(plaintextKey)
	past := time.Now().UTC().Add(-1 * time.Hour)
	keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     hash,
		KeyPrefix:   plaintextKey[:16],
		Name:        "Expired Key",
		Environment: "sandbox",
		ExpiresAt:   &past,
	})

	body := `{"api_key":"` + plaintextKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ValidateKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}
}
