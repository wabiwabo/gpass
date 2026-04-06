package store

import (
	"sync"
	"time"
)

// DailyUsage represents aggregated usage for a single day.
type DailyUsage struct {
	Date   time.Time
	Calls  int64
	Errors int64
}

// EndpointUsage represents aggregated usage for a single endpoint.
type EndpointUsage struct {
	Endpoint string
	Calls    int64
	Errors   int64
}

// UsageStore defines the interface for usage metering persistence.
type UsageStore interface {
	Increment(appID, endpoint string, isError bool) error
	GetDailyUsage(appID string, date time.Time) (int64, error)
	GetUsageRange(appID string, from, to time.Time) ([]DailyUsage, error)
	GetUsageByEndpoint(appID string, from, to time.Time) ([]EndpointUsage, error)
}

// usageKey uniquely identifies a usage record by app, date, and endpoint.
type usageKey struct {
	AppID    string
	Date     string // YYYY-MM-DD
	Endpoint string
}

type usageRecord struct {
	Calls  int64
	Errors int64
}

// InMemoryUsageStore is a thread-safe in-memory implementation of UsageStore.
type InMemoryUsageStore struct {
	mu      sync.RWMutex
	records map[usageKey]*usageRecord
}

// NewInMemoryUsageStore creates a new in-memory usage store.
func NewInMemoryUsageStore() *InMemoryUsageStore {
	return &InMemoryUsageStore{
		records: make(map[usageKey]*usageRecord),
	}
}

func dateKey(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// Increment records an API call for the given app and endpoint.
func (s *InMemoryUsageStore) Increment(appID, endpoint string, isError bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := usageKey{
		AppID:    appID,
		Date:     dateKey(time.Now()),
		Endpoint: endpoint,
	}

	rec, ok := s.records[key]
	if !ok {
		rec = &usageRecord{}
		s.records[key] = rec
	}

	rec.Calls++
	if isError {
		rec.Errors++
	}

	return nil
}

// GetDailyUsage returns the total number of calls for an app on a given day.
func (s *InMemoryUsageStore) GetDailyUsage(appID string, date time.Time) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	dateStr := dateKey(date)
	var total int64
	for k, rec := range s.records {
		if k.AppID == appID && k.Date == dateStr {
			total += rec.Calls
		}
	}
	return total, nil
}

// GetUsageRange returns daily-aggregated usage for an app within a date range (inclusive).
func (s *InMemoryUsageStore) GetUsageRange(appID string, from, to time.Time) ([]DailyUsage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fromStr := dateKey(from)
	toStr := dateKey(to)

	// Aggregate by date
	byDate := make(map[string]*DailyUsage)
	for k, rec := range s.records {
		if k.AppID != appID {
			continue
		}
		if k.Date < fromStr || k.Date > toStr {
			continue
		}
		du, ok := byDate[k.Date]
		if !ok {
			parsed, _ := time.Parse("2006-01-02", k.Date)
			du = &DailyUsage{Date: parsed}
			byDate[k.Date] = du
		}
		du.Calls += rec.Calls
		du.Errors += rec.Errors
	}

	// Collect sorted results
	var result []DailyUsage
	current := from.UTC().Truncate(24 * time.Hour)
	end := to.UTC().Truncate(24 * time.Hour)
	for !current.After(end) {
		ds := dateKey(current)
		if du, ok := byDate[ds]; ok {
			result = append(result, *du)
		}
		current = current.Add(24 * time.Hour)
	}

	return result, nil
}

// GetUsageByEndpoint returns endpoint-aggregated usage for an app within a date range.
func (s *InMemoryUsageStore) GetUsageByEndpoint(appID string, from, to time.Time) ([]EndpointUsage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fromStr := dateKey(from)
	toStr := dateKey(to)

	byEndpoint := make(map[string]*EndpointUsage)
	for k, rec := range s.records {
		if k.AppID != appID {
			continue
		}
		if k.Date < fromStr || k.Date > toStr {
			continue
		}
		eu, ok := byEndpoint[k.Endpoint]
		if !ok {
			eu = &EndpointUsage{Endpoint: k.Endpoint}
			byEndpoint[k.Endpoint] = eu
		}
		eu.Calls += rec.Calls
		eu.Errors += rec.Errors
	}

	var result []EndpointUsage
	for _, eu := range byEndpoint {
		result = append(result, *eu)
	}
	return result, nil
}
