//go:build windows

package sink

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// windowsDir is a directory addressed by path. Windows has no portable openat,
// so unlike the POSIX handle nothing is pinned: instead every name joined onto
// path is a single segment this package generated, the directory was validated
// as a plain directory when it was opened (or created by this process), and
// creation uses CREATE_NEW semantics, which never open or follow an existing
// entry.
type windowsDir struct {
	path string
}

// openDirectory validates absolute component by component with lstat, refusing
// a symbolic link or any reparse point (junction, mount point, OneDrive
// placeholder) at any depth -- the rule the source adapter applies to a
// selection. Extended-length and device namespaces are refused: a destination
// chosen in the native folder dialog never needs them.
func openDirectory(absolute string) (dirHandle, error) {
	if absolute == "" || strings.IndexByte(absolute, 0) >= 0 || !filepath.IsAbs(absolute) {
		return nil, errUnusable
	}
	lower := strings.ToLower(absolute)
	if strings.HasPrefix(lower, `\\.\`) || strings.HasPrefix(lower, `\\?\`) || strings.HasPrefix(lower, `\??\`) {
		return nil, errUnusable
	}
	clean := filepath.Clean(absolute)
	volume := filepath.VolumeName(clean)
	if volume == "" {
		return nil, errUnusable
	}
	rest := strings.TrimPrefix(clean, volume)
	current := volume + string(filepath.Separator)
	for _, component := range strings.Split(rest, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		if component == ".." || component == "." {
			return nil, errUnusable
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fs.ErrNotExist
			}
			return nil, errUnusable
		}
		if linkLike(info) {
			return nil, errLinkLike
		}
		if !info.IsDir() {
			return nil, errNotDirectory
		}
	}
	return &windowsDir{path: clean}, nil
}

// linkLike reports a symbolic link or any reparse point.
func linkLike(info fs.FileInfo) bool {
	if info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
		return true
	}
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && data != nil {
		return data.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
	}
	return false
}

func (d *windowsDir) child(name string) string {
	return filepath.Join(d.path, name)
}

func (d *windowsDir) mkdirExclusive(name string) (dirHandle, error) {
	target := d.child(name)
	// Mkdir fails with ERROR_ALREADY_EXISTS for any existing entry, a link
	// included, and never follows one.
	if err := os.Mkdir(target, 0o755); err != nil {
		return nil, err
	}
	return &windowsDir{path: target}, nil
}

func (d *windowsDir) rmdir(name string) error {
	return os.Remove(d.child(name))
}

func (d *windowsDir) createExclusive(name string) (*os.File, error) {
	// O_EXCL is CREATE_NEW: it fails if the name exists as anything.
	return os.OpenFile(d.child(name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
}

func (d *windowsDir) unlink(name string) error {
	return os.Remove(d.child(name))
}

// renameNoReplace is MoveFileEx without MOVEFILE_REPLACE_EXISTING, which fails
// with ERROR_ALREADY_EXISTS rather than replacing. A failure is passed through
// untranslated except that "exists" is reported as fs.ErrExist, which
// syscall.Errno does for ERROR_ALREADY_EXISTS and ERROR_FILE_EXISTS.
func (d *windowsDir) renameNoReplace(from, to string) error {
	source, err := windows.UTF16PtrFromString(d.child(from))
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(d.child(to))
	if err != nil {
		return err
	}
	return windows.MoveFileEx(source, target, 0)
}

// mark writes the Zone.Identifier alternate data stream with ZoneId=3 (the
// Internet zone), which is what the Mark of the Web is. The stream belongs to
// the file and moves with it through the rename. A volume with no streams (FAT,
// exFAT, some network shares) fails here and the caller reports a warning.
func (d *windowsDir) mark(_ *os.File, name string, _ time.Time) error {
	stream, err := os.OpenFile(d.child(name)+":Zone.Identifier", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, writeErr := stream.WriteString("[ZoneTransfer]\r\nZoneId=3\r\n")
	closeErr := stream.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func (d *windowsDir) availableBytes() (uint64, error) {
	path, err := windows.UTF16PtrFromString(d.path)
	if err != nil {
		return 0, err
	}
	var available, total, free uint64
	if err := windows.GetDiskFreeSpaceEx(path, &available, &total, &free); err != nil {
		return 0, err
	}
	return available, nil
}

func (d *windowsDir) close() error { return nil }
