package source

import (
	"context"
	"io/fs"
	"sync"

	"fairdrop/internal/transfer"
)

func directoryDepthError() error {
	return transfer.NewError(transfer.ErrPathUnsupported, "selection exceeds the directory handle limit")
}

func sourceFault(visit transfer.SourceVisitor, message string) error {
	code := transfer.ErrSetupFailed
	if visit != nil {
		code = transfer.ErrTransferFailed
	}
	return transfer.NewError(code, message)
}

// PrepareDirectory acquires only search rights; enumeration remains lazy.
func (i *Inspector) PrepareDirectory(ctx context.Context, path string) (transfer.PreparedDirectory, error) {
	var pin metadataHandle
	err := i.withSelection(ctx, path, func(selected selection) error {
		if selected.isFile {
			return transfer.NewError(transfer.ErrSourceChanged, "selection is no longer a directory")
		}
		if selected.retained >= maxRetainedDirectoryHandles {
			return directoryDepthError()
		}
		opened, err := selected.handle.OpenSearch()
		if ctx.Err() != nil {
			return closeMetadataHandles(ctx, []metadataHandle{opened}, cancelledError(ctx.Err()))
		}
		if err != nil {
			return closeMetadataHandles(ctx, []metadataHandle{opened}, i.classifyMetadataError(err))
		}
		if _, err := i.verifyOpened(ctx, selected.info, opened, true); err != nil {
			return closeMetadataHandles(ctx, []metadataHandle{opened}, err)
		}
		pin = opened
		return nil
	})
	if err != nil {
		return nil, closeMetadataHandles(ctx, []metadataHandle{pin}, err)
	}
	return &preparedDirectory{inspector: i, path: path, pin: pin}, nil
}

type preparedDirectory struct {
	mu        sync.Mutex
	inspector *Inspector
	path      string
	pin       metadataHandle
}

func (p *preparedDirectory) Walk(ctx context.Context, visit transfer.SourceVisitor) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pin == nil {
		return transfer.WrapError(transfer.ErrTransferFailed, "prepared directory was closed", fs.ErrClosed)
	}
	if visit == nil {
		return transfer.NewError(transfer.ErrTransferFailed, "selection walk requires a visitor")
	}
	return p.inspector.withSelectionRetained(ctx, p.path, 1, func(selected selection) error {
		if selected.isFile {
			return transfer.NewError(transfer.ErrSourceChanged, "prepared directory was replaced")
		}
		if _, err := p.inspector.verifyOpened(ctx, selected.info, p.pin, true); err != nil {
			return err
		}
		_, err := p.inspector.walkDirectory(ctx, selected.handle, selected.info, visit, selected.retained)
		return err
	})
}

func (p *preparedDirectory) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pin == nil {
		return nil
	}
	pin := p.pin
	p.pin = nil
	return closeChecked(context.Background(), pin)
}
