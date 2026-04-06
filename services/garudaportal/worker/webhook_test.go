package worker

import (
	"errors"
	"testing"
	"time"
)

// --- Mock implementations ---

type mockDeliveryStore struct {
	pending    []*Delivery
	listErr    error
	updates    []statusUpdate
	updateErr  error
}

type statusUpdate struct {
	ID        string
	Status    string
	Code      int
	Body      string
	NextRetry *time.Time
}

func (m *mockDeliveryStore) ListPending() ([]*Delivery, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.pending, nil
}

func (m *mockDeliveryStore) UpdateStatus(id, status string, code int, body string, nextRetry *time.Time) error {
	m.updates = append(m.updates, statusUpdate{
		ID:        id,
		Status:    status,
		Code:      code,
		Body:      body,
		NextRetry: nextRetry,
	})
	return m.updateErr
}

type mockWebhookStore struct {
	subs   map[string]*Subscription
	getErr error
}

func (m *mockWebhookStore) GetByID(id string) (*Subscription, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	sub, ok := m.subs[id]
	if !ok {
		return nil, errors.New("subscription not found")
	}
	return sub, nil
}

type mockDispatcher struct {
	statusCode int
	respBody   string
	err        error
	calls      []dispatchCall
}

type dispatchCall struct {
	URL       string
	Payload   []byte
	Secret    string
	EventType string
}

func (m *mockDispatcher) Deliver(url string, payload []byte, secret string, eventType string) (int, string, error) {
	m.calls = append(m.calls, dispatchCall{
		URL:       url,
		Payload:   payload,
		Secret:    secret,
		EventType: eventType,
	})
	return m.statusCode, m.respBody, m.err
}

// --- Tests ---

func TestProcessPending_DeliversPendingItems(t *testing.T) {
	ds := &mockDeliveryStore{
		pending: []*Delivery{
			{ID: "d1", SubscriptionID: "s1", EventType: "user.created", Payload: []byte(`{"id":"1"}`)},
			{ID: "d2", SubscriptionID: "s1", EventType: "user.updated", Payload: []byte(`{"id":"2"}`)},
		},
	}
	ws := &mockWebhookStore{
		subs: map[string]*Subscription{
			"s1": {ID: "s1", URL: "https://example.com/hook", Secret: "secret", Status: "ACTIVE"},
		},
	}
	disp := &mockDispatcher{statusCode: 200, respBody: "OK"}

	w := New(ds, ws, disp, time.Second)
	processed, succeeded, failed := w.ProcessPending()

	if processed != 2 {
		t.Errorf("expected 2 processed, got %d", processed)
	}
	if succeeded != 2 {
		t.Errorf("expected 2 succeeded, got %d", succeeded)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}
	if len(disp.calls) != 2 {
		t.Errorf("expected 2 dispatch calls, got %d", len(disp.calls))
	}
}

func TestProcessPending_UpdatesStatusOnSuccess(t *testing.T) {
	ds := &mockDeliveryStore{
		pending: []*Delivery{
			{ID: "d1", SubscriptionID: "s1", EventType: "user.created", Payload: []byte(`{}`)},
		},
	}
	ws := &mockWebhookStore{
		subs: map[string]*Subscription{
			"s1": {ID: "s1", URL: "https://example.com/hook", Secret: "sec", Status: "ACTIVE"},
		},
	}
	disp := &mockDispatcher{statusCode: 200, respBody: "OK"}

	w := New(ds, ws, disp, time.Second)
	w.ProcessPending()

	if len(ds.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(ds.updates))
	}
	u := ds.updates[0]
	if u.ID != "d1" {
		t.Errorf("expected update for d1, got %s", u.ID)
	}
	if u.Status != "DELIVERED" {
		t.Errorf("expected DELIVERED status, got %s", u.Status)
	}
	if u.Code != 200 {
		t.Errorf("expected status code 200, got %d", u.Code)
	}
	if u.NextRetry != nil {
		t.Error("expected nil NextRetry for successful delivery")
	}
}

func TestProcessPending_SchedulesRetryOnFailure(t *testing.T) {
	ds := &mockDeliveryStore{
		pending: []*Delivery{
			{ID: "d1", SubscriptionID: "s1", EventType: "user.created", Payload: []byte(`{}`), Attempts: 0},
		},
	}
	ws := &mockWebhookStore{
		subs: map[string]*Subscription{
			"s1": {ID: "s1", URL: "https://example.com/hook", Secret: "sec", Status: "ACTIVE"},
		},
	}
	disp := &mockDispatcher{statusCode: 500, respBody: "Internal Server Error"}

	w := New(ds, ws, disp, time.Second)
	processed, succeeded, failed := w.ProcessPending()

	if processed != 1 {
		t.Errorf("expected 1 processed, got %d", processed)
	}
	if succeeded != 0 {
		t.Errorf("expected 0 succeeded, got %d", succeeded)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}

	if len(ds.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(ds.updates))
	}
	u := ds.updates[0]
	if u.Status != "PENDING" {
		t.Errorf("expected PENDING status for retry, got %s", u.Status)
	}
	if u.NextRetry == nil {
		t.Error("expected NextRetry to be set for failed delivery")
	}
}

func TestProcessPending_SchedulesRetryOnDispatchError(t *testing.T) {
	ds := &mockDeliveryStore{
		pending: []*Delivery{
			{ID: "d1", SubscriptionID: "s1", EventType: "user.created", Payload: []byte(`{}`), Attempts: 1},
		},
	}
	ws := &mockWebhookStore{
		subs: map[string]*Subscription{
			"s1": {ID: "s1", URL: "https://example.com/hook", Secret: "sec", Status: "ACTIVE"},
		},
	}
	disp := &mockDispatcher{err: errors.New("connection refused")}

	w := New(ds, ws, disp, time.Second)
	_, _, failed := w.ProcessPending()

	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
	if len(ds.updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(ds.updates))
	}
	u := ds.updates[0]
	if u.Status != "PENDING" {
		t.Errorf("expected PENDING for retry, got %s", u.Status)
	}
	if u.NextRetry == nil {
		t.Error("expected NextRetry to be set")
	}
	if u.Body != "connection refused" {
		t.Errorf("expected error message in body, got %q", u.Body)
	}
}

func TestProcessPending_SkipsDisabledSubscriptions(t *testing.T) {
	ds := &mockDeliveryStore{
		pending: []*Delivery{
			{ID: "d1", SubscriptionID: "s1", EventType: "user.created", Payload: []byte(`{}`)},
		},
	}
	ws := &mockWebhookStore{
		subs: map[string]*Subscription{
			"s1": {ID: "s1", URL: "https://example.com/hook", Secret: "sec", Status: "DISABLED"},
		},
	}
	disp := &mockDispatcher{statusCode: 200}

	w := New(ds, ws, disp, time.Second)
	processed, succeeded, failed := w.ProcessPending()

	if processed != 1 {
		t.Errorf("expected 1 processed, got %d", processed)
	}
	if succeeded != 0 {
		t.Errorf("expected 0 succeeded, got %d", succeeded)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}
	if len(disp.calls) != 0 {
		t.Error("expected no dispatch calls for disabled subscription")
	}
}

func TestProcessPending_NoPendingItems(t *testing.T) {
	ds := &mockDeliveryStore{pending: nil}
	ws := &mockWebhookStore{subs: map[string]*Subscription{}}
	disp := &mockDispatcher{}

	w := New(ds, ws, disp, time.Second)
	processed, succeeded, failed := w.ProcessPending()

	if processed != 0 || succeeded != 0 || failed != 0 {
		t.Errorf("expected all zeros, got processed=%d, succeeded=%d, failed=%d", processed, succeeded, failed)
	}
}

func TestStartStop_Lifecycle(t *testing.T) {
	ds := &mockDeliveryStore{pending: nil}
	ws := &mockWebhookStore{subs: map[string]*Subscription{}}
	disp := &mockDispatcher{}

	w := New(ds, ws, disp, 10*time.Millisecond)
	w.Start()

	// Let the worker run a few cycles
	time.Sleep(50 * time.Millisecond)

	// Stop should return without hanging
	done := make(chan struct{})
	go func() {
		w.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop did not return within timeout")
	}
}

func TestProcessPending_ListError(t *testing.T) {
	ds := &mockDeliveryStore{listErr: errors.New("db down")}
	ws := &mockWebhookStore{subs: map[string]*Subscription{}}
	disp := &mockDispatcher{}

	w := New(ds, ws, disp, time.Second)
	processed, succeeded, failed := w.ProcessPending()

	if processed != 0 || succeeded != 0 || failed != 0 {
		t.Errorf("expected all zeros on list error, got processed=%d, succeeded=%d, failed=%d", processed, succeeded, failed)
	}
}

func TestProcessPending_SubscriptionNotFound(t *testing.T) {
	ds := &mockDeliveryStore{
		pending: []*Delivery{
			{ID: "d1", SubscriptionID: "nonexistent", EventType: "user.created", Payload: []byte(`{}`)},
		},
	}
	ws := &mockWebhookStore{subs: map[string]*Subscription{}}
	disp := &mockDispatcher{}

	w := New(ds, ws, disp, time.Second)
	processed, _, failed := w.ProcessPending()

	if processed != 1 {
		t.Errorf("expected 1 processed, got %d", processed)
	}
	if failed != 1 {
		t.Errorf("expected 1 failed, got %d", failed)
	}
	if len(disp.calls) != 0 {
		t.Error("expected no dispatch calls for missing subscription")
	}
}
