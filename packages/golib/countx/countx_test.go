package countx

import (
	"sync"
	"testing"
)

func TestCounter(t *testing.T) {
	c := &Counter{}
	if c.Value() != 0 {
		t.Error("should start at 0")
	}

	c.Inc()
	c.Inc()
	if c.Value() != 2 {
		t.Errorf("Value = %d", c.Value())
	}
}

func TestCounter_Add(t *testing.T) {
	c := &Counter{}
	c.Add(10)
	c.Add(5)
	if c.Value() != 15 {
		t.Errorf("Value = %d", c.Value())
	}
}

func TestCounter_Reset(t *testing.T) {
	c := &Counter{}
	c.Add(42)
	old := c.Reset()
	if old != 42 {
		t.Errorf("old = %d", old)
	}
	if c.Value() != 0 {
		t.Errorf("after reset = %d", c.Value())
	}
}

func TestCounter_Concurrent(t *testing.T) {
	c := &Counter{}
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}
	wg.Wait()

	if c.Value() != 1000 {
		t.Errorf("Value = %d, want 1000", c.Value())
	}
}

func TestGroup(t *testing.T) {
	g := NewGroup()
	g.Inc("requests")
	g.Inc("requests")
	g.Inc("errors")

	if g.Get("requests") != 2 {
		t.Errorf("requests = %d", g.Get("requests"))
	}
	if g.Get("errors") != 1 {
		t.Errorf("errors = %d", g.Get("errors"))
	}
	if g.Get("missing") != 0 {
		t.Error("missing should be 0")
	}
}

func TestGroup_Add(t *testing.T) {
	g := NewGroup()
	g.Add("bytes", 1024)
	g.Add("bytes", 512)

	if g.Get("bytes") != 1536 {
		t.Errorf("bytes = %d", g.Get("bytes"))
	}
}

func TestGroup_All(t *testing.T) {
	g := NewGroup()
	g.Inc("a")
	g.Add("b", 5)

	all := g.All()
	if all["a"] != 1 || all["b"] != 5 {
		t.Errorf("All = %v", all)
	}
}

func TestGroup_Names(t *testing.T) {
	g := NewGroup()
	g.Inc("z")
	g.Inc("a")
	g.Inc("m")

	names := g.Names()
	if len(names) != 3 || names[0] != "a" || names[2] != "z" {
		t.Errorf("Names = %v (should be sorted)", names)
	}
}

func TestGroup_Reset(t *testing.T) {
	g := NewGroup()
	g.Add("x", 10)
	g.Add("y", 20)

	old := g.Reset()
	if old["x"] != 10 || old["y"] != 20 {
		t.Errorf("old = %v", old)
	}
	if g.Get("x") != 0 || g.Get("y") != 0 {
		t.Error("should be reset to 0")
	}
}

func TestGroup_Len(t *testing.T) {
	g := NewGroup()
	g.Inc("a")
	g.Inc("b")
	if g.Len() != 2 {
		t.Errorf("Len = %d", g.Len())
	}
}

func TestGroup_Concurrent(t *testing.T) {
	g := NewGroup()
	var wg sync.WaitGroup

	for i := 0; i < 500; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			g.Inc("total")
		}()
		go func() {
			defer wg.Done()
			g.Get("total")
		}()
	}
	wg.Wait()

	if g.Get("total") != 500 {
		t.Errorf("total = %d, want 500", g.Get("total"))
	}
}
