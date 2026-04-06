package reqbody

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadJSON(t *testing.T) {
	body := `{"name":"John","age":30}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var result struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	if err := ReadJSON(req, &result, DefaultConfig()); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if result.Name != "John" || result.Age != 30 {
		t.Errorf("result = %+v", result)
	}
}

func TestReadJSON_WrongContentType(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("data"))
	req.Header.Set("Content-Type", "text/plain")

	var v interface{}
	if err := ReadJSON(req, &v, DefaultConfig()); err == nil {
		t.Error("should reject non-JSON content type")
	}
}

func TestReadJSON_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")

	var v interface{}
	if err := ReadJSON(req, &v, DefaultConfig()); err == nil {
		t.Error("should reject invalid JSON")
	}
}

func TestReadJSON_TooLarge(t *testing.T) {
	huge := strings.Repeat("x", 1024*1024+1)
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"data":"`+huge+`"}`))
	req.Header.Set("Content-Type", "application/json")

	var v interface{}
	cfg := Config{MaxSize: 1024} // 1KB limit
	if err := ReadJSON(req, &v, cfg); err == nil {
		t.Error("should reject oversized body")
	}
}

func TestReadJSON_NoContentType(t *testing.T) {
	body := `{"key":"value"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	// No Content-Type header

	var v map[string]string
	if err := ReadJSON(req, &v, DefaultConfig()); err != nil {
		t.Errorf("should accept missing content type: %v", err)
	}
}

func TestReadBytes(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("raw data"))

	data, err := ReadBytes(req, DefaultConfig())
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if string(data) != "raw data" {
		t.Errorf("data = %q", data)
	}
}

func TestReadBytes_ContentTypeFilter(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("data"))
	req.Header.Set("Content-Type", "text/plain")

	cfg := Config{
		MaxSize:      1 << 20,
		AllowedTypes: []string{"application/json"},
	}
	_, err := ReadBytes(req, cfg)
	if err == nil {
		t.Error("should reject disallowed content type")
	}
}

func TestContentLength(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("test"))
	cl := ContentLength(req)
	if cl != 4 {
		t.Errorf("ContentLength = %d", cl)
	}
}

func TestHasBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader("data"))
	if !HasBody(req) {
		t.Error("POST with body should have body")
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxSize != 1<<20 {
		t.Errorf("MaxSize = %d, want 1MB", cfg.MaxSize)
	}
}
