package multipart

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestParse_DefaultsAppliedWhenZero pins the cfg.MaxTotalSize<=0 → 50MB
// fallback branch in Parse.
func TestParse_DefaultsAppliedWhenZero(t *testing.T) {
	req := buildMultipart(t, "doc",
		[]fileSpec{{filename: "tiny.pdf", contentType: "application/pdf", body: []byte("x")}},
		nil,
	)
	// MaxTotalSize zero must be coerced to 50MB; the rest of the config
	// is permissive (no allowlists, generous file size).
	cfg := Config{MaxFileSize: 1024}
	res, err := Parse(req, "doc", cfg)
	if err != nil {
		t.Fatalf("Parse with zero cfg: %v", err)
	}
	if len(res.Files) != 1 {
		t.Errorf("got %d files", len(res.Files))
	}
}

// TestParse_BadFormBody pins the r.ParseMultipartForm error path.
func TestParse_BadFormBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("not multipart at all")))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=zzz")
	_, err := Parse(req, "doc", DefaultConfig())
	if err == nil || !strings.Contains(err.Error(), "parse form") {
		t.Errorf("err = %v, want parse-form error", err)
	}
}

// TestProcessFile_OnlyExtensionRejected pins that processFile rejects on
// extension before reading the body, regardless of content-type allow.
func TestProcessFile_OnlyExtensionRejected(t *testing.T) {
	req := buildMultipart(t, "doc",
		[]fileSpec{{filename: "x.bin", contentType: "application/pdf", body: []byte("ok")}},
		nil,
	)
	_, err := Parse(req, "doc", DefaultConfig())
	if err == nil || !strings.Contains(err.Error(), "disallowed extension") {
		t.Errorf("err = %v", err)
	}
}

// TestProcessFile_AllowAnyContentTypeWhenAllowlistEmpty pins the branch
// where AllowedTypes is nil → any content type passes.
func TestProcessFile_AllowAnyContentTypeWhenAllowlistEmpty(t *testing.T) {
	req := buildMultipart(t, "doc",
		[]fileSpec{{filename: "x.pdf", contentType: "weird/thing", body: []byte("ok")}},
		nil,
	)
	cfg := DefaultConfig()
	cfg.AllowedTypes = nil
	res, err := Parse(req, "doc", cfg)
	if err != nil || len(res.Files) != 1 {
		t.Errorf("err=%v files=%d", err, len(res.Files))
	}
}
