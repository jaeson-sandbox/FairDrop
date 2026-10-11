//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// nativeUnwritableDir returns an existing plain directory the current user cannot
// create anything in: mode 0555 on POSIX. Root ignores directory modes, so the
// test is skipped there rather than made to pass for the wrong reason.
func nativeUnwritableDir(t *testing.T, parent string) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory modes, so no directory is unwritable to it")
	}
	dir := filepath.Join(parent, "readonly")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	return dir
}
