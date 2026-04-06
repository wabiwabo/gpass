package propagation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtract_AllHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderUserID, "user-123")
	req.Header.Set(HeaderRequestID, "req-456")
	req.Header.Set(HeaderCorrelationID, "corr-789")
	req.Header.Set(HeaderTraceParent, "00-trace-span-01")
	req.Header.Set(HeaderAuthMethod, "oauth2")
	req.Header.Set(HeaderAuthSubject, "sub-abc")
	req.Header.Set(HeaderServiceName, "bff")
	req.Header.Set(HeaderSessionID, "sess-xyz")

	sc := Extract(req)

	if sc.UserID != "user-123" {
		t.Errorf("UserID: got %q, want %q", sc.UserID, "user-123")
	}
	if sc.RequestID != "req-456" {
		t.Errorf("RequestID: got %q, want %q", sc.RequestID, "req-456")
	}
	if sc.CorrelationID != "corr-789" {
		t.Errorf("CorrelationID: got %q, want %q", sc.CorrelationID, "corr-789")
	}
	if sc.TraceParent != "00-trace-span-01" {
		t.Errorf("TraceParent: got %q, want %q", sc.TraceParent, "00-trace-span-01")
	}
	if sc.AuthMethod != "oauth2" {
		t.Errorf("AuthMethod: got %q, want %q", sc.AuthMethod, "oauth2")
	}
	if sc.AuthSubject != "sub-abc" {
		t.Errorf("AuthSubject: got %q, want %q", sc.AuthSubject, "sub-abc")
	}
	if sc.SourceService != "bff" {
		t.Errorf("SourceService: got %q, want %q", sc.SourceService, "bff")
	}
	if sc.SessionID != "sess-xyz" {
		t.Errorf("SessionID: got %q, want %q", sc.SessionID, "sess-xyz")
	}
}

func TestInject_SetsAllHeaders(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	sc := ServiceContext{
		UserID:        "u1",
		RequestID:     "r1",
		CorrelationID: "c1",
		TraceParent:   "t1",
		AuthMethod:    "jwt",
		AuthSubject:   "s1",
		SourceService: "identity",
		SessionID:     "sess1",
	}

	Inject(req, sc)

	checks := []struct {
		header string
		want   string
	}{
		{HeaderUserID, "u1"},
		{HeaderRequestID, "r1"},
		{HeaderCorrelationID, "c1"},
		{HeaderTraceParent, "t1"},
		{HeaderAuthMethod, "jwt"},
		{HeaderAuthSubject, "s1"},
		{HeaderServiceName, "identity"},
		{HeaderSessionID, "sess1"},
	}
	for _, c := range checks {
		got := req.Header.Get(c.header)
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.header, got, c.want)
		}
	}
}

func TestMiddleware_StoresInContext(t *testing.T) {
	var captured ServiceContext

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	handler := Middleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderUserID, "uid-test")
	req.Header.Set(HeaderRequestID, "rid-test")
	req.Header.Set(HeaderCorrelationID, "cid-test")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if captured.UserID != "uid-test" {
		t.Errorf("UserID from context: got %q, want %q", captured.UserID, "uid-test")
	}
	if captured.RequestID != "rid-test" {
		t.Errorf("RequestID from context: got %q, want %q", captured.RequestID, "rid-test")
	}
	if captured.CorrelationID != "cid-test" {
		t.Errorf("CorrelationID from context: got %q, want %q", captured.CorrelationID, "cid-test")
	}
}

func TestFromContext_NoValue(t *testing.T) {
	sc := FromContext(context.Background())

	if sc.UserID != "" || sc.RequestID != "" || sc.SessionID != "" {
		t.Error("expected empty ServiceContext from context without value")
	}
}

func TestPropagatingClient_InjectsHeaders(t *testing.T) {
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sc := ServiceContext{
		UserID:    "u-prop",
		RequestID: "r-prop",
		SessionID: "s-prop",
	}
	ctx := ToContext(context.Background(), sc)

	client := NewPropagatingClient(server.Client(), "my-service")

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(ctx, req)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	defer resp.Body.Close()

	if got := receivedHeaders.Get(HeaderUserID); got != "u-prop" {
		t.Errorf("UserID header: got %q, want %q", got, "u-prop")
	}
	if got := receivedHeaders.Get(HeaderRequestID); got != "r-prop" {
		t.Errorf("RequestID header: got %q, want %q", got, "r-prop")
	}
	if got := receivedHeaders.Get(HeaderSessionID); got != "s-prop" {
		t.Errorf("SessionID header: got %q, want %q", got, "s-prop")
	}
	if got := receivedHeaders.Get(HeaderServiceName); got != "my-service" {
		t.Errorf("SourceService header: got %q, want %q", got, "my-service")
	}
}

func TestMissingHeaders_EmptyStrings(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No propagation headers set at all
	sc := Extract(req)

	if sc.UserID != "" {
		t.Errorf("expected empty UserID, got %q", sc.UserID)
	}
	if sc.RequestID != "" {
		t.Errorf("expected empty RequestID, got %q", sc.RequestID)
	}
	if sc.CorrelationID != "" {
		t.Errorf("expected empty CorrelationID, got %q", sc.CorrelationID)
	}
	if sc.TraceParent != "" {
		t.Errorf("expected empty TraceParent, got %q", sc.TraceParent)
	}
	if sc.AuthMethod != "" {
		t.Errorf("expected empty AuthMethod, got %q", sc.AuthMethod)
	}
	if sc.AuthSubject != "" {
		t.Errorf("expected empty AuthSubject, got %q", sc.AuthSubject)
	}
	if sc.SourceService != "" {
		t.Errorf("expected empty SourceService, got %q", sc.SourceService)
	}
	if sc.SessionID != "" {
		t.Errorf("expected empty SessionID, got %q", sc.SessionID)
	}
}

func TestRoundTrip_InjectExtract(t *testing.T) {
	original := ServiceContext{
		UserID:        "round-user",
		RequestID:     "round-req",
		CorrelationID: "round-corr",
		TraceParent:   "00-abc-def-01",
		AuthMethod:    "pkce",
		AuthSubject:   "round-sub",
		SourceService: "round-svc",
		SessionID:     "round-sess",
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	Inject(req, original)
	extracted := Extract(req)

	if extracted != original {
		t.Errorf("round-trip mismatch:\ngot:  %+v\nwant: %+v", extracted, original)
	}
}
