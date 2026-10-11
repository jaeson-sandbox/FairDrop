//go:build windows

package sink

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// maxPath is the classic Win32 path limit the raw MoveFileEx call is subject to
// unless it is handed an extended-length path.
const maxPath = 260

// deepFixture returns a directory whose own path is well past 200 characters, so
// that any file of a realistic name placed inside a session subfolder lands on a
// final path longer than MAX_PATH. Go's os.* calls (used to build it) lift the
// limit themselves; the raw rename in the sink must too.
func deepFixture(t *testing.T) string {
	t.Helper()
	dir := fixtureDir(t)
	for _, segment := range []string{strings.Repeat("a", 70), strings.Repeat("b", 70), strings.Repeat("c", 70)} {
		dir = filepath.Join(dir, segment)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Skipf("this runner cannot create a path past MAX_PATH: %v", err)
	}
	return dir
}

func TestRenameNoReplaceWorksOnAFinalPathLongerThanMaxPath(t *testing.T) {
	dir := deepFixture(t)
	handle := openHandle(t, dir)

	finalName := strings.Repeat("n", 120) + ".txt"
	if got := len(filepath.Join(dir, finalName)); got <= maxPath {
		t.Fatalf("the fixture final path is %d characters, want more than %d", got, maxPath)
	}
	file, err := handle.createExclusive("from.part")
	if err != nil {
		t.Fatalf("createExclusive on a long path = %v", err)
	}
	if _, err := file.WriteString("LONG"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if err := handle.renameNoReplace("from.part", finalName); err != nil {
		t.Fatalf("renameNoReplace to a %d character path = %v, want it renamed", len(filepath.Join(dir, finalName)), err)
	}
	if got := readFile(t, filepath.Join(dir, finalName)); got != "LONG" {
		t.Fatalf("the renamed file holds %q, want %q", got, "LONG")
	}
}

func TestAFileWhoseFinalPathExceedsMaxPathIsSavedNotDiscarded(t *testing.T) {
	dir := deepFixture(t)
	dest := openTest(t, newTestSink(), dir)

	name := strings.Repeat("x", 120) + ".bin"
	if _, err := dest.SaveFile(context.Background(), name, strings.NewReader("payload")); err != nil {
		t.Fatalf("SaveFile of a file whose final path is past MAX_PATH = %v, want it saved", err)
	}
	folder, exists := dest.Folder()
	if !exists {
		t.Fatal("no subfolder exists after a saved file")
	}
	if got := len(filepath.Join(folder, name)); got <= maxPath {
		t.Fatalf("the saved file's path is %d characters, want more than %d", got, maxPath)
	}
	if got := readFile(t, filepath.Join(folder, name)); got != "payload" {
		t.Fatalf("saved content = %q, want %q", got, "payload")
	}
	if result, _ := dest.Close(); result.FilesSaved != 1 {
		t.Fatalf("FilesSaved = %d, want 1", result.FilesSaved)
	}
}
