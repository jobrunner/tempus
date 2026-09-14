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

// Fitness function: every provider the composition root registers must declare a
// COMPLETE static licence. That block is published verbatim on
// /api/v1/providers, and unlike Feature.License it is never validated at
// request time — so without this test an incomplete one would be served to
// clients and nothing would notice.
//
// Kept as a test rather than a startup check on purpose: a misconfigured
// provider is a programming error, and failing CI is more useful than failing
// to boot in production.
func TestEveryRegisteredProviderDeclaresACompleteLicense(t *testing.T) {
	cfg := testConfig(t)
	reg := buildRegistry(cfg, nil, clock.System{}, buildOpenMeteoClients(cfg, clock.System{}))

	all := reg.All()
	if len(all) == 0 {
		t.Fatal("no providers registered — this assertion would be vacuous")
	}
	for _, p := range all {
		if err := p.Attribution().Validate(); err != nil {
			t.Errorf("provider %q (%s) declares an incomplete licence: %v", p.ID(), p.Kind(), err)
		}
	}
}
