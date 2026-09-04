//go:build darwin

package source

import "golang.org/x/sys/unix"

// O_EVTONLY acquires a metadata/search handle without read access. Enumeration
// is opened separately only for a directory that traversal will list.
func nativeMetadataFlags() int {
	return unix.O_EVTONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC
}

func nativeSearchFlags() int {
	return unix.O_EVTONLY | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_DIRECTORY
}

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
