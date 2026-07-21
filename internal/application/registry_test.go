package application

import (
	"context"
	"testing"

	"github.com/jobrunner/tempus/internal/domain"
)

type stubProvider struct{ id, kind string }

func (s stubProvider) ID() string                 { return s.id }
func (s stubProvider) Kind() string               { return s.kind }
func (s stubProvider) Attribution() domain.License { return domain.License{Name: s.id} }
func (s stubProvider) Fetch(context.Context, domain.QueryRequest) (domain.ProviderResult, error) {
	return domain.ProviderResult{}, nil
}

func TestRegistryOrderAndLookup(t *testing.T) {
	r := NewRegistry()
	r.Register(stubProvider{"open-meteo", "weather"})
	r.Register(stubProvider{"astro", "astronomy"})

	if got := r.All(); len(got) != 2 || got[0].ID() != "open-meteo" || got[1].ID() != "astro" {
		t.Fatalf("All() order wrong: %v", got)
	}
	if _, ok := r.Get("astro"); !ok {
		t.Error("Get(astro) missing")
	}
	if _, ok := r.Get("nope"); ok {
		t.Error("Get(nope) should be absent")
	}
	infos := r.Providers(context.Background())
	if len(infos) != 2 || infos[0].License.Name != "open-meteo" {
		t.Fatalf("Providers() wrong: %v", infos)
	}
}
