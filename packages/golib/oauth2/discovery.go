package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// DiscoveryConfig holds the OpenID Connect provider metadata.
type DiscoveryConfig struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	UserinfoEndpoint                  string   `json:"userinfo_endpoint"`
	JwksURI                           string   `json:"jwks_uri"`
	IntrospectionEndpoint             string   `json:"introspection_endpoint"`
	RevocationEndpoint                string   `json:"revocation_endpoint"`
	EndSessionEndpoint                string   `json:"end_session_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	SubjectTypesSupported             []string `json:"subject_types_supported"`
	IDTokenSigningAlgValues           []string `json:"id_token_signing_alg_values_supported"`
	TokenEndpointAuthMethods          []string `json:"token_endpoint_auth_methods_supported"`
	ClaimsSupported                   []string `json:"claims_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	RequestParameterSupported         bool     `json:"request_parameter_supported"`
	RequestURIParameterSupported      bool     `json:"request_uri_parameter_supported"`
	RequirePushedAuthorizationRequests bool    `json:"require_pushed_authorization_requests"`
	DPoPSigningAlgValues              []string `json:"dpop_signing_alg_values_supported,omitempty"`
}

// DefaultGarudaPassDiscovery returns the FAPI 2.0 compliant discovery config.
func DefaultGarudaPassDiscovery(issuerURL string) *DiscoveryConfig {
	base := strings.TrimRight(issuerURL, "/")
	return &DiscoveryConfig{
		Issuer:                            base,
		AuthorizationEndpoint:             base + "/protocol/openid-connect/auth",
		TokenEndpoint:                     base + "/protocol/openid-connect/token",
		UserinfoEndpoint:                  base + "/protocol/openid-connect/userinfo",
		JwksURI:                           base + "/protocol/openid-connect/certs",
		IntrospectionEndpoint:             base + "/protocol/openid-connect/token/introspect",
		RevocationEndpoint:                base + "/protocol/openid-connect/revoke",
		EndSessionEndpoint:                base + "/protocol/openid-connect/logout",
		ScopesSupported:                   []string{"openid", "profile", "email", "address", "phone", "offline_access"},
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:token-exchange"},
		SubjectTypesSupported:             []string{"public", "pairwise"},
		IDTokenSigningAlgValues:           []string{"PS256", "ES256"},
		TokenEndpointAuthMethods:          []string{"private_key_jwt", "tls_client_auth"},
		ClaimsSupported:                   []string{"sub", "iss", "aud", "exp", "iat", "auth_time", "nonce", "acr", "name", "email", "email_verified", "phone_number", "phone_number_verified", "address", "nik_verified"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		RequestParameterSupported:         true,
		RequestURIParameterSupported:      false,
		RequirePushedAuthorizationRequests: true,
		DPoPSigningAlgValues:              []string{"PS256", "ES256"},
	}
}

// DiscoveryHandler returns an HTTP handler for /.well-known/openid-configuration.
func DiscoveryHandler(config *DiscoveryConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(config)
	}
}

// FetchDiscovery fetches OIDC discovery from a remote issuer.
func FetchDiscovery(ctx context.Context, issuerURL string) (*DiscoveryConfig, error) {
	base := strings.TrimRight(issuerURL, "/")
	endpoint := base + "/.well-known/openid-configuration"

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("discovery request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery endpoint returned status %d", resp.StatusCode)
	}

	var config DiscoveryConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("decoding discovery response: %w", err)
	}

	return &config, nil
}
