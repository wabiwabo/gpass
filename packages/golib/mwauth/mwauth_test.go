package mwauth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func validValidator(_ context.Context, token string) (TokenInfo, error) {
	if token == "valid-token" {
		return TokenInfo{Subject: "user-123", TokenType: "access"}, nil
	}
	return TokenInfo{}, errors.New("invalid token")
}

func TestMiddleware_ValidToken(t *testing.T) {
	mw := Middleware(Config{Validator: validValidator})
	var captured TokenInfo

	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured, _ = GetToken(r.Context())
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 { t.Errorf("status = %d", w.Code) }
	if captured.Subject != "user-123" { t.Errorf("Subject = %q", captured.Subject) }
	if captured.Raw != "valid-token" { t.Errorf("Raw = %q", captured.Raw) }
}

func TestMiddleware_InvalidToken(t *testing.T) {
	mw := Middleware(Config{Validator: validValidator})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call handler")
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 { t.Errorf("status = %d", w.Code) }
}

func TestMiddleware_MissingAuth(t *testing.T) {
	mw := Middleware(Config{Validator: validValidator})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call handler")
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 { t.Errorf("status = %d", w.Code) }
}

func TestMiddleware_NotBearer(t *testing.T) {
	mw := Middleware(Config{Validator: validValidator})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call handler")
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 { t.Errorf("status = %d", w.Code) }
}

func TestMiddleware_SkipPaths(t *testing.T) {
	mw := Middleware(Config{
		Validator: validValidator,
		SkipPaths: []string{"/health", "/ready"},
	})
	var called bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called { t.Error("should skip auth for /health") }
}

func TestMiddleware_EmptyBearer(t *testing.T) {
	mw := Middleware(Config{Validator: validValidator})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not call handler")
	}))

	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 401 { t.Errorf("status = %d", w.Code) }
}

func TestExtractBearer(t *testing.T) {
	if ExtractBearer("Bearer abc123") != "abc123" { t.Error("valid") }
	if ExtractBearer("Basic xyz") != "" { t.Error("basic") }
	if ExtractBearer("") != "" { t.Error("empty") }
}

func TestGetToken_Empty(t *testing.T) {
	_, ok := GetToken(context.Background())
	if ok { t.Error("should be false") }
}

func TestMiddleware_CustomError(t *testing.T) {
	mw := Middleware(Config{
		Validator:    validValidator,
		ErrorMessage: `{"custom":"error"}`,
	})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/api", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !strings.Contains(w.Body.String(), "custom") { t.Error("custom error") }
}
