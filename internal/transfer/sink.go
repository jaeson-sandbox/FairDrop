package transfer

import (
	"context"
	"errors"
	"io"
)

// ErrInsufficientSpace is the sentinel ReceiveDestination.CheckSpace returns
// when the declared upload size plus the free-space reserve does not fit on the
// destination volume. It is a plain error value rather than an ErrorCode
// because it never crosses the UI boundary as a command failure: the server
// turns it into a fixed 413 page and a notice.
var ErrInsufficientSpace = errors.New("transfer: declared upload exceeds the free space on the destination")

// ReceiveResult is a point-in-time view of what a destination has saved.
//
// FilesSaved counts only files whose no-replace rename succeeded.
// SubfolderExists is true while the session's subfolder is on disk: it becomes
// true at the first file part and false again if the destination removed it
// because nothing was saved. MarkingFailed is true when at least one saved file
// could not receive the OS download marking.
type ReceiveResult struct {
	FilesSaved      int
	SubfolderExists bool
	MarkingFailed   bool
}

// SinkPort validates a receive destination and hands back the one writer a
// receive session uses. The coordinator and server consume it; internal/sink
// implements it. It is consumer-owned, like SourcePort: this package states what
// a receive session needs from the filesystem and nothing else.
type SinkPort interface {
	// OpenDestination checks that absolutePath names an existing directory
	// that is not link-like, and returns a destination bound to it. It creates
	// nothing: the session subfolder is made lazily at the first file part.
	// A missing folder is path_not_found; a non-directory or link-like folder
	// is path_unsupported; a cancelled context is cancelled.
	OpenDestination(ctx context.Context, absolutePath string) (ReceiveDestination, error)
}

// ReceiveDestination is one receive session's write boundary. It owns every
// filesystem guarantee: files and the session subfolder are created
// exclusively and without following links, an existing file is never opened for
// writing, replaced or appended to, and a file counts as saved only after it
// was written to a temporary name, flushed, closed and renamed to its final
// name without replacing anything.
//
// All methods are safe for concurrent use. Snapshot, Folder and Name never
// touch the filesystem, so they cannot block behind a stuck volume; Close,
// CheckSpace and SaveFile can, and the callers that matter bound them.
type ReceiveDestination interface {
	// Name is the destination folder's basename. It is the only part of the
	// path allowed to leave the process.
	Name() string
	// CheckSpace returns ErrInsufficientSpace when declared plus the 3 GiB
	// reserve does not fit in the destination volume's available space, nil
	// when it does, and another error when the space could not be determined
	// (which a caller must treat as a refusal). It writes nothing.
	CheckSpace(declared int64) error
	// SaveFile writes one file part. name is the phone's untrusted file name:
	// the destination sanitizes it and de-duplicates it within the upload. It
	// creates the session subfolder if this is the first file. On any error
	// the partial file is removed, the file is not counted, and every
	// previously saved file is left alone. After Close it saves nothing.
	// It returns the number of content bytes written.
	SaveFile(ctx context.Context, name string, content io.Reader) (int64, error)
	// Snapshot reports what has been saved so far without blocking.
	Snapshot() ReceiveResult
	// Folder returns the absolute path of the session subfolder, and whether it
	// currently exists. The path is for the desktop's Show in Folder only and
	// must never reach HTTP, mDNS, a diagnostic or an event.
	Folder() (string, bool)
	// Close ends the destination. After it returns no further file is renamed
	// into place, so the returned result is final. It removes the subfolder
	// only when nothing was saved, and removes no saved file. It releases the
	// destination handle, is idempotent, and returns the same final result on
	// every call.
	Close() (ReceiveResult, error)
}
