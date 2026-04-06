package oauth2

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Token type URNs per RFC 8693.
const (
	TokenTypeAccessToken  = "urn:ietf:params:oauth:token-type:access_token"
	TokenTypeRefreshToken = "urn:ietf:params:oauth:token-type:refresh_token"
	TokenTypeIDToken      = "urn:ietf:params:oauth:token-type:id_token"
	TokenTypeJWT          = "urn:ietf:params:oauth:token-type:jwt"

	GrantTypeTokenExchange = "urn:ietf:params:oauth:grant-type:token-exchange"
)

// TokenExchangeRequest per RFC 8693.
type TokenExchangeRequest struct {
	GrantType          string `json:"grant_type"`
	SubjectToken       string `json:"subject_token"`
	SubjectTokenType   string `json:"subject_token_type"`
	RequestedTokenType string `json:"requested_token_type,omitempty"`
	Audience           string `json:"audience,omitempty"`
	Scope              string `json:"scope,omitempty"`
	Resource           string `json:"resource,omitempty"`
}

// TokenExchangeResponse per RFC 8693.
type TokenExchangeResponse struct {
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int    `json:"expires_in"`
	Scope           string `json:"scope,omitempty"`
}

// TokenExchanger handles token exchange operations.
type TokenExchanger struct {
	endpoint     string
	clientID     string
	clientSecret string
	httpClient   *http.Client
}

// NewTokenExchanger creates a new token exchanger.
func NewTokenExchanger(endpoint, clientID, clientSecret string) *TokenExchanger {
	return &TokenExchanger{
		endpoint:     endpoint,
		clientID:     clientID,
		clientSecret: clientSecret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Exchange performs an OAuth 2.0 token exchange per RFC 8693.
func (e *TokenExchanger) Exchange(ctx context.Context, req TokenExchangeRequest) (*TokenExchangeResponse, error) {
	form := url.Values{}
	form.Set("grant_type", GrantTypeTokenExchange)
	form.Set("subject_token", req.SubjectToken)
	form.Set("subject_token_type", req.SubjectTokenType)

	if req.RequestedTokenType != "" {
		form.Set("requested_token_type", req.RequestedTokenType)
	}
	if req.Audience != "" {
		form.Set("audience", req.Audience)
	}
	if req.Scope != "" {
		form.Set("scope", req.Scope)
	}
	if req.Resource != "" {
		form.Set("resource", req.Resource)
	}

	body := form.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating token exchange request: %w", err)
	}

	httpReq.SetBasicAuth(e.clientID, e.clientSecret)
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Body = io.NopCloser(strings.NewReader(body))
	httpReq.ContentLength = int64(len(body))

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var exchangeResp TokenExchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&exchangeResp); err != nil {
		return nil, fmt.Errorf("decoding token exchange response: %w", err)
	}

	return &exchangeResp, nil
}

// MockTokenExchanger for testing.
type MockTokenExchanger struct {
	Exchanges map[string]*TokenExchangeResponse
}

// NewMockTokenExchanger creates a new mock token exchanger.
func NewMockTokenExchanger() *MockTokenExchanger {
	return &MockTokenExchanger{
		Exchanges: make(map[string]*TokenExchangeResponse),
	}
}

// Exchange returns the configured response for the subject token.
func (m *MockTokenExchanger) Exchange(_ context.Context, req TokenExchangeRequest) (*TokenExchangeResponse, error) {
	resp, ok := m.Exchanges[req.SubjectToken]
	if !ok {
		return nil, fmt.Errorf("no exchange configured for subject token %q", req.SubjectToken)
	}
	return resp, nil
}

// AddExchange registers an exchange response for a given subject token.
func (m *MockTokenExchanger) AddExchange(subjectToken string, resp *TokenExchangeResponse) {
	m.Exchanges[subjectToken] = resp
}
