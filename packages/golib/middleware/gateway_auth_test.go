package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewayAuth_APIKeySuccess(t *testing.T) {
	auth := NewAuthenticator(
		WithAPIKeyValidator(func(key string) (*AuthResult, error) {
			if key == "valid-key" {
				return &AuthResult{Authenticated: true, Subject: "app-123", Metadata: map[string]string{"plan": "starter"}}, nil
			}
			return nil, errors.New("invalid key")
		}),
	)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "valid-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if m := w.Header().Get("X-Auth-Method"); m != "api_key" {
		t.Errorf("expected X-Auth-Method=api_key, got %q", m)
	}
	if s := w.Header().Get("X-Auth-Subject"); s != "app-123" {
		t.Errorf("expected X-Auth-Subject=app-123, got %q", s)
	}
}

func TestGatewayAuth_BearerTokenSuccess(t *testing.T) {
	auth := NewAuthenticator(
		WithTokenValidator(func(token string) (*AuthResult, error) {
			if token == "valid-token" {
				return &AuthResult{Authenticated: true, Subject: "user-456"}, nil
			}
			return nil, errors.New("invalid token")
		}),
	)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if m := w.Header().Get("X-Auth-Method"); m != "bearer" {
		t.Errorf("expected X-Auth-Method=bearer, got %q", m)
	}
}

func TestGatewayAuth_SessionCookieSuccess(t *testing.T) {
	auth := NewAuthenticator(
		WithSessionValidator(func(cookie string) (*AuthResult, error) {
			if cookie == "valid-session" {
				return &AuthResult{Authenticated: true, Subject: "user-789"}, nil
			}
			return nil, errors.New("invalid session")
		}),
	)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: "session", Value: "valid-session"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if m := w.Header().Get("X-Auth-Method"); m != "session" {
		t.Errorf("expected X-Auth-Method=session, got %q", m)
	}
}

func TestGatewayAuth_ServiceSignatureSuccess(t *testing.T) {
	auth := NewAuthenticator(
		WithServiceValidator(func(sig string, r *http.Request) (*AuthResult, error) {
			if sig == "valid-sig" {
				return &AuthResult{Authenticated: true, Subject: "svc-billing"}, nil
			}
			return nil, errors.New("invalid signature")
		}),
	)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Service-Signature", "valid-sig")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if m := w.Header().Get("X-Auth-Method"); m != "service" {
		t.Errorf("expected X-Auth-Method=service, got %q", m)
	}
}

func TestGatewayAuth_NoCredentials401(t *testing.T) {
	auth := NewAuthenticator(
		WithAPIKeyValidator(func(key string) (*AuthResult, error) {
			return nil, errors.New("invalid")
		}),
	)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGatewayAuth_InvalidAPIKey401(t *testing.T) {
	auth := NewAuthenticator(
		WithAPIKeyValidator(func(key string) (*AuthResult, error) {
			return nil, errors.New("invalid key")
		}),
	)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "bad-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestGatewayAuth_ResultInContext(t *testing.T) {
	auth := NewAuthenticator(
		WithAPIKeyValidator(func(key string) (*AuthResult, error) {
			return &AuthResult{
				Authenticated: true,
				Subject:       "app-ctx",
				Metadata:      map[string]string{"role": "admin"},
			}, nil
		}),
	)

	var gotResult *AuthResult
	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		result, ok := GetAuthResult(r.Context())
		if !ok {
			t.Error("expected auth result in context")
			return
		}
		gotResult = result
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "any-key")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if gotResult == nil {
		t.Fatal("auth result was nil")
	}
	if gotResult.Subject != "app-ctx" {
		t.Errorf("expected subject app-ctx, got %q", gotResult.Subject)
	}
	if gotResult.Method != AuthAPIKey {
		t.Errorf("expected method AuthAPIKey, got %v", gotResult.Method)
	}
}

func TestGatewayAuth_XAuthMethodHeader(t *testing.T) {
	auth := NewAuthenticator(
		WithTokenValidator(func(token string) (*AuthResult, error) {
			return &AuthResult{Authenticated: true, Subject: "user-1"}, nil
		}),
	)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if m := w.Header().Get("X-Auth-Method"); m != "bearer" {
		t.Errorf("expected X-Auth-Method=bearer, got %q", m)
	}
}

func TestGatewayAuth_ServicePrecedenceOverBearer(t *testing.T) {
	auth := NewAuthenticator(
		WithServiceValidator(func(sig string, r *http.Request) (*AuthResult, error) {
			return &AuthResult{Authenticated: true, Subject: "svc-internal"}, nil
		}),
		WithTokenValidator(func(token string) (*AuthResult, error) {
			return &AuthResult{Authenticated: true, Subject: "user-external"}, nil
		}),
	)

	handler := auth.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Service-Signature", "sig")
	r.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if m := w.Header().Get("X-Auth-Method"); m != "service" {
		t.Errorf("expected service to take precedence, got X-Auth-Method=%q", m)
	}
	if s := w.Header().Get("X-Auth-Subject"); s != "svc-internal" {
		t.Errorf("expected svc-internal, got %q", s)
	}
}

func TestGetAuthResult_NilContext(t *testing.T) {
	result, ok := GetAuthResult(nil)
	if ok || result != nil {
		t.Error("expected nil result and false for nil context")
	}
}
