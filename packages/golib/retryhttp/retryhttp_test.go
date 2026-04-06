package retryhttp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxRetries != 3 {
		t.Errorf("MaxRetries: got %d, want 3", cfg.MaxRetries)
	}
	if cfg.BaseDelay != 100*time.Millisecond {
		t.Errorf("BaseDelay: got %v, want 100ms", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 5*time.Second {
		t.Errorf("MaxDelay: got %v, want 5s", cfg.MaxDelay)
	}
	if !cfg.Jitter {
		t.Error("Jitter should be true by default")
	}
	if cfg.RetryOn == nil {
		t.Error("RetryOn should not be nil")
	}
	if cfg.Client == nil {
		t.Error("Client should not be nil")
	}
}

func TestDefaultRetryOn(t *testing.T) {
	tests := []struct {
		name   string
		resp   *http.Response
		err    error
		want   bool
	}{
		{"error", nil, fmt.Errorf("connection refused"), true},
		{"500", &http.Response{StatusCode: 500}, nil, true},
		{"502", &http.Response{StatusCode: 502}, nil, true},
		{"503", &http.Response{StatusCode: 503}, nil, true},
		{"429", &http.Response{StatusCode: 429}, nil, true},
		{"200", &http.Response{StatusCode: 200}, nil, false},
		{"400", &http.Response{StatusCode: 400}, nil, false},
		{"404", &http.Response{StatusCode: 404}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultRetryOn(tt.resp, tt.err); got != tt.want {
				t.Errorf("DefaultRetryOn() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDoSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	cfg := Config{
		MaxRetries: 3,
		BaseDelay:  time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Jitter:     false,
		RetryOn:    DefaultRetryOn,
		Client:     ts.Client(),
	}
	resp, err := Do(cfg, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestDoRetryThenSuccess(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := count.Add(1)
		if n <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	cfg := Config{
		MaxRetries: 3,
		BaseDelay:  time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Jitter:     false,
		RetryOn:    DefaultRetryOn,
		Client:     ts.Client(),
	}
	resp, err := Do(cfg, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if got := count.Load(); got != 3 {
		t.Errorf("attempts: got %d, want 3", got)
	}
}

func TestDoExhaustedRetries(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	cfg := Config{
		MaxRetries: 2,
		BaseDelay:  time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Jitter:     false,
		RetryOn:    DefaultRetryOn,
		Client:     ts.Client(),
	}
	resp, err := Do(cfg, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After exhausting retries, returns last response
	if resp.StatusCode != 500 {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
	// initial + 2 retries = 3 attempts
	if got := count.Load(); got != 3 {
		t.Errorf("attempts: got %d, want 3", got)
	}
}

func TestDoNoRetry(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	cfg := Config{
		MaxRetries: 0,
		BaseDelay:  time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		RetryOn:    DefaultRetryOn,
		Client:     ts.Client(),
	}
	resp, err := Do(cfg, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Errorf("status: got %d, want 500", resp.StatusCode)
	}
	if got := count.Load(); got != 1 {
		t.Errorf("attempts: got %d, want 1", got)
	}
}

func TestDoContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())

	var count atomic.Int32
	cfg := Config{
		MaxRetries: 10,
		BaseDelay:  50 * time.Millisecond,
		MaxDelay:   100 * time.Millisecond,
		Jitter:     false,
		RetryOn: func(resp *http.Response, err error) bool {
			if count.Add(1) == 2 {
				cancel()
			}
			return DefaultRetryOn(resp, err)
		},
		Client: ts.Client(),
	}

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL, nil)
	_, err := Do(cfg, req)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestDoNegativeRetries(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	cfg := Config{MaxRetries: -5, Client: ts.Client()}
	resp, err := Do(cfg, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestDoDefaultsZeroConfig(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	// Zero config — all defaults should be applied
	resp, err := Do(Config{Client: ts.Client()}, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestBackoff(t *testing.T) {
	tests := []struct {
		name    string
		base    time.Duration
		max     time.Duration
		attempt int
		jitter  bool
		wantMax time.Duration
	}{
		{"attempt0", 100 * time.Millisecond, 5 * time.Second, 0, false, 100 * time.Millisecond},
		{"attempt1", 100 * time.Millisecond, 5 * time.Second, 1, false, 200 * time.Millisecond},
		{"attempt2", 100 * time.Millisecond, 5 * time.Second, 2, false, 400 * time.Millisecond},
		{"capped", 100 * time.Millisecond, 500 * time.Millisecond, 10, false, 500 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backoff(tt.base, tt.max, tt.attempt, tt.jitter)
			if got != tt.wantMax {
				t.Errorf("backoff() = %v, want %v", got, tt.wantMax)
			}
		})
	}
}

func TestBackoffWithJitter(t *testing.T) {
	base := 100 * time.Millisecond
	max := 5 * time.Second
	// With jitter, result should be <= base for attempt 0
	for i := 0; i < 100; i++ {
		d := backoff(base, max, 0, true)
		if d > base {
			t.Errorf("jitter delay %v > base %v", d, base)
		}
		if d < 0 {
			t.Errorf("jitter delay %v < 0", d)
		}
	}
}

func TestDoCustomRetryOn(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	// Custom: retry on 400 too
	cfg := Config{
		MaxRetries: 2,
		BaseDelay:  time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		RetryOn: func(resp *http.Response, err error) bool {
			if err != nil {
				return true
			}
			return resp.StatusCode == 400
		},
		Client: ts.Client(),
	}

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := Do(cfg, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Errorf("status: got %d, want 400", resp.StatusCode)
	}
	if got := count.Load(); got != 3 {
		t.Errorf("attempts: got %d, want 3", got)
	}
}

func TestSimpleGet(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: got %q, want GET", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	resp, err := SimpleGet(context.Background(), ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}

func TestDo429Retry(t *testing.T) {
	var count atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if count.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	cfg := Config{
		MaxRetries: 3,
		BaseDelay:  time.Millisecond,
		MaxDelay:   10 * time.Millisecond,
		Jitter:     false,
		RetryOn:    DefaultRetryOn,
		Client:     ts.Client(),
	}
	resp, err := Do(cfg, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
}
