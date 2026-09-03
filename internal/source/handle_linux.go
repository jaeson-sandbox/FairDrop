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
