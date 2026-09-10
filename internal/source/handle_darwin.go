//go:build darwin

package source

import "golang.org/x/sys/unix"

// Darwin's <fcntl.h> defines O_EXEC as 0x40000000 and O_SEARCH as
// O_EXEC|O_DIRECTORY. golang.org/x/sys/unix exports neither, which is the most
// likely reason this file reached for O_EVTONLY instead, so the two values are
// spelled out here rather than imported.
//
// O_SEARCH is this platform's O_PATH: it opens a directory for traversal with
// no read access, and it is the only flag set that can open a directory
// carrying execute permission but not read permission. O_EVTONLY reads like the
// natural analogue and is not one: opening a mode 0o100 directory with it is
// refused EACCES. Both facts were measured on a macOS runner rather than
// reasoned about, because this package had never been compiled for darwin, let
// alone run -- see the Story 3.2 evidence file.
const (
	darwinOExec   = 0x40000000
	darwinOSearch = darwinOExec | unix.O_DIRECTORY
)

// nativeMetadataFlags acquires a metadata handle without read access. The
// entry's kind is not known before the open, so these flags must work for a
// regular file, a directory, or anything else the name turns out to be, and
// O_EVTONLY is the only darwin flag that grants no read on all of them.
// Enumeration is opened separately, only for a directory traversal will list.
func nativeMetadataFlags() int {
	return unix.O_EVTONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC
}

// nativeMetadataFallbackFlags is the second attempt for a metadata open that
// the primary flags refused with EACCES.
//
// A directory carrying execute permission but not read permission is
// traversable, and a selection inside it is inspectable -- Linux gets that from
// O_PATH in a single open. Darwin does not: O_EVTONLY is refused, and only
// O_SEARCH will open it. O_SEARCH implies O_DIRECTORY, so the retry fails
// ENOTDIR on anything that is not a directory, which is exactly when the
// original EACCES was the honest answer.
func nativeMetadataFallbackFlags() (int, bool) {
	return darwinOSearch | unix.O_NOFOLLOW | unix.O_CLOEXEC, true
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

// nativeContentFlags is the only read-granting open in the package. O_NONBLOCK
// is what stops a FIFO substituted for a regular file from parking the
// traversal forever; the caller clears it after fstat proves the object
// regular.
func nativeContentFlags() int {
	return unix.O_RDONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK
}
