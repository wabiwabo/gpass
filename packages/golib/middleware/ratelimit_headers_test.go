package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimitHeaders_PresentOnSuccess(t *testing.T) {
	tiers := map[string]TierConfig{
		"free": {Name: "Free", DailyLimit: 100, BurstLimit: 10},
	}
	limiter := NewTieredRateLimiter(tiers)
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	handler := RateLimitHeaders(limiter,
		func(r *http.Request) string { return "testkey" },
		func(r *http.Request) string { return "free" },
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	for _, header := range []string{"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset"} {
		if v := w.Header().Get(header); v == "" {
			t.Errorf("expected %s header to be present", header)
		}
	}
}

func TestRateLimitHeaders_RemainingDecrements(t *testing.T) {
	tiers := map[string]TierConfig{
		"free": {Name: "Free", DailyLimit: 100, BurstLimit: 0},
	}
	limiter := NewTieredRateLimiter(tiers)
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	handler := RateLimitHeaders(limiter,
		func(r *http.Request) string { return "testkey" },
		func(r *http.Request) string { return "free" },
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	remaining1, _ := strconv.Atoi(w.Header().Get("RateLimit-Remaining"))

	// Second request.
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	remaining2, _ := strconv.Atoi(w.Header().Get("RateLimit-Remaining"))

	if remaining2 >= remaining1 {
		t.Errorf("remaining should decrement: first=%d, second=%d", remaining1, remaining2)
	}
	if remaining1 != 99 {
		t.Errorf("expected first remaining=99, got %d", remaining1)
	}
	if remaining2 != 98 {
		t.Errorf("expected second remaining=98, got %d", remaining2)
	}
}

func TestRateLimitHeaders_RetryAfterOn429(t *testing.T) {
	tiers := map[string]TierConfig{
		"free": {Name: "Free", DailyLimit: 1, BurstLimit: 0},
	}
	limiter := NewTieredRateLimiter(tiers)
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	handler := RateLimitHeaders(limiter,
		func(r *http.Request) string { return "testkey" },
		func(r *http.Request) string { return "free" },
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request: allowed.
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("first request should succeed, got %d", w.Code)
	}

	// Second request: rate limited.
	r = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}

	retryAfter := w.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header on 429 response")
	}

	retryVal, err := strconv.Atoi(retryAfter)
	if err != nil {
		t.Fatalf("Retry-After should be an integer: %v", err)
	}
	if retryVal <= 0 {
		t.Errorf("Retry-After should be positive, got %d", retryVal)
	}
}

func TestRateLimitHeaders_LimitMatchesTierConfig(t *testing.T) {
	tiers := map[string]TierConfig{
		"starter": {Name: "Starter", DailyLimit: 10000, BurstLimit: 100},
	}
	limiter := NewTieredRateLimiter(tiers)
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	handler := RateLimitHeaders(limiter,
		func(r *http.Request) string { return "testkey" },
		func(r *http.Request) string { return "starter" },
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	limit := w.Header().Get("RateLimit-Limit")
	if limit != "10000" {
		t.Errorf("expected RateLimit-Limit=10000, got %s", limit)
	}
}

func TestRateLimitHeaders_ResetIsPositiveInteger(t *testing.T) {
	tiers := map[string]TierConfig{
		"free": {Name: "Free", DailyLimit: 100, BurstLimit: 10},
	}
	limiter := NewTieredRateLimiter(tiers)
	now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }

	handler := RateLimitHeaders(limiter,
		func(r *http.Request) string { return "testkey" },
		func(r *http.Request) string { return "free" },
	)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	resetStr := w.Header().Get("RateLimit-Reset")
	resetVal, err := strconv.Atoi(resetStr)
	if err != nil {
		t.Fatalf("RateLimit-Reset should be an integer: %v", err)
	}
	if resetVal <= 0 {
		t.Errorf("RateLimit-Reset should be positive, got %d", resetVal)
	}
}
