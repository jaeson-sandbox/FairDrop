//go:build darwin

package main

import (
	"testing"
	"time"
)

/*
TestTheThemeReadStopsWaitingAtItsBound is the behavioural half of B8.

nativeOSPrefersDarkTheme runs on the main goroutine before wails.Run, outside
everything D-107 built to keep a failed start from being silent. Until the Epic
3 retrospective it called exec.Command with no context, so a wedged cfprefsd
meant no window, no fatal dialog and no log line -- the exact silent hang that
machinery exists to prevent, one function too early for it to reach.

`sleep` stands in for a `defaults` that never answers. It is a real subprocess
that really does hang, so this proves the bound rather than a stub's claim
about one.
*/
func TestTheThemeReadStopsWaitingAtItsBound(t *testing.T) {
	t.Parallel()

	started := time.Now()
	dark := darkThemeFrom(100*time.Millisecond, "sleep", "30")
	waited := time.Since(started)

	if waited > 10*time.Second {
		t.Fatalf("a command that never answers held the theme read for %s: FairDrop reaches no window "+
			"and shows no dialog while this blocks", waited)
	}
	if dark {
		t.Error("a theme read that hit its bound reported dark: an elapsed bound must answer light, " +
			"the shade FairDrop shipped before this function existed")
	}
}

/*
TestTheThemeReadAnswersWhatTheCommandPrints keeps the test above from being the
only one: a darkThemeFrom that always returned false would satisfy it, and
would also make every macOS machine launch light forever.
*/
func TestTheThemeReadAnswersWhatTheCommandPrints(t *testing.T) {
	t.Parallel()

	if !darkThemeFrom(themeReadBound, "echo", "Dark") {
		t.Error(`a command printing "Dark" was read as light: every dark-mode Mac would flash cream at launch`)
	}
	if darkThemeFrom(themeReadBound, "echo", "Light") {
		t.Error(`a command printing "Light" was read as dark`)
	}
	// AppleInterfaceStyle is absent in light mode, so `defaults` exits
	// non-zero: the expected light answer, not an error worth reporting.
	if darkThemeFrom(themeReadBound, "false") {
		t.Error("a command that exited non-zero was read as dark, but that is how light mode answers")
	}
}
