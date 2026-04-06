package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	apk "github.com/garudapass/gpass/services/garudaportal/apikey"
	"github.com/garudapass/gpass/services/garudaportal/store"
)

// TestEdge_CreateKey_EmptyAppName verifies that creating a key with an
// empty name in the request body defaults to "Default".
func TestEdge_CreateKey_EmptyAppName(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-1",
		Name:        "Edge App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys",
		bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-edge-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp keyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "Default" {
		t.Errorf("expected name 'Default' for empty name, got %q", resp.Name)
	}
}

// TestEdge_CreateKey_VeryLongAppName verifies that creating a key with
// a very long name (1000 chars) succeeds without truncation or error.
func TestEdge_CreateKey_VeryLongAppName(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-2",
		Name:        "Long Name App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	longName := strings.Repeat("A", 1000)
	bodyMap := map[string]string{"name": longName}
	bodyBytes, _ := json.Marshal(bodyMap)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys",
		bytes.NewBuffer(bodyBytes))
	req.Header.Set("X-User-ID", "user-edge-2")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp keyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != longName {
		t.Errorf("expected name length %d, got %d", len(longName), len(resp.Name))
	}
}

// TestEdge_RotateKey_NonExistent verifies that rotating a key that does
// not exist returns 404.
func TestEdge_RotateKey_NonExistent(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-3",
		Name:        "Rotate App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewRotationHandler(appStore, keyStore)

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/portal/apps/"+app.ID+"/keys/does-not-exist/rotate", nil)
	req.Header.Set("X-User-ID", "user-edge-3")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("key_id", "does-not-exist")
	w := httptest.NewRecorder()

	h.RotateKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEdge_RotateKey_AlreadyRotatedGracePeriod verifies that rotating
// a key that was previously rotated (old key still active during grace
// period) succeeds for the old key since it is still ACTIVE.
func TestEdge_RotateKey_AlreadyRotatedGracePeriod(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-4",
		Name:        "GracePeriod App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	originalKey, _ := keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     "hash-original",
		KeyPrefix:   "gp_test_orig0001",
		Name:        "Original Key",
		Environment: "sandbox",
	})

	h := NewRotationHandler(appStore, keyStore)

	// First rotation
	req1 := httptest.NewRequest(http.MethodPost,
		"/api/v1/portal/apps/"+app.ID+"/keys/"+originalKey.ID+"/rotate", nil)
	req1.Header.Set("X-User-ID", "user-edge-4")
	req1.SetPathValue("app_id", app.ID)
	req1.SetPathValue("key_id", originalKey.ID)
	w1 := httptest.NewRecorder()

	h.RotateKey(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("first rotation: expected 201, got %d: %s", w1.Code, w1.Body.String())
	}

	// Original key is still ACTIVE (grace period), rotate it again
	req2 := httptest.NewRequest(http.MethodPost,
		"/api/v1/portal/apps/"+app.ID+"/keys/"+originalKey.ID+"/rotate", nil)
	req2.Header.Set("X-User-ID", "user-edge-4")
	req2.SetPathValue("app_id", app.ID)
	req2.SetPathValue("key_id", originalKey.ID)
	w2 := httptest.NewRecorder()

	h.RotateKey(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("second rotation of same key (grace period): expected 201, got %d: %s",
			w2.Code, w2.Body.String())
	}

	// Verify we now have 3 keys total (original + 2 rotated)
	keys, _ := keyStore.ListByApp(app.ID)
	if len(keys) != 3 {
		t.Errorf("expected 3 keys after double rotation, got %d", len(keys))
	}
}

// TestEdge_DeleteKey_NonExistent verifies that deleting (revoking) a
// key that does not exist returns 404.
func TestEdge_DeleteKey_NonExistent(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-5",
		Name:        "Delete App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/portal/apps/"+app.ID+"/keys/nonexistent-key-id", nil)
	req.Header.Set("X-User-ID", "user-edge-5")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("key_id", "nonexistent-key-id")
	w := httptest.NewRecorder()

	h.RevokeKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "not_found" {
		t.Errorf("expected error code 'not_found', got %q", errResp["error"])
	}
}

// TestEdge_Webhook_InvalidURL verifies that subscribing to a webhook
// with a non-HTTPS URL (and not localhost) returns 400.
func TestEdge_Webhook_InvalidURL(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-6",
		Name:        "Webhook App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewWebhookHandler(appStore, webhookStore)

	tests := []struct {
		name string
		url  string
	}{
		{"plain HTTP", "http://example.com/hook"},
		{"FTP scheme", "ftp://example.com/hook"},
		{"no scheme", "example.com/hook"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"url":"` + tc.url + `","events":["identity.verified"]}`
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/portal/apps/"+app.ID+"/webhooks",
				bytes.NewBufferString(body))
			req.Header.Set("X-User-ID", "user-edge-6")
			req.SetPathValue("app_id", app.ID)
			w := httptest.NewRecorder()

			h.Subscribe(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for URL %q, got %d: %s",
					tc.url, w.Code, w.Body.String())
			}
		})
	}
}

// TestEdge_Webhook_EmptyEventsList verifies that subscribing with an
// empty events array returns 400.
func TestEdge_Webhook_EmptyEventsList(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-7",
		Name:        "Events App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewWebhookHandler(appStore, webhookStore)

	body := `{"url":"https://example.com/hook","events":[]}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/portal/apps/"+app.ID+"/webhooks",
		bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-edge-7")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.Subscribe(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "invalid_request" {
		t.Errorf("expected error 'invalid_request', got %q", errResp["error"])
	}
}

// TestEdge_Usage_NonExistentApp verifies that requesting usage stats
// for a non-existent app returns 404.
func TestEdge_Usage_NonExistentApp(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	usageStore := store.NewInMemoryUsageStore()

	h := NewUsageHandler(appStore, usageStore)

	today := time.Now().UTC().Format("2006-01-02")
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/portal/apps/nonexistent-app/usage?from="+today+"&to="+today, nil)
	req.Header.Set("X-User-ID", "user-edge-8")
	req.SetPathValue("app_id", "nonexistent-app")
	w := httptest.NewRecorder()

	h.GetUsage(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEdge_Validate_UnknownTier verifies that validating a key where
// the app has an uncommon tier value still works correctly (tier is
// just a label, not validated against a fixed list).
func TestEdge_Validate_UnknownTier(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	usageStore := store.NewInMemoryUsageStore()

	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-9",
		Name:        "Unknown Tier App",
		Environment: "sandbox",
		Tier:        "ultra_premium_999",
		DailyLimit:  50000,
	})

	plaintextKey := "gp_test_unknowntierkey12345"
	hash := apk.HashKey(plaintextKey)
	keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     hash,
		KeyPrefix:   plaintextKey[:16],
		Name:        "Tier Test Key",
		Environment: "sandbox",
	})

	h := NewValidateHandler(appStore, keyStore, usageStore)

	body := `{"api_key":"` + plaintextKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate",
		bytes.NewBufferString(body))
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
	if result.Tier != "ultra_premium_999" {
		t.Errorf("expected tier 'ultra_premium_999', got %q", result.Tier)
	}
	if result.DailyLimit != 50000 {
		t.Errorf("expected daily_limit 50000, got %d", result.DailyLimit)
	}
}

// TestEdge_Validate_ExpiredGracePeriodKey verifies that a key whose
// expiration has passed is rejected during validation.
func TestEdge_Validate_ExpiredGracePeriodKey(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	usageStore := store.NewInMemoryUsageStore()

	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-10",
		Name:        "Expired Grace App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	plaintextKey := "gp_test_expiredgrace12345"
	hash := apk.HashKey(plaintextKey)
	expired := time.Now().UTC().Add(-48 * time.Hour) // expired 2 days ago
	keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     hash,
		KeyPrefix:   plaintextKey[:16],
		Name:        "Expired Grace Key",
		Environment: "sandbox",
		ExpiresAt:   &expired,
	})

	h := NewValidateHandler(appStore, keyStore, usageStore)

	body := `{"api_key":"` + plaintextKey + `"}`
	req := httptest.NewRequest(http.MethodPost, "/internal/keys/validate",
		bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.ValidateKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var result apk.KeyValidationResult
	json.NewDecoder(w.Body).Decode(&result)

	if result.Valid {
		t.Error("expected valid=false for expired key")
	}
	if result.Error != "key_expired" {
		t.Errorf("expected error 'key_expired', got %q", result.Error)
	}
}

// TestEdge_ListKeys_NoResults verifies that listing keys for an app
// with zero keys returns an empty JSON array, not null.
func TestEdge_ListKeys_NoResults(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-11",
		Name:        "No Keys App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/portal/apps/"+app.ID+"/keys", nil)
	req.Header.Set("X-User-ID", "user-edge-11")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.ListKeys(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify JSON contains empty array, not null
	var raw map[string]json.RawMessage
	json.NewDecoder(w.Body).Decode(&raw)

	keysJSON := strings.TrimSpace(string(raw["keys"]))
	if keysJSON != "[]" {
		t.Errorf("expected empty array '[]', got %s", keysJSON)
	}
}

// TestEdge_Validate_KeyPrefixFormat verifies that sandbox keys have
// gp_test_ prefix and production keys have gp_live_ prefix via the
// key validation flow.
func TestEdge_Validate_KeyPrefixFormat(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		wantPrefix  string
	}{
		{"sandbox key prefix", "sandbox", "gp_test_"},
		{"production key prefix", "production", "gp_live_"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			appStore := store.NewInMemoryAppStore()
			keyStore := store.NewInMemoryKeyStore()
			app, _ := appStore.Create(&store.App{
				OwnerUserID: "user-prefix-check",
				Name:        "Prefix App",
				Environment: tc.environment,
				Tier:        "free",
				DailyLimit:  100,
			})

			h := NewKeyHandler(appStore, keyStore)

			body := `{"name":"Prefix Check Key"}`
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/portal/apps/"+app.ID+"/keys",
				bytes.NewBufferString(body))
			req.Header.Set("X-User-ID", "user-prefix-check")
			req.SetPathValue("app_id", app.ID)
			w := httptest.NewRecorder()

			h.CreateKey(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
			}

			var resp keyResponse
			json.NewDecoder(w.Body).Decode(&resp)

			if !strings.HasPrefix(resp.PlaintextKey, tc.wantPrefix) {
				t.Errorf("expected prefix %q, got key starting with %q",
					tc.wantPrefix, resp.PlaintextKey[:len(tc.wantPrefix)])
			}
			if !strings.HasPrefix(resp.KeyPrefix, tc.wantPrefix) {
				t.Errorf("expected key_prefix to start with %q, got %q",
					tc.wantPrefix, resp.KeyPrefix)
			}
		})
	}
}

// TestEdge_ConcurrentKeyCreation verifies that creating many keys
// concurrently is race-free and each key has a unique ID and plaintext.
func TestEdge_ConcurrentKeyCreation(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-concurrent",
		Name:        "Concurrent App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	const goroutines = 20

	var mu sync.Mutex
	plaintextKeys := make(map[string]bool)
	keyIDs := make(map[string]bool)
	errors := make([]string, 0)

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()

			body := `{"name":"Concurrent Key"}`
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/portal/apps/"+app.ID+"/keys",
				bytes.NewBufferString(body))
			req.Header.Set("X-User-ID", "user-concurrent")
			req.SetPathValue("app_id", app.ID)
			w := httptest.NewRecorder()

			h.CreateKey(w, req)

			mu.Lock()
			defer mu.Unlock()

			if w.Code != http.StatusCreated {
				errors = append(errors, w.Body.String())
				return
			}

			var resp keyResponse
			json.NewDecoder(w.Body).Decode(&resp)

			if plaintextKeys[resp.PlaintextKey] {
				errors = append(errors, "duplicate plaintext key: "+resp.PlaintextKey)
			}
			plaintextKeys[resp.PlaintextKey] = true

			if keyIDs[resp.ID] {
				errors = append(errors, "duplicate key ID: "+resp.ID)
			}
			keyIDs[resp.ID] = true
		}()
	}

	wg.Wait()

	if len(errors) > 0 {
		for _, e := range errors {
			t.Error(e)
		}
	}

	if len(plaintextKeys) != goroutines {
		t.Errorf("expected %d unique plaintext keys, got %d", goroutines, len(plaintextKeys))
	}
	if len(keyIDs) != goroutines {
		t.Errorf("expected %d unique key IDs, got %d", goroutines, len(keyIDs))
	}
}

// TestEdge_DeleteWebhook_NonExistent verifies that deleting a webhook
// subscription that does not exist returns 404.
func TestEdge_DeleteWebhook_NonExistent(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	webhookStore := store.NewInMemoryWebhookStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-wh",
		Name:        "Webhook Delete App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewWebhookHandler(appStore, webhookStore)

	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/portal/apps/"+app.ID+"/webhooks/nonexistent-wh", nil)
	req.Header.Set("X-User-ID", "user-edge-wh")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("webhook_id", "nonexistent-wh")
	w := httptest.NewRecorder()

	h.DeleteWebhook(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestEdge_Usage_InvalidDateFormat verifies that requesting usage with
// an invalid date format returns 400.
func TestEdge_Usage_InvalidDateFormat(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	usageStore := store.NewInMemoryUsageStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-edge-date",
		Name:        "Date Format App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewUsageHandler(appStore, usageStore)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/portal/apps/"+app.ID+"/usage?from=01-01-2024&to=01-31-2024", nil)
	req.Header.Set("X-User-ID", "user-edge-date")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.GetUsage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
