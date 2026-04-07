package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

func newWebhookSetup(t *testing.T) (*WebhookHandler, *store.App) {
	t.Helper()
	appStore := store.NewInMemoryAppStore()
	whStore := store.NewInMemoryWebhookStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1", Name: "App", Environment: "sandbox", Tier: "free", DailyLimit: 100,
	})
	return NewWebhookHandler(appStore, whStore), app
}

func reqWH(method, body string, hdr map[string]string, h http.HandlerFunc, pv map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/", bytes.NewBufferString(body))
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

func TestSubscribe_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("POST", `{}`, nil, h.Subscribe, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("missing app id", func(t *testing.T) {
		h, _ := newWebhookSetup(t)
		rec := reqWH("POST", `{}`, map[string]string{"X-User-ID": "user-1"}, h.Subscribe, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("app not found", func(t *testing.T) {
		h, _ := newWebhookSetup(t)
		rec := reqWH("POST", `{}`, map[string]string{"X-User-ID": "user-1"}, h.Subscribe, map[string]string{"app_id": "missing"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("POST", `{}`, map[string]string{"X-User-ID": "intruder"}, h.Subscribe, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("bad json", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("POST", "{bad", map[string]string{"X-User-ID": "user-1"}, h.Subscribe, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("missing url", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("POST", `{"events":["e"]}`, map[string]string{"X-User-ID": "user-1"}, h.Subscribe, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("empty events", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("POST", `{"url":"https://x.test"}`, map[string]string{"X-User-ID": "user-1"}, h.Subscribe, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("bad scheme", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("POST", `{"url":"http://evil.test","events":["e"]}`, map[string]string{"X-User-ID": "user-1"}, h.Subscribe, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("happy", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("POST", `{"url":"https://x.test","events":["sign.completed"]}`, map[string]string{"X-User-ID": "user-1"}, h.Subscribe, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusCreated {
			t.Errorf("code = %d body=%s", rec.Code, rec.Body)
		}
	})
}

func TestListWebhooks_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("GET", "", nil, h.ListWebhooks, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("missing app id", func(t *testing.T) {
		h, _ := newWebhookSetup(t)
		rec := reqWH("GET", "", map[string]string{"X-User-ID": "user-1"}, h.ListWebhooks, nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("app not found", func(t *testing.T) {
		h, _ := newWebhookSetup(t)
		rec := reqWH("GET", "", map[string]string{"X-User-ID": "user-1"}, h.ListWebhooks, map[string]string{"app_id": "missing"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("GET", "", map[string]string{"X-User-ID": "intruder"}, h.ListWebhooks, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("happy", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("GET", "", map[string]string{"X-User-ID": "user-1"}, h.ListWebhooks, map[string]string{"app_id": app.ID})
		if rec.Code != 200 {
			t.Errorf("code = %d", rec.Code)
		}
	})
}

func TestDeleteWebhook_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("DELETE", "", nil, h.DeleteWebhook, map[string]string{"app_id": app.ID, "webhook_id": "w1"})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("app not found", func(t *testing.T) {
		h, _ := newWebhookSetup(t)
		rec := reqWH("DELETE", "", map[string]string{"X-User-ID": "user-1"}, h.DeleteWebhook, map[string]string{"app_id": "missing", "webhook_id": "w1"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("DELETE", "", map[string]string{"X-User-ID": "intruder"}, h.DeleteWebhook, map[string]string{"app_id": app.ID, "webhook_id": "w1"})
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("webhook not found", func(t *testing.T) {
		h, app := newWebhookSetup(t)
		rec := reqWH("DELETE", "", map[string]string{"X-User-ID": "user-1"}, h.DeleteWebhook, map[string]string{"app_id": app.ID, "webhook_id": "missing"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
}
