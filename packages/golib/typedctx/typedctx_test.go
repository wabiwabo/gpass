package typedctx

import (
	"context"
	"testing"
)

func TestKey_SetGet(t *testing.T) {
	key := NewKey[string]("user_id")
	ctx := key.Set(context.Background(), "user-123")

	v, ok := key.Get(ctx)
	if !ok {
		t.Error("should find value")
	}
	if v != "user-123" {
		t.Errorf("value: got %q", v)
	}
}

func TestKey_GetMissing(t *testing.T) {
	key := NewKey[string]("missing")
	_, ok := key.Get(context.Background())
	if ok {
		t.Error("missing key should return false")
	}
}

func TestKey_MustGet(t *testing.T) {
	key := NewKey[int]("count")
	ctx := key.Set(context.Background(), 42)

	v := key.MustGet(ctx)
	if v != 42 {
		t.Errorf("value: got %d", v)
	}
}

func TestKey_MustGet_Panic(t *testing.T) {
	key := NewKey[string]("missing")
	defer func() {
		if r := recover(); r == nil {
			t.Error("should panic on missing key")
		}
	}()
	key.MustGet(context.Background())
}

func TestKey_GetOrDefault(t *testing.T) {
	key := NewKey[string]("lang")

	v := key.GetOrDefault(context.Background(), "id")
	if v != "id" {
		t.Errorf("default: got %q", v)
	}

	ctx := key.Set(context.Background(), "en")
	v = key.GetOrDefault(ctx, "id")
	if v != "en" {
		t.Errorf("existing: got %q", v)
	}
}

func TestKey_Name(t *testing.T) {
	key := NewKey[string]("test_key")
	if key.Name() != "test_key" {
		t.Errorf("name: got %q", key.Name())
	}
}

func TestKey_TypeIsolation(t *testing.T) {
	strKey := NewKey[string]("val")
	intKey := NewKey[int]("val")

	ctx := strKey.Set(context.Background(), "hello")
	ctx = intKey.Set(ctx, 42)

	s, ok := strKey.Get(ctx)
	if !ok || s != "hello" {
		t.Errorf("string key: got %q", s)
	}

	i, ok := intKey.Get(ctx)
	if !ok || i != 42 {
		t.Errorf("int key: got %d", i)
	}
}

func TestKey_SliceType(t *testing.T) {
	key := NewKey[[]string]("roles")
	ctx := key.Set(context.Background(), []string{"admin", "user"})

	v, ok := key.Get(ctx)
	if !ok {
		t.Error("should find value")
	}
	if len(v) != 2 || v[0] != "admin" {
		t.Errorf("roles: got %v", v)
	}
}

func TestPredefinedKeys(t *testing.T) {
	ctx := context.Background()
	ctx = UserIDKey.Set(ctx, "user-1")
	ctx = TenantIDKey.Set(ctx, "tenant-1")
	ctx = RolesKey.Set(ctx, []string{"admin"})
	ctx = LocaleKey.Set(ctx, "id-ID")

	if v, _ := UserIDKey.Get(ctx); v != "user-1" {
		t.Error("user ID")
	}
	if v, _ := TenantIDKey.Get(ctx); v != "tenant-1" {
		t.Error("tenant ID")
	}
	if v, _ := RolesKey.Get(ctx); len(v) != 1 {
		t.Error("roles")
	}
	if v, _ := LocaleKey.Get(ctx); v != "id-ID" {
		t.Error("locale")
	}
}
