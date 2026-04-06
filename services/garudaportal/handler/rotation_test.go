package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func setupRotationTest(t *testing.T) (*RotationHandler, *store.App, *store.APIKey) {
	t.Helper()
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()

	app, err := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "My App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	key, err := keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     "oldhash",
		KeyPrefix:   "gp_test_00000001",
		Name:        "Original Key",
		Environment: "sandbox",
	})
	if err != nil {
		t.Fatalf("failed to create key: %v", err)
	}

	h := NewRotationHandler(appStore, keyStore)
	return h, app, key
}

func TestRotateKey_Success(t *testing.T) {
	h, app, key := setupRotationTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys/"+key.ID+"/rotate", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("key_id", key.ID)
	w := httptest.NewRecorder()

	h.RotateKey(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp rotationResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.NewKey.PlaintextKey == "" {
		t.Error("expected new plaintext key")
	}
	if !strings.HasPrefix(resp.NewKey.PlaintextKey, "gp_test_") {
		t.Errorf("expected gp_test_ prefix, got %s", resp.NewKey.PlaintextKey)
	}
	if resp.NewKey.Status != "ACTIVE" {
		t.Errorf("expected new key ACTIVE, got %s", resp.NewKey.Status)
	}
	if resp.NewKey.Environment != "sandbox" {
		t.Errorf("expected sandbox environment, got %s", resp.NewKey.Environment)
	}
	if resp.OldKey.ID != key.ID {
		t.Errorf("expected old key ID %s, got %s", key.ID, resp.OldKey.ID)
	}
	if resp.OldKey.Status != "ACTIVE" {
		t.Errorf("expected old key still ACTIVE, got %s", resp.OldKey.Status)
	}
	if len(resp.RotationInstructions) != 3 {
		t.Errorf("expected 3 rotation instructions, got %d", len(resp.RotationInstructions))
	}
}

func TestRotateKey_AppNotFound(t *testing.T) {
	h, _, _ := setupRotationTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/nonexistent/keys/k1/rotate", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", "nonexistent")
	req.SetPathValue("key_id", "k1")
	w := httptest.NewRecorder()

	h.RotateKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRotateKey_NotOwner(t *testing.T) {
	h, app, key := setupRotationTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys/"+key.ID+"/rotate", nil)
	req.Header.Set("X-User-ID", "user-2")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("key_id", key.ID)
	w := httptest.NewRecorder()

	h.RotateKey(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestRotateKey_KeyNotFound(t *testing.T) {
	h, app, _ := setupRotationTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys/nonexistent/rotate", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("key_id", "nonexistent")
	w := httptest.NewRecorder()

	h.RotateKey(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRotateKey_AlreadyRevoked(t *testing.T) {
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1",
		Name:        "My App",
		Environment: "sandbox",
		Tier:        "free",
		DailyLimit:  100,
	})
	key, _ := keyStore.Create(&store.APIKey{
		AppID:       app.ID,
		KeyHash:     "h1",
		KeyPrefix:   "gp_test_00000001",
		Name:        "Key 1",
		Environment: "sandbox",
	})
	keyStore.Revoke(key.ID)

	h := NewRotationHandler(appStore, keyStore)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/portal/apps/"+app.ID+"/keys/"+key.ID+"/rotate", nil)
	req.Header.Set("X-User-ID", "user-1")
	req.SetPathValue("app_id", app.ID)
	req.SetPathValue("key_id", key.ID)
	w := httptest.NewRecorder()

	h.RotateKey(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}
