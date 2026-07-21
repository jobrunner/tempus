package output

import (
	"context"

	"github.com/jobrunner/tempus/internal/domain"
)

// FeatureProvider is a driven port: one external source or computation.
// Fetch returns the feature (with attribution) and whether it was cached.
type FeatureProvider interface {
	ID() string
	Kind() string
	Attribution() domain.License
	Fetch(ctx context.Context, req domain.QueryRequest) (domain.ProviderResult, error)
}
