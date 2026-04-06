package sdkgen

import (
	"strings"
	"testing"
)

func TestExtractPathParams(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/api/v1/users", nil},
		{"/api/v1/users/{user_id}", []string{"user_id"}},
		{"/api/v1/users/{user_id}/certs/{cert_id}", []string{"user_id", "cert_id"}},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ExtractPathParams(tt.path)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractPathParams(%q) = %v, want %v", tt.path, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("param %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestGenerateGoSDK_SingleEndpoint(t *testing.T) {
	cfg := SDKConfig{
		PackageName: "identitysdk",
		ServiceName: "Identity",
		BaseURL:     "https://api.garudapass.id",
		Endpoints: []Endpoint{
			{
				Method:       "GET",
				Path:         "/api/v1/certificates",
				Description:  "list certificates",
				ResponseType: "CertificateList",
				AuthRequired: true,
			},
		},
	}

	code, err := GenerateGoSDK(cfg)
	if err != nil {
		t.Fatalf("GenerateGoSDK: %v", err)
	}

	if !strings.Contains(code, "IdentityClient") {
		t.Error("generated code should contain client struct")
	}
	if !strings.Contains(code, "NewIdentityClient") {
		t.Error("generated code should contain constructor")
	}
	if !strings.Contains(code, "GetCertificates") {
		t.Error("generated code should contain endpoint method")
	}
	if !strings.Contains(code, "X-API-Key") {
		t.Error("auth-required endpoint should include API key header")
	}
}

func TestGenerateGoSDK_MultipleEndpoints(t *testing.T) {
	cfg := SDKConfig{
		PackageName: "garudapasssdk",
		ServiceName: "GarudaPass",
		BaseURL:     "https://api.garudapass.id",
		Endpoints: []Endpoint{
			{
				Method:       "GET",
				Path:         "/api/v1/users/{user_id}",
				Description:  "get user by ID",
				ResponseType: "User",
				AuthRequired: true,
			},
			{
				Method:       "POST",
				Path:         "/api/v1/sign/documents",
				Description:  "sign a document",
				RequestType:  "SignRequest",
				ResponseType: "SignResponse",
				AuthRequired: true,
			},
			{
				Method:       "DELETE",
				Path:         "/api/v1/certificates/{cert_id}",
				Description:  "revoke a certificate",
				AuthRequired: true,
			},
		},
	}

	code, err := GenerateGoSDK(cfg)
	if err != nil {
		t.Fatalf("GenerateGoSDK: %v", err)
	}

	// Client struct.
	if !strings.Contains(code, "GarudaPassClient") {
		t.Error("generated code should contain GarudaPassClient struct")
	}

	// Endpoint methods.
	if !strings.Contains(code, "GetUsers") {
		t.Error("should generate GetUsers method")
	}
	if !strings.Contains(code, "CreateSignDocuments") {
		t.Error("should generate CreateSignDocuments method")
	}
	if !strings.Contains(code, "DeleteCertificates") {
		t.Error("should generate DeleteCertificates method")
	}

	// Path params.
	if !strings.Contains(code, "userId string") {
		t.Error("should have userId path param for GetUsers")
	}
	if !strings.Contains(code, "certId string") {
		t.Error("should have certId path param for DeleteCertificates")
	}

	// Auth header for all endpoints.
	if strings.Count(code, "X-API-Key") != 3 {
		t.Errorf("expected 3 X-API-Key references, got %d", strings.Count(code, "X-API-Key"))
	}
}

func TestGenerateGoSDK_NoAuth(t *testing.T) {
	cfg := SDKConfig{
		PackageName: "pubsdk",
		ServiceName: "Public",
		BaseURL:     "https://api.garudapass.id",
		Endpoints: []Endpoint{
			{
				Method:       "GET",
				Path:         "/api/v1/health",
				Description:  "health check",
				ResponseType: "HealthResponse",
				AuthRequired: false,
			},
		},
	}

	code, err := GenerateGoSDK(cfg)
	if err != nil {
		t.Fatalf("GenerateGoSDK: %v", err)
	}

	if strings.Contains(code, "X-API-Key") {
		t.Error("non-auth endpoint should not include API key header")
	}
}

func TestGenerateGoSDK_ContainsClientStruct(t *testing.T) {
	cfg := SDKConfig{
		PackageName: "testsdk",
		ServiceName: "Test",
		BaseURL:     "https://test.example.com",
		Endpoints:   []Endpoint{},
	}

	code, err := GenerateGoSDK(cfg)
	if err != nil {
		t.Fatalf("GenerateGoSDK: %v", err)
	}

	if !strings.Contains(code, "type TestClient struct") {
		t.Error("should contain client struct definition")
	}
	if !strings.Contains(code, "baseURL") {
		t.Error("client struct should have baseURL field")
	}
	if !strings.Contains(code, "httpClient") {
		t.Error("client struct should have httpClient field")
	}
}

func TestGenerateGoTypes(t *testing.T) {
	cfg := SDKConfig{
		PackageName: "garudapasssdk",
		Endpoints: []Endpoint{
			{
				Method:       "POST",
				Path:         "/api/v1/sign",
				Description:  "sign a document",
				RequestType:  "SignRequest",
				ResponseType: "SignResponse",
			},
			{
				Method:       "GET",
				Path:         "/api/v1/verify",
				Description:  "verify a signature",
				ResponseType: "VerifyResponse",
			},
		},
	}

	code, err := GenerateGoTypes(cfg)
	if err != nil {
		t.Fatalf("GenerateGoTypes: %v", err)
	}

	if !strings.Contains(code, "type SignRequest struct") {
		t.Error("should generate SignRequest type")
	}
	if !strings.Contains(code, "type SignResponse struct") {
		t.Error("should generate SignResponse type")
	}
	if !strings.Contains(code, "type VerifyResponse struct") {
		t.Error("should generate VerifyResponse type")
	}
	if !strings.Contains(code, "package garudapasssdk") {
		t.Error("should use configured package name")
	}
}

func TestGenerateGoSDK_PathParamsExtracted(t *testing.T) {
	cfg := SDKConfig{
		PackageName: "sdk",
		ServiceName: "Svc",
		BaseURL:     "http://localhost",
		Endpoints: []Endpoint{
			{
				Method:       "GET",
				Path:         "/api/v1/orgs/{org_id}/members/{member_id}",
				Description:  "get org member",
				ResponseType: "Member",
			},
		},
	}

	code, err := GenerateGoSDK(cfg)
	if err != nil {
		t.Fatalf("GenerateGoSDK: %v", err)
	}

	if !strings.Contains(code, "orgId string") {
		t.Error("should extract org_id path param")
	}
	if !strings.Contains(code, "memberId string") {
		t.Error("should extract member_id path param")
	}
	// Should use fmt.Sprintf for path.
	if !strings.Contains(code, "fmt.Sprintf") {
		t.Error("should use fmt.Sprintf for path with params")
	}
}
