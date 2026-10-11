//go:build windows

package sink

import (
	"crypto/rand"
	"encoding/hex"
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
	// A folder the user cannot write into is refused here, before a QR code
	// exists, as the POSIX handle does with faccessat. Windows has no
	// descriptor-relative access query that honours ACLs, so the only honest
	// answer is to try.
	if err := probeWritable(clean); err != nil {
		return nil, errUnusable
	}
	return &windowsDir{path: clean}, nil
}

// writableProbeAttempts bounds the (astronomically unlikely) name collision.
const writableProbeAttempts = 4

// probeWritable proves directory is one a child directory can be created in --
// the first thing a receive does -- by creating a uniquely named, empty probe
// directory and removing it again. Creation is os.Mkdir, which is CREATE_NEW: it
// fails for any existing name, a link included, and never follows one, so the
// probe cannot land anywhere but directly inside directory. The probe is
// removed on every path that created it; if it cannot be removed the folder is
// reported unusable rather than left holding a stray entry.
func probeWritable(directory string) error {
	var lastErr error
	for attempt := 0; attempt < writableProbeAttempts; attempt++ {
		var random [8]byte
		if _, err := rand.Read(random[:]); err != nil {
			return err
		}
		probe := filepath.Join(directory, tempPrefix+"probe-"+hex.EncodeToString(random[:]))
		err := os.Mkdir(probe, 0o755)
		if err == nil {
			return os.Remove(probe)
		}
		lastErr = err
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return lastErr
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
//
// Both paths are spelled in extended-length form. Go's os.* calls lift MAX_PATH
// for a long path by themselves, but this raw call does not, so a long final
// name under a deep destination would otherwise fail here and discard a file
// that had been received and flushed in full.
func (d *windowsDir) renameNoReplace(from, to string) error {
	source, err := windows.UTF16PtrFromString(extendedLengthPath(d.child(from)))
	if err != nil {
		return err
	}
	target, err := windows.UTF16PtrFromString(extendedLengthPath(d.child(to)))
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
	// Extended-length only once the path is long enough to need it (Go's own
	// threshold), so the common case keeps the plain spelling GetDiskFreeSpaceEx
	// is documented against.
	query := d.path
	if len(query) >= 248 {
		query = extendedLengthPath(query)
	}
	path, err := windows.UTF16PtrFromString(query)
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
