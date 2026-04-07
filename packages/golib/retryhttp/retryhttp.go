// Package retryhttp provides HTTP client retry with exponential backoff.
// Handles transient failures (5xx, timeouts, connection errors) with
// configurable retry count, backoff, and jitter.
package retryhttp

import (
	"context"
	"math/rand/v2"
	"net/http"
	"time"
)

// Config controls retry behavior.
type Config struct {
	MaxRetries  int           // max retry attempts (0 = no retry)
	BaseDelay   time.Duration // initial backoff delay
	MaxDelay    time.Duration // maximum backoff delay cap
	Jitter      bool          // add randomized jitter
	RetryOn     func(resp *http.Response, err error) bool
	Client      *http.Client
}

// DefaultConfig returns defaults: 3 retries, 100ms base, 5s cap, jitter on.
func DefaultConfig() Config {
	return Config{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   5 * time.Second,
		Jitter:     true,
		RetryOn:    DefaultRetryOn,
		Client:     http.DefaultClient,
	}
}

// DefaultRetryOn retries on connection errors and 5xx responses.
func DefaultRetryOn(resp *http.Response, err error) bool {
	if err != nil {
		return true
	}
	return resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests
}

// Do executes an HTTP request with retry logic.
func Do(cfg Config, req *http.Request) (*http.Response, error) {
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.BaseDelay <= 0 {
		cfg.BaseDelay = 100 * time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 5 * time.Second
	}
	if cfg.RetryOn == nil {
		cfg.RetryOn = DefaultRetryOn
	}
	if cfg.Client == nil {
		cfg.Client = http.DefaultClient
	}

	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := backoff(cfg.BaseDelay, cfg.MaxDelay, attempt-1, cfg.Jitter)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
		}

		resp, err := cfg.Client.Do(req)
		if err == nil && !cfg.RetryOn(resp, nil) {
			return resp, nil
		}
		if err != nil && !cfg.RetryOn(nil, err) {
			return nil, err
		}

		lastErr = err
		lastResp = resp
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

// backoff calculates delay with exponential backoff and optional jitter.
func backoff(base, max time.Duration, attempt int, jitter bool) time.Duration {
	delay := base
	for i := 0; i < attempt; i++ {
		delay *= 2
		if delay > max {
			delay = max
			break
		}
	}
	if jitter {
		delay = time.Duration(rand.Int64N(int64(delay) + 1))
	}
	return delay
}

// SimpleGet performs a GET with default retry config.
func SimpleGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return Do(DefaultConfig(), req)
}
