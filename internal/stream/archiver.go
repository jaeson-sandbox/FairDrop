// Package stream writes payloads to an HTTP response without staging them on
// disk: single files are copied in chunks, directories are zipped on the fly
// through an io.Pipe.
package stream

import "net/http"

// Streamer writes a staged path to an HTTP response body.
//
// Implemented in Phase 3.
type Streamer interface {
	// StreamFile writes a single file directly to the HTTP response
	StreamFile(w http.ResponseWriter, filePath string) error
	// StreamZip walks a directory and writes a zip archive to the response via io.Pipe
	StreamZip(w http.ResponseWriter, dirPath string) error
}
