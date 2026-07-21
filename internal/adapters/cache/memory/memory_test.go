package memory

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_SetGetExpire(t *testing.T) {
	c := New()
	ctx := context.Background()
	if err := c.Set(ctx, "k", []byte("v"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := c.Get(ctx, "k"); !ok || string(v) != "v" {
		t.Fatalf("get miss: %q ok=%v", v, ok)
	}
	c.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Error("entry should have expired")
	}
	// ttl 0 = permanent
	_ = c.Set(ctx, "p", []byte("x"), 0)
	c.now = func() time.Time { return time.Now().Add(1000 * time.Hour) }
	if _, ok, _ := c.Get(ctx, "p"); !ok {
		t.Error("ttl=0 entry must not expire")
	}
}
