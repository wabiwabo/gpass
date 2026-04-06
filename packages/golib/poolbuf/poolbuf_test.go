package poolbuf

import (
	"sync"
	"testing"
)

func TestGetPut(t *testing.T) {
	p := New(256)
	buf := p.Get()
	if buf == nil {
		t.Fatal("Get returned nil")
	}
	buf.WriteString("hello")
	if buf.String() != "hello" {
		t.Errorf("got %q", buf.String())
	}
	p.Put(buf)
	// After put, buf should be reset
	buf2 := p.Get()
	if buf2.Len() != 0 {
		t.Error("reused buffer should be empty")
	}
}

func TestDefaultPool(t *testing.T) {
	buf := Get()
	if buf == nil {
		t.Fatal("Get returned nil")
	}
	buf.WriteString("test")
	Put(buf)
}

func TestConcurrentAccess(t *testing.T) {
	p := New(128)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buf := p.Get()
			buf.WriteString("concurrent")
			if buf.Len() == 0 {
				t.Error("buffer should have data")
			}
			p.Put(buf)
		}()
	}
	wg.Wait()
}

func TestBufferCapacity(t *testing.T) {
	p := New(1024)
	buf := p.Get()
	if buf.Cap() < 1024 {
		t.Errorf("initial capacity: got %d, want >= 1024", buf.Cap())
	}
	p.Put(buf)
}

func TestPutResets(t *testing.T) {
	p := New(64)
	buf := p.Get()
	buf.WriteString("data")
	p.Put(buf)
	buf = p.Get()
	if buf.Len() != 0 {
		t.Errorf("Len after reset: got %d, want 0", buf.Len())
	}
}

func BenchmarkGetPut(b *testing.B) {
	p := New(4096)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := p.Get()
		buf.WriteString("benchmark data")
		p.Put(buf)
	}
}
