package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
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

func TestInspectDirectoryReturnsExactMetadataForNestedAndEmptyTrees(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	root := filepath.Join(parent, "holiday 🌍 日本語")
	if err := os.MkdirAll(filepath.Join(root, "nested", "empty"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "first.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "second.txt"), []byte("directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		t.Fatal(err)
	}

	got, err := New().Inspect(context.Background(), root)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if got.Path != root {
		t.Fatalf("Path = %q, want byte-identical %q", got.Path, root)
	}
	if got.Name != "holiday 🌍 日本語" {
		t.Fatalf("Name = %q, want literal root basename", got.Name)
	}
	if got.Kind != transfer.ItemDirectory {
		t.Fatalf("Kind = %q, want %q", got.Kind, transfer.ItemDirectory)
	}
	if got.LogicalSize != 14 {
		t.Fatalf("LogicalSize = %d, want 14", got.LogicalSize)
	}
	if !got.ModTime.Equal(rootInfo.ModTime()) {
		t.Fatalf("ModTime = %v, want root time %v", got.ModTime, rootInfo.ModTime())
	}

	empty := filepath.Join(parent, "empty-root")
	if err := os.Mkdir(empty, 0o700); err != nil {
		t.Fatal(err)
	}
	emptyItem, err := New().Inspect(context.Background(), empty)
	if err != nil {
		t.Fatalf("Inspect(empty) error = %v", err)
	}
	if emptyItem.LogicalSize != 0 || emptyItem.Kind != transfer.ItemDirectory {
		t.Fatalf("empty metadata = %+v, want a zero-byte directory", emptyItem)
	}
}

func TestInspectDirectoryPreservesTrailingSeparatorOnlyInReturnedPath(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "folder")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	selected := root + string(os.PathSeparator)
	item, err := New().Inspect(context.Background(), selected)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if item.Path != selected {
		t.Fatalf("Path = %q, want byte-identical %q", item.Path, selected)
	}
	if item.Name != "folder" {
		t.Fatalf("Name = %q, want %q", item.Name, "folder")
	}
}

func TestInspectDirectoryUsesInspectedNameForDotAndDotDotSelections(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "actual-root")
	child := appendPath(root, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	separator := string(os.PathSeparator)
	for _, selected := range []string{
		root + separator + ".",
		child + separator + "..",
	} {
		item, err := New().Inspect(context.Background(), selected)
		if err != nil {
			t.Fatalf("Inspect(%q) error = %v", selected, err)
		}
		if item.Path != selected {
			t.Fatalf("Path = %q, want byte-identical %q", item.Path, selected)
		}
		if item.Name != "actual-root" {
			t.Fatalf("Name for %q = %q, want inspected root name %q", selected, item.Name, "actual-root")
		}
	}
}

func TestInspectRefusesRegularFileWithTrailingSeparators(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	separator := string(os.PathSeparator)
	for _, selected := range []string{path + separator, path + separator + separator + separator} {
		item, err := New().Inspect(context.Background(), selected)
		assertCode(t, err, transfer.ErrPathUnsupported)
		if item.Path != "" || item.Kind != "" || item.Name != "" {
			t.Fatalf("trailing-separator file %q returned reinterpreted metadata %+v", selected, item)
		}
	}
}

func TestInspectDirectoryUsesOneFixedPositiveBatchAndClosesEveryReader(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "wide-deep")
	for _, relative := range []string{"a/aa/aaa", "b/bb", "c", "d/dd"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(relative)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for index, relative := range []string{"one", "a/two", "a/aa/three", "b/four", "d/dd/five"} {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(relative)), []byte(strings.Repeat("x", index+1)), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	tracker := &readerTracker{}
	inspector := New()
	inspector.openDirectory = func(path string) (directoryReader, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		tracker.opened()
		return &trackingDirectoryReader{File: file, tracker: tracker}, nil
	}

	item, err := inspector.Inspect(context.Background(), root)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if tracker.active != 0 || tracker.opens != tracker.closes {
		t.Fatalf("reader ownership: opens=%d closes=%d active=%d", tracker.opens, tracker.closes, tracker.active)
	}
	if tracker.maxActive != 4 {
		t.Fatalf("max active readers = %d, want exact fixture depth 4", tracker.maxActive)
	}
	if len(tracker.batchSizes) == 0 {
		t.Fatal("ReadDir was never called")
	}
	for _, size := range tracker.batchSizes {
		if size != 1 {
			t.Fatalf("ReadDir batch = %d, want literal fixed positive batch 1", size)
		}
	}
	if item.LogicalSize != 15 {
		t.Fatalf("LogicalSize = %d, want 15", item.LogicalSize)
	}
}

func TestInspectDirectoryRefusesChangedOpenedIdentityBeforeEnumeration(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "selected")
	reader := &fakeDirectoryReader{statInfo: fakeFileInfo{name: "replacement", mode: os.ModeDir}}
	inspector := inspectorWithSelectedInfo(selected, fakeFileInfo{name: "selected", mode: os.ModeDir})
	inspector.openDirectory = func(string) (directoryReader, error) { return reader, nil }
	inspector.sameFile = func(fs.FileInfo, fs.FileInfo) bool { return false }

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrSourceChanged)
	if reader.readCalls != 0 {
		t.Fatalf("ReadDir called %d times before identity was proven", reader.readCalls)
	}
	if reader.closeCalls != 1 {
		t.Fatalf("Close called %d times, want 1", reader.closeCalls)
	}
}

func TestInspectDirectoryDefaultSameFileRejectsDifferentOpenedDirectory(t *testing.T) {
	t.Parallel()

	base := t.TempDir()
	selected := filepath.Join(base, "selected")
	replacement := filepath.Join(base, "replacement")
	if err := os.Mkdir(selected, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(replacement, 0o700); err != nil {
		t.Fatal(err)
	}
	tracker := &readerTracker{}
	inspector := New()
	inspector.openDirectory = func(string) (directoryReader, error) {
		file, err := os.Open(replacement)
		if err != nil {
			return nil, err
		}
		tracker.opened()
		return &trackingDirectoryReader{File: file, tracker: tracker}, nil
	}

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrSourceChanged)
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	if len(tracker.batchSizes) != 0 {
		t.Fatalf("ReadDir batches = %v, want none before default os.SameFile rejects identity", tracker.batchSizes)
	}
	if tracker.opens != 1 || tracker.closes != 1 || tracker.active != 0 {
		t.Fatalf("reader ownership opens/closes/active = %d/%d/%d, want 1/1/0", tracker.opens, tracker.closes, tracker.active)
	}
}

func TestInspectDirectoryRevalidatesLinkStatusAfterOpenBeforeEnumeration(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "selected")
	reader := &fakeDirectoryReader{statInfo: fakeFileInfo{name: "opened", mode: os.ModeDir, sys: "same-directory"}}
	selectedLstats := 0
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			if path == selected {
				selectedLstats++
				if selectedLstats == 1 {
					return fakeFileInfo{name: "selected", mode: os.ModeDir, sys: "same-directory"}, nil
				}
				return fakeFileInfo{name: "replacement-link", mode: os.ModeDir, sys: "same-directory"}, nil
			}
			return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir, sys: path}, nil
		},
		isReparse: func(info fs.FileInfo) (bool, error) {
			return info.Name() == "replacement-link", nil
		},
		openDirectory: func(string) (directoryReader, error) { return reader, nil },
		sameFile:      func(first, second fs.FileInfo) bool { return first.Sys() == second.Sys() },
	}

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if selectedLstats != 2 {
		t.Fatalf("selected path Lstat calls = %d, want initial inspection plus post-open revalidation", selectedLstats)
	}
	if reader.readCalls != 0 || reader.closeCalls != 1 {
		t.Fatalf("reader calls read/close = %d/%d, want 0/1", reader.readCalls, reader.closeCalls)
	}
}

func TestInspectDirectoryPostOpenRevalidationCancellationWins(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{"lstat", "reparse"} {
		operation := operation
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			selected := filepath.Join(t.TempDir(), "selected")
			reader := &fakeDirectoryReader{
				statInfo: fakeFileInfo{name: "opened", mode: os.ModeDir, sys: "same-directory"},
			}
			selectedLstats := 0
			inspector := &Inspector{
				lstat: func(path string) (fs.FileInfo, error) {
					if path == selected {
						selectedLstats++
						if selectedLstats == 2 && operation == "lstat" {
							cancel()
							return nil, fs.ErrPermission
						}
						return fakeFileInfo{name: fmt.Sprintf("selected-%d", selectedLstats), mode: os.ModeDir, sys: "same-directory"}, nil
					}
					return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir, sys: path}, nil
				},
				isReparse: func(info fs.FileInfo) (bool, error) {
					if operation == "reparse" && info.Name() == "selected-2" {
						cancel()
						return false, fs.ErrPermission
					}
					return false, nil
				},
				openDirectory: func(string) (directoryReader, error) { return reader, nil },
				sameFile:      func(first, second fs.FileInfo) bool { return first.Sys() == second.Sys() },
			}

			_, err := inspector.Inspect(ctx, selected)
			assertCode(t, err, transfer.ErrCancelled)
			if !errors.Is(err, context.Canceled) {
				t.Fatal("post-open revalidation cancellation cause was not preserved")
			}
			if reader.readCalls != 0 || reader.closeCalls != 1 {
				t.Fatalf("reader calls read/close = %d/%d, want 0/1", reader.readCalls, reader.closeCalls)
			}
		})
	}
}

func TestInspectDirectoryClosesReaderReturnedWithOpenError(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "selected")
	reader := &fakeDirectoryReader{statInfo: fakeFileInfo{name: "selected", mode: os.ModeDir}}
	inspector := inspectorWithSelectedInfo(selected, fakeFileInfo{name: "selected", mode: os.ModeDir})
	inspector.openDirectory = func(string) (directoryReader, error) { return reader, fs.ErrPermission }

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatal("directory open permission cause was not preserved")
	}
	if reader.readCalls != 0 || reader.closeCalls != 1 {
		t.Fatalf("reader calls read/close = %d/%d, want 0/1", reader.readCalls, reader.closeCalls)
	}
}

func TestInspectDirectoryRefusesChangedNestedIdentityAndClosesReaderStack(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "root")
	childPath := appendPath(root, "child")
	rootReader := &fakeDirectoryReader{
		statInfo: fakeFileInfo{name: "root-opened", mode: os.ModeDir},
		batches:  [][]fs.DirEntry{{fakeDirEntry{name: "child", mode: os.ModeDir}}},
	}
	childReader := &fakeDirectoryReader{statInfo: fakeFileInfo{name: "replacement", mode: os.ModeDir}}
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			switch path {
			case root:
				return fakeFileInfo{name: "root-inspected", mode: os.ModeDir}, nil
			case childPath:
				return fakeFileInfo{name: "child-inspected", mode: os.ModeDir}, nil
			default:
				return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
			}
		},
		isReparse: trustedSyntheticMetadata,
		openDirectory: func(path string) (directoryReader, error) {
			if path == childPath {
				return childReader, nil
			}
			return rootReader, nil
		},
		sameFile: func(inspected, opened fs.FileInfo) bool {
			return inspected.Name() == "root-inspected" && opened.Name() == "root-opened"
		},
	}

	_, err := inspector.Inspect(context.Background(), root)
	assertCode(t, err, transfer.ErrSourceChanged)
	if childReader.readCalls != 0 {
		t.Fatalf("nested ReadDir called %d times before identity was proven", childReader.readCalls)
	}
	if rootReader.closeCalls != 1 || childReader.closeCalls != 1 {
		t.Fatalf("reader closes root/child = %d/%d, want 1/1", rootReader.closeCalls, childReader.closeCalls)
	}
}

func TestInspectDirectoryRejectsOpenedAncestorCycleAndClosesBoundedStack(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "root")
	childPath := appendPath(root, "cycle")
	rootReader := &fakeDirectoryReader{
		statInfo: fakeFileInfo{name: "root-opened", mode: os.ModeDir, sys: "shared-identity"},
		batches:  [][]fs.DirEntry{{fakeDirEntry{name: "cycle", mode: os.ModeDir}}},
	}
	childReader := &fakeDirectoryReader{
		statInfo: fakeFileInfo{name: "cycle-opened", mode: os.ModeDir, sys: "shared-identity"},
	}
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			switch path {
			case root:
				return fakeFileInfo{name: "root", mode: os.ModeDir, sys: "shared-identity"}, nil
			case childPath:
				return fakeFileInfo{name: "cycle", mode: os.ModeDir, sys: "shared-identity"}, nil
			default:
				return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir, sys: path}, nil
			}
		},
		isReparse: trustedSyntheticMetadata,
		openDirectory: func(path string) (directoryReader, error) {
			if path == childPath {
				return childReader, nil
			}
			return rootReader, nil
		},
		sameFile: func(first, second fs.FileInfo) bool { return first.Sys() == second.Sys() },
	}

	_, err := inspector.Inspect(context.Background(), root)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if rootReader.readCalls != 1 || childReader.readCalls != 0 {
		t.Fatalf("ReadDir root/child calls = %d/%d, want 1/0", rootReader.readCalls, childReader.readCalls)
	}
	if rootReader.closeCalls != 1 || childReader.closeCalls != 1 {
		t.Fatalf("reader closes root/child = %d/%d, want 1/1", rootReader.closeCalls, childReader.closeCalls)
	}
}

func TestInspectDirectoryRejectsNestedLinkBeforeLaterEntry(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "root")
	linkPath := appendPath(root, "unsafe")
	laterPath := appendPath(root, "later")
	reader := &fakeDirectoryReader{
		statInfo: fakeFileInfo{name: "root", mode: os.ModeDir},
		batches:  [][]fs.DirEntry{{fakeDirEntry{name: "unsafe"}}, {fakeDirEntry{name: "later"}}},
	}
	var lstats []string
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			lstats = append(lstats, path)
			switch path {
			case root:
				return fakeFileInfo{name: "root", mode: os.ModeDir}, nil
			case linkPath:
				return fakeFileInfo{name: "unsafe", mode: os.ModeSymlink}, nil
			case laterPath:
				t.Fatal("later entry was visited after unsafe entry")
			}
			return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
		},
		isReparse:     trustedSyntheticMetadata,
		openDirectory: func(string) (directoryReader, error) { return reader, nil },
		sameFile:      func(fs.FileInfo, fs.FileInfo) bool { return true },
	}

	_, err := inspector.Inspect(context.Background(), root)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if slices.Contains(lstats, laterPath) {
		t.Fatalf("later entry was inspected; lstat calls = %q", lstats)
	}
	if reader.readCalls != 1 {
		t.Fatalf("ReadDir called %d times, want traversal stopped after the first batch", reader.readCalls)
	}
	if reader.closeCalls != 1 {
		t.Fatalf("Close called %d times, want 1", reader.closeCalls)
	}
}

func TestInspectDirectoryRejectsNestedSpecialFile(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "root")
	entryPath := appendPath(root, "device")
	reader := &fakeDirectoryReader{
		statInfo: fakeFileInfo{name: "root", mode: os.ModeDir},
		batches:  [][]fs.DirEntry{{fakeDirEntry{name: "device", mode: os.ModeDevice}}},
	}
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			if path == entryPath {
				return fakeFileInfo{name: "device", mode: os.ModeDevice}, nil
			}
			return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
		},
		isReparse:     trustedSyntheticMetadata,
		openDirectory: func(string) (directoryReader, error) { return reader, nil },
		sameFile:      func(fs.FileInfo, fs.FileInfo) bool { return true },
	}

	_, err := inspector.Inspect(context.Background(), root)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if reader.closeCalls != 1 {
		t.Fatalf("Close called %d times, want 1", reader.closeCalls)
	}
}

func TestInspectDirectoryClassifiesDisappearingEntryAndClosesReader(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "root")
	missing := appendPath(root, "gone")
	reader := &fakeDirectoryReader{
		statInfo: fakeFileInfo{name: "root", mode: os.ModeDir},
		batches:  [][]fs.DirEntry{{fakeDirEntry{name: "gone"}}},
	}
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			if path == missing {
				return nil, &os.PathError{Op: "Lstat", Path: path, Err: fs.ErrNotExist}
			}
			return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
		},
		isReparse:     trustedSyntheticMetadata,
		openDirectory: func(string) (directoryReader, error) { return reader, nil },
		sameFile:      func(fs.FileInfo, fs.FileInfo) bool { return true },
	}

	_, err := inspector.Inspect(context.Background(), root)
	assertCode(t, err, transfer.ErrPathNotFound)
	if reader.closeCalls != 1 {
		t.Fatalf("Close called %d times, want 1", reader.closeCalls)
	}
	public := transfer.PublicErrorOf(err)
	if strings.Contains(public.Message, missing) || strings.Contains(public.Message, "gone") {
		t.Fatalf("public message disclosed nested path: %q", public.Message)
	}
}

func TestInspectDirectoryRefusesNilReader(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "selected")
	inspector := inspectorWithSelectedInfo(selected, fakeFileInfo{name: "selected", mode: os.ModeDir})
	inspector.openDirectory = func(string) (directoryReader, error) { return nil, nil }

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
}

func TestInspectDirectoryPreservesHandleStatFailureAndClosesReader(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "selected")
	reader := &fakeDirectoryReader{statErr: fs.ErrPermission}
	inspector := inspectorWithSelectedInfo(selected, fakeFileInfo{name: "selected", mode: os.ModeDir})
	inspector.openDirectory = func(string) (directoryReader, error) { return reader, nil }

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatal("handle Stat permission cause was not preserved")
	}
	if reader.readCalls != 0 || reader.closeCalls != 1 {
		t.Fatalf("reader calls read/close = %d/%d, want 0/1", reader.readCalls, reader.closeCalls)
	}
}

func TestInspectDirectoryPreservesPostOpenReparseFailureAndClosesReader(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "selected")
	reader := &fakeDirectoryReader{statInfo: fakeFileInfo{name: "opened", mode: os.ModeDir}}
	selectedLstats := 0
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			if path == selected {
				selectedLstats++
				return fakeFileInfo{name: fmt.Sprintf("selected-%d", selectedLstats), mode: os.ModeDir}, nil
			}
			return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
		},
		isReparse: func(info fs.FileInfo) (bool, error) {
			if info.Name() == "selected-2" {
				return false, fs.ErrPermission
			}
			return false, nil
		},
		openDirectory: func(string) (directoryReader, error) { return reader, nil },
		sameFile:      func(fs.FileInfo, fs.FileInfo) bool { return true },
	}

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatal("post-open reparse cause was not preserved")
	}
	if reader.readCalls != 0 || reader.closeCalls != 1 {
		t.Fatalf("reader calls read/close = %d/%d, want 0/1", reader.readCalls, reader.closeCalls)
	}
}

func TestInspectDirectoryPreservesReadDirFailureAndClosesReader(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(t.TempDir(), "selected")
	reader := &fakeDirectoryReader{
		statInfo: fakeFileInfo{name: "selected", mode: os.ModeDir},
		readErr:  fs.ErrPermission,
	}
	inspector := inspectorWithSelectedInfo(selected, fakeFileInfo{name: "selected", mode: os.ModeDir})
	inspector.openDirectory = func(string) (directoryReader, error) { return reader, nil }
	inspector.sameFile = func(fs.FileInfo, fs.FileInfo) bool { return true }

	_, err := inspector.Inspect(context.Background(), selected)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatal("ReadDir permission cause was not preserved")
	}
	if reader.readCalls != 1 || reader.closeCalls != 1 {
		t.Fatalf("reader calls read/close = %d/%d, want 1/1", reader.readCalls, reader.closeCalls)
	}
}

func TestInspectDirectoryPreservesCloseFailureAndClosesFullReaderStack(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "root")
	childPath := appendPath(root, "child")
	closeCause := errors.New("close failed")
	rootReader := &fakeDirectoryReader{
		statInfo: fakeFileInfo{name: "root-opened", mode: os.ModeDir, sys: "root"},
		batches:  [][]fs.DirEntry{{fakeDirEntry{name: "child", mode: os.ModeDir}}},
	}
	childReader := &fakeDirectoryReader{
		statInfo: fakeFileInfo{name: "child-opened", mode: os.ModeDir, sys: "child"},
		closeErr: closeCause,
	}
	inspector := &Inspector{
		lstat: func(path string) (fs.FileInfo, error) {
			switch path {
			case root:
				return fakeFileInfo{name: "root", mode: os.ModeDir, sys: "root"}, nil
			case childPath:
				return fakeFileInfo{name: "child", mode: os.ModeDir, sys: "child"}, nil
			default:
				return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir, sys: path}, nil
			}
		},
		isReparse: trustedSyntheticMetadata,
		openDirectory: func(path string) (directoryReader, error) {
			if path == childPath {
				return childReader, nil
			}
			return rootReader, nil
		},
		sameFile: func(first, second fs.FileInfo) bool { return first.Sys() == second.Sys() },
	}

	_, err := inspector.Inspect(context.Background(), root)
	assertCode(t, err, transfer.ErrTransferFailed)
	if !errors.Is(err, closeCause) {
		t.Fatal("directory Close cause was not preserved")
	}
	if rootReader.closeCalls != 1 || childReader.closeCalls != 1 {
		t.Fatalf("reader closes root/child = %d/%d, want 1/1", rootReader.closeCalls, childReader.closeCalls)
	}
}

func TestInspectDirectoryRejectsNegativeAndOverflowingLogicalSizes(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name  string
		sizes []int64
	}{
		{name: "negative", sizes: []int64{-1}},
		{name: "overflow", sizes: []int64{math.MaxInt64, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "root")
			reader := &fakeDirectoryReader{statInfo: fakeFileInfo{name: "root", mode: os.ModeDir}}
			infos := map[string]fs.FileInfo{}
			for index, size := range test.sizes {
				name := fmt.Sprintf("entry-%d", index)
				reader.batches = append(reader.batches, []fs.DirEntry{fakeDirEntry{name: name}})
				infos[appendPath(root, name)] = fakeFileInfo{name: name, size: size}
			}
			inspector := &Inspector{
				lstat: func(path string) (fs.FileInfo, error) {
					if info, ok := infos[path]; ok {
						return info, nil
					}
					return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir}, nil
				},
				isReparse:     trustedSyntheticMetadata,
				openDirectory: func(string) (directoryReader, error) { return reader, nil },
				sameFile:      func(fs.FileInfo, fs.FileInfo) bool { return true },
			}

			_, err := inspector.Inspect(context.Background(), root)
			assertCode(t, err, transfer.ErrTransferFailed)
			if reader.closeCalls != 1 {
				t.Fatalf("Close called %d times, want 1", reader.closeCalls)
			}
		})
	}
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

func TestInspectDirectoryCancellationWinsAfterEachActiveOperation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name      string
		configure func(*Inspector, context.CancelFunc, string, *fakeDirectoryReader)
	}{
		{
			name: "reparse check",
			configure: func(inspector *Inspector, cancel context.CancelFunc, root string, _ *fakeDirectoryReader) {
				inspector.isReparse = func(info fs.FileInfo) (bool, error) {
					if info.Name() == filepath.Base(root) {
						cancel()
						return false, fs.ErrPermission
					}
					return false, nil
				}
			},
		},
		{
			name: "open",
			configure: func(inspector *Inspector, cancel context.CancelFunc, _ string, reader *fakeDirectoryReader) {
				inspector.openDirectory = func(string) (directoryReader, error) {
					cancel()
					return reader, fs.ErrPermission
				}
			},
		},
		{
			name: "handle stat",
			configure: func(_ *Inspector, cancel context.CancelFunc, _ string, reader *fakeDirectoryReader) {
				reader.onStat = cancel
				reader.statErr = fs.ErrPermission
			},
		},
		{
			name: "directory read",
			configure: func(_ *Inspector, cancel context.CancelFunc, _ string, reader *fakeDirectoryReader) {
				reader.onRead = cancel
				reader.readErr = fs.ErrPermission
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			root := filepath.Join(t.TempDir(), "root")
			reader := &fakeDirectoryReader{statInfo: fakeFileInfo{name: "root", mode: os.ModeDir}}
			inspector := inspectorWithSelectedInfo(root, fakeFileInfo{name: "root", mode: os.ModeDir})
			inspector.openDirectory = func(string) (directoryReader, error) { return reader, nil }
			inspector.sameFile = func(fs.FileInfo, fs.FileInfo) bool { return true }
			test.configure(inspector, cancel, root, reader)

			_, err := inspector.Inspect(ctx, root)
			assertCode(t, err, transfer.ErrCancelled)
			if !errors.Is(err, context.Canceled) {
				t.Fatal("context cancellation cause is not preserved")
			}
			if test.name != "reparse check" && reader.closeCalls != 1 {
				t.Fatalf("Close called %d times, want 1", reader.closeCalls)
			}
		})
	}
}

func TestInspectDirectoryNestedCancellationWinsAndClosesReaderStack(t *testing.T) {
	t.Parallel()

	for _, operation := range []string{"lstat", "reparse", "open"} {
		operation := operation
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			root := filepath.Join(t.TempDir(), "root")
			childPath := appendPath(root, "child")
			rootReader := &fakeDirectoryReader{
				statInfo: fakeFileInfo{name: "root-opened", mode: os.ModeDir, sys: "root"},
				batches:  [][]fs.DirEntry{{fakeDirEntry{name: "child", mode: os.ModeDir}}},
			}
			childReader := &fakeDirectoryReader{
				statInfo: fakeFileInfo{name: "child-opened", mode: os.ModeDir, sys: "child"},
			}
			inspector := &Inspector{
				lstat: func(path string) (fs.FileInfo, error) {
					switch path {
					case root:
						return fakeFileInfo{name: "root", mode: os.ModeDir, sys: "root"}, nil
					case childPath:
						if operation == "lstat" {
							cancel()
							return fakeFileInfo{name: "child", mode: os.ModeDir, sys: "child"}, fs.ErrPermission
						}
						return fakeFileInfo{name: "child", mode: os.ModeDir, sys: "child"}, nil
					default:
						return fakeFileInfo{name: filepath.Base(path), mode: os.ModeDir, sys: path}, nil
					}
				},
				isReparse: func(info fs.FileInfo) (bool, error) {
					if operation == "reparse" && info.Name() == "child" {
						cancel()
						return false, fs.ErrPermission
					}
					return false, nil
				},
				openDirectory: func(path string) (directoryReader, error) {
					if path == childPath {
						if operation == "open" {
							cancel()
							return childReader, fs.ErrPermission
						}
						return childReader, nil
					}
					return rootReader, nil
				},
				sameFile: func(first, second fs.FileInfo) bool { return first.Sys() == second.Sys() },
			}

			_, err := inspector.Inspect(ctx, root)
			assertCode(t, err, transfer.ErrCancelled)
			if !errors.Is(err, context.Canceled) {
				t.Fatal("nested cancellation cause was not preserved")
			}
			if rootReader.closeCalls != 1 {
				t.Fatalf("root Close called %d times, want 1", rootReader.closeCalls)
			}
			wantChildCloses := 0
			if operation == "open" {
				wantChildCloses = 1
			}
			if childReader.closeCalls != wantChildCloses {
				t.Fatalf("child Close called %d times, want %d", childReader.closeCalls, wantChildCloses)
			}
		})
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

type fakeDirEntry struct {
	name string
	mode fs.FileMode
}

func (f fakeDirEntry) Name() string      { return f.name }
func (f fakeDirEntry) IsDir() bool       { return f.mode.IsDir() }
func (f fakeDirEntry) Type() fs.FileMode { return f.mode.Type() }
func (f fakeDirEntry) Info() (fs.FileInfo, error) {
	return fakeFileInfo{name: f.name, mode: f.mode}, nil
}

type fakeDirectoryReader struct {
	statInfo   fs.FileInfo
	statErr    error
	batches    [][]fs.DirEntry
	readErr    error
	closeErr   error
	readCalls  int
	closeCalls int
	onStat     func()
	onRead     func()
}

func (f *fakeDirectoryReader) Stat() (fs.FileInfo, error) {
	if f.onStat != nil {
		f.onStat()
	}
	return f.statInfo, f.statErr
}

func (f *fakeDirectoryReader) ReadDir(int) ([]fs.DirEntry, error) {
	f.readCalls++
	if f.onRead != nil {
		f.onRead()
	}
	if len(f.batches) == 0 {
		if f.readErr != nil {
			return nil, f.readErr
		}
		return nil, io.EOF
	}
	batch := f.batches[0]
	f.batches = f.batches[1:]
	return batch, nil
}

func (f *fakeDirectoryReader) Close() error {
	f.closeCalls++
	return f.closeErr
}

type readerTracker struct {
	mu         sync.Mutex
	opens      int
	closes     int
	active     int
	maxActive  int
	batchSizes []int
}

func (t *readerTracker) opened() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.opens++
	t.active++
	if t.active > t.maxActive {
		t.maxActive = t.active
	}
}

type trackingDirectoryReader struct {
	*os.File
	tracker *readerTracker
	closed  bool
}

func (r *trackingDirectoryReader) ReadDir(n int) ([]fs.DirEntry, error) {
	r.tracker.mu.Lock()
	r.tracker.batchSizes = append(r.tracker.batchSizes, n)
	r.tracker.mu.Unlock()
	return r.File.ReadDir(n)
}

func (r *trackingDirectoryReader) Close() error {
	err := r.File.Close()
	r.tracker.mu.Lock()
	defer r.tracker.mu.Unlock()
	if !r.closed {
		r.closed = true
		r.tracker.closes++
		r.tracker.active--
	}
	return err
}
