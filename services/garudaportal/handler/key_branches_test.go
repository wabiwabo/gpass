package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func newKeySetup(t *testing.T) (*KeyHandler, *store.App) {
	t.Helper()
	appStore := store.NewInMemoryAppStore()
	keyStore := store.NewInMemoryKeyStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1", Name: "App", Environment: "sandbox", Tier: "free", DailyLimit: 100,
	})
	return NewKeyHandler(appStore, keyStore), app
}

// reqWithKey runs a request through the handler with PathValues populated.
func reqWithKey(method, path string, hdr map[string]string, body string, h http.HandlerFunc, pv map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
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

// TestCreateKey_MissingUserID pins the unauthorized branch.
func TestCreateKey_MissingUserID(t *testing.T) {
	h, app := newKeySetup(t)
	rec := reqWithKey("POST", "/", nil, `{"name":"k"}`, h.CreateKey, map[string]string{"app_id": app.ID})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestCreateKey_BadJSON pins the JSON-decode error branch.
func TestCreateKey_BadJSON(t *testing.T) {
	h, app := newKeySetup(t)
	rec := reqWithKey("POST", "/", map[string]string{"X-User-ID": "user-1"}, "{not json", h.CreateKey, map[string]string{"app_id": app.ID})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestCreateKey_AppNotFound pins the GetByID → ErrAppNotFound branch.
func TestCreateKey_AppNotFound(t *testing.T) {
	h, _ := newKeySetup(t)
	rec := reqWithKey("POST", "/", map[string]string{"X-User-ID": "user-1"}, `{"name":"k"}`, h.CreateKey, map[string]string{"app_id": "missing"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("code = %d", rec.Code)
	}
}

// TestListKeys_FullMatrix pins all branches in ListKeys.
func TestListKeys_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app := newKeySetup(t)
		rec := reqWithKey("GET", "/", nil, "", h.ListKeys, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})

	t.Run("missing app id", func(t *testing.T) {
		h, _ := newKeySetup(t)
		rec := reqWithKey("GET", "/", map[string]string{"X-User-ID": "user-1"}, "", h.ListKeys, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})

	t.Run("app not found", func(t *testing.T) {
		h, _ := newKeySetup(t)
		rec := reqWithKey("GET", "/", map[string]string{"X-User-ID": "user-1"}, "", h.ListKeys, map[string]string{"app_id": "missing"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})

	t.Run("not owner", func(t *testing.T) {
		h, app := newKeySetup(t)
		rec := reqWithKey("GET", "/", map[string]string{"X-User-ID": "intruder"}, "", h.ListKeys, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})

	t.Run("happy path empty", func(t *testing.T) {
		h, app := newKeySetup(t)
		rec := reqWithKey("GET", "/", map[string]string{"X-User-ID": "user-1"}, "", h.ListKeys, map[string]string{"app_id": app.ID})
		if rec.Code != 200 {
			t.Errorf("code = %d", rec.Code)
		}
	})
}

// TestRevokeKey_FullMatrix pins all branches in RevokeKey.
func TestRevokeKey_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app := newKeySetup(t)
		rec := reqWithKey("DELETE", "/", nil, "", h.RevokeKey, map[string]string{"app_id": app.ID, "key_id": "k1"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})

	t.Run("app not found", func(t *testing.T) {
		h, _ := newKeySetup(t)
		rec := reqWithKey("DELETE", "/", map[string]string{"X-User-ID": "user-1"}, "", h.RevokeKey, map[string]string{"app_id": "missing", "key_id": "k1"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})

	t.Run("not owner", func(t *testing.T) {
		h, app := newKeySetup(t)
		rec := reqWithKey("DELETE", "/", map[string]string{"X-User-ID": "intruder"}, "", h.RevokeKey, map[string]string{"app_id": app.ID, "key_id": "k1"})
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})

	t.Run("key not found", func(t *testing.T) {
		h, app := newKeySetup(t)
		rec := reqWithKey("DELETE", "/", map[string]string{"X-User-ID": "user-1"}, "", h.RevokeKey, map[string]string{"app_id": app.ID, "key_id": "missing-key"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
}
