package transfer

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
)

// TestCancelFromEveryState walks the command table's Cancel rows. Each case
// leaves the coordinator IDLE with its lease free, and differs only in what it
// had to release and whether the UI had a session to be told about.
func TestCancelFromEveryState(t *testing.T) {
	for _, testCase := range []struct {
		name string
		// run reaches the state under test and returns Cancel's own result.
		run func(t *testing.T, h *harness) error
		// wantResets is the number of transfer-reset events Cancel publishes.
		wantResets int
		// wantEvents is every kind published across the whole case, in order.
		wantEvents  []EventKind
		wantStops   int
		wantBeacons int
	}{
		{
			name: "IDLE",
			run: func(_ *testing.T, h *harness) error {
				return h.coordinator.Cancel()
			},
			wantResets: 0,
			wantEvents: nil,
		},
		{
			name: "STAGING",
			run: func(t *testing.T, h *harness) error {
				cancelled := make(chan error, 1)
				// The QR step is an unlocked setup step, so a cancellation can
				// land between it and the revalidation that follows it.
				h.qr.encode = func(context.Context, string) ([]byte, error) {
					go func() { cancelled <- h.coordinator.Cancel() }()
					h.awaitCancelled()
					return append([]byte(nil), testPNG...), nil
				}

				_, stageErr := h.stage()
				if got := ErrorCodeOf(stageErr); got != ErrCancelled {
					t.Errorf("Stage returned %q, want %q", got, ErrCancelled)
				}
				return <-cancelled
			},
			wantResets: 0,
			// Nothing was acknowledged, so there is no UI session to terminate.
			wantEvents:  nil,
			wantStops:   1,
			wantBeacons: 0,
		},
		{
			name: "STAGED",
			run: func(_ *testing.T, h *harness) error {
				h.stageSuccessfully()
				return h.coordinator.Cancel()
			},
			wantResets:  1,
			wantEvents:  []EventKind{TransferReset},
			wantStops:   1,
			wantBeacons: 1,
		},
		{
			name: "CLAIMING",
			run: func(t *testing.T, h *harness) error {
				metadata := h.stageSuccessfully()
				cancelled := make(chan error, 1)
				// StopBeacon is the one unlocked step inside the handshake, so
				// this lands the cancellation after CLAIMING and before the
				// TRANSFERRING commit -- the state Story 1.5 could not leave.
				h.network.stopBeacon = func() error {
					go func() { cancelled <- h.coordinator.Cancel() }()
					h.awaitCancelled()
					return nil
				}

				claimErr := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID)
				if got := ErrorCodeOf(claimErr); got != ErrCancelled {
					t.Errorf("the claim returned %q, want %q", got, ErrCancelled)
				}
				return <-cancelled
			},
			wantResets:  1,
			wantEvents:  []EventKind{TransferReset},
			wantStops:   1,
			wantBeacons: 1,
		},
		{
			name: "TRANSFERRING",
			run: func(_ *testing.T, h *harness) error {
				metadata := h.transferring()
				h.emit(progressEvent(metadata.SessionID, testProgress(1024, 25)))
				h.awaitEvents(2)
				return h.coordinator.Cancel()
			},
			wantResets:  1,
			wantEvents:  []EventKind{TransferStarted, TransferProgress, TransferReset},
			wantStops:   1,
			wantBeacons: 1,
		},
		{
			name: "DONE",
			run: func(_ *testing.T, h *harness) error {
				metadata := h.transferring()
				h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
				h.awaitDrainer()
				return h.coordinator.Cancel()
			},
			wantResets: 1,
			wantEvents: []EventKind{
				TransferStarted, TransferProgress, TransferComplete, TransferReset,
			},
			wantStops:   1,
			wantBeacons: 1,
		},
		{
			name: "ERROR",
			run: func(_ *testing.T, h *harness) error {
				metadata := h.transferring()
				h.emit(failedEvent(metadata.SessionID, nil, NewError(ErrTransferFailed, "the stream broke")))
				h.awaitDrainer()
				return h.coordinator.Cancel()
			},
			wantResets:  1,
			wantEvents:  []EventKind{TransferStarted, TransferError, TransferReset},
			wantStops:   1,
			wantBeacons: 1,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)

			if err := testCase.run(t, h); err != nil {
				t.Fatalf("Cancel returned %v, want success once the state is quiescent", err)
			}

			if got := h.state(); got != stateIdle {
				t.Errorf("state is %q, want %q", got, stateIdle)
			}
			if h.liveSession() != nil {
				t.Error("Cancel returned with a session still installed")
			}
			if h.coordinator.leaseHeld() {
				t.Error("Cancel returned while the operation lease was still held")
			}

			events := h.observer.published()
			kinds := make([]EventKind, len(events))
			resets := 0
			for index, event := range events {
				kinds[index] = event.Kind
				if event.Kind == TransferReset {
					resets++
				}
			}
			if !slices.Equal(kinds, testCase.wantEvents) {
				t.Errorf("published %v, want %v", kinds, testCase.wantEvents)
			}
			if resets != testCase.wantResets {
				t.Errorf("%d resets published, want %d", resets, testCase.wantResets)
			}
			if len(events) > 0 {
				assertEventGrammar(t, events[0].SessionID, events)
			}
			if got := h.calls.count("server.Stop"); got != testCase.wantStops {
				t.Errorf("server.Stop ran %d times, want %d -- every resource is released exactly once", got, testCase.wantStops)
			}
			if got := h.calls.count("network.StopBeacon"); got != testCase.wantBeacons {
				t.Errorf("network.StopBeacon ran %d times, want %d", got, testCase.wantBeacons)
			}
		})
	}
}

// Cancel after the TRANSFERRING commit owns the outcome, so a server report
// that races it is discarded rather than published: no complete, no error.
func TestCancelDiscardsAnOutcomeThatRacesIt(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	// The marker is what a racing outcome loses to, so it is set first and the
	// outcome is then provably processed: the second emit can only be received
	// once the first has been handled.
	h.coordinator.cancelSession()
	h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
	h.emit(progressEvent(metadata.SessionID, testProgress(2048, 50)))

	if err := h.coordinator.Cancel(); err != nil {
		t.Fatalf("Cancel returned %v", err)
	}

	events := h.observer.published()
	kinds := []EventKind{TransferStarted, TransferReset}
	got := make([]EventKind, len(events))
	for index, event := range events {
		got[index] = event.Kind
	}
	if !slices.Equal(got, kinds) {
		t.Errorf("published %v, want %v -- a cancelled transfer reports no outcome", got, kinds)
	}
	assertEventGrammar(t, metadata.SessionID, events)
	if h.timer.armed() != 0 {
		t.Errorf("%d resets are armed, want none: the discarded outcome must not hold a terminal lease", h.timer.armed())
	}
}

// Cancel from a terminal state stops the armed reset and publishes its own, so
// the two paths cannot both announce IDLE.
func TestCancelStopsTheArmedReset(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()
	h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
	h.awaitDrainer()
	if h.timer.armed() != 1 {
		t.Fatalf("%d resets are armed before Cancel, want one", h.timer.armed())
	}

	if err := h.coordinator.Cancel(); err != nil {
		t.Fatalf("Cancel returned %v", err)
	}

	if h.timer.stops() != 1 {
		t.Errorf("%d armed resets were stopped, want one", h.timer.stops())
	}
	before := len(h.observer.published())

	// A timer that lost the stop race still runs its callback; it must decide
	// for itself that it lost.
	h.timer.fire()

	if after := len(h.observer.published()); after != before {
		t.Errorf("the stale timer published %d extra events, want none", after-before)
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
}

func TestResetTimerClearsTheSessionAndReturnsToIdle(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()
	h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
	h.awaitDrainer()
	live := h.liveSession()
	if live == nil {
		t.Fatal("the terminal outcome cleared the session before its reset")
	}

	h.timer.fire()

	events := h.observer.published()
	if len(events) != 4 {
		t.Fatalf("published %+v, want started, progress, complete and reset", events)
	}
	assertEventGrammar(t, metadata.SessionID, events)
	if events[3].Kind != TransferReset || events[3].Seq != 4 {
		t.Errorf("event 3 is %+v, want the reset at seq 4", events[3])
	}
	if !h.observer.leaseHeldAt(3) {
		t.Error("the reset was published without the operation lease")
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
	if h.liveSession() != nil {
		t.Error("the reset left the session installed")
	}
	if h.coordinator.leaseHeld() {
		t.Error("the reset kept the operation lease")
	}
	if live.ctx.Err() == nil {
		t.Error("the reset left the session context uncancelled")
	}

	// IDLE means IDLE: a new Stage is accepted immediately.
	if _, err := h.stage(); err != nil {
		t.Errorf("Stage after the reset returned %v, want a fresh session", err)
	}
}

func TestAStaleResetTimerPublishesNothingAndMutatesNothing(t *testing.T) {
	for _, testCase := range []struct {
		name string
		// stale reaches a state where the armed timer no longer owns anything,
		// and returns the number of events published up to that point.
		stale func(t *testing.T, h *harness) int
	}{
		{
			name: "the session was already cleared",
			stale: func(t *testing.T, h *harness) int {
				metadata := h.transferring()
				h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
				h.awaitDrainer()
				if err := h.coordinator.Cancel(); err != nil {
					t.Fatalf("Cancel returned %v", err)
				}
				return len(h.observer.published())
			},
		},
		{
			name: "the session was replaced",
			stale: func(t *testing.T, h *harness) int {
				metadata := h.transferring()
				h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
				h.awaitDrainer()
				if err := h.coordinator.Cancel(); err != nil {
					t.Fatalf("Cancel returned %v", err)
				}
				// A second session takes the coordinator back out of IDLE, so
				// the pending timer now names a generation that is gone.
				h.server.events = make(chan ServerEvent)
				h.server.closed = false
				if _, err := h.stage(); err != nil {
					t.Fatalf("the replacement Stage returned %v", err)
				}
				return len(h.observer.published())
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)

			before := testCase.stale(t, h)
			stateBefore := h.state()
			sessionBefore := h.liveSession()

			h.timer.fire()

			if after := len(h.observer.published()); after != before {
				t.Errorf("the stale timer published %d extra events, want none", after-before)
			}
			if got := h.state(); got != stateBefore {
				t.Errorf("the stale timer moved the state to %q from %q", got, stateBefore)
			}
			if got := h.liveSession(); got != sessionBefore {
				t.Error("the stale timer replaced the live session")
			}
			if h.coordinator.leaseHeld() {
				t.Error("the stale timer kept the operation lease")
			}
		})
	}
}

// The timer and Cancel can both reach the terminal transition. Exactly one
// reset may be published whichever wins, and the loser must leave no trace.
func TestTheResetTimerAndCancelProduceExactlyOneReset(t *testing.T) {
	const iterations = 40
	var outcomes struct {
		mu                sync.Mutex
		timerWon, cancels int
	}

	for iteration := range iterations {
		t.Run(fmt.Sprintf("iteration-%d", iteration), func(t *testing.T) {
			h := newHarness(t)
			metadata := h.transferring()
			h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
			h.awaitDrainer()

			var running sync.WaitGroup
			running.Add(2)
			ready := make(chan struct{})
			var cancelErr error

			go func() {
				defer running.Done()
				<-ready
				h.timer.fireIfArmed()
			}()
			go func() {
				defer running.Done()
				<-ready
				cancelErr = h.coordinator.Cancel()
			}()
			close(ready)
			running.Wait()

			if cancelErr != nil {
				t.Fatalf("Cancel returned %v", cancelErr)
			}
			events := h.observer.published()
			assertEventGrammar(t, metadata.SessionID, events)
			resets := 0
			for _, event := range events {
				if event.Kind == TransferReset {
					resets++
				}
			}
			if resets != 1 {
				t.Fatalf("%d resets published, want exactly one: %+v", resets, events)
			}
			if got := h.state(); got != stateIdle {
				t.Errorf("state is %q, want %q", got, stateIdle)
			}
			if h.liveSession() != nil {
				t.Error("a session outlived the race")
			}
			if h.coordinator.leaseHeld() {
				t.Error("the operation lease outlived the race")
			}

			outcomes.mu.Lock()
			if h.timer.stops() == 0 {
				outcomes.timerWon++
			} else {
				outcomes.cancels++
			}
			outcomes.mu.Unlock()
		})
	}

	outcomes.mu.Lock()
	timerWon, cancels := outcomes.timerWon, outcomes.cancels
	outcomes.mu.Unlock()
	// Logged rather than asserted: which side wins is a scheduling accident,
	// and requiring both would be a coin flip. Each outcome is forced
	// deterministically by TestCancelStopsTheArmedReset and
	// TestResetTimerClearsTheSessionAndReturnsToIdle.
	t.Logf("the timer outran Cancel %d times, Cancel stopped it %d times, over %d iterations",
		timerWon, cancels, iterations)
}

func TestShutdownQuiescesEverythingAndPublishesNothing(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()
	h.emit(progressEvent(metadata.SessionID, testProgress(1024, 25)))
	h.awaitEvents(2)
	live := h.liveSession()
	before := len(h.observer.published())

	if err := h.coordinator.Shutdown(); err != nil {
		t.Fatalf("Shutdown returned %v", err)
	}

	if after := len(h.observer.published()); after != before {
		t.Errorf("Shutdown published %d events, want none -- a closing application has no UI to tell", after-before)
	}
	if got := h.calls.count("server.Stop"); got != 1 {
		t.Errorf("server.Stop ran %d times, want once", got)
	}
	if h.liveSession() != nil {
		t.Error("Shutdown returned with a session still installed")
	}
	if h.coordinator.leaseHeld() {
		t.Error("Shutdown returned while the operation lease was still held")
	}
	if live.ctx.Err() == nil {
		t.Error("Shutdown left the session context uncancelled")
	}
	select {
	case <-live.drainerDone:
	default:
		t.Error("Shutdown returned while the drainer was still running")
	}
}

func TestShutdownStopsAnArmedResetAndTheStaleTimerStaysSilent(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()
	h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
	h.awaitDrainer()
	before := len(h.observer.published())

	if err := h.coordinator.Shutdown(); err != nil {
		t.Fatalf("Shutdown returned %v", err)
	}
	if h.timer.stops() != 1 {
		t.Errorf("%d armed resets were stopped, want one", h.timer.stops())
	}

	h.timer.fire()

	if after := len(h.observer.published()); after != before {
		t.Errorf("a reset was published after Shutdown: %+v", h.observer.published())
	}
}

func TestCommandsAfterShutdownAreRefused(t *testing.T) {
	h := newHarness(t)
	h.stageSuccessfully()
	if err := h.coordinator.Shutdown(); err != nil {
		t.Fatalf("Shutdown returned %v", err)
	}
	callsBefore := portCalls(h)

	if _, err := h.stage(); ErrorCodeOf(err) != ErrShuttingDown {
		t.Errorf("Stage returned %q, want %q", ErrorCodeOf(err), ErrShuttingDown)
	}
	if err := h.coordinator.Cancel(); ErrorCodeOf(err) != ErrShuttingDown {
		t.Errorf("Cancel returned %q, want %q", ErrorCodeOf(err), ErrShuttingDown)
	}
	if err := h.coordinator.AuthorizeClaim(context.Background(), testSessionID); ErrorCodeOf(err) != ErrShuttingDown {
		t.Errorf("AuthorizeClaim returned %q, want %q", ErrorCodeOf(err), ErrShuttingDown)
	}

	if got := portCalls(h); !slices.Equal(got, callsBefore) {
		t.Errorf("a refused command touched an adapter: %v", got[len(callsBefore):])
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
	if h.coordinator.leaseHeld() {
		t.Error("a refused command took the operation lease")
	}
}

func TestShutdownIsIdempotent(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()
	h.emit(progressEvent(metadata.SessionID, testProgress(1024, 25)))
	h.awaitEvents(2)

	if err := h.coordinator.Shutdown(); err != nil {
		t.Fatalf("the first Shutdown returned %v", err)
	}
	after := h.calls.snapshot()
	events := len(h.observer.published())

	if err := h.coordinator.Shutdown(); err != nil {
		t.Fatalf("the second Shutdown returned %v", err)
	}

	if got := h.calls.snapshot(); !slices.Equal(got, after) {
		t.Errorf("the second Shutdown ran %v, want no second teardown", got[len(after):])
	}
	if got := len(h.observer.published()); got != events {
		t.Errorf("the second Shutdown published %d events, want none", got-events)
	}
	if h.coordinator.leaseHeld() {
		t.Error("the second Shutdown kept the operation lease")
	}
}

func TestShutdownFromIdleIsSafe(t *testing.T) {
	h := newHarness(t)

	if err := h.coordinator.Shutdown(); err != nil {
		t.Fatalf("Shutdown returned %v", err)
	}

	if calls := h.calls.snapshot(); len(calls) != 0 {
		t.Errorf("Shutdown from IDLE called %v, want no adapter at all", calls)
	}
	if events := h.observer.published(); len(events) != 0 {
		t.Errorf("Shutdown from IDLE published %+v, want nothing", events)
	}
	if h.coordinator.leaseHeld() {
		t.Error("Shutdown from IDLE kept the operation lease")
	}
}

// TestLifecycleContentionHoldsItsInvariants drives the drainer, a Cancel and
// the reset timer at one another so the race detector visits the interleavings
// between the deterministic tests above. It asserts only what must hold in
// every one of them.
func TestLifecycleContentionHoldsItsInvariants(t *testing.T) {
	const iterations = 30

	for iteration := range iterations {
		t.Run(fmt.Sprintf("iteration-%d", iteration), func(t *testing.T) {
			h := newHarness(t)
			h.bufferLane(8)
			metadata := h.transferring()

			var running sync.WaitGroup
			running.Add(3)
			ready := make(chan struct{})
			var cancelErr error

			go func() {
				defer running.Done()
				<-ready
				// Non-blocking, and never sends on a closed lane, exactly like
				// the real server's producer racing its own Stop.
				h.server.publish(progressEvent(metadata.SessionID, testProgress(1024, 25)))
				h.server.publish(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
			}()
			go func() {
				defer running.Done()
				<-ready
				cancelErr = h.coordinator.Cancel()
			}()
			go func() {
				defer running.Done()
				<-ready
				h.timer.fireIfArmed()
			}()
			close(ready)
			running.Wait()

			if cancelErr != nil {
				t.Fatalf("Cancel returned %v", cancelErr)
			}
			events := h.observer.published()
			assertEventGrammar(t, metadata.SessionID, events)
			resets := 0
			for _, event := range events {
				if event.Kind == TransferReset {
					resets++
				}
			}
			if resets != 1 {
				t.Errorf("%d resets published, want exactly one: %+v", resets, events)
			}
			if got := h.state(); got != stateIdle {
				t.Errorf("state is %q, want %q", got, stateIdle)
			}
			if h.liveSession() != nil {
				t.Error("a session outlived the contention")
			}
			if h.coordinator.leaseHeld() {
				t.Error("the operation lease outlived the contention")
			}
		})
	}
}

// portCalls is the call log without the entropy draws. Stage draws its
// identity before it looks at any state, so a refused Stage still costs two
// reads -- but a draw acquires nothing, changes nothing, and is discarded with
// the attempt. Every other entry names a port that owns something.
func portCalls(h *harness) []string {
	var out []string
	for _, call := range h.calls.snapshot() {
		if call != "entropy.Read" {
			out = append(out, call)
		}
	}
	return out
}
