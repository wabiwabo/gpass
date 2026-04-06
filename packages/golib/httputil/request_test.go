package httputil

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUserID_Present(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-User-ID", "user-123")

	id, err := UserID(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "user-123" {
		t.Errorf("expected 'user-123', got %q", id)
	}
}

func TestUserID_Missing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := UserID(r)
	if err == nil {
		t.Fatal("expected error for missing header")
	}
	if !strings.Contains(err.Error(), "X-User-ID") {
		t.Errorf("expected error to mention X-User-ID, got: %v", err)
	}
}

func TestDecodeJSON_Valid(t *testing.T) {
	type input struct {
		Name string `json:"name"`
	}

	body := bytes.NewBufferString(`{"name":"Alice"}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var got input
	if err := DecodeJSON(r, &got); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name != "Alice" {
		t.Errorf("expected 'Alice', got %q", got.Name)
	}
}

func TestDecodeJSON_Invalid(t *testing.T) {
	body := bytes.NewBufferString(`{invalid}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var got map[string]string
	err := DecodeJSON(r, &got)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecodeJSON_TooLarge(t *testing.T) {
	// Create a body larger than 1MB
	large := strings.Repeat("a", 1<<20+100)
	body := bytes.NewBufferString(`{"name":"` + large + `"}`)
	r := httptest.NewRequest(http.MethodPost, "/", body)

	var got map[string]string
	err := DecodeJSON(r, &got)
	if err == nil {
		t.Fatal("expected error for too-large body")
	}
}

func TestDecodeJSON_EmptyBody(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))

	var got map[string]string
	err := DecodeJSON(r, &got)
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestPathParam(t *testing.T) {
	mux := http.NewServeMux()
	var got string
	mux.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		got = PathParam(r, "id")
	})

	r := httptest.NewRequest(http.MethodGet, "/users/abc-123", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)

	if got != "abc-123" {
		t.Errorf("expected 'abc-123', got %q", got)
	}
}
