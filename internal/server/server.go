// Package server owns FairDrop's ephemeral HTTP server: it exists only while a
// file is staged or transferring, binds to an OS-assigned port, and reports
// transfer progress back to the caller.
package server

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
	// Start boots the HTTP server on port 0 and returns the assigned port
	Start(filePath string, onProgress func(stats TransferStats)) (int, error)
	// Stop force-closes active connections and stops listening
	Stop() error
}
