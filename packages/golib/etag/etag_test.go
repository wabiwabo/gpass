package etag

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGenerate(t *testing.T) {
	tag := Generate([]byte("hello world"))
	if tag == "" {
		t.Error("should generate non-empty ETag")
	}
	if tag[0:2] != `W/` {
		t.Error("should be weak ETag")
	}

	// Same content = same ETag.
	tag2 := Generate([]byte("hello world"))
	if tag != tag2 {
		t.Error("same content should produce same ETag")
	}

	// Different content = different ETag.
	tag3 := Generate([]byte("different"))
	if tag == tag3 {
		t.Error("different content should produce different ETag")
	}
}

func TestGenerateStrong(t *testing.T) {
	tag := GenerateStrong([]byte("data"))
	if tag[0] != '"' {
		t.Error("strong ETag should start with quote")
	}
	if tag[0:2] == `W/` {
		t.Error("strong ETag should not have W/ prefix")
	}
}

func TestMatch(t *testing.T) {
	etag := `W/"abc123"`
	if !Match(`W/"abc123"`, etag) {
		t.Error("exact match should work")
	}
	if !Match(`"abc123"`, etag) {
		t.Error("weak comparison should ignore W/ prefix")
	}
	if Match(`W/"different"`, etag) {
		t.Error("different ETags should not match")
	}
	if Match("", etag) {
		t.Error("empty should not match")
	}
	if !Match("*", etag) {
		t.Error("wildcard should match everything")
	}
}

func TestMatch_CommaSeparated(t *testing.T) {
	if !Match(`W/"a", W/"b", W/"c"`, `W/"b"`) {
		t.Error("should match in comma-separated list")
	}
}

func TestNotModified_ETag(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", `W/"abc"`)

	if !NotModified(req, `W/"abc"`, time.Time{}) {
		t.Error("matching ETag should be not modified")
	}

	if NotModified(req, `W/"xyz"`, time.Time{}) {
		t.Error("different ETag should not match")
	}
}

func TestNotModified_LastModified(t *testing.T) {
	lastMod := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-Modified-Since", lastMod.Format(http.TimeFormat))

	if !NotModified(req, "", lastMod) {
		t.Error("same time should be not modified")
	}

	newer := lastMod.Add(1 * time.Hour)
	if NotModified(req, "", newer) {
		t.Error("newer content should be modified")
	}
}

func TestSetHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	lastMod := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
	SetHeaders(w, `W/"abc"`, lastMod)

	if w.Header().Get("ETag") != `W/"abc"` {
		t.Errorf("ETag: got %q", w.Header().Get("ETag"))
	}
	if w.Header().Get("Last-Modified") == "" {
		t.Error("should set Last-Modified")
	}
}

func TestMiddleware_304(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `W/"test123"`)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("body"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", `W/"test123"`)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("should be 304: got %d", w.Code)
	}
}

func TestMiddleware_200(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `W/"new"`)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("body"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", `W/"old"`)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("should be 200: got %d", w.Code)
	}
}

func TestMiddleware_SkipsPOST(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("POST should pass through: got %d", w.Code)
	}
}
