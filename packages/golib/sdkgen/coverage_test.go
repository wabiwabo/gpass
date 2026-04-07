package sdkgen

import (
	"strings"
	"testing"
)

// TestExtractPathParams_EdgeCases pins the unmatched-brace branches.
func TestExtractPathParams_EdgeCases(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"none", "/api/users", nil},
		{"single", "/api/users/{id}", []string{"id"}},
		{"multiple", "/api/{org}/users/{id}", []string{"org", "id"}},
		{"unmatched open", "/api/{broken", nil},
		{"unmatched close", "/api/broken}", nil},
	}
	for _, tc := range cases {
		got := ExtractPathParams(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("%s: len = %d, want %d (%v)", tc.name, len(got), len(tc.want), got)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got[%d] = %q, want %q", tc.name, i, got[i], tc.want[i])
			}
		}
	}
}

// TestMethodName_AllVerbsAndPrefixSkipping pins the verb→prefix mapping
// (GET→Get, POST→Create, PUT→Update, DELETE→Delete, PATCH→Patch),
// the path-segment skipping for {param}/api/v1/v2, and the unknown-verb
// fallback that returns the verb verbatim.
func TestMethodName_AllVerbsAndPrefixSkipping(t *testing.T) {
	cases := []struct {
		ep   Endpoint
		want string
	}{
		{Endpoint{Method: "GET", Path: "/api/v1/users"}, "GetUsers"},
		{Endpoint{Method: "POST", Path: "/api/v1/users"}, "CreateUsers"},
		{Endpoint{Method: "PUT", Path: "/api/v1/users/{id}"}, "UpdateUsers"},
		{Endpoint{Method: "DELETE", Path: "/api/v1/users/{id}"}, "DeleteUsers"},
		{Endpoint{Method: "PATCH", Path: "/api/v2/orders"}, "PatchOrders"},
		// Unknown method falls through to "OPTIONS" + path.
		{Endpoint{Method: "OPTIONS", Path: "/api/v1/things"}, "OPTIONSThings"},
		// Multi-segment path.
		{Endpoint{Method: "GET", Path: "/api/v1/users/{id}/profile"}, "GetUsersProfile"},
	}
	for _, tc := range cases {
		got := methodName(tc.ep)
		if got != tc.want {
			t.Errorf("%s %s = %q, want %q", tc.ep.Method, tc.ep.Path, got, tc.want)
		}
	}
}

// TestCapitalize_EmptyAndSingle covers the empty-input branch which the
// existing tests skipped.
func TestCapitalize_EmptyAndSingle(t *testing.T) {
	if got := capitalize(""); got != "" {
		t.Errorf("empty: %q", got)
	}
	if got := capitalize("a"); got != "A" {
		t.Errorf("single: %q", got)
	}
	if got := capitalize("hello"); got != "Hello" {
		t.Errorf("hello: %q", got)
	}
	// Already capitalized — must not double-capitalize.
	if got := capitalize("Hello"); got != "Hello" {
		t.Errorf("already cap: %q", got)
	}
}

// TestGenerateGoTypes_EmitsTypeStubs pins that GenerateGoTypes emits
// `type Foo struct {}` blocks for every endpoint with a RequestType
// or ResponseType. The existing tests didn't cover the type generator.
func TestGenerateGoTypes_EmitsTypeStubs(t *testing.T) {
	cfg := SDKConfig{
		PackageName: "garudaaudit",
		ServiceName: "GarudaAudit",
		BaseURL:     "http://localhost:4010",
		Endpoints: []Endpoint{
			{
				Method: "POST", Path: "/api/v1/audit/events",
				RequestType: "AuditEventRequest", ResponseType: "AuditEventResponse",
			},
		},
	}
	out, err := GenerateGoTypes(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"package garudaaudit",
		"type AuditEventRequest struct",
		"type AuditEventResponse struct",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// TestGenerateGoSDK_PopulatesPathParams pins that GenerateGoSDK auto-
// extracts path params when the Endpoint left them empty. Otherwise the
// generated client would have signatures without the {id} parameter.
func TestGenerateGoSDK_PopulatesPathParams(t *testing.T) {
	cfg := SDKConfig{
		PackageName: "client",
		ServiceName: "Test",
		BaseURL:     "http://localhost",
		Endpoints: []Endpoint{
			// PathParams intentionally left nil — GenerateGoSDK must
			// extract them from the Path string.
			{Method: "GET", Path: "/api/v1/users/{user_id}/certs/{cert_id}"},
		},
	}
	out, err := GenerateGoSDK(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Both extracted params must appear as function arguments. The
	// generator converts snake_case to camelCase, so user_id → userId.
	if !strings.Contains(out, "userId") {
		t.Errorf("userId not in output:\n%s", out)
	}
	if !strings.Contains(out, "certId") {
		t.Errorf("certId not in output:\n%s", out)
	}
	// And the path placeholders must be replaced.
	if !strings.Contains(out, "/api/v1/users/%s/certs/%s") {
		t.Errorf("path format string not in output:\n%s", out)
	}
}
