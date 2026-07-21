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
