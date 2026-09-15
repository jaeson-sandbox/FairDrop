//go:build darwin

package main

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// themeReadBound is how long the appearance preference may take before FairDrop
// stops waiting for it.
//
// It is not a performance budget. `defaults` normally answers in single-digit
// milliseconds; this exists for the case where it does not answer at all,
// because cfprefsd is wedged. Short enough that a user meets a window rather
// than a hang, long enough that a machine under real load still gets its own
// answer instead of a wrong one.
const themeReadBound = 2 * time.Second

// nativeOSPrefersDarkTheme reads the macOS appearance preference.
//
// AppleInterfaceStyle is "Dark" in dark mode and absent otherwise, so `defaults`
// exits non-zero on a light-mode machine -- that is the expected light answer,
// not an error worth reporting. Read through `defaults` rather than the binary
// plist because the file is cached by cfprefsd and a direct read can return a
// stale value; and read here, before the options are built, because Wails takes
// one background colour and cannot be told about a second.
//
// Bounded, because this runs on the main goroutine before wails.Run and outside
// everything D-107 built to keep a failed start from being silent. An unbounded
// `defaults` that never returns means no window, no fatal dialog and no log
// line -- precisely the silent hang that machinery exists to prevent, one
// function earlier than it reaches (Epic 3 retrospective, B8). A bound that
// elapses answers light, which is what every other failure here already
// answers: the shade FairDrop shipped before this function existed.
//
// Nothing from the subprocess is logged: AD-9 holds for output this process did
// not author.
func nativeOSPrefersDarkTheme() bool {
	return darkThemeWithin(themeReadBound)
}

func darkThemeWithin(bound time.Duration) bool {
	return darkThemeFrom(bound, "defaults", "read", "-g", "AppleInterfaceStyle")
}

// darkThemeFrom is darkThemeWithin with the command passed in, so a test can
// bound a subprocess that really does hang rather than assert against a stub
// that only claims to.
func darkThemeFrom(bound time.Duration, name string, args ...string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), bound)
	defer cancel()

	// CommandContext kills the process when the bound elapses, so a wedged
	// `defaults` is cleaned up by the operating system rather than left for
	// this one to wait on, and Output stops waiting either way.
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(string(output)), "Dark")
}
