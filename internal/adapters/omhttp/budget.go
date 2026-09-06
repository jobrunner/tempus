// Package omhttp provides the shared HTTP transport for every Open-Meteo call:
// a token-bucket rate limit, transparent retry honoring Retry-After, and a
// weighted daily call budget enforced only for batch-originated requests.
// Adapters receive it as a plain *http.Client, so no adapter imports this
// package (composition happens in internal/app).
package omhttp

import (
	"sync"
	"time"

	"github.com/jobrunner/tempus/internal/ports/output"
)

// minUntilReset floors UntilReset so a clock sitting exactly at (or a hair
// past) midnight never yields a zero or negative retry hint.
const minUntilReset = time.Second

const dayLayout = "2006-01-02"

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
	b.rollover()
	if b.spent+weight > b.limit {
		return false
	}
	b.spent += weight
	return true
}

// Spent returns today's weighted call count, rolling the day over first.
func (b *Budget) Spent() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rollover()
	return b.spent
}

// Limit returns the configured daily weighted-call limit.
func (b *Budget) Limit() int {
	return b.limit
}

// UntilReset returns the duration until the next UTC midnight, per the
// injected clock, floored at minUntilReset so callers never get a zero/negative
// retry hint.
func (b *Budget) UntilReset() time.Duration {
	now := b.clock.Now().UTC()
	tomorrow := now.Truncate(24 * time.Hour).Add(24 * time.Hour)
	if d := tomorrow.Sub(now); d > minUntilReset {
		return d
	}
	return minUntilReset
}

// rollover resets the counter when the clock has crossed into a new UTC day.
// Callers must hold b.mu.
func (b *Budget) rollover() {
	today := b.clock.Now().UTC().Format(dayLayout)
	if today != b.day {
		b.day, b.spent = today, 0
	}
}
