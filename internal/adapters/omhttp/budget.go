// Package omhttp provides the shared HTTP transport for every Open-Meteo call:
// a token-bucket rate limit, transparent retry honoring Retry-After, and a
// weighted daily call budget enforced only for batch-originated requests.
// Adapters receive it as a plain *http.Client, so no adapter imports this
// package (composition happens in internal/app).
package omhttp

import (
	"sync"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// Budget is a weighted daily call counter, resetting at the UTC day boundary.
type Budget struct {
	mu    sync.Mutex
	clock output.Clock
	limit int
	day   string
	spent int
}

// NewBudget builds a Budget with the given daily limit in weighted calls.
func NewBudget(limit int, clock output.Clock) *Budget {
	return &Budget{clock: clock, limit: limit}
}

// TryReserve atomically reserves weight from today's budget; false when the
// remaining budget is insufficient. The counter rolls over at UTC midnight.
func (b *Budget) TryReserve(weight int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	today := b.clock.Now().UTC().Format("2006-01-02")
	if today != b.day {
		b.day, b.spent = today, 0
	}
	if b.spent+weight > b.limit {
		return false
	}
	b.spent += weight
	return true
}
