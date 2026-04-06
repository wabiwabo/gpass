package worker

import (
	"context"
	"sync"
	"sync/atomic"
)

// Task represents a unit of work to be processed by the pool.
type Task struct {
	ID      string
	Payload interface{}
}

// Result represents the outcome of processing a task.
type Result struct {
	TaskID string
	Error  error
}

// PoolStats contains runtime statistics for a worker pool.
type PoolStats struct {
	Workers   int   `json:"workers"`
	Processed int64 `json:"processed"`
	Failed    int64 `json:"failed"`
	Pending   int   `json:"pending"`
	Running   bool  `json:"running"`
}

// Pool manages a fixed number of workers processing tasks from a channel.
type Pool struct {
	workers   int
	taskCh    chan Task
	resultCh  chan Result
	handler   func(ctx context.Context, task Task) error
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	ctx       context.Context
	processed atomic.Int64
	failed    atomic.Int64
	running   atomic.Bool
}

// New creates a new worker pool with the given number of workers, buffer size
// for the task channel, and a handler function that processes each task.
// The pool does not start processing until Start is called.
func New(workers int, bufferSize int, handler func(ctx context.Context, task Task) error) *Pool {
	if workers < 1 {
		workers = 1
	}
	if bufferSize < 0 {
		bufferSize = 0
	}
	return &Pool{
		workers:  workers,
		taskCh:   make(chan Task, bufferSize),
		resultCh: make(chan Result, bufferSize),
		handler:  handler,
	}
}

// Start begins processing tasks with the configured number of workers.
// The provided context controls the lifetime of all workers. When the
// context is cancelled, workers finish their current task and exit.
func (p *Pool) Start(ctx context.Context) {
	if p.running.Load() {
		return
	}
	p.ctx, p.cancel = context.WithCancel(ctx)
	p.running.Store(true)

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(p.ctx)
	}
}

func (p *Pool) worker(ctx context.Context) {
	defer p.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case task, ok := <-p.taskCh:
			if !ok {
				return
			}
			err := p.handler(ctx, task)
			p.processed.Add(1)
			if err != nil {
				p.failed.Add(1)
			}
			// Non-blocking send to result channel; drop if full.
			select {
			case p.resultCh <- Result{TaskID: task.ID, Error: err}:
			default:
			}
		}
	}
}

// Submit adds a task to the pool for processing. It returns false if the
// pool is stopped or the task buffer is full.
func (p *Pool) Submit(task Task) bool {
	if !p.running.Load() {
		return false
	}
	select {
	case p.taskCh <- task:
		return true
	default:
		return false
	}
}

// Results returns a read-only channel that receives the outcome of each
// processed task.
func (p *Pool) Results() <-chan Result {
	return p.resultCh
}

// Stop gracefully shuts down the pool. It closes the task channel, waits
// for all in-flight tasks to complete, then closes the result channel.
func (p *Pool) Stop() {
	if !p.running.CompareAndSwap(true, false) {
		return
	}
	p.cancel()
	// Drain remaining tasks so workers unblock.
	go func() {
		for range p.taskCh {
		}
	}()
	p.wg.Wait()
	close(p.taskCh)
	close(p.resultCh)
}

// Stats returns a snapshot of the pool's processing statistics.
func (p *Pool) Stats() PoolStats {
	return PoolStats{
		Workers:   p.workers,
		Processed: p.processed.Load(),
		Failed:    p.failed.Load(),
		Pending:   len(p.taskCh),
		Running:   p.running.Load(),
	}
}
