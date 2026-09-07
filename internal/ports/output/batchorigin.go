package output

import "context"

// batchOriginKey marks a context as originating from the batch path. The
// Open-Meteo transport enforces the daily call budget only for batch-marked
// requests, so interactive single queries are never budget-limited.
type batchOriginKey struct{}

// WithBatchOrigin marks ctx as coming from the batch path.
func WithBatchOrigin(ctx context.Context) context.Context {
	return context.WithValue(ctx, batchOriginKey{}, true)
}

// IsBatchOrigin reports whether ctx was marked via WithBatchOrigin.
func IsBatchOrigin(ctx context.Context) bool {
	v, _ := ctx.Value(batchOriginKey{}).(bool)
	return v
}
