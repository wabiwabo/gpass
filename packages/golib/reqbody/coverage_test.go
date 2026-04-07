package reqbody

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestReadBytes_AllowedTypesAccepts pins the allow-list happy path:
// a Content-Type that prefix-matches one of AllowedTypes is read through.
func TestReadBytes_AllowedTypesAccepts(t *testing.T) {
	body := strings.NewReader("hello")
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/octet-stream; charset=binary")

	got, err := ReadBytes(req, Config{
		MaxSize:      1024,
		AllowedTypes: []string{"application/octet-stream"},
	})
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("body = %q", got)
	}
}

// TestReadBytes_AllowedTypesRejects pins the deny path: no entry in
// AllowedTypes prefix-matches the request → "content type not allowed".
func TestReadBytes_AllowedTypesRejects(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("x"))
	req.Header.Set("Content-Type", "text/html")

	_, err := ReadBytes(req, Config{
		MaxSize:      1024,
		AllowedTypes: []string{"application/json", "application/cbor"},
	})
	if err == nil {
		t.Fatal("expected rejection")
	}
	if !strings.Contains(err.Error(), `content type "text/html" not allowed`) {
		t.Errorf("err = %q", err.Error())
	}
}

// TestReadBytes_OversizedBody covers the MaxBytesReader truncation branch.
// io.ReadAll returns the wrapped "http: request body too large" sentinel,
// which reqbody must rewrap into its own size-aware error.
func TestReadBytes_OversizedBody(t *testing.T) {
	// 100 bytes of body, 10-byte limit → must fail.
	body := strings.NewReader(strings.Repeat("x", 100))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	_, err := ReadBytes(req, Config{MaxSize: 10})
	if err == nil {
		t.Fatal("expected size-limit error")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size of 10 bytes") {
		t.Errorf("err = %q", err.Error())
	}
}

// TestReadBytes_DefaultMaxSize covers the cfg.MaxSize<=0 → 1MB default
// branch. We pass MaxSize=0 and a small body; success means the default
// kicked in (otherwise the body would be capped at 0 and ReadAll would
// return empty bytes plus an error).
func TestReadBytes_DefaultMaxSize(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("ok"))
	got, err := ReadBytes(req, Config{MaxSize: 0})
	if err != nil {
		t.Fatalf("ReadBytes: %v", err)
	}
	if string(got) != "ok" {
		t.Errorf("body = %q", got)
	}
}

// TestReadJSON_TrailingContentRejected covers the decoder.More() branch:
// a valid JSON object followed by extra data must be rejected so that
// callers can't smuggle a second payload past the schema check.
func TestReadJSON_TrailingContentRejected(t *testing.T) {
	body := strings.NewReader(`{"a":1} {"b":2}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	var v map[string]int
	err := ReadJSON(req, &v, Config{MaxSize: 1024})
	if err == nil {
		t.Fatal("expected trailing-content rejection")
	}
	if !strings.Contains(err.Error(), "unexpected data after JSON body") {
		t.Errorf("err = %q", err.Error())
	}
}

// TestReadJSON_DisallowUnknownFields pins the json.Decoder strict-mode
// branch. The decoder is configured with DisallowUnknownFields, so a
// payload with an extra field must be rejected — this is the second-line
// defence against schema drift / API hardening regressions.
func TestReadJSON_DisallowUnknownFields(t *testing.T) {
	type Want struct {
		Name string `json:"name"`
	}
	body := strings.NewReader(`{"name":"x","sneaky":true}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", "application/json")
	var w Want
	err := ReadJSON(req, &w, Config{MaxSize: 1024})
	if err == nil {
		t.Fatal("expected unknown-field rejection")
	}
	if !strings.Contains(err.Error(), "invalid JSON") {
		t.Errorf("err = %q", err.Error())
	}
}

// TestReadJSON_OversizedBody covers the same MaxBytesReader path through
// the JSON decoder route, which has its own error wrap.
func TestReadJSON_OversizedBody(t *testing.T) {
	big := `{"name":"` + strings.Repeat("x", 200) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	var v map[string]string
	err := ReadJSON(req, &v, Config{MaxSize: 50})
	if err == nil {
		t.Fatal("expected size-limit error")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size of 50 bytes") {
		t.Errorf("err = %q", err.Error())
	}
}

// TestHasBody_AndContentLength cover the two trivial helpers, which had
// no tests of their own.
func TestHasBody_AndContentLength(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("body"))
	if !HasBody(req) {
		t.Error("HasBody should be true for request with body")
	}
	if cl := ContentLength(req); cl != 4 {
		t.Errorf("ContentLength = %d, want 4", cl)
	}
}
