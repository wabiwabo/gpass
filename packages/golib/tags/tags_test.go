package tags

import (
	"context"
	"sync"
	"testing"
)

func TestTags_SetGet(t *testing.T) {
	tags := New()
	tags.Set("service", "identity")

	v, ok := tags.Get("service")
	if !ok || v != "identity" {
		t.Errorf("Get('service') = %q, %v", v, ok)
	}
}

func TestTags_GetMissing(t *testing.T) {
	tags := New()
	_, ok := tags.Get("nonexistent")
	if ok {
		t.Error("missing key should return false")
	}
}

func TestTags_Delete(t *testing.T) {
	tags := New()
	tags.Set("key", "value")
	tags.Delete("key")

	_, ok := tags.Get("key")
	if ok {
		t.Error("deleted key should not be found")
	}
}

func TestTags_Len(t *testing.T) {
	tags := New()
	tags.Set("a", "1")
	tags.Set("b", "2")
	if tags.Len() != 2 {
		t.Errorf("len: got %d", tags.Len())
	}
}

func TestTags_All(t *testing.T) {
	tags := New()
	tags.Set("a", "1")
	tags.Set("b", "2")

	all := tags.All()
	if len(all) != 2 {
		t.Errorf("all: got %d entries", len(all))
	}
	if all["a"] != "1" || all["b"] != "2" {
		t.Errorf("all: got %v", all)
	}

	// Modifying the returned map should not affect original.
	all["c"] = "3"
	if tags.Len() != 2 {
		t.Error("modifying All() result should not change tags")
	}
}

func TestTags_String(t *testing.T) {
	tags := New()
	tags.Set("env", "prod")
	tags.Set("app", "bff")

	s := tags.String()
	if s != "app=bff,env=prod" {
		t.Errorf("String() = %q, want 'app=bff,env=prod'", s)
	}
}

func TestTags_String_Empty(t *testing.T) {
	tags := New()
	if tags.String() != "" {
		t.Error("empty tags should return empty string")
	}
}

func TestTags_FromMap(t *testing.T) {
	tags := FromMap(map[string]string{"x": "1", "y": "2"})
	if tags.Len() != 2 {
		t.Errorf("len: got %d", tags.Len())
	}
}

func TestTags_Merge(t *testing.T) {
	t1 := FromMap(map[string]string{"a": "1", "b": "2"})
	t2 := FromMap(map[string]string{"b": "overwritten", "c": "3"})

	t1.Merge(t2)

	v, _ := t1.Get("b")
	if v != "overwritten" {
		t.Errorf("merge should overwrite: got %q", v)
	}
	if t1.Len() != 3 {
		t.Errorf("after merge len: got %d", t1.Len())
	}
}

func TestTags_Merge_Nil(t *testing.T) {
	tags := New()
	tags.Set("a", "1")
	tags.Merge(nil) // should not panic
	if tags.Len() != 1 {
		t.Error("merge nil should be no-op")
	}
}

func TestTags_Context(t *testing.T) {
	tags := New()
	tags.Set("user_id", "u123")

	ctx := ToContext(context.Background(), tags)
	got := FromContext(ctx)

	v, ok := got.Get("user_id")
	if !ok || v != "u123" {
		t.Errorf("FromContext: got %q, %v", v, ok)
	}
}

func TestTags_FromContext_Missing(t *testing.T) {
	tags := FromContext(context.Background())
	if tags.Len() != 0 {
		t.Error("missing context should return empty tags")
	}
}

func TestWithService(t *testing.T) {
	tags := New()
	WithService(tags, "identity", "1.0.0", "production")

	v, _ := tags.Get("service")
	if v != "identity" {
		t.Errorf("service: got %q", v)
	}
	v, _ = tags.Get("version")
	if v != "1.0.0" {
		t.Errorf("version: got %q", v)
	}
	v, _ = tags.Get("environment")
	if v != "production" {
		t.Errorf("env: got %q", v)
	}
}

func TestTags_ConcurrentAccess(t *testing.T) {
	tags := New()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := string(rune('a' + (n % 26)))
			tags.Set(key, "value")
			tags.Get(key)
			tags.All()
			tags.String()
			tags.Len()
		}(i)
	}
	wg.Wait()
}
