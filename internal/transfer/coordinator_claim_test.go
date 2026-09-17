package transfer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestAuthorizeClaimCommitsAndPublishesStarted(t *testing.T) {
	h := newHarness(t)
	metadata := h.stageSuccessfully()
	before := len(h.calls.snapshot())

	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		t.Fatalf("AuthorizeClaim returned %v, want a committed transfer", err)
	}

	if got := h.state(); got != stateTransferring {
		t.Errorf("state is %q, want %q", got, stateTransferring)
	}
	if h.coordinator.leaseHeld() {
		t.Error("the operation lease was not returned after publication")
	}

	events := h.observer.published()
	if len(events) != 1 {
		t.Fatalf("authorization published %+v, want exactly one event", events)
	}
	want := Event{SessionID: metadata.SessionID, Seq: 1, Kind: TransferStarted}
	if events[0] != want {
		t.Errorf("published %+v, want %+v", events[0], want)
	}
	if events[0].Progress != nil || events[0].Error != nil {
		t.Errorf("started carried a payload: %+v", events[0])
	}
	if !h.observer.leaseHeldAt(0) {
		t.Error("started was published after the operation lease was released")
	}

	// The commit happens first and the beacon stops after it, both without the
	// mutex and both still under the operation lease. That order is the point:
	// the beacon stop used to run ahead of the commit, holding the receiver's
	// open request behind a mDNS teardown bounded at fifteen seconds, and the
	// beacon's liveness never had any bearing on whether the claim proceeds
	// (owner decision, 2026-09-15; D-024 had already settled the same question
	// for a teardown that never returns at all).
	//
	// Each timer.AfterFunc arms a bound before the call it bounds: the first
	// arms observerPublishBound around the publication (D-034, so a blocking
	// observer cannot hold the lease), the second arms the StopBeacon bound
	// (D-024). Both are stopped the instant their call returns, but each is
	// armed first, deterministically, so it always precedes the call it bounds.
	want0 := []string{"clock.Now", "timer.AfterFunc", "observer.Publish", "timer.AfterFunc", "network.StopBeacon"}
	if got := h.calls.snapshot()[before:]; !slices.Equal(got, want0) {
		t.Errorf("the claim handshake ran %v, want %v", got, want0)
	}

	live := h.liveSession()
	if !live.startedAt.Equal(h.clock.current) || !live.startedAt.After(live.stagedAt) {
		t.Errorf("the transfer is stamped %v, want the injected clock's %v after the staging stamp %v",
			live.startedAt, h.clock.current, live.stagedAt)
	}
}

func TestAuthorizeClaimPublishesStartedBeforeACancellationCanFollow(t *testing.T) {
	h := newHarness(t)
	metadata := h.stageSuccessfully()
	// Cancel arrives while started is being published: the commit already won,
	// so the event stands and the cancellation becomes the next outcome rather
	// than erasing this one.
	h.observer.publish = func(Event) { h.coordinator.cancelSession() }

	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		t.Fatalf("AuthorizeClaim returned %v, want the commit to win", err)
	}

	events := h.observer.published()
	if len(events) != 1 || events[0].Kind != TransferStarted || events[0].Seq != 1 {
		t.Fatalf("published %+v, want one started event at seq 1", events)
	}
	if got := h.state(); got != stateTransferring {
		t.Errorf("state is %q, want %q", got, stateTransferring)
	}
	if live := h.liveSession(); live == nil || !live.cancelled {
		t.Error("the cancellation that lost the race was not recorded")
	}
}

func TestAuthorizeClaimLosesToACancellationBeforeTheCommit(t *testing.T) {
	h := newHarness(t)
	metadata := h.stageSuccessfully()
	// The startedAt clock read is the one unlocked step inside the handshake,
	// so it is where a cancellation can land after CLAIMING and before
	// TRANSFERRING. It used to be the beacon stop; that call moved after the
	// commit, because holding the receiver's open request behind a slow mDNS
	// teardown for up to AdapterCleanupBound was the defect. The guarantee
	// under test is unchanged -- a cancellation that lands before the commit
	// wins, and no started event is published -- only the step it lands in.
	h.clock.onNow = func() { h.coordinator.cancelSession() }

	err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID)

	if got := ErrorCodeOf(err); got != ErrCancelled {
		t.Errorf("error code is %q, want %q", got, ErrCancelled)
	}
	if events := h.observer.published(); len(events) != 0 {
		t.Errorf("a refused claim published %+v, want no started event at all", events)
	}
	if got := h.state(); got == stateTransferring {
		t.Error("a refused claim committed TRANSFERRING")
	}
	if h.coordinator.leaseHeld() {
		t.Error("a refused claim kept the operation lease that the teardown needs")
	}
}

func TestAuthorizeClaimRefusesOutsideAMatchingStagedSession(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		run      func(t *testing.T, h *harness) error
		wantCode ErrorCode
	}{
		{
			name: "no session at all",
			run: func(_ *testing.T, h *harness) error {
				return h.coordinator.AuthorizeClaim(context.Background(), testSessionID)
			},
			wantCode: ErrCancelled,
		},
		{
			name: "a different session",
			run: func(_ *testing.T, h *harness) error {
				h.stageSuccessfully()
				return h.coordinator.AuthorizeClaim(context.Background(), "some-other-session")
			},
			wantCode: ErrCancelled,
		},
		{
			name: "still staging",
			run: func(_ *testing.T, h *harness) error {
				var claimErr error
				h.qr.encode = func(context.Context, string) ([]byte, error) {
					claimErr = h.coordinator.AuthorizeClaim(context.Background(), testSessionID)
					return append([]byte(nil), testPNG...), nil
				}
				h.stageSuccessfully()
				return claimErr
			},
			wantCode: ErrCancelled,
		},
		{
			name: "already cancelled",
			run: func(_ *testing.T, h *harness) error {
				metadata := h.stageSuccessfully()
				h.coordinator.cancelSession()
				return h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID)
			},
			wantCode: ErrCancelled,
		},
		{
			name: "already transferring",
			run: func(t *testing.T, h *harness) error {
				metadata := h.stageSuccessfully()
				if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
					t.Fatalf("the first claim returned %v", err)
				}
				return h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID)
			},
			wantCode: ErrCancelled,
		},
		{
			name: "a cancelled request context",
			run: func(_ *testing.T, h *harness) error {
				metadata := h.stageSuccessfully()
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return h.coordinator.AuthorizeClaim(ctx, metadata.SessionID)
			},
			wantCode: ErrCancelled,
		},
		{
			name: "the application is closing",
			run: func(_ *testing.T, h *harness) error {
				metadata := h.stageSuccessfully()
				h.coordinator.beginClosing()
				return h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID)
			},
			wantCode: ErrShuttingDown,
		},
		{
			name: "a teardown already owns the lease",
			run: func(t *testing.T, h *harness) error {
				metadata := h.stageSuccessfully()
				h.coordinator.mu.Lock()
				took := h.coordinator.acquireLease()
				h.coordinator.mu.Unlock()
				if !took {
					t.Fatal("the lease was still held after a committed Stage")
				}
				defer h.coordinator.releaseLease()
				return h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID)
			},
			wantCode: ErrCancelled,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)

			err := testCase.run(t, h)

			if got := ErrorCodeOf(err); got != testCase.wantCode {
				t.Errorf("error code is %q, want %q", got, testCase.wantCode)
			}
			started := 0
			for _, event := range h.observer.published() {
				if event.Kind == TransferStarted {
					started++
				}
			}
			if started > 1 {
				t.Errorf("%d started events exist, want at most the one a winning claim publishes", started)
			}
			if got := h.state(); got == stateClaiming {
				t.Error("a refused claim left the coordinator in CLAIMING")
			}
		})
	}
}

// An empty session id must be refused, not treated as a wildcard. ClaimAuthorizer
// is a public interface: the coordinator cannot assume its caller passes the id
// it was given, and "names no session" must never mean "matches whichever
// session is staged".
func TestAuthorizeClaimRefusesAnUnnamedSession(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	staged := h.stageSuccessfully()

	err := h.coordinator.AuthorizeClaim(context.Background(), "")

	if got := ErrorCodeOf(err); got != ErrCancelled {
		t.Fatalf("code = %q, want %q", got, ErrCancelled)
	}
	if events := h.observer.published(); len(events) != 0 {
		t.Fatalf("published %d events for an unnamed claim, want none", len(events))
	}
	// The real session is untouched and still claimable.
	if err := h.coordinator.AuthorizeClaim(context.Background(), staged.SessionID); err != nil {
		t.Fatalf("the named session was no longer claimable: %v", err)
	}
}

func TestAuthorizeClaimTreatsABeaconStopDiagnosticAsSafe(t *testing.T) {
	h := newHarness(t)
	metadata := h.stageSuccessfully()
	h.network.stopBeacon = func() error {
		return WrapError(ErrBeaconWarning, "shutdown reported a problem", errors.New(testPath))
	}

	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		t.Fatalf("AuthorizeClaim returned %v, want the diagnostic to be non-blocking", err)
	}

	if got := h.state(); got != stateTransferring {
		t.Errorf("state is %q, want %q", got, stateTransferring)
	}
	if events := h.observer.published(); len(events) != 1 {
		t.Errorf("published %+v, want the started event", events)
	}

	entries := h.coordinator.diagnostics.snapshot()
	if len(entries) != 1 {
		t.Fatalf("diagnostics are %+v, want exactly one cleanup note", entries)
	}
	if entries[0].code != ErrBeaconWarning {
		t.Errorf("diagnostic code is %q, want %q", entries[0].code, ErrBeaconWarning)
	}
	assertSafe(t, "diagnostic", fmt.Sprintf("%s %s", entries[0].code, entries[0].message), string(h.liveSession().token))

	// The beacon is gone whatever the diagnostic said, so nothing about the
	// session may still claim one is live.
	if live := h.liveSession(); slices.Contains(live.acquired, resourceBeacon) {
		t.Error("the session still records a live beacon after StopBeacon returned")
	}
}

// TestClaimRaceResolvesOneWayOrTheOther drives the claim and the cancellation
// concurrently. The deterministic tests above force each outcome on purpose;
// this one exists so the race detector visits the interleavings in between,
// and it asserts the invariant that separates them: a started event exists if
// and only if the claim was authorized.
// TestClaimRaceResolvesOneWayOrTheOther drives the claim and a cancellation
// concurrently and checks the invariants hold whichever way each iteration
// lands: exactly one started event or none at all, a matching state, and the
// lease handed back either way.
//
// It does NOT own branch coverage, and deliberately does not require both
// outcomes to appear. Measured on this machine the race resolves in
// cancellation's favour roughly 49 times out of 50, and one run in six sees
// the claim win zero times out of fifty -- so asserting both would be a
// coin-flip failure, not a signal. The distribution is logged instead, and the
// two outcomes are each forced deterministically by
// TestAuthorizeClaimPublishesStartedBeforeACancellationCanFollow and
// TestAuthorizeClaimLosesToACancellationBeforeTheCommit.
func TestClaimRaceResolvesOneWayOrTheOther(t *testing.T) {
	const iterations = 50
	var outcomes struct {
		mu        sync.Mutex
		won, lost int
	}

	for iteration := range iterations {
		t.Run(fmt.Sprintf("iteration-%d", iteration), func(t *testing.T) {
			h := newHarness(t)
			metadata := h.stageSuccessfully()

			var claimErr error
			var waiting sync.WaitGroup
			waiting.Add(2)
			ready := make(chan struct{})

			go func() {
				defer waiting.Done()
				<-ready
				claimErr = h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID)
			}()
			go func() {
				defer waiting.Done()
				<-ready
				h.coordinator.cancelSession()
			}()
			close(ready)
			waiting.Wait()

			events := h.observer.published()
			switch {
			case claimErr == nil:
				outcomes.mu.Lock()
				outcomes.won++
				outcomes.mu.Unlock()
				if len(events) != 1 {
					t.Fatalf("the claim was authorized but published %+v, want one started event", events)
				}
				if events[0].Seq != 1 || events[0].Kind != TransferStarted {
					t.Errorf("published %+v, want started at seq 1", events[0])
				}
				if got := h.state(); got != stateTransferring {
					t.Errorf("the claim was authorized but the state is %q", got)
				}
			default:
				outcomes.mu.Lock()
				outcomes.lost++
				outcomes.mu.Unlock()
				if got := ErrorCodeOf(claimErr); got != ErrCancelled {
					t.Errorf("the claim lost with %q, want %q", got, ErrCancelled)
				}
				if len(events) != 0 {
					t.Errorf("the claim lost but published %+v, want nothing at all", events)
				}
			}
			if h.coordinator.leaseHeld() {
				t.Error("the operation lease outlived the race")
			}
		})
	}

	outcomes.mu.Lock()
	won, lost := outcomes.won, outcomes.lost
	outcomes.mu.Unlock()
	// Logged, not asserted -- see the note on this function about the skew.
	t.Logf("claim won %d, cancellation won %d, over %d iterations", won, lost, iterations)
	if won+lost != iterations {
		t.Fatalf("counted %d outcomes over %d iterations: an iteration resolved as neither",
			won+lost, iterations)
	}
}

/*
TestASlowBeaconStopDoesNotDelayTheClaimCommit states the guarantee the call
order above only implies.

The receiver's HTTP request is already open when AuthorizeClaim runs. Stopping
the beacon ahead of the commit put an unrelated adapter in front of that
request, bounded at AdapterCleanupBound -- fifteen seconds of no response and
no feedback on a contended interface, which a receiver reads as a hung link.
D-024 had already decided a beacon that never returns must not hang the claim;
this is the same reasoning for one that returns slowly.

The beacon here never returns during the test. The started event must be
published anyway, which is what a receiver waiting on the first byte actually
needs.
*/
func TestASlowBeaconStopDoesNotDelayTheClaimCommit(t *testing.T) {
	h := newHarness(t)
	metadata := h.stageSuccessfully()

	stopping := make(chan struct{})
	release := sync.OnceFunc(func() { close(stopping) })
	defer release()

	published := make(chan struct{})
	var once sync.Once
	h.observer.publish = func(event Event) {
		if event.Kind == TransferStarted {
			once.Do(func() { close(published) })
		}
	}
	h.network.stopBeacon = func() error {
		<-stopping // never returns while the claim is in flight
		return nil
	}

	claimed := make(chan error, 1)
	go func() { claimed <- h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID) }()

	select {
	case <-published:
	case <-time.After(5 * time.Second):
		t.Fatal("the started event waited on a beacon stop that never returned: the receiver's request " +
			"is held open behind an adapter whose liveness has nothing to do with the claim")
	}

	// Only now let the beacon finish, so the claim can return.
	release()
	select {
	case err := <-claimed:
		if err != nil {
			t.Fatalf("AuthorizeClaim returned %v, want the claim to commit", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("AuthorizeClaim did not return after its beacon stop completed")
	}

	if got := h.state(); got != stateTransferring {
		t.Errorf("state is %q, want %q", got, stateTransferring)
	}
}
