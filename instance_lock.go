package main

import (
	"log"
	"os"
	"path/filepath"
)

// instanceLockName is the file this user's running FairDrop holds open and
// locked. It lives beside the user's other application data rather than in a
// temporary directory, so a cleaner that empties temp cannot hand a second
// instance the lock while the first still runs.
const instanceLockName = "instance.lock"

// acquireInstanceLock takes this user's FairDrop lock, or reports that another
// process already holds it.
//
// It exists because Wails' own Windows lock has two reachable ways to miss a
// running first instance (D-088): SetupSingleInstance treats any CreateMutex
// error other than ERROR_ALREADY_EXISTS as "nobody is running", which an
// elevated first instance produces, and it falls through when FindWindowW has
// not found the first instance's event window yet, which a tight double launch
// produces. Either way two coordinators, listeners and beacons start.
//
// This is a backstop, not a replacement. It runs before wails.Run, so Wails'
// own mechanism still performs the handoff that restores the existing window
// on the ordinary path; what this adds is that a process which loses the race
// never composes a coordinator, so nothing competes for a port or advertises a
// second beacon even when Wails misses.
//
// Per-user by design. Wails' mutex is session-scoped, so two logged-in Windows
// users already get one instance each, and that is correct -- each has their
// own listener, port and transfer. A machine-wide lock would need privileges
// and would let one user deny another the application.
//
// An advisory lock on an open descriptor, never a lock file's existence: a
// lock held by a process that crashed is released by the operating system,
// while a file left behind by one would block every later launch forever.
func acquireInstanceLock() (release func(), held bool) {
	return acquireInstanceLockIn(os.UserConfigDir, log.Printf)
}

// lockUnavailable is the whole of what a disabled backstop leaves behind.
//
// Fixed and path-free: the three ways the setup below fails are all filesystem
// errors, and a filesystem error's text carries the path it was about (AD-9).
// What a reader needs is the fact, not the location -- a launch with no
// backstop is exactly the D-088 double-instance condition this file exists to
// catch, and before this line it left no trail at all (Epic 3 retrospective,
// B13). Every other protection in this build that switches itself off says so:
// singleInstanceOption logs when Wails' own lock is unusable, and buildApp logs
// when this one is not held.
const lockUnavailable = "fairdrop: instance lock unavailable; launching without the single-instance backstop"

// acquireInstanceLockIn is acquireInstanceLock with its two process-wide
// dependencies passed in, so the function main actually calls can be driven by
// a test rather than only its inner lockFileExclusive. It was untestable until
// the Epic 3 retrospective replaced its whole body with `return func() {}, true`
// against a green repository (B1).
func acquireInstanceLockIn(configDir func() (string, error), logf func(string, ...any)) (release func(), held bool) {
	directory, err := configDir()
	if err != nil {
		// No configuration directory means no backstop. Wails' own lock is
		// unaffected, so this is the behaviour that shipped before D-088
		// rather than a new failure, and refusing to launch over it would
		// trade a rare double instance for a certain dead application.
		logf("%s", lockUnavailable)
		return func() {}, true
	}

	directory = filepath.Join(directory, "FairDrop")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		logf("%s", lockUnavailable)
		return func() {}, true
	}

	file, err := os.OpenFile(filepath.Join(directory, instanceLockName), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		logf("%s", lockUnavailable)
		return func() {}, true
	}
	if !lockFileExclusive(file) {
		_ = file.Close()
		return func() {}, false
	}
	return func() {
		// Closing the descriptor releases the lock on every platform this
		// builds for; unlocking first would be redundant and would need its
		// own error handling for nothing.
		_ = file.Close()
	}, true
}
