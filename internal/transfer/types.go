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
