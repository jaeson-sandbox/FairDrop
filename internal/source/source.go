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

func (i *Inspector) Inspect(ctx context.Context, absolutePath string) (item transfer.StagedItem, returnedErr error) {
	if ctx == nil {
		return transfer.StagedItem{}, transfer.NewError(transfer.ErrTransferFailed, "selection inspection requires a context")
	}
	if err := ctx.Err(); err != nil {
		return transfer.StagedItem{}, cancelledError(err)
	}

	factory := productionHandleFactory()
	if i != nil && i.handles != nil {
		factory = i.handles
	}
	plan, err := factory.Parse(absolutePath)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return transfer.StagedItem{}, cancelledError(ctxErr)
	}
	if err != nil {
		return transfer.StagedItem{}, err
	}

	anchor, err := factory.OpenAnchor(plan)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{anchor}, cancelledError(ctxErr))
	}
	if err != nil {
		return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{anchor}, i.classifyMetadataError(err))
	}
	if anchor == nil {
		return transfer.StagedItem{}, transfer.NewError(transfer.ErrPathUnsupported, "selection root could not be opened")
	}

	// Lexical ancestors remain open so ".." can pop without re-resolving a
	// path. They use metadata/search handles only; enumeration is acquired only
	// for the selected directory.
	stack := []metadataHandle{anchor}
	defer func() {
		returnedErr = closeMetadataHandles(ctx, stack, returnedErr)
		if returnedErr != nil {
			item = transfer.StagedItem{}
		}
	}()

	currentInfo, err := statChecked(ctx, anchor)
	if err != nil {
		return transfer.StagedItem{}, i.classifyOperationError(err)
	}
	if err := rejectUnsupportedInfo(currentInfo); err != nil {
		return transfer.StagedItem{}, err
	}
	selectedName := plan.rootLabel

	for componentIndex, component := range plan.components {
		if err := ctx.Err(); err != nil {
			return transfer.StagedItem{}, cancelledError(err)
		}
		switch component {
		case ".":
			continue
		case "..":
			if len(stack) == 1 {
				return transfer.StagedItem{}, transfer.NewError(transfer.ErrPathUnsupported, "selection escapes its filesystem root")
			}
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if err := closeChecked(ctx, last); err != nil {
				return transfer.StagedItem{}, err
			}
			currentInfo, err = statChecked(ctx, stack[len(stack)-1])
			if err != nil {
				return transfer.StagedItem{}, i.classifyOperationError(err)
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
			return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{metadata}, cancelledError(ctxErr))
		}
		if openErr != nil {
			return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{metadata}, i.classifyMetadataError(openErr))
		}
		if metadata == nil {
			return transfer.StagedItem{}, transfer.NewError(transfer.ErrPathUnsupported, "selection metadata handle is unavailable")
		}

		info, statErr := statChecked(ctx, metadata)
		if statErr != nil {
			return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{metadata}, i.classifyOperationError(statErr))
		}
		if unsupportedErr := rejectUnsupportedInfo(info); unsupportedErr != nil {
			return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{metadata}, unsupportedErr)
		}
		selectedName = info.Name()
		currentInfo = info

		if info.IsDir() {
			search, searchErr := metadata.OpenSearch()
			if ctxErr := ctx.Err(); ctxErr != nil {
				return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{metadata, search}, cancelledError(ctxErr))
			}
			if searchErr != nil {
				return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{metadata, search}, i.classifyMetadataError(searchErr))
			}
			openedInfo, verifyErr := i.verifyOpened(ctx, info, search, false)
			if verifyErr != nil {
				return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{metadata, search}, verifyErr)
			}
			if closeErr := closeChecked(ctx, metadata); closeErr != nil {
				return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{search}, closeErr)
			}
			currentInfo = openedInfo
			stack = append(stack, search)
			continue
		}

		// A regular file may only be final. Keeping it out of the stack means
		// lexical traversal never requests search rights from a non-directory.
		if componentIndex != len(plan.components)-1 {
			primary := transfer.NewError(transfer.ErrPathUnsupported, "selection traverses a non-directory ancestor")
			return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{metadata}, primary)
		}
		if plan.hadTrailingSep {
			primary := transfer.NewError(transfer.ErrPathUnsupported, "selection with a trailing separator is not a directory")
			return transfer.StagedItem{}, closeMetadataHandles(ctx, []metadataHandle{metadata}, primary)
		}
		if closeErr := closeChecked(ctx, metadata); closeErr != nil {
			return transfer.StagedItem{}, closeErr
		}
		return transfer.StagedItem{
			Path: absolutePath, Name: selectedName, Kind: transfer.ItemFile,
			LogicalSize: currentInfo.Size(), ModTime: currentInfo.ModTime(),
		}, nil
	}

	if !currentInfo.IsDir() {
		return transfer.StagedItem{}, transfer.NewError(transfer.ErrPathUnsupported, "selection is not a regular file or directory")
	}
	root := stack[len(stack)-1]
	logicalSize, err := i.inspectDirectory(ctx, root, currentInfo)
	if err != nil {
		return transfer.StagedItem{}, err
	}
	return transfer.StagedItem{
		Path: absolutePath, Name: selectedName, Kind: transfer.ItemDirectory,
		LogicalSize: logicalSize, ModTime: currentInfo.ModTime(),
	}, nil
}

func (i *Inspector) inspectDirectory(ctx context.Context, root metadataHandle, inspected fs.FileInfo) (size int64, returnedErr error) {
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
		handle directoryHandle
		info   fs.FileInfo
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

		metadata, openErr := current.handle.OpenChildMetadata(entries[0].Name())
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
			stack = append(stack, frame{handle: child, info: childInfo})
		default:
			primary := transfer.NewError(transfer.ErrPathUnsupported, "selection contains an unsupported entry")
			return 0, closeMetadataHandles(ctx, []metadataHandle{metadata}, primary)
		}
	}
	return size, nil
}

func (i *Inspector) verifyOpened(ctx context.Context, inspected fs.FileInfo, opened metadataHandle, requireDirectory bool) (fs.FileInfo, error) {
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

func statChecked(ctx context.Context, handle metadataHandle) (fs.FileInfo, error) {
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
