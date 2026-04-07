package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func newRotationSetup(t *testing.T) (*RotationHandler, *store.App, *store.APIKey) {
	t.Helper()
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1", Name: "App", Environment: "sandbox", Tier: "free", DailyLimit: 100,
	})
	key, _ := keyStore.Create(&store.APIKey{
		AppID: app.ID, KeyHash: "h", KeyPrefix: "gpk_sandbox_abc", Name: "k1", Environment: "sandbox",
	})
	return NewRotationHandler(appStore, keyStore), app, key
}

func reqRot(hdr map[string]string, pv map[string]string, h http.HandlerFunc) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/", bytes.NewBufferString(""))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	for k, v := range pv {
		req.SetPathValue(k, v)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func TestRotateKey_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app, key := newRotationSetup(t)
		rec := reqRot(nil, map[string]string{"app_id": app.ID, "key_id": key.ID}, h.RotateKey)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("app not found", func(t *testing.T) {
		h, _, key := newRotationSetup(t)
		rec := reqRot(map[string]string{"X-User-ID": "user-1"}, map[string]string{"app_id": "missing", "key_id": key.ID}, h.RotateKey)
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		h, app, key := newRotationSetup(t)
		rec := reqRot(map[string]string{"X-User-ID": "intruder"}, map[string]string{"app_id": app.ID, "key_id": key.ID}, h.RotateKey)
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("key not found", func(t *testing.T) {
		h, app, _ := newRotationSetup(t)
		rec := reqRot(map[string]string{"X-User-ID": "user-1"}, map[string]string{"app_id": app.ID, "key_id": "missing"}, h.RotateKey)
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("happy", func(t *testing.T) {
		h, app, key := newRotationSetup(t)
		rec := reqRot(map[string]string{"X-User-ID": "user-1"}, map[string]string{"app_id": app.ID, "key_id": key.ID}, h.RotateKey)
		if rec.Code != http.StatusCreated {
			t.Errorf("code = %d body=%s", rec.Code, rec.Body)
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"plaintext_key"`)) {
			t.Error("new key not returned")
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(`"rotation_instructions"`)) {
			t.Error("instructions missing")
		}
	})
	t.Run("already revoked", func(t *testing.T) {
		h, app, key := newRotationSetup(t)
		// Revoke the key directly in the store.
		keyStore := store.NewInMemoryKeyStore()
		appStore := store.NewInMemoryAppStore()
		app, _ = appStore.Create(&store.App{
			OwnerUserID: "user-1", Name: "App", Environment: "sandbox", Tier: "free", DailyLimit: 100,
		})
		key, _ = keyStore.Create(&store.APIKey{
			AppID: app.ID, KeyHash: "h", KeyPrefix: "gpk_sandbox_xyz", Name: "k1", Environment: "sandbox",
		})
		keyStore.Revoke(key.ID)
		h = NewRotationHandler(appStore, keyStore)
		rec := reqRot(map[string]string{"X-User-ID": "user-1"}, map[string]string{"app_id": app.ID, "key_id": key.ID}, h.RotateKey)
		if rec.Code != http.StatusConflict {
			t.Errorf("code = %d body=%s", rec.Code, rec.Body)
		}
	})
}
