package transfer

import (
	"context"
	"errors"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type recoveryEntropyFunc func([]byte) (int, error)

func (read recoveryEntropyFunc) Read(p []byte) (int, error) { return read(p) }

func TestRefreshConnectionRetiresOldSendBeforePublishingFreshIdentity(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	options, err := h.coordinator.ListConnectionOptions(context.Background(), old.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if options.CurrentAddress != "192.168.1.50" || len(options.Choices) != 1 {
		t.Fatalf("options = %+v", options)
	}
	if h.calls.count("network.StopBeacon") != 0 {
		t.Fatal("listing mutated the live beacon")
	}
	result, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "lan-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "send" || result.Send == nil || result.Receive != nil {
		t.Fatalf("result = %+v", result)
	}
	if result.Send.SessionID == old.SessionID || result.Send.URL == old.URL || !strings.Contains(result.Send.URL, "/download/") {
		t.Fatalf("identity did not rotate: %+v", result.Send)
	}
	if requests := h.server.startRequests(); len(requests) != 2 || requests[0].Token == requests[1].Token {
		t.Fatalf("server starts = %+v", requests)
	}
	if events := h.observer.published(); len(events) != 1 || events[0].Kind != TransferReset || events[0].SessionID != old.SessionID {
		t.Fatalf("retirement events = %+v", events)
	}
	if err := h.coordinator.AuthorizeClaim(context.Background(), old.SessionID); err == nil {
		t.Fatal("retired session authorized")
	}
	if got := h.calls.snapshot(); callIndex(got, "server.Stop") > callIndex(got, "server.Start", 1) {
		t.Fatalf("replacement listener preceded old stop: %v", got)
	}
}

func TestRefreshConnectionForwardsDistinctExplicitEndpointIntoURLAndBeacon(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	const chosen = "other-lan"
	var selected string
	var beaconAfterSelection bool
	h.network.options = func(context.Context) ([]ConnectionOption, error) {
		return []ConnectionOption{{ID: chosen, Name: "Ethernet", Address: "10.2.3.4"}}, nil
	}
	h.network.selectEndpoint = func(_ context.Context, id string) (netip.Addr, error) {
		selected = id
		h.network.mu.Lock()
		h.network.addrChosen = true
		h.network.mu.Unlock()
		return netip.MustParseAddr("10.2.3.4"), nil
	}
	h.network.startBeacon = func(context.Context, BeaconRequest) error {
		beaconAfterSelection = selected == chosen
		return nil
	}
	result, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, chosen)
	if err != nil {
		t.Fatal(err)
	}
	if selected != chosen || !beaconAfterSelection || result.Send == nil || !strings.HasPrefix(result.Send.URL, "http://10.2.3.4:") || len(h.network.beaconRequests()) != 2 {
		t.Fatalf("selection=%q beacon=%t result=%+v requests=%+v", selected, beaconAfterSelection, result, h.network.beaconRequests())
	}
}

func TestRefreshConnectionPreflightRefusesStaleChoiceWithoutRetirement(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "missing")
	if ErrorCodeOf(err) != ErrNetworkUnavailable {
		t.Fatalf("code = %v", err)
	}
	if h.state() != stateStaged || h.calls.count("server.Stop") != 0 || len(h.observer.published()) != 0 {
		t.Fatal("preflight failure retired the waiting session")
	}
}

func TestRefreshConnectionRefusesEmptyAndStaleSessionIDs(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	for _, id := range []SessionID{"", "ffffffffffffffffffffffffffffffff"} {
		if _, err := h.coordinator.ListConnectionOptions(context.Background(), id); err == nil {
			t.Fatalf("listed %q", id)
		}
		if _, err := h.coordinator.RefreshConnection(context.Background(), id, ""); err == nil {
			t.Fatalf("refreshed %q", id)
		}
	}
	if h.state() != stateStaged || h.calls.count("server.Stop") != 0 || len(h.observer.published()) != 0 {
		t.Fatal("bad IDs changed the session")
	}
	if _, err := h.coordinator.ListConnectionOptions(context.Background(), old.SessionID); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshReservationExcludesClaimAndSecondRefresh(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	h.server.stop = func() error { once.Do(func() { close(entered) }); <-release; return nil }
	result := make(chan error, 1)
	go func() {
		_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
		result <- err
	}()
	<-entered
	if err := h.coordinator.AuthorizeClaim(context.Background(), old.SessionID); err == nil {
		t.Fatal("claim crossed refresh reservation")
	}
	if _, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, ""); err == nil {
		t.Fatal("duplicate refresh crossed reservation")
	}
	close(release)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestRecoveryReservationIsClaimFenceBeforeAdapterStop(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	var fenced bool
	h.server.stop = func() error { fenced = h.state() == stateStaging; return nil }
	_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
	if !fenced {
		t.Fatal("claim fence was not reserved before old listener stop")
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestCancelDuringRecoveryRetirementPreventsReplacement(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	h.server.stop = func() error { once.Do(func() { close(entered) }); <-release; return nil }
	refreshDone := make(chan error, 1)
	go func() {
		_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
		refreshDone <- err
	}()
	<-entered
	cancelDone := make(chan error, 1)
	go func() { cancelDone <- h.coordinator.Cancel(context.Background()) }()
	// Wait for the cancellation marker, not elapsed time.
	for {
		h.coordinator.mu.Lock()
		cancelled := h.coordinator.session.cancelled
		h.coordinator.mu.Unlock()
		if cancelled {
			break
		}
		runtime.Gosched()
	}
	close(release)
	if err := <-refreshDone; ErrorCodeOf(err) != ErrCancelled {
		t.Fatalf("refresh = %v", err)
	}
	if err := <-cancelDone; err != nil {
		t.Fatalf("cancel = %v", err)
	}
	if h.state() != stateIdle || len(h.server.startRequests()) != 1 {
		t.Fatal("cancel allowed replacement")
	}
}

func TestCancelDuringRecoveryIdentityGenerationPreventsReplacement(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	entered := make(chan struct{})
	release := make(chan struct{})
	var enteredOnce sync.Once
	h.coordinator.entropy = recoveryEntropyFunc(func(p []byte) (int, error) {
		enteredOnce.Do(func() { close(entered) })
		<-release
		for i := range p {
			p[i] = byte(i + 1)
		}
		return len(p), nil
	})
	refreshDone := make(chan error, 1)
	go func() {
		_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
		refreshDone <- err
	}()
	<-entered
	cancelDone := make(chan error, 1)
	go func() { cancelDone <- h.coordinator.Cancel(context.Background()) }()
	for {
		h.coordinator.mu.Lock()
		cancelled := h.coordinator.session != nil && h.coordinator.session.cancelled
		h.coordinator.mu.Unlock()
		if cancelled {
			break
		}
		runtime.Gosched()
	}
	close(release)
	if err := <-refreshDone; ErrorCodeOf(err) != ErrCancelled {
		t.Fatalf("refresh = %v", err)
	}
	if err := <-cancelDone; err != nil {
		t.Fatalf("cancel = %v", err)
	}
	if h.state() != stateIdle || len(h.server.startRequests()) != 1 {
		t.Fatal("identity handoff admitted replacement")
	}
}

func TestShutdownDuringRecoveryRebuildPreventsCommit(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	entered := make(chan struct{})
	release := make(chan struct{})
	h.network.selectEndpoint = func(context.Context, string) (netip.Addr, error) { close(entered); <-release; return testAddr(), nil }
	refreshDone := make(chan error, 1)
	go func() {
		_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
		refreshDone <- err
	}()
	<-entered
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- h.coordinator.Shutdown(context.Background()) }()
	for {
		h.coordinator.mu.Lock()
		closing := h.coordinator.closing
		h.coordinator.mu.Unlock()
		if closing {
			break
		}
		runtime.Gosched()
	}
	close(release)
	if err := <-refreshDone; ErrorCodeOf(err) != ErrShuttingDown {
		t.Fatalf("refresh = %v", err)
	}
	if err := <-shutdownDone; err != nil {
		t.Fatalf("shutdown = %v", err)
	}
	if h.state() != stateIdle || len(h.server.startRequests()) != 1 {
		t.Fatal("shutdown allowed replacement")
	}
}

func TestRefreshConnectionCannotCancelWinningClaim(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	entered := make(chan struct{})
	release := make(chan struct{})
	h.network.options = func(context.Context) ([]ConnectionOption, error) {
		close(entered)
		<-release
		return []ConnectionOption{{ID: "lan-1", Name: "Wi-Fi", Address: testAddress}}, nil
	}
	result := make(chan error, 1)
	go func() {
		_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
		result <- err
	}()
	<-entered
	if err := h.coordinator.AuthorizeClaim(context.Background(), old.SessionID); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-result; err == nil {
		t.Fatal("losing recovery succeeded")
	}
	if h.state() != stateTransferring || h.calls.count("server.Stop") != 0 {
		t.Fatal("winning claim was stopped")
	}
}

func TestRefreshConnectionCleanupFailureBlocksReplacement(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	h.server.keepLaneOpenOnStop = true
	defer h.server.closeEvents()
	result := make(chan error, 1)
	go func() {
		_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
		result <- err
	}()
	var err error
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err = <-result:
			goto settled
		default:
			if time.Now().After(deadline) {
				t.Fatal("recovery did not settle after cleanup bound")
			}
			if !h.bounds.fireIfArmed() {
				runtime.Gosched()
			}
		}
	}
settled:
	if ErrorCodeOf(err) != ErrCleanupUnconfirmed {
		t.Fatalf("cleanup = %v", err)
	}
	if len(h.server.startRequests()) != 1 {
		t.Fatal("replacement listener started without drainer quiescence")
	}
	if events := h.awaitEvents(1); len(events) != 1 || events[0].Kind != TransferReset {
		t.Fatalf("old code not retired: %+v", events)
	}
}

func TestRefreshReceiveRetainsOneDestinationAndClosesOnExit(t *testing.T) {
	h := newReceiveHarness(t)
	old := h.receiveWaiting()
	result, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "receive" || result.Receive == nil || result.Send != nil || result.Receive.SessionID == old.SessionID {
		t.Fatalf("result = %+v", result)
	}
	if len(h.sink.opened()) != 1 || h.sink.destination(0).closeCalls() != 0 {
		t.Fatal("receive destination was reopened or closed")
	}
	if requests := h.server.startRequests(); len(requests) != 2 || requests[0].Destination != requests[1].Destination {
		t.Fatalf("destination handoff = %+v", requests)
	}
	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if h.sink.destination(0).closeCalls() != 1 {
		t.Fatalf("destination closes = %d", h.sink.destination(0).closeCalls())
	}
}

func TestRefreshReceiveReplacementFailureClosesRetainedDestinationOnce(t *testing.T) {
	h := newReceiveHarness(t)
	old := h.receiveWaiting()
	h.qr.encode = func(context.Context, string) ([]byte, error) { return nil, errors.New("private QR failure") }
	_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
	if err == nil {
		t.Fatal("replacement unexpectedly succeeded")
	}
	if h.state() != stateIdle || len(h.sink.opened()) != 1 || h.sink.destination(0).closeCalls() != 1 {
		t.Fatal("destination ownership not settled once")
	}
}

func TestRefreshFailureAfterRetirementRemovesOldCode(t *testing.T) {
	h := newHarness(t)
	old := h.stageSuccessfully()
	h.qr.encode = func(context.Context, string) ([]byte, error) { return nil, errors.New("private adapter detail") }
	_, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
	if err == nil {
		t.Fatal("replacement unexpectedly succeeded")
	}
	if h.state() != stateIdle || len(h.observer.published()) != 1 || h.observer.published()[0].Kind != TransferReset {
		t.Fatal("old code remains after replacement failure")
	}
}

func TestRefreshCollectionReinspectsInArchiveOrder(t *testing.T) {
	h := newHarness(t)
	paths := []string{"/a/one", "/b/two", "/c/three"}
	h.source.inspect = func(_ context.Context, path string) (StagedItem, error) {
		item := testItem()
		item.Path, item.Name = path, path[len(path)-3:]
		return item, nil
	}
	old, err := h.coordinator.StageTransfers(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Send == nil || !fresh.Send.IsCollection || fresh.Send.ItemCount != 3 {
		t.Fatalf("result = %+v", fresh)
	}
	inspected := h.source.inspected()
	if len(inspected) != 6 {
		t.Fatalf("inspections = %v", inspected)
	}
	for index, path := range paths {
		if inspected[index] != path || inspected[index+3] != path {
			t.Fatalf("inspection order = %v", inspected)
		}
		if h.liveSession().item.Collection.Members[index].Path != path {
			t.Fatalf("archive order = %+v", h.liveSession().item.Collection.Members)
		}
	}
}

func TestRefreshCollectionInspectionFailureRetiresOldCode(t *testing.T) {
	h := newHarness(t)
	paths := []string{"/a/one", "/b/two"}
	fail := false
	h.source.inspect = func(_ context.Context, path string) (StagedItem, error) {
		if fail && path == paths[1] {
			return StagedItem{}, NewError(ErrSourceChanged, "selected item changed")
		}
		item := testItem()
		item.Path = path
		return item, nil
	}
	old, err := h.coordinator.StageTransfers(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	fail = true
	_, err = h.coordinator.RefreshConnection(context.Background(), old.SessionID, "")
	if ErrorCodeOf(err) != ErrSourceChanged || h.state() != stateIdle || len(h.server.startRequests()) != 1 {
		t.Fatalf("failed reinspection = %v, state=%v, starts=%d", err, h.state(), len(h.server.startRequests()))
	}
	if err := h.coordinator.AuthorizeClaim(context.Background(), old.SessionID); err == nil {
		t.Fatal("old collection code claimed")
	}
}

func callIndex(calls []string, name string, skip ...int) int {
	want := 0
	if len(skip) != 0 {
		want = skip[0]
	}
	for index, call := range calls {
		if call == name {
			if want == 0 {
				return index
			}
			want--
		}
	}
	return len(calls)
}
