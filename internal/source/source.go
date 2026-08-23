// Package source implements transfer.SourcePort using local filesystem
// metadata only. It never opens a selected file or follows a selected link.
package source

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"fairdrop/internal/transfer"
)

type lstatFunc func(string) (fs.FileInfo, error)
type reparseFunc func(fs.FileInfo) (bool, error)
type unreachableNetworkFunc func(error) bool

// Inspector is the file-only source adapter. Its function fields are
// immutable per-instance test seams; the zero value uses the operating system.
type Inspector struct {
	lstat                lstatFunc
	isReparse            reparseFunc
	isUnreachableNetwork unreachableNetworkFunc
}

var _ transfer.SourcePort = (*Inspector)(nil)

// New returns a file-only source inspector.
func New() *Inspector { return &Inspector{} }

// Inspect validates absolutePath and returns metadata for one regular file.
func (i *Inspector) Inspect(ctx context.Context, absolutePath string) (transfer.StagedItem, error) {
	if ctx == nil {
		return transfer.StagedItem{}, transfer.NewError(
			transfer.ErrTransferFailed,
			"selection inspection requires a context",
		)
	}
	if err := ctx.Err(); err != nil {
		return transfer.StagedItem{}, cancelledError(err)
	}
	if absolutePath == "" || !filepath.IsAbs(absolutePath) {
		return transfer.StagedItem{}, transfer.NewError(
			transfer.ErrInvalidSelection,
			"selection must be one absolute path",
		)
	}

	paths := syntacticPrefixes(absolutePath)
	for index, path := range paths {
		if err := ctx.Err(); err != nil {
			return transfer.StagedItem{}, cancelledError(err)
		}

		info, err := i.lstatPath(path)
		// Cancellation takes precedence when it arrives during a filesystem
		// inspection, regardless of the metadata result returned alongside it.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return transfer.StagedItem{}, cancelledError(ctxErr)
		}
		if err != nil {
			return transfer.StagedItem{}, i.classifyMetadataError(err)
		}
		if info == nil {
			return transfer.StagedItem{}, transfer.NewError(
				transfer.ErrPathUnsupported,
				"selection metadata is unavailable",
			)
		}

		// Mode bits are not a complete reparse defense on Windows: Go reports
		// some reparse tags as regular. Check the native attribute separately
		// before applying the portable file/directory mode gate below.
		reparse, err := i.reparsePoint(info)
		if err != nil {
			return transfer.StagedItem{}, transfer.WrapError(
				transfer.ErrPathUnsupported,
				"selection platform metadata is unavailable",
				err,
			)
		}
		if reparse {
			return transfer.StagedItem{}, transfer.NewError(
				transfer.ErrPathUnsupported,
				"selection traverses a reparse point",
			)
		}

		selected := index == len(paths)-1
		if !selected {
			// Lstat protects only the component it names. Validating each lexical
			// prefix first prevents a link-like ancestor from being followed by
			// the final lookup, including an ancestor written before "..".
			if !info.IsDir() {
				return transfer.StagedItem{}, transfer.NewError(
					transfer.ErrPathUnsupported,
					"selection traverses a non-directory ancestor",
				)
			}
			continue
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

	// filepath.IsAbs excludes this in practice, but keep the boundary typed if
	// an operating system ever accepts an absolute path with no components.
	return transfer.StagedItem{}, transfer.NewError(
		transfer.ErrInvalidSelection,
		"selection path has no inspectable component",
	)
}

func (i *Inspector) lstatPath(path string) (fs.FileInfo, error) {
	if i != nil && i.lstat != nil {
		return i.lstat(path)
	}
	return os.Lstat(path)
}

func (i *Inspector) reparsePoint(info fs.FileInfo) (bool, error) {
	if i != nil && i.isReparse != nil {
		return i.isReparse(info)
	}
	return platformReparsePoint(info)
}

func (i *Inspector) classifyMetadataError(err error) error {
	unreachable := platformUnreachableNetworkError
	if i != nil && i.isUnreachableNetwork != nil {
		unreachable = i.isUnreachableNetwork
	}
	// Windows deliberately makes ERROR_BAD_NETPATH satisfy fs.ErrNotExist.
	// Keep this narrower classification first so an unreachable host/share is
	// never mislabeled as a selected item that disappeared.
	if unreachable(err) {
		return transfer.WrapError(
			transfer.ErrPathUnsupported,
			"selection network path is unreachable",
			err,
		)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return transfer.WrapError(
			transfer.ErrPathNotFound,
			"selection does not exist",
			err,
		)
	}
	return transfer.WrapError(
		transfer.ErrPathUnsupported,
		"selection metadata could not be read",
		err,
	)
}

func cancelledError(cause error) error {
	return transfer.WrapError(
		transfer.ErrCancelled,
		"selection inspection cancelled",
		cause,
	)
}

// syntacticPrefixes returns the root, each syntactically traversed ancestor,
// and the selected path in traversal order. Prefixes are slices of the caller's
// string, so components such as ".." are not cleaned away and the final value
// is byte-for-byte identical to the input.
func syntacticPrefixes(path string) []string {
	volumeLength := len(filepath.VolumeName(path))
	rootEnd := volumeLength
	for rootEnd < len(path) && os.IsPathSeparator(path[rootEnd]) {
		rootEnd++
	}

	paths := make([]string, 0, 4)
	if rootEnd > 0 && rootEnd < len(path) {
		paths = append(paths, path[:rootEnd])
	}
	for index := rootEnd; index < len(path); index++ {
		if !os.IsPathSeparator(path[index]) {
			continue
		}
		if index == rootEnd || os.IsPathSeparator(path[index-1]) {
			continue
		}
		paths = append(paths, path[:index])
	}
	if len(paths) == 0 || paths[len(paths)-1] != path {
		paths = append(paths, path)
	}
	return paths
}
