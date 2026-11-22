package bolt

import (
	"context"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

// fakeClock is a settable clock for the access-time bookkeeping.
type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time          { return f.t }
func (f *fakeClock) advance(d time.Duration) { f.t = f.t.Add(d) }

func openCapped(t *testing.T, path string, maxBytes int64, clk *fakeClock) *Cache {
	t.Helper()
	c, err := Open(path, Options{MaxBytes: maxBytes, Now: clk.now})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func value(n int) []byte { return make([]byte, n) }

func TestBoltCache_StaysUnderMaxBytes(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := openCapped(t, filepath.Join(t.TempDir(), "c.bolt"), 10_000, clk)
	defer c.Close()
	ctx := context.Background()

	for i := range 50 {
		clk.advance(time.Second)
		if err := c.Set(ctx, fmt.Sprintf("k%02d", i), value(1000), time.Hour); err != nil {
			t.Fatal(err)
		}
		if got := c.Bytes(); got > 10_000 {
			t.Fatalf("after %d sets: %d bytes > cap 10000", i+1, got)
		}
	}
	if _, ok, _ := c.Get(ctx, "k49"); !ok {
		t.Error("newest entry must survive eviction")
	}
	if _, ok, _ := c.Get(ctx, "k00"); ok {
		t.Error("oldest entry must be evicted")
	}
}

func TestBoltCache_ReadEntrySurvivesEviction(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := openCapped(t, filepath.Join(t.TempDir(), "c.bolt"), 5_000, clk)
	defer c.Close()
	ctx := context.Background()

	for i := range 4 {
		clk.advance(time.Second)
		_ = c.Set(ctx, fmt.Sprintf("k%d", i), value(1000), 0)
	}
	// Reading k0 after the touch interval makes it the most recently used.
	clk.advance(2 * touchInterval)
	if _, ok, _ := c.Get(ctx, "k0"); !ok {
		t.Fatal("k0 missing before eviction")
	}
	for i := 4; i < 7; i++ {
		clk.advance(time.Second)
		_ = c.Set(ctx, fmt.Sprintf("k%d", i), value(1000), 0)
	}
	if _, ok, _ := c.Get(ctx, "k0"); !ok {
		t.Error("recently read k0 was evicted")
	}
	if _, ok, _ := c.Get(ctx, "k1"); ok {
		t.Error("least recently used k1 must be evicted")
	}
}

func TestBoltCache_OverwriteAndExpiryKeepAccounting(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := openCapped(t, filepath.Join(t.TempDir(), "c.bolt"), 0, clk)
	defer c.Close()
	ctx := context.Background()

	_ = c.Set(ctx, "k", value(100), time.Hour)
	first := c.Bytes()
	if first <= 100 {
		t.Fatalf("bytes = %d, want > 100 (value plus key and header)", first)
	}
	_ = c.Set(ctx, "k", value(100), time.Hour)
	if c.Bytes() != first {
		t.Errorf("overwrite changed bytes %d -> %d", first, c.Bytes())
	}
	clk.advance(2 * time.Hour)
	if _, ok, _ := c.Get(ctx, "k"); ok {
		t.Fatal("expired entry must read as absent")
	}
	if c.Bytes() != 0 {
		t.Errorf("bytes after expiry = %d, want 0", c.Bytes())
	}
}

func TestBoltCache_OversizedValueIsNotStored(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := openCapped(t, filepath.Join(t.TempDir(), "c.bolt"), 1_000, clk)
	defer c.Close()
	ctx := context.Background()

	_ = c.Set(ctx, "small", value(100), time.Hour)
	if err := c.Set(ctx, "huge", value(5_000), time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := c.Get(ctx, "huge"); ok {
		t.Error("a value larger than the cap must not be stored")
	}
	if _, ok, _ := c.Get(ctx, "small"); !ok {
		t.Error("an oversized value must not evict the rest")
	}
}

func TestBoltCache_ReopenKeepsAccounting(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	path := filepath.Join(t.TempDir(), "c.bolt")
	c := openCapped(t, path, 0, clk)
	_ = c.Set(context.Background(), "k", value(100), 0)
	want := c.Bytes()
	_ = c.Close()

	c = openCapped(t, path, 0, clk)
	defer c.Close()
	if c.Bytes() != want {
		t.Errorf("bytes after reopen = %d, want %d", c.Bytes(), want)
	}
	if _, ok, _ := c.Get(context.Background(), "k"); !ok {
		t.Error("entry lost on reopen")
	}
}

// TestBoltCache_DropsLegacyFormat: files written before the size cap hold
// entries without access time or index. They are a cache, so they are
// discarded on open instead of being misread.
func TestBoltCache_DropsLegacyFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.bolt")
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Update(func(tx *bolt.Tx) error {
		b, _ := tx.CreateBucketIfNotExists([]byte("cache"))
		raw := make([]byte, 8+1)
		binary.BigEndian.PutUint64(raw, 0)
		return b.Put([]byte("old"), raw)
	})
	_ = db.Close()

	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := openCapped(t, path, 0, clk)
	defer c.Close()
	if _, ok, _ := c.Get(context.Background(), "old"); ok {
		t.Error("legacy entry must be dropped")
	}
	if c.Bytes() != 0 {
		t.Errorf("bytes = %d, want 0", c.Bytes())
	}
}

func TestBoltCache_ValueExactlyAtCapIsStored(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := openCapped(t, filepath.Join(t.TempDir(), "c.bolt"), entryCost([]byte("k"), 100), clk)
	defer c.Close()
	_ = c.Set(context.Background(), "k", value(100), 0)
	if _, ok, _ := c.Get(context.Background(), "k"); !ok {
		t.Error("an entry exactly at the cap must be stored")
	}
}

func TestBoltCache_ReopenWithLowerCapTrims(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	path := filepath.Join(t.TempDir(), "c.bolt")
	c := openCapped(t, path, 0, clk)
	for i := range 10 {
		clk.advance(time.Second)
		_ = c.Set(context.Background(), fmt.Sprintf("k%d", i), value(1000), 0)
	}
	_ = c.Close()

	c = openCapped(t, path, 5_000, clk)
	defer c.Close()
	if got := c.Bytes(); got > 5_000 {
		t.Errorf("bytes after reopen with a lower cap = %d, want <= 5000", got)
	}
	if _, ok, _ := c.Get(context.Background(), "k9"); !ok {
		t.Error("most recent entry must survive the trim")
	}
}

// TestBoltCache_EntryAtCapEvictsOthersNotItself: an entry exactly at the cap
// exceeds the 90 % target on its own; it must evict the rest, not itself.
func TestBoltCache_EntryAtCapEvictsOthersNotItself(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := openCapped(t, filepath.Join(t.TempDir(), "c.bolt"), 1_000, clk)
	defer c.Close()
	ctx := context.Background()
	_ = c.Set(ctx, "old", value(100), 0)
	clk.advance(time.Second)
	_ = c.Set(ctx, "k", value(int(1_000-entryCost([]byte("k"), 0))), 0)
	if _, ok, _ := c.Get(ctx, "k"); !ok {
		t.Error("the entry just written must be kept")
	}
	if _, ok, _ := c.Get(ctx, "old"); ok {
		t.Error("older entry must make room")
	}
	if got := c.Bytes(); got != 1_000 {
		t.Errorf("bytes = %d, want 1000", got)
	}
}

// TestBoltCache_StaleTouchDoesNotRewind: a touch computed from an older
// clock reading must not move the access time backwards.
func TestBoltCache_StaleTouchDoesNotRewind(t *testing.T) {
	clk := &fakeClock{t: time.Unix(1_700_000_000, 0)}
	c := openCapped(t, filepath.Join(t.TempDir(), "c.bolt"), 0, clk)
	defer c.Close()
	_ = c.Set(context.Background(), "k", value(10), 0)
	setAt := clk.t.UnixNano()

	_ = c.db.Update(func(tx *bolt.Tx) error { return touch(tx, []byte("k"), setAt-int64(2*touchInterval)) })
	_ = c.db.View(func(tx *bolt.Tx) error {
		if got := decodeHeader(tx.Bucket(entriesBucket).Get([]byte("k"))).atime; got != setAt {
			t.Errorf("atime = %d, want %d (not rewound)", got, setAt)
		}
		return nil
	})
}
