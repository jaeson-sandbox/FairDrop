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
