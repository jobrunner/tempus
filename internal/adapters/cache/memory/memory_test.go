package memory

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_SetGetExpire(t *testing.T) {
	c := New(0)
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

func TestMemoryCache_EvictsLeastRecentlyUsed(t *testing.T) {
	c := New(3 * entryCost("k0", 100))
	ctx := context.Background()
	for _, k := range []string{"k0", "k1", "k2"} {
		_ = c.Set(ctx, k, make([]byte, 100), 0)
	}
	_, _, _ = c.Get(ctx, "k0") // k1 is now least recently used
	_ = c.Set(ctx, "k3", make([]byte, 100), 0)

	if _, ok, _ := c.Get(ctx, "k1"); ok {
		t.Error("least recently used k1 must be evicted")
	}
	for _, k := range []string{"k0", "k2", "k3"} {
		if _, ok, _ := c.Get(ctx, k); !ok {
			t.Errorf("%s evicted", k)
		}
	}
	if c.Bytes() > 3*entryCost("k0", 100) {
		t.Errorf("bytes %d over cap", c.Bytes())
	}
}

func TestMemoryCache_AccountingFollowsOverwriteExpiryAndOversize(t *testing.T) {
	c := New(1000)
	ctx := context.Background()
	_ = c.Set(ctx, "k", make([]byte, 100), time.Hour)
	_ = c.Set(ctx, "k", make([]byte, 50), time.Hour)
	if got, want := c.Bytes(), entryCost("k", 50); got != want {
		t.Errorf("bytes after overwrite = %d, want %d", got, want)
	}
	_ = c.Set(ctx, "huge", make([]byte, 5000), 0)
	if _, ok, _ := c.Get(ctx, "huge"); ok {
		t.Error("a value larger than the cap must not be stored")
	}
	c.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("entry should have expired")
	}
	if c.Bytes() != 0 {
		t.Errorf("bytes after expiry = %d, want 0", c.Bytes())
	}
}

func TestMemoryCache_ValueExactlyAtCapIsStored(t *testing.T) {
	c := New(entryCost("k", 100))
	_ = c.Set(context.Background(), "k", make([]byte, 100), 0)
	if _, ok, _ := c.Get(context.Background(), "k"); !ok {
		t.Error("an entry exactly at the cap must be stored")
	}
}
