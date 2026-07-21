// Package input holds the driving ports the HTTP adapter depends on.
package input

import (
	"context"

	"github.com/jobrunner/tempus/internal/domain"
)

// FeatureService is the primary business port the HTTP adapter calls.
type FeatureService interface {
	Query(ctx context.Context, req domain.QueryRequest) (domain.QueryResult, error)
}

// ProviderInfo describes an available provider for GET /api/v1/providers.
type ProviderInfo struct {
	ID      string         `json:"id"`
	Kind    string         `json:"kind"`
	License domain.License `json:"license"`
}

// ProviderLister lists the registered providers and their attribution.
type ProviderLister interface {
	Providers(ctx context.Context) []ProviderInfo
}

// HealthChecker backs the readiness probe.
type HealthChecker interface {
	Ready(ctx context.Context) bool
}
