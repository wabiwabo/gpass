package reqctx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUserID(t *testing.T) {
	ctx := context.Background()
	if got := UserID(ctx); got != "" {
		t.Errorf("empty ctx: UserID = %q, want empty", got)
	}

	ctx = SetUserID(ctx, "user-123")
	if got := UserID(ctx); got != "user-123" {
		t.Errorf("UserID = %q, want user-123", got)
	}
}

func TestTenantID(t *testing.T) {
	ctx := context.Background()
	if got := TenantID(ctx); got != "" {
		t.Errorf("empty ctx: TenantID = %q, want empty", got)
	}

	ctx = SetTenantID(ctx, "tenant-abc")
	if got := TenantID(ctx); got != "tenant-abc" {
		t.Errorf("TenantID = %q, want tenant-abc", got)
	}
}

func TestRequestID(t *testing.T) {
	ctx := context.Background()
	if got := RequestID(ctx); got != "" {
		t.Errorf("empty ctx: RequestID = %q, want empty", got)
	}

	ctx = SetRequestID(ctx, "req-456")
	if got := RequestID(ctx); got != "req-456" {
		t.Errorf("RequestID = %q, want req-456", got)
	}
}

func TestCorrelationID(t *testing.T) {
	ctx := context.Background()
	if got := CorrelationID(ctx); got != "" {
		t.Errorf("empty ctx: CorrelationID = %q, want empty", got)
	}

	ctx = SetCorrelationID(ctx, "corr-789")
	if got := CorrelationID(ctx); got != "corr-789" {
		t.Errorf("CorrelationID = %q, want corr-789", got)
	}
}

func TestSessionID(t *testing.T) {
	ctx := context.Background()
	if got := SessionID(ctx); got != "" {
		t.Errorf("empty ctx: SessionID = %q, want empty", got)
	}

	ctx = SetSessionID(ctx, "sess-xyz")
	if got := SessionID(ctx); got != "sess-xyz" {
		t.Errorf("SessionID = %q, want sess-xyz", got)
	}
}

func TestRoles(t *testing.T) {
	ctx := context.Background()
	if got := Roles(ctx); got != nil {
		t.Errorf("empty ctx: Roles = %v, want nil", got)
	}

	roles := []string{"admin", "user", "auditor"}
	ctx = SetRoles(ctx, roles)
	got := Roles(ctx)
	if len(got) != 3 {
		t.Fatalf("Roles len = %d, want 3", len(got))
	}
	for i, want := range roles {
		if got[i] != want {
			t.Errorf("Roles[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestHasRole(t *testing.T) {
	ctx := context.Background()

	// No roles set
	if HasRole(ctx, "admin") {
		t.Error("should return false when no roles set")
	}

	ctx = SetRoles(ctx, []string{"user", "auditor"})

	tests := []struct {
		role string
		want bool
	}{
		{"user", true},
		{"auditor", true},
		{"admin", false},
		{"", false},
		{"USER", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			if got := HasRole(ctx, tt.role); got != tt.want {
				t.Errorf("HasRole(%q) = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	ctx := context.Background()
	if got := ClientIP(ctx); got != "" {
		t.Errorf("empty ctx: ClientIP = %q, want empty", got)
	}

	ctx = SetClientIP(ctx, "203.0.113.50")
	if got := ClientIP(ctx); got != "203.0.113.50" {
		t.Errorf("ClientIP = %q, want 203.0.113.50", got)
	}
}

func TestEnrichFromRequest_AllHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user-100")
	req.Header.Set("X-Tenant-ID", "tenant-200")
	req.Header.Set("X-Request-ID", "req-300")
	req.Header.Set("X-Correlation-ID", "corr-400")

	enriched := EnrichFromRequest(req)
	ctx := enriched.Context()

	if got := UserID(ctx); got != "user-100" {
		t.Errorf("UserID = %q, want user-100", got)
	}
	if got := TenantID(ctx); got != "tenant-200" {
		t.Errorf("TenantID = %q, want tenant-200", got)
	}
	if got := RequestID(ctx); got != "req-300" {
		t.Errorf("RequestID = %q, want req-300", got)
	}
	if got := CorrelationID(ctx); got != "corr-400" {
		t.Errorf("CorrelationID = %q, want corr-400", got)
	}
}

func TestEnrichFromRequest_PartialHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user-only")

	enriched := EnrichFromRequest(req)
	ctx := enriched.Context()

	if got := UserID(ctx); got != "user-only" {
		t.Errorf("UserID = %q, want user-only", got)
	}
	if got := TenantID(ctx); got != "" {
		t.Errorf("TenantID should be empty, got %q", got)
	}
	if got := RequestID(ctx); got != "" {
		t.Errorf("RequestID should be empty, got %q", got)
	}
	if got := CorrelationID(ctx); got != "" {
		t.Errorf("CorrelationID should be empty, got %q", got)
	}
}

func TestEnrichFromRequest_NoHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/test", nil)
	enriched := EnrichFromRequest(req)
	ctx := enriched.Context()

	if got := UserID(ctx); got != "" {
		t.Errorf("UserID should be empty, got %q", got)
	}
	if got := TenantID(ctx); got != "" {
		t.Errorf("TenantID should be empty, got %q", got)
	}
}

func TestMiddleware(t *testing.T) {
	var capturedUserID, capturedTenantID string

	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID = UserID(r.Context())
		capturedTenantID = TenantID(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-User-ID", "mw-user")
	req.Header.Set("X-Tenant-ID", "mw-tenant")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedUserID != "mw-user" {
		t.Errorf("middleware UserID = %q, want mw-user", capturedUserID)
	}
	if capturedTenantID != "mw-tenant" {
		t.Errorf("middleware TenantID = %q, want mw-tenant", capturedTenantID)
	}
	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestContextKeys_NoCollision(t *testing.T) {
	ctx := context.Background()
	ctx = SetUserID(ctx, "user")
	ctx = SetTenantID(ctx, "tenant")
	ctx = SetRequestID(ctx, "request")
	ctx = SetCorrelationID(ctx, "correlation")
	ctx = SetSessionID(ctx, "session")
	ctx = SetClientIP(ctx, "127.0.0.1")
	ctx = SetRoles(ctx, []string{"admin"})

	// All values should be independently retrievable
	if UserID(ctx) != "user" {
		t.Error("UserID collision")
	}
	if TenantID(ctx) != "tenant" {
		t.Error("TenantID collision")
	}
	if RequestID(ctx) != "request" {
		t.Error("RequestID collision")
	}
	if CorrelationID(ctx) != "correlation" {
		t.Error("CorrelationID collision")
	}
	if SessionID(ctx) != "session" {
		t.Error("SessionID collision")
	}
	if ClientIP(ctx) != "127.0.0.1" {
		t.Error("ClientIP collision")
	}
	if !HasRole(ctx, "admin") {
		t.Error("Roles collision")
	}
}

func TestOverwrite_Values(t *testing.T) {
	ctx := context.Background()
	ctx = SetUserID(ctx, "first")
	ctx = SetUserID(ctx, "second")

	if got := UserID(ctx); got != "second" {
		t.Errorf("overwritten UserID = %q, want second", got)
	}
}

func TestEmptyRoles(t *testing.T) {
	ctx := SetRoles(context.Background(), []string{})
	roles := Roles(ctx)
	if len(roles) != 0 {
		t.Errorf("empty roles len = %d, want 0", len(roles))
	}
	if HasRole(ctx, "anything") {
		t.Error("HasRole should return false for empty roles")
	}
}
