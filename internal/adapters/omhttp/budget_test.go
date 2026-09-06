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

func TestBudget_UntilReset(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 6, 23, 0, 0, 0, time.UTC)}
	b := NewBudget(5, clk)
	if got, want := b.UntilReset(), time.Hour; got != want {
		t.Errorf("UntilReset() = %v, want %v", got, want)
	}
}

func TestBudget_UntilReset_HasAMinimumFloor(t *testing.T) {
	// A clock sitting exactly at midnight (or a hair past it, after rounding)
	// must not yield a zero or negative retry hint.
	clk := &fakeClock{now: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)}
	b := NewBudget(5, clk)
	if got, floor := b.UntilReset(), time.Second; got < floor {
		t.Errorf("UntilReset() = %v, want >= %v", got, floor)
	}
}

func TestBudget_SpentAndLimit(t *testing.T) {
	clk := &fakeClock{now: time.Date(2026, 9, 6, 10, 0, 0, 0, time.UTC)}
	b := NewBudget(10, clk)
	if got := b.Limit(); got != 10 {
		t.Errorf("Limit() = %d, want 10", got)
	}
	if got := b.Spent(); got != 0 {
		t.Errorf("Spent() = %d, want 0", got)
	}
	b.TryReserve(4)
	if got := b.Spent(); got != 4 {
		t.Errorf("Spent() = %d, want 4", got)
	}

	// Spent rolls the day over before reporting.
	clk.now = clk.now.Add(24 * time.Hour)
	if got := b.Spent(); got != 0 {
		t.Errorf("Spent() after day rollover = %d, want 0", got)
	}
}
