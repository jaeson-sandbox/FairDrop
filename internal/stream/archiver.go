// Package stream writes payloads to an HTTP response without staging them on
// disk: single files are copied in chunks, directories are zipped on the fly
// through an io.Pipe.
package stream

import (
	"context"
	"net/http"
)

// Streamer writes a staged path to an HTTP response body.
//
// Implemented in Phase 3.
type Streamer interface {
	// StreamFile writes a single file directly to the HTTP response.
	//
	// ctx carries cancellation for the in-flight transfer. Implementations must
	// abort promptly on ctx.Done() rather than copying the remaining bytes.
	StreamFile(ctx context.Context, w http.ResponseWriter, filePath string) error
	// StreamZip walks a directory and writes a zip archive to the response via io.Pipe.
	//
	// ctx carries cancellation for the in-flight transfer. Implementations must
	// abort promptly on ctx.Done() and close both ends of the pipe, or the
	// zipping goroutine will block forever waiting for a read.
	StreamZip(ctx context.Context, w http.ResponseWriter, dirPath string) error
}
