package clock

import "time"

// System is the real Clock (output.Clock port).
type System struct{}

// Now returns the current time.
func (System) Now() time.Time { return time.Now() }
