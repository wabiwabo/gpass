package hashring

import (
	"fmt"
	"testing"
)

func TestRing_GetSingleNode(t *testing.T) {
	r := New(100)
	r.Add("node-a")

	if node := r.Get("any-key"); node != "node-a" {
		t.Errorf("single node: got %q", node)
	}
}

func TestRing_GetDistribution(t *testing.T) {
	r := New(100)
	r.Add("node-a")
	r.Add("node-b")
	r.Add("node-c")

	counts := make(map[string]int)
	for i := 0; i < 1000; i++ {
		node := r.Get(fmt.Sprintf("key-%d", i))
		counts[node]++
	}

	// Each node should get at least some keys.
	for _, node := range []string{"node-a", "node-b", "node-c"} {
		if counts[node] < 100 {
			t.Errorf("%s only got %d keys (expect >100 of 1000)", node, counts[node])
		}
	}
}

func TestRing_Consistency(t *testing.T) {
	r := New(100)
	r.Add("node-a")
	r.Add("node-b")

	node1 := r.Get("my-key")
	node2 := r.Get("my-key")
	if node1 != node2 {
		t.Error("same key should always map to same node")
	}
}

func TestRing_AddNode_MinimalRebalance(t *testing.T) {
	r := New(100)
	r.Add("node-a")
	r.Add("node-b")

	// Record assignments.
	before := make(map[string]string)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		before[key] = r.Get(key)
	}

	// Add a third node.
	r.Add("node-c")

	moved := 0
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		if r.Get(key) != before[key] {
			moved++
		}
	}

	// With consistent hashing, most keys should stay.
	if moved > 50 {
		t.Errorf("too many keys moved: %d/100", moved)
	}
}

func TestRing_Remove(t *testing.T) {
	r := New(100)
	r.Add("node-a")
	r.Add("node-b")
	r.Remove("node-b")

	// All keys should now go to node-a.
	for i := 0; i < 10; i++ {
		if r.Get(fmt.Sprintf("key-%d", i)) != "node-a" {
			t.Error("after removing node-b, all should go to node-a")
		}
	}
}

func TestRing_Empty(t *testing.T) {
	r := New(100)
	if node := r.Get("key"); node != "" {
		t.Error("empty ring should return empty string")
	}
}

func TestRing_GetN(t *testing.T) {
	r := New(100)
	r.Add("node-a")
	r.Add("node-b")
	r.Add("node-c")

	nodes := r.GetN("my-key", 2)
	if len(nodes) != 2 {
		t.Errorf("GetN: got %d nodes", len(nodes))
	}
	// All should be unique.
	if nodes[0] == nodes[1] {
		t.Error("GetN should return unique nodes")
	}
}

func TestRing_GetN_MoreThanNodes(t *testing.T) {
	r := New(100)
	r.Add("node-a")
	r.Add("node-b")

	nodes := r.GetN("key", 5)
	if len(nodes) != 2 {
		t.Errorf("should cap at node count: got %d", len(nodes))
	}
}

func TestRing_Nodes(t *testing.T) {
	r := New(100)
	r.Add("node-c")
	r.Add("node-a")
	r.Add("node-b")

	nodes := r.Nodes()
	if len(nodes) != 3 {
		t.Errorf("nodes: got %d", len(nodes))
	}
	// Should be sorted.
	if nodes[0] != "node-a" {
		t.Error("should be sorted")
	}
}

func TestRing_Size(t *testing.T) {
	r := New(100)
	r.Add("a")
	r.Add("b")

	if r.Size() != 2 {
		t.Errorf("size: got %d", r.Size())
	}
}

func TestRing_DuplicateAdd(t *testing.T) {
	r := New(100)
	r.Add("node-a")
	r.Add("node-a") // Should be idempotent.

	if r.Size() != 1 {
		t.Errorf("duplicate add: got %d", r.Size())
	}
}

func TestRing_RemoveNonExistent(t *testing.T) {
	r := New(100)
	r.Remove("nonexistent") // Should not panic.
}
