package config

import (
	"testing"
	"time"
)

func TestLoadDefaultsAndEnvOverride(t *testing.T) {
	t.Setenv("TEMPUS_CACHE_TYPE", "memory")
	t.Setenv("TEMPUS_PROVIDERS_REANALYSIS_TIMEOUT", "7s")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Cache.Type != "memory" {
		t.Errorf("cache.type = %q, want memory", cfg.Cache.Type)
	}
	if cfg.Cache.Path == "" {
		t.Error("cache.path default must be set")
	}
	r := cfg.Providers.Reanalysis
	if r.Timeout != 7*time.Second {
		t.Errorf("reanalysis.timeout = %v, want 7s", r.Timeout)
	}
	if !r.Enabled || r.BaseURL != "https://openmeteo.s3.amazonaws.com" {
		t.Errorf("reanalysis defaults = %+v", r)
	}
	if r.Maturity != 72*time.Hour || r.ListTTL != time.Hour || r.MaxParallel != 32 || r.RetryAttempts != 3 {
		t.Errorf("reanalysis defaults = %+v", r)
	}
}

func TestLoad_BatchDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Query.Batch.MaxPoints; got != 10000 {
		t.Errorf("query.batch.max_points = %d, want 10000", got)
	}
	if got := cfg.Query.Batch.MaxSyncPoints; got != 1000 {
		t.Errorf("query.batch.max_sync_points = %d, want 1000", got)
	}
	if got := cfg.Query.Batch.Concurrency; got != 4 {
		t.Errorf("query.batch.concurrency = %d, want 4", got)
	}
}

func TestLoad_CORSDisabledByDefault(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.CORS.Enabled() {
		t.Errorf("CORS must be off by default, got origins %v", cfg.Server.CORS.AllowedOrigins)
	}
}

// A comma-separated env var must arrive as separate origins — viper's decoder
// hook is what makes TEMPUS_SERVER_CORS_ALLOWED_ORIGINS usable at all.
func TestLoad_CORSOriginsFromEnv(t *testing.T) {
	t.Setenv("TEMPUS_SERVER_CORS_ALLOWED_ORIGINS", "https://a.test,https://*.b.test")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := cfg.Server.CORS.AllowedOrigins
	want := []string{"https://a.test", "https://*.b.test"}
	if len(got) != len(want) {
		t.Fatalf("origins = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("origins[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if !cfg.Server.CORS.Enabled() {
		t.Error("CORS must report enabled when origins are configured")
	}
}

func TestLoad_RateLimitDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rl := cfg.Server.RateLimit
	if rl.Enabled {
		t.Error("rate limiting must be off by default")
	}
	if rl.Rate != 100.0 || rl.Burst != 200 {
		t.Errorf("rate/burst = %v/%d, want 100/200", rl.Rate, rl.Burst)
	}
	if len(rl.TrustedProxies) != 0 {
		t.Errorf("trusted_proxies = %v, want empty (never trust XFF unless configured)", rl.TrustedProxies)
	}
}

func TestLoad_RateLimitFromEnv(t *testing.T) {
	t.Setenv("TEMPUS_SERVER_RATE_LIMIT_ENABLED", "true")
	t.Setenv("TEMPUS_SERVER_RATE_LIMIT_RATE", "5.5")
	t.Setenv("TEMPUS_SERVER_RATE_LIMIT_BURST", "9")
	t.Setenv("TEMPUS_SERVER_RATE_LIMIT_TRUSTED_PROXIES", "10.0.0.0/8,192.168.0.0/16")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rl := cfg.Server.RateLimit
	if !rl.Enabled || rl.Rate != 5.5 || rl.Burst != 9 {
		t.Errorf("got enabled=%v rate=%v burst=%d, want true/5.5/9", rl.Enabled, rl.Rate, rl.Burst)
	}
	if len(rl.TrustedProxies) != 2 || rl.TrustedProxies[0] != "10.0.0.0/8" {
		t.Errorf("trusted_proxies = %#v, want the two CIDRs split apart", rl.TrustedProxies)
	}
}
