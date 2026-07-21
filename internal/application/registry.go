package application

import (
	"context"

	"github.com/jobrunner/tempus/internal/ports/input"
	"github.com/jobrunner/tempus/internal/ports/output"
)

// Registry holds the registered feature providers in registration order.
type Registry struct {
	providers map[string]output.FeatureProvider
	order     []string
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{providers: map[string]output.FeatureProvider{}}
}

// Register adds a provider (last registration for an id wins; order preserved).
func (r *Registry) Register(p output.FeatureProvider) {
	if _, exists := r.providers[p.ID()]; !exists {
		r.order = append(r.order, p.ID())
	}
	r.providers[p.ID()] = p
}

// Get returns the provider with id, if registered.
func (r *Registry) Get(id string) (output.FeatureProvider, bool) {
	p, ok := r.providers[id]
	return p, ok
}

// All returns providers in registration order.
func (r *Registry) All() []output.FeatureProvider {
	out := make([]output.FeatureProvider, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.providers[id])
	}
	return out
}

// Providers implements input.ProviderLister.
func (r *Registry) Providers(context.Context) []input.ProviderInfo {
	out := make([]input.ProviderInfo, 0, len(r.order))
	for _, id := range r.order {
		p := r.providers[id]
		out = append(out, input.ProviderInfo{ID: p.ID(), Kind: p.Kind(), License: p.Attribution()})
	}
	return out
}
