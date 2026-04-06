// Package hashring provides consistent hashing for distributing
// keys across a set of nodes. Used for cache sharding, request
// routing, and partition assignment.
package hashring

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
	"sync"
)

// Ring is a consistent hash ring.
type Ring struct {
	mu       sync.RWMutex
	nodes    map[string]bool
	ring     []point
	replicas int
}

type point struct {
	hash uint32
	node string
}

// New creates a hash ring with the given number of virtual nodes per node.
func New(replicas int) *Ring {
	if replicas <= 0 {
		replicas = 100
	}
	return &Ring{
		nodes:    make(map[string]bool),
		replicas: replicas,
	}
}

// Add adds a node to the ring.
func (r *Ring) Add(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.nodes[node] {
		return
	}
	r.nodes[node] = true

	for i := 0; i < r.replicas; i++ {
		h := hashKey(node, i)
		r.ring = append(r.ring, point{hash: h, node: node})
	}

	sort.Slice(r.ring, func(i, j int) bool {
		return r.ring[i].hash < r.ring[j].hash
	})
}

// Remove removes a node from the ring.
func (r *Ring) Remove(node string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.nodes[node] {
		return
	}
	delete(r.nodes, node)

	newRing := make([]point, 0, len(r.ring)-r.replicas)
	for _, p := range r.ring {
		if p.node != node {
			newRing = append(newRing, p)
		}
	}
	r.ring = newRing
}

// Get returns the node responsible for the given key.
func (r *Ring) Get(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ring) == 0 {
		return ""
	}

	h := hash(key)
	idx := sort.Search(len(r.ring), func(i int) bool {
		return r.ring[i].hash >= h
	})

	if idx >= len(r.ring) {
		idx = 0 // Wrap around.
	}

	return r.ring[idx].node
}

// GetN returns the N closest nodes for a key (for replication).
func (r *Ring) GetN(key string, n int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.ring) == 0 {
		return nil
	}
	if n > len(r.nodes) {
		n = len(r.nodes)
	}

	h := hash(key)
	idx := sort.Search(len(r.ring), func(i int) bool {
		return r.ring[i].hash >= h
	})

	seen := make(map[string]bool)
	var result []string

	for i := 0; i < len(r.ring) && len(result) < n; i++ {
		pos := (idx + i) % len(r.ring)
		node := r.ring[pos].node
		if !seen[node] {
			seen[node] = true
			result = append(result, node)
		}
	}

	return result
}

// Nodes returns all registered nodes.
func (r *Ring) Nodes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	nodes := make([]string, 0, len(r.nodes))
	for n := range r.nodes {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)
	return nodes
}

// Size returns the number of nodes.
func (r *Ring) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.nodes)
}

func hash(key string) uint32 {
	h := sha256.Sum256([]byte(key))
	return binary.BigEndian.Uint32(h[:4])
}

func hashKey(node string, replica int) uint32 {
	key := node + ":" + itoa(replica)
	return hash(key)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [10]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
