//go:build !windows && !darwin

package main

// showFatalDialog has nothing to show on platforms FairDrop does not ship a
// desktop build for. Linux is a build target for adapter verification only, and
// a CI runner has no one to read a dialog.
func showFatalDialog(string) {}
