package circuitbreaker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	cb := New(5, time.Second)
	r.Register("service-a", cb)

	got, ok := r.Get("service-a")
	if !ok {
		t.Fatal("expected to find service-a")
	}
	if got != cb {
		t.Error("returned breaker does not match registered one")
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	r := NewRegistry()

	_, ok := r.Get("unknown")
	if ok {
		t.Error("expected false for unknown breaker")
	}
}

func TestRegistryStatus(t *testing.T) {
	r := NewRegistry()
	cb1 := New(3, time.Second)
	cb2 := New(5, time.Second)

	r.Register("service-a", cb1)
	r.Register("service-b", cb2)

	statuses := r.Status()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	found := make(map[string]BreakerStatus)
	for _, s := range statuses {
		found[s.Name] = s
	}

	a, ok := found["service-a"]
	if !ok {
		t.Fatal("missing service-a")
	}
	if a.State != StateClosed {
		t.Errorf("expected closed, got %s", a.State)
	}
	if a.Threshold != 3 {
		t.Errorf("expected threshold 3, got %d", a.Threshold)
	}
	if a.FailureCount != 0 {
		t.Errorf("expected 0 failures, got %d", a.FailureCount)
	}
	if a.LastFailure != "" {
		t.Errorf("expected empty last_failure, got %s", a.LastFailure)
	}
}

func TestRegistryStatusReflectsStateChange(t *testing.T) {
	r := NewRegistry()
	cb := New(2, time.Second)
	r.Register("svc", cb)

	// Verify closed
	statuses := r.Status()
	if statuses[0].State != StateClosed {
		t.Fatalf("expected closed, got %s", statuses[0].State)
	}

	// Trip the breaker
	cb.RecordFailure()
	cb.RecordFailure()

	statuses = r.Status()
	if statuses[0].State != StateOpen {
		t.Errorf("expected open, got %s", statuses[0].State)
	}
	if statuses[0].FailureCount != 2 {
		t.Errorf("expected 2 failures, got %d", statuses[0].FailureCount)
	}
	if statuses[0].LastFailure == "" {
		t.Error("expected last_failure to be set")
	}
}

func TestRegistryHandler(t *testing.T) {
	r := NewRegistry()
	cb := New(3, time.Second)
	r.Register("payment-gw", cb)
	cb.RecordFailure()

	handler := r.Handler()
	req := httptest.NewRequest(http.MethodGet, "/internal/circuit-breakers", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var statuses []BreakerStatus
	if err := json.Unmarshal(w.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}

	if statuses[0].Name != "payment-gw" {
		t.Errorf("expected name payment-gw, got %s", statuses[0].Name)
	}
	if statuses[0].FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", statuses[0].FailureCount)
	}
}
