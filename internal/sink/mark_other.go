//go:build !windows && !darwin

package sink

import (
	"os"
	"time"
)

// markFile is a no-op where the platform has no download marking the contract
// requires. Returning nil rather than an error is deliberate: an absent feature
// is not a failed one, and must not raise the outcome warning.
func markFile(*os.File, time.Time) error { return nil }
