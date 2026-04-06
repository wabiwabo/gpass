package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	apperrors "github.com/garudapass/gpass/packages/golib/errors"
)

func TestHandleError_NoError(t *testing.T) {
	handler := HandleError(func(w http.ResponseWriter, r *http.Request) error {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHandleError_AppError(t *testing.T) {
	handler := HandleError(func(w http.ResponseWriter, r *http.Request) error {
		return apperrors.NotFound(apperrors.CodeResourceNotFound, "user not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("Content-Type: got %q", ct)
	}

	var problem ProblemResponse
	json.NewDecoder(w.Body).Decode(&problem)
	if problem.Title != apperrors.CodeResourceNotFound {
		t.Errorf("title: got %q", problem.Title)
	}
	if problem.Detail != "user not found" {
		t.Errorf("detail: got %q", problem.Detail)
	}
}

func TestHandleError_WrappedAppError(t *testing.T) {
	handler := HandleError(func(w http.ResponseWriter, r *http.Request) error {
		cause := fmt.Errorf("db connection failed")
		return apperrors.Wrap(cause, apperrors.Internal(apperrors.CodeServiceUnavailable, "database error"))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var problem ProblemResponse
	json.NewDecoder(w.Body).Decode(&problem)
	if problem.Title != apperrors.CodeServiceUnavailable {
		t.Errorf("title: got %q", problem.Title)
	}
}

func TestHandleError_GenericError(t *testing.T) {
	handler := HandleError(func(w http.ResponseWriter, r *http.Request) error {
		return fmt.Errorf("something went wrong")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	var problem ProblemResponse
	json.NewDecoder(w.Body).Decode(&problem)
	if problem.Title != "internal_error" {
		t.Errorf("title: got %q", problem.Title)
	}
	// Should NOT expose internal error message.
	if problem.Detail == "something went wrong" {
		t.Error("should not expose internal error details")
	}
}

func TestPanicRecovery_NoPanic(t *testing.T) {
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestPanicRecovery_RecoversPanic(t *testing.T) {
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something terrible happened")
	}))

	req := httptest.NewRequest(http.MethodGet, "/dangerous", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/problem+json" {
		t.Errorf("Content-Type: got %q", ct)
	}

	var problem ProblemResponse
	json.NewDecoder(w.Body).Decode(&problem)
	if problem.Title != "internal_error" {
		t.Errorf("title: got %q", problem.Title)
	}
}

func TestPanicRecovery_NilPanicValue(t *testing.T) {
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(nil)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// panic(nil) in Go 1.21+ is treated as a real panic.
	handler.ServeHTTP(w, req)
	// Should not crash regardless.
}

func TestHandleError_CacheControlNoStore(t *testing.T) {
	handler := HandleError(func(w http.ResponseWriter, r *http.Request) error {
		return apperrors.BadRequest(apperrors.CodeInvalidRequest, "bad")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control: got %q, want no-store", cc)
	}
}

func TestHandleError_BadRequest(t *testing.T) {
	handler := HandleError(func(w http.ResponseWriter, r *http.Request) error {
		return apperrors.BadRequest(apperrors.CodeInvalidJSON, "invalid JSON body")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/resource", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestHandleError_TooManyRequests(t *testing.T) {
	handler := HandleError(func(w http.ResponseWriter, r *http.Request) error {
		return apperrors.TooManyRequests(apperrors.CodeRateLimitExceeded, "rate limit exceeded")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
}
