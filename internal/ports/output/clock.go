package output

import "time"

// Clock is a driven port for the current time (injected for testability).
type Clock interface {
	Now() time.Time
}
