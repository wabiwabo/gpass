// Package contextutil provides context helpers for deadline management,
// value extraction, and common context patterns used across services.
package contextutil

import (
	"context"
	"time"
)

// RemainingTime returns how much time is left before the context deadline.
// Returns 0 if no deadline is set.
func RemainingTime(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// HasDeadline checks if the context has a deadline set.
func HasDeadline(ctx context.Context) bool {
	_, ok := ctx.Deadline()
	return ok
}

// IsExpired checks if the context has already expired.
func IsExpired(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// WithMinTimeout ensures the context has at least the given timeout.
// If the existing deadline is farther than minTimeout, the original
// context is returned unchanged. Otherwise, a new context with
// minTimeout is returned.
func WithMinTimeout(ctx context.Context, minTimeout time.Duration) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if ok && time.Until(deadline) >= minTimeout {
		return ctx, func() {} // No-op cancel.
	}
	return context.WithTimeout(ctx, minTimeout)
}

// WithMaxTimeout ensures the context timeout doesn't exceed maxTimeout.
func WithMaxTimeout(ctx context.Context, maxTimeout time.Duration) (context.Context, context.CancelFunc) {
	deadline, ok := ctx.Deadline()
	if ok && time.Until(deadline) <= maxTimeout {
		return ctx, func() {} // Already within bounds.
	}
	return context.WithTimeout(ctx, maxTimeout)
}

// SplitTimeout divides the remaining context time into n equal parts.
// Useful for giving each step in a multi-step operation a fair share.
func SplitTimeout(ctx context.Context, n int) time.Duration {
	if n <= 0 {
		n = 1
	}
	remaining := RemainingTime(ctx)
	if remaining <= 0 {
		return 0
	}
	return remaining / time.Duration(n)
}

// Detach creates a new context without the parent's cancellation or deadline
// but carrying all the parent's values. Useful for background tasks that
// should outlive the request.
func Detach(ctx context.Context) context.Context {
	return detachedCtx{ctx}
}

type detachedCtx struct {
	parent context.Context
}

func (d detachedCtx) Deadline() (time.Time, bool)       { return time.Time{}, false }
func (d detachedCtx) Done() <-chan struct{}              { return nil }
func (d detachedCtx) Err() error                        { return nil }
func (d detachedCtx) Value(key interface{}) interface{} { return d.parent.Value(key) }

// Merge combines two contexts: cancellation from ctx1, values from ctx2.
func Merge(ctx1, ctx2 context.Context) context.Context {
	return mergedCtx{ctx1: ctx1, ctx2: ctx2}
}

type mergedCtx struct {
	ctx1 context.Context // Provides cancellation.
	ctx2 context.Context // Provides values.
}

func (m mergedCtx) Deadline() (time.Time, bool)       { return m.ctx1.Deadline() }
func (m mergedCtx) Done() <-chan struct{}              { return m.ctx1.Done() }
func (m mergedCtx) Err() error                        { return m.ctx1.Err() }
func (m mergedCtx) Value(key interface{}) interface{} {
	if v := m.ctx1.Value(key); v != nil {
		return v
	}
	return m.ctx2.Value(key)
}
