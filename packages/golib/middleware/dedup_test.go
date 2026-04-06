package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDedup_SameRequestWithinWindowRejected(t *testing.T) {
	handler := Dedup(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	}))

	// First request should pass.
	req1 := httptest.NewRequest(http.MethodPost, "/api/submit", strings.NewReader(`{"name":"test"}`))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Errorf("first request: got %d, want %d", rec1.Code, http.StatusCreated)
	}

	// Same request should be rejected.
	req2 := httptest.NewRequest(http.MethodPost, "/api/submit", strings.NewReader(`{"name":"test"}`))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("duplicate request: got %d, want %d", rec2.Code, http.StatusConflict)
	}
}

func TestDedup_SameRequestAfterWindowPassesThrough(t *testing.T) {
	handler := Dedup(50 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/api/submit", strings.NewReader(`{"data":"1"}`))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first request: got %d, want %d", rec1.Code, http.StatusCreated)
	}

	// Wait for window to expire.
	time.Sleep(60 * time.Millisecond)

	req2 := httptest.NewRequest(http.MethodPost, "/api/submit", strings.NewReader(`{"data":"1"}`))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Errorf("request after window: got %d, want %d", rec2.Code, http.StatusCreated)
	}
}

func TestDedup_DifferentRequestsPassThrough(t *testing.T) {
	callCount := 0
	handler := Dedup(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusCreated)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/api/submit", strings.NewReader(`{"name":"Alice"}`))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/api/submit", strings.NewReader(`{"name":"Bob"}`))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec1.Code != http.StatusCreated || rec2.Code != http.StatusCreated {
		t.Errorf("different requests should both pass: got %d and %d", rec1.Code, rec2.Code)
	}
	if callCount != 2 {
		t.Errorf("handler should be called twice, got %d", callCount)
	}
}

func TestDedup_GETNotDeduplicated(t *testing.T) {
	callCount := 0
	handler := Dedup(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("GET request %d: got %d, want %d", i, rec.Code, http.StatusOK)
		}
	}

	if callCount != 3 {
		t.Errorf("GET requests should all pass through: got %d calls, want 3", callCount)
	}
}

func TestDedup_DELETENotDeduplicated(t *testing.T) {
	callCount := 0
	handler := Dedup(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusNoContent)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodDelete, "/api/resource/1", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Errorf("DELETE request %d: got %d, want %d", i, rec.Code, http.StatusNoContent)
		}
	}

	if callCount != 2 {
		t.Errorf("DELETE requests should all pass through: got %d calls, want 2", callCount)
	}
}

func TestDedup_DifferentUsersCanMakeSameRequest(t *testing.T) {
	callCount := 0
	handler := Dedup(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusCreated)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/api/submit", strings.NewReader(`{"data":"same"}`))
	req1.Header.Set("X-User-ID", "user-1")
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/api/submit", strings.NewReader(`{"data":"same"}`))
	req2.Header.Set("X-User-ID", "user-2")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec1.Code != http.StatusCreated || rec2.Code != http.StatusCreated {
		t.Errorf("different users same request should both pass: got %d and %d", rec1.Code, rec2.Code)
	}
	if callCount != 2 {
		t.Errorf("handler should be called twice for different users, got %d", callCount)
	}
}

func TestDedup_DifferentPathsPassThrough(t *testing.T) {
	callCount := 0
	handler := Dedup(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusCreated)
	}))

	req1 := httptest.NewRequest(http.MethodPost, "/api/resource-a", strings.NewReader(`{"data":"same"}`))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/api/resource-b", strings.NewReader(`{"data":"same"}`))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec1.Code != http.StatusCreated || rec2.Code != http.StatusCreated {
		t.Errorf("different paths should both pass: got %d and %d", rec1.Code, rec2.Code)
	}
	if callCount != 2 {
		t.Errorf("handler should be called twice, got %d", callCount)
	}
}

func TestDedup_PUTDeduplicated(t *testing.T) {
	handler := Dedup(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodPut, "/api/resource/1", strings.NewReader(`{"name":"updated"}`))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first PUT: got %d, want %d", rec1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodPut, "/api/resource/1", strings.NewReader(`{"name":"updated"}`))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("duplicate PUT: got %d, want %d", rec2.Code, http.StatusConflict)
	}
}

func TestDedup_PATCHDeduplicated(t *testing.T) {
	handler := Dedup(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodPatch, "/api/resource/1", strings.NewReader(`{"name":"patched"}`))
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first PATCH: got %d, want %d", rec1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodPatch, "/api/resource/1", strings.NewReader(`{"name":"patched"}`))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Errorf("duplicate PATCH: got %d, want %d", rec2.Code, http.StatusConflict)
	}
}

func TestDedup_NilBody(t *testing.T) {
	handler := Dedup(5 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/action", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("POST with nil body: got %d, want %d", rec.Code, http.StatusCreated)
	}
}
