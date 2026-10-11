//go:build windows

package sink

import "testing"

// clearUmask is a no-op on Windows, which has no umask; the tests that use it
// skip there because Windows has no POSIX permission bits to inspect.
func clearUmask(t *testing.T) { t.Helper() }
