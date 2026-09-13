//go:build !windows

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// lockFileExclusive takes an exclusive, non-blocking flock on the descriptor.
//
// LOCK_NB for the same reason Windows uses LOCKFILE_FAIL_IMMEDIATELY: this is
// a probe. macOS reaches here too, where Wails' own lock is a flock on a file
// in the native temporary directory -- a different file with a different
// failure mode, which is why this one lives under the user's configuration
// directory and answers only for FairDrop.
func lockFileExclusive(file *os.File) bool {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB) == nil
}
