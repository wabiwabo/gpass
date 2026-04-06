package scheduler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Job represents a scheduled task.
type Job struct {
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context) error
	LastRun  time.Time
	RunCount int64
	Errors   int64
}

// JobStatus describes the current state of a job.
type JobStatus struct {
	Name     string `json:"name"`
	Interval string `json:"interval"`
	LastRun  string `json:"last_run"`
	RunCount int64  `json:"run_count"`
	Errors   int64  `json:"errors"`
	Status   string `json:"status"`
}

// Scheduler manages periodic background jobs.
type Scheduler struct {
	jobs    []*Job
	mu      sync.RWMutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
}

// New creates a new Scheduler.
func New() *Scheduler {
	return &Scheduler{}
}

// Register adds a job to the scheduler. Must be called before Start.
func (s *Scheduler) Register(name string, interval time.Duration, fn func(ctx context.Context) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, &Job{
		Name:     name,
		Interval: interval,
		Fn:       fn,
	})
}

// Start begins executing all registered jobs.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	jobs := make([]*Job, len(s.jobs))
	copy(jobs, s.jobs)
	s.mu.Unlock()

	for _, job := range jobs {
		s.wg.Add(1)
		go s.runJob(ctx, job)
	}
}

func (s *Scheduler) runJob(ctx context.Context, job *Job) {
	defer s.wg.Done()

	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()

	// Run immediately on start.
	s.executeJob(ctx, job)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.executeJob(ctx, job)
		}
	}
}

func (s *Scheduler) executeJob(ctx context.Context, job *Job) {
	err := job.Fn(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	job.RunCount++
	job.LastRun = time.Now()
	if err != nil {
		job.Errors++
		slog.Error("scheduler job failed", "job", job.Name, "error", err)
	}
}

// Stop gracefully stops all jobs.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	cancel := s.cancel
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

// Status returns the status of all jobs.
func (s *Scheduler) Status() []JobStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status string
	if s.running {
		status = "running"
	} else {
		status = "stopped"
	}

	result := make([]JobStatus, len(s.jobs))
	for i, job := range s.jobs {
		lastRun := ""
		if !job.LastRun.IsZero() {
			lastRun = job.LastRun.Format(time.RFC3339)
		}
		result[i] = JobStatus{
			Name:     job.Name,
			Interval: job.Interval.String(),
			LastRun:  lastRun,
			RunCount: job.RunCount,
			Errors:   job.Errors,
			Status:   status,
		}
	}
	return result
}

// Handler returns an HTTP handler that serves job status as JSON.
func (s *Scheduler) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(s.Status())
	}
}
