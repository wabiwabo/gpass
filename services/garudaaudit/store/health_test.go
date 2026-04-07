package store

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadinessHandler_NilDB(t *testing.T) {
	h := ReadinessHandler(nil, "test-svc")
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()
	h(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"db":"in-memory"`) {
		t.Errorf("body = %s, want in-memory marker", body)
	}
	if !strings.Contains(body, `"service":"test-svc"`) {
		t.Errorf("body = %s, want service name", body)
	}
}
