package app

import (
	"testing"
	"time"

	"github.com/jobrunner/tempus/internal/adapters/clock"
)

func TestBuildOpenMeteoClients_SharedLimiterDistinctTimeouts(t *testing.T) {
	cfg := testConfig(t)
	c := buildOpenMeteoClients(cfg, clock.System{})
	if c.weather == nil || c.aggregate == nil || c.bioclim == nil {
		t.Fatal("all three clients must be built")
	}
	if c.weather.Timeout != cfg.Providers.OpenMeteo.Timeout {
		t.Errorf("weather timeout = %v, want %v", c.weather.Timeout, cfg.Providers.OpenMeteo.Timeout)
	}
	if c.bioclim.Timeout != 60*time.Second {
		t.Errorf("bioclim timeout = %v, want 60s (large 30-year fetch)", c.bioclim.Timeout)
	}
	if c.weather.Transport == c.aggregate.Transport {
		t.Error("each client needs its own transport (distinct weights)")
	}
}
