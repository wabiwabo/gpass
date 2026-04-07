package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/garudapass/gpass/services/garudaportal/store"
)

type noopDispatcher struct{ err error }

func (n *noopDispatcher) Deliver(_ *store.WebhookSubscription, _ string, _ []byte) error {
	return n.err
}

func newReplaySetup(t *testing.T) (*ReplayHandler, *store.App, *store.WebhookSubscription, *store.WebhookDelivery) {
	t.Helper()
	appStore := store.NewInMemoryAppStore()
	whStore := store.NewInMemoryWebhookStore()
	delStore := store.NewInMemoryDeliveryStore()
	app, _ := appStore.Create(&store.App{
		OwnerUserID: "user-1", Name: "App", Environment: "sandbox", Tier: "free", DailyLimit: 100,
	})
	sub, _ := whStore.Create(&store.WebhookSubscription{
		AppID: app.ID, URL: "https://x.test", Events: []string{"e"}, Secret: "whsec_abc",
	})
	del, _ := delStore.Create(&store.WebhookDelivery{
		SubscriptionID: sub.ID, EventType: "e", Payload: `{"x":1}`, Status: "FAILED",
	})
	return NewReplayHandler(delStore, whStore, appStore, &noopDispatcher{}), app, sub, del
}

func reqReplay(method, path string, hdr map[string]string, h http.HandlerFunc, pv map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(""))
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

func TestReplayDelivery_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app, _, del := newReplaySetup(t)
		rec := reqReplay("POST", "/", nil, h.ReplayDelivery, map[string]string{"app_id": app.ID, "delivery_id": del.ID})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("app not found", func(t *testing.T) {
		h, _, _, del := newReplaySetup(t)
		rec := reqReplay("POST", "/", map[string]string{"X-User-ID": "user-1"}, h.ReplayDelivery, map[string]string{"app_id": "missing", "delivery_id": del.ID})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		h, app, _, del := newReplaySetup(t)
		rec := reqReplay("POST", "/", map[string]string{"X-User-ID": "intruder"}, h.ReplayDelivery, map[string]string{"app_id": app.ID, "delivery_id": del.ID})
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("delivery not found", func(t *testing.T) {
		h, app, _, _ := newReplaySetup(t)
		rec := reqReplay("POST", "/", map[string]string{"X-User-ID": "user-1"}, h.ReplayDelivery, map[string]string{"app_id": app.ID, "delivery_id": "missing"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("happy", func(t *testing.T) {
		h, app, _, del := newReplaySetup(t)
		rec := reqReplay("POST", "/", map[string]string{"X-User-ID": "user-1"}, h.ReplayDelivery, map[string]string{"app_id": app.ID, "delivery_id": del.ID})
		if rec.Code != 200 {
			t.Errorf("code = %d body=%s", rec.Code, rec.Body)
		}
	})
}

func TestListDeliveries_FullMatrix(t *testing.T) {
	t.Run("missing user id", func(t *testing.T) {
		h, app, _, _ := newReplaySetup(t)
		rec := reqReplay("GET", "/", nil, h.ListDeliveries, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("app not found", func(t *testing.T) {
		h, _, _, _ := newReplaySetup(t)
		rec := reqReplay("GET", "/", map[string]string{"X-User-ID": "user-1"}, h.ListDeliveries, map[string]string{"app_id": "missing"})
		if rec.Code != http.StatusNotFound {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("not owner", func(t *testing.T) {
		h, app, _, _ := newReplaySetup(t)
		rec := reqReplay("GET", "/", map[string]string{"X-User-ID": "intruder"}, h.ListDeliveries, map[string]string{"app_id": app.ID})
		if rec.Code != http.StatusForbidden {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("happy with limit", func(t *testing.T) {
		h, app, _, _ := newReplaySetup(t)
		rec := reqReplay("GET", "/?status=FAILED&limit=5", map[string]string{"X-User-ID": "user-1"}, h.ListDeliveries, map[string]string{"app_id": app.ID})
		if rec.Code != 200 {
			t.Errorf("code = %d", rec.Code)
		}
	})
	t.Run("bad limit ignored", func(t *testing.T) {
		h, app, _, _ := newReplaySetup(t)
		rec := reqReplay("GET", "/?limit=notanum", map[string]string{"X-User-ID": "user-1"}, h.ListDeliveries, map[string]string{"app_id": app.ID})
		if rec.Code != 200 {
			t.Errorf("code = %d", rec.Code)
		}
	})
}
