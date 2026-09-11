//go:build darwin

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDarwinLockPreflightAllowsUsableAndContendedLocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.lock")
	if !darwinLockPathUsable(path) {
		t.Fatal("usable lock refused")
	}
	file, err := os.OpenFile(path, os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	if !darwinLockPathUsable(path) {
		t.Fatal("contention must reach Wails for second-instance handoff")
	}
}

func TestDarwinLockPreflightDisablesWailsForUnusablePath(t *testing.T) {
	dir := t.TempDir()
	for _, path := range []string{dir, filepath.Join(dir, "absent", "fixture.lock")} {
		if darwinLockPathUsable(path) {
			t.Fatal("unusable lock path accepted")
		}
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)
	probe := filepath.Join(dir, "probe")
	if file, err := os.Create(probe); err == nil {
		file.Close()
		t.Skip("runner privileges bypass unwritable directory restriction; missing-parent and directory-as-lock cases executed")
	}
	app := NewApp()
	logged := false
	app.logf = func(string, ...any) { logged = true }
	if singleInstanceOption(app, func() bool { return darwinLockPathUsable(filepath.Join(dir, "fixture.lock")) }) != nil || !logged {
		t.Fatal("unwritable lock must disable Wails single-instance option and report degraded launch")
	}
}

func TestDarwinNativeLockTemporaryDirectoryIsUsable(t *testing.T) {
	if nativeLockTempDir() == "" {
		t.Fatal("Foundation returned no lock temporary directory")
	}
	if !nativeSingleInstanceLockUsable() {
		t.Fatal("native Wails lock location is unusable on this runner")
	}
}
