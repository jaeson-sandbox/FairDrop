// Package network owns FairDrop's local network identity: resolving the best
// LAN-routable IPv4 address and running the mDNS beacon that lets receivers
// discover a staged transfer.
package network

// NetworkManager resolves the host's LAN address and controls the mDNS beacon.
//
// Implemented in Phase 2.
type NetworkManager interface {
	// GetLocalIP returns the best IPv4 address for local P2P routing
	GetLocalIP() (string, error)
	// StartBeacon begins the mDNS broadcast for "_fairdrop._tcp"
	StartBeacon(port int, metadata map[string]string) error
	// StopBeacon gracefully shuts down mDNS
	StopBeacon()
}
