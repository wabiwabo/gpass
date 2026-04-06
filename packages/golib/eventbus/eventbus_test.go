package eventbus

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	b := New()
	if len(b.Topics()) != 0 {
		t.Error("should have no topics")
	}
}

func TestPublish_Subscribe(t *testing.T) {
	b := New()
	var received interface{}

	b.Subscribe("user.created", func(event interface{}) {
		received = event
	})

	b.Publish("user.created", map[string]string{"id": "123"})

	if received == nil {
		t.Fatal("should receive event")
	}
	m := received.(map[string]string)
	if m["id"] != "123" {
		t.Errorf("id = %q", m["id"])
	}
}

func TestPublish_MultipleSubscribers(t *testing.T) {
	b := New()
	var count atomic.Int32

	b.Subscribe("event", func(e interface{}) { count.Add(1) })
	b.Subscribe("event", func(e interface{}) { count.Add(1) })
	b.Subscribe("event", func(e interface{}) { count.Add(1) })

	b.Publish("event", "data")

	if count.Load() != 3 {
		t.Errorf("count = %d, want 3", count.Load())
	}
}

func TestPublish_NoSubscribers(t *testing.T) {
	b := New()
	// Should not panic
	b.Publish("no-subscribers", "data")
}

func TestPublish_DifferentTopics(t *testing.T) {
	b := New()
	var topic1, topic2 bool

	b.Subscribe("topic1", func(e interface{}) { topic1 = true })
	b.Subscribe("topic2", func(e interface{}) { topic2 = true })

	b.Publish("topic1", nil)

	if !topic1 {
		t.Error("topic1 should fire")
	}
	if topic2 {
		t.Error("topic2 should not fire")
	}
}

func TestPublishAsync(t *testing.T) {
	b := New()
	var count atomic.Int32
	done := make(chan struct{})

	b.Subscribe("async", func(e interface{}) {
		count.Add(1)
		if count.Load() == 3 {
			close(done)
		}
	})
	b.Subscribe("async", func(e interface{}) {
		count.Add(1)
		if count.Load() == 3 {
			close(done)
		}
	})
	b.Subscribe("async", func(e interface{}) {
		count.Add(1)
		if count.Load() == 3 {
			close(done)
		}
	})

	b.PublishAsync("async", "data")

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("async handlers should complete")
	}
}

func TestHasSubscribers(t *testing.T) {
	b := New()
	if b.HasSubscribers("topic") {
		t.Error("should have no subscribers")
	}

	b.Subscribe("topic", func(e interface{}) {})
	if !b.HasSubscribers("topic") {
		t.Error("should have subscribers")
	}
}

func TestSubscriberCount(t *testing.T) {
	b := New()
	b.Subscribe("topic", func(e interface{}) {})
	b.Subscribe("topic", func(e interface{}) {})

	if b.SubscriberCount("topic") != 2 {
		t.Errorf("count = %d", b.SubscriberCount("topic"))
	}
	if b.SubscriberCount("missing") != 0 {
		t.Error("missing should be 0")
	}
}

func TestTopics(t *testing.T) {
	b := New()
	b.Subscribe("a", func(e interface{}) {})
	b.Subscribe("b", func(e interface{}) {})

	topics := b.Topics()
	if len(topics) != 2 {
		t.Errorf("topics = %d", len(topics))
	}
}

func TestClear(t *testing.T) {
	b := New()
	b.Subscribe("topic", func(e interface{}) {})
	b.Clear()

	if b.HasSubscribers("topic") {
		t.Error("should be cleared")
	}
}

func TestConcurrent_Subscribe_Publish(t *testing.T) {
	b := New()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			b.Subscribe("topic", func(e interface{}) {})
		}()
		go func() {
			defer wg.Done()
			b.Publish("topic", "data")
		}()
	}
	wg.Wait()
}
