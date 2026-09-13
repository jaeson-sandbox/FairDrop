//go:build windows

package main

import "golang.org/x/sys/windows/registry"

// nativeOSPrefersDarkTheme reads the Windows app-theme preference.
//
// AppsUseLightTheme is the value File Explorer and the Settings app follow, and
// it is per-user, which is why it lives under HKCU. It is absent on a fresh
// profile and on editions that never wrote it, and any failure to read it
// answers light -- the shade FairDrop shipped before this existed, so an
// unreadable preference changes nothing rather than guessing dark.
func nativeOSPrefersDarkTheme() bool {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return false
	}
	defer func() { _ = key.Close() }()

	light, _, err := key.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return false
	}
	return light == 0
}
