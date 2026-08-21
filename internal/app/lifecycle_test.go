package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.Cache.Type = cacheTypeMemory
	cfg.Cache.Path = filepath.Join(t.TempDir(), "c.bolt")
	// Port 0 lets the OS pick a free port, so a test never collides with a
	// running instance or another test.
	cfg.Server.Host = "127.0.0.1"
	cfg.Server.Port = 0
	// config.Load merges TEMPUS_* environment variables, so every field these
	// tests assert on is pinned here: otherwise a developer machine or runner
	// with, say, TEMPUS_PROVIDERS_BIOCLIM_ENABLED=false would fail the registry
	// test for reasons that have nothing to do with the code.
	cfg.Providers.OpenMeteo.Enabled = true
	cfg.Providers.Aggregate.Enabled = true
	cfg.Providers.Bioclim.Enabled = true
	cfg.Tracing.Enabled = false
	cfg.Metrics.Enabled = false
	return cfg
}

// Run serves until the context is cancelled and then shuts down cleanly. A
// cancellation is the normal stop path (SIGINT in main), so it must not surface
// as an error.
func TestApp_RunShutsDownOnContextCancellation(t *testing.T) {
	a, err := New(testConfig(t), slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run returned %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}
}

// Run's closers must fire on the way out — the disk cache is the one that
// matters, since leaving the BoltDB file locked would block the next start.
func TestApp_RunClosesTheDiskCache(t *testing.T) {
	cfg := testConfig(t)
	cfg.Cache.Type = "disk"
	a, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- a.Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the context was cancelled")
	}

	// Re-opening the same file proves the previous handle was released.
	second, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err != nil {
		t.Fatalf("second New on the same cache path: %v", err)
	}
	for _, c := range second.closers {
		_ = c()
	}
}

func TestBuildCache(t *testing.T) {
	t.Run("memory needs no closer", func(t *testing.T) {
		cache, closer, err := buildCache(config.CacheConfig{Type: cacheTypeMemory})
		if err != nil {
			t.Fatalf("buildCache: %v", err)
		}
		if cache == nil {
			t.Error("cache is nil")
		}
		if closer != nil {
			t.Error("memory cache should not need closing")
		}
	})

	t.Run("disk returns a closer", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "c.bolt")
		cache, closer, err := buildCache(config.CacheConfig{Type: "disk", Path: path})
		if err != nil {
			t.Fatalf("buildCache: %v", err)
		}
		if cache == nil || closer == nil {
			t.Fatalf("want both cache and closer set, got cache=%v closer set=%v", cache, closer != nil)
		}
		if err := closer(); err != nil {
			t.Errorf("closer: %v", err)
		}
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected the cache file to exist: %v", err)
		}
	})

	t.Run("an unusable path is an error, not a silent fallback", func(t *testing.T) {
		// A file where a directory would have to be: BoltDB cannot create this.
		file := filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, err := buildCache(config.CacheConfig{Type: "disk", Path: filepath.Join(file, "c.bolt")})
		if err == nil {
			t.Error("want an error for an unusable cache path, got nil")
		}
	})
}

// New must surface a cache that cannot be opened rather than starting without
// one: a service silently running without its cache would hammer the upstream
// provider.
func TestNew_FailsOnUnusableCache(t *testing.T) {
	cfg := testConfig(t)
	cfg.Cache.Type = "disk"
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg.Cache.Path = filepath.Join(file, "c.bolt")

	if _, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test"); err == nil {
		t.Error("want an error from New for an unusable cache path, got nil")
	}
}

// The registry's contents are configuration-driven; this pins which providers
// each switch actually produces.
func TestBuildRegistry_RespectsConfiguration(t *testing.T) {
	cfg := testConfig(t)
	cache, closer, err := buildCache(cfg.Cache)
	if err != nil {
		t.Fatalf("buildCache: %v", err)
	}
	if closer != nil {
		t.Cleanup(func() { _ = closer() })
	}

	kinds := func(cfg *config.Config) map[string]bool {
		out := map[string]bool{}
		for _, p := range buildRegistry(cfg, cache, stoppedClock{}).Providers(context.Background()) {
			out[p.Kind] = true
		}
		return out
	}

	all := kinds(cfg)
	for _, want := range []string{"weather", "sun", "moon", "aggregate", "bioclim"} {
		if !all[want] {
			t.Errorf("default config: %s provider missing (got %v)", want, all)
		}
	}

	// Sun and moon are pure computation and stay registered whatever else is off;
	// everything that needs Open-Meteo goes away with it.
	off := testConfig(t)
	off.Providers.OpenMeteo.Enabled = false
	got := kinds(off)
	for _, want := range []string{"sun", "moon"} {
		if !got[want] {
			t.Errorf("with Open-Meteo disabled: %s should still be registered (got %v)", want, got)
		}
	}
	for _, unwanted := range []string{"weather", "aggregate", "bioclim"} {
		if got[unwanted] {
			t.Errorf("with Open-Meteo disabled: %s should not be registered (got %v)", unwanted, got)
		}
	}
}

type stoppedClock struct{}

func (stoppedClock) Now() time.Time { return time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC) }
