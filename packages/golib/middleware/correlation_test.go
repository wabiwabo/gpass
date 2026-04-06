package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorrelation_GeneratesRequestIDIfMissing(t *testing.T) {
	var ids CorrelationIDs
	handler := Correlation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = GetCorrelation(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if ids.RequestID == "" {
		t.Error("expected RequestID to be generated")
	}
	if respID := w.Header().Get("X-Request-Id"); respID == "" {
		t.Error("expected X-Request-Id in response header")
	}
}

func TestCorrelation_GeneratesCorrelationIDIfMissing(t *testing.T) {
	var ids CorrelationIDs
	handler := Correlation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = GetCorrelation(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if ids.CorrelationID == "" {
		t.Error("expected CorrelationID to be generated")
	}
	if respID := w.Header().Get("X-Correlation-Id"); respID == "" {
		t.Error("expected X-Correlation-Id in response header")
	}
}

func TestCorrelation_PreservesExistingIDs(t *testing.T) {
	existingReqID := "req-123"
	existingCorID := "cor-456"

	var ids CorrelationIDs
	handler := Correlation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = GetCorrelation(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-Id", existingReqID)
	r.Header.Set("X-Correlation-Id", existingCorID)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if ids.RequestID != existingReqID {
		t.Errorf("expected RequestID %q, got %q", existingReqID, ids.RequestID)
	}
	if ids.CorrelationID != existingCorID {
		t.Errorf("expected CorrelationID %q, got %q", existingCorID, ids.CorrelationID)
	}
}

func TestCorrelation_SetsAllIDsOnResponse(t *testing.T) {
	handler := Correlation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-Id", "req-1")
	r.Header.Set("X-Correlation-Id", "cor-1")
	r.Header.Set("X-Trace-Parent", "00-trace-span-01")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if v := w.Header().Get("X-Request-Id"); v != "req-1" {
		t.Errorf("expected X-Request-Id req-1, got %q", v)
	}
	if v := w.Header().Get("X-Correlation-Id"); v != "cor-1" {
		t.Errorf("expected X-Correlation-Id cor-1, got %q", v)
	}
	if v := w.Header().Get("X-Trace-Parent"); v != "00-trace-span-01" {
		t.Errorf("expected X-Trace-Parent 00-trace-span-01, got %q", v)
	}
}

func TestCorrelation_AvailableInContext(t *testing.T) {
	var ids CorrelationIDs
	handler := Correlation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = GetCorrelation(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-Id", "req-ctx")
	r.Header.Set("X-Correlation-Id", "cor-ctx")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if ids.RequestID != "req-ctx" {
		t.Errorf("expected req-ctx, got %q", ids.RequestID)
	}
	if ids.CorrelationID != "cor-ctx" {
		t.Errorf("expected cor-ctx, got %q", ids.CorrelationID)
	}
}

func TestPropagateHeaders_SetsAllHeaders(t *testing.T) {
	ids := CorrelationIDs{
		RequestID:     "req-prop",
		CorrelationID: "cor-prop",
		TraceParent:   "00-trace-prop-01",
	}

	req := httptest.NewRequest(http.MethodGet, "/downstream", nil)
	PropagateHeaders(req, ids)

	if v := req.Header.Get("X-Request-Id"); v != "req-prop" {
		t.Errorf("expected X-Request-Id req-prop, got %q", v)
	}
	if v := req.Header.Get("X-Correlation-Id"); v != "cor-prop" {
		t.Errorf("expected X-Correlation-Id cor-prop, got %q", v)
	}
	if v := req.Header.Get("X-Trace-Parent"); v != "00-trace-prop-01" {
		t.Errorf("expected X-Trace-Parent 00-trace-prop-01, got %q", v)
	}
}

func TestPropagateHeaders_SkipsEmptyValues(t *testing.T) {
	ids := CorrelationIDs{
		RequestID: "req-only",
	}

	req := httptest.NewRequest(http.MethodGet, "/downstream", nil)
	PropagateHeaders(req, ids)

	if v := req.Header.Get("X-Request-Id"); v != "req-only" {
		t.Errorf("expected req-only, got %q", v)
	}
	if v := req.Header.Get("X-Correlation-Id"); v != "" {
		t.Errorf("expected empty X-Correlation-Id, got %q", v)
	}
	if v := req.Header.Get("X-Trace-Parent"); v != "" {
		t.Errorf("expected empty X-Trace-Parent, got %q", v)
	}
}

func TestCorrelation_TraceParentForwardedWhenPresent(t *testing.T) {
	var ids CorrelationIDs
	handler := Correlation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = GetCorrelation(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	traceParent := "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Trace-Parent", traceParent)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if ids.TraceParent != traceParent {
		t.Errorf("expected TraceParent %q, got %q", traceParent, ids.TraceParent)
	}
	if v := w.Header().Get("X-Trace-Parent"); v != traceParent {
		t.Errorf("expected X-Trace-Parent in response, got %q", v)
	}
}

func TestCorrelation_TraceParentNotSetWhenMissing(t *testing.T) {
	handler := Correlation(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if v := w.Header().Get("X-Trace-Parent"); v != "" {
		t.Errorf("expected no X-Trace-Parent when not provided, got %q", v)
	}
}

func TestGetCorrelation_NilContext(t *testing.T) {
	ids := GetCorrelation(nil)
	if ids.RequestID != "" || ids.CorrelationID != "" || ids.TraceParent != "" {
		t.Error("expected empty CorrelationIDs for nil context")
	}
}
