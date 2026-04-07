package httpx

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLog_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	out := buf.String()
	if !strings.Contains(out, `"http_request"`) {
		t.Errorf("expected http_request log, got %s", out)
	}
	if !strings.Contains(out, `"status":201`) {
		t.Errorf("expected status 201, got %s", out)
	}
	if !strings.Contains(out, `"method":"POST"`) {
		t.Errorf("expected method POST, got %s", out)
	}
}

func TestAccessLog_SkipsProbes(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for _, path := range []string{"/health", "/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
	}
	if buf.Len() != 0 {
		t.Errorf("expected no logs for probes, got %s", buf.String())
	}
}

func TestAccessLog_LevelByStatus(t *testing.T) {
	cases := []struct {
		status   int
		wantLvl  string
	}{
		{200, `"level":"INFO"`},
		{404, `"level":"WARN"`},
		{500, `"level":"ERROR"`},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
		h := AccessLog(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
		}))
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		slog.SetDefault(prev)
		if !strings.Contains(buf.String(), tc.wantLvl) {
			t.Errorf("status %d: want %s in %s", tc.status, tc.wantLvl, buf.String())
		}
	}
}
