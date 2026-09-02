package network

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"

	"fairdrop/internal/transfer"
	"github.com/hashicorp/mdns"
)

func TestSelectCandidateRejectsIneligibleInterfaces(t *testing.T) {
	t.Parallel()

	validFlags := net.FlagUp | net.FlagBroadcast
	tests := []net.Interface{
		{Name: "down", Flags: net.FlagBroadcast},
		{Name: "no-broadcast", Flags: net.FlagUp},
		{Name: "loopback", Flags: validFlags | net.FlagLoopback},
		{Name: "point-to-point", Flags: validFlags | net.FlagPointToPoint},
		{Name: "DOCKER0", Flags: validFlags},
		{Name: "myVeth12", Flags: validFlags},
		{Name: "WireTUNnel", Flags: validFlags},
	}
	for _, iface := range tests {
		if eligibleInterface(iface) {
			t.Fatalf("eligibleInterface(%q) = true, want false", iface.Name)
		}
	}
	if !eligibleInterface(net.Interface{Name: "Ethernet", Flags: validFlags}) {
		t.Fatal("eligibleInterface(valid) = false, want true")
	}
}

func TestSelectCandidateRejectsNonReachableAddresses(t *testing.T) {
	t.Parallel()

	snapshot := interfaceSnapshot{
		iface: net.Interface{Index: 1, Name: "lan", Flags: net.FlagUp | net.FlagBroadcast},
		addresses: []net.Addr{
			testIPNet(t, "::1/128"),
			testIPNet(t, "2001:db8::1/64"),
			testIPNet(t, "0.0.0.0/8"),
			testIPNet(t, "127.0.0.1/8"),
			testIPNet(t, "224.0.0.1/24"),
			testIPNet(t, "255.255.255.255/32"),
			testIPNet(t, "192.168.4.255/24"),
		},
	}
	if candidate, ok := selectCandidate([]interfaceSnapshot{snapshot}); ok {
		t.Fatalf("selectCandidate() = %v, want no candidate", candidate.addr)
	}
}

func TestSelectCandidateRankingAndTieBreaks(t *testing.T) {
	t.Parallel()

	flags := net.FlagUp | net.FlagBroadcast
	tests := []struct {
		name      string
		snapshots []interfaceSnapshot
		want      string
	}{
		{
			name: "private before global and link-local",
			snapshots: []interfaceSnapshot{
				{iface: net.Interface{Index: 1, Name: "global", Flags: flags}, addresses: []net.Addr{testIPNet(t, "8.8.8.8/24")}},
				{iface: net.Interface{Index: 2, Name: "link", Flags: flags}, addresses: []net.Addr{testIPNet(t, "169.254.2.3/16")}},
				{iface: net.Interface{Index: 99, Name: "private", Flags: flags}, addresses: []net.Addr{testIPNet(t, "10.2.3.4/8")}},
			},
			want: "10.2.3.4",
		},
		{
			name: "global before link-local",
			snapshots: []interfaceSnapshot{
				{iface: net.Interface{Index: 1, Name: "link", Flags: flags}, addresses: []net.Addr{testIPNet(t, "169.254.2.3/16")}},
				{iface: net.Interface{Index: 99, Name: "global", Flags: flags}, addresses: []net.Addr{testIPNet(t, "8.8.8.8/24")}},
			},
			want: "8.8.8.8",
		},
		{
			name: "link-local fallback",
			snapshots: []interfaceSnapshot{
				{iface: net.Interface{Index: 1, Name: "link", Flags: flags}, addresses: []net.Addr{testIPNet(t, "169.254.2.3/16")}},
			},
			want: "169.254.2.3",
		},
		{
			name: "interface index before name and address",
			snapshots: []interfaceSnapshot{
				{iface: net.Interface{Index: 3, Name: "aaa", Flags: flags}, addresses: []net.Addr{testIPNet(t, "10.0.0.1/8")}},
				{iface: net.Interface{Index: 2, Name: "zzz", Flags: flags}, addresses: []net.Addr{testIPNet(t, "10.0.0.9/8")}},
			},
			want: "10.0.0.9",
		},
		{
			name: "folded interface name before numeric address",
			snapshots: []interfaceSnapshot{
				{iface: net.Interface{Index: 1, Name: "Zulu", Flags: flags}, addresses: []net.Addr{testIPNet(t, "10.0.0.1/8")}},
				{iface: net.Interface{Index: 1, Name: "alpha", Flags: flags}, addresses: []net.Addr{testIPNet(t, "10.0.0.9/8")}},
			},
			want: "10.0.0.9",
		},
		{
			name: "original interface name before numeric address",
			snapshots: []interfaceSnapshot{
				{iface: net.Interface{Index: 1, Name: "alpha", Flags: flags}, addresses: []net.Addr{testIPNet(t, "10.0.0.1/8")}},
				{iface: net.Interface{Index: 1, Name: "Alpha", Flags: flags}, addresses: []net.Addr{testIPNet(t, "10.0.0.9/8")}},
			},
			want: "10.0.0.9",
		},
		{
			name: "numeric address after identical interface keys",
			snapshots: []interfaceSnapshot{{
				iface:     net.Interface{Index: 1, Name: "alpha", Flags: flags},
				addresses: []net.Addr{testIPNet(t, "10.0.0.9/8"), testIPNet(t, "10.0.0.3/8")},
			}},
			want: "10.0.0.3",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			candidate, ok := selectCandidate(test.snapshots)
			if !ok {
				t.Fatal("selectCandidate() found no candidate")
			}
			if got := candidate.addr.String(); got != test.want {
				t.Fatalf("selectCandidate() = %s, want %s", got, test.want)
			}
		})
	}
}

func TestGetLocalIPRejectsExcludedCandidatesWithTypedFailure(t *testing.T) {
	t.Parallel()

	manager := addressTestManager(
		[]net.Interface{{Index: 1, Name: "lan", Flags: net.FlagUp | net.FlagBroadcast}},
		func(net.Interface) ([]net.Addr, error) {
			return []net.Addr{
				testIPNet(t, "127.0.0.1/8"),
				testIPNet(t, "224.0.0.1/24"),
				testIPNet(t, "192.168.4.255/24"),
			}, nil
		},
	)
	_, err := manager.GetLocalIP(context.Background())
	if transfer.ErrorCodeOf(err) != transfer.ErrNetworkUnavailable {
		t.Fatalf("GetLocalIP() code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrNetworkUnavailable)
	}
}

func TestSelectCandidateIsIndependentOfEnumerationOrder(t *testing.T) {
	t.Parallel()

	flags := net.FlagUp | net.FlagBroadcast
	original := []interfaceSnapshot{
		{iface: net.Interface{Index: 4, Name: "wifi", Flags: flags}, addresses: []net.Addr{testIPNet(t, "192.168.1.20/24"), testIPNet(t, "192.168.1.10/24")}},
		{iface: net.Interface{Index: 2, Name: "ethernet", Flags: flags}, addresses: []net.Addr{testIPNet(t, "10.0.0.50/8"), testIPNet(t, "10.0.0.2/8")}},
		{iface: net.Interface{Index: 1, Name: "public", Flags: flags}, addresses: []net.Addr{testIPNet(t, "8.8.8.8/24")}},
	}
	reordered := []interfaceSnapshot{original[2], original[1], original[0]}
	reordered[1].addresses = []net.Addr{original[1].addresses[1], original[1].addresses[0]}

	for index, snapshots := range [][]interfaceSnapshot{original, reordered} {
		candidate, ok := selectCandidate(snapshots)
		if !ok || candidate.addr != netip.MustParseAddr("10.0.0.2") {
			t.Fatalf("ordering %d selected %v, want 10.0.0.2", index, candidate.addr)
		}
	}
}

func TestSelectCandidateCanonicalizesMappedIPv4(t *testing.T) {
	t.Parallel()

	mapped := &net.IPNet{
		IP:   net.ParseIP("::ffff:192.168.10.7"),
		Mask: net.CIDRMask(128, 128),
	}
	candidate, ok := selectCandidate([]interfaceSnapshot{{
		iface:     net.Interface{Index: 1, Name: "lan", Flags: net.FlagUp | net.FlagBroadcast},
		addresses: []net.Addr{mapped},
	}})
	if !ok {
		t.Fatal("selectCandidate() found no mapped IPv4 candidate")
	}
	if !candidate.addr.Is4() || candidate.addr.Is4In6() || candidate.addr.String() != "192.168.10.7" {
		t.Fatalf("selectCandidate() = %v, want canonical IPv4", candidate.addr)
	}
}

func TestSelectCandidateRejectsNetworkAddressButPreservesRFC3021Hosts(t *testing.T) {
	t.Parallel()

	iface := net.Interface{Index: 1, Name: "lan", Flags: net.FlagUp | net.FlagBroadcast}
	if candidate, ok := selectCandidate([]interfaceSnapshot{{
		iface:     iface,
		addresses: []net.Addr{testIPNet(t, "192.168.40.0/24")},
	}}); ok {
		t.Fatalf("selectCandidate(network address) = %v, want no candidate", candidate.addr)
	}

	for _, address := range []string{"192.168.40.0/31", "192.168.40.1/31"} {
		candidate, ok := selectCandidate([]interfaceSnapshot{{
			iface:     iface,
			addresses: []net.Addr{testIPNet(t, address)},
		}})
		if !ok || candidate.addr.String() != strings.TrimSuffix(address, "/31") {
			t.Fatalf("selectCandidate(%s) = %v, %v; want valid host", address, candidate.addr, ok)
		}
	}

	candidate, ok := selectCandidate([]interfaceSnapshot{{
		iface:     iface,
		addresses: []net.Addr{testIPNet(t, "192.168.40.0/32")},
	}})
	if !ok || candidate.addr.String() != "192.168.40.0" {
		t.Fatalf("selectCandidate(/32 host) = %v, %v; want 192.168.40.0, true", candidate.addr, ok)
	}
}

func TestGetLocalIPSkipsAddressesForExcludedInterfaces(t *testing.T) {
	t.Parallel()

	interfaces := []net.Interface{
		{Index: 1, Name: "down", Flags: net.FlagBroadcast},
		{Index: 2, Name: "DOCKER0", Flags: net.FlagUp | net.FlagBroadcast},
		{Index: 3, Name: "lan", Flags: net.FlagUp | net.FlagBroadcast},
	}
	var called []string
	manager := addressTestManager(interfaces, func(iface net.Interface) ([]net.Addr, error) {
		called = append(called, iface.Name)
		return []net.Addr{testIPNet(t, "192.168.7.4/24")}, nil
	})
	got, err := manager.GetLocalIP(context.Background())
	if err != nil || got.String() != "192.168.7.4" {
		t.Fatalf("GetLocalIP() = %v, %v; want 192.168.7.4, nil", got, err)
	}
	if len(called) != 1 || called[0] != "lan" {
		t.Fatalf("addresses called for %v, want only lan", called)
	}
}

func TestGetLocalIPContinuesAfterOneInterfaceFails(t *testing.T) {
	t.Parallel()

	failed := errors.New("address enumeration failed")
	interfaces := []net.Interface{
		{Index: 1, Name: "broken", Flags: net.FlagUp | net.FlagBroadcast},
		{Index: 2, Name: "working", Flags: net.FlagUp | net.FlagBroadcast},
	}
	manager := addressTestManager(interfaces, func(iface net.Interface) ([]net.Addr, error) {
		if iface.Name == "broken" {
			return nil, failed
		}
		return []net.Addr{testIPNet(t, "192.168.7.4/24")}, nil
	})
	got, err := manager.GetLocalIP(context.Background())
	if err != nil || got.String() != "192.168.7.4" {
		t.Fatalf("GetLocalIP() = %v, %v; want 192.168.7.4, nil", got, err)
	}
}

func TestGetLocalIPJoinsEnumerationFailuresWhenNoneWork(t *testing.T) {
	t.Parallel()

	first := errors.New("first interface failed")
	second := errors.New("second interface failed")
	interfaces := []net.Interface{
		{Index: 1, Name: "first", Flags: net.FlagUp | net.FlagBroadcast},
		{Index: 2, Name: "second", Flags: net.FlagUp | net.FlagBroadcast},
	}
	manager := addressTestManager(interfaces, func(iface net.Interface) ([]net.Addr, error) {
		if iface.Name == "first" {
			return nil, first
		}
		return nil, second
	})
	_, err := manager.GetLocalIP(context.Background())
	if transfer.ErrorCodeOf(err) != transfer.ErrNetworkUnavailable {
		t.Fatalf("GetLocalIP() code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrNetworkUnavailable)
	}
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Fatalf("GetLocalIP() error does not preserve both causes: %v", err)
	}
}

func TestGetLocalIPCancellationNeverCachesCandidate(t *testing.T) {
	t.Parallel()

	iface := net.Interface{Index: 1, Name: "lan", Flags: net.FlagUp | net.FlagBroadcast}
	tests := []struct {
		name string
		run  func(*Manager) error
	}{
		{
			name: "nil context",
			run: func(manager *Manager) error {
				_, err := manager.GetLocalIP(nil)
				if transfer.ErrorCodeOf(err) != transfer.ErrTransferFailed {
					t.Fatalf("nil context code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrTransferFailed)
				}
				return err
			},
		},
		{
			name: "already cancelled",
			run: func(manager *Manager) error {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_, err := manager.GetLocalIP(ctx)
				if transfer.ErrorCodeOf(err) != transfer.ErrCancelled {
					t.Fatalf("already-cancelled code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrCancelled)
				}
				return err
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager := addressTestManager([]net.Interface{iface}, func(net.Interface) ([]net.Addr, error) {
				return []net.Addr{testIPNet(t, "192.168.1.4/24")}, nil
			})
			manager.selected = &selection{iface: iface, addr: netip.MustParseAddr("10.0.0.1")}
			_ = test.run(manager)
			if manager.selected != nil {
				t.Fatal("cancelled GetLocalIP cached a candidate")
			}
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	manager := addressTestManager([]net.Interface{iface}, func(net.Interface) ([]net.Addr, error) {
		cancel()
		return []net.Addr{testIPNet(t, "192.168.1.4/24")}, nil
	})
	manager.selected = &selection{iface: iface, addr: netip.MustParseAddr("10.0.0.1")}
	_, err := manager.GetLocalIP(ctx)
	if transfer.ErrorCodeOf(err) != transfer.ErrCancelled {
		t.Fatalf("during-enumeration code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrCancelled)
	}
	if manager.selected != nil {
		t.Fatal("during-enumeration cancellation cached a candidate")
	}
}

func addressTestManager(interfaces []net.Interface, addresses func(net.Interface) ([]net.Addr, error)) *Manager {
	return newManager(managerDependencies{
		interfaces: func() ([]net.Interface, error) { return interfaces, nil },
		addresses:  addresses,
		hostname:   func() (string, error) { return "test-host", nil },
		entropy:    bytes.NewReader(make([]byte, processSuffixBytes)),
		start: func(*mdns.Config) (beaconHandle, error) {
			panic("unexpected beacon start")
		},
	})
}

func testIPNet(t *testing.T, value string) net.Addr {
	t.Helper()
	ip, network, err := net.ParseCIDR(value)
	if err != nil {
		t.Fatalf("net.ParseCIDR(%q): %v", value, err)
	}
	network.IP = ip
	return network
}
