// Package network owns FairDrop's local network identity: resolving the best
// LAN-routable IPv4 address and running the mDNS beacon that lets receivers
// discover a staged transfer.
package network

import (
	"context"
	"crypto/rand"
	"io"
	"net"
	"net/netip"
	"os"
	"sync"

	"fairdrop/internal/transfer"
	"github.com/hashicorp/mdns"
)

type beaconHandle interface {
	Shutdown() error
}

type managerDependencies struct {
	interfaces func() ([]net.Interface, error)
	addresses  func(net.Interface) ([]net.Addr, error)
	hostname   func() (string, error)
	entropy    io.Reader
	start      func(*mdns.Config) (beaconHandle, error)
}

type selection struct {
	iface net.Interface
	addr  netip.Addr
}

type processIdentity struct {
	host   string
	suffix string
}

// Manager deterministically selects one LAN endpoint and owns at most one
// matching mDNS responder.
type Manager struct {
	deps managerDependencies

	selectionGate chan struct{}
	mu            sync.Mutex
	selected      *selection
	identity      *processIdentity
	beacon        beaconHandle
}

var _ transfer.NetworkPort = (*Manager)(nil)

// NewManager constructs a network adapter with process-local, immutable OS and
// mDNS dependencies.
func NewManager() *Manager {
	return newManager(managerDependencies{
		interfaces: net.Interfaces,
		addresses: func(iface net.Interface) ([]net.Addr, error) {
			return iface.Addrs()
		},
		hostname: os.Hostname,
		entropy:  rand.Reader,
		start: func(config *mdns.Config) (beaconHandle, error) {
			server, err := mdns.NewServer(config)
			if server == nil {
				return nil, err
			}
			return server, err
		},
	})
}

func newManager(deps managerDependencies) *Manager {
	selectionGate := make(chan struct{}, 1)
	selectionGate <- struct{}{}
	return &Manager{deps: deps, selectionGate: selectionGate}
}

// GetLocalIP returns and caches the deterministic best LAN IPv4 candidate.
func (m *Manager) GetLocalIP(ctx context.Context) (netip.Addr, error) {
	if ctx == nil {
		m.clearInactiveSelectionIfGateAvailable()
		return netip.Addr{}, transfer.NewError(transfer.ErrTransferFailed, "network selection requires a context")
	}
	if err := contextError(ctx); err != nil {
		m.clearInactiveSelectionIfGateAvailable()
		return netip.Addr{}, err
	}
	if err := m.acquireSelectionGate(ctx, contextError); err != nil {
		return netip.Addr{}, err
	}
	defer m.releaseSelectionGate()
	if err := contextError(ctx); err != nil {
		m.mu.Lock()
		if !beaconHandlePresent(m.beacon) {
			m.selected = nil
		}
		m.mu.Unlock()
		return netip.Addr{}, err
	}

	m.mu.Lock()
	if beaconHandlePresent(m.beacon) {
		selected := m.selected.addr
		m.mu.Unlock()
		return selected, nil
	}
	m.selected = nil
	m.mu.Unlock()

	snapshots, causes, err := m.enumerate(ctx)
	if err != nil {
		return netip.Addr{}, err
	}
	candidate, ok := selectCandidate(snapshots)
	if err := contextError(ctx); err != nil {
		return netip.Addr{}, err
	}
	if !ok {
		return netip.Addr{}, unavailableError(causes)
	}

	m.mu.Lock()
	m.selected = &selection{iface: candidate.iface, addr: candidate.addr}
	m.mu.Unlock()
	return candidate.addr, nil
}

func (m *Manager) acquireSelectionGate(ctx context.Context, mapContextError func(context.Context) error) error {
	select {
	case <-ctx.Done():
		return mapContextError(ctx)
	case <-m.selectionGate:
		return nil
	}
}

func (m *Manager) releaseSelectionGate() {
	m.selectionGate <- struct{}{}
}

func (m *Manager) clearInactiveSelectionIfGateAvailable() {
	select {
	case <-m.selectionGate:
		m.mu.Lock()
		if !beaconHandlePresent(m.beacon) {
			m.selected = nil
		}
		m.mu.Unlock()
		m.releaseSelectionGate()
	default:
	}
}

func (m *Manager) enumerate(ctx context.Context) ([]interfaceSnapshot, []error, error) {
	if err := contextError(ctx); err != nil {
		return nil, nil, err
	}
	interfaces, interfacesErr := m.deps.interfaces()
	if err := contextError(ctx); err != nil {
		return nil, nil, err
	}

	var causes []error
	if interfacesErr != nil {
		causes = append(causes, interfacesErr)
	}
	snapshots := make([]interfaceSnapshot, 0, len(interfaces))
	for _, iface := range interfaces {
		if err := contextError(ctx); err != nil {
			return nil, nil, err
		}
		if !eligibleInterface(iface) {
			continue
		}
		addresses, addressErr := m.deps.addresses(iface)
		if err := contextError(ctx); err != nil {
			return nil, nil, err
		}
		if addressErr != nil {
			causes = append(causes, addressErr)
			continue
		}
		snapshots = append(snapshots, interfaceSnapshot{iface: iface, addresses: addresses})
	}
	return snapshots, causes, nil
}

func contextError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return transfer.WrapError(transfer.ErrCancelled, "network operation was cancelled", err)
	}
	return nil
}
