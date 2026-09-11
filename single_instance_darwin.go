//go:build darwin

package main

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func nativeSingleInstanceLockUsable() bool {
	directory := nativeLockTempDir()
	return directory != "" && darwinLockPathUsable(filepath.Join(directory, singleInstanceLockUniqueID+".lock"))
}

// Match Wails v2.15.0's actual open and flock, without mistaking other errors
// for contention. A contended lock must still reach Wails' handoff. Release
// our probe before Wails opens its own descriptor; never remove the lock inode.
func darwinLockPathUsable(path string) bool {
	return probeDarwinLock(path, syscall.Flock)
}

func probeDarwinLock(path string, flock func(int, int) error) bool {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return false
	}
	info, statErr := file.Stat()
	if statErr != nil || !info.Mode().IsRegular() {
		_ = file.Close()
		return false
	}
	err = flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	closeErr := file.Close()
	return closeErr == nil && (err == nil || errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN))
}
