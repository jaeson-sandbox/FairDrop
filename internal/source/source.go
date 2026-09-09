// Package source implements transfer.SourcePort with native no-follow,
// handle-relative filesystem inspection.
package source

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"

	"fairdrop/internal/transfer"
)

type sameFileFunc func(fs.FileInfo, fs.FileInfo) bool
type unreachableNetworkFunc func(error) bool

const directoryReadBatchSize = 1

// Inspector is the local source adapter. Production uses native no-follow
// handles. The private seams allow deterministic algorithm tests without
// weakening the default path.
type Inspector struct {
	handles              handleFactory
	sameFile             sameFileFunc
	isUnreachableNetwork unreachableNetworkFunc
}

var _ transfer.SourcePort = (*Inspector)(nil)

func New() *Inspector { return &Inspector{} }

// Inspect describes the selection. A directory is traversed once to sum its
// logical size; nothing it contains is opened for reading.
func (i *Inspector) Inspect(ctx context.Context, absolutePath string) (transfer.StagedItem, error) {
	var item transfer.StagedItem
	err := i.withSelection(ctx, absolutePath, func(selected selection) error {
		if selected.isFile {
			item = transfer.StagedItem{
				Path: absolutePath, Name: selected.name, Kind: transfer.ItemFile,
				LogicalSize: selected.info.Size(), ModTime: selected.info.ModTime(),
			}
			return nil
		}
		logicalSize, err := i.walkDirectory(ctx, selected.handle, selected.info, nil)
		if err != nil {
			return err
		}
		item = transfer.StagedItem{
			Path: absolutePath, Name: selected.name, Kind: transfer.ItemDirectory,
			LogicalSize: logicalSize, ModTime: selected.info.ModTime(),
		}
		return nil
	})
	if err != nil {
		return transfer.StagedItem{}, err
	}
	return item, nil
}

// Walk re-runs the whole selection policy and then emits every nested entry
// through visit. It is the streaming half of the same traversal Inspect uses:
// one enumeration handle per active depth, one entry at a time, every handle
// closed in reverse order on every exit, and no per-entry state kept after the
// visitor returns.
func (i *Inspector) Walk(ctx context.Context, absolutePath string, visit transfer.SourceVisitor) error {
	if visit == nil {
		return transfer.NewError(transfer.ErrTransferFailed, "selection walk requires a visitor")
	}
	return i.withSelection(ctx, absolutePath, func(selected selection) error {
		if selected.isFile {
			return transfer.NewError(transfer.ErrPathUnsupported, "selection walk requires a directory")
		}
		_, err := i.walkDirectory(ctx, selected.handle, selected.info, visit)
		return err
	})
}

// selection is the validated result of the lexical walk, handed to a caller
// while the handle stack that proves it is still open. handle is the search
// handle for a directory selection and nil for a file, whose own metadata
// handle is already closed by the time use runs.
type selection struct {
	handle metadataHandle
	info   fs.FileInfo
	name   string
	isFile bool
}

// withSelection parses the path, opens the anchor, walks the components on a
// no-follow handle stack, and calls use with the validated selection. Every
// handle it opened is closed in reverse order before it returns, whatever use
// did, so no caller can outlive a descriptor it did not open.
func (i *Inspector) withSelection(ctx context.Context, absolutePath string, use func(selection) error) (returnedErr error) {
	if ctx == nil {
		return transfer.NewError(transfer.ErrTransferFailed, "selection inspection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return cancelledError(err)
	}

	factory := productionHandleFactory()
	if i != nil && i.handles != nil {
		factory = i.handles
	}
	plan, err := factory.Parse(absolutePath)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return cancelledError(ctxErr)
	}
	if err != nil {
		return err
	}

	anchor, err := factory.OpenAnchor(plan)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return closeMetadataHandles(ctx, []metadataHandle{anchor}, cancelledError(ctxErr))
	}
	if err != nil {
		return closeMetadataHandles(ctx, []metadataHandle{anchor}, i.classifyMetadataError(err))
	}
	if anchor == nil {
		return transfer.NewError(transfer.ErrPathUnsupported, "selection root could not be opened")
	}

	// Lexical ancestors remain open so ".." can pop without re-resolving a
	// path. They use metadata/search handles only; enumeration is acquired only
	// for the selected directory.
	stack := []metadataHandle{anchor}
	defer func() {
		returnedErr = closeMetadataHandles(ctx, stack, returnedErr)
	}()

	currentInfo, err := statChecked(ctx, anchor)
	if err != nil {
		return i.classifyOperationError(err)
	}
	if err := rejectUnsupportedInfo(currentInfo); err != nil {
		return err
	}
	selectedName := plan.rootLabel

	for componentIndex, component := range plan.components {
		if err := ctx.Err(); err != nil {
			return cancelledError(err)
		}
		switch component {
		case ".":
			continue
		case "..":
			if len(stack) == 1 {
				return transfer.NewError(transfer.ErrPathUnsupported, "selection escapes its filesystem root")
			}
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if err := closeChecked(ctx, last); err != nil {
				return err
			}
			currentInfo, err = statChecked(ctx, stack[len(stack)-1])
			if err != nil {
				return i.classifyOperationError(err)
			}
			if len(stack) == 1 {
				selectedName = plan.rootLabel
			} else {
				selectedName = currentInfo.Name()
			}
			continue
		}

		parent := stack[len(stack)-1]
		metadata, openErr := parent.OpenChildMetadata(component)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return closeMetadataHandles(ctx, []metadataHandle{metadata}, cancelledError(ctxErr))
		}
		if openErr != nil {
			return closeMetadataHandles(ctx, []metadataHandle{metadata}, i.classifyMetadataError(openErr))
		}
		if metadata == nil {
			return transfer.NewError(transfer.ErrPathUnsupported, "selection metadata handle is unavailable")
		}

		info, statErr := statChecked(ctx, metadata)
		if statErr != nil {
			return closeMetadataHandles(ctx, []metadataHandle{metadata}, i.classifyOperationError(statErr))
		}
		if unsupportedErr := rejectUnsupportedInfo(info); unsupportedErr != nil {
			return closeMetadataHandles(ctx, []metadataHandle{metadata}, unsupportedErr)
		}
		selectedName = info.Name()
		currentInfo = info

		if info.IsDir() {
			search, searchErr := metadata.OpenSearch()
			if ctxErr := ctx.Err(); ctxErr != nil {
				return closeMetadataHandles(ctx, []metadataHandle{metadata, search}, cancelledError(ctxErr))
			}
			if searchErr != nil {
				return closeMetadataHandles(ctx, []metadataHandle{metadata, search}, i.classifyMetadataError(searchErr))
			}
			openedInfo, verifyErr := i.verifyOpened(ctx, info, search, false)
			if verifyErr != nil {
				return closeMetadataHandles(ctx, []metadataHandle{metadata, search}, verifyErr)
			}
			if closeErr := closeChecked(ctx, metadata); closeErr != nil {
				return closeMetadataHandles(ctx, []metadataHandle{search}, closeErr)
			}
			currentInfo = openedInfo
			stack = append(stack, search)
			continue
		}

		// A regular file may only be final. Keeping it out of the stack means
		// lexical traversal never requests search rights from a non-directory.
		if componentIndex != len(plan.components)-1 {
			primary := transfer.NewError(transfer.ErrPathUnsupported, "selection traverses a non-directory ancestor")
			return closeMetadataHandles(ctx, []metadataHandle{metadata}, primary)
		}
		if plan.hadTrailingSep {
			primary := transfer.NewError(transfer.ErrPathUnsupported, "selection with a trailing separator is not a directory")
			return closeMetadataHandles(ctx, []metadataHandle{metadata}, primary)
		}
		if closeErr := closeChecked(ctx, metadata); closeErr != nil {
			return closeErr
		}
		return use(selection{info: currentInfo, name: selectedName, isFile: true})
	}

	// A "." or ".." component re-Stats an already-open ancestor rather than
	// re-running the component checks, so the selection can still land on
	// something that stopped being a directory between descent and pop. This is
	// the last gate before enumeration rights are requested.
	if !currentInfo.IsDir() {
		return transfer.NewError(transfer.ErrPathUnsupported, "selection is not a regular file or directory")
	}
	return use(selection{handle: stack[len(stack)-1], info: currentInfo, name: selectedName})
}

// walkDirectory is the one tree traversal in the package. Inspect runs it with
// a nil visitor to sum logical size; Walk runs it with a visitor to emit
// entries for streaming. Keeping both on the same code means the link, reparse,
// special-file, identity, cycle, batching, and cancellation rules cannot drift
// apart between preflight and the stream that follows it.
//
// Each frame carries the root-relative name of the directory it enumerates, so
// an entry name is accumulated as the walk descends rather than reconstructed
// from a path afterwards. State is one enumeration handle per active depth plus
// the single entry being visited: nothing per-entry survives the iteration.
func (i *Inspector) walkDirectory(ctx context.Context, root metadataHandle, inspected fs.FileInfo, visit transfer.SourceVisitor) (size int64, returnedErr error) {
	opened, err := root.OpenEnumeration()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return 0, closeMetadataHandles(ctx, []metadataHandle{opened}, cancelledError(ctxErr))
	}
	if err != nil {
		return 0, closeMetadataHandles(ctx, []metadataHandle{opened}, i.classifyMetadataError(err))
	}
	if opened == nil {
		return 0, transfer.NewError(transfer.ErrPathUnsupported, "selection directory could not be opened")
	}
	openedInfo, err := i.verifyOpened(ctx, inspected, opened, true)
	if err != nil {
		return 0, closeMetadataHandles(ctx, []metadataHandle{opened}, err)
	}

	type frame struct {
		handle   directoryHandle
		info     fs.FileInfo
		relative string
	}
	stack := []frame{{handle: opened, info: openedInfo}}
	defer func() {
		for index := len(stack) - 1; index >= 0; index-- {
			returnedErr = preferCloseResult(ctx, returnedErr, stack[index].handle.Close())
		}
	}()

	for len(stack) > 0 {
		if err := ctx.Err(); err != nil {
			return 0, cancelledError(err)
		}
		current := &stack[len(stack)-1]
		entries, readErr := current.handle.ReadDir(directoryReadBatchSize)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, cancelledError(ctxErr)
		}
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return 0, i.classifyMetadataError(readErr)
		}
		if len(entries) == 0 {
			finished := current.handle
			stack = stack[:len(stack)-1]
			if err := closeChecked(ctx, finished); err != nil {
				return 0, err
			}
			continue
		}
		if len(entries) != 1 {
			return 0, transfer.NewError(transfer.ErrTransferFailed, "selection enumeration exceeded its fixed batch")
		}

		// Read out of the frame before anything can append to the stack: growing
		// it may move the element this pointer refers to.
		parent := current.handle
		entryName := entries[0].Name()
		relative, relativeErr := childRelativeName(current.relative, entryName)
		if relativeErr != nil {
			return 0, relativeErr
		}

		metadata, openErr := parent.OpenChildMetadata(entryName)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return 0, closeMetadataHandles(ctx, []metadataHandle{metadata}, cancelledError(ctxErr))
		}
		if openErr != nil {
			return 0, closeMetadataHandles(ctx, []metadataHandle{metadata}, i.classifyMetadataError(openErr))
		}
		if metadata == nil {
			return 0, transfer.NewError(transfer.ErrPathUnsupported, "selection entry metadata is unavailable")
		}
		info, statErr := statChecked(ctx, metadata)
		if statErr != nil {
			return 0, closeMetadataHandles(ctx, []metadataHandle{metadata}, i.classifyOperationError(statErr))
		}
		if unsupportedErr := rejectUnsupportedInfo(info); unsupportedErr != nil {
			return 0, closeMetadataHandles(ctx, []metadataHandle{metadata}, unsupportedErr)
		}

		switch {
		case info.Mode().IsRegular():
			entrySize := info.Size()
			if visit != nil {
				if emitErr := i.emitFile(ctx, parent, entryName, relative, info, visit); emitErr != nil {
					return 0, closeMetadataHandles(ctx, []metadataHandle{metadata}, emitErr)
				}
			}
			closeErr := closeChecked(ctx, metadata)
			if closeErr != nil {
				return 0, closeErr
			}
			if entrySize < 0 || size > math.MaxInt64-entrySize {
				return 0, transfer.NewError(transfer.ErrTransferFailed, "selection logical size is invalid")
			}
			size += entrySize
		case info.IsDir():
			child, childErr := metadata.OpenEnumeration()
			if ctxErr := ctx.Err(); ctxErr != nil {
				return 0, closeMetadataHandles(ctx, []metadataHandle{metadata, child}, cancelledError(ctxErr))
			}
			if childErr != nil {
				return 0, closeMetadataHandles(ctx, []metadataHandle{metadata, child}, i.classifyMetadataError(childErr))
			}
			childInfo, verifyErr := i.verifyOpened(ctx, info, child, true)
			if verifyErr != nil {
				return 0, closeMetadataHandles(ctx, []metadataHandle{metadata, child}, verifyErr)
			}
			if closeErr := closeChecked(ctx, metadata); closeErr != nil {
				return 0, closeMetadataHandles(ctx, []metadataHandle{child}, closeErr)
			}
			for _, ancestor := range stack {
				if i.sameFileInfo(ancestor.info, childInfo) {
					primary := transfer.NewError(transfer.ErrPathUnsupported, "selection contains a directory cycle")
					return 0, closeMetadataHandles(ctx, []metadataHandle{child}, primary)
				}
			}
			stack = append(stack, frame{handle: child, info: childInfo, relative: relative})
			// Emitted after the push so the deferred unwind owns the handle even
			// when the visitor fails, and before its children are read so a
			// consumer always sees a directory before anything inside it.
			if visit != nil {
				entry := transfer.SourceEntry{
					RelativePath: relative, Kind: transfer.ItemDirectory, ModTime: childInfo.ModTime(),
				}
				if visitErr := visit(entry, nil); visitErr != nil {
					return 0, visitErr
				}
			}
		default:
			primary := transfer.NewError(transfer.ErrPathUnsupported, "selection contains an unsupported entry")
			return 0, closeMetadataHandles(ctx, []metadataHandle{metadata}, primary)
		}
	}
	return size, nil
}

// emitFile opens one entry's bytes, proves the descriptor is still the object
// the metadata handle described, lends it to the visitor, and closes it before
// returning. The visitor never receives anything it could keep: the reader is
// invalidated the moment this returns, so a consumer that stashed it reads
// nothing rather than reading a descriptor this package has already released.
func (i *Inspector) emitFile(
	ctx context.Context,
	parent directoryHandle,
	entryName, relative string,
	inspected fs.FileInfo,
	visit transfer.SourceVisitor,
) error {
	content, err := parent.OpenChildContent(entryName)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return closeContentHandle(ctx, content, cancelledError(ctxErr))
	}
	if err != nil {
		return closeContentHandle(ctx, content, i.classifyMetadataError(err))
	}
	if content == nil {
		return transfer.NewError(transfer.ErrPathUnsupported, "selection entry content is unavailable")
	}
	// The descriptor, not the name, is the authority from here: a swap inside
	// the window between the metadata open and this one fails identity rather
	// than streaming another object's bytes under this entry's name.
	openedInfo, verifyErr := i.verifyOpened(ctx, inspected, content, false)
	if verifyErr != nil {
		return closeContentHandle(ctx, content, verifyErr)
	}
	if !openedInfo.Mode().IsRegular() {
		primary := transfer.NewError(transfer.ErrPathUnsupported, "selection entry is not a regular file")
		return closeContentHandle(ctx, content, primary)
	}
	borrowed := &borrowedContent{handle: content}
	entry := transfer.SourceEntry{
		RelativePath: relative,
		Kind:         transfer.ItemFile,
		Size:         openedInfo.Size(),
		ModTime:      openedInfo.ModTime(),
	}
	visitErr := visit(entry, borrowed)
	borrowed.release()
	return closeContentHandle(ctx, content, visitErr)
}

// borrowedContent is the io.Reader a visitor receives. It exposes Read and
// nothing else, so a visitor cannot close, stat, or seek a descriptor this
// package still owns, and it stops reading the moment the loan ends.
type borrowedContent struct {
	handle   contentHandle
	returned bool
}

func (b *borrowedContent) Read(p []byte) (int, error) {
	if b == nil || b.returned || b.handle == nil {
		return 0, fs.ErrClosed
	}
	return b.handle.Read(p)
}

func (b *borrowedContent) release() { b.returned = true }

// childRelativeName accumulates one entry's root-relative, slash-separated
// name. Directory entry names come from the filesystem, so they are checked
// rather than trusted: a name that is empty, a dot element, separator-bearing,
// volume-qualified, or NUL-bearing would become a traversal primitive once it
// reached an archive a receiver extracts.
func childRelativeName(parent, name string) (string, error) {
	if name == "" || name == "." || name == ".." ||
		strings.ContainsAny(name, "/\\") ||
		strings.IndexByte(name, 0) >= 0 ||
		filepath.IsAbs(name) || filepath.VolumeName(name) != "" {
		return "", transfer.NewError(transfer.ErrPathUnsupported, "selection contains an unsupported entry name")
	}
	slashed := filepath.ToSlash(name)
	if parent == "" {
		return slashed, nil
	}
	return parent + "/" + slashed, nil
}

func closeContentHandle(ctx context.Context, handle contentHandle, primary error) error {
	if handle == nil {
		return primary
	}
	return preferCloseResult(ctx, primary, handle.Close())
}

func (i *Inspector) verifyOpened(ctx context.Context, inspected fs.FileInfo, opened statHandle, requireDirectory bool) (fs.FileInfo, error) {
	if opened == nil {
		return nil, transfer.NewError(transfer.ErrPathUnsupported, "selection handle is unavailable")
	}
	info, err := statChecked(ctx, opened)
	if err != nil {
		return nil, i.classifyOperationError(err)
	}
	if err := rejectUnsupportedInfo(info); err != nil {
		return nil, err
	}
	if requireDirectory && !info.IsDir() {
		return nil, transfer.NewError(transfer.ErrSourceChanged, "selection directory changed before enumeration")
	}
	if !i.sameFileInfo(inspected, info) {
		return nil, transfer.NewError(transfer.ErrSourceChanged, "selection identity changed before open")
	}
	return info, nil
}

func rejectUnsupportedInfo(info fs.FileInfo) error {
	if info == nil {
		return transfer.NewError(transfer.ErrPathUnsupported, "selection metadata is unavailable")
	}
	reparse, err := platformReparsePoint(info)
	if err != nil {
		return transfer.WrapError(transfer.ErrPathUnsupported, "selection platform metadata is unavailable", err)
	}
	if reparse || info.Mode()&os.ModeSymlink != 0 {
		return transfer.NewError(transfer.ErrPathUnsupported, "selection contains a link-like entry")
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return transfer.NewError(transfer.ErrPathUnsupported, "selection contains an unsupported entry")
	}
	return nil
}

func statChecked(ctx context.Context, handle statHandle) (fs.FileInfo, error) {
	info, err := handle.Stat()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, cancelledError(ctxErr)
	}
	return info, err
}

func closeChecked(ctx context.Context, handle metadataHandle) error {
	if handle == nil {
		return nil
	}
	err := handle.Close()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return cancelledError(ctxErr)
	}
	if err != nil {
		return transfer.WrapError(transfer.ErrTransferFailed, "selection handle could not be closed", err)
	}
	return nil
}

func closeMetadataHandles(ctx context.Context, handles []metadataHandle, primary error) error {
	for index := len(handles) - 1; index >= 0; index-- {
		if handles[index] == nil {
			continue
		}
		primary = preferCloseResult(ctx, primary, handles[index].Close())
	}
	return primary
}

func preferCloseResult(ctx context.Context, primary, closeErr error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return cancelledError(ctxErr)
	}
	if primary != nil {
		return primary
	}
	if closeErr != nil {
		return transfer.WrapError(transfer.ErrTransferFailed, "selection handle could not be closed", closeErr)
	}
	return nil
}

func (i *Inspector) sameFileInfo(first, second fs.FileInfo) bool {
	if i != nil && i.sameFile != nil {
		return i.sameFile(first, second)
	}
	return os.SameFile(first, second)
}

func (i *Inspector) classifyOperationError(err error) error {
	return i.classifyMetadataError(err)
}

func (i *Inspector) classifyMetadataError(err error) error {
	if err == nil {
		return nil
	}
	var coded transfer.CodedError
	if errors.As(err, &coded) {
		return err
	}
	unreachable := platformUnreachableNetworkError
	if i != nil && i.isUnreachableNetwork != nil {
		unreachable = i.isUnreachableNetwork
	}
	if unreachable(err) {
		return transfer.WrapError(transfer.ErrPathUnsupported, "selection network path is unreachable", err)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return transfer.WrapError(transfer.ErrPathNotFound, "selection does not exist", err)
	}
	return transfer.WrapError(transfer.ErrPathUnsupported, "selection metadata could not be read", err)
}

func cancelledError(cause error) error {
	return transfer.WrapError(transfer.ErrCancelled, "selection inspection cancelled", cause)
}
