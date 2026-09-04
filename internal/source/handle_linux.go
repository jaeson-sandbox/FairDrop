//go:build linux

package source

import "golang.org/x/sys/unix"

func nativeMetadataFlags() int {
	return unix.O_PATH | unix.O_NOFOLLOW | unix.O_CLOEXEC
}

func nativeSearchFlags() int {
	return unix.O_PATH | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_DIRECTORY
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
