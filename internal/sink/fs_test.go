package sink

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// These tests drive the platform handle directly, so the three guarantees every
// platform file must keep -- exclusive creation, exclusive directory creation
// and a rename that never replaces -- are pinned on the real filesystem each CI
// job runs on, not only through the destination above them.

func openHandle(t *testing.T, dir string) dirHandle {
	t.Helper()
	handle, err := openDirectory(dir)
	if err != nil {
		t.Fatalf("openDirectory(%q) = %v", dir, err)
	}
	t.Cleanup(func() { _ = handle.close() })
	return handle
}

func TestCreateExclusiveNeverOpensAnExistingName(t *testing.T) {
	dir := fixtureDir(t)
	handle := openHandle(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "taken.txt"), []byte("ORIGINAL"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := handle.createExclusive("taken.txt")
	if file != nil {
		_ = file.Close()
		t.Fatal("createExclusive opened a file that already existed")
	}
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("createExclusive over an existing file = %v, want fs.ErrExist", err)
	}
	if got := readFile(t, filepath.Join(dir, "taken.txt")); got != "ORIGINAL" {
		t.Fatalf("the existing file was changed: %q", got)
	}

	fresh, err := handle.createExclusive("fresh.txt")
	if err != nil {
		t.Fatalf("createExclusive of a free name = %v", err)
	}
	_ = fresh.Close()
}

func TestCreateExclusiveDoesNotFollowALinkOnItsName(t *testing.T) {
	dir := fixtureDir(t)
	handle := openHandle(t, dir)
	target := filepath.Join(fixtureDir(t), "target.txt")
	if err := os.WriteFile(target, []byte("TARGET"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link.txt")); err != nil {
		t.Skip("this runner cannot create the symbolic link fixture")
	}

	if file, err := handle.createExclusive("link.txt"); err == nil {
		_ = file.Close()
		t.Fatal("createExclusive succeeded on a name held by a symbolic link")
	}
	if got := readFile(t, target); got != "TARGET" {
		t.Fatalf("the link target was written through: %q", got)
	}
}

func TestMkdirExclusiveRefusesAnyExistingName(t *testing.T) {
	dir := fixtureDir(t)
	handle := openHandle(t, dir)

	child, err := handle.mkdirExclusive("session")
	if err != nil {
		t.Fatalf("mkdirExclusive of a free name = %v", err)
	}
	_ = child.close()
	if again, err := handle.mkdirExclusive("session"); err == nil || !errors.Is(err, fs.ErrExist) {
		if again != nil {
			_ = again.close()
		}
		t.Fatalf("mkdirExclusive of an existing directory = %v, want fs.ErrExist", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "afile"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.mkdirExclusive("afile"); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("mkdirExclusive over a file = %v, want fs.ErrExist", err)
	}
}

// The core of the write-safety claim: a rename onto an existing name fails,
// reports fs.ErrExist, and changes neither file.
func TestRenameNoReplaceNeverReplacesAnExistingFile(t *testing.T) {
	dir := fixtureDir(t)
	handle := openHandle(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "from.part"), []byte("NEW"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "final.txt"), []byte("ORIGINAL"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := handle.renameNoReplace("from.part", "final.txt")
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("renameNoReplace onto an existing file = %v, want fs.ErrExist", err)
	}
	if got := readFile(t, filepath.Join(dir, "final.txt")); got != "ORIGINAL" {
		t.Fatalf("final.txt = %q, want the original left alone", got)
	}
	if got := readFile(t, filepath.Join(dir, "from.part")); got != "NEW" {
		t.Fatalf("from.part = %q, want the source left in place after a refused rename", got)
	}

	if err := handle.renameNoReplace("from.part", "free.txt"); err != nil {
		t.Fatalf("renameNoReplace onto a free name = %v", err)
	}
	if got := readFile(t, filepath.Join(dir, "free.txt")); got != "NEW" {
		t.Fatalf("free.txt = %q, want the renamed bytes", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "from.part")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the temporary name still exists after a successful rename: %v", err)
	}
}

func TestRenameNoReplaceDoesNotReplaceADirectoryEither(t *testing.T) {
	dir := fixtureDir(t)
	handle := openHandle(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "from.part"), []byte("NEW"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "final.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := handle.renameNoReplace("from.part", "final.txt"); err == nil {
		t.Fatal("renameNoReplace onto an existing directory succeeded")
	}
}

func TestOpenDirectoryPinsTheDirectoryItWasHanded(t *testing.T) {
	dir := fixtureDir(t)
	handle := openHandle(t, dir)
	if _, err := handle.availableBytes(); err != nil {
		t.Fatalf("availableBytes = %v", err)
	}
	// markFile is best-effort and must not panic on a plain temporary file.
	file, err := handle.createExclusive("m.part")
	if err != nil {
		t.Fatal(err)
	}
	_ = handle.mark(file, "m.part", time.Now())
	_ = file.Close()
	if err := handle.unlink("m.part"); err != nil {
		t.Fatalf("unlink = %v", err)
	}
	if err := handle.close(); err != nil {
		t.Fatalf("close = %v", err)
	}
	if err := handle.close(); err != nil {
		t.Fatalf("a second close = %v, want it idempotent", err)
	}
}
