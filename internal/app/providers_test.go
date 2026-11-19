package app

import (
	"testing"

	"github.com/jobrunner/tempus/internal/adapters/clock"
)

func TestBuildReanalysis_DisabledIsNil(t *testing.T) {
	cfg := testConfig(t)
	if buildReanalysis(cfg, nil) == nil {
		t.Fatal("enabled archive must be built")
	}
	cfg.Providers.Reanalysis.Enabled = false
	if buildReanalysis(cfg, nil) != nil {
		t.Fatal("disabled archive must be nil, so its providers are not registered")
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
	reg := buildRegistry(cfg, nil, clock.System{}, buildReanalysis(cfg, nil))

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
