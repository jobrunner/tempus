package output

import (
	"context"
	"time"
)

// Cache is a driven port for a byte-blob cache. ttl == 0 means "never expire".
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}
