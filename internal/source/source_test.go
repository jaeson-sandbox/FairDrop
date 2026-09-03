package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

func TestInspectRegularFile(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "report 🌍 日本語.txt")
	contents := []byte("hello, FairDrop")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	wantInfo, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	got, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got.Path != path {
		t.Fatalf("Path = %q, want unchanged %q", got.Path, path)
	}
	if got.Name != filepath.Base(path) {
		t.Fatalf("Name = %q, want %q", got.Name, filepath.Base(path))
	}
	if got.Kind != transfer.ItemFile {
		t.Fatalf("Kind = %q, want %q", got.Kind, transfer.ItemFile)
	}
	if got.LogicalSize != int64(len(contents)) {
		t.Fatalf("LogicalSize = %d, want %d", got.LogicalSize, len(contents))
	}
	if !got.ModTime.Equal(wantInfo.ModTime()) {
		t.Fatalf("ModTime = %v, want %v", got.ModTime, wantInfo.ModTime())
	}
}

func TestInspectZeroByteFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got.LogicalSize != 0 {
		t.Fatalf("LogicalSize = %d, want 0", got.LogicalSize)
	}
}

func TestInspectRejectsInvalidSelectionBeforeFilesystem(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"", filepath.Join("relative", "notes.txt")} {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			calls := 0
			inspector := &Inspector{lstat: func(string) (fs.FileInfo, error) {
				calls++
				return nil, errors.New("must not run")
			}}
			_, err := inspector.Inspect(context.Background(), path)
			assertCode(t, err, transfer.ErrInvalidSelection)
			if calls != 0 {
				t.Fatalf("lstat called %d times, want 0", calls)
			}
		})
	}
}

func TestInspectRejectsMissingPathWithoutPublicDisclosure(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing-private-name.txt")
	_, err := New().Inspect(context.Background(), path)
	assertCode(t, err, transfer.ErrPathNotFound)
	public := transfer.PublicErrorOf(err)
	if strings.Contains(public.Message, path) || strings.Contains(public.Message, filepath.Base(path)) {
		t.Fatalf("public message disclosed selected path: %q", public.Message)
	}
}

func TestInspectRejectsDirectory(t *testing.T) {
	t.Parallel()

	_, err := New().Inspect(context.Background(), t.TempDir())
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsSelectedSymlink(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	target := filepath.Join(directory, "target.txt")
	link := filepath.Join(directory, "link.txt")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	_, err := New().Inspect(context.Background(), link)
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsDanglingSymlink(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	link := filepath.Join(directory, "dangling.txt")
	if err := os.Symlink(filepath.Join(directory, "absent.txt"), link); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	_, err := New().Inspect(context.Background(), link)
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsAncestorSymlinkWhenFixtureAvailable(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	targetDirectory := filepath.Join(directory, "target")
	if err := os.Mkdir(targetDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDirectory, "file.txt"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "linked-directory")
	if err := os.Symlink(targetDirectory, link); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	_, err := New().Inspect(context.Background(), filepath.Join(link, "file.txt"))
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsSyntheticAncestorLinkAndStops(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	link := base + string(os.PathSeparator) + "link"
	selected := link + string(os.PathSeparator) + "file.txt"
	var calls []string
	inspector := &Inspector{lstat: func(path string) (fs.FileInfo, error) {
		calls = append(calls, path)
		if path == link {
			return fakeFileInfo{name: "link", mode: os.ModeSymlink}, nil
		}
		return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
	}, isReparse: trustedSyntheticMetadata}

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if !slices.Contains(calls, link) {
		t.Fatalf("ancestor link %q was not inspected; calls = %q", link, calls)
	}
	if slices.Contains(calls, selected) {
		t.Fatalf("selected path was inspected after link ancestor rejection; calls = %q", calls)
	}
}

func TestInspectPreservesFilesystemCallPathAndTraversesDotDotSyntax(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	separator := string(os.PathSeparator)
	beforeDotDot := base + separator + "link-like-before-dot-dot"
	dotDotPrefix := beforeDotDot + separator + ".."
	selected := dotDotPrefix + separator + "final 🌍.txt"
	modTime := time.Unix(1_700_000_000, 123)
	var calls []string
	inspector := &Inspector{lstat: func(path string) (fs.FileInfo, error) {
		calls = append(calls, path)
		if path == selected {
			return fakeFileInfo{name: filepath.Base(path), size: 7, modTime: modTime}, nil
		}
		return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
	}, isReparse: trustedSyntheticMetadata}

	got, err := inspector.Inspect(context.Background(), selected)
	if err != nil {
		t.Fatalf("Inspect() error = %v; calls = %q", err, calls)
	}
	if !slices.Contains(calls, beforeDotDot) || !slices.Contains(calls, dotDotPrefix) {
		t.Fatalf("syntactic ancestors around .. were not both inspected; calls = %q", calls)
	}
	if calls[len(calls)-1] != selected {
		t.Fatalf("final lstat path = %q, want byte-identical %q", calls[len(calls)-1], selected)
	}
	if got.Path != selected {
		t.Fatalf("StagedItem.Path = %q, want byte-identical %q", got.Path, selected)
	}
}

func TestInspectRejectsRegularModeReparsePoint(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "synthetic-reparse.txt")
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			if path == selected {
				return fakeFileInfo{name: filepath.Base(path)}, nil
			}
			return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
		},
		isReparse: func(info fs.FileInfo) (bool, error) {
			return info.Name() == filepath.Base(selected), nil
		},
	}

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectRejectsSpecialModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		mode fs.FileMode
	}{
		{"symlink", os.ModeSymlink},
		{"device", os.ModeDevice},
		{"named-pipe", os.ModeNamedPipe},
		{"socket", os.ModeSocket},
		{"junction-like-irregular", os.ModeIrregular},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			selected := filepath.Join(t.TempDir(), test.name)
			inspector := inspectorWithSelectedInfo(selected, fakeFileInfo{name: test.name, mode: test.mode})
			_, err := inspector.Inspect(context.Background(), selected)
			assertCode(t, err, transfer.ErrPathUnsupported)
		})
	}
}

func TestInspectClassifiesUnreadableMetadata(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "unreadable.txt")
	cause := fs.ErrPermission
	inspector := &Inspector{lstat: func(path string) (fs.FileInfo, error) {
		if path == selected {
			return nil, cause
		}
		return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
	}, isReparse: trustedSyntheticMetadata}
	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if !errors.Is(err, cause) {
		t.Fatal("metadata cause is not preserved through Unwrap")
	}
}

func TestInspectRejectsDirectoryModeReparseAncestorBeforeSelectedChild(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	reparseAncestor := filepath.Join(base, "junction")
	selected := filepath.Join(reparseAncestor, "child.txt")
	var calls []string
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			calls = append(calls, path)
			return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
		},
		isReparse: func(info fs.FileInfo) (bool, error) {
			return info.Name() == filepath.Base(reparseAncestor), nil
		},
	}

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if !slices.Contains(calls, reparseAncestor) {
		t.Fatalf("directory-mode reparse ancestor %q was not inspected; calls = %q", reparseAncestor, calls)
	}
	if slices.Contains(calls, selected) {
		t.Fatalf("selected child was inspected after reparse ancestor rejection; calls = %q", calls)
	}
}

func TestInspectClassifiesUnreachableNetworkBeforeNotExist(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "network-share", "file.txt")
	wrappedNotExist := &os.PathError{Op: "Lstat", Path: selected, Err: fs.ErrNotExist}
	inspector := &Inspector{
		lstat: func(string) (fs.FileInfo, error) { return nil, wrappedNotExist },
		isUnreachableNetwork: func(err error) bool {
			return errors.Is(err, fs.ErrNotExist)
		},
	}
	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("network error cause is not preserved through Unwrap")
	}
}

func TestInspectHonorsAlreadyCancelledContextBeforeFilesystem(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	inspector := &Inspector{lstat: func(string) (fs.FileInfo, error) {
		calls++
		return nil, nil
	}}
	_, err := inspector.Inspect(ctx, filepath.Join(t.TempDir(), "file.txt"))
	assertCode(t, err, transfer.ErrCancelled)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("context cancellation cause is not preserved")
	}
	if calls != 0 {
		t.Fatalf("lstat called %d times, want 0", calls)
	}
}

func TestInspectNilContextReturnsTypedError(t *testing.T) {
	t.Parallel()

	calls := 0
	inspector := &Inspector{lstat: func(string) (fs.FileInfo, error) {
		calls++
		return nil, errors.New("must not run")
	}}
	_, err := inspector.Inspect(nil, filepath.Join(t.TempDir(), "file.txt"))
	assertCode(t, err, transfer.ErrTransferFailed)
	if calls != 0 {
		t.Fatalf("lstat called %d times, want 0", calls)
	}
}

func TestInspectCancellationDuringFilesystemInspectionWins(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	selected := filepath.Join(t.TempDir(), "file.txt")
	var calls []string
	inspector := &Inspector{lstat: func(path string) (fs.FileInfo, error) {
		calls = append(calls, path)
		if path == selected {
			cancel()
			return fakeFileInfo{name: filepath.Base(path), size: 17}, nil
		}
		return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
	}, isReparse: trustedSyntheticMetadata}

	_, err := inspector.Inspect(ctx, selected)
	assertCode(t, err, transfer.ErrCancelled)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("context cancellation cause is not preserved")
	}
	if len(calls) < 2 || calls[len(calls)-1] != selected {
		t.Fatalf("lstat calls = %q, want ancestors followed by selected path %q", calls, selected)
	}
}

func inspectorWithSelectedInfo(selected string, selectedInfo fs.FileInfo) *Inspector {
	return &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			if path == selected {
				return selectedInfo, nil
			}
			return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
		},
		isReparse: trustedSyntheticMetadata,
	}
}

func trustedSyntheticMetadata(fs.FileInfo) (bool, error) { return false, nil }

func assertCode(t *testing.T, err error, want transfer.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("Inspect() error = nil, want code %q", want)
	}
	if got := transfer.ErrorCodeOf(err); got != want {
		t.Fatalf("ErrorCodeOf(Inspect()) = %q, want %q (error: %v)", got, want, err)
	}
}

type fakeFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	sys     any
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return f.modTime }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return f.sys }
