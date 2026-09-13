//go:build darwin

package main

import (
	"os/exec"
	"strings"
)

// nativeOSPrefersDarkTheme reads the macOS appearance preference.
//
// AppleInterfaceStyle is "Dark" in dark mode and absent otherwise, so `defaults`
// exits non-zero on a light-mode machine -- that is the expected light answer,
// not an error worth reporting. Read through `defaults` rather than the binary
// plist because the file is cached by cfprefsd and a direct read can return a
// stale value; and read here, before the options are built, because Wails takes
// one background colour and cannot be told about a second.
//
// Nothing from the subprocess is logged: AD-9 holds for output this process did
// not author.
func nativeOSPrefersDarkTheme() bool {
	output, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(output)), "Dark")
}
