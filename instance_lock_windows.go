//go:build windows

package main

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFileExclusive takes an exclusive, non-blocking lock on the whole file.
//
// LOCKFILE_FAIL_IMMEDIATELY is what makes this a probe rather than a wait: a
// launch that would otherwise block behind the running instance answers "not
// held" at once and leaves Wails' handoff to do its work.
func lockFileExclusive(file *os.File) bool {
	overlapped := new(windows.Overlapped)
	err := windows.LockFileEx(
		windows.Handle(file.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0,
		^uint32(0),
		^uint32(0),
		overlapped,
	)
	return err == nil
}
