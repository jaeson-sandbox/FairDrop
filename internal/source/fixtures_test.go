package source

import (
	"path/filepath"
	"testing"
)

// fixtureDir is t.TempDir() with every symlink in the returned path resolved.
//
// It exists because macOS puts the per-user temp tree under /var, and /var is a
// symlink to /private/var. This package refuses a link-like component anywhere
// in a selection on purpose -- rejectUnsupportedInfo, pinned by
// TestInspectRejectsLinksSpecialsAndStopsBeforeLaterEntries -- so an unresolved
// t.TempDir() hands every darwin fixture a path the package is required to
// refuse, for a reason that has nothing to do with what the test is checking.
// The first CI run on a macOS runner failed roughly forty tests across this
// package and internal/stream on exactly that, all reporting ELOOP.
//
// On Linux and Windows the resolved path is the same path, so this changes
// nothing there. A test that wants to exercise the link refusal builds its own
// symlink inside the fixture, which still works.
func fixtureDir(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving the fixture directory: %v", err)
	}
	return resolved
}
