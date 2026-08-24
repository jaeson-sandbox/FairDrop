// Package stream turns one validated staged item into payload bytes for a
// single authorized HTTP response. It opens nothing until Prepare runs, never
// stages a copy on disk, and copies through one reusable buffer whose size is
// independent of the payload's size.
package stream

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"fairdrop/internal/server"
	"fairdrop/internal/transfer"
)

// defaultBufferSize is the reusable copy buffer used for every payload.
//
// It is a benchmark choice, not a value derived from the file.
// BenchmarkWriteToBufferSizes over a 32 MiB payload (Ryzen 7 9800X3D, warm page
// cache, 200x/count=3) measured 8.8 GB/s at 32 KiB, 11.4 at 64 KiB, 12.9 at
// 128 KiB, and a 13.6-14.1 plateau from 256 KiB through 1 MiB. 128 KiB sits
// within a few percent of that plateau at a quarter of 512 KiB's per-stream
// memory, and is still two orders of magnitude faster than the gigabit LAN this
// buffer actually feeds, so the remaining throughput is not worth the bytes.
//
// What is load-bearing is not the number: the buffer is allocated once per
// stream and its size never depends on the payload, so a 16 MiB file and a
// 16 GiB file cost the same payload memory.
const defaultBufferSize = 128 << 10

// payloadFile is the descriptor behavior the adapter needs from an opened
// source. *os.File satisfies it; tests inject fakes through the open seam.
type payloadFile interface {
	Read(p []byte) (int, error)
	Stat() (fs.FileInfo, error)
	Close() error
}

type openFunc func(string) (payloadFile, error)

// Payloads is the file-only payload adapter. Its function fields are immutable
// per-instance test seams; the zero value uses the operating system.
//
// Directories are refused rather than stubbed: Epic 2 adds that payload kind
// behind this same port without changing the port's shape.
type Payloads struct {
	source     transfer.SourcePort
	open       openFunc
	bufferSize int
}

var _ server.PayloadPort = (*Payloads)(nil)

// New returns a file-only payload adapter that re-validates every selection
// through source before opening it.
func New(source transfer.SourcePort) *Payloads {
	return &Payloads{source: source}
}

// Prepare re-validates the staged root, opens it, and derives the wire length
// from that same descriptor. It runs before any response header is written, so
// every failure returns a coded error and retains no descriptor.
func (p *Payloads) Prepare(ctx context.Context, item transfer.StagedItem) (server.PreparedPayload, error) {
	if ctx == nil {
		return nil, transfer.NewError(
			transfer.ErrTransferFailed,
			"payload preparation requires a context",
		)
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if item.Kind != transfer.ItemFile {
		return nil, transfer.NewError(
			transfer.ErrPathUnsupported,
			"payload preparation supports regular files only",
		)
	}
	if p == nil || p.source == nil {
		return nil, transfer.NewError(
			transfer.ErrTransferFailed,
			"payload preparation requires a source port",
		)
	}

	// First layer: re-run the selection policy at claim time so a root that
	// disappeared, changed kind, or became link-like is refused before anything
	// is opened. Reusing the port keeps one implementation of the link,
	// reparse, ancestor, and special-file rules.
	fresh, err := p.source.Inspect(ctx, item.Path)
	if err != nil {
		return nil, err
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if divergesFromStaged(item, fresh.Kind, fresh.LogicalSize, fresh.ModTime) {
		return nil, sourceChangedError()
	}

	// Second layer: the path-based check above cannot close the window between
	// itself and the open, so the descriptor is the authority from here on. The
	// wire length comes from its Stat, never from staging metadata, and a swap
	// inside that window fails source_changed instead of streaming unexpected
	// bytes under the staged length.
	file, err := p.openPath(item.Path)
	if err != nil {
		return nil, classifyOpenError(err)
	}
	if file == nil {
		return nil, transfer.NewError(
			transfer.ErrTransferFailed,
			"payload source descriptor is unavailable",
		)
	}

	info, statErr := file.Stat()
	if err := contextError(ctx); err != nil {
		return nil, release(file, err)
	}
	if statErr != nil {
		return nil, release(file, transfer.WrapError(
			transfer.ErrTransferFailed,
			"payload descriptor metadata could not be read",
			statErr,
		))
	}
	if info == nil {
		return nil, release(file, transfer.NewError(
			transfer.ErrTransferFailed,
			"payload descriptor metadata is unavailable",
		))
	}
	if !info.Mode().IsRegular() {
		return nil, release(file, transfer.NewError(
			transfer.ErrPathUnsupported,
			"payload descriptor is not a regular file",
		))
	}
	if divergesFromStaged(item, transfer.ItemFile, info.Size(), info.ModTime()) {
		return nil, release(file, sourceChangedError())
	}

	return &payload{
		name:       downloadName(item),
		size:       info.Size(),
		bufferSize: p.bufferLength(),
		file:       file,
	}, nil
}

func (p *Payloads) openPath(path string) (payloadFile, error) {
	if p != nil && p.open != nil {
		return p.open(path)
	}
	// The selected path reaches the native API as a value, unchanged: FairDrop
	// never cleans, rewrites, or shells out to reach a source.
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (p *Payloads) bufferLength() int {
	if p != nil && p.bufferSize > 0 {
		return p.bufferSize
	}
	return defaultBufferSize
}

// payload is one opened regular file bound to exactly one response.
type payload struct {
	name       string
	size       int64
	bufferSize int

	file      payloadFile
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

var _ server.PreparedPayload = (*payload)(nil)

// DownloadName is the selected basename; it is never a source path.
func (p *payload) DownloadName() string { return p.name }

// Size reports the length taken from the opened descriptor. A regular file
// always has a known length, including a zero-byte one.
func (p *payload) Size() (int64, bool) { return p.size, true }

// WriteTo copies the descriptor to dst through one reusable buffer. It creates
// no goroutine, never retries, and appends nothing once a failure is seen.
func (p *payload) WriteTo(ctx context.Context, dst io.Writer) error {
	if ctx == nil {
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"payload streaming requires a context",
		)
	}
	if dst == nil {
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"payload streaming requires a destination",
		)
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	if p.closed.Load() {
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"payload was released before streaming",
		)
	}

	// One buffer for the whole stream keeps payload memory O(buffer): nothing
	// here is sized from, or grows with, the file.
	buffer := make([]byte, p.bufferSize)
	for {
		if err := contextError(ctx); err != nil {
			return err
		}
		read, readErr := p.file.Read(buffer)
		if read > 0 {
			// Re-check before writing: a cancellation that lands during the
			// read must not put another chunk on the wire.
			if err := contextError(ctx); err != nil {
				return err
			}
			written, writeErr := dst.Write(buffer[:read])
			if writeErr != nil {
				return transfer.WrapError(
					transfer.ErrTransferFailed,
					"payload destination failed mid-stream",
					writeErr,
				)
			}
			if written != read {
				return transfer.NewError(
					transfer.ErrTransferFailed,
					"payload destination accepted an incomplete write",
				)
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return transfer.WrapError(
				transfer.ErrTransferFailed,
				"payload source failed mid-stream",
				readErr,
			)
		}
	}
}

// Close releases the descriptor exactly once. Repeated calls are safe no-ops
// that do not re-report the first call's cause.
func (p *payload) Close() error {
	owner := false
	p.closeOnce.Do(func() {
		owner = true
		p.closed.Store(true)
		if err := p.file.Close(); err != nil {
			p.closeErr = transfer.WrapError(
				transfer.ErrTransferFailed,
				"payload descriptor could not be released",
				err,
			)
		}
	})
	if !owner {
		return nil
	}
	return p.closeErr
}

// release discards a descriptor that must not outlive a failed Prepare and
// returns the failure that caused it.
func release(file payloadFile, err error) error {
	_ = file.Close()
	return err
}

func divergesFromStaged(staged transfer.StagedItem, kind transfer.ItemKind, size int64, modTime time.Time) bool {
	return kind != staged.Kind ||
		size != staged.LogicalSize ||
		!modTime.Equal(staged.ModTime)
}

func downloadName(item transfer.StagedItem) string {
	if item.Name != "" {
		return item.Name
	}
	return filepath.Base(item.Path)
}

func sourceChangedError() error {
	return transfer.NewError(
		transfer.ErrSourceChanged,
		"payload source changed after it was staged",
	)
}

func classifyOpenError(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return transfer.WrapError(
			transfer.ErrPathNotFound,
			"payload source no longer exists",
			err,
		)
	}
	return transfer.WrapError(
		transfer.ErrTransferFailed,
		"payload source could not be opened",
		err,
	)
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return transfer.WrapError(
			transfer.ErrCancelled,
			"payload operation was cancelled",
			err,
		)
	}
	return nil
}
