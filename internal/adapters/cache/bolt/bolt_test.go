package bolt

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestBoltCache_Roundtrip(t *testing.T) {
	c, err := Open(filepath.Join(t.TempDir(), "c.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()

	if err := c.Set(ctx, "k", []byte("v"), time.Hour); err != nil {
		t.Fatal(err)
	}
	if v, ok, _ := c.Get(ctx, "k"); !ok || string(v) != "v" {
		t.Fatalf("miss: %q ok=%v", v, ok)
	}
	if err := c.Set(ctx, "gone", []byte("x"), -time.Hour); err != nil { // already expired
		t.Fatal(err)
	}
	if _, ok, _ := c.Get(ctx, "gone"); ok {
		t.Error("expired entry must read as absent")
	}
}
