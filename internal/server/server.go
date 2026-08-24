// Package server owns FairDrop's ephemeral HTTP server: it exists only while a
// file is staged or transferring, binds to an OS-assigned port, and reports
// transfer progress back to the caller.
package server

import (
	"context"
	"io"

	"fairdrop/internal/transfer"
)

// PreparedPayload is one source item opened for exactly one authorized
// response. It is produced by PayloadPort.Prepare before any response header is
// written, so a failure to produce it can still change the HTTP status.
//
// Ownership: after a successful Prepare the server owns exactly one Close, and
// it never calls Close concurrently with WriteTo. The cancellation order is
// cancel the data-plane context, force-close the destination so writes unblock,
// wait for WriteTo and any workers to return, then Close. That single ownership
// covers normal completion, receiver disconnect, header failure, Cancel, and
// Stop-before-Write. Close is idempotent: it releases the descriptor exactly
// once and reports a cleanup cause only on its first call.
type PreparedPayload interface {
	// DownloadName is the sanitizable basename offered to the receiver. It is
	// never an absolute or relative source path.
	DownloadName() string
	// Size reports the wire length. known is false only when the payload's
	// length cannot be established before streaming, as for a directory
	// archive; a known empty payload reports (0, true).
	Size() (bytes int64, known bool)
	// WriteTo copies the payload to dst exactly once. It returns promptly when
	// ctx is cancelled or dst fails, appends nothing after a failure, and
	// leaves no goroutine of its own running when it returns.
	WriteTo(ctx context.Context, dst io.Writer) error
	// Close releases the payload's descriptor. It is safe to call repeatedly
	// and safe to call without a preceding WriteTo.
	Close() error
}

// PayloadPort turns a staged item into bytes for one authorized response.
//
// Prepare runs after claim authorization and before response headers. For a
// file it re-validates the staged root, opens it, stats that same descriptor,
// and returns a known length derived from the descriptor rather than from
// staging metadata. Every Prepare failure returns a coded transfer error and
// retains no descriptor.
type PayloadPort interface {
	Prepare(ctx context.Context, item transfer.StagedItem) (PreparedPayload, error)
}

// TransferStats is the progress snapshot reported during an active transfer.
// It is the payload behind the "transfer-progress" frontend event.
type TransferStats struct {
	BytesSent        int64   `json:"bytesSent"`
	TotalBytes       int64   `json:"totalBytes"`
	Percent          float64 `json:"percent"`
	SpeedBytesPerSec float64 `json:"speedBytesPerSec"`
}

// TransferServer is the lifecycle of the ephemeral HTTP server.
//
// Implemented in Phase 4.
type TransferServer interface {
	// Start boots the HTTP server on port 0 and returns the assigned port.
	//
	// ctx carries cancellation for the in-flight transfer. Implementations must
	// abort promptly on ctx.Done(), force-dropping active connections and
	// closing the listener, so a user-initiated cancel takes effect immediately.
	Start(ctx context.Context, filePath string, onProgress func(stats TransferStats)) (int, error)
	// Stop force-closes active connections and stops listening
	Stop() error
}
