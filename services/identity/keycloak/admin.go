package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Credential represents a Keycloak user credential.
type Credential struct {
	Type      string `json:"type"`
	Value     string `json:"value"`
	Temporary bool   `json:"temporary"`
}

// CreateUserRequest is the payload for creating a Keycloak user.
type CreateUserRequest struct {
	Username    string            `json:"username"`
	Email       string            `json:"email,omitempty"`
	FirstName   string            `json:"firstName,omitempty"`
	LastName    string            `json:"lastName,omitempty"`
	Enabled     bool              `json:"enabled"`
	Credentials []Credential      `json:"credentials,omitempty"`
	Attributes  map[string][]string `json:"attributes,omitempty"`
}

// AdminClient communicates with the Keycloak Admin REST API.
type AdminClient struct {
	baseURL  string
	username string
	password string
	realm    string

	mu         sync.Mutex
	token      string
	tokenExpiry time.Time
	httpClient *http.Client
}

// NewAdminClient creates a new Keycloak admin client.
func NewAdminClient(baseURL, username, password, realm string) *AdminClient {
	return &AdminClient{
		baseURL:    baseURL,
		username:   username,
		password:   password,
		realm:      realm,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// CreateUser creates a new user in the configured realm and returns the user ID.
func (ac *AdminClient) CreateUser(ctx context.Context, req CreateUserRequest) (string, error) {
	token, err := ac.getToken(ctx)
	if err != nil {
		return "", fmt.Errorf("get admin token: %w", err)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal user: %w", err)
	}

	endpoint := fmt.Sprintf("%s/admin/realms/%s/users", ac.baseURL, ac.realm)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+token)

	resp, err := ac.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("keycloak request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return "", fmt.Errorf("user already exists")
	}
	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keycloak returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Extract user ID from Location header
	location := resp.Header.Get("Location")
	if location == "" {
		return "", fmt.Errorf("no Location header in response")
	}
	// Location format: .../users/{id}
	parsed, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("parse location: %w", err)
	}
	segments := splitPath(parsed.Path)
	if len(segments) == 0 {
		return "", fmt.Errorf("empty path in Location header")
	}
	return segments[len(segments)-1], nil
}

// getToken returns a cached admin token or fetches a new one.
func (ac *AdminClient) getToken(ctx context.Context) (string, error) {
	ac.mu.Lock()
	defer ac.mu.Unlock()

	// Return cached token if still valid (with 30s buffer)
	if ac.token != "" && time.Now().Before(ac.tokenExpiry) {
		return ac.token, nil
	}

	// Fetch new token from master realm
	tokenURL := fmt.Sprintf("%s/realms/master/protocol/openid-connect/token", ac.baseURL)
	data := url.Values{
		"grant_type": {"password"},
		"client_id":  {"admin-cli"},
		"username":   {ac.username},
		"password":   {ac.password},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader([]byte(data.Encode())))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := ac.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	ac.token = tokenResp.AccessToken
	ac.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn)*time.Second - 30*time.Second)

	return ac.token, nil
}

func splitPath(path string) []string {
	var segments []string
	for _, s := range bytes.Split([]byte(path), []byte("/")) {
		if len(s) > 0 {
			segments = append(segments, string(s))
		}
	}
	return segments
}
