package transfer

import (
	"context"
	"io"
	"net/netip"
	"time"
)

const (
	// BeaconService is FairDrop's fixed DNS-SD service type.
	BeaconService = "_fairdrop._tcp"
	// BeaconVersionTXT is the complete v1 TXT payload advertised by FairDrop.
	BeaconVersionTXT = "version=1"
)

// SourceEntry is one entry reached by a SourcePort walk.
//
// RelativePath locates the entry beneath the walked root with forward slashes
// and nothing else: it is never empty, absolute, volume-qualified, dot-dot
// bearing, nor does it name the root itself. A consumer is therefore free to
// place every entry under whatever single top-level name it chooses without
// re-deriving a path. Size is meaningful only for ItemFile.
type SourceEntry struct {
	RelativePath string
	Kind         ItemKind
	Size         int64
	ModTime      time.Time
}

// SourceVisitor receives one entry per call, a parent directory before any of
// its children.
//
// content is nil for a directory. For a regular file it is a reader borrowed
// for exactly the duration of the call: the source owns the descriptor behind
// it and releases it as soon as the visitor returns, so a retained reader is a
// use-after-close and reads nothing. A non-nil return stops the walk; the
// source unwinds every handle it owns and reports that error unchanged unless
// cancellation or a close failure takes precedence.
type SourceVisitor func(entry SourceEntry, content io.Reader) error

// SourcePort validates and describes a selected source path, and walks a
// selected directory without ever handing out a descriptor it does not close.
type SourcePort interface {
	// Inspect describes the selection without opening its contents.
	Inspect(ctx context.Context, absolutePath string) (StagedItem, error)
	// Walk re-validates absolutePath under the same link, reparse, special-file
	// and identity rules as Inspect, then calls visit once for every entry
	// beneath it. Preflight is not a snapshot, so every entry is re-checked
	// here rather than trusted from Inspect. Walk retains no per-entry index:
	// it holds one enumeration handle per active depth plus the entry it is
	// visiting, and it closes every handle it opened, in reverse order, on
	// every exit. A selection that is not a directory is path_unsupported.
	Walk(ctx context.Context, absolutePath string, visit SourceVisitor) error
}

// BeaconRequest contains the non-sensitive values needed to publish a staged
// transfer. SessionID is correlation-only and is never advertised.
type BeaconRequest struct {
	SessionID SessionID
	Service   string
	Instance  string
	Port      int
	TXT       []string
}

// NetworkPort selects the address used by the direct URL and owns the matching
// discovery beacon.
type NetworkPort interface {
	GetLocalIP(ctx context.Context) (netip.Addr, error)
	StartBeacon(ctx context.Context, request BeaconRequest) error
	StopBeacon() error
}

// ServerStartRequest is the immutable description of the one transfer an
// ephemeral server exists to serve. Token authorizes exactly one download;
// SessionID only correlates events and is never put on the wire.
type ServerStartRequest struct {
	SessionID SessionID
	Token     CapabilityToken
	Item      StagedItem
}

// ClaimAuthorizer is the coordinator handshake a reserved claim must clear
// before the server opens a payload or writes a response header.
type ClaimAuthorizer interface {
	// AuthorizeClaim runs synchronously on the serving goroutine and returns
	// only after the coordinator has committed the transfer or refused it.
	// A refusal is authoritative and final: the coordinator already owns that
	// outcome, so the server reports no event for it.
	AuthorizeClaim(ctx context.Context, sessionID SessionID) error
}

// ServerEventKind names the three things an ephemeral server reports.
type ServerEventKind string

const (
	ServerProgress ServerEventKind = "progress"
	ServerComplete ServerEventKind = "complete"
	ServerFailed   ServerEventKind = "failed"
)

// ServerEvent is one observation from the serving lane.
//
// Progress carries the authoritative terminal snapshot on ServerComplete. On
// ServerFailed it is present only when bytes reached the receiver, and nil
// otherwise, so a failure before any byte cannot be mistaken for one that
// stalled at zero. Err is set only on ServerFailed and preserves the coded
// cause unchanged for the coordinator to classify.
type ServerEvent struct {
	SessionID SessionID
	Kind      ServerEventKind
	Progress  *ProgressSnapshot
	Err       error
}

// ServerHandle is what a started server hands back: the port the receiver must
// reach and the lane its events arrive on. Events is closed exactly once, when
// the server is torn down, and never produces another event afterwards.
//
// Publication onto Events must never block. The coordinator calls Stop from
// inside the goroutine that drains this lane, so nothing is reading it while
// Stop runs; a producer that blocked on a full lane would make Stop wait for a
// consumer that is itself inside Stop.
type ServerHandle struct {
	Port   int
	Events <-chan ServerEvent
}

// QRPort renders the capability URL as a scannable image. It returns PNG bytes;
// the base64 the frontend consumes is produced at the coordinator boundary, not
// here.
type QRPort interface {
	EncodePNG(ctx context.Context, content string) ([]byte, error)
}

// ServerPort is the ephemeral one-shot HTTP server the coordinator owns.
//
// A successful Start means the listener is bound and its accept loop is ready
// before return; a failed Start leaves no listener, goroutine, or channel
// behind. Stop is idempotent and force-closing: on every return, including one
// that reports a cleanup diagnostic, the listener, connections, handlers,
// payload workers, and event producers have ended and the event channel is
// closed for good. A cleanup diagnostic is a report, never a transfer of
// ownership back to the caller.
type ServerPort interface {
	Start(ctx context.Context, request ServerStartRequest, authorizer ClaimAuthorizer) (ServerHandle, error)
	Stop() error
}

// EventKind names the five lifecycle events the coordinator publishes.
type EventKind string

const (
	TransferStarted  EventKind = "transfer-started"
	TransferProgress EventKind = "transfer-progress"
	TransferComplete EventKind = "transfer-complete"
	TransferError    EventKind = "transfer-error"
	TransferReset    EventKind = "transfer-reset"
)

// Event is one lifecycle observation for one session.
//
// Seq starts at 1 for the first published event of a session and increases by
// exactly one per published event; coalesced or dropped progress snapshots are
// never assigned a sequence number, so a gap cannot occur. Kind is excluded
// from JSON because the Wails adapter carries it as the event name rather than
// as part of the payload.
type Event struct {
	SessionID SessionID         `json:"sessionId"`
	Seq       uint64            `json:"seq"`
	Kind      EventKind         `json:"-"`
	Progress  *ProgressSnapshot `json:"progress,omitempty"`
	Error     *PublicError      `json:"error,omitempty"`
}

// Observer receives lifecycle events on the coordinator's single emission
// lane. Publish is a synchronous FIFO handoff: it is called from whichever
// operation owns the sequence it was just assigned, and an implementation must
// deliver in call order rather than reordering or buffering out of band.
type Observer interface {
	Publish(event Event)
}
