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

func TestKeyHandler_CreateKey(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewKeyHandler(appStore, keyStore)

	body := `{"name":"Production Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp keyResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.PlaintextKey == "" {
		t.Error("expected plaintext key to be returned")
	}
	if !strings.HasPrefix(resp.PlaintextKey, "gp_test_") {
		t.Errorf("expected gp_test_ prefix, got %s", resp.PlaintextKey[:16])
	}
	if resp.Name != "Production Key" {
		t.Errorf("expected Production Key, got %s", resp.Name)
	}
	if resp.Status != "ACTIVE" {
		t.Errorf("expected ACTIVE, got %s", resp.Status)
	}
}

func TestKeyHandler_CreateKey_NotOwner(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	h := NewKeyHandler(appStore, keyStore)

	body := `{"name":"Key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys", bytes.NewBufferString(body))
	req.Header.Set("X-User-ID", "user-2")
	req.SetPathValue("app_id", app.ID)
	w := httptest.NewRecorder()

	h.CreateKey(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestKeyHandler_ListKeys(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})

	keyStore.Create(&store.APIKey{AppID: app.ID, KeyHash: "h1", KeyPrefix: "gp_test_00000001", Name: "Key 1", Environment: "sandbox"})
	keyStore.Create(&store.APIKey{AppID: app.ID, KeyHash: "h2", KeyPrefix: "gp_test_00000002", Name: "Key 2", Environment: "sandbox"})

	h := NewKeyHandler(appStore, keyStore)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/portal/apps/"+app.ID+"/keys", nil)
	req.Header.Set("X-User-ID", "user-1")
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

	if len(resp.Keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(resp.Keys))
	}

	// Verify no plaintext key is returned in list
	for _, k := range resp.Keys {
		if k.PlaintextKey != "" {
			t.Error("plaintext key should not be returned in list")
		}
	}
}

func TestKeyHandler_RevokeKey(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{OwnerUserID: "user-1", Name: "My App", Environment: "sandbox", Tier: "free", DailyLimit: 100})
	key, _ := keyStore.Create(&store.APIKey{AppID: app.ID, KeyHash: "h1", KeyPrefix: "gp_test_00000001", Name: "Key 1", Environment: "sandbox"})

	h := NewKeyHandler(appStore, keyStore)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/portal/apps/"+app.ID+"/keys/"+key.ID, nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("key_id", key.ID)
	w := httptest.NewRecorder()

	h.RevokeKey(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "REVOKED" {
		t.Errorf("expected REVOKED status, got %v", resp["status"])
	}
}
