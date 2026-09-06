package omhttp

import (
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (f *fakeClock) Now() time.Time { return f.now }

func TestBudget_ReserveUntilExhausted(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)}
	b := NewBudget(10, clk)
	if !b.TryReserve(6) || !b.TryReserve(4) {
		t.Fatal("reservations within the limit must succeed")
	}
	if b.TryReserve(1) {
		t.Fatal("reservation beyond the limit must fail")
	}
}

func TestBudget_ResetsAtUTCDayRollover(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 6, 23, 0, 0, 0, time.UTC)}
	b := NewBudget(5, clk)
	if !b.TryReserve(5) || b.TryReserve(1) {
		t.Fatal("day one: limit must be enforced")
	}
	clk.now = clk.now.Add(2 * time.Hour) // 2026-09-07 01:00 UTC
	if !b.TryReserve(5) {
		t.Fatal("new UTC day: budget must reset")
	}
}
