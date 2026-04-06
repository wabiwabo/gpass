package middleware

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	})
}

func computeWeakETag(body string) string {
	hash := sha256.Sum256([]byte(body))
	return fmt.Sprintf(`W/"%x"`, hash)
}

func computeStrongETag(body string) string {
	hash := sha256.Sum256([]byte(body))
	return fmt.Sprintf(`"%x"`, hash)
}

func TestETag_ResponseIncludesETagHeader(t *testing.T) {
	handler := ETag(testHandler(`{"ok":true}`))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	etag := w.Header().Get("ETag")
	if etag == "" {
		t.Fatal("response should include ETag header")
	}
	if !strings.HasPrefix(etag, `W/"`) {
		t.Errorf("weak ETag should start with W/\", got %s", etag)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != `{"ok":true}` {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestETag_ConditionalMatchReturns304(t *testing.T) {
	body := `{"data":"test"}`
	handler := ETag(testHandler(body))
	etag := computeWeakETag(body)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", etag)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("expected 304, got %d", w.Code)
	}
	if w.Body.Len() != 0 {
		t.Errorf("304 response should have no body, got %d bytes", w.Body.Len())
	}
	if w.Header().Get("ETag") != etag {
		t.Errorf("ETag header should still be set on 304")
	}
}

func TestETag_ConditionalNonMatchReturns200(t *testing.T) {
	body := `{"data":"test"}`
	handler := ETag(testHandler(body))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", `W/"stale-hash"`)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != body {
		t.Errorf("expected body %s, got %s", body, w.Body.String())
	}
}

func TestETag_POSTDoesNotGetETag(t *testing.T) {
	handler := ETag(testHandler(`{"created":true}`))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("ETag") != "" {
		t.Error("POST requests should not get ETag headers")
	}
}

func TestETag_WeakFormat(t *testing.T) {
	body := `hello`
	handler := ETag(testHandler(body))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	etag := w.Header().Get("ETag")
	expected := computeWeakETag(body)
	if etag != expected {
		t.Errorf("expected ETag %s, got %s", expected, etag)
	}
}

func TestStrongETag_Format(t *testing.T) {
	body := `static content`
	handler := StrongETag(testHandler(body))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	etag := w.Header().Get("ETag")
	expected := computeStrongETag(body)
	if etag != expected {
		t.Errorf("expected ETag %s, got %s", expected, etag)
	}
	if strings.HasPrefix(etag, "W/") {
		t.Error("strong ETag should not have W/ prefix")
	}
}

func TestStrongETag_ConditionalMatchReturns304(t *testing.T) {
	body := `static content`
	handler := StrongETag(testHandler(body))
	etag := computeStrongETag(body)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", etag)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("expected 304, got %d", w.Code)
	}
}

func TestETag_MultipleIfNoneMatchValues(t *testing.T) {
	body := `{"multi":true}`
	handler := ETag(testHandler(body))
	etag := computeWeakETag(body)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", `W/"old-hash", `+etag+`, W/"other-hash"`)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("expected 304 when ETag is among multiple If-None-Match values, got %d", w.Code)
	}
}

func TestETag_WildcardIfNoneMatchAlwaysMatches(t *testing.T) {
	handler := ETag(testHandler(`{"any":"content"}`))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("If-None-Match", "*")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotModified {
		t.Errorf("expected 304 for wildcard If-None-Match, got %d", w.Code)
	}
}

func TestETag_HEADRequestGetsETag(t *testing.T) {
	handler := ETag(testHandler(`{"head":true}`))

	req := httptest.NewRequest(http.MethodHead, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("ETag") == "" {
		t.Error("HEAD requests should get ETag headers")
	}
}

func TestETag_PUTDoesNotGetETag(t *testing.T) {
	handler := ETag(testHandler(`{"updated":true}`))

	req := httptest.NewRequest(http.MethodPut, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("ETag") != "" {
		t.Error("PUT requests should not get ETag headers")
	}
}

func TestETag_DELETEDoesNotGetETag(t *testing.T) {
	handler := ETag(testHandler(`{"deleted":true}`))

	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("ETag") != "" {
		t.Error("DELETE requests should not get ETag headers")
	}
}
