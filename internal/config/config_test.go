package config

import (
	"testing"
	"time"
)

func TestLoadDefaultsAndEnvOverride(t *testing.T) {
	t.Setenv("TEMPUS_CACHE_TYPE", "memory")
	t.Setenv("TEMPUS_PROVIDERS_OPENMETEO_TIMEOUT", "7s")

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
	if cfg.Providers.OpenMeteo.Timeout != 7*time.Second {
		t.Errorf("openmeteo.timeout = %v, want 7s", cfg.Providers.OpenMeteo.Timeout)
	}
	if !cfg.Providers.OpenMeteo.Enabled {
		t.Error("openmeteo enabled default must be true")
	}
	if cfg.Providers.OpenMeteo.ArchiveDelay != 5*24*time.Hour {
		t.Errorf("archiveDelay = %v, want 120h", cfg.Providers.OpenMeteo.ArchiveDelay)
	}
}

func TestLoad_BatchAndThrottleDefaults(t *testing.T) {
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
	om := cfg.Providers.OpenMeteo
	if om.RatePerMinute != 500 || om.RetryAttempts != 3 || om.DailyBudget != 8000 {
		t.Errorf("openmeteo throttle defaults = %+v, want 500/3/8000", om)
	}
	if om.Weights.Weather != 1 || om.Weights.Aggregate != 2 || om.Weights.Bioclim != 30 {
		t.Errorf("openmeteo weights = %+v, want 1/2/30", om.Weights)
	}
}
