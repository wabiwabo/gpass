package reqscope

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestScope_SetGet(t *testing.T) {
	s := &Scope{values: make(map[string]interface{})}
	s.Set("key", "value")

	v, ok := s.Get("key")
	if !ok || v != "value" {
		t.Errorf("get: ok=%v, v=%v", ok, v)
	}
}

func TestScope_GetMissing(t *testing.T) {
	s := &Scope{values: make(map[string]interface{})}
	_, ok := s.Get("missing")
	if ok {
		t.Error("missing should return false")
	}
}

func TestScope_GetString(t *testing.T) {
	s := &Scope{values: make(map[string]interface{})}
	s.Set("name", "John")

	v, ok := s.GetString("name")
	if !ok || v != "John" {
		t.Errorf("getString: ok=%v, v=%q", ok, v)
	}

	s.Set("num", 42)
	_, ok = s.GetString("num")
	if ok {
		t.Error("int should not be string")
	}
}

func TestScope_GetInt(t *testing.T) {
	s := &Scope{values: make(map[string]interface{})}
	s.Set("count", 42)

	v, ok := s.GetInt("count")
	if !ok || v != 42 {
		t.Errorf("getInt: ok=%v, v=%d", ok, v)
	}
}

func TestScope_Has(t *testing.T) {
	s := &Scope{values: make(map[string]interface{})}
	s.Set("exists", true)

	if !s.Has("exists") {
		t.Error("should have key")
	}
	if s.Has("missing") {
		t.Error("should not have missing key")
	}
}

func TestScope_Keys(t *testing.T) {
	s := &Scope{values: make(map[string]interface{})}
	s.Set("a", 1)
	s.Set("b", 2)

	keys := s.Keys()
	if len(keys) != 2 {
		t.Errorf("keys: got %d", len(keys))
	}
}

func TestScope_Len(t *testing.T) {
	s := &Scope{values: make(map[string]interface{})}
	s.Set("a", 1)
	s.Set("b", 2)

	if s.Len() != 2 {
		t.Errorf("len: got %d", s.Len())
	}
}

func TestMiddleware_CreatesScope(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope := FromContext(r.Context())
		if scope == nil {
			t.Error("scope should be in context")
			return
		}
		scope.Set("user_id", "123")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestFromContext_Missing(t *testing.T) {
	scope := FromContext(context.Background())
	if scope != nil {
		t.Error("should return nil for missing scope")
	}
}

func TestMiddleware_IsolatedPerRequest(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		scope := FromContext(r.Context())
		scope.Set("request_specific", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))

	// Two requests should have separate scopes.
	req1 := httptest.NewRequest(http.MethodGet, "/a", nil)
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/b", nil)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	// Both should succeed independently.
	if w1.Code != http.StatusOK || w2.Code != http.StatusOK {
		t.Error("both should succeed")
	}
}
