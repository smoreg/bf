package bf

import (
	"testing"
	"time"
)

// mustTimeout returns a channel that fires after a short timeout, used by tests
// that wait for goroutine completion. Centralised so the timeout is consistent.
func mustTimeout(_ *testing.T) <-chan time.Time {
	return time.After(2 * time.Second)
}
