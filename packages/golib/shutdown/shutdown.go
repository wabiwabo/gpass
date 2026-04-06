package shutdown

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Hook is a function to run during shutdown.
type Hook struct {
	Name     string
	Priority int // lower = runs first
	Fn       func(ctx context.Context) error
}

// Coordinator manages graceful shutdown of multiple components.
type Coordinator struct {
	mu     sync.Mutex
	hooks  []Hook
	logger *slog.Logger
}

// NewCoordinator creates a new shutdown coordinator.
func NewCoordinator(logger *slog.Logger) *Coordinator {
	if logger == nil {
		logger = slog.Default()
	}
	return &Coordinator{logger: logger}
}

// Register adds a shutdown hook.
func (c *Coordinator) Register(hook Hook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hooks = append(c.hooks, hook)
}

// RegisterFunc is a convenience method for registering a simple shutdown function.
func (c *Coordinator) RegisterFunc(name string, priority int, fn func(ctx context.Context) error) {
	c.Register(Hook{Name: name, Priority: priority, Fn: fn})
}

// WaitForSignal blocks until SIGINT or SIGTERM is received, then runs shutdown hooks.
// Returns after all hooks complete or timeout is reached.
func (c *Coordinator) WaitForSignal(timeout time.Duration) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	sig := <-quit
	c.logger.Info("shutdown signal received", "signal", sig.String())

	c.Shutdown(timeout)
}

// Shutdown runs all registered hooks in priority order with the given timeout.
func (c *Coordinator) Shutdown(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	c.mu.Lock()
	hooks := make([]Hook, len(c.hooks))
	copy(hooks, c.hooks)
	c.mu.Unlock()

	// Sort by priority (lower first).
	sortHooks(hooks)

	// Group by priority level for parallel execution within same level.
	groups := groupByPriority(hooks)

	for _, group := range groups {
		var wg sync.WaitGroup
		for _, hook := range group {
			wg.Add(1)
			go func(h Hook) {
				defer wg.Done()
				start := time.Now()
				c.logger.Info("running shutdown hook", "name", h.Name, "priority", h.Priority)

				if err := h.Fn(ctx); err != nil {
					c.logger.Error("shutdown hook failed", "name", h.Name, "error", err, "duration", time.Since(start))
				} else {
					c.logger.Info("shutdown hook completed", "name", h.Name, "duration", time.Since(start))
				}
			}(hook)
		}
		wg.Wait()

		// Check if context expired between groups.
		if ctx.Err() != nil {
			c.logger.Warn("shutdown timeout reached, aborting remaining hooks")
			return
		}
	}
}

// Hooks returns the number of registered hooks.
func (c *Coordinator) Hooks() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.hooks)
}

func sortHooks(hooks []Hook) {
	for i := 1; i < len(hooks); i++ {
		for j := i; j > 0 && hooks[j].Priority < hooks[j-1].Priority; j-- {
			hooks[j], hooks[j-1] = hooks[j-1], hooks[j]
		}
	}
}

func groupByPriority(hooks []Hook) [][]Hook {
	if len(hooks) == 0 {
		return nil
	}

	var groups [][]Hook
	currentPriority := hooks[0].Priority
	var currentGroup []Hook

	for _, h := range hooks {
		if h.Priority != currentPriority {
			groups = append(groups, currentGroup)
			currentGroup = nil
			currentPriority = h.Priority
		}
		currentGroup = append(currentGroup, h)
	}
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups
}
