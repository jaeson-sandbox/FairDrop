//go:build !windows

package sink

import (
	"syscall"
	"testing"
)

// clearUmask makes the modes a test observes the modes the code asked for. With
// the usual umask of 022 a requested 0777 directory reads back as 0755, which
// would let a loosened mode constant pass any "no bit beyond 0755" check. The
// umask is process-wide, so only a non-parallel test may call this; the previous
// value is restored when the test ends.
func clearUmask(t *testing.T) {
	t.Helper()
	previous := syscall.Umask(0)
	t.Cleanup(func() { syscall.Umask(previous) })
}
