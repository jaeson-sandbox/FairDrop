package stream

import (
	"archive/zip"
	"context"
	"errors"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"fairdrop/internal/server"
	"fairdrop/internal/transfer"
)

// archiveExtension is what a staged folder arrives as. It is part of the
// download name, never of an entry name inside the archive.
const archiveExtension = ".zip"

// errArchiveHalted stops the ZIP writer from emitting anything more once the
// stream has failed. It never reaches a caller: it exists so that finalizing a
// failed archive writes no central directory.
var errArchiveHalted = errors.New("archive halted before completion")

// archive is one staged directory bound to exactly one response.
//
// It holds no descriptor. Every handle the stream needs is opened, used, and
// closed inside SourcePort.Walk, which returns only after releasing all of
// them, so there is nothing for Close to release and nothing that can outlive
// WriteTo.
type archive struct {
	name       string
	root       string
	path       string
	modTime    time.Time
	source     transfer.SourcePort
	bufferSize int

	streamed  atomic.Bool
	closed    atomic.Bool
	closeOnce sync.Once
}

var _ server.PreparedPayload = (*archive)(nil)

// DownloadName is the sanitized root plus ".zip"; it is never a source path.
func (a *archive) DownloadName() string { return a.name }

// Size is unknown by construction. A ZIP's length depends on how well every
// entry compresses, which is not known until the last one has been written, and
// the only honest alternatives to "unknown" are buffering the whole archive or
// putting a Content-Length on the wire the body cannot match.
func (a *archive) Size() (int64, bool) { return 0, false }

// WriteTo builds the archive on a worker and copies it to dst as it is
// produced. The worker is joined before this returns on every path -- success,
// cancellation, receiver disconnect, unsafe entry, read failure -- so no
// goroutine of this payload's outlives the call, and both ends of the pipe are
// closed by the time it does.
func (a *archive) WriteTo(ctx context.Context, dst io.Writer) error {
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
	if a.closed.Load() {
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"payload was released before streaming",
		)
	}
	// Exactly one archive per prepared payload. A second call would append a
	// second archive to a response that already carries one and report it as
	// success.
	if !a.streamed.CompareAndSwap(false, true) {
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"payload was already streamed",
		)
	}
	if err := contextError(ctx); err != nil {
		return err
	}

	reader, writer := io.Pipe()
	produced := make(chan error, 1)
	go func() { produced <- a.produce(ctx, writer) }()

	destinationErr, pipeErr := a.drain(ctx, dst, reader)

	// Whatever ended the copy, the worker must not stay parked writing into a
	// pipe nobody is reading: closing the read end fails its next write and lets
	// it unwind. On the success path it has already finished.
	_ = reader.CloseWithError(io.ErrClosedPipe)
	workerErr := <-produced

	switch {
	// The destination and the context are the outcomes the server acts on, so
	// they outrank a worker error that is usually just the broken pipe they
	// caused.
	case destinationErr != nil:
		return destinationErr
	case workerErr != nil:
		return workerErr
	case pipeErr != nil:
		return transfer.WrapError(
			transfer.ErrTransferFailed,
			"payload archive ended before it was complete",
			pipeErr,
		)
	default:
		return nil
	}
}

// produce writes the whole archive into the pipe and always closes the ZIP
// writer before the pipe writer, because zip.Writer.Close is what emits the
// central directory: closing the pipe first would deliver a headless archive no
// reader can open.
//
// On failure the same ordering must not hand the receiver a *valid* archive
// that silently omits whatever failed, so the underlying writer is halted
// first. zip.Writer.Close then still runs, and still runs first, but every byte
// it produces is refused.
func (a *archive) produce(ctx context.Context, writer *io.PipeWriter) error {
	halting := &haltableWriter{dst: writer}
	archiveWriter := zip.NewWriter(halting)

	err := a.writeEntries(ctx, archiveWriter)
	if err != nil {
		halting.halt()
	}
	closeErr := archiveWriter.Close()
	if err == nil && closeErr != nil {
		err = transfer.WrapError(
			transfer.ErrTransferFailed,
			"payload archive could not be finalized",
			closeErr,
		)
	}
	_ = writer.CloseWithError(err)
	return err
}

// drain copies the produced archive to the destination through one buffer
// allocated once for the whole stream.
//
// A read failure from the pipe is the worker's own error arriving backwards, so
// it is reported separately from a destination failure and yields to the
// authoritative copy the join collects.
func (a *archive) drain(ctx context.Context, dst io.Writer, src io.Reader) (destinationErr, pipeErr error) {
	buffer := make([]byte, a.bufferSize)
	for {
		if err := contextError(ctx); err != nil {
			return err, nil
		}
		read, readErr := src.Read(buffer)
		if read > 0 {
			// Re-check before writing: a cancellation that lands during the read
			// must not put another chunk on the wire.
			if err := contextError(ctx); err != nil {
				return err, nil
			}
			written, writeErr := dst.Write(buffer[:read])
			if writeErr != nil {
				return transfer.WrapError(
					transfer.ErrTransferFailed,
					"payload destination failed mid-stream",
					writeErr,
				), nil
			}
			if written != read {
				return transfer.WrapError(
					transfer.ErrTransferFailed,
					"payload destination accepted an incomplete write",
					io.ErrShortWrite,
				), nil
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil, nil
			}
			return nil, readErr
		}
	}
}

// writeEntries emits the archive: one top-level directory, then every entry the
// source walk reaches, each below that directory.
func (a *archive) writeEntries(ctx context.Context, out *zip.Writer) error {
	// The root entry is written even for an empty folder, so an empty selection
	// still arrives as a folder rather than as an archive of nothing.
	if err := writeArchiveDirectory(out, a.root+"/", a.modTime); err != nil {
		return err
	}
	// One buffer for every entry in the tree: nothing here is sized from, or
	// grows with, the number of entries or the bytes inside them.
	buffer := make([]byte, a.bufferSize)
	return a.source.Walk(ctx, a.path, func(entry transfer.SourceEntry, content io.Reader) error {
		if err := contextError(ctx); err != nil {
			return err
		}
		name, err := archiveEntryName(a.root, entry.RelativePath)
		if err != nil {
			return err
		}
		switch entry.Kind {
		case transfer.ItemDirectory:
			return writeArchiveDirectory(out, name+"/", entry.ModTime)
		case transfer.ItemFile:
			return writeArchiveFile(ctx, out, name, entry.ModTime, content, buffer)
		default:
			return transfer.NewError(
				transfer.ErrPathUnsupported,
				"payload archive received an unsupported entry kind",
			)
		}
	})
}

// Close releases nothing because the payload holds nothing: the tree walk owns
// every descriptor and has already closed them by the time WriteTo returns. It
// stays idempotent and callable without a preceding WriteTo so the server's
// single-Close ownership rule needs no special case for a directory.
func (a *archive) Close() error {
	a.closeOnce.Do(func() { a.closed.Store(true) })
	return nil
}

// haltableWriter refuses everything after halt. It is what lets a failed
// archive be finalized without finalizing: zip.Writer.Close still runs, and
// still runs before the pipe closes, but its central directory never reaches
// the receiver.
type haltableWriter struct {
	dst    io.Writer
	halted atomic.Bool
}

func (h *haltableWriter) Write(p []byte) (int, error) {
	if h.halted.Load() {
		return 0, errArchiveHalted
	}
	return h.dst.Write(p)
}

func (h *haltableWriter) halt() { h.halted.Store(true) }

// writeArchiveDirectory records a directory entry. The trailing slash is what
// marks it as one in the ZIP format; the mode makes an extracting tool create
// it as a directory rather than an empty file.
func writeArchiveDirectory(out *zip.Writer, name string, modTime time.Time) error {
	header := &zip.FileHeader{Name: name, Method: zip.Store, Modified: modTime}
	header.SetMode(fs.ModeDir | 0o755)
	if _, err := out.CreateHeader(header); err != nil {
		return transfer.WrapError(
			transfer.ErrTransferFailed,
			"payload archive directory could not be written",
			err,
		)
	}
	return nil
}

// writeArchiveFile copies one borrowed reader into one archive entry. The
// reader is valid only for this call, so nothing is deferred past it, and the
// buffer is the caller's: a per-entry buffer would make payload memory grow
// with the tree.
func writeArchiveFile(
	ctx context.Context,
	out *zip.Writer,
	name string,
	modTime time.Time,
	content io.Reader,
	buffer []byte,
) error {
	if content == nil {
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"payload archive entry arrived without content",
		)
	}
	// No size is declared up front: the ZIP writer emits a data descriptor
	// after the entry instead, which is what lets a file be compressed while it
	// is being read rather than measured first.
	entry, err := out.CreateHeader(&zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: modTime,
	})
	if err != nil {
		return transfer.WrapError(
			transfer.ErrTransferFailed,
			"payload archive entry could not be started",
			err,
		)
	}
	// io.Reader permits (0, nil) indefinitely, and the reader is borrowed from
	// an injectable port, so a source that never progresses must end the stream
	// rather than spin a core until the context happens to be cancelled.
	stalls := 0
	for {
		if err := contextError(ctx); err != nil {
			return err
		}
		read, readErr := content.Read(buffer)
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
			if err := contextError(ctx); err != nil {
				return err
			}
			written, writeErr := entry.Write(buffer[:read])
			if writeErr != nil {
				return transfer.WrapError(
					transfer.ErrTransferFailed,
					"payload archive entry could not be written",
					writeErr,
				)
			}
			if written != read {
				return transfer.WrapError(
					transfer.ErrTransferFailed,
					"payload archive accepted an incomplete write",
					io.ErrShortWrite,
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

// archiveEntryName places one relative source name under the single top-level
// root.
//
// The source already promises relative, slash-separated names. This re-checks
// anyway because it is the last gate before a name is written into an archive
// somebody else will extract: an absolute, volume-qualified, empty, or dot-dot
// bearing entry name is a path-traversal primitive on the receiving side, and
// the cost of proving it here is a string scan per entry.
func archiveEntryName(root, relative string) (string, error) {
	if relative == "" ||
		strings.ContainsAny(relative, "\\\x00") ||
		strings.HasPrefix(relative, "/") ||
		strings.HasSuffix(relative, "/") ||
		filepath.VolumeName(relative) != "" {
		return "", unsafeArchiveEntryName()
	}
	for _, segment := range strings.Split(relative, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", unsafeArchiveEntryName()
		}
	}
	return root + "/" + relative, nil
}

// unsafeArchiveEntryName names no path: the entry name is derived from the
// source tree, and the receiver has no business learning what it looked like.
func unsafeArchiveEntryName() error {
	return transfer.NewError(
		transfer.ErrPathUnsupported,
		"payload archive entry name is not safe to place under the root",
	)
}
