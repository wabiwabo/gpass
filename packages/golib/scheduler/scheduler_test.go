package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestRegisterAndStatus(t *testing.T) {
	s := New()
	s.Register("test-job", 1*time.Second, func(ctx context.Context) error {
		return nil
	})

	statuses := s.Status()
	if len(statuses) != 1 {
		t.Fatalf("expected 1 job, got %d", len(statuses))
	}
	if statuses[0].Name != "test-job" {
		t.Errorf("expected name 'test-job', got %q", statuses[0].Name)
	}
	if statuses[0].Interval != "1s" {
		t.Errorf("expected interval '1s', got %q", statuses[0].Interval)
	}
	if statuses[0].Status != "stopped" {
		t.Errorf("expected status 'stopped', got %q", statuses[0].Status)
	}
	if statuses[0].LastRun != "" {
		t.Errorf("expected empty last_run, got %q", statuses[0].LastRun)
	}
}

func TestStartExecutesJobs(t *testing.T) {
	var count atomic.Int64
	s := New()
	s.Register("counter", 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	s.Start()

	// Wait for a few executions.
	time.Sleep(60 * time.Millisecond)
	s.Stop()

	c := count.Load()
	if c < 2 {
		t.Errorf("expected at least 2 executions, got %d", c)
	}

	statuses := s.Status()
	if statuses[0].RunCount < 2 {
		t.Errorf("expected RunCount >= 2, got %d", statuses[0].RunCount)
	}
}

func TestJobErrorIncrementsErrorCount(t *testing.T) {
	s := New()
	s.Register("failing-job", 10*time.Millisecond, func(ctx context.Context) error {
		return errors.New("test error")
	})

	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Stop()

	statuses := s.Status()
	if statuses[0].Errors == 0 {
		t.Error("expected errors > 0")
	}
	if statuses[0].Errors != statuses[0].RunCount {
		t.Errorf("expected errors (%d) == run count (%d) for always-failing job",
			statuses[0].Errors, statuses[0].RunCount)
	}
}

func TestStopHaltsExecution(t *testing.T) {
	var count atomic.Int64
	s := New()
	s.Register("stopper", 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Stop()

	countAfterStop := count.Load()
	time.Sleep(30 * time.Millisecond)
	countLater := count.Load()

	if countLater != countAfterStop {
		t.Errorf("job continued after stop: count went from %d to %d", countAfterStop, countLater)
	}
}

func TestMultipleJobsRunIndependently(t *testing.T) {
	var count1, count2 atomic.Int64
	s := New()
	s.Register("job1", 10*time.Millisecond, func(ctx context.Context) error {
		count1.Add(1)
		return nil
	})
	s.Register("job2", 10*time.Millisecond, func(ctx context.Context) error {
		count2.Add(1)
		return nil
	})

	s.Start()
	time.Sleep(50 * time.Millisecond)
	s.Stop()

	if count1.Load() < 2 {
		t.Errorf("job1 expected at least 2 runs, got %d", count1.Load())
	}
	if count2.Load() < 2 {
		t.Errorf("job2 expected at least 2 runs, got %d", count2.Load())
	}

	statuses := s.Status()
	if len(statuses) != 2 {
		t.Fatalf("expected 2 job statuses, got %d", len(statuses))
	}
}

func TestHandlerReturnsJSON(t *testing.T) {
	s := New()
	s.Register("http-job", 1*time.Second, func(ctx context.Context) error {
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/scheduler/status", nil)
	rec := httptest.NewRecorder()

	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("expected JSON content type, got %q", ct)
	}

	var statuses []JobStatus
	if err := json.NewDecoder(rec.Body).Decode(&statuses); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Name != "http-job" {
		t.Errorf("expected name 'http-job', got %q", statuses[0].Name)
	}
}

func TestStartStopLifecycle(t *testing.T) {
	var count atomic.Int64
	s := New()
	s.Register("lifecycle", 10*time.Millisecond, func(ctx context.Context) error {
		count.Add(1)
		return nil
	})

	// Start and stop multiple times.
	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Stop()

	first := count.Load()

	s.Start()
	time.Sleep(30 * time.Millisecond)
	s.Stop()

	second := count.Load()
	if second <= first {
		t.Errorf("expected more runs after restart, first=%d, second=%d", first, second)
	}

	statuses := s.Status()
	if statuses[0].Status != "stopped" {
		t.Errorf("expected stopped status, got %q", statuses[0].Status)
	}
}
