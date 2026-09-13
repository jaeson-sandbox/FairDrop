// Package transfer owns FairDrop's shared domain vocabulary and the ports
// consumed by the transfer coordinator.
package transfer

import "time"

// SessionID correlates one transfer session internally and with the UI. It
// carries at least 128 random bits from a CSPRNG, is independent of the
// session's CapabilityToken, and is never persisted.
type SessionID string

// CapabilityToken authorizes access to one transfer over HTTP. It carries at
// least 128 random bits from a CSPRNG, is independent of the session's
// SessionID -- neither is derived from the other -- and is never persisted.
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

	// UnportableNames counts entries whose names a Windows receiver cannot
	// save. It is a count, never the names themselves: AD-9 forbids a path,
	// and one segment of a user's filesystem is close enough to one that
	// echoing it is a decision nobody has taken.
	//
	// Zero for a file, and for a directory whose every entry travels. The
	// coordinator turns a non-zero count into the Staged warning; nothing
	// here refuses, because a name Windows dislikes is still an ordinary name
	// on the sender and on the phone that is this product's usual receiver.
	UnportableNames int
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

// WarningCode is a stable code for a non-fatal condition. It is a narrower
// type than ErrorCode on purpose: a Warning may carry only a code this
// package has deliberately decided describes a warning, not any of the
// thirteen recognized at a failure boundary. Before this type existed,
// Warning.Code was plain ErrorCode, so nothing stopped a future warning from
// compiling with a code frontend/src/transfer/validation.ts's parseWarning
// does not recognize -- and that parser rejects the whole Stage
// acknowledgement, not just the one warning, when it sees a code it does not
// know (Epic 1 retrospective item 3).
type WarningCode string

// WarnBeaconUnavailable is the one WarningCode this build produces. It
// shares ErrBeaconWarning's wire value deliberately -- the two describe the
// same condition from two different boundaries -- without sharing its type,
// which is the whole point: assigning any other ErrorCode to a Warning is a
// compile error.
const WarnBeaconUnavailable WarningCode = WarningCode(ErrBeaconWarning)

// WarnUnportableNames is raised at Stage when a selected folder holds entries
// a Windows receiver cannot save. Non-terminal by construction: the transfer
// proceeds and the names are sent unchanged, because the sender may well be
// sending to a phone, where they are perfectly ordinary.
const WarnUnportableNames WarningCode = WarningCode(ErrNameWarning)

// Warning is one non-fatal condition attached to an otherwise successful
// command result. A warning never carries adapter text: its code selects fixed
// public copy, so nothing a warning can say could contain a path or a token.
type Warning struct {
	Code    WarningCode `json:"code"`
	Message string      `json:"message"`
}

// FileMetadata is the acknowledgement of a staged transfer: everything the
// sender's UI needs to show a session, and nothing more.
//
// URL and QR are the only two places the capability token is allowed to
// appear. Path is deliberately absent -- Name is the basename the receiver is
// offered -- and Warnings is always a non-nil slice so it serializes as an
// empty JSON array rather than null.
type FileMetadata struct {
	SessionID SessionID `json:"sessionId"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	IsDir     bool      `json:"isDir"`
	URL       string    `json:"url"`
	QR        string    `json:"qrBase64"`
	Warnings  []Warning `json:"warnings"`
}
