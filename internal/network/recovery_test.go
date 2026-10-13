package network

import (
	"context"
	"net"
	"testing"

	"fairdrop/internal/transfer"
)

func TestConnectionOptionsAreSortedDeduplicatedAndReadOnly(t *testing.T) {
	first := net.Interface{Index: 3, Name: "Wi-Fi", Flags: net.FlagUp | net.FlagBroadcast}
	second := net.Interface{Index: 9, Name: "docker0", Flags: net.FlagUp | net.FlagBroadcast}
	m := addressTestManager([]net.Interface{second, first}, func(iface net.Interface) ([]net.Addr, error) {
		if iface.Index == 3 {
			return []net.Addr{testIPNet(t, "192.168.4.12/24"), testIPNet(t, "192.168.4.12/24"), testIPNet(t, "10.0.0.8/24")}, nil
		}
		return []net.Addr{testIPNet(t, "172.17.0.1/16")}, nil
	})
	options, err := m.Options(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 2 || options[0].Address != "10.0.0.8" || options[1].Address != "192.168.4.12" {
		t.Fatalf("choices = %+v", options)
	}
	if options[0].ID == "" || options[0].ID == options[1].ID {
		t.Fatalf("IDs are not distinct: %+v", options)
	}
	m.mu.Lock()
	selected := m.selected
	m.mu.Unlock()
	if selected != nil {
		t.Fatal("listing changed selected endpoint")
	}
	if _, err := m.SelectEndpoint(context.Background(), options[1].ID); err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	selected = m.selected
	m.mu.Unlock()
	if selected == nil || selected.addr.String() != "192.168.4.12" {
		t.Fatalf("selected = %+v", selected)
	}
	if _, err := m.SelectEndpoint(context.Background(), "vanished"); transfer.ErrorCodeOf(err) != transfer.ErrNetworkUnavailable {
		t.Fatalf("stale choice = %v", err)
	}
	m.mu.Lock()
	still := m.selected
	m.mu.Unlock()
	if still != selected {
		t.Fatal("stale choice changed selected endpoint")
	}
}
