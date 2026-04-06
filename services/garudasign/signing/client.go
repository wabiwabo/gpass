package signing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Client is an HTTP client for the signing backend with a simple circuit breaker.
type Client struct {
	baseURL    string
	httpClient *http.Client

	mu             sync.Mutex
	failureCount   int
	maxFailures    int
	cooldown       time.Duration
	lastFailedAt   time.Time
	circuitOpen    bool
}

// NewClient creates a new signing client.
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		maxFailures: 5,
		cooldown:    30 * time.Second,
	}
}

// IssueCertificate issues a new certificate via the signing backend.
func (c *Client) IssueCertificate(ctx context.Context, req CertificateIssueRequest) (*CertificateIssueResponse, error) {
	var resp CertificateIssueResponse
	if err := c.doPost(ctx, "/certificates/issue", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// SignDocument signs a document via the signing backend.
func (c *Client) SignDocument(ctx context.Context, req SignRequest) (*SignResponse, error) {
	var resp SignResponse
	if err := c.doPost(ctx, "/sign/pades", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) doPost(ctx context.Context, path string, reqBody, respBody any) error {
	if err := c.checkCircuit(); err != nil {
		return err
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.recordFailure()
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		c.recordFailure()
		return fmt.Errorf("server error: status %d", resp.StatusCode)
	}

	c.recordSuccess()

	if resp.StatusCode >= 400 {
		var errResp struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("client error: %s - %s", errResp.Error, errResp.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

func (c *Client) checkCircuit() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.circuitOpen {
		return nil
	}

	if time.Since(c.lastFailedAt) > c.cooldown {
		c.circuitOpen = false
		c.failureCount = 0
		return nil
	}

	return fmt.Errorf("circuit breaker open: service unavailable")
}

func (c *Client) recordFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failureCount++
	c.lastFailedAt = time.Now()
	if c.failureCount >= c.maxFailures {
		c.circuitOpen = true
	}
}

func (c *Client) recordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failureCount = 0
	c.circuitOpen = false
}
