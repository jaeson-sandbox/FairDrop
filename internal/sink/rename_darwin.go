//go:build darwin

package sink

import (
	"errors"

	"golang.org/x/sys/unix"
)

// renameNoReplaceAt renames from to to inside the directory dirfd with
// renameatx_np(RENAME_EXCL), which fails with EEXIST instead of replacing.
//
// A filesystem that does not implement the flag answers ENOTSUP or EINVAL; only
// then does the hard-link form run, and if that cannot work either the rename
// fails closed. There is deliberately no stat-then-rename fallback: that would
// be a check followed by an overwrite, which is the thing this exists to avoid.
func renameNoReplaceAt(dirfd int, from, to string) error {
	err := unix.RenameatxNp(dirfd, from, dirfd, to, unix.RENAME_EXCL)
	if errors.Is(err, unix.ENOTSUP) || errors.Is(err, unix.EINVAL) {
		return linkThenUnlink(dirfd, from, to)
	}
	return err
}
