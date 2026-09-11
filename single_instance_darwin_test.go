//go:build darwin

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
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

// Foundation ignores TMPDIR on the hosted image. Use its actual path only
// in isolated CI. Lock the original inode, change its mode, then restore
// through that descriptor; never remove or rename a potentially shared lock.
func TestDarwinBuiltAppSurvivesUnusableLock(t *testing.T) {
	if os.Getenv("FAIRDROP_NATIVE_APP_SMOKE") != "1" || os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("built-app smoke is an explicit isolated-runner CI step")
	}
	directory := nativeLockTempDir()
	if directory == "" || !filepath.IsAbs(directory) {
		t.Fatal("Foundation returned no absolute temporary directory")
	}
	lockPath := filepath.Join(directory, "d1766c78-45cf-4e6d-9f04-c3700ab32024.lock")
	file, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		t.Fatal("cannot acquire the controlled native lock fixture")
	}
	t.Cleanup(func() { _ = file.Close() })
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		t.Fatal("native lock fixture is not a regular file")
	}
	status, ok := info.Sys().(*syscall.Stat_t)
	if !ok || status.Uid != uint32(os.Getuid()) {
		t.Fatal("native lock fixture is not owned by this runner")
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal("native lock is in use; refusing to interfere with another process")
	}
	t.Cleanup(func() {
		if err := file.Chmod(info.Mode()); err != nil {
			t.Error("failed to restore native lock fixture permissions")
		}
	})
	if err := file.Chmod(0); err != nil {
		t.Fatal("cannot make native lock fixture unusable")
	}
	if probe, err := os.OpenFile(lockPath, os.O_WRONLY, 0); err == nil {
		_ = probe.Close()
		t.Fatal("runner bypasses the required lock permission refusal")
	} else if !os.IsPermission(err) {
		t.Fatal("native lock failed for a reason other than fixture permissions")
	}
	binary, err := filepath.Abs("build/bin/fairdrop.app/Contents/MacOS/fairdrop")
	if err != nil {
		t.Fatal("cannot resolve built native executable")
	}
	var output smokeOutput
	cmd := exec.Command(binary)
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatal("cannot launch built native executable")
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
			return
		case <-time.After(5 * time.Second):
		}
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("native smoke child did not exit after kill")
		}
	})
	deadline := time.NewTimer(20 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-done:
			t.Fatalf("native app exited during startup; controlled blank-app output: %s", output.snapshot())
		case <-deadline.C:
			t.Fatalf("native app omitted degraded-lock diagnostic; controlled blank-app output: %s", output.snapshot())
		case <-tick.C:
			if strings.Contains(output.snapshot(), "fairdrop: single-instance protection unavailable (temporary lock file unusable); launching without protection") {
				select {
				case <-done:
					t.Fatalf("native app exited after diagnostic: %s", output.snapshot())
				case <-time.After(2 * time.Second):
					t.Log("built native process survives unusable lock; window visibility is unobserved")
					return
				}
			}
		}
	}
}

type smokeOutput struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (s *smokeOutput) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.Write(p)
}

func (s *smokeOutput) snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buffer.String()
}
