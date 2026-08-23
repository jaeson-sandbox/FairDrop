package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// --- helpers -----------------------------------------------------------------

// writeFile creates a regular file under dir and returns its absolute path.
func writeFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("create fixture %q: %v", name, err)
	}
	return path
}

// requireCode asserts the failure classification and, just as importantly, that
// the failure produced no partial StagedItem for a caller to misuse.
func requireCode(t *testing.T, item transfer.StagedItem, err error, want transfer.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("Inspect succeeded with %+v, want error %q", item, want)
	}
	if got := transfer.ErrorCodeOf(err); got != want {
		t.Fatalf("ErrorCodeOf = %q, want %q (err: %v)", got, want, err)
	}
	if item != (transfer.StagedItem{}) {
		t.Errorf("Inspect returned %+v alongside an error, want the zero StagedItem", item)
	}
}

// requireNoDisclosure enforces the disclosure matrix at the only boundary that
// matters: what PublicErrorOf hands the UI. The local err.Error() is allowed to
// name the path.
func requireNoDisclosure(t *testing.T, err error, secrets ...string) {
	t.Helper()
	message := transfer.PublicErrorOf(err).Message
	if message == "" {
		t.Fatal("public message is empty")
	}
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		if strings.Contains(message, secret) {
			t.Errorf("public message leaked %q: %q", secret, message)
		}
	}
	for _, shape := range []string{`\`, "/", ":"} {
		if strings.Contains(message, shape) {
			t.Errorf("public message %q contains path-shaped text %q", message, shape)
		}
	}
}

// --- success rows ------------------------------------------------------------

func TestInspectRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "notes.txt", []byte("hello world"))
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("stat fixture: %v", err)
	}

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}

	if item.Path != path {
		t.Errorf("Path = %q, want the input verbatim %q", item.Path, path)
	}
	if item.Name != "notes.txt" {
		t.Errorf("Name = %q, want %q", item.Name, "notes.txt")
	}
	if item.Kind != transfer.ItemFile {
		t.Errorf("Kind = %q, want %q", item.Kind, transfer.ItemFile)
	}
	if item.LogicalSize != int64(len("hello world")) {
		t.Errorf("LogicalSize = %d, want %d", item.LogicalSize, len("hello world"))
	}
	if !item.ModTime.Equal(info.ModTime()) {
		t.Errorf("ModTime = %v, want the filesystem's %v", item.ModTime, info.ModTime())
	}
}

// TestInspectPreservesSpacesAndUnicode: the path is carried, never normalized.
// A cleaned or case-folded path would later open the wrong file, or none.
func TestInspectPreservesSpacesAndUnicode(t *testing.T) {
	names := []string{
		"two words.txt",
		"ünïcödé nàme.txt",
		"emoji 😀 payload.bin",
		"日本語のファイル.txt",
		"trailing.dots...txt",
		"MiXeD CaSe.TXT",
	}

	dir := t.TempDir()
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			path := writeFile(t, dir, name, []byte("x"))

			item, err := New().Inspect(context.Background(), path)
			if err != nil {
				t.Fatalf("Inspect: %v", err)
			}
			if item.Path != path {
				t.Errorf("Path = %q, want %q byte-for-byte", item.Path, path)
			}
			if item.Name != name {
				t.Errorf("Name = %q, want %q", item.Name, name)
			}
		})
	}
}

// TestInspectZeroByteFile: an empty file is a valid selection, not an error.
// The contract gives it TotalKnown=true with TotalBytes=0 downstream, which
// only works if inspection accepts it here.
func TestInspectZeroByteFile(t *testing.T) {
	path := writeFile(t, t.TempDir(), "empty.bin", nil)

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if item.LogicalSize != 0 {
		t.Errorf("LogicalSize = %d, want 0", item.LogicalSize)
	}
	if item.Kind != transfer.ItemFile {
		t.Errorf("Kind = %q, want %q", item.Kind, transfer.ItemFile)
	}
}

// TestInspectLongPath covers the "long Windows paths wherever Go permits"
// constraint: well past the legacy 260-character MAX_PATH.
func TestInspectLongPath(t *testing.T) {
	dir := t.TempDir()
	deep := dir
	for i := 0; i < 14; i++ {
		deep = filepath.Join(deep, "abcdefghij0123456789abcdefghij")
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Skipf("host cannot create a long path fixture: %v", err)
	}
	path := filepath.Join(deep, "deep.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Skipf("host cannot write at a long path: %v", err)
	}
	if len(path) < 300 {
		t.Fatalf("fixture is only %d characters; it does not exercise long-path handling", len(path))
	}

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect on a %d-character path: %v", len(path), err)
	}
	if item.Path != path {
		t.Errorf("Path = %q, want the input verbatim", item.Path)
	}
	if item.Name != "deep.txt" {
		t.Errorf("Name = %q, want %q", item.Name, "deep.txt")
	}
}

// TestInspectExtendedLengthPrefix: the \\?\ form is absolute and must survive
// unrewritten, since stripping the prefix would reintroduce the MAX_PATH limit
// downstream.
func TestInspectExtendedLengthPrefix(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("extended-length prefixes are Windows-only")
	}
	plain := writeFile(t, t.TempDir(), "prefixed.txt", []byte("abc"))
	path := `\\?\` + plain
	if _, err := os.Lstat(path); err != nil {
		t.Skipf("host does not accept the extended-length prefix: %v", err)
	}

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if item.Path != path {
		t.Errorf("Path = %q, want the prefix preserved: %q", item.Path, path)
	}
	if item.Name != "prefixed.txt" {
		t.Errorf("Name = %q, want %q", item.Name, "prefixed.txt")
	}
}

// TestInspectUNCPath covers the UNC half of the same constraint. It needs a
// reachable share, so it skips when the host has none.
func TestInspectUNCPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("UNC paths are Windows-only")
	}
	path := `\\localhost\C$\Windows\notepad.exe`
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		t.Skipf("no reachable UNC fixture on this host: %v", err)
	}

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if item.Path != path {
		t.Errorf("Path = %q, want the UNC form preserved: %q", item.Path, path)
	}
	if item.Name != "notepad.exe" {
		t.Errorf("Name = %q, want %q", item.Name, "notepad.exe")
	}
}

// --- rejection rows ----------------------------------------------------------

// TestInspectRejectsEmptyPath. os.Lstat("") fails with a not-exist error, so
// path_not_found would be the answer if the shape check ran after the syscall.
// Getting invalid_selection is the proof that it runs before.
func TestInspectRejectsEmptyPath(t *testing.T) {
	item, err := New().Inspect(context.Background(), "")

	requireCode(t, item, err, transfer.ErrInvalidSelection)
	requireNoDisclosure(t, err)
}

// TestInspectRejectsRelativePath uses paths that really exist relative to the
// test's working directory. If the adapter resolved instead of rejecting them,
// Inspect would succeed -- which is exactly the bug: it would stage a file the
// user never pointed at.
func TestInspectRejectsRelativePath(t *testing.T) {
	if _, err := os.Lstat("source.go"); err != nil {
		t.Fatalf("fixture assumption broken: source.go is not readable from the test's cwd: %v", err)
	}

	paths := []string{"source.go", "./source.go", filepath.Join(".", "source.go"), "notes.txt", "sub/notes.txt"}
	if runtime.GOOS == "windows" {
		// Drive-relative and rooted-without-volume forms are not absolute on
		// Windows, and resolving either would depend on hidden process state.
		paths = append(paths, `C:source.go`, `\rooted-no-volume.txt`, "NUL")
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			item, err := New().Inspect(context.Background(), path)

			requireCode(t, item, err, transfer.ErrInvalidSelection)
			requireNoDisclosure(t, err, path)
		})
	}
}

func TestInspectMissingPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "was-here.txt")

	item, err := New().Inspect(context.Background(), path)

	requireCode(t, item, err, transfer.ErrPathNotFound)
	requireNoDisclosure(t, err, path, dir, "was-here.txt")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Error("errors.Is(err, fs.ErrNotExist) = false, want true: the cause must stay wrapped for local diagnosis")
	}
}

// TestInspectDeletedAfterCreation is the realistic path_not_found: the user
// picked a real file and something removed it before staging.
func TestInspectDeletedAfterCreation(t *testing.T) {
	path := writeFile(t, t.TempDir(), "vanishing.txt", []byte("x"))
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove fixture: %v", err)
	}

	item, err := New().Inspect(context.Background(), path)

	requireCode(t, item, err, transfer.ErrPathNotFound)
	requireNoDisclosure(t, err, path, "vanishing.txt")
}

// TestInspectRejectsDirectory: Epic 1 sends files only. The directory holds a
// child, and a walk would have to observe it; the zero StagedItem check in
// requireCode proves nothing was collected from inside.
func TestInspectRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	// Named distinctively on purpose: a fixture called "folder" would appear in
	// the fixed copy by coincidence and make the disclosure check meaningless.
	nested := filepath.Join(dir, "picked-directory")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	writeFile(t, nested, "inside.txt", []byte("should never be read"))

	item, err := New().Inspect(context.Background(), nested)

	requireCode(t, item, err, transfer.ErrPathUnsupported)
	requireNoDisclosure(t, err, nested, "picked-directory", "inside.txt")
}

// TestInspectRejectsSymlink covers both the valid and the dangling symlink
// rows. Creating one needs elevation or Developer Mode on Windows, so an
// ordinary user's run skips rather than fails.
func TestInspectRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := writeFile(t, dir, "target.txt", []byte("secret target contents"))
	targetDir := filepath.Join(dir, "target-dir")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}

	cases := []struct {
		name   string
		link   string
		target string
	}{
		{"to-file", filepath.Join(dir, "link-to-file.txt"), target},
		{"to-directory", filepath.Join(dir, "link-to-dir"), targetDir},
		{"dangling", filepath.Join(dir, "dangling.txt"), filepath.Join(dir, "no-such-target.txt")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.Symlink(tc.target, tc.link); err != nil {
				t.Skipf("host cannot create symlinks unprivileged: %v", err)
			}
			t.Cleanup(func() { _ = os.Remove(tc.link) })

			item, err := New().Inspect(context.Background(), tc.link)

			// A symlink to a perfectly good regular file is still rejected:
			// that is the proof the target was never followed. A dangling one
			// reports path_unsupported, not path_not_found, because the link
			// itself exists.
			requireCode(t, item, err, transfer.ErrPathUnsupported)
			requireNoDisclosure(t, err, tc.link, tc.target)
		})
	}
}

// TestInspectRejectsJunction is the row a symlink-bit check would fail:
// measured on go1.26.7/windows/amd64, Lstat reports a junction as
// ModeIrregular with ModeSymlink clear.
//
// Creating a junction needs mklink; Go has no stdlib API for it. Shelling out
// is a fixture-only exception -- the adapter under test never does.
func TestInspectRejectsJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions are Windows-only")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "junction-target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatalf("create junction target: %v", err)
	}
	writeFile(t, target, "inside.txt", []byte("should never be read"))

	link := filepath.Join(dir, "junction.lnkdir")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("host cannot create a junction: %v (%s)", err, out)
	}
	// Remove the reparse point before t.TempDir's own cleanup walks the tree.
	t.Cleanup(func() { _ = os.Remove(link) })

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("stat junction fixture: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Log("note: this host reports the junction as ModeSymlink; the IsRegular check covers both")
	}

	item, inspectErr := New().Inspect(context.Background(), link)

	requireCode(t, item, inspectErr, transfer.ErrPathUnsupported)
	requireNoDisclosure(t, inspectErr, link, target, "inside.txt")
}

// TestInspectRejectsSpecialFile covers devices, pipes, and sockets through the
// one such path every host of its family has.
func TestInspectRejectsSpecialFile(t *testing.T) {
	var path string
	if runtime.GOOS == "windows" {
		// A reserved device name resolves in any directory, which gives an
		// absolute form; the bare name "NUL" is not absolute and is covered by
		// the relative-path test instead.
		path = filepath.Join(t.TempDir(), "NUL")
	} else {
		path = "/dev/null"
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Skipf("no special-file fixture on this host: %v", err)
	}
	if info.Mode().IsRegular() {
		t.Skipf("%q is a regular file on this host, not a special file", path)
	}

	item, inspectErr := New().Inspect(context.Background(), path)

	requireCode(t, item, inspectErr, transfer.ErrPathUnsupported)
	requireNoDisclosure(t, inspectErr, path)
}

// TestInspectUnreadableMetadata is the "Lstat fails for a reason other than
// not-exist" row: it must classify as path_unsupported, keep the cause wrapped
// for diagnosis, and surface none of it.
func TestInspectUnreadableMetadata(t *testing.T) {
	path, cause := unreadableMetadataFixture(t)

	item, err := New().Inspect(context.Background(), path)

	requireCode(t, item, err, transfer.ErrPathUnsupported)
	requireNoDisclosure(t, err, path, cause.Error())
	if errors.Unwrap(err) == nil {
		t.Error("errors.Unwrap = nil, want the Lstat cause wrapped for local diagnosis")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Error("errors.Is(err, fs.ErrNotExist) = true: this fixture must fail for another reason, or the test proves nothing")
	}
}

// unreadableMetadataFixture returns a path whose Lstat fails with something
// other than a not-exist error, or skips when the host cannot produce one.
func unreadableMetadataFixture(t *testing.T) (string, error) {
	t.Helper()
	dir := t.TempDir()

	candidates := []string{
		// Characters Windows forbids in a name: ERROR_INVALID_NAME, not
		// ERROR_FILE_NOT_FOUND.
		filepath.Join(dir, "bad<name>.txt"),
		filepath.Join(dir, "bad|name.txt"),
		filepath.Join(dir, `bad"name.txt`),
	}

	if runtime.GOOS != "windows" {
		locked := filepath.Join(dir, "locked")
		if err := os.Mkdir(locked, 0o000); err == nil {
			t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
			candidates = append(candidates, filepath.Join(locked, "child.txt"))
		}
	}

	for _, path := range candidates {
		_, err := os.Lstat(path)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return path, err
		}
	}
	t.Skip("host produces no non-not-exist Lstat failure to exercise")
	return "", nil
}

// --- cancellation ------------------------------------------------------------

func TestInspectCancelledContext(t *testing.T) {
	path := writeFile(t, t.TempDir(), "ready.txt", []byte("x"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	item, err := New().Inspect(ctx, path)

	requireCode(t, item, err, transfer.ErrCancelled)
	requireNoDisclosure(t, err, path, "ready.txt")
	if !errors.Is(err, context.Canceled) {
		t.Error("errors.Is(err, context.Canceled) = false, want true: the cause must stay wrapped")
	}
}

func TestInspectExpiredDeadline(t *testing.T) {
	path := writeFile(t, t.TempDir(), "ready.txt", []byte("x"))

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	item, err := New().Inspect(ctx, path)

	requireCode(t, item, err, transfer.ErrCancelled)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("errors.Is(err, context.DeadlineExceeded) = false, want true")
	}
}

// TestInspectCancelledBeforeFilesystemWork: with a cancelled context and a path
// that does not exist, path_not_found would mean the syscall ran first.
// Cancellation must win.
func TestInspectCancelledBeforeFilesystemWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cases := map[string]string{
		"missing":  filepath.Join(t.TempDir(), "missing.txt"),
		"empty":    "",
		"relative": "source.go",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			item, err := New().Inspect(ctx, path)
			requireCode(t, item, err, transfer.ErrCancelled)
		})
	}
}

// TestInspectLiveContextIsNotCancellation guards the inverse: a healthy context
// must never be read as cancelled.
func TestInspectLiveContextIsNotCancellation(t *testing.T) {
	path := writeFile(t, t.TempDir(), "ready.txt", []byte("x"))
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if _, err := New().Inspect(ctx, path); err != nil {
		t.Fatalf("Inspect with a live context: %v", err)
	}
}

// --- shape -------------------------------------------------------------------

// TestInspectorIsConcurrencySafe: the adapter is stateless, and the coordinator
// shares one instance. Meaningful under -race.
func TestInspectorIsConcurrencySafe(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "shared.txt", []byte("payload"))
	missing := filepath.Join(dir, "absent.txt")

	inspector := New()
	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			if i%2 == 0 {
				if _, err := inspector.Inspect(context.Background(), path); err != nil {
					t.Errorf("Inspect: %v", err)
				}
				return
			}
			if _, err := inspector.Inspect(context.Background(), missing); err == nil {
				t.Error("Inspect on a missing path succeeded")
			}
		}(i)
	}
	for i := 0; i < 8; i++ {
		<-done
	}
}

// TestZeroInspectorWorks: New returns a pointer, but the zero value must not be
// a trap for a caller that composes an Inspector into a struct.
func TestZeroInspectorWorks(t *testing.T) {
	path := writeFile(t, t.TempDir(), "zero.txt", []byte("x"))
	var inspector Inspector

	if _, err := inspector.Inspect(context.Background(), path); err != nil {
		t.Fatalf("zero-value Inspect: %v", err)
	}
}
