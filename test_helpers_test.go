package bf

import "time"

// nowPlus returns a time.Time d into the future. Tiny helper to keep tests terse.
func nowPlus(d time.Duration) time.Time { return time.Now().Add(d) }
