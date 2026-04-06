package store

import (
	"testing"
	"time"
)

func TestUsageStore_IncrementAndGetDaily(t *testing.T) {
	s := NewInMemoryUsageStore()

	s.Increment("app-1", "/api/v1/verify", false)
	s.Increment("app-1", "/api/v1/verify", false)
	s.Increment("app-1", "/api/v1/sign", true)

	today := time.Now().UTC()
	count, err := s.GetDailyUsage("app-1", today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 calls, got %d", count)
	}
}

func TestUsageStore_GetDailyUsage_NoData(t *testing.T) {
	s := NewInMemoryUsageStore()

	count, err := s.GetDailyUsage("app-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 calls for no data, got %d", count)
	}
}

func TestUsageStore_GetUsageRange(t *testing.T) {
	s := NewInMemoryUsageStore()

	// Manually insert records for different dates
	s.mu.Lock()
	today := time.Now().UTC()
	todayStr := dateKey(today)
	s.records[usageKey{AppID: "app-1", Date: todayStr, Endpoint: "/verify"}] = &usageRecord{Calls: 10, Errors: 1}
	s.records[usageKey{AppID: "app-1", Date: todayStr, Endpoint: "/sign"}] = &usageRecord{Calls: 5, Errors: 0}
	s.mu.Unlock()

	from := today.Add(-24 * time.Hour)
	to := today.Add(24 * time.Hour)
	usage, err := s.GetUsageRange("app-1", from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(usage) != 1 {
		t.Fatalf("expected 1 day of usage, got %d", len(usage))
	}
	if usage[0].Calls != 15 {
		t.Errorf("expected 15 total calls, got %d", usage[0].Calls)
	}
	if usage[0].Errors != 1 {
		t.Errorf("expected 1 error, got %d", usage[0].Errors)
	}
}

func TestUsageStore_GetUsageByEndpoint(t *testing.T) {
	s := NewInMemoryUsageStore()

	s.Increment("app-1", "/api/v1/verify", false)
	s.Increment("app-1", "/api/v1/verify", true)
	s.Increment("app-1", "/api/v1/sign", false)

	today := time.Now().UTC()
	from := today.Add(-1 * time.Hour)
	to := today.Add(1 * time.Hour)

	usage, err := s.GetUsageByEndpoint("app-1", from, to)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(usage) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(usage))
	}

	endpointMap := make(map[string]EndpointUsage)
	for _, eu := range usage {
		endpointMap[eu.Endpoint] = eu
	}

	verify := endpointMap["/api/v1/verify"]
	if verify.Calls != 2 {
		t.Errorf("expected 2 calls for /verify, got %d", verify.Calls)
	}
	if verify.Errors != 1 {
		t.Errorf("expected 1 error for /verify, got %d", verify.Errors)
	}

	sign := endpointMap["/api/v1/sign"]
	if sign.Calls != 1 {
		t.Errorf("expected 1 call for /sign, got %d", sign.Calls)
	}
}
