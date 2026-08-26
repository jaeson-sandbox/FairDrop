package transfer

import (
	"context"
	"net/netip"
)

const (
	// BeaconService is FairDrop's fixed DNS-SD service type.
	BeaconService = "_fairdrop._tcp"
	// BeaconVersionTXT is the complete v1 TXT payload advertised by FairDrop.
	BeaconVersionTXT = "version=1"
)

// SourcePort validates and describes a selected source path without opening
// its contents.
type SourcePort interface {
	Inspect(ctx context.Context, absolutePath string) (StagedItem, error)
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
