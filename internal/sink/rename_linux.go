//go:build linux

package sink

import (
	"errors"

	"golang.org/x/sys/unix"
)

// renameNoReplaceAt renames from to to inside the directory dirfd with
// renameat2(RENAME_NOREPLACE), which fails with EEXIST instead of replacing.
//
// A kernel or filesystem without the flag answers ENOSYS or EINVAL; only then
// does the hard-link form run, and if that cannot work either the rename fails
// closed. There is deliberately no stat-then-rename fallback.
func renameNoReplaceAt(dirfd int, from, to string) error {
	err := unix.Renameat2(dirfd, from, dirfd, to, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOTSUP) {
		return linkThenUnlink(dirfd, from, to)
	}
	return err
}
