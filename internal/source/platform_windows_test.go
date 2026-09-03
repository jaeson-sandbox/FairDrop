//go:build windows

package source

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"fairdrop/internal/transfer"
)

func TestPlatformReparsePointReadsWindowsAttributes(t *testing.T) {
	t.Parallel()

	info := fakeFileInfo{
		name: "regular-mode-reparse",
		mode: 0,
		sys: &syscall.Win32FileAttributeData{
			FileAttributes: syscall.FILE_ATTRIBUTE_REPARSE_POINT,
		},
	}
	if !info.Mode().IsRegular() {
		t.Fatal("fixture must have regular Go mode")
	}
	reparse, err := platformReparsePoint(info)
	if err != nil {
		t.Fatalf("platformReparsePoint() error = %v", err)
	}
	if !reparse {
		t.Fatal("Windows reparse attribute was not detected")
	}
}

func TestInspectRejectsUnexpectedWindowsFileInfoSys(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		sys  any
	}{
		{"nil", nil},
		{"typed-nil", (*syscall.Win32FileAttributeData)(nil)},
		{"wrong-type", struct{}{}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			selected := filepath.Join(t.TempDir(), "synthetic.txt")
			inspector := &Inspector{lstat: func(path string) (fs.FileInfo, error) {
				if path == selected {
					return fakeFileInfo{name: filepath.Base(path), sys: test.sys}, nil
				}
				return os.Lstat(path)
			}}

			_, err := inspector.Inspect(context.Background(), selected)
			assertCode(t, err, transfer.ErrPathUnsupported)
		})
	}
}

func TestPlatformUnreachableNetworkErrorsOverrideNotExist(t *testing.T) {
	t.Parallel()

	for _, errno := range []syscall.Errno{53, 67, 1203, 1231, 1232} {
		err := &os.PathError{Op: "Lstat", Path: `\\server\share\file`, Err: errno}
		if !platformUnreachableNetworkError(err) {
			t.Errorf("Win32 error %d was not classified as an unreachable network path", errno)
		}
	}
	if platformUnreachableNetworkError(fs.ErrNotExist) {
		t.Fatal("plain not-exist was incorrectly classified as an unreachable network path")
	}
}

func TestInspectRejectsNativeJunctionAncestor(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	target := filepath.Join(base, "target")
	junction := filepath.Join(base, "junction")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "child.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Fixture-only shell exception: Windows exposes junction creation through
	// the cmd.exe mklink builtin. Production inspection never invokes a shell.
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Skipf("junction fixture unavailable: %v (%s)", err, strings.TrimSpace(string(output)))
	}
	t.Cleanup(func() {
		// Remove the reparse point before TempDir cleanup reaches its target.
		_ = os.Remove(junction)
	})

	_, err := New().Inspect(context.Background(), filepath.Join(junction, "child.txt"))
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsNativeNULDevice(t *testing.T) {
	t.Parallel()

	volume := filepath.VolumeName(t.TempDir())
	path := volume + string(os.PathSeparator) + "NUL"
	_, err := New().Inspect(context.Background(), path)
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectPreservesLongWindowsPath(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for len(directory) < 280 {
		directory = filepath.Join(directory, "long-path-segment")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Skipf("native Go cannot create the long-path fixture: %v", err)
	}
	path := filepath.Join(directory, "file.txt")
	if err := os.WriteFile(path, []byte("long"), 0o600); err != nil {
		t.Skipf("native Go cannot write the long-path fixture: %v", err)
	}

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect(long path) error = %v", err)
	}
	if item.Path != path {
		t.Fatalf("Path = %q, want byte-identical %q", item.Path, path)
	}
}

func TestInspectPreservesExtendedLengthWindowsPath(t *testing.T) {
	t.Parallel()

	ordinary := filepath.Join(t.TempDir(), "extended.txt")
	if err := os.WriteFile(ordinary, []byte("extended"), 0o600); err != nil {
		t.Fatal(err)
	}
	extended := extendedLengthPath(ordinary)
	if _, err := os.Lstat(extended); err != nil {
		t.Skipf("native Go cannot inspect the extended-length fixture: %v", err)
	}

	item, err := New().Inspect(context.Background(), extended)
	if err != nil {
		t.Fatalf("Inspect(extended path) error = %v", err)
	}
	if item.Path != extended {
		t.Fatalf("Path = %q, want byte-identical %q", item.Path, extended)
	}
}

func TestInspectPreservesReachableUNCPath(t *testing.T) {
	path := os.Getenv("FAIRDROP_TEST_UNC_FILE")
	if path == "" {
		t.Skip("FAIRDROP_TEST_UNC_FILE does not name a reachable UNC fixture")
	}
	if !strings.HasPrefix(path, `\\`) {
		t.Fatalf("FAIRDROP_TEST_UNC_FILE = %q, want a UNC path", path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Skipf("configured UNC fixture is not reachable: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("configured UNC fixture mode = %v, want a regular file", info.Mode())
	}

	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect(UNC path) error = %v", err)
	}
	if item.Path != path {
		t.Fatalf("Path = %q, want byte-identical %q", item.Path, path)
	}
}

func extendedLengthPath(path string) string {
	if strings.HasPrefix(path, `\\`) {
		return `\\?\UNC\` + strings.TrimPrefix(path, `\\`)
	}
	return `\\?\` + path
}
