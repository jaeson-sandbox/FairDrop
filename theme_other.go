//go:build !windows && !darwin

package main

// nativeOSPrefersDarkTheme answers light on platforms FairDrop does not ship.
// Linux is a build target for adapter verification only, never a supported
// desktop, so there is no preference to read and no flash to avoid.
func nativeOSPrefersDarkTheme() bool { return false }
