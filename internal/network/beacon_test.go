package network

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
	"github.com/hashicorp/mdns"
)

func TestStartBeaconBuildsExplicitNonSensitiveConfiguration(t *testing.T) {
	t.Parallel()

	handle := &fakeBeacon{}
	var captured *mdns.Config
	manager := beaconTestManager(t, bytes.NewReader(make([]byte, processSuffixBytes)), func(config *mdns.Config) (beaconHandle, error) {
		captured = config
		return handle, nil
	})
	request := validBeaconRequest()
	request.SessionID = transfer.SessionID("session-must-not-be-advertised")
	if err := manager.StartBeacon(context.Background(), request); err != nil {
		t.Fatalf("StartBeacon() error = %v", err)
	}
	if captured == nil {
		t.Fatal("StartBeacon() did not call registrar factory")
	}
	if captured.Iface == nil || captured.Iface.Index != 7 || captured.Iface.Name != "Ethernet" {
		t.Fatalf("configured interface = %#v, want selected Ethernet interface", captured.Iface)
	}
	if captured.Logger == nil || captured.Logger.Writer() != io.Discard {
		t.Fatal("configured logger does not discard mDNS diagnostics")
	}
	service, ok := captured.Zone.(*mdns.MDNSService)
	if !ok {
		t.Fatalf("configured zone type = %T, want *mdns.MDNSService", captured.Zone)
	}
	suffix := strings.Repeat("00", processSuffixBytes)
	if service.Instance != "FairDrop-work-station-local-"+suffix {
		t.Fatalf("service instance = %q", service.Instance)
	}
	if service.Service != transfer.BeaconService || service.Domain != "local." {
		t.Fatalf("service identity = %q in %q", service.Service, service.Domain)
	}
	if service.HostName != "work-station-local-"+suffix+".local." {
		t.Fatalf("service hostname = %q", service.HostName)
	}
	if service.Port != request.Port {
		t.Fatalf("service port = %d, want %d", service.Port, request.Port)
	}
	if len(service.IPs) != 1 || !service.IPs[0].Equal(net.ParseIP("192.168.50.9")) {
		t.Fatalf("service IPs = %v, want selected IPv4 only", service.IPs)
	}
	if len(service.TXT) != 1 || service.TXT[0] != transfer.BeaconVersionTXT {
		t.Fatalf("service TXT = %v, want only %q", service.TXT, transfer.BeaconVersionTXT)
	}
	disclosed := strings.Join([]string{service.Instance, service.Service, service.Domain, service.HostName, strings.Join(service.TXT, ",")}, " ")
	if strings.Contains(disclosed, string(request.SessionID)) {
		t.Fatalf("mDNS configuration disclosed SessionID: %q", disclosed)
	}
	manager.mu.Lock()
	active := manager.beacon
	manager.mu.Unlock()
	if active != handle {
		t.Fatal("StartBeacon() returned before retaining the active handle")
	}
	if err := manager.StopBeacon(); err != nil {
		t.Fatalf("StopBeacon() error = %v", err)
	}
}

func TestGetLocalIPReturnsCachedEndpointWhileBeaconIsActive(t *testing.T) {
	t.Parallel()

	interfaceCalls := 0
	addressCalls := 0
	currentAddress := "192.168.50.9/24"
	handle := &fakeBeacon{}
	iface := net.Interface{Index: 7, Name: "Ethernet", Flags: net.FlagUp | net.FlagBroadcast}
	manager := newManager(managerDependencies{
		interfaces: func() ([]net.Interface, error) {
			interfaceCalls++
			return []net.Interface{iface}, nil
		},
		addresses: func(net.Interface) ([]net.Addr, error) {
			addressCalls++
			return []net.Addr{mustIPNet(currentAddress)}, nil
		},
		hostname: func() (string, error) { return "test-host", nil },
		entropy:  bytes.NewReader(make([]byte, processSuffixBytes)),
		start:    func(*mdns.Config) (beaconHandle, error) { return handle, nil },
	})
	selected, err := manager.GetLocalIP(context.Background())
	if err != nil || selected.String() != "192.168.50.9" {
		t.Fatalf("initial GetLocalIP() = %v, %v", selected, err)
	}
	if err := manager.StartBeacon(context.Background(), validBeaconRequest()); err != nil {
		t.Fatalf("StartBeacon() error = %v", err)
	}

	currentAddress = "10.20.30.40/8"
	cached, err := manager.GetLocalIP(context.Background())
	if err != nil || cached != selected {
		t.Fatalf("active GetLocalIP() = %v, %v; want cached %v", cached, err, selected)
	}
	if interfaceCalls != 1 || addressCalls != 1 {
		t.Fatalf("active reselection enumerated interfaces=%d addresses=%d; want 1, 1", interfaceCalls, addressCalls)
	}
	if err := manager.StopBeacon(); err != nil {
		t.Fatalf("StopBeacon() error = %v", err)
	}
}

func TestStartBeaconWaitsForSelectionAndUsesCommittedEndpoint(t *testing.T) {
	t.Parallel()

	selectionEntered := make(chan struct{})
	releaseSelection := make(chan struct{})
	factoryCalled := make(chan struct{})
	var captured *mdns.Config
	iface := net.Interface{Index: 7, Name: "Ethernet", Flags: net.FlagUp | net.FlagBroadcast}
	manager := newManager(managerDependencies{
		interfaces: func() ([]net.Interface, error) { return []net.Interface{iface}, nil },
		addresses: func(net.Interface) ([]net.Addr, error) {
			close(selectionEntered)
			<-releaseSelection
			return []net.Addr{mustIPNet("192.168.60.8/24")}, nil
		},
		hostname: func() (string, error) { return "test-host", nil },
		entropy:  bytes.NewReader(make([]byte, processSuffixBytes)),
		start: func(config *mdns.Config) (beaconHandle, error) {
			captured = config
			close(factoryCalled)
			return &fakeBeacon{}, nil
		},
	})

	selectionResult := make(chan addressResult, 1)
	go func() {
		address, err := manager.GetLocalIP(context.Background())
		selectionResult <- addressResult{address: address.String(), err: err}
	}()
	<-selectionEntered

	startContext := newObservedContext(context.Background())
	startResult := make(chan error, 1)
	go func() { startResult <- manager.StartBeacon(startContext, validBeaconRequest()) }()
	<-startContext.observed
	select {
	case <-factoryCalled:
		t.Fatal("StartBeacon called factory before in-flight selection committed")
	default:
	}

	close(releaseSelection)
	selected := receiveAddressResult(t, selectionResult)
	if selected.err != nil || selected.address != "192.168.60.8" {
		t.Fatalf("GetLocalIP() = %q, %v", selected.address, selected.err)
	}
	if err := receiveError(t, startResult); err != nil {
		t.Fatalf("StartBeacon() error = %v", err)
	}
	service := captured.Zone.(*mdns.MDNSService)
	if len(service.IPs) != 1 || !service.IPs[0].Equal(net.ParseIP("192.168.60.8")) {
		t.Fatalf("StartBeacon configured IPs %v, want committed endpoint", service.IPs)
	}
	if err := manager.StopBeacon(); err != nil {
		t.Fatalf("StopBeacon() error = %v", err)
	}
}

func TestStartBeaconWaitCancellationMapsToBeaconWarning(t *testing.T) {
	t.Parallel()

	selectionEntered := make(chan struct{})
	releaseSelection := make(chan struct{})
	factoryCalls := 0
	iface := net.Interface{Index: 7, Name: "Ethernet", Flags: net.FlagUp | net.FlagBroadcast}
	manager := newManager(managerDependencies{
		interfaces: func() ([]net.Interface, error) { return []net.Interface{iface}, nil },
		addresses: func(net.Interface) ([]net.Addr, error) {
			close(selectionEntered)
			<-releaseSelection
			return []net.Addr{mustIPNet("192.168.65.8/24")}, nil
		},
		hostname: func() (string, error) { return "test-host", nil },
		entropy:  bytes.NewReader(make([]byte, processSuffixBytes)),
		start: func(*mdns.Config) (beaconHandle, error) {
			factoryCalls++
			return &fakeBeacon{}, nil
		},
	})

	selectionResult := make(chan addressResult, 1)
	go func() {
		address, err := manager.GetLocalIP(context.Background())
		selectionResult <- addressResult{address: address.String(), err: err}
	}()
	<-selectionEntered

	baseContext, cancel := context.WithCancel(context.Background())
	startContext := newObservedContext(baseContext)
	startResult := make(chan error, 1)
	go func() { startResult <- manager.StartBeacon(startContext, validBeaconRequest()) }()
	<-startContext.observed
	cancel()
	err := receiveError(t, startResult)
	if transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning || !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting StartBeacon() error = %v", err)
	}
	if factoryCalls != 0 {
		t.Fatalf("cancelled waiting StartBeacon called factory %d times", factoryCalls)
	}

	close(releaseSelection)
	selected := receiveAddressResult(t, selectionResult)
	if selected.err != nil || selected.address != "192.168.65.8" {
		t.Fatalf("GetLocalIP() = %q, %v", selected.address, selected.err)
	}
}

func TestGetLocalIPWaitCancellationDoesNotWaitForInFlightSelection(t *testing.T) {
	t.Parallel()

	selectionEntered := make(chan struct{})
	releaseSelection := make(chan struct{})
	iface := net.Interface{Index: 7, Name: "Ethernet", Flags: net.FlagUp | net.FlagBroadcast}
	manager := newManager(managerDependencies{
		interfaces: func() ([]net.Interface, error) { return []net.Interface{iface}, nil },
		addresses: func(net.Interface) ([]net.Addr, error) {
			close(selectionEntered)
			<-releaseSelection
			return []net.Addr{mustIPNet("192.168.70.9/24")}, nil
		},
		hostname: func() (string, error) { return "test-host", nil },
		entropy:  bytes.NewReader(make([]byte, processSuffixBytes)),
		start: func(*mdns.Config) (beaconHandle, error) {
			panic("unexpected beacon start")
		},
	})

	firstResult := make(chan addressResult, 1)
	go func() {
		address, err := manager.GetLocalIP(context.Background())
		firstResult <- addressResult{address: address.String(), err: err}
	}()
	<-selectionEntered

	baseContext, cancel := context.WithCancel(context.Background())
	waitContext := newObservedContext(baseContext)
	waitResult := make(chan error, 1)
	go func() {
		_, err := manager.GetLocalIP(waitContext)
		waitResult <- err
	}()
	<-waitContext.observed
	cancel()
	err := receiveError(t, waitResult)
	if transfer.ErrorCodeOf(err) != transfer.ErrCancelled || !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting GetLocalIP() error = %v", err)
	}

	close(releaseSelection)
	first := receiveAddressResult(t, firstResult)
	if first.err != nil || first.address != "192.168.70.9" {
		t.Fatalf("first GetLocalIP() = %q, %v", first.address, first.err)
	}
}

func TestStartBeaconRejectsInvalidAndDuplicateRequestsWithoutReplacement(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	factoryCalls := 0
	manager := beaconTestManager(t, bytes.NewReader(make([]byte, processSuffixBytes)), func(*mdns.Config) (beaconHandle, error) {
		mu.Lock()
		factoryCalls++
		mu.Unlock()
		return &fakeBeacon{}, nil
	})

	invalid := []struct {
		name    string
		request transfer.BeaconRequest
	}{
		{name: "wrong service", request: mutateRequest(func(request *transfer.BeaconRequest) { request.Service = "_other._tcp" })},
		{name: "blank instance", request: mutateRequest(func(request *transfer.BeaconRequest) { request.Instance = "" })},
		{name: "unsafe instance", request: mutateRequest(func(request *transfer.BeaconRequest) { request.Instance = "Fair Drop" })},
		{name: "long instance", request: mutateRequest(func(request *transfer.BeaconRequest) { request.Instance = strings.Repeat("a", maxInstanceBaseBytes+1) })},
		{name: "zero port", request: mutateRequest(func(request *transfer.BeaconRequest) { request.Port = 0 })},
		{name: "large port", request: mutateRequest(func(request *transfer.BeaconRequest) { request.Port = 65536 })},
		{name: "missing TXT", request: mutateRequest(func(request *transfer.BeaconRequest) { request.TXT = nil })},
		{name: "extra TXT", request: mutateRequest(func(request *transfer.BeaconRequest) {
			request.TXT = []string{transfer.BeaconVersionTXT, "token=secret"}
		})},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if err := manager.StartBeacon(context.Background(), test.request); transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning {
				t.Fatalf("StartBeacon() code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrBeaconWarning)
			}
		})
	}
	if factoryCalls != 0 {
		t.Fatalf("invalid requests called factory %d times", factoryCalls)
	}

	if err := manager.StartBeacon(context.Background(), validBeaconRequest()); err != nil {
		t.Fatalf("first StartBeacon() error = %v", err)
	}
	if err := manager.StartBeacon(context.Background(), validBeaconRequest()); transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning {
		t.Fatalf("duplicate StartBeacon() code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrBeaconWarning)
	}
	if factoryCalls != 1 {
		t.Fatalf("duplicate StartBeacon called factory %d times, want 1", factoryCalls)
	}
	if err := manager.StopBeacon(); err != nil {
		t.Fatalf("StopBeacon() error = %v", err)
	}
}

func TestStartBeaconRequiresSelectionAndLiveContext(t *testing.T) {
	t.Parallel()

	factoryCalls := 0
	manager := unselectedBeaconTestManager(bytes.NewReader(make([]byte, processSuffixBytes)), func(*mdns.Config) (beaconHandle, error) {
		factoryCalls++
		return &fakeBeacon{}, nil
	})
	if err := manager.StartBeacon(context.Background(), validBeaconRequest()); transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning {
		t.Fatalf("unselected StartBeacon() code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrBeaconWarning)
	}
	if err := manager.StartBeacon(nil, validBeaconRequest()); transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning {
		t.Fatalf("nil-context StartBeacon() code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrBeaconWarning)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := manager.StartBeacon(ctx, validBeaconRequest()); transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning {
		t.Fatalf("cancelled StartBeacon() code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrBeaconWarning)
	}
	if factoryCalls != 0 {
		t.Fatalf("invalid StartBeacon calls invoked factory %d times", factoryCalls)
	}
}

func TestStartBeaconCleansPartialHandleOnFactoryFailure(t *testing.T) {
	t.Parallel()

	startFailure := errors.New("start failed")
	cleanupFailure := errors.New("cleanup diagnostic")
	handle := &fakeBeacon{err: cleanupFailure}
	manager := beaconTestManager(t, bytes.NewReader(make([]byte, processSuffixBytes)), func(*mdns.Config) (beaconHandle, error) {
		return handle, startFailure
	})
	err := manager.StartBeacon(context.Background(), validBeaconRequest())
	if transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning {
		t.Fatalf("StartBeacon() code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrBeaconWarning)
	}
	if !errors.Is(err, startFailure) || !errors.Is(err, cleanupFailure) {
		t.Fatalf("StartBeacon() did not preserve start and cleanup causes: %v", err)
	}
	if handle.calls() != 1 || manager.beacon != nil {
		t.Fatalf("failed StartBeacon retained ownership: shutdown calls=%d active=%v", handle.calls(), manager.beacon)
	}
	if err := manager.StopBeacon(); err != nil {
		t.Fatalf("StopBeacon() after failed start = %v", err)
	}
}

func TestStartBeaconTreatsTypedNilPartialHandleAsAbsent(t *testing.T) {
	t.Parallel()

	startFailure := errors.New("start failed with typed nil")
	var typedNil *fakeBeacon
	manager := beaconTestManager(t, bytes.NewReader(make([]byte, processSuffixBytes)), func(*mdns.Config) (beaconHandle, error) {
		return typedNil, startFailure
	})
	err := manager.StartBeacon(context.Background(), validBeaconRequest())
	if transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning || !errors.Is(err, startFailure) {
		t.Fatalf("StartBeacon() error = %v", err)
	}
	manager.mu.Lock()
	active := manager.beacon
	manager.mu.Unlock()
	if beaconHandlePresent(active) {
		t.Fatal("StartBeacon retained a typed-nil handle")
	}
	if err := manager.StopBeacon(); err != nil {
		t.Fatalf("StopBeacon() after typed-nil failure = %v", err)
	}
}

func TestStartBeaconCleansHandleWhenContextCancelsAfterCreation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	handle := &fakeBeacon{}
	manager := beaconTestManager(t, bytes.NewReader(make([]byte, processSuffixBytes)), func(*mdns.Config) (beaconHandle, error) {
		cancel()
		return handle, nil
	})
	err := manager.StartBeacon(ctx, validBeaconRequest())
	if transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning {
		t.Fatalf("StartBeacon() code = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrBeaconWarning)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("StartBeacon() does not preserve context cancellation: %v", err)
	}
	if handle.calls() != 1 || manager.beacon != nil {
		t.Fatalf("cancelled StartBeacon retained ownership: shutdown calls=%d active=%v", handle.calls(), manager.beacon)
	}
}

func TestStartBeaconReusesProcessIdentityAcrossRetry(t *testing.T) {
	t.Parallel()

	firstFailure := errors.New("first start fails")
	var instances []string
	calls := 0
	manager := beaconTestManager(t, bytes.NewReader(bytes.Repeat([]byte{0xab}, processSuffixBytes)), func(config *mdns.Config) (beaconHandle, error) {
		instances = append(instances, config.Zone.(*mdns.MDNSService).Instance)
		calls++
		if calls == 1 {
			return nil, firstFailure
		}
		return &fakeBeacon{}, nil
	})
	if err := manager.StartBeacon(context.Background(), validBeaconRequest()); !errors.Is(err, firstFailure) {
		t.Fatalf("first StartBeacon() error = %v", err)
	}
	if err := manager.StartBeacon(context.Background(), validBeaconRequest()); err != nil {
		t.Fatalf("second StartBeacon() error = %v", err)
	}
	if len(instances) != 2 || instances[0] != instances[1] {
		t.Fatalf("process identity changed across retry: %v", instances)
	}
	if err := manager.StopBeacon(); err != nil {
		t.Fatalf("StopBeacon() error = %v", err)
	}
}

func TestStartBeaconIdentityFailuresPreserveCauseWithoutOwnership(t *testing.T) {
	t.Parallel()

	hostnameFailure := errors.New("hostname unavailable")
	tests := []struct {
		name     string
		hostname func() (string, error)
		entropy  io.Reader
		cause    error
	}{
		{
			name:     "hostname error",
			hostname: func() (string, error) { return "", hostnameFailure },
			entropy:  bytes.NewReader(make([]byte, processSuffixBytes)),
			cause:    hostnameFailure,
		},
		{
			name:     "short entropy",
			hostname: func() (string, error) { return "test-host", nil },
			entropy:  bytes.NewReader([]byte{1, 2, 3}),
			cause:    io.ErrUnexpectedEOF,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			factoryCalls := 0
			iface := net.Interface{Index: 7, Name: "Ethernet", Flags: net.FlagUp | net.FlagBroadcast}
			manager := newManager(managerDependencies{
				interfaces: func() ([]net.Interface, error) { return []net.Interface{iface}, nil },
				addresses: func(net.Interface) ([]net.Addr, error) {
					return []net.Addr{mustIPNet("192.168.50.9/24")}, nil
				},
				hostname: test.hostname,
				entropy:  test.entropy,
				start: func(*mdns.Config) (beaconHandle, error) {
					factoryCalls++
					return &fakeBeacon{}, nil
				},
			})
			if _, err := manager.GetLocalIP(context.Background()); err != nil {
				t.Fatalf("GetLocalIP() error = %v", err)
			}
			err := manager.StartBeacon(context.Background(), validBeaconRequest())
			if transfer.ErrorCodeOf(err) != transfer.ErrBeaconWarning || !errors.Is(err, test.cause) {
				t.Fatalf("StartBeacon() error = %v", err)
			}
			manager.mu.Lock()
			identity, beacon := manager.identity, manager.beacon
			manager.mu.Unlock()
			if identity != nil || beaconHandlePresent(beacon) || factoryCalls != 0 {
				t.Fatalf("identity failure retained identity=%v beacon=%v factory calls=%d", identity, beacon, factoryCalls)
			}
		})
	}
}

func TestConcurrentStartBeaconHasOneWinnerAndOneFactoryCall(t *testing.T) {
	t.Parallel()

	factoryCalls := 0
	var factoryMu sync.Mutex
	manager := beaconTestManager(t, bytes.NewReader(make([]byte, processSuffixBytes)), func(*mdns.Config) (beaconHandle, error) {
		factoryMu.Lock()
		factoryCalls++
		factoryMu.Unlock()
		return &fakeBeacon{}, nil
	})

	startLine := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-startLine
			results <- manager.StartBeacon(context.Background(), validBeaconRequest())
		}()
	}
	close(startLine)
	first, second := receiveError(t, results), receiveError(t, results)
	successes := 0
	warnings := 0
	for _, err := range []error{first, second} {
		switch transfer.ErrorCodeOf(err) {
		case "":
			successes++
		case transfer.ErrBeaconWarning:
			warnings++
		default:
			t.Fatalf("StartBeacon() unexpected error = %v", err)
		}
	}
	factoryMu.Lock()
	calls := factoryCalls
	factoryMu.Unlock()
	if successes != 1 || warnings != 1 || calls != 1 {
		t.Fatalf("concurrent starts: successes=%d warnings=%d factory calls=%d", successes, warnings, calls)
	}
	if err := manager.StopBeacon(); err != nil {
		t.Fatalf("StopBeacon() error = %v", err)
	}
}

func TestStopBeaconIsIdempotentConcurrentAndForgetsCleanupErrors(t *testing.T) {
	t.Parallel()

	if err := unselectedBeaconTestManager(bytes.NewReader(make([]byte, processSuffixBytes)), nil).StopBeacon(); err != nil {
		t.Fatalf("StopBeacon() before start = %v", err)
	}

	cleanupFailure := errors.New("cleanup diagnostic")
	entered := make(chan struct{})
	release := make(chan struct{})
	handle := &fakeBeacon{err: cleanupFailure, entered: entered, release: release}
	manager := beaconTestManager(t, bytes.NewReader(make([]byte, processSuffixBytes)), func(*mdns.Config) (beaconHandle, error) {
		return handle, nil
	})
	if err := manager.StartBeacon(context.Background(), validBeaconRequest()); err != nil {
		t.Fatalf("StartBeacon() error = %v", err)
	}

	results := make(chan error, 2)
	go func() { results <- manager.StopBeacon() }()
	<-entered
	go func() { results <- manager.StopBeacon() }()
	close(release)
	first, second := <-results, <-results
	if transfer.ErrorCodeOf(first) != transfer.ErrBeaconWarning && transfer.ErrorCodeOf(second) != transfer.ErrBeaconWarning {
		t.Fatalf("concurrent StopBeacon calls did not report cleanup diagnostic: %v, %v", first, second)
	}
	if first != nil && second != nil {
		t.Fatalf("both concurrent StopBeacon calls returned errors: %v, %v", first, second)
	}
	if handle.calls() != 1 || manager.beacon != nil {
		t.Fatalf("StopBeacon ownership mismatch: shutdown calls=%d active=%v", handle.calls(), manager.beacon)
	}
	if err := manager.StopBeacon(); err != nil {
		t.Fatalf("repeated StopBeacon() = %v", err)
	}
}

func beaconTestManager(t *testing.T, entropy io.Reader, start func(*mdns.Config) (beaconHandle, error)) *Manager {
	t.Helper()
	manager := unselectedBeaconTestManager(entropy, start)
	address, err := manager.GetLocalIP(context.Background())
	if err != nil || address.String() != "192.168.50.9" {
		t.Fatalf("GetLocalIP() = %v, %v", address, err)
	}
	return manager
}

func unselectedBeaconTestManager(entropy io.Reader, start func(*mdns.Config) (beaconHandle, error)) *Manager {
	if start == nil {
		start = func(*mdns.Config) (beaconHandle, error) { panic("unexpected beacon start") }
	}
	iface := net.Interface{Index: 7, Name: "Ethernet", Flags: net.FlagUp | net.FlagBroadcast}
	return newManager(managerDependencies{
		interfaces: func() ([]net.Interface, error) { return []net.Interface{iface}, nil },
		addresses: func(net.Interface) ([]net.Addr, error) {
			ip, network, _ := net.ParseCIDR("192.168.50.9/24")
			network.IP = ip
			return []net.Addr{network}, nil
		},
		hostname: func() (string, error) { return "Work Station.local", nil },
		entropy:  entropy,
		start:    start,
	})
}

func validBeaconRequest() transfer.BeaconRequest {
	return transfer.BeaconRequest{
		SessionID: "session-id",
		Service:   transfer.BeaconService,
		Instance:  "FairDrop",
		Port:      41234,
		TXT:       []string{transfer.BeaconVersionTXT},
	}
}

func mutateRequest(mutate func(*transfer.BeaconRequest)) transfer.BeaconRequest {
	request := validBeaconRequest()
	mutate(&request)
	return request
}

type fakeBeacon struct {
	mu       sync.Mutex
	count    int
	err      error
	entered  chan struct{}
	release  chan struct{}
	enterOne sync.Once
}

func (beacon *fakeBeacon) Shutdown() error {
	beacon.mu.Lock()
	beacon.count++
	beacon.mu.Unlock()
	if beacon.entered != nil {
		beacon.enterOne.Do(func() { close(beacon.entered) })
	}
	if beacon.release != nil {
		<-beacon.release
	}
	return beacon.err
}

func (beacon *fakeBeacon) calls() int {
	beacon.mu.Lock()
	defer beacon.mu.Unlock()
	return beacon.count
}

type observedContext struct {
	context.Context
	observed chan struct{}
	once     sync.Once
}

func newObservedContext(ctx context.Context) *observedContext {
	return &observedContext{Context: ctx, observed: make(chan struct{})}
}

func (ctx *observedContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.observed) })
	return ctx.Context.Done()
}

type addressResult struct {
	address string
	err     error
}

func receiveAddressResult(t *testing.T, results <-chan addressResult) addressResult {
	t.Helper()
	select {
	case result := <-results:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for address result")
		return addressResult{}
	}
}

func receiveError(t *testing.T, results <-chan error) error {
	t.Helper()
	select {
	case err := <-results:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for operation result")
		return nil
	}
}

func mustIPNet(value string) net.Addr {
	ip, network, err := net.ParseCIDR(value)
	if err != nil {
		panic(err)
	}
	network.IP = ip
	return network
}
