package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jobrunner/tempus/internal/adapters/clock"
)

func TestBuildOpenMeteoClients_SharedLimiterDistinctTimeouts(t *testing.T) {
	cfg := testConfig(t)
	c := buildOpenMeteoClients(cfg, clock.System{}, nil)
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
	reg := buildRegistry(cfg, nil, clock.System{}, buildOpenMeteoClients(cfg, clock.System{}, nil))

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

// The TracerProvider handed to buildOpenMeteoClients must reach every client's
// transport (each call becomes a client span); nil keeps them untraced.
func TestBuildOpenMeteoClients_TracerProviderReachesEveryClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	call := func(c omClients) {
		for _, client := range []*http.Client{c.weather, c.aggregate, c.bioclim} {
			resp, err := client.Get(srv.URL)
			if err != nil {
				t.Fatalf("get: %v", err)
			}
			resp.Body.Close()
		}
	}

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	cfg := testConfig(t)
	call(buildOpenMeteoClients(cfg, clock.System{}, nil))
	if n := len(rec.Ended()); n != 0 {
		t.Fatalf("spans without a TracerProvider = %d, want 0", n)
	}
	call(buildOpenMeteoClients(cfg, clock.System{}, tp))
	if n := len(rec.Ended()); n != 3 {
		t.Fatalf("client spans = %d, want 3 (one per client)", n)
	}
}
