package chanutil

import (
	"context"
	"sort"
	"testing"
	"time"
)

func TestMerge(t *testing.T) {
	ch1 := make(chan int, 2)
	ch2 := make(chan int, 2)
	ch1 <- 1
	ch1 <- 2
	close(ch1)
	ch2 <- 3
	ch2 <- 4
	close(ch2)

	merged := Merge(ch1, ch2)
	result := Collect(merged)

	sort.Ints(result)
	if len(result) != 4 {
		t.Fatalf("length: got %d, want 4", len(result))
	}
	for i, want := range []int{1, 2, 3, 4} {
		if result[i] != want {
			t.Errorf("index %d: got %d, want %d", i, result[i], want)
		}
	}
}

func TestMergeEmpty(t *testing.T) {
	merged := Merge[int]()
	result := Collect(merged)
	if len(result) != 0 {
		t.Errorf("empty merge: got %d items", len(result))
	}
}

func TestMergeSingle(t *testing.T) {
	ch := make(chan string, 1)
	ch <- "hello"
	close(ch)

	merged := Merge(ch)
	result := Collect(merged)
	if len(result) != 1 || result[0] != "hello" {
		t.Errorf("got %v", result)
	}
}

func TestOrDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	ch <- 3

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	out := OrDone(ctx, ch)
	var count int
	for range out {
		count++
	}
	// Should get some values before cancellation
	if count == 0 {
		t.Error("should have received at least one value")
	}
}

func TestOrDoneChannelClose(t *testing.T) {
	ctx := context.Background()
	ch := make(chan int, 2)
	ch <- 1
	ch <- 2
	close(ch)

	out := OrDone(ctx, ch)
	result := Collect(out)
	if len(result) != 2 {
		t.Errorf("got %d values, want 2", len(result))
	}
}

func TestWithTimeout(t *testing.T) {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	ch <- 3

	out := WithTimeout(ch, 50*time.Millisecond)
	var result []int
	for v := range out {
		result = append(result, v)
	}
	// Should get the buffered values then timeout
	if len(result) != 3 {
		t.Errorf("got %d values, want 3", len(result))
	}
}

func TestWithTimeoutExpires(t *testing.T) {
	ch := make(chan int) // unbuffered, will block
	out := WithTimeout(ch, 20*time.Millisecond)
	result := Collect(out)
	if len(result) != 0 {
		t.Errorf("should timeout with 0 items, got %d", len(result))
	}
}

func TestGenerate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	counter := 0
	out := Generate(ctx, func() int {
		counter++
		return counter
	})

	var result []int
	for v := range out {
		result = append(result, v)
		if len(result) >= 5 {
			cancel()
		}
	}
	if len(result) < 5 {
		t.Errorf("got %d values, want at least 5", len(result))
	}
}

func TestCollect(t *testing.T) {
	ch := make(chan string, 3)
	ch <- "a"
	ch <- "b"
	ch <- "c"
	close(ch)

	result := Collect(ch)
	if len(result) != 3 {
		t.Fatalf("length: got %d, want 3", len(result))
	}
	if result[0] != "a" || result[1] != "b" || result[2] != "c" {
		t.Errorf("got %v", result)
	}
}

func TestCollectEmpty(t *testing.T) {
	ch := make(chan int)
	close(ch)
	result := Collect(ch)
	if len(result) != 0 {
		t.Errorf("got %d items", len(result))
	}
}

func TestSendCtx(t *testing.T) {
	ch := make(chan int, 1)
	ctx := context.Background()
	if !SendCtx(ctx, ch, 42) {
		t.Error("should succeed")
	}
	if <-ch != 42 {
		t.Error("value mismatch")
	}
}

func TestSendCtxCancelled(t *testing.T) {
	ch := make(chan int) // unbuffered, will block
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if SendCtx(ctx, ch, 42) {
		t.Error("should fail on cancelled context")
	}
}
