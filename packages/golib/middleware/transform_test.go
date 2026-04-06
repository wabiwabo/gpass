package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func jsonHandler(data map[string]interface{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(data)
	})
}

func TestStripPrefix(t *testing.T) {
	var capturedPath string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})

	handler := TransformRequest(StripPrefix("/api/v1"))(inner)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedPath != "/users" {
		t.Errorf("expected path '/users', got '%s'", capturedPath)
	}
}

func TestAddHeader(t *testing.T) {
	var capturedHeader string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get("X-Tenant-ID")
		w.WriteHeader(http.StatusOK)
	})

	handler := TransformRequest(AddHeader("X-Tenant-ID", "tenant-123"))(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedHeader != "tenant-123" {
		t.Errorf("expected header 'tenant-123', got '%s'", capturedHeader)
	}
}

func TestRedactResponseField(t *testing.T) {
	data := map[string]interface{}{
		"name":     "John",
		"email":    "john@example.com",
		"password": "secret",
	}

	handler := TransformResponse(RedactResponseField("password"))(jsonHandler(data))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)

	if _, ok := result["password"]; ok {
		t.Error("expected 'password' field to be redacted")
	}
	if result["name"] != "John" {
		t.Errorf("expected 'name' to be 'John', got '%v'", result["name"])
	}
	if result["email"] != "john@example.com" {
		t.Errorf("expected 'email' to be preserved, got '%v'", result["email"])
	}
}

func TestAddResponseHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})

	handler := TransformResponse(AddResponseHeader("X-Custom", "value-123"))(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Custom") != "value-123" {
		t.Errorf("expected header 'X-Custom' to be 'value-123', got '%s'", rec.Header().Get("X-Custom"))
	}
}

func TestMultipleTransformersChained(t *testing.T) {
	var capturedPath string
	var capturedTenant string

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedTenant = r.Header.Get("X-Tenant-ID")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"name":   "Alice",
			"secret": "hidden",
		})
	})

	// Chain request transformers
	reqMiddleware := TransformRequest(
		StripPrefix("/api/v2"),
		AddHeader("X-Tenant-ID", "tenant-abc"),
	)
	// Chain response transformers
	respMiddleware := TransformResponse(
		RedactResponseField("secret"),
		AddResponseHeader("X-API-Version", "2"),
	)

	handler := reqMiddleware(respMiddleware(inner))

	req := httptest.NewRequest(http.MethodGet, "/api/v2/users", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedPath != "/users" {
		t.Errorf("expected path '/users', got '%s'", capturedPath)
	}
	if capturedTenant != "tenant-abc" {
		t.Errorf("expected tenant 'tenant-abc', got '%s'", capturedTenant)
	}

	var result map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&result)
	if _, ok := result["secret"]; ok {
		t.Error("expected 'secret' field to be redacted")
	}
	if result["name"] != "Alice" {
		t.Errorf("expected 'name' to be 'Alice', got '%v'", result["name"])
	}
	if rec.Header().Get("X-API-Version") != "2" {
		t.Errorf("expected X-API-Version '2', got '%s'", rec.Header().Get("X-API-Version"))
	}
}

func TestRedactResponseField_NonJSON(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello world"))
	})

	handler := TransformResponse(RedactResponseField("password"))(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if body != "hello world" {
		t.Errorf("expected body 'hello world', got '%s'", body)
	}
}
