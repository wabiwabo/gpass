package backpressure

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewController_Defaults(t *testing.T) {
	c := NewController(Config{})
	if c.config.MaxInFlight != 1000 {
		t.Errorf("max in-flight: got %d", c.config.MaxInFlight)
	}
	if c.config.ShedThreshold != 2000 {
		t.Errorf("shed threshold: got %d", c.config.ShedThreshold)
	}
}

func TestController_AdmitNormal(t *testing.T) {
	c := NewController(Config{MaxInFlight: 100, ShedThreshold: 200})
	if !c.Admit() {
		t.Error("should admit when no load")
	}
	if c.State() != StateNormal {
		t.Errorf("state: got %v", c.State())
	}
}

func TestController_ShedAtThreshold(t *testing.T) {
	c := NewController(Config{MaxInFlight: 5, ShedThreshold: 10, CooldownPeriod: 1 * time.Nanosecond})

	// Simulate 10 in-flight requests.
	for i := 0; i < 10; i++ {
		c.inFlight.Add(1)
	}

	if c.Admit() {
		t.Error("should shed at threshold")
	}
	// Give time for async state transition.
	time.Sleep(10 * time.Millisecond)
	if c.State() != StateShed {
		t.Errorf("state: got %v, want shed", c.State())
	}
}

func TestController_ThrottleZone(t *testing.T) {
	c := NewController(Config{MaxInFlight: 10, ShedThreshold: 20, CooldownPeriod: 1 * time.Nanosecond})

	// Put in-flight above max threshold well into throttle zone (ratio > 0.5).
	c.inFlight.Store(16) // ratio = (16-10)/(20-10) = 0.6 > 0.5 → reject

	rejected := 0
	for i := 0; i < 10; i++ {
		if !c.Admit() {
			rejected++
		}
	}

	if rejected == 0 {
		t.Error("should reject requests in deep throttle zone")
	}
}

func TestController_BeginEnd(t *testing.T) {
	c := NewController(DefaultConfig())

	done := c.Begin()
	if c.InFlight() != 1 {
		t.Errorf("in-flight: got %d", c.InFlight())
	}
	done()
	if c.InFlight() != 0 {
		t.Errorf("after done: got %d", c.InFlight())
	}
}

func TestController_LatencyTracking(t *testing.T) {
	c := NewController(DefaultConfig())

	done := c.Begin()
	time.Sleep(10 * time.Millisecond)
	done()

	stats := c.Stats()
	if stats.P99Latency < 5*time.Millisecond {
		t.Errorf("p99 should reflect sleep: got %v", stats.P99Latency)
	}
}

func TestController_Stats(t *testing.T) {
	c := NewController(Config{MaxInFlight: 5, ShedThreshold: 10})

	c.Admit()
	c.Admit()
	done := c.Begin()
	done()

	stats := c.Stats()
	if stats.TotalReqs != 2 {
		t.Errorf("total: got %d", stats.TotalReqs)
	}
	if stats.InFlight != 0 {
		t.Errorf("in-flight: got %d", stats.InFlight)
	}
}

func TestStats_RejectRatio(t *testing.T) {
	s := Stats{TotalReqs: 100, RejectedReqs: 25}
	if r := s.RejectRatio(); r != 0.25 {
		t.Errorf("ratio: got %f", r)
	}

	s = Stats{TotalReqs: 0}
	if r := s.RejectRatio(); r != 0 {
		t.Error("zero total should return 0")
	}
}

func TestController_OnStateChange(t *testing.T) {
	c := NewController(Config{MaxInFlight: 5, ShedThreshold: 10, CooldownPeriod: 1 * time.Nanosecond})

	var oldState, newState State
	var called atomic.Bool
	c.OnStateChange(func(o, n State) {
		oldState = o
		newState = n
		called.Store(true)
	})

	// Push past shed threshold.
	for i := 0; i < 10; i++ {
		c.inFlight.Add(1)
	}
	c.Admit()

	time.Sleep(50 * time.Millisecond)
	if !called.Load() {
		t.Error("state change callback should fire")
	}
	if oldState != StateNormal {
		t.Errorf("old state: got %v", oldState)
	}
	if newState != StateShed {
		t.Errorf("new state: got %v", newState)
	}
}

func TestController_CooldownPreventsRapidTransitions(t *testing.T) {
	c := NewController(Config{
		MaxInFlight:    5,
		ShedThreshold:  10,
		CooldownPeriod: 1 * time.Second,
	})

	// Trigger shed.
	for i := 0; i < 10; i++ {
		c.inFlight.Add(1)
	}
	c.Admit()

	// Immediately try to recover — cooldown should prevent it.
	c.inFlight.Store(0)
	c.Admit()

	// State should still be shed due to cooldown.
	if c.State() != StateShed {
		t.Errorf("cooldown should prevent transition: got %v", c.State())
	}
}

func TestController_RecoveryToNormal(t *testing.T) {
	c := NewController(Config{MaxInFlight: 10, ShedThreshold: 20, CooldownPeriod: 1 * time.Nanosecond})

	// Trigger shed.
	c.inFlight.Store(20)
	c.Admit()
	time.Sleep(10 * time.Millisecond)

	// Reduce load well below threshold.
	c.inFlight.Store(2) // Below MaxInFlight/2 = 5.
	c.Admit()
	time.Sleep(10 * time.Millisecond)

	if c.State() != StateNormal {
		t.Errorf("should recover to normal: got %v", c.State())
	}
}

func TestMiddleware_Accepts(t *testing.T) {
	c := NewController(DefaultConfig())
	handler := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("should accept: got %d", w.Code)
	}
}

func TestMiddleware_Rejects(t *testing.T) {
	c := NewController(Config{MaxInFlight: 1, ShedThreshold: 2})
	c.inFlight.Store(5)

	handler := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called when shedding")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("should reject: got %d", w.Code)
	}
	if w.Header().Get("Retry-After") != "5" {
		t.Error("should set Retry-After header")
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != float64(503) {
		t.Error("body should have RFC 7807 status")
	}
}

func TestMiddleware_TracksInFlight(t *testing.T) {
	c := NewController(DefaultConfig())
	var inFlightDuringRequest int64

	handler := c.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inFlightDuringRequest = c.InFlight()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if inFlightDuringRequest != 1 {
		t.Errorf("in-flight during request: got %d", inFlightDuringRequest)
	}
	if c.InFlight() != 0 {
		t.Errorf("in-flight after request: got %d", c.InFlight())
	}
}

func TestPriorityAdmit_CriticalAtFullShed(t *testing.T) {
	c := NewController(Config{MaxInFlight: 5, ShedThreshold: 10})
	c.inFlight.Store(15) // Beyond shed threshold.

	if !c.PriorityAdmit(0) {
		t.Error("critical priority should pass even at full shed")
	}
	if c.PriorityAdmit(2) {
		t.Error("low priority should be rejected at full shed")
	}
}

func TestPriorityAdmit_HighDuringThrottle(t *testing.T) {
	c := NewController(Config{MaxInFlight: 5, ShedThreshold: 10})
	c.inFlight.Store(7) // In throttle zone.

	if !c.PriorityAdmit(1) {
		t.Error("high priority should pass during throttle")
	}
	if c.PriorityAdmit(3) {
		t.Error("low priority should be rejected during throttle")
	}
}

func TestPriorityAdmit_AllPassNormal(t *testing.T) {
	c := NewController(Config{MaxInFlight: 100, ShedThreshold: 200})

	if !c.PriorityAdmit(5) {
		t.Error("all priorities should pass in normal state")
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateNormal, "normal"},
		{StateThrottle, "throttle"},
		{StateShed, "shed"},
		{State(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("%d.String(): got %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestController_ConcurrentAdmit(t *testing.T) {
	c := NewController(Config{MaxInFlight: 1000, ShedThreshold: 2000})

	var wg sync.WaitGroup
	var admitted, rejected atomic.Int64
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c.Admit() {
				admitted.Add(1)
			} else {
				rejected.Add(1)
			}
		}()
	}
	wg.Wait()

	total := admitted.Load() + rejected.Load()
	if total != 200 {
		t.Errorf("total: got %d", total)
	}
}

func TestController_ConcurrentBeginEnd(t *testing.T) {
	c := NewController(DefaultConfig())

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			done := c.Begin()
			time.Sleep(time.Millisecond)
			done()
		}()
	}
	wg.Wait()

	if c.InFlight() != 0 {
		t.Errorf("in-flight should be 0 after all done: got %d", c.InFlight())
	}
}

func TestWithController_Context(t *testing.T) {
	c := NewController(DefaultConfig())
	ctx := WithController(context.Background(), c)

	got, ok := ControllerFromContext(ctx)
	if !ok {
		t.Fatal("should find controller")
	}
	if got != c {
		t.Error("should return same controller")
	}
}

func TestControllerFromContext_Missing(t *testing.T) {
	_, ok := ControllerFromContext(context.Background())
	if ok {
		t.Error("should return false for empty context")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxInFlight != 1000 {
		t.Errorf("max in-flight: got %d", cfg.MaxInFlight)
	}
	if cfg.ShedThreshold != 2000 {
		t.Errorf("shed threshold: got %d", cfg.ShedThreshold)
	}
	if cfg.LatencyThreshold != 5*time.Second {
		t.Errorf("latency threshold: got %v", cfg.LatencyThreshold)
	}
}
