//go:build windows

package main

import "golang.org/x/sys/windows"

// showFatalDialog puts one fixed sentence in front of the user.
//
// A release build has no console, so a panic before the window exists takes the
// process down with nothing on screen and nothing written anywhere a person
// would look (D-107). MB_ICONERROR|MB_OK|MB_SETFOREGROUND is the smallest
// dialog that says so; MB_SETFOREGROUND because this process has no window of
// its own to be in front of.
//
// Best effort by construction: this runs while the process is already failing,
// so an error opening a dialog is nothing to report to.
func showFatalDialog(message string) {
	title, titleErr := windows.UTF16PtrFromString("FairDrop")
	body, bodyErr := windows.UTF16PtrFromString(message)
	if titleErr != nil || bodyErr != nil {
		return
	}
	_, _ = windows.MessageBox(0, body, title, windows.MB_OK|windows.MB_ICONERROR|windows.MB_SETFOREGROUND)
}
