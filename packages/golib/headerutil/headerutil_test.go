package headerutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBearerToken(t *testing.T) {
	tests := []struct {
		name string
		auth string
		want string
	}{
		{"valid", "Bearer abc123", "abc123"},
		{"valid_lowercase", "bearer xyz", "xyz"},
		{"empty", "", ""},
		{"basic", "Basic dXNlcjpwYXNz", ""},
		{"no_space", "Bearertoken", ""},
		{"extra_spaces", "Bearer   token  ", "token"},
		{"just_bearer", "Bearer ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.auth != "" {
				r.Header.Set("Authorization", tt.auth)
			}
			if got := BearerToken(r); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContentType(t *testing.T) {
	tests := []struct {
		name string
		ct   string
		want string
	}{
		{"json", "application/json", "application/json"},
		{"json_charset", "application/json; charset=utf-8", "application/json"},
		{"form", "application/x-www-form-urlencoded", "application/x-www-form-urlencoded"},
		{"multipart", "multipart/form-data; boundary=abc", "multipart/form-data"},
		{"empty", "", ""},
		{"uppercase", "Application/JSON", "application/json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.ct != "" {
				r.Header.Set("Content-Type", tt.ct)
			}
			if got := ContentType(r); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsJSON(t *testing.T) {
	tests := []struct {
		name string
		ct   string
		want bool
	}{
		{"json", "application/json", true},
		{"json_charset", "application/json; charset=utf-8", true},
		{"vnd_json", "application/vnd.api+json", true},
		{"html", "text/html", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.ct != "" {
				r.Header.Set("Content-Type", tt.ct)
			}
			if got := IsJSON(r); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAcceptsJSON(t *testing.T) {
	tests := []struct {
		name   string
		accept string
		want   bool
	}{
		{"json", "application/json", true},
		{"wildcard", "*/*", true},
		{"empty", "", true},
		{"html_only", "text/html", false},
		{"mixed", "text/html, application/json", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.accept != "" {
				r.Header.Set("Accept", tt.accept)
			}
			if got := AcceptsJSON(r); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRealIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xri        string
		remoteAddr string
		want       string
	}{
		{"xff_single", "1.2.3.4", "", "5.6.7.8:1234", "1.2.3.4"},
		{"xff_multiple", "1.2.3.4, 10.0.0.1, 192.168.1.1", "", "5.6.7.8:1234", "1.2.3.4"},
		{"xri", "", "9.8.7.6", "5.6.7.8:1234", "9.8.7.6"},
		{"remoteaddr", "", "", "5.6.7.8:1234", "5.6.7.8"},
		{"remoteaddr_no_port", "", "", "5.6.7.8", "5.6.7.8"},
		{"xff_priority", "1.1.1.1", "2.2.2.2", "3.3.3.3:80", "1.1.1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				r.Header.Set("X-Real-IP", tt.xri)
			}
			if got := RealIP(r); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRequestID(t *testing.T) {
	tests := []struct {
		name   string
		header string
		value  string
		want   string
	}{
		{"request_id", "X-Request-ID", "req-123", "req-123"},
		{"correlation_id", "X-Correlation-ID", "corr-456", "corr-456"},
		{"trace_id", "X-Trace-ID", "trace-789", "trace-789"},
		{"none", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				r.Header.Set(tt.header, tt.value)
			}
			if got := RequestID(r); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContentLength(t *testing.T) {
	tests := []struct {
		name string
		cl   string
		want int64
	}{
		{"valid", "1024", 1024},
		{"zero", "0", 0},
		{"empty", "", -1},
		{"invalid", "abc", -1},
		{"negative", "-1", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			if tt.cl != "" {
				r.Header.Set("Content-Length", tt.cl)
			}
			if got := ContentLength(r); got != tt.want {
				t.Errorf("got %d, want %d", got, tt.want)
			}
		})
	}
}

func TestSetJSON(t *testing.T) {
	w := httptest.NewRecorder()
	SetJSON(w)
	ct := w.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("got %q", ct)
	}
}

func TestSetNoCache(t *testing.T) {
	w := httptest.NewRecorder()
	SetNoCache(w)
	if w.Header().Get("Cache-Control") == "" {
		t.Error("Cache-Control not set")
	}
	if w.Header().Get("Pragma") != "no-cache" {
		t.Error("Pragma not set")
	}
	if w.Header().Get("Expires") != "0" {
		t.Error("Expires not set")
	}
}

func TestSetNoSniff(t *testing.T) {
	w := httptest.NewRecorder()
	SetNoSniff(w)
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("nosniff not set")
	}
}

func TestCopyHeaders(t *testing.T) {
	src := http.Header{}
	src.Set("X-Custom", "value1")
	src.Add("X-Multi", "a")
	src.Add("X-Multi", "b")

	dst := http.Header{}
	CopyHeaders(dst, src)

	if dst.Get("X-Custom") != "value1" {
		t.Errorf("X-Custom: got %q", dst.Get("X-Custom"))
	}
	multi := dst.Values("X-Multi")
	if len(multi) != 2 {
		t.Errorf("X-Multi values: got %d, want 2", len(multi))
	}
}

func TestRequestIDPriority(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "req")
	r.Header.Set("X-Correlation-ID", "corr")
	// X-Request-ID should take priority
	if got := RequestID(r); got != "req" {
		t.Errorf("got %q, want %q", got, "req")
	}
}
