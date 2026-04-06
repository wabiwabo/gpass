package circuithttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTransport_SuccessfulRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewTransport(http.DefaultTransport, DefaultConfig())
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d", resp.StatusCode)
	}
	if tr.State() != StateClosed {
		t.Errorf("state: got %v", tr.State())
	}
}

func TestTransport_OpensAfterFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{FailureThreshold: 3, OpenDuration: 1 * time.Second}
	tr := NewTransport(http.DefaultTransport, cfg)

	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, _ := tr.RoundTrip(req)
		if resp != nil {
			resp.Body.Close()
		}
	}

	if tr.State() != StateOpen {
		t.Errorf("should be open after %d failures: got %v", 3, tr.State())
	}

	// Next request should fail fast.
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	_, err := tr.RoundTrip(req)
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("should return ErrCircuitOpen: got %v", err)
	}
}

func TestTransport_HalfOpenRecovery(t *testing.T) {
	var serverHealthy atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serverHealthy.Load() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	cfg := Config{
		FailureThreshold: 2,
		SuccessThreshold: 2,
		OpenDuration:     50 * time.Millisecond,
		HalfOpenMax:      1,
	}
	tr := NewTransport(http.DefaultTransport, cfg)

	// Trip the circuit.
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, _ := tr.RoundTrip(req)
		if resp != nil {
			resp.Body.Close()
		}
	}

	if tr.State() != StateOpen {
		t.Fatalf("should be open: got %v", tr.State())
	}

	// Wait for open duration.
	time.Sleep(60 * time.Millisecond)
	serverHealthy.Store(true)

	// First request should transition to half-open.
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatalf("half-open request %d: %v", i, err)
		}
		resp.Body.Close()
	}

	if tr.State() != StateClosed {
		t.Errorf("should recover to closed: got %v", tr.State())
	}
}

func TestTransport_HalfOpenFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{
		FailureThreshold: 2,
		OpenDuration:     50 * time.Millisecond,
		HalfOpenMax:      1,
	}
	tr := NewTransport(http.DefaultTransport, cfg)

	// Trip it.
	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, _ := tr.RoundTrip(req)
		if resp != nil {
			resp.Body.Close()
		}
	}

	time.Sleep(60 * time.Millisecond)

	// Half-open probe fails → back to open.
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, _ := tr.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}

	if tr.State() != StateOpen {
		t.Errorf("should return to open: got %v", tr.State())
	}
}

func TestTransport_CustomFailureClassifier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests) // 429
	}))
	defer server.Close()

	cfg := Config{
		FailureThreshold: 2,
		IsFailure: func(resp *http.Response, err error) bool {
			if err != nil {
				return true
			}
			return resp.StatusCode == 429 || resp.StatusCode >= 500
		},
	}
	tr := NewTransport(http.DefaultTransport, cfg)

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, _ := tr.RoundTrip(req)
		if resp != nil {
			resp.Body.Close()
		}
	}

	if tr.State() != StateOpen {
		t.Errorf("429 should trigger circuit: got %v", tr.State())
	}
}

func TestTransport_Stats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewTransport(http.DefaultTransport, DefaultConfig())
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, _ := tr.RoundTrip(req)
	resp.Body.Close()

	stats := tr.Stats()
	if stats.TotalRequests != 1 {
		t.Errorf("total: got %d", stats.TotalRequests)
	}
	if stats.State != StateClosed {
		t.Errorf("state: got %v", stats.State)
	}
}

func TestTransport_TrippedStats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{FailureThreshold: 1, OpenDuration: 10 * time.Second}
	tr := NewTransport(http.DefaultTransport, cfg)

	// Trip it.
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, _ := tr.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}

	// Try when open.
	req, _ = http.NewRequest(http.MethodGet, server.URL, nil)
	tr.RoundTrip(req)

	stats := tr.Stats()
	if stats.TrippedRequests != 1 {
		t.Errorf("tripped: got %d", stats.TrippedRequests)
	}
}

func TestTransport_Reset(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := Config{FailureThreshold: 1, OpenDuration: 1 * time.Hour}
	tr := NewTransport(http.DefaultTransport, cfg)

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, _ := tr.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}

	tr.Reset()
	if tr.State() != StateClosed {
		t.Errorf("after reset: got %v", tr.State())
	}
}

func TestTransport_OnStateChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	var transitions []string
	var mu sync.Mutex

	cfg := Config{FailureThreshold: 1, OpenDuration: 10 * time.Second}
	tr := NewTransport(http.DefaultTransport, cfg)
	tr.OnStateChange(func(from, to State) {
		mu.Lock()
		defer mu.Unlock()
		transitions = append(transitions, from.String()+"→"+to.String())
	})

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, _ := tr.RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(transitions) == 0 {
		t.Error("should record state transition")
	}
	if transitions[0] != "closed→open" {
		t.Errorf("transition: got %q", transitions[0])
	}
}

func TestTransport_DefaultTransport(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewTransport(nil, DefaultConfig()) // nil base.
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestTransport_Client(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewTransport(http.DefaultTransport, DefaultConfig())
	client := tr.Client()

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestTransport_Do(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewTransport(http.DefaultTransport, DefaultConfig())
	resp, err := tr.Do(context.Background(), http.MethodGet, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
}

func TestTransport_ConcurrentRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tr := NewTransport(http.DefaultTransport, DefaultConfig())

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
			resp, err := tr.RoundTrip(req)
			if err != nil {
				return
			}
			resp.Body.Close()
		}()
	}
	wg.Wait()

	stats := tr.Stats()
	if stats.TotalRequests != 50 {
		t.Errorf("total: got %d", stats.TotalRequests)
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("%d: got %q", tt.state, got)
		}
	}
}

func TestTransport_SuccessResetsFailures(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := Config{FailureThreshold: 3, OpenDuration: 1 * time.Second}
	tr := NewTransport(http.DefaultTransport, cfg)

	// 2 failures, then 1 success → should reset failure counter.
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
		resp, _ := tr.RoundTrip(req)
		if resp != nil {
			resp.Body.Close()
		}
	}

	if tr.State() != StateClosed {
		t.Errorf("success should reset: got %v", tr.State())
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.FailureThreshold != 5 {
		t.Errorf("failure threshold: got %d", cfg.FailureThreshold)
	}
	if cfg.SuccessThreshold != 3 {
		t.Errorf("success threshold: got %d", cfg.SuccessThreshold)
	}
	if cfg.OpenDuration != 30*time.Second {
		t.Errorf("open duration: got %v", cfg.OpenDuration)
	}
}
