package audithttp

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type testEmitter struct {
	mu     sync.Mutex
	events []Event
}

func (e *testEmitter) Emit(event Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, event)
}

func (e *testEmitter) last() Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.events[len(e.events)-1]
}

func TestMiddleware_EmitsEvent(t *testing.T) {
	emitter := &testEmitter{}
	handler := Middleware(Config{Emitter: emitter})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req.Header.Set("X-User-ID", "user-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if len(emitter.events) != 1 {
		t.Fatalf("events: got %d", len(emitter.events))
	}

	event := emitter.last()
	if event.Method != "GET" {
		t.Errorf("method: got %q", event.Method)
	}
	if event.Path != "/api/users" {
		t.Errorf("path: got %q", event.Path)
	}
	if event.UserID != "user-123" {
		t.Errorf("user: got %q", event.UserID)
	}
	if event.Action != "READ" {
		t.Errorf("action: got %q", event.Action)
	}
	if event.StatusCode != 200 {
		t.Errorf("status: got %d", event.StatusCode)
	}
}

func TestMiddleware_ActionMapping(t *testing.T) {
	tests := []struct {
		method string
		action string
	}{
		{http.MethodGet, "READ"},
		{http.MethodPost, "CREATE"},
		{http.MethodPut, "UPDATE"},
		{http.MethodPatch, "UPDATE"},
		{http.MethodDelete, "DELETE"},
		{http.MethodOptions, "ACCESS"},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			emitter := &testEmitter{}
			handler := Middleware(Config{Emitter: emitter})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(tt.method, "/api/data", nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if emitter.last().Action != tt.action {
				t.Errorf("%s: got %q, want %q", tt.method, emitter.last().Action, tt.action)
			}
		})
	}
}

func TestMiddleware_SkipPaths(t *testing.T) {
	emitter := &testEmitter{}
	handler := Middleware(Config{
		Emitter:   emitter,
		SkipPaths: map[string]bool{"/healthz": true},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if len(emitter.events) != 0 {
		t.Error("healthz should be skipped")
	}
}

func TestMiddleware_ResourceExtraction(t *testing.T) {
	emitter := &testEmitter{}
	handler := Middleware(Config{Emitter: emitter})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/550e8400-e29b-41d4-a716-446655440000", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if emitter.last().Resource != "users" {
		t.Errorf("resource: got %q, want 'users'", emitter.last().Resource)
	}
}

func TestMiddleware_Duration(t *testing.T) {
	emitter := &testEmitter{}
	handler := Middleware(Config{Emitter: emitter})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if emitter.last().Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestMiddleware_NilEmitter(t *testing.T) {
	handler := Middleware(Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req) // Should not panic.

	if w.Code != http.StatusOK {
		t.Errorf("nil emitter: got %d", w.Code)
	}
}

func TestMiddleware_TenantHeader(t *testing.T) {
	emitter := &testEmitter{}
	handler := Middleware(Config{Emitter: emitter})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if emitter.last().TenantID != "tenant-abc" {
		t.Errorf("tenant: got %q", emitter.last().TenantID)
	}
}

func TestEmitterFunc(t *testing.T) {
	var captured Event
	fn := EmitterFunc(func(e Event) { captured = e })

	fn.Emit(Event{Method: "GET", Path: "/test"})
	if captured.Method != "GET" {
		t.Error("EmitterFunc should work")
	}
}

func TestIsID(t *testing.T) {
	if !isID("550e8400-e29b-41d4-a716-446655440000") {
		t.Error("UUID should be recognized as ID")
	}
	if !isID("12345") {
		t.Error("numeric should be recognized as ID")
	}
	if isID("users") {
		t.Error("word should not be ID")
	}
}
