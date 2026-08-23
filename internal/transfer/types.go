// Package transfer owns FairDrop's shared domain vocabulary: the values that
// describe a staged transfer, the stable error contract every adapter reports
// through, and the coordinator-facing ports adapters implement.
//
// The package is consumer-owned on purpose. The coordinator declares what it
// needs; the filesystem, network, HTTP, QR, and Wails adapters depend on this
// package and never the other way around, so no adapter can invent a parallel
// vocabulary or a shadow interface. Exported names, shapes, and meanings here
// are fixed by docs/fairdrop-contracts.md and may not drift without a contract
// update.
package transfer

import "time"

// SessionID correlates one staging session between the coordinator and the UI.
// It carries at least 128 random bits, is never persisted, and is distinct from
// the capability token so that leaking one cannot authorize the other.
type SessionID string

// CapabilityToken authorizes exactly one HTTP download. It carries at least 128
// random bits drawn independently of the SessionID, is never persisted, and
// appears only in the local stage URL, the QR code, and the receiver's
// authorized request path.
type CapabilityToken string

// ItemKind distinguishes the two things FairDrop can send. Epic 1 stages only
// ItemFile; ItemDirectory exists here because the value is part of the binding
// contract and Epic 2 extends the same adapters rather than replacing them.
type ItemKind string

const (
	// ItemFile is a single regular file, sent as-is.
	ItemFile ItemKind = "file"
	// ItemDirectory is a directory, sent as an unsnapshotted ZIP stream.
	ItemDirectory ItemKind = "directory"
)

// StagedItem is the immutable description of one validated selection.
//
// It is produced by SourcePort.Inspect and treated as a value from then on:
// nothing mutates a StagedItem after inspection, and the claim-time
// revalidation compares a fresh inspection against this snapshot rather than
// updating it in place.
type StagedItem struct {
	// Path is the selected absolute path exactly as the caller supplied it,
	// byte-for-byte, with no cleaning, case-folding, or rewriting. It is
	// sender-private: it must never reach the wire, the UI, mDNS records,
	// HTTP responses, or a public error message.
	Path string
	// Name is the basename derived from Path. It may be disclosed in local
	// stage metadata and in a sanitized Content-Disposition header.
	Name string
	// Kind is ItemFile or ItemDirectory.
	Kind ItemKind
	// LogicalSize is the on-disk size in bytes for a file. For a directory it
	// is the logical total, which is not the ZIP wire length.
	LogicalSize int64
	// ModTime is the modification time observed at inspection. Claim-time
	// revalidation compares against it to detect a changed source.
	ModTime time.Time
}
