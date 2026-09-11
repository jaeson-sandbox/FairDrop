//go:build darwin

package source

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"fairdrop/internal/transfer"
	"golang.org/x/sys/unix"
)

func TestDarwinMetadataSnapshotNeedsNoContentPermission(t *testing.T) {
	selected := filepath.Join(fixtureDir(t), "metadata-only.bin")
	if err := os.WriteFile(selected, []byte("metadata"), 0o000); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	if content, err := os.Open(selected); err == nil {
		_ = content.Close()
		t.Skip("runner privileges bypass the no-read file restriction")
	} else if !os.IsPermission(err) {
		t.Fatalf("fixture content refusal = %T, want permission refusal", err)
	}
	metadata, ancestors := openPOSIXMetadataForSelection(t, selected)
	defer func() { _ = metadata.Close(); _ = closeMetadataHandles(context.Background(), ancestors, nil) }()
	info, err := metadata.Stat()
	if err != nil || info.Size() != 8 {
		t.Fatalf("no-read fstatat metadata = %v, %v", info, err)
	}
	if _, ok := metadata.(*darwinMetadata); !ok {
		t.Fatalf("metadata type = %T, want descriptor-free stat snapshot", metadata)
	}
	if _, ok := metadata.(io.Reader); ok {
		t.Fatal("metadata snapshot exposes content reads")
	}
	if _, err := New().Inspect(context.Background(), selected); err != nil {
		t.Fatalf("production Inspect(no-read file) = %v", err)
	}
	if _, err := New().Inspect(context.Background(), filepath.Dir(selected)); err != nil {
		t.Fatalf("production Inspect(directory containing no-read file) = %v", err)
	}
	err = New().Walk(context.Background(), filepath.Dir(selected), func(transfer.SourceEntry, io.Reader) error {
		t.Fatal("Walk lent content without file read permission")
		return nil
	})
	if transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
		t.Fatalf("content permission gate = %v, want path_unsupported", err)
	}
}

func TestDarwinMetadataIdentityMatchesOpenedDescriptorAndRefusesReplacement(t *testing.T) {
	for _, directory := range []bool{false, true} {
		name := "file"
		if directory {
			name = "directory"
		}
		t.Run(name, func(t *testing.T) {
			base := fixtureDir(t)
			selected := filepath.Join(base, "selected")
			create := func() {
				t.Helper()
				if directory {
					if err := os.Mkdir(selected, 0o700); err != nil {
						t.Fatalf("native fixture operation failed: %T", err)
					}
				} else if err := os.WriteFile(selected, []byte("content"), 0o600); err != nil {
					t.Fatalf("native fixture operation failed: %T", err)
				}
			}
			create()
			metadata, ancestors := openPOSIXMetadataForSelection(t, selected)
			defer func() { _ = metadata.Close(); _ = closeMetadataHandles(context.Background(), ancestors, nil) }()
			inspected, err := metadata.Stat()
			if err != nil {
				t.Fatalf("native fixture operation failed: %T", err)
			}
			open := func() (statHandle, func()) {
				t.Helper()
				if directory {
					h, err := metadata.OpenEnumeration()
					if err != nil {
						t.Fatalf("native fixture operation failed: %T", err)
					}
					return h, func() { _ = h.Close() }
				}
				h, err := ancestors[len(ancestors)-1].(*posixNode).OpenChildContent("selected")
				if err != nil {
					t.Fatalf("native fixture operation failed: %T", err)
				}
				return h, func() { _ = h.Close() }
			}
			opened, closeOpened := open()
			openedInfo, err := New().verifyOpened(context.Background(), inspected, opened, directory)
			closeOpened()
			if err != nil {
				t.Fatalf("snapshot-to-descriptor identity rejected the same object: %v", err)
			}
			if inspected.Name() != openedInfo.Name() || inspected.Size() != openedInfo.Size() || inspected.Mode() != openedInfo.Mode() || !inspected.ModTime().Equal(openedInfo.ModTime()) {
				t.Fatal("stat-derived metadata differs from os descriptor metadata")
			}
			if !nativeSameFile(openedInfo, inspected) || !nativeSameFile(inspected, inspected) || !nativeSameFile(openedInfo, openedInfo) {
				t.Fatal("native identity failed reversed or same-representation comparisons")
			}
			if err := os.Rename(selected, filepath.Join(base, "original")); err != nil {
				t.Fatalf("native fixture operation failed: %T", err)
			}
			create()
			replacement, closeReplacement := open()
			defer closeReplacement()
			if _, err := New().verifyOpened(context.Background(), inspected, replacement, directory); transfer.ErrorCodeOf(err) != transfer.ErrSourceChanged {
				t.Fatalf("replacement identity gate = %v, want source_changed", err)
			}
		})
	}
}

func TestDarwinMetadataIsParentRelativeAndCloseDoesNotOwnParent(t *testing.T) {
	base := fixtureDir(t)
	parentPath := filepath.Join(base, "parent")
	if err := os.Mkdir(parentPath, 0o700); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	selected := filepath.Join(parentPath, "child")
	if err := os.WriteFile(selected, []byte("original"), 0o600); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	metadata, ancestors := openPOSIXMetadataForSelection(t, selected)
	defer func() { _ = metadata.Close(); _ = closeMetadataHandles(context.Background(), ancestors, nil) }()
	inspected, err := metadata.Stat()
	if err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	if err := os.Rename(parentPath, filepath.Join(base, "moved")); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	if err := os.Mkdir(parentPath, 0o700); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	if err := os.WriteFile(selected, []byte("replacement"), 0o600); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	parent := ancestors[len(ancestors)-1]
	fresh, err := parent.OpenChildMetadata("child")
	if err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	defer fresh.Close()
	freshInfo, err := fresh.Stat()
	if err != nil || !nativeSameFile(inspected, freshInfo) || freshInfo.Size() != 8 {
		t.Fatalf("metadata followed a replaced absolute parent: %v, %v", freshInfo, err)
	}
	if err := metadata.Close(); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	if _, err := metadata.Stat(); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("closed snapshot Stat = %v", err)
	}
	if _, err := metadata.OpenSearch(); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("closed snapshot OpenSearch = %v", err)
	}
	if _, err := metadata.OpenEnumeration(); !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("closed snapshot OpenEnumeration = %v", err)
	}
	if _, err := parent.Stat(); err != nil {
		t.Fatalf("snapshot Close closed its borrowed parent: %v", err)
	}
}

func TestDarwinMetadataNoFollowAndSpecialFileClassification(t *testing.T) {
	base := fixtureDir(t)
	target := filepath.Join(base, "ordinary")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	for _, test := range []struct {
		name   string
		mode   fs.FileMode
		create func(string) error
	}{
		{"symlink", fs.ModeSymlink, func(path string) error { return os.Symlink(target, path) }},
		{"fifo", fs.ModeNamedPipe, func(path string) error { return unix.Mkfifo(path, 0o600) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(base, test.name)
			if err := test.create(path); err != nil {
				// Native macOS CI must exercise the guard, not silently skip it.
				t.Fatalf("native fixture operation failed: %T", err)
			}
			metadata, ancestors := openPOSIXMetadataForSelection(t, path)
			defer func() { _ = metadata.Close(); _ = closeMetadataHandles(context.Background(), ancestors, nil) }()
			info, err := metadata.Stat()
			if err != nil || info.Mode().Type() != test.mode {
				t.Fatalf("no-follow metadata = %v, %v, want mode %v", info, err, test.mode)
			}
			if _, err := New().Inspect(context.Background(), path); transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
				t.Fatalf("unsupported native entry accepted: %v", err)
			}
		})
	}
}

func TestDarwinStatModeMappingAndIdentityFields(t *testing.T) {
	for _, test := range []struct {
		mode uint16
		want fs.FileMode
	}{
		{unix.S_IFREG, 0}, {unix.S_IFDIR, fs.ModeDir}, {unix.S_IFLNK, fs.ModeSymlink},
		{unix.S_IFIFO, fs.ModeNamedPipe}, {unix.S_IFSOCK, fs.ModeSocket},
		{unix.S_IFCHR, fs.ModeDevice | fs.ModeCharDevice}, {unix.S_IFBLK, fs.ModeDevice},
		{unix.S_IFWHT, fs.ModeDevice}, {0, fs.ModeIrregular},
	} {
		info := &darwinFileInfo{status: unix.Stat_t{Mode: test.mode | 0o764 | unix.S_ISUID | unix.S_ISGID | unix.S_ISVTX}}
		if got := info.Mode(); got != test.want|0o764|fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky {
			t.Errorf("mode %o became %v", test.mode, got)
		}
	}
	first := &darwinFileInfo{status: unix.Stat_t{Dev: 1, Ino: 2}}
	if nativeSameFile(first, &darwinFileInfo{status: unix.Stat_t{Dev: 3, Ino: 2}}) {
		t.Fatal("identity ignored device")
	}
	if nativeSameFile(first, &darwinFileInfo{status: unix.Stat_t{Dev: 1, Ino: 3}}) {
		t.Fatal("identity ignored inode")
	}
	if nativeSameFile(nil, first) || nativeSameFile(first, nil) {
		t.Fatal("identity accepted missing metadata")
	}
}

type syscallDarwinInfo struct {
	fs.FileInfo
	status syscall.Stat_t
}

func (i syscallDarwinInfo) Sys() any { return &i.status }

func TestDarwinMetadataRefusesRecycledIdentityAcrossStatRepresentations(t *testing.T) {
	original := unix.Stat_t{Dev: 1, Ino: 2, Gen: 3, Btim: unix.Timespec{Sec: 4, Nsec: 5}}
	for _, field := range []string{"device", "inode", "generation", "birth-seconds", "birth-nanoseconds"} {
		t.Run(field, func(t *testing.T) {
			changed := original
			switch field {
			case "device":
				changed.Dev++
			case "inode":
				changed.Ino++
			case "generation":
				changed.Gen++
			case "birth-seconds":
				changed.Btim.Sec++
			case "birth-nanoseconds":
				changed.Btim.Nsec++
			}
			forms := func(s unix.Stat_t) []fs.FileInfo {
				unixInfo := &darwinFileInfo{status: s}
				return []fs.FileInfo{unixInfo, syscallDarwinInfo{FileInfo: unixInfo, status: syscall.Stat_t{
					Dev: s.Dev, Ino: s.Ino, Gen: s.Gen,
					Birthtimespec: syscall.Timespec{Sec: s.Btim.Sec, Nsec: s.Btim.Nsec},
				}}}
			}
			for _, first := range forms(original) {
				for _, same := range forms(original) {
					if !nativeSameFile(first, same) {
						t.Fatal("same identity rejected across stat representations")
					}
				}
				for _, replacement := range forms(changed) {
					if nativeSameFile(first, replacement) || nativeSameFile(replacement, first) {
						t.Fatal("recycled identity accepted after " + field + " changed")
					}
				}
			}
		})
	}
}

func TestDarwinMetadataRefusesUnlinkRecreate(t *testing.T) {
	for _, directory := range []bool{false, true} {
		base := fixtureDir(t)
		path := filepath.Join(base, "selected")
		create := func() {
			t.Helper()
			if directory {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal("directory fixture failed")
				}
			} else if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
				t.Fatal("file fixture failed")
			}
		}
		create()
		metadata, ancestors := openPOSIXMetadataForSelection(t, path)
		inspected, err := metadata.Stat()
		if err != nil {
			t.Fatal("snapshot failed")
		}
		if err := os.Remove(path); err != nil {
			t.Fatal("unlink fixture failed")
		}
		create()
		var opened statHandle
		var release func() error
		if directory {
			h, err := metadata.OpenEnumeration()
			if err != nil {
				t.Fatal("replacement directory open failed")
			}
			opened, release = h, h.Close
		} else {
			h, err := ancestors[len(ancestors)-1].(*posixNode).OpenChildContent("selected")
			if err != nil {
				t.Fatal("replacement content open failed")
			}
			opened, release = h, h.Close
		}
		_, verifyErr := New().verifyOpened(context.Background(), inspected, opened, directory)
		_ = release()
		_ = metadata.Close()
		_ = closeMetadataHandles(context.Background(), ancestors, nil)
		if transfer.ErrorCodeOf(verifyErr) != transfer.ErrSourceChanged {
			t.Fatal("unlink/recreate escaped the snapshot identity gate")
		}
	}
}
