package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

// TestKeyHandler_CreateKeyReturnsUniqueKeys verifies that consecutive
// key creation calls produce unique plaintext keys.
func TestKeyHandler_CreateKeyReturnsUniqueKeys(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-unique",
		Name:        "Unique Keys App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	keys := make(map[string]bool)
	for i := 0; i < 10; i++ {
		body := `{"name":"Key"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys",
			bytes.NewBufferString(body))
		req.Header.Set("X-User-ID", "user-unique")
		req.SetPathValue("app_id", app.ID)
		w := httptest.NewRecorder()

		h.CreateKey(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("iteration %d: expected 201, got %d: %s", i, w.Code, w.Body.String())
		}

		var resp keyResponse
		json.NewDecoder(w.Body).Decode(&resp)

		if keys[resp.PlaintextKey] {
			t.Errorf("iteration %d: duplicate key generated", i)
		}
		keys[resp.PlaintextKey] = true
	}

	if len(keys) != 10 {
		t.Errorf("expected 10 unique keys, got %d", len(keys))
	}
}

// TestKeyHandler_KeyPrefixMatchesSandbox verifies that keys created for
// sandbox environment have the gp_test_ prefix.
func TestKeyHandler_KeyPrefixMatchesSandbox(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-prefix",
		Name:        "Sandbox App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	body := `{"name":"Sandbox Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys",
		bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-prefix")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp keyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if !strings.HasPrefix(resp.PlaintextKey, "gp_test_") {
		t.Errorf("expected gp_test_ prefix for sandbox, got prefix: %s",
			resp.PlaintextKey[:min(16, len(resp.PlaintextKey))])
	}

	// Key prefix (first 16 chars) should also start with gp_test_
	if !strings.HasPrefix(resp.KeyPrefix, "gp_test_") {
		t.Errorf("key_prefix should start with gp_test_, got %s", resp.KeyPrefix)
	}
}

// TestKeyHandler_KeyPrefixMatchesProduction verifies that keys created for
// production environment have the gp_live_ prefix.
func TestKeyHandler_KeyPrefixMatchesProduction(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-prefix",
		Name:        "Production App",
		Environment: "production",
		Tier:        "pro",
		DailyLimit:  10000,
	})

	h := NewKeyHandler(appStore, keyStore)

	body := `{"name":"Production Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys",
		bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-prefix")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp keyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if !strings.HasPrefix(resp.PlaintextKey, "gp_live_") {
		t.Errorf("expected gp_live_ prefix for production, got prefix: %s",
			resp.PlaintextKey[:min(16, len(resp.PlaintextKey))])
	}
}

// TestKeyHandler_RevokedKeyCannotBeReRevoked verifies that revoking an
// already-revoked key still succeeds (idempotent operation) but key
// status remains REVOKED.
func TestKeyHandler_RevokedKeyCannotBeReRevoked(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-revoke",
		Name:        "Revoke Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})
	key, _ := keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     "h-revoke",
		KeyPrefix:   "gp_test_revokexx",
		Name:        "To Revoke",
		Environment: "sandbox",
	})

	h := NewKeyHandler(appStore, keyStore)

	// First revocation
	req1 := httptest.NewRequest(http.MethodDelete,
		"/api/v1/portal/apps/"+app.ID+"/keys/"+key.ID, nil)
	req1.Header.Set("X-User-ID", "user-revoke")
	req1.SetPathValue("app_id", app.ID)
	req1.SetPathValue("key_id", key.ID)
	w1 := httptest.NewRecorder()

	h.RevokeKey(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("first revoke: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	// Second revocation (re-revoke)
	req2 := httptest.NewRequest(http.MethodDelete,
		"/api/v1/portal/apps/"+app.ID+"/keys/"+key.ID, nil)
	req2.Header.Set("X-User-ID", "user-revoke")
	req2.SetPathValue("app_id", app.ID)
	req2.SetPathValue("key_id", key.ID)
	w2 := httptest.NewRecorder()

	h.RevokeKey(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second revoke: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	// Verify key is still REVOKED
	retrieved, err := keyStore.GetByID(key.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if retrieved.Status != "REVOKED" {
		t.Errorf("expected REVOKED, got %s", retrieved.Status)
	}
}

// TestKeyHandler_ListKeysEmptyArray verifies that listing keys for an app
// with no keys returns an empty array (not null).
func TestKeyHandler_ListKeysEmptyArray(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-empty",
		Name:        "Empty App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID+"/keys", nil)
	req.Header.Set("X-User-ID", "user-empty")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.ListKeys(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp struct {
		Keys []keyResponse `json:"keys"`
	}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Keys == nil {
		t.Error("expected empty array, got nil")
	}
	if len(resp.Keys) != 0 {
		t.Errorf("expected 0 keys, got %d", len(resp.Keys))
	}
}

// TestKeyHandler_CreateKeyWithExpiry verifies that a key created with
// an expires_in_days parameter has the correct expiration set.
func TestKeyHandler_CreateKeyWithExpiry(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-expiry",
		Name:        "Expiry App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	body := `{"name":"Expiring Key","expires_in_days":30}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys",
		bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-expiry")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp keyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.ExpiresAt == nil {
		t.Fatal("expected expires_at to be set")
	}

	// The key should have been stored with expiry
	keys, _ := keyStore.ListByApp(app.ID)
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].ExpiresAt == nil {
		t.Fatal("stored key should have ExpiresAt set")
	}
}

// TestKeyHandler_CreateKeyWithoutExpiry verifies that a key created
// without expires_in_days has no expiration.
func TestKeyHandler_CreateKeyWithoutExpiry(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-noexp",
		Name:        "No Expiry App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	body := `{"name":"Permanent Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys",
		bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-noexp")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp keyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.ExpiresAt != nil {
		t.Errorf("expected nil expires_at, got %v", *resp.ExpiresAt)
	}
}

// TestKeyHandler_CreateKeyMissingUserID verifies that creating a key
// without the X-User-ID header returns 401.
func TestKeyHandler_CreateKeyMissingUserID(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	body := `{"name":"Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys",
		bytes.NewBufferString(body))
	// No X-User-ID header
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestKeyHandler_CreateKeyAppNotFound verifies that creating a key for
// a non-existent app returns 404.
func TestKeyHandler_CreateKeyAppNotFound(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()

	h := NewKeyHandler(appStore, keyStore)

	body := `{"name":"Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/nonexistent/keys",
		bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", "nonexistent")
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestKeyHandler_RevokeKeyNotFound verifies that revoking a non-existent
// key returns 404.
func TestKeyHandler_RevokeKeyNotFound(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	req := httptest.NewRequest(http.MethodDelete,
		"/api/v1/portal/apps/"+app.ID+"/keys/nonexistent-key", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("key_id", "nonexistent-key")
	w := httptest.NewRecorder()

	h.RevokeKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestKeyHandler_ListKeysMissingUserID verifies that listing keys
// without X-User-ID returns 401.
func TestKeyHandler_ListKeysMissingUserID(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "Test App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID+"/keys", nil)
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.ListKeys(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestKeyHandler_CreateKeyDefaultName verifies that when no name is
// provided, the key defaults to "Default".
func TestKeyHandler_CreateKeyDefaultName(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-default",
		Name:        "Default Name App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})

	h := NewKeyHandler(appStore, keyStore)

	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys",
		bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-default")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp keyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Name != "Default" {
		t.Errorf("expected name 'Default', got %q", resp.Name)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
