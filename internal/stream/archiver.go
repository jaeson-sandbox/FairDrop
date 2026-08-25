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
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"fairdrop/internal/server"
	"fairdrop/internal/transfer"
)

// defaultBufferSize is the reusable copy buffer used for every payload.
//
// It is a benchmark choice, not a value derived from the file.
// BenchmarkWriteToBufferSizes times only WriteTo -- Prepare and Close are
// stopped out of the timed region -- over a 32 MiB payload with a warm page
// cache. Measured 2026-08-24 on a Ryzen 7 9800X3D: 9.5 GB/s at 32 KiB, 12.7 at
// 64 KiB, 14.4 at 128 KiB, peaking at 15.5 around 256 KiB and falling back to
// 14.8 by 1 MiB. 128 KiB reaches ~93% of that peak for half the per-stream
// memory, and even the slowest arm is ~76x the ~0.125 GB/s of the gigabit LAN
// this buffer actually feeds, so the remaining throughput is not worth the
// bytes. These figures come from io.Discard against a warm page cache, so they
// measure read-plus-copy rather than what an http.ResponseWriter will do.
// Re-measure before trusting them on other hardware.
//
// What is load-bearing is not the number: the buffer is allocated once per
// stream and its size never depends on the payload, so a 16 MiB file and a
// 16 GiB file cost the same payload memory.
const defaultBufferSize = 128 << 10

// fallbackDownloadName is offered when a staged name sanitizes to nothing at
// all, so the receiver is never handed an empty filename.
const fallbackDownloadName = "download"

// maxEmptyReads bounds consecutive zero-byte, nil-error reads before the stream
// is abandoned. It mirrors the constant io.ReadAtLeast uses for the same guard.
const maxEmptyReads = 100

// maxDownloadNameRunes caps the sanitized name. Header values are not unbounded,
// and a staged name is only ever a basename in practice.
const maxDownloadNameRunes = 200

// payloadFile is the descriptor behavior the adapter needs from an opened
// source. *os.File satisfies it; tests inject fakes through the open seam.
type payloadFile interface {
	Read(p []byte) (int, error)
	Stat() (fs.FileInfo, error)
	Close() error
}

type (
	openFunc     func(string) (payloadFile, error)
	lstatFunc    func(string) (fs.FileInfo, error)
	sameFileFunc func(fs.FileInfo, fs.FileInfo) bool
)

// Payloads is the file-only payload adapter. Its function fields are immutable
// per-instance test seams; the zero value uses the operating system.
//
// Directories are refused rather than stubbed: Epic 2 adds that payload kind
// behind this same port without changing the port's shape.
type Payloads struct {
	source     transfer.SourcePort
	open       openFunc
	lstat      lstatFunc
	sameFile   sameFileFunc
	bufferSize int
}

var _ server.PayloadPort = (*Payloads)(nil)

// New returns a file-only payload adapter that re-validates every selection
// through source before opening it.
func New(source transfer.SourcePort) *Payloads {
	return &Payloads{source: source}
}

// Prepare re-validates the staged root, pins its filesystem identity, opens it,
// and derives the wire length from that same descriptor. It runs before any
// response header is written, so every failure returns a coded error and
// retains no descriptor.
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

	// Second layer: kind, size, and modtime are forgeable together, because
	// os.Chtimes restores a modification time onto a replacement. Pin the
	// filesystem identity immediately before the open so it can be compared
	// against the descriptor below: the object verified is then the object
	// streamed, not merely one whose metadata matches.
	identity, err := p.lstatPath(item.Path)
	if err != nil {
		return nil, classifyAccessError(err, "payload source metadata could not be read")
	}
	if identity == nil {
		return nil, transfer.NewError(
			transfer.ErrTransferFailed,
			"payload source metadata is unavailable",
		)
	}
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	// Third layer: the path-based checks above cannot close the window between
	// themselves and the open, so the descriptor is the authority from here on.
	// The wire length comes from its Stat, never from staging metadata, and a
	// swap inside that window fails source_changed instead of streaming
	// unexpected bytes under the staged length.
	file, err := p.openPath(item.Path)
	if err != nil {
		return nil, classifyAccessError(err, "payload source could not be opened")
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
	if !p.sameFileAs(identity, info) {
		return nil, release(file, sourceChangedError())
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

func (p *Payloads) lstatPath(path string) (fs.FileInfo, error) {
	if p != nil && p.lstat != nil {
		return p.lstat(path)
	}
	return os.Lstat(path)
}

func (p *Payloads) sameFileAs(before, after fs.FileInfo) bool {
	if p != nil && p.sameFile != nil {
		return p.sameFile(before, after)
	}
	return os.SameFile(before, after)
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
	streamed  atomic.Bool
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

var _ server.PreparedPayload = (*payload)(nil)

// DownloadName is the sanitized selected basename; it is never a source path.
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
	if p.closed.Load() {
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"payload was released before streaming",
		)
	}
	// Exactly one copy per prepared payload. A second call would resume at the
	// descriptor's current offset and report writing nothing as success.
	if !p.streamed.CompareAndSwap(false, true) {
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"payload was already streamed",
		)
	}
	if err := contextError(ctx); err != nil {
		return err
	}

	// Size is the promise Content-Length is built from, which makes it a bound
	// rather than a hint. Reads are capped at what remains, so a source that
	// grew after Prepare is stopped at the advertised length; a source that
	// shrank fails below instead of reporting a short body as success.
	//
	// One buffer for the whole stream keeps payload memory O(buffer): nothing
	// here is sized from, or grows with, the file.
	buffer := make([]byte, p.bufferSize)
	remaining := p.size
	// io.Reader permits (0, nil) indefinitely. payloadFile is an injectable
	// seam, so a source that never progresses must end the stream rather than
	// spin a core until the context happens to be cancelled.
	stalls := 0
	for remaining > 0 {
		if err := contextError(ctx); err != nil {
			return err
		}
		chunk := buffer
		if int64(len(chunk)) > remaining {
			chunk = buffer[:remaining]
		}
		read, readErr := p.file.Read(chunk)
		if read == 0 && readErr == nil {
			stalls++
			if stalls > maxEmptyReads {
				return transfer.WrapError(
					transfer.ErrTransferFailed,
					"payload source stopped making progress",
					io.ErrNoProgress,
				)
			}
			continue
		}
		stalls = 0
		if read > 0 {
			// Re-check before writing: a cancellation that lands during the
			// read must not put another chunk on the wire.
			if err := contextError(ctx); err != nil {
				return err
			}
			written, writeErr := dst.Write(chunk[:read])
			if writeErr != nil {
				return transfer.WrapError(
					transfer.ErrTransferFailed,
					"payload destination failed mid-stream",
					writeErr,
				)
			}
			if written != read {
				return transfer.WrapError(
					transfer.ErrTransferFailed,
					"payload destination accepted an incomplete write",
					io.ErrShortWrite,
				)
			}
			remaining -= int64(read)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return transfer.WrapError(
				transfer.ErrTransferFailed,
				"payload source failed mid-stream",
				readErr,
			)
		}
	}
	if remaining > 0 {
		return transfer.WrapError(
			transfer.ErrTransferFailed,
			"payload source delivered fewer bytes than its advertised length",
			io.ErrUnexpectedEOF,
		)
	}
	return nil
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

// downloadName reduces a staged name to a bare basename safe to place in a
// response header. StagedItem.Name is data, not a path: separators and ".."
// are discarded rather than traversed, and control characters are removed
// because CR/LF in a filename is a header-injection primitive for the
// Content-Disposition the server writes.
func downloadName(item transfer.StagedItem) string {
	if name := sanitizeDownloadName(item.Name); name != "" {
		return name
	}
	if name := sanitizeDownloadName(filepath.Base(item.Path)); name != "" {
		return name
	}
	return fallbackDownloadName
}

func sanitizeDownloadName(raw string) string {
	// Keep only the final component under either separator convention, so a
	// name carrying a whole path contributes just its last element.
	if index := strings.LastIndexAny(raw, `/\`); index >= 0 {
		raw = raw[index+1:]
	}
	cleaned := strings.Map(func(r rune) rune {
		switch {
		// Cc, plus the Cf class IsControl does not cover -- U+202E
		// RIGHT-TO-LEFT OVERRIDE is the classic extension spoof.
		case unicode.IsControl(r), unicode.Is(unicode.Cf, r):
			return -1
		// Content-Disposition carries the name inside filename="...", so the
		// characters that terminate or extend that parameter cannot survive.
		case strings.ContainsRune(`"';`, r):
			return -1
		// Stripped, not treated as a separator: on Windows a colon introduces
		// a drive ("C:evil.exe") and an NTFS alternate data stream
		// ("report.pdf:payload"). Splitting on it would keep the stream name
		// and discard the real one, so remove it and keep the whole name.
		case r == ':':
			return -1
		}
		return r
	}, raw)
	if runes := []rune(cleaned); len(runes) > maxDownloadNameRunes {
		cleaned = string(runes[:maxDownloadNameRunes])
	}
	// Windows drops trailing dots and spaces from a filename anyway; trimming
	// them here also reduces "." and ".." to nothing while leaving a leading
	// dot, and therefore a legitimate dotfile, intact. Trim the non-breaking
	// space too: it survives the ASCII trim and reads as trailing whitespace.
	return strings.TrimRight(cleaned, ".  ")
}

func sourceChangedError() error {
	return transfer.NewError(
		transfer.ErrSourceChanged,
		"payload source changed after it was staged",
	)
}

func classifyAccessError(err error, safeMessage string) error {
	if errors.Is(err, fs.ErrNotExist) {
		return transfer.WrapError(
			transfer.ErrPathNotFound,
			"payload source no longer exists",
			err,
		)
	}
	return transfer.WrapError(
		transfer.ErrTransferFailed,
		safeMessage,
		err,
	)
}

func contextError(ctx context.Context) error {
	err := ctx.Err()
	if err == nil {
		return nil
	}
	// A deadline is FairDrop's own timeout, not the user's cancel. Reporting it
	// as cancelled would hide it in a UI that treats cancellation as a
	// non-error outcome.
	if errors.Is(err, context.DeadlineExceeded) {
		return transfer.WrapError(
			transfer.ErrTransferFailed,
			"payload operation exceeded its deadline",
			err,
		)
	}
	return transfer.WrapError(
		transfer.ErrCancelled,
		"payload operation was cancelled",
		err,
	)
}
