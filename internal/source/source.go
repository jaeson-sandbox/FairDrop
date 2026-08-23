// Package source implements transfer.SourcePort against the local filesystem.
//
// It is the first gate every transfer passes: one absolute path in, one
// immutable transfer.StagedItem or one coded error out, decided entirely from
// metadata before any network resource exists. It opens nothing, reads no file
// content, walks no directory, follows no link, and never shells out.
package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"fairdrop/internal/transfer"
)

// Inspector is the file-only source adapter.
//
// It is stateless, so the zero value is usable and one instance is safe for
// concurrent use by any number of callers.
type Inspector struct{}

// Compile-time proof that this is the module's single SourcePort
// implementation; the port lives in internal/transfer and is never mirrored
// here.
var _ transfer.SourcePort = (*Inspector)(nil)

// New returns the file-only source adapter.
func New() *Inspector { return &Inspector{} }

// Inspect validates absolutePath and describes it as a staged regular file.
//
// Checks run cheapest-first and each one is a hard stop:
//
//  1. ctx. A cancelled context returns transfer.ErrCancelled before any
//     filesystem call, so a user who cancels during staging never pays for a
//     syscall on a path that may be slow (a disconnected network share) and
//     never sees cancellation reported as a path problem.
//  2. Shape. An empty or non-absolute path is rejected before any syscall.
//     Relative paths are rejected rather than resolved: resolving one against
//     this process's working directory would stage a file the user never
//     pointed at.
//  3. os.Lstat, never os.Stat. Lstat describes the link itself; Stat would
//     silently describe and later open the link's target, which is the exact
//     escape this adapter exists to prevent.
//  4. Mode().IsRegular(). This is mode&ModeType == 0, so a single check rejects
//     directories, symlinks, Windows junctions and other reparse points,
//     devices, pipes, and sockets. Testing os.ModeSymlink instead would be a
//     hole: measured on go1.26.7/windows/amd64, Lstat reports a junction as
//     ModeIrregular and not ModeSymlink, so a symlink-bit check accepts
//     junctions.
//
// absolutePath is carried into the result byte-for-byte. Nothing here cleans,
// case-folds, shortens, or otherwise rewrites it, so spaces, Unicode, long
// Windows paths, extended-length \\?\ prefixes, and UNC paths survive exactly
// as the caller supplied them.
//
// Epic 2 extends this adapter for directories: the seam is the IsRegular check,
// which becomes a branch on info.IsDir() producing transfer.ItemDirectory.
// Until then a directory is a supported-shape-not-yet-supported selection and
// reports ErrPathUnsupported, matching the contract's "host-unsupported path"
// meaning.
func (i *Inspector) Inspect(ctx context.Context, absolutePath string) (transfer.StagedItem, error) {
	if err := ctx.Err(); err != nil {
		// Wrapped, so callers can still distinguish Canceled from
		// DeadlineExceeded while the UI only ever sees "cancelled".
		return transfer.StagedItem{}, transfer.WrapError(
			transfer.ErrCancelled,
			"selection inspection cancelled before filesystem access",
			err,
		)
	}

	if absolutePath == "" {
		return transfer.StagedItem{}, transfer.NewError(
			transfer.ErrInvalidSelection,
			"selection path is empty",
		)
	}
	if !filepath.IsAbs(absolutePath) {
		return transfer.StagedItem{}, transfer.NewError(
			transfer.ErrInvalidSelection,
			"selection path is not absolute",
		)
	}

	info, err := os.Lstat(absolutePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return transfer.StagedItem{}, transfer.WrapError(
				transfer.ErrPathNotFound,
				"selection does not exist",
				err,
			)
		}
		// Everything else -- a permission failure, an invalid name, an
		// unreachable share, a filesystem the host cannot describe -- is a path
		// this host cannot use. The cause is wrapped for local diagnosis and
		// never surfaced: it typically embeds the absolute path.
		return transfer.StagedItem{}, transfer.WrapError(
			transfer.ErrPathUnsupported,
			"selection metadata could not be read",
			err,
		)
	}

	if !info.Mode().IsRegular() {
		return transfer.StagedItem{}, transfer.NewError(
			transfer.ErrPathUnsupported,
			"selection is not a regular file",
		)
	}

	return transfer.StagedItem{
		Path:        absolutePath,
		Name:        filepath.Base(absolutePath),
		Kind:        transfer.ItemFile,
		LogicalSize: info.Size(),
		ModTime:     info.ModTime(),
	}, nil
}
