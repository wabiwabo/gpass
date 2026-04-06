// Package taskqueue provides an in-memory task queue with
// configurable workers, priority support, and retry logic.
// Designed for background job processing within a service.
package taskqueue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Task represents a unit of work.
type Task struct {
	ID       string
	Payload  interface{}
	Priority int
	Retries  int
	MaxRetry int
	Created  time.Time
}

// Handler processes a task.
type Handler func(ctx context.Context, task Task) error

// Queue is an in-memory task queue.
type Queue struct {
	mu       sync.Mutex
	tasks    []Task
	handler  Handler
	workers  int
	running  atomic.Bool
	stop     chan struct{}
	wg       sync.WaitGroup
	processed atomic.Int64
	failed    atomic.Int64
}

// New creates a task queue.
func New(handler Handler, workers int) *Queue {
	if workers <= 0 {
		workers = 1
	}
	return &Queue{
		handler: handler,
		workers: workers,
		stop:    make(chan struct{}),
	}
}

// Enqueue adds a task to the queue.
func (q *Queue) Enqueue(task Task) {
	if task.Created.IsZero() {
		task.Created = time.Now()
	}
	q.mu.Lock()
	q.tasks = append(q.tasks, task)
	q.mu.Unlock()
}

// Start begins processing tasks.
func (q *Queue) Start(ctx context.Context) {
	if q.running.Load() {
		return
	}
	q.running.Store(true)

	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(ctx)
	}
}

// Stop gracefully stops all workers.
func (q *Queue) Stop() {
	if !q.running.Load() {
		return
	}
	q.running.Store(false)
	close(q.stop)
	q.wg.Wait()
}

// Pending returns the number of pending tasks.
func (q *Queue) Pending() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.tasks)
}

// Stats returns queue statistics.
type Stats struct {
	Pending   int   `json:"pending"`
	Processed int64 `json:"processed"`
	Failed    int64 `json:"failed"`
	Workers   int   `json:"workers"`
	Running   bool  `json:"running"`
}

func (q *Queue) Stats() Stats {
	return Stats{
		Pending:   q.Pending(),
		Processed: q.processed.Load(),
		Failed:    q.failed.Load(),
		Workers:   q.workers,
		Running:   q.running.Load(),
	}
}

func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()

	for {
		select {
		case <-q.stop:
			return
		case <-ctx.Done():
			return
		default:
		}

		task, ok := q.dequeue()
		if !ok {
			time.Sleep(10 * time.Millisecond)
			continue
		}

		if err := q.handler(ctx, task); err != nil {
			if task.Retries < task.MaxRetry {
				task.Retries++
				q.Enqueue(task)
			} else {
				q.failed.Add(1)
			}
		} else {
			q.processed.Add(1)
		}
	}
}

func (q *Queue) dequeue() (Task, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.tasks) == 0 {
		return Task{}, false
	}

	// Find highest priority (lowest number)
	bestIdx := 0
	for i := 1; i < len(q.tasks); i++ {
		if q.tasks[i].Priority < q.tasks[bestIdx].Priority {
			bestIdx = i
		}
	}

	task := q.tasks[bestIdx]
	q.tasks = append(q.tasks[:bestIdx], q.tasks[bestIdx+1:]...)
	return task, true
}
