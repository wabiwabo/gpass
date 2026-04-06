package scopecheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetScopes_GetScopes(t *testing.T) {
	ctx := context.Background()
	ctx = SetScopes(ctx, []string{"openid", "profile", "email"})

	scopes := GetScopes(ctx)
	if len(scopes) != 3 {
		t.Fatalf("len = %d, want 3", len(scopes))
	}
	// Sorted
	want := []string{"email", "openid", "profile"}
	for i, w := range want {
		if scopes[i] != w {
			t.Errorf("scopes[%d] = %q, want %q", i, scopes[i], w)
		}
	}
}

func TestSetScopes_TrimsWhitespace(t *testing.T) {
	ctx := SetScopes(context.Background(), []string{"  openid  ", " profile ", ""})
	scopes := GetScopes(ctx)
	if len(scopes) != 2 {
		t.Fatalf("len = %d, want 2 (empty string filtered)", len(scopes))
	}
}

func TestSetScopes_Deduplication(t *testing.T) {
	ctx := SetScopes(context.Background(), []string{"read", "write", "read"})
	scopes := GetScopes(ctx)
	if len(scopes) != 2 {
		t.Fatalf("len = %d, want 2 (deduped)", len(scopes))
	}
}

func TestGetScopes_EmptyContext(t *testing.T) {
	scopes := GetScopes(context.Background())
	if scopes != nil {
		t.Errorf("scopes = %v, want nil", scopes)
	}
}

func TestHasScope(t *testing.T) {
	ctx := SetScopes(context.Background(), []string{"openid", "profile", "email"})

	tests := []struct {
		scope string
		want  bool
	}{
		{"openid", true},
		{"profile", true},
		{"email", true},
		{"admin", false},
		{"", false},
		{"OPENID", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			if got := HasScope(ctx, tt.scope); got != tt.want {
				t.Errorf("HasScope(%q) = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestHasScope_EmptyContext(t *testing.T) {
	if HasScope(context.Background(), "anything") {
		t.Error("empty context should not have any scope")
	}
}

func TestHasAllScopes(t *testing.T) {
	ctx := SetScopes(context.Background(), []string{"read", "write", "admin"})

	tests := []struct {
		name     string
		required []string
		want     bool
	}{
		{"all present", []string{"read", "write"}, true},
		{"single present", []string{"admin"}, true},
		{"one missing", []string{"read", "delete"}, false},
		{"all missing", []string{"delete", "manage"}, false},
		{"empty required", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAllScopes(ctx, tt.required...); got != tt.want {
				t.Errorf("HasAllScopes(%v) = %v, want %v", tt.required, got, tt.want)
			}
		})
	}
}

func TestHasAnyScope(t *testing.T) {
	ctx := SetScopes(context.Background(), []string{"read", "write"})

	tests := []struct {
		name   string
		scopes []string
		want   bool
	}{
		{"one matches", []string{"read", "admin"}, true},
		{"both match", []string{"read", "write"}, true},
		{"none match", []string{"admin", "delete"}, false},
		{"empty", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAnyScope(ctx, tt.scopes...); got != tt.want {
				t.Errorf("HasAnyScope(%v) = %v, want %v", tt.scopes, got, tt.want)
			}
		})
	}
}

func TestRequire_Allowed(t *testing.T) {
	mw := Require("read", "write")
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := SetScopes(req.Context(), []string{"read", "write", "admin"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called when scopes are present")
	}
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestRequire_Denied(t *testing.T) {
	mw := Require("admin")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/admin", nil)
	ctx := SetScopes(req.Context(), []string{"read"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "insufficient_scope" {
		t.Errorf("error = %v", body["error"])
	}
}

func TestRequire_NoScopes(t *testing.T) {
	mw := Require("read")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestRequireAny_Allowed(t *testing.T) {
	mw := RequireAny("admin", "superadmin")
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := SetScopes(req.Context(), []string{"admin"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should be called with any matching scope")
	}
}

func TestRequireAny_Denied(t *testing.T) {
	mw := RequireAny("admin", "superadmin")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	ctx := SetScopes(req.Context(), []string{"read", "write"})
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 403 {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestParseSpaceDelimited(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"standard", "openid profile email", 3},
		{"single", "openid", 1},
		{"extra spaces", "  openid   profile  ", 2},
		{"empty", "", 0},
		{"whitespace only", "   ", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseSpaceDelimited(tt.input)
			if len(got) != tt.want {
				t.Errorf("len = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestScopeMiddleware(t *testing.T) {
	mw := ScopeMiddleware("X-Scopes")

	var captured []string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetScopes(r.Context())
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Scopes", "read write admin")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if len(captured) != 3 {
		t.Fatalf("captured scopes len = %d, want 3", len(captured))
	}
}

func TestScopeMiddleware_NoHeader(t *testing.T) {
	mw := ScopeMiddleware("")

	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		scopes := GetScopes(r.Context())
		if scopes != nil {
			t.Error("should have no scopes when header is absent")
		}
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("handler should still be called")
	}
}

func TestScopeMiddleware_DefaultHeader(t *testing.T) {
	mw := ScopeMiddleware("") // should default to X-Scopes

	var captured []string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = GetScopes(r.Context())
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Scopes", "openid")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if len(captured) != 1 || captured[0] != "openid" {
		t.Errorf("captured = %v", captured)
	}
}

func TestRequire_ResponseContentType(t *testing.T) {
	mw := Require("admin")
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
