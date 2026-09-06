package output_test

import (
	"context"
	"testing"

	"github.com/jobrunner/tempus/internal/ports/output"
)

func TestBatchOrigin_RoundTrip(t *testing.T) {
	ctx := context.Background()
	if output.IsBatchOrigin(ctx) {
		t.Fatal("plain context must not be batch-origin")
	}
	if !output.IsBatchOrigin(output.WithBatchOrigin(ctx)) {
		t.Fatal("marked context must be batch-origin")
	}
}
