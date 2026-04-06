package apiver

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		input string
		want  Version
		err   bool
	}{
		{"v1", Version{1, 0}, false},
		{"v2.1", Version{2, 1}, false},
		{"1", Version{1, 0}, false},
		{"3.5", Version{3, 5}, false},
		{"V1", Version{1, 0}, false},
		{"abc", Version{}, true},
		{"v-1", Version{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.err {
				t.Errorf("Parse(%q): err=%v", tt.input, err)
			}
			if !tt.err && !got.Equal(tt.want) {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestVersion_String(t *testing.T) {
	if v := (Version{1, 0}).String(); v != "v1" {
		t.Errorf("got %q", v)
	}
	if v := (Version{2, 1}).String(); v != "v2.1" {
		t.Errorf("got %q", v)
	}
}

func TestVersion_LessThan(t *testing.T) {
	if !(Version{1, 0}).LessThan(Version{2, 0}) {
		t.Error("v1 should be less than v2")
	}
	if !(Version{1, 0}).LessThan(Version{1, 1}) {
		t.Error("v1.0 should be less than v1.1")
	}
	if (Version{2, 0}).LessThan(Version{1, 0}) {
		t.Error("v2 should not be less than v1")
	}
}

func TestVersion_Equal(t *testing.T) {
	if !(Version{1, 0}).Equal(Version{1, 0}) {
		t.Error("should be equal")
	}
	if (Version{1, 0}).Equal(Version{1, 1}) {
		t.Error("should not be equal")
	}
}

func TestExtract_Header(t *testing.T) {
	cfg := Config{
		Current: Version{2, 0},
		Source:  "header",
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API-Version", "v1")

	v, err := Extract(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Equal(Version{1, 0}) {
		t.Errorf("got %v", v)
	}
}

func TestExtract_Query(t *testing.T) {
	cfg := Config{
		Current: Version{2, 0},
		Source:  "query",
	}

	req := httptest.NewRequest(http.MethodGet, "/?version=v1.5", nil)
	v, err := Extract(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Equal(Version{1, 5}) {
		t.Errorf("got %v", v)
	}
}

func TestExtract_Path(t *testing.T) {
	cfg := Config{
		Current: Version{2, 0},
		Source:  "path",
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	v, err := Extract(req, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Equal(Version{1, 0}) {
		t.Errorf("got %v", v)
	}
}

func TestExtract_Default(t *testing.T) {
	cfg := Config{
		Current: Version{2, 0},
		Source:  "header",
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil) // No version header.
	v, _ := Extract(req, cfg)
	if !v.Equal(Version{2, 0}) {
		t.Errorf("should default to current: got %v", v)
	}
}

func TestMiddleware_ValidVersion(t *testing.T) {
	cfg := Config{
		Current: Version{2, 0},
		Minimum: Version{1, 0},
		Source:  "header",
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API-Version", "v1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d", w.Code)
	}
	if w.Header().Get("API-Version") != "v1" {
		t.Errorf("response header: got %q", w.Header().Get("API-Version"))
	}
}

func TestMiddleware_TooOld(t *testing.T) {
	cfg := Config{
		Current: Version{3, 0},
		Minimum: Version{2, 0},
		Source:  "header",
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API-Version", "v1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusGone {
		t.Errorf("status: got %d, want 410", w.Code)
	}
}

func TestMiddleware_Deprecated(t *testing.T) {
	cfg := Config{
		Current:    Version{3, 0},
		Minimum:    Version{1, 0},
		Deprecated: []Version{{1, 0}},
		Source:     "header",
	}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API-Version", "v1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("deprecated should still work: got %d", w.Code)
	}
	if w.Header().Get("Deprecation") != "true" {
		t.Error("should set Deprecation header")
	}
}

func TestMiddleware_InvalidVersion(t *testing.T) {
	cfg := Config{Current: Version{1, 0}, Source: "header"}

	handler := Middleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API-Version", "invalid")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid version: got %d", w.Code)
	}
}
