// Package sink implements transfer.SinkPort: the only code in FairDrop that
// writes a received file to the user's disk.
//
// Everything here is organised around one rule -- FairDrop never overwrites,
// appends to or reopens a file it did not just create -- and the machinery that
// serves it: a destination pinned by descriptor where the platform allows, a
// session subfolder and every file created exclusively and without following
// links, and a rename that fails instead of replacing.
package sink

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"fairdrop/internal/transfer"
)

const (
	// ReserveBytes is the free space that must remain on the destination volume
	// after the declared upload size is subtracted: 3 GiB.
	ReserveBytes int64 = 3 << 30

	// copyBufferBytes is the whole of the memory one file part costs. The part
	// is read and written through this buffer and nothing else: a large upload
	// is never held, and never staged in OS temp storage.
	copyBufferBytes = 64 << 10

	// subfolderLayout is the local-time subfolder name, minutes resolution,
	// with a dot rather than a colon so it is a legal name on every platform.
	subfolderLayout = "2006-01-02 15.04"

	// maxCollisionAttempts bounds every "try the next name" loop so a hostile or
	// broken directory cannot keep one running forever.
	maxCollisionAttempts = 10000

	tempPrefix = ".fairdrop-"
	tempSuffix = ".part"
)

var errClosed = errors.New("sink: the destination is closed")

// Sink is the production transfer.SinkPort.
type Sink struct {
	// The fields below are seams. Production uses New; a test replaces one in
	// place and, per this repo's rule, a separate test drives the default.
	now     func() time.Time
	entropy io.Reader
	open    func(absolute string) (dirHandle, error)
	// available overrides the volume's free-space query. Nil means ask the
	// directory handle, which is what production does.
	available func(dirHandle) (uint64, error)
	// beforeRename runs on the writing goroutine after a temporary file is
	// closed and before it is renamed into place. Test-only; nil in production.
	// It is where a test lands a file on the final name to prove a rename never
	// replaces.
	beforeRename func(directory, finalName string)
	// markOverride replaces the OS marking call. Test-only; nil in production.
	markOverride func() error
}

var _ transfer.SinkPort = (*Sink)(nil)

// New returns the production sink.
func New() *Sink {
	return &Sink{now: time.Now, entropy: rand.Reader, open: openDirectory}
}

// OpenDestination validates absolutePath and returns a destination bound to it.
// It creates nothing.
func (s *Sink) OpenDestination(ctx context.Context, absolutePath string) (transfer.ReceiveDestination, error) {
	if ctx == nil {
		return nil, transfer.NewError(transfer.ErrSetupFailed, "choosing a destination requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, transfer.WrapError(transfer.ErrCancelled, "the transfer was cancelled", err)
	}
	if !filepath.IsAbs(absolutePath) {
		return nil, transfer.NewError(transfer.ErrInvalidSelection, "the destination must be one absolute path")
	}

	root, err := s.openFunc()(absolutePath)
	if err != nil {
		return nil, classifyDestinationError(err)
	}
	if err := ctx.Err(); err != nil {
		_ = root.close()
		return nil, transfer.WrapError(transfer.ErrCancelled, "the transfer was cancelled", err)
	}
	return &destination{
		sink:     s,
		root:     root,
		rootPath: filepath.Clean(absolutePath),
		name:     displayName(absolutePath),
		names:    newNameSet(),
	}, nil
}

func (s *Sink) openFunc() func(string) (dirHandle, error) {
	if s != nil && s.open != nil {
		return s.open
	}
	return openDirectory
}

func (s *Sink) clock() time.Time {
	if s != nil && s.now != nil {
		return s.now()
	}
	return time.Now()
}

// classifyDestinationError maps a platform refusal to the coded errors the
// destination chooser reports. Platform text never crosses: it names the path.
func classifyDestinationError(err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return transfer.NewError(transfer.ErrPathNotFound, "the destination folder does not exist")
	case errors.Is(err, errLinkLike), errors.Is(err, errNotDirectory):
		return transfer.NewError(transfer.ErrPathUnsupported, "the destination is not a plain folder")
	default:
		return transfer.NewError(transfer.ErrPathUnsupported, "the destination folder cannot be written to")
	}
}

// displayName is the basename the UI may show. The root of a volume has no
// basename, so the volume spelling stands in.
func displayName(absolute string) string {
	clean := filepath.Clean(absolute)
	base := filepath.Base(clean)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return clean
	}
	return base
}

// destination is one receive session's write boundary.
//
// Two kinds of state live here and must not be confused. mu guards the small
// bookkeeping every reader (Snapshot, Folder, Name) needs, and is never held
// across a filesystem call, so a stuck volume cannot make the desktop's
// progress read hang. The filesystem work itself is done by the one goroutine
// that is saving a file, and Close coordinates with it through the in-flight
// count and the commit flag rather than a lock held over I/O.
type destination struct {
	sink     *Sink
	root     dirHandle
	rootPath string
	name     string
	names    *nameSet // guarded by mu

	mu   sync.Mutex
	cond *sync.Cond // lazily built on mu; signalled when a commit ends

	closed     bool
	released   bool
	writing    bool // a SaveFile is running; SaveFile is serial per destination
	inflight   int
	committing bool

	sub        dirHandle
	subName    string
	subExists  bool
	saved      int
	markFailed bool
	temp       string // the in-progress temporary name, "" when none

	final     transfer.ReceiveResult
	finalized bool
}

var _ transfer.ReceiveDestination = (*destination)(nil)

func (d *destination) Name() string { return d.name }

// condLocked returns the commit condition, building it on first use. The caller
// holds d.mu.
func (d *destination) condLocked() *sync.Cond {
	if d.cond == nil {
		d.cond = sync.NewCond(&d.mu)
	}
	return d.cond
}

// CheckSpace reports whether declared bytes plus ReserveBytes fit in the space
// available to an unprivileged writer. Equality passes: the contract says at
// least 3 GiB must remain, and exactly 3 GiB is at least 3 GiB.
func (d *destination) CheckSpace(declared int64) error {
	if declared < 0 {
		return transfer.ErrInsufficientSpace
	}
	d.mu.Lock()
	closed := d.closed
	d.mu.Unlock()
	if closed {
		return errClosed
	}
	available, err := d.sink.availableBytes(d.root)
	if err != nil {
		return err
	}
	if !fitsWithReserve(available, declared) {
		return transfer.ErrInsufficientSpace
	}
	return nil
}

// fitsWithReserve is the boundary arithmetic, separated so it can be driven
// directly. It never overflows: declared is subtracted from available, not added
// to the reserve.
func fitsWithReserve(available uint64, declared int64) bool {
	if declared < 0 || uint64(declared) > available {
		return false
	}
	return available-uint64(declared) >= uint64(ReserveBytes)
}

func (s *Sink) availableBytes(handle dirHandle) (uint64, error) {
	if s != nil && s.available != nil {
		return s.available(handle)
	}
	return handle.availableBytes()
}

func (d *destination) Snapshot() transfer.ReceiveResult {
	d.mu.Lock()
	defer d.mu.Unlock()
	return transfer.ReceiveResult{FilesSaved: d.saved, SubfolderExists: d.subExists, MarkingFailed: d.markFailed}
}

func (d *destination) Folder() (string, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.subName == "" || !d.subExists {
		return "", false
	}
	return filepath.Join(d.rootPath, d.subName), true
}

// SaveFile writes one file part: temp create, bounded copy, fsync, mark, close,
// no-replace rename. See transfer.ReceiveDestination.
func (d *destination) SaveFile(ctx context.Context, name string, content io.Reader) (int64, error) {
	if ctx == nil || content == nil {
		return 0, transfer.NewError(transfer.ErrTransferFailed, "saving a file requires a context and content")
	}

	d.mu.Lock()
	if d.closed {
		d.mu.Unlock()
		return 0, errClosed
	}
	if d.writing {
		d.mu.Unlock()
		return 0, transfer.NewError(transfer.ErrTransferFailed, "a received file is already being written")
	}
	d.writing = true
	d.inflight++
	d.mu.Unlock()
	// Whatever way this returns, the last writer out of a closed destination
	// performs the release Close deferred to it.
	defer d.finishWrite()

	sub, err := d.ensureSubfolder()
	if err != nil {
		return 0, transfer.WrapError(transfer.ErrTransferFailed, "the received-files folder could not be created", err)
	}

	base := SanitizeName(name)
	d.mu.Lock()
	want := d.names.claim(base)
	d.mu.Unlock()

	file, tempName, err := d.createTemp(sub)
	if err != nil {
		return 0, transfer.WrapError(transfer.ErrTransferFailed, "a received file could not be started", err)
	}
	d.mu.Lock()
	d.temp = tempName
	d.mu.Unlock()

	written, markFailed, err := d.writeAndClose(ctx, sub, file, tempName, content)
	if err != nil {
		d.discardTemp(sub, tempName)
		return written, err
	}

	if hook := d.sink.beforeRename; hook != nil {
		hook(filepath.Join(d.rootPath, d.currentSubName()), want)
	}
	if err := d.commit(sub, tempName, base, want, markFailed); err != nil {
		d.discardTemp(sub, tempName)
		return written, err
	}
	return written, nil
}

func (d *destination) currentSubName() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.subName
}

// writeAndClose copies content into the temporary file through one bounded
// buffer, flushes it to stable storage, applies the OS marking, and closes it.
// A returned error means the temporary file holds no committed data; the caller
// removes it.
func (d *destination) writeAndClose(ctx context.Context, sub dirHandle, file *os.File, tempName string, content io.Reader) (written int64, markFailed bool, err error) {
	buffer := make([]byte, copyBufferBytes)
	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			_ = file.Close()
			return written, false, transfer.WrapError(transfer.ErrCancelled, "the transfer was cancelled", ctxErr)
		}
		read, readErr := content.Read(buffer)
		if read > 0 {
			count, writeErr := file.Write(buffer[:read])
			written += int64(count)
			if writeErr == nil && count < read {
				writeErr = io.ErrShortWrite
			}
			if writeErr != nil {
				_ = file.Close()
				return written, false, transfer.WrapError(transfer.ErrTransferFailed, "a received file could not be written", writeErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = file.Close()
			return written, false, transfer.WrapError(transfer.ErrTransferFailed, "a received file did not arrive completely", readErr)
		}
	}
	if syncErr := file.Sync(); syncErr != nil {
		_ = file.Close()
		return written, false, transfer.WrapError(transfer.ErrTransferFailed, "a received file could not be flushed to disk", syncErr)
	}
	// Marked before the rename so the file never exists under its final name
	// unmarked. Best-effort: a failure is reported, the file is kept.
	markFailed = d.mark(sub, file, tempName) != nil
	if closeErr := file.Close(); closeErr != nil {
		return written, markFailed, transfer.WrapError(transfer.ErrTransferFailed, "a received file could not be closed", closeErr)
	}
	return written, markFailed, nil
}

func (d *destination) mark(sub dirHandle, file *os.File, tempName string) error {
	if override := d.sink.markOverride; override != nil {
		return override()
	}
	return sub.mark(file, tempName, d.sink.clock())
}

// commit renames the finished temporary file onto its final name without
// replacing anything, then counts it. If the name was taken in the meantime the
// next " (n)" is tried. The closed check and the counter move together under the
// commit flag, so once Close has returned no file can appear under a final name
// and the count it reported is the count on disk.
func (d *destination) commit(sub dirHandle, tempName, base, want string, markFailed bool) error {
	candidate := want
	for attempt := 0; attempt < maxCollisionAttempts; attempt++ {
		d.mu.Lock()
		if d.closed {
			d.mu.Unlock()
			return errClosed
		}
		d.committing = true
		d.mu.Unlock()

		err := sub.renameNoReplace(tempName, candidate)

		d.mu.Lock()
		d.committing = false
		if err == nil {
			d.saved++
			d.temp = ""
			d.markFailed = d.markFailed || markFailed
		}
		d.condLocked().Broadcast()
		if err == nil {
			d.mu.Unlock()
			return nil
		}
		if !errors.Is(err, fs.ErrExist) {
			d.mu.Unlock()
			return transfer.WrapError(transfer.ErrTransferFailed, "a received file could not be put in place", err)
		}
		// Taken in the meantime: pick the next free name in this upload's own
		// numbering and try that.
		candidate = d.names.claim(base)
		d.mu.Unlock()
	}
	return transfer.NewError(transfer.ErrTransferFailed, "no free name was found for a received file")
}

// ensureSubfolder creates the session subfolder on first use and returns it.
// Creation is exclusive: an existing name of any kind, including a link, is
// never reused -- the next " (n)" is tried instead.
func (d *destination) ensureSubfolder() (dirHandle, error) {
	d.mu.Lock()
	if d.sub != nil {
		sub := d.sub
		d.mu.Unlock()
		return sub, nil
	}
	d.mu.Unlock()

	base := d.sink.clock().Format(subfolderLayout)
	var lastErr error
	for attempt := 0; attempt < maxCollisionAttempts; attempt++ {
		name := "FairDrop " + base
		if attempt > 0 {
			name += " (" + strconv.Itoa(attempt) + ")"
		}
		sub, err := d.root.mkdirExclusive(name)
		if err == nil {
			d.mu.Lock()
			if d.closed {
				// Close raced the first file. Do not leave the folder behind.
				d.mu.Unlock()
				_ = sub.close()
				_ = d.root.rmdir(name)
				return nil, errClosed
			}
			d.sub, d.subName, d.subExists = sub, name, true
			d.mu.Unlock()
			return sub, nil
		}
		lastErr = err
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	return nil, lastErr
}

// createTemp exclusively creates a uniquely named temporary file in the
// subfolder, retrying on the (astronomically unlikely) collision.
func (d *destination) createTemp(sub dirHandle) (*os.File, string, error) {
	var lastErr error
	for attempt := 0; attempt < 16; attempt++ {
		var random [8]byte
		entropy := d.sink.entropy
		if entropy == nil {
			entropy = rand.Reader
		}
		if _, err := io.ReadFull(entropy, random[:]); err != nil {
			return nil, "", err
		}
		name := tempPrefix + hex.EncodeToString(random[:]) + tempSuffix
		file, err := sub.createExclusive(name)
		if err == nil {
			return file, name, nil
		}
		lastErr = err
		if !errors.Is(err, fs.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", lastErr
}

// discardTemp removes the in-progress temporary file, and only that file. A
// failed upload keeps every saved file and removes exactly the one it was
// writing.
func (d *destination) discardTemp(sub dirHandle, tempName string) {
	_ = sub.unlink(tempName)
	d.mu.Lock()
	if d.temp == tempName {
		d.temp = ""
	}
	d.mu.Unlock()
}

// finishWrite ends one SaveFile. If Close ran while it was writing, Close left
// the release of the handles to the last writer out, and this is that writer.
func (d *destination) finishWrite() {
	d.mu.Lock()
	d.inflight--
	d.writing = false
	run := d.closed && d.inflight == 0 && !d.released
	if run {
		d.released = true
	}
	d.mu.Unlock()
	if run {
		d.release()
	}
}

// Close ends the destination. See transfer.ReceiveDestination.
func (d *destination) Close() (transfer.ReceiveResult, error) {
	d.mu.Lock()
	if d.finalized {
		result := d.final
		d.mu.Unlock()
		return result, nil
	}
	d.closed = true
	// A commit already underway is allowed to finish, and Close waits for it, so
	// the count reported below is exactly the files on disk. The wait is short:
	// it is one rename. Every other writer is refused at its next commit.
	for d.committing {
		d.condLocked().Wait()
	}
	run := d.inflight == 0 && !d.released
	if run {
		d.released = true
	}
	// With nothing saved the subfolder is about to be removed, so it does not
	// count as existing. Computed before the removal so a Close that races a
	// writer reports the same thing whichever of them runs the release.
	result := transfer.ReceiveResult{
		FilesSaved:      d.saved,
		SubfolderExists: d.subExists && d.saved > 0,
		MarkingFailed:   d.markFailed,
	}
	d.final, d.finalized = result, true
	d.mu.Unlock()

	var err error
	if run {
		err = d.release()
	}
	return result, err
}

// release removes the subfolder when nothing was saved and closes the handles.
// It runs exactly once, on whichever of Close and the last writer got there.
func (d *destination) release() error {
	d.mu.Lock()
	sub, name, saved := d.sub, d.subName, d.saved
	d.mu.Unlock()

	var firstErr error
	if sub != nil {
		if saved == 0 {
			// The in-progress temp file of a writer that lost the race is
			// already gone: release runs after the last writer has left.
			if err := d.root.rmdir(name); err != nil && !errors.Is(err, fs.ErrNotExist) {
				firstErr = err
			} else {
				d.mu.Lock()
				d.subExists = false
				d.mu.Unlock()
			}
		}
		if err := sub.close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := d.root.close(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
