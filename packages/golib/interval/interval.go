// Package interval provides periodic task execution with jitter,
// error handling, and graceful shutdown. Designed for background
// maintenance tasks like cache cleanup, health checks, and metric publishing.
package interval

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Task defines a periodic task.
type Task struct {
	Name     string
	Interval time.Duration
	Jitter   time.Duration // Random jitter added to interval.
	Fn       func(ctx context.Context) error
}

// Runner manages periodic task execution.
type Runner struct {
	mu    sync.Mutex
	tasks []Task
	wg    sync.WaitGroup
	cancel context.CancelFunc
	running atomic.Bool
	stats  sync.Map // name → *TaskStats
}

// TaskStats holds execution statistics for a task.
type TaskStats struct {
	Runs      atomic.Int64
	Errors    atomic.Int64
	LastRun   atomic.Value // time.Time
	LastError atomic.Value // string
}

// NewRunner creates a new interval runner.
func NewRunner() *Runner {
	return &Runner{}
}

// Add registers a periodic task.
func (r *Runner) Add(task Task) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if task.Interval <= 0 {
		task.Interval = time.Minute
	}
	r.tasks = append(r.tasks, task)
	r.stats.Store(task.Name, &TaskStats{})
}

// Start begins executing all registered tasks.
func (r *Runner) Start(ctx context.Context) {
	if r.running.Swap(true) {
		return // Already running.
	}

	ctx, r.cancel = context.WithCancel(ctx)

	r.mu.Lock()
	tasks := make([]Task, len(r.tasks))
	copy(tasks, r.tasks)
	r.mu.Unlock()

	for _, task := range tasks {
		r.wg.Add(1)
		go r.runTask(ctx, task)
	}
}

// Stop stops all tasks and waits for completion.
func (r *Runner) Stop() {
	if !r.running.Load() {
		return
	}
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	r.running.Store(false)
}

func (r *Runner) runTask(ctx context.Context, task Task) {
	defer r.wg.Done()

	statsVal, _ := r.stats.Load(task.Name)
	stats := statsVal.(*TaskStats)

	for {
		interval := task.Interval
		if task.Jitter > 0 {
			jitter := time.Duration(rand.Int63n(int64(task.Jitter)))
			interval += jitter
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			stats.Runs.Add(1)
			stats.LastRun.Store(time.Now())

			if err := task.Fn(ctx); err != nil {
				stats.Errors.Add(1)
				stats.LastError.Store(err.Error())
			}
		}
	}
}

// Stats returns statistics for a named task.
func (r *Runner) Stats(name string) (runs, errors int64) {
	val, ok := r.stats.Load(name)
	if !ok {
		return 0, 0
	}
	s := val.(*TaskStats)
	return s.Runs.Load(), s.Errors.Load()
}

// IsRunning returns whether the runner is active.
func (r *Runner) IsRunning() bool {
	return r.running.Load()
}

// TaskCount returns the number of registered tasks.
func (r *Runner) TaskCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.tasks)
}
