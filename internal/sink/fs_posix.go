//go:build !windows

package sink

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// posixDir is a directory held open by descriptor. Every operation is relative
// to it, so the path that led here is never resolved again.
type posixDir struct {
	fd   int
	once sync.Once
	err  error
}

const (
	directoryFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC
	fileMode       = 0o644
	directoryMode  = 0o755
)

// openDirectory walks absolute from the filesystem root one component at a
// time, opening each with O_NOFOLLOW|O_DIRECTORY relative to the previous one.
// A symbolic link anywhere on the path is therefore refused, not resolved --
// the same rule the source adapter applies to a selection -- and the returned
// descriptor pins the directory itself.
func openDirectory(absolute string) (dirHandle, error) {
	if absolute == "" || absolute[0] != '/' || strings.IndexByte(absolute, 0) >= 0 {
		return nil, errUnusable
	}
	current, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, component := range strings.Split(absolute, "/") {
		switch component {
		case "", ".":
			continue
		case "..":
			// Dot-dot would be resolved by the kernel after the fact, which is
			// the walk this function exists to avoid.
			_ = unix.Close(current)
			return nil, errUnusable
		}
		next, openErr := unix.Openat(current, component, directoryFlags, 0)
		if openErr != nil {
			classified := classifyOpenError(current, component, openErr)
			_ = unix.Close(current)
			return nil, classified
		}
		_ = unix.Close(current)
		current = next
	}
	// A folder the user cannot write into is refused here, before a QR code
	// exists, rather than surfacing as an upload that saved nothing.
	if err := unix.Faccessat(current, ".", unix.W_OK|unix.X_OK, 0); err != nil {
		_ = unix.Close(current)
		return nil, errUnusable
	}
	return &posixDir{fd: current}, nil
}

// classifyOpenError separates the three refusals the caller reports differently.
// ENOTDIR is ambiguous: some kernels answer O_NOFOLLOW|O_DIRECTORY on a link
// with ENOTDIR rather than ELOOP, so the entry is looked at without following.
func classifyOpenError(parent int, name string, err error) error {
	switch {
	case errors.Is(err, unix.ENOENT):
		return fs.ErrNotExist
	case errors.Is(err, unix.ELOOP):
		return errLinkLike
	case errors.Is(err, unix.ENOTDIR):
		var status unix.Stat_t
		if unix.Fstatat(parent, name, &status, unix.AT_SYMLINK_NOFOLLOW) == nil && status.Mode&unix.S_IFMT == unix.S_IFLNK {
			return errLinkLike
		}
		return errNotDirectory
	default:
		return errUnusable
	}
}

func (d *posixDir) mkdirExclusive(name string) (dirHandle, error) {
	if err := unix.Mkdirat(d.fd, name, directoryMode); err != nil {
		return nil, err
	}
	child, err := unix.Openat(d.fd, name, directoryFlags, 0)
	if err != nil {
		// Created but not openable: do not leave an empty directory behind.
		_ = unix.Unlinkat(d.fd, name, unix.AT_REMOVEDIR)
		return nil, err
	}
	return &posixDir{fd: child}, nil
}

func (d *posixDir) rmdir(name string) error {
	return unix.Unlinkat(d.fd, name, unix.AT_REMOVEDIR)
}

func (d *posixDir) createExclusive(name string) (*os.File, error) {
	fd, err := unix.Openat(d.fd, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, fileMode)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fs.ErrInvalid
	}
	return file, nil
}

func (d *posixDir) unlink(name string) error {
	return unix.Unlinkat(d.fd, name, 0)
}

func (d *posixDir) renameNoReplace(from, to string) error {
	return renameNoReplaceAt(d.fd, from, to)
}

func (d *posixDir) mark(file *os.File, _ string, now time.Time) error {
	return markFile(file, now)
}

func (d *posixDir) availableBytes() (uint64, error) {
	var status unix.Statfs_t
	if err := unix.Fstatfs(d.fd, &status); err != nil {
		return 0, err
	}
	// Bavail is the space an unprivileged writer can use, which is the space
	// that matters here; Bfree includes blocks reserved for the superuser.
	return uint64(status.Bavail) * uint64(status.Bsize), nil
}

func (d *posixDir) close() error {
	d.once.Do(func() { d.err = unix.Close(d.fd) })
	return d.err
}

// linkThenUnlink is the portable no-replace rename: link(2) refuses to replace
// an existing name, so linking the final name and then removing the temporary
// one moves the file without ever overwriting. It is only a fallback, for a
// filesystem that offers no exclusive rename, and it fails closed (returning the
// link error) on one that offers no hard links either.
func linkThenUnlink(dirfd int, from, to string) error {
	if err := unix.Linkat(dirfd, from, dirfd, to, 0); err != nil {
		return err
	}
	// The data is already reachable under its final name. A failure to remove
	// the temporary name leaves a stray file, never a lost or replaced one.
	_ = unix.Unlinkat(dirfd, from, 0)
	return nil
}
