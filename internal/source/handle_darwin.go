//go:build darwin

package source

import (
	"io/fs"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Darwin's <fcntl.h> defines O_EXEC as 0x40000000 and O_SEARCH as
// O_EXEC|O_DIRECTORY. golang.org/x/sys/unix exports neither, so the supported
// header values are spelled out here rather than imported.
//
// O_SEARCH opens a directory for traversal without requiring enumeration
// rights. It is not a general metadata capability for arbitrary file kinds.
const (
	darwinOExec   = 0x40000000
	darwinOSearch = darwinOExec | unix.O_DIRECTORY
)

// darwinMetadata is a snapshot and a borrowed parent-relative locator, not an
// open descriptor for the leaf. Search/enumeration/content are separate opens;
// verifyOpened compares their device/inode identity with this snapshot before
// traversal or a visitor can use them. No O_EVTONLY entitlement is needed.
type darwinMetadata struct {
	locator posixLocator
	info    darwinFileInfo
	closed  bool
}

func openNativeMetadata(locator posixLocator) (metadataHandle, error) {
	if locator.parent == nil || locator.parent.file == nil {
		return nil, fs.ErrClosed
	}
	var status unix.Stat_t
	if err := unix.Fstatat(int(locator.parent.file.Fd()), locator.name, &status, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, err
	}
	return &darwinMetadata{locator: locator, info: darwinFileInfo{name: locator.name, status: status}}, nil
}

func (m *darwinMetadata) Stat() (fs.FileInfo, error) {
	if m == nil || m.closed {
		return nil, fs.ErrClosed
	}
	return &m.info, nil
}

func (m *darwinMetadata) OpenChildMetadata(string) (metadataHandle, error) {
	// A metadata snapshot has no directory descriptor. Acquire and validate a
	// search or enumeration descriptor before looking up another component.
	if m == nil || m.closed {
		return nil, fs.ErrClosed
	}
	return nil, fs.ErrPermission
}

func (m *darwinMetadata) OpenSearch() (metadataHandle, error) {
	if m == nil || m.closed {
		return nil, fs.ErrClosed
	}
	return openPosixNode(m.locator, nativeSearchFlags(), false)
}

func (m *darwinMetadata) OpenEnumeration() (directoryHandle, error) {
	if m == nil || m.closed {
		return nil, fs.ErrClosed
	}
	return openPosixNode(m.locator, nativeEnumerationFlags(), true)
}

func (m *darwinMetadata) Close() error {
	if m != nil {
		m.closed = true
	}
	return nil
}

type darwinFileInfo struct {
	name   string
	status unix.Stat_t
}

func (i *darwinFileInfo) Name() string       { return i.name }
func (i *darwinFileInfo) Size() int64        { return i.status.Size }
func (i *darwinFileInfo) ModTime() time.Time { return time.Unix(i.status.Mtim.Unix()) }
func (i *darwinFileInfo) IsDir() bool        { return i.Mode().IsDir() }
func (i *darwinFileInfo) Sys() any           { return &i.status }
func (i *darwinFileInfo) Mode() fs.FileMode {
	mode := fs.FileMode(i.status.Mode & 0o777)
	switch i.status.Mode & unix.S_IFMT {
	case unix.S_IFREG:
	case unix.S_IFDIR:
		mode |= fs.ModeDir
	case unix.S_IFLNK:
		mode |= fs.ModeSymlink
	case unix.S_IFIFO:
		mode |= fs.ModeNamedPipe
	case unix.S_IFSOCK:
		mode |= fs.ModeSocket
	case unix.S_IFCHR:
		mode |= fs.ModeDevice | fs.ModeCharDevice
	case unix.S_IFBLK, unix.S_IFWHT:
		mode |= fs.ModeDevice
	default:
		mode |= fs.ModeIrregular
	}
	if i.status.Mode&unix.S_ISUID != 0 {
		mode |= fs.ModeSetuid
	}
	if i.status.Mode&unix.S_ISGID != 0 {
		mode |= fs.ModeSetgid
	}
	if i.status.Mode&unix.S_ISVTX != 0 {
		mode |= fs.ModeSticky
	}
	return mode
}

// os.SameFile accepts only os's private FileInfo type. Metadata from fstatat
// uses unix.Stat_t while os.File.Stat uses syscall.Stat_t; compare both forms
// explicitly so the production snapshot-to-descriptor identity gate works.
func nativeSameFile(first, second fs.FileInfo) bool {
	firstID, firstOK := darwinIdentity(first)
	secondID, secondOK := darwinIdentity(second)
	return firstOK && secondOK && firstID == secondID
}

// A snapshot cannot pin an unlinked inode. Generation and birth time reject
// recycled device/inode pairs without mistaking ordinary content changes for
// replacement. Filesystems exposing identical/zero generation and birth fields
// retain a residual fingerprint collision risk; this is not an inode lease.
type darwinFileIdentity struct {
	device       int32
	inode        uint64
	generation   uint32
	birthSeconds int64
	birthNanos   int64
}

func darwinIdentity(info fs.FileInfo) (darwinFileIdentity, bool) {
	if info == nil {
		return darwinFileIdentity{}, false
	}
	switch status := info.Sys().(type) {
	case *unix.Stat_t:
		if status != nil {
			return darwinFileIdentity{status.Dev, status.Ino, status.Gen, status.Btim.Sec, status.Btim.Nsec}, true
		}
	case *syscall.Stat_t:
		if status != nil {
			return darwinFileIdentity{status.Dev, status.Ino, status.Gen, status.Birthtimespec.Sec, status.Birthtimespec.Nsec}, true
		}
	}
	return darwinFileIdentity{}, false
}

// nativeSearchFlags opens a directory purely as a base for relative lookups.
// Every openat in this package resolves against one of these.
func nativeSearchFlags() int {
	return darwinOSearch | unix.O_NOFOLLOW | unix.O_CLOEXEC
}

// nativeEnumerationFlags is read access to a directory's entry list, and
// nothing else. An O_SEARCH descriptor cannot be enumerated -- fdopendir
// answers EBADF -- which is why listing is a separate open from searching
// rather than the same handle used twice.
func nativeEnumerationFlags() int {
	return unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_DIRECTORY
}

// nativeContentFlags is the only file-content open in the package. O_NONBLOCK
// is what stops a FIFO substituted for a regular file from parking the
// traversal forever; the caller clears it after fstat proves the object
// regular.
func nativeContentFlags() int {
	return unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
}
