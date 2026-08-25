// Package transfer owns FairDrop's shared domain vocabulary and the ports
// consumed by the transfer coordinator.
package transfer

import "time"

// SessionID correlates one transfer session internally and with the UI.
type SessionID string

// CapabilityToken authorizes access to one transfer over HTTP.
type CapabilityToken string

// ItemKind identifies the kind of staged source item.
type ItemKind string

const (
	ItemFile      ItemKind = "file"
	ItemDirectory ItemKind = "directory"
)

// StagedItem is an immutable metadata snapshot of a validated source item.
type StagedItem struct {
	Path        string
	Name        string
	Kind        ItemKind
	LogicalSize int64
	ModTime     time.Time
}

// ProgressSnapshot is one wire-accurate view of a transfer in flight.
//
// BytesSent counts only bytes the receiver's connection accepted, never bytes
// merely read from the source. TotalKnown is the explicit discriminator for a
// payload whose length cannot be established before streaming: an unknown
// total reports TotalBytes 0 and Percent 0 rather than guessing. Percent is
// always finite and clamped to [0,100]; NaN and infinity are forbidden because
// they would fail JSON marshalling at the UI boundary.
type ProgressSnapshot struct {
	BytesSent        int64   `json:"bytesSent"`
	TotalBytes       int64   `json:"totalBytes"`
	TotalKnown       bool    `json:"totalKnown"`
	Percent          float64 `json:"percent"`
	SpeedBytesPerSec float64 `json:"speedBytesPerSec"`
}
