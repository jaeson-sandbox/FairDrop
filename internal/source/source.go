// Package source implements transfer.SourcePort using local filesystem
// metadata only. It never opens a selected file or follows a selected link.
package source

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"

	"fairdrop/internal/transfer"
)

type lstatFunc func(string) (fs.FileInfo, error)
type reparseFunc func(fs.FileInfo) (bool, error)
type unreachableNetworkFunc func(error) bool
type openDirectoryFunc func(string) (directoryReader, error)
type sameFileFunc func(fs.FileInfo, fs.FileInfo) bool

type directoryReader interface {
	Stat() (fs.FileInfo, error)
	ReadDir(int) ([]fs.DirEntry, error)
	Close() error
}

// Reading one entry at a time is an intentional fixed-size batch. A recursive
// reader stack can then retain only one entry plus one handle per depth, rather
// than one batch per depth or an index proportional to the tree width.
const directoryReadBatchSize = 1

// Inspector is the local source adapter. Its function fields are
// immutable per-instance test seams; the zero value uses the operating system.
type Inspector struct {
	lstat                lstatFunc
	isReparse            reparseFunc
	isUnreachableNetwork unreachableNetworkFunc
	openDirectory        openDirectoryFunc
	sameFile             sameFileFunc
}

var _ transfer.SourcePort = (*Inspector)(nil)

// New returns a local source inspector.
func New() *Inspector { return &Inspector{} }

// Inspect validates absolutePath and returns metadata for one regular file or
// one safely traversable directory. Directory inspection retains no entry
// index and opens no regular file.
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

	lookupPath := trimMetadataSeparators(absolutePath)
	paths := syntacticPrefixes(lookupPath)
	var selectedInfo fs.FileInfo
	var selectedRootName string
	resolvedNames := make([]string, 0, len(paths))
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
		if ctxErr := ctx.Err(); ctxErr != nil {
			return transfer.StagedItem{}, cancelledError(ctxErr)
		}
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

		// FileInfo.Name follows the spelling passed to Lstat, so a final "."
		// or ".." can report that token instead of the directory's real name.
		// Resolve only the display-name stack from metadata already inspected;
		// filesystem lookups and StagedItem.Path keep the caller's exact syntax.
		switch filepath.Base(path) {
		case ".":
			if len(resolvedNames) == 0 {
				resolvedNames = append(resolvedNames, info.Name())
			}
		case "..":
			if len(resolvedNames) > 1 {
				resolvedNames = resolvedNames[:len(resolvedNames)-1]
			} else if len(resolvedNames) == 0 {
				resolvedNames = append(resolvedNames, info.Name())
			}
		default:
			resolvedNames = append(resolvedNames, info.Name())
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

		selectedInfo = info
		if len(resolvedNames) > 0 {
			selectedRootName = resolvedNames[len(resolvedNames)-1]
		} else {
			selectedRootName = info.Name()
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return transfer.StagedItem{}, transfer.NewError(
				transfer.ErrPathUnsupported,
				"selection is not a regular file or directory",
			)
		}
	}

	if selectedInfo == nil {
		// filepath.IsAbs excludes this in practice, but keep the boundary typed
		// if an operating system ever accepts an absolute path with no metadata.
		return transfer.StagedItem{}, transfer.NewError(
			transfer.ErrInvalidSelection,
			"selection path has no inspectable component",
		)
	}

	if selectedInfo.Mode().IsRegular() {
		// A trailing separator denotes a directory to native path APIs. It is
		// trimmed only so link-like directory roots cannot hide behind it; do not
		// silently reinterpret a regular file as that directory selection.
		if lookupPath != absolutePath {
			return transfer.StagedItem{}, transfer.NewError(
				transfer.ErrPathUnsupported,
				"selection with a trailing separator is not a directory",
			)
		}
		return transfer.StagedItem{
			Path:        absolutePath,
			Name:        filepath.Base(lookupPath),
			Kind:        transfer.ItemFile,
			LogicalSize: selectedInfo.Size(),
			ModTime:     selectedInfo.ModTime(),
		}, nil
	}

	logicalSize, err := i.inspectDirectory(ctx, lookupPath, selectedInfo)
	if err != nil {
		return transfer.StagedItem{}, err
	}
	return transfer.StagedItem{
		Path:        absolutePath,
		Name:        selectedRootName,
		Kind:        transfer.ItemDirectory,
		LogicalSize: logicalSize,
		ModTime:     selectedInfo.ModTime(),
	}, nil
}

func (i *Inspector) inspectDirectory(ctx context.Context, root string, rootInfo fs.FileInfo) (size int64, returnedErr error) {
	rootReader, err := i.openDirectoryPath(root)
	if ctxErr := ctx.Err(); ctxErr != nil {
		if rootReader != nil {
			_ = rootReader.Close()
		}
		return 0, cancelledError(ctxErr)
	}
	if err != nil {
		if rootReader != nil {
			_ = rootReader.Close()
		}
		return 0, i.classifyMetadataError(err)
	}
	if rootReader == nil {
		return 0, transfer.NewError(transfer.ErrPathUnsupported, "selection directory could not be opened")
	}

	type frame struct {
		path   string
		reader directoryReader
		info   fs.FileInfo
	}
	rootOpened, err := i.verifyOpenedDirectory(ctx, root, rootInfo, rootReader)
	if err != nil {
		_ = rootReader.Close()
		return 0, err
	}
	stack := []frame{{path: root, reader: rootReader, info: rootOpened}}
	defer func() {
		for index := len(stack) - 1; index >= 0; index-- {
			if err := stack[index].reader.Close(); returnedErr == nil && err != nil {
				returnedErr = transfer.WrapError(transfer.ErrTransferFailed, "selection directory could not be closed", err)
			}
		}
	}()

	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return 0, cancelledError(err)
		}

		current := &stack[len(stack)-1]
		entries, readErr := current.reader.ReadDir(directoryReadBatchSize)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, cancelledError(ctxErr)
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return 0, i.classifyMetadataError(readErr)
		}
		if len(entries) == 0 {
			reader := current.reader
			stack = stack[:len(stack)-1]
			if err := reader.Close(); err != nil {
				return 0, transfer.WrapError(transfer.ErrTransferFailed, "selection directory could not be closed", err)
			}
			continue
		}

		entryPath := appendPath(current.path, entries[0].Name())
		entryInfo, err := i.lstatPath(entryPath)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, cancelledError(ctxErr)
		}
		if err != nil {
			return 0, i.classifyMetadataError(err)
		}
		if entryInfo == nil {
			return 0, transfer.NewError(transfer.ErrPathUnsupported, "selection entry metadata is unavailable")
		}

		reparse, err := i.reparsePoint(entryInfo)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, cancelledError(ctxErr)
		}
		if err != nil {
			return 0, transfer.WrapError(transfer.ErrPathUnsupported, "selection platform metadata is unavailable", err)
		}
		if reparse || entryInfo.Mode()&os.ModeSymlink != 0 {
			return 0, transfer.NewError(transfer.ErrPathUnsupported, "selection contains a link-like entry")
		}

		switch {
		case entryInfo.Mode().IsRegular():
			entrySize := entryInfo.Size()
			if entrySize < 0 || size > math.MaxInt64-entrySize {
				return 0, transfer.NewError(transfer.ErrTransferFailed, "selection logical size is invalid")
			}
			size += entrySize
		case entryInfo.IsDir():
			reader, err := i.openDirectoryPath(entryPath)
			if ctxErr := ctx.Err(); ctxErr != nil {
				if reader != nil {
					_ = reader.Close()
				}
				return 0, cancelledError(ctxErr)
			}
			if err != nil {
				if reader != nil {
					_ = reader.Close()
				}
				return 0, i.classifyMetadataError(err)
			}
			if reader == nil {
				return 0, transfer.NewError(transfer.ErrPathUnsupported, "selection directory could not be opened")
			}
			opened, err := i.verifyOpenedDirectory(ctx, entryPath, entryInfo, reader)
			if err != nil {
				_ = reader.Close()
				return 0, err
			}
			for _, ancestor := range stack {
				if i.sameFileInfo(ancestor.info, opened) {
					_ = reader.Close()
					return 0, transfer.NewError(transfer.ErrPathUnsupported, "selection contains a directory cycle")
				}
			}
			stack = append(stack, frame{path: entryPath, reader: reader, info: opened})
		default:
			return 0, transfer.NewError(transfer.ErrPathUnsupported, "selection contains an unsupported entry")
		}
	}

	return size, nil
}

func (i *Inspector) verifyOpenedDirectory(
	ctx context.Context,
	path string,
	inspected fs.FileInfo,
	reader directoryReader,
) (fs.FileInfo, error) {
	opened, err := reader.Stat()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, cancelledError(ctxErr)
	}
	if err != nil {
		return nil, i.classifyMetadataError(err)
	}
	if opened == nil || !opened.IsDir() || !i.sameFileInfo(inspected, opened) {
		return nil, transfer.NewError(transfer.ErrSourceChanged, "selection directory changed before enumeration")
	}

	// Revalidate the path after the handle is open. Identity comparison alone
	// cannot prove that the path is still non-link-like: a junction or bind-like
	// replacement may resolve to the same underlying directory.
	current, err := i.lstatPath(path)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, cancelledError(ctxErr)
	}
	if err != nil {
		return nil, i.classifyMetadataError(err)
	}
	if current == nil {
		return nil, transfer.NewError(transfer.ErrPathUnsupported, "selection directory metadata is unavailable")
	}
	reparse, err := i.reparsePoint(current)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, cancelledError(ctxErr)
	}
	if err != nil {
		return nil, transfer.WrapError(
			transfer.ErrPathUnsupported,
			"selection platform metadata is unavailable",
			err,
		)
	}
	if reparse || current.Mode()&os.ModeSymlink != 0 {
		return nil, transfer.NewError(transfer.ErrPathUnsupported, "selection directory became link-like before enumeration")
	}
	if !current.IsDir() || !i.sameFileInfo(current, opened) {
		return nil, transfer.NewError(transfer.ErrSourceChanged, "selection directory changed before enumeration")
	}
	return opened, nil
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

func (i *Inspector) openDirectoryPath(path string) (directoryReader, error) {
	if i != nil && i.openDirectory != nil {
		return i.openDirectory(path)
	}
	return os.Open(path)
}

func (i *Inspector) sameFileInfo(first, second fs.FileInfo) bool {
	if i != nil && i.sameFile != nil {
		return i.sameFile(first, second)
	}
	return os.SameFile(first, second)
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

// trimMetadataSeparators removes only trailing native separators beyond a
// volume/share root. It deliberately does not call filepath.Clean: callers may
// use meaningful spelling such as "..", and StagedItem.Path keeps that exact
// spelling regardless of the metadata lookup path.
func trimMetadataSeparators(path string) string {
	volumeLength := len(filepath.VolumeName(path))
	rootEnd := volumeLength
	for rootEnd < len(path) && os.IsPathSeparator(path[rootEnd]) {
		rootEnd++
	}
	end := len(path)
	for end > rootEnd && os.IsPathSeparator(path[end-1]) {
		end--
	}
	return path[:end]
}

func appendPath(parent, name string) string {
	if len(parent) > 0 && os.IsPathSeparator(parent[len(parent)-1]) {
		return parent + name
	}
	return parent + string(os.PathSeparator) + name
}
