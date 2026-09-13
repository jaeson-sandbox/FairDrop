//go:build darwin

package main

import "os/exec"

// showFatalDialog puts one fixed sentence in front of the user.
//
// osascript rather than an NSAlert through cgo: this runs while the process is
// already failing, and a half-composed process is the worst place to make a
// first call into AppKit. The message is a fixed literal assembled here, so
// nothing the panic carried reaches the script.
//
// Best effort by construction: a machine that refuses to run osascript is not
// one this can report to.
func showFatalDialog(message string) {
	script := `display dialog "` + message + `" with title "FairDrop" buttons {"OK"} default button "OK" with icon stop`
	_ = exec.Command("osascript", "-e", script).Run()
}
