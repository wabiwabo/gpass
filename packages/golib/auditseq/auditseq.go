// Package auditseq provides monotonic sequence generation for
// audit trail entries. Ensures audit events are strictly ordered
// even across concurrent requests, supporting PP 71/2019 immutable
// audit requirements.
package auditseq

import (
	"fmt"
	"sync/atomic"
	"time"
)

// Generator produces monotonically increasing sequence numbers.
type Generator struct {
	counter atomic.Int64
	prefix  string
}

// New creates a sequence generator with a prefix.
func New(prefix string) *Generator {
	return &Generator{prefix: prefix}
}

// Next returns the next sequence number.
func (g *Generator) Next() int64 {
	return g.counter.Add(1)
}

// NextID returns the next sequence as a formatted string ID.
// Format: {prefix}-{timestamp}-{sequence}
func (g *Generator) NextID() string {
	seq := g.counter.Add(1)
	ts := time.Now().UTC().Format("20060102150405")
	if g.prefix != "" {
		return fmt.Sprintf("%s-%s-%06d", g.prefix, ts, seq)
	}
	return fmt.Sprintf("%s-%06d", ts, seq)
}

// Current returns the current counter value without incrementing.
func (g *Generator) Current() int64 {
	return g.counter.Load()
}

// Reset sets the counter to a specific value.
func (g *Generator) Reset(value int64) {
	g.counter.Store(value)
}

// Pair generates a pair of sequence numbers for before/after audit entries.
type Pair struct {
	Before int64
	After  int64
}

// NextPair generates a pair of consecutive sequence numbers.
func (g *Generator) NextPair() Pair {
	before := g.counter.Add(1)
	after := g.counter.Add(1)
	return Pair{Before: before, After: after}
}
