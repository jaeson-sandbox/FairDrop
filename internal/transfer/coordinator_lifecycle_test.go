package transfer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
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
				return h.coordinator.Cancel(context.Background())
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
					go func() { cancelled <- h.coordinator.Cancel(context.Background()) }()
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
				return h.coordinator.Cancel(context.Background())
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
					go func() { cancelled <- h.coordinator.Cancel(context.Background()) }()
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
				return h.coordinator.Cancel(context.Background())
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
				return h.coordinator.Cancel(context.Background())
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
				return h.coordinator.Cancel(context.Background())
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
				// The expected id comes from the staged metadata, never from the
				// stream under test: taking it from events[0] would make the
				// "every event belongs to this session" clause unable to fail.
				assertEventGrammar(t, testSessionID, events)
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

	if err := h.coordinator.Cancel(context.Background()); err != nil {
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

	if err := h.coordinator.Cancel(context.Background()); err != nil {
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
				if err := h.coordinator.Cancel(context.Background()); err != nil {
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
				if err := h.coordinator.Cancel(context.Background()); err != nil {
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
				cancelErr = h.coordinator.Cancel(context.Background())
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

	if err := h.coordinator.Shutdown(context.Background()); err != nil {
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

	if err := h.coordinator.Shutdown(context.Background()); err != nil {
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
	if err := h.coordinator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned %v", err)
	}
	callsBefore := portCalls(h)

	if _, err := h.stage(); ErrorCodeOf(err) != ErrShuttingDown {
		t.Errorf("Stage returned %q, want %q", ErrorCodeOf(err), ErrShuttingDown)
	}
	if err := h.coordinator.Cancel(context.Background()); ErrorCodeOf(err) != ErrShuttingDown {
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

	if err := h.coordinator.Shutdown(context.Background()); err != nil {
		t.Fatalf("the first Shutdown returned %v", err)
	}
	after := h.calls.snapshot()
	events := len(h.observer.published())

	if err := h.coordinator.Shutdown(context.Background()); err != nil {
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

	if err := h.coordinator.Shutdown(context.Background()); err != nil {
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
	// Subtests here are sequential, so a plain counter needs no mutex. It
	// exists because publish is non-blocking and returns false on a lane the
	// teardown already closed: without counting, every iteration could have
	// raced a teardown that won before the outcome was ever offered, and the
	// invariants below would hold over a contention that never happened.
	delivered := 0

	for iteration := range iterations {
		t.Run(fmt.Sprintf("iteration-%d", iteration), func(t *testing.T) {
			h := newHarness(t)
			h.bufferLane(8)
			metadata := h.transferring()

			var running sync.WaitGroup
			running.Add(3)
			ready := make(chan struct{})
			var cancelErr error
			var outcomeLanded bool

			go func() {
				defer running.Done()
				<-ready
				// Non-blocking, and never sends on a closed lane, exactly like
				// the real server's producer racing its own Stop.
				h.server.publish(progressEvent(metadata.SessionID, testProgress(1024, 25)))
				outcomeLanded = h.server.publish(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
			}()
			go func() {
				defer running.Done()
				<-ready
				cancelErr = h.coordinator.Cancel(context.Background())
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
			if outcomeLanded {
				delivered++
			}
		})
	}

	// Which side wins is a scheduling accident and is not asserted, in the same
	// way TestTheResetTimerAndCancelProduceExactlyOneReset logs its split. What
	// is asserted is that the race happened at all in some iteration.
	t.Logf("the outcome reached the lane in %d of %d iterations", delivered, iterations)
	if delivered == 0 {
		t.Errorf("no iteration ever got the outcome onto the lane, so nothing above raced a live outcome")
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

// adapterCalls is the call log without the entropy draws or the bound-timer
// seam. A wait that finds the lease already held arms and immediately stops
// its own bound as a genuine part of waiting, not because it reached any
// port -- so it is excluded here the same way portCalls excludes
// entropy.Read.
func adapterCalls(h *harness) []string {
	var out []string
	for _, call := range h.calls.snapshot() {
		if call != "entropy.Read" && call != "timer.AfterFunc" {
			out = append(out, call)
		}
	}
	return out
}

// TestShutdownContendsWithEveryOtherActor adds Shutdown to the contention: the
// drainer, a Cancel, a Shutdown and the reset timer all reach for one session
// at once. The invariants weaken by exactly one -- a suppressed reset is a
// legitimate outcome once Shutdown wins -- and everything else must still hold.
func TestShutdownContendsWithEveryOtherActor(t *testing.T) {
	const iterations = 30
	// Counted for the same reason as TestLifecycleContentionHoldsItsInvariants:
	// a discarded publish result cannot distinguish "Shutdown beat the outcome"
	// from "the outcome was never offered", and only the first is a contention.
	delivered, cancelsWon := 0, 0

	for iteration := range iterations {
		t.Run(fmt.Sprintf("iteration-%d", iteration), func(t *testing.T) {
			h := newHarness(t)
			h.bufferLane(8)
			metadata := h.transferring()

			var running sync.WaitGroup
			running.Add(4)
			ready := make(chan struct{})
			var cancelErr, shutdownErr error
			var outcomeLanded bool

			go func() {
				defer running.Done()
				<-ready
				h.server.publish(progressEvent(metadata.SessionID, testProgress(1024, 25)))
				outcomeLanded = h.server.publish(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
			}()
			go func() {
				defer running.Done()
				<-ready
				cancelErr = h.coordinator.Cancel(context.Background())
			}()
			go func() {
				defer running.Done()
				<-ready
				shutdownErr = h.coordinator.Shutdown(context.Background())
			}()
			go func() {
				defer running.Done()
				<-ready
				h.timer.fireIfArmed()
			}()
			close(ready)
			running.Wait()

			if shutdownErr != nil {
				t.Fatalf("Shutdown returned %v", shutdownErr)
			}
			// Cancel either won its own race or was refused by the Shutdown
			// that beat it. There is no third answer.
			if code := ErrorCodeOf(cancelErr); cancelErr != nil && code != ErrShuttingDown {
				t.Fatalf("Cancel returned %q, want success or %q", code, ErrShuttingDown)
			}

			events := h.observer.published()
			assertEventGrammar(t, metadata.SessionID, events)
			resets := 0
			for _, event := range events {
				if event.Kind == TransferReset {
					resets++
				}
			}
			if resets > 1 {
				t.Errorf("%d resets published, want at most one: %+v", resets, events)
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
			if _, err := h.stage(); ErrorCodeOf(err) != ErrShuttingDown {
				t.Errorf("Stage after the contention returned %q, want %q", ErrorCodeOf(err), ErrShuttingDown)
			}
			if outcomeLanded {
				delivered++
			}
			if cancelErr == nil {
				cancelsWon++
			}
		})
	}

	t.Logf("the outcome reached the lane in %d of %d iterations; Cancel linearized first in %d",
		delivered, iterations, cancelsWon)
	if delivered == 0 {
		t.Errorf("no iteration ever got the outcome onto the lane, so Shutdown never actually contended with one")
	}
}

// TestTheDefaultResetSchedulerIsARealTimer drives the AfterFunc that
// NewCoordinator installs when a caller supplies none -- which is how main.go
// will build the coordinator in Story 1.7.
//
// Every other test in this package injects the seam, so without this the
// production default is never executed once: a default that returned a stop
// function and scheduled nothing would leave every terminal session parked in
// DONE forever, and the whole suite would still be green.
func TestTheDefaultResetSchedulerIsARealTimer(t *testing.T) {
	coordinator := NewCoordinator(Dependencies{})
	if coordinator.afterFunc == nil {
		t.Fatal("NewCoordinator installed no reset scheduler")
	}

	ran := make(chan struct{})
	stop := coordinator.afterFunc(20*time.Millisecond, func() { close(ran) })
	if stop == nil {
		t.Fatal("the default scheduler returned no stop function")
	}
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("the default scheduler never ran its callback")
	}
	if stop() {
		t.Error("stopping an already-fired timer reported that it prevented the run")
	}

	// The same default must be able to prevent a run, which is what Cancel and
	// Shutdown rely on to stop an armed reset.
	unwanted := make(chan struct{})
	stopEarly := coordinator.afterFunc(time.Hour, func() { close(unwanted) })
	if !stopEarly() {
		t.Error("stopping a pending timer reported that it did not prevent the run")
	}
	select {
	case <-unwanted:
		t.Error("a stopped timer ran its callback anyway")
	case <-time.After(50 * time.Millisecond):
	}
}

// TestShutdownWaitsForTheLeaseWithNothingStaged pins the claim Shutdown makes
// with no session at all: the lease being free is the proof that no setup or
// teardown is still running.
//
// It matters because retire clears the session several lines before it hands
// the lease back. A Shutdown that skipped the wait would return during that
// window -- reporting that everything is gone while a reset was still being
// published.
func TestShutdownWaitsForTheLeaseWithNothingStaged(t *testing.T) {
	h := newHarness(t)

	// Nothing staged, and somebody else owns the lease.
	<-h.coordinator.lease
	if h.liveSession() != nil {
		t.Fatal("no session should exist")
	}

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		if err := h.coordinator.Shutdown(context.Background()); err != nil {
			t.Errorf("Shutdown returned %v, want nil", err)
		}
	}()

	select {
	case <-returned:
		t.Fatal("Shutdown returned while the operation lease was still held")
	case <-time.After(50 * time.Millisecond):
	}

	h.coordinator.lease <- struct{}{}
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown never returned after the lease was released")
	}
	if h.coordinator.leaseHeld() {
		t.Error("the lease is still held after Shutdown returned")
	}
}

// TestCancelInterruptsASetupStepRatherThanWaitingForIt proves Cancel cancels
// the session's data-plane context, not merely its generation marker.
//
// The marker alone is enough for the fakes, which all return immediately, so
// this is the one shape that tells the two apart: a setup step that will not
// finish until its context is cancelled. In production that step is an mDNS
// registration or a listener bind, and a Cancel that only marked the generation
// would block until the adapter finished on its own.
func TestCancelInterruptsASetupStepRatherThanWaitingForIt(t *testing.T) {
	h := newHarness(t)
	released := h.blockQRUntilCancelled()

	staged := make(chan error, 1)
	go func() {
		_, err := h.stage()
		staged <- err
	}()

	// Wait until Stage is parked inside the QR step, so Cancel cannot win by
	// arriving before the step it has to interrupt.
	deadline := time.Now().Add(mutexProbeTimeout)
	for h.calls.count("qr.EncodePNG") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("Stage never reached the capability code step")
		}
		time.Sleep(200 * time.Microsecond)
	}

	cancelled := make(chan error, 1)
	go func() { cancelled <- h.coordinator.Cancel(context.Background()) }()

	select {
	case err := <-released:
		if err == nil {
			t.Fatal("the setup step was never cancelled; Cancel only marked the generation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel never cancelled the staging step's context")
	}

	select {
	case err := <-cancelled:
		if err != nil {
			t.Errorf("Cancel returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel never returned")
	}
	select {
	case err := <-staged:
		if got := ErrorCodeOf(err); got != ErrCancelled {
			t.Errorf("Stage returned %q, want %q", got, ErrCancelled)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stage never returned")
	}

	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
	if h.coordinator.leaseHeld() {
		t.Error("the lease is still held after Cancel returned")
	}
	if events := h.observer.published(); len(events) != 0 {
		t.Errorf("published %+v, want no lifecycle event before a STAGED acknowledgement", events)
	}
}

// The reset timer's contract row names DONE *and* ERROR. Every timer-firing
// test in the suite reached DONE, so narrowing fireReset to stateDone alone
// left the suite green while a failed transfer parked in ERROR forever and
// every later Stage was refused as busy.
func TestTheResetTimerFiresFromEitherTerminalState(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		outcome func(id SessionID) ServerEvent
		want    EventKind
	}{
		{"after a completed transfer", func(id SessionID) ServerEvent {
			return completeEvent(id, testProgress(testSize, 100))
		}, TransferComplete},
		{"after a failed transfer", func(id SessionID) ServerEvent {
			return failedEvent(id, nil, NewError(ErrTransferFailed, "the stream broke"))
		}, TransferError},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			metadata := h.transferring()
			h.emit(testCase.outcome(metadata.SessionID))
			h.awaitDrainer()

			live := h.liveSession()
			if live == nil {
				t.Fatal("the terminal outcome cleared the session before its reset")
			}
			// The reset must not be announced while a goroutine of the finished
			// session is still running.
			h.observer.publish = func(event Event) {
				if event.Kind != TransferReset {
					return
				}
				select {
				case <-live.drainerDone:
				default:
					t.Error("the reset was published while the session's drainer was still running")
				}
			}
			if h.timer.armed() != 1 {
				t.Fatalf("%d resets are armed, want exactly one", h.timer.armed())
			}

			h.timer.fire()

			events := h.observer.published()
			assertEventGrammar(t, metadata.SessionID, events)
			last := events[len(events)-1]
			if last.Kind != TransferReset {
				t.Errorf("the last event is %q, want %q", last.Kind, TransferReset)
			}
			if !slices.Contains(kindsOf(events), testCase.want) {
				t.Errorf("published %v, want it to contain %q", kindsOf(events), testCase.want)
			}
			if got := h.state(); got != stateIdle {
				t.Errorf("state is %q, want %q", got, stateIdle)
			}
			if h.liveSession() != nil {
				t.Error("the reset left the session installed")
			}
			if live.ctx.Err() == nil {
				t.Error("the reset left the session context uncancelled")
			}
			if h.coordinator.leaseHeld() {
				t.Error("the reset kept the operation lease")
			}
		})
	}
}

// Shutdown declares the UI gone. An outcome that took the lease just before
// that must not publish into it -- Shutdown is waiting for exactly that lease,
// so without a re-check the event lands after the flag is up.
func TestAnOutcomeInFlightPublishesNothingOnceShutdownBegins(t *testing.T) {
	h := newHarness(t)
	h.bufferLane(4)
	metadata := h.transferring()

	// Raise the flag from inside the teardown the outcome runs, which is the
	// window: the outcome already owns the lease, and Shutdown is blocked on it.
	shutdown := make(chan error, 1)
	h.server.stop = func() error {
		go func() { shutdown <- h.coordinator.Shutdown(context.Background()) }()
		h.awaitClosing()
		return nil
	}

	if !h.server.publish(completeEvent(metadata.SessionID, testProgress(testSize, 100))) {
		t.Fatal("the complete event was not queued")
	}
	h.awaitDrainer()

	select {
	case err := <-shutdown:
		if err != nil {
			t.Errorf("Shutdown returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown never returned")
	}

	events := h.observer.published()
	kinds := kindsOf(events)
	if !slices.Equal(kinds, []EventKind{TransferStarted}) {
		t.Errorf("published %v, want only the started event -- Shutdown suppresses the rest", kinds)
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
	if h.timer.armed() != 0 {
		t.Errorf("%d resets are armed, want none once Shutdown has begun", h.timer.armed())
	}
}

// Cancel promises success even when a cleanup step reports a diagnostic: every
// Stop is quiescent on return, so a completed cancellation is not a failed
// command.
func TestCancelSucceedsThroughACleanupDiagnostic(t *testing.T) {
	h := newHarness(t)
	h.stageSuccessfully()
	before := len(h.coordinator.diagnostics.snapshot())

	h.server.stop = func() error {
		return WrapError(ErrTransferFailed, "cleanup reported a problem", errors.New("boom"))
	}
	h.network.stopBeacon = func() error {
		return WrapError(ErrBeaconWarning, "the advertisement lingered", errors.New("boom"))
	}

	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatalf("Cancel returned %v, want success once the state is quiescent", err)
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
	if got := len(h.coordinator.diagnostics.snapshot()); got <= before {
		t.Errorf("%d diagnostics recorded, want more than the %d before", got, before)
	}
	events := h.observer.published()
	if !slices.Equal(kindsOf(events), []EventKind{TransferReset}) {
		t.Errorf("published %v, want exactly one reset", kindsOf(events))
	}
}

// Neither command may panic where Stage and AuthorizeClaim answer with a code.
func TestLifecycleCommandsSurviveAMissingCoordinator(t *testing.T) {
	var absent *Coordinator
	if got := ErrorCodeOf(absent.Cancel(context.Background())); got != ErrTransferFailed {
		t.Errorf("Cancel on a nil coordinator returned %q, want %q", got, ErrTransferFailed)
	}
	if err := absent.Shutdown(context.Background()); err != nil {
		t.Errorf("Shutdown on a nil coordinator returned %v, want nil -- nothing is running", err)
	}
}

// The mirror of TestShutdownWaitsForTheLeaseWithNothingStaged. retire and
// fireReset both clear the session several lines before they hand the lease
// back, so a Cancel that read "no session" and returned would report IDLE while
// a reset was still being published.
func TestCancelFromIdleWaitsForAFinishingTeardown(t *testing.T) {
	h := newHarness(t)

	<-h.coordinator.lease
	if h.liveSession() != nil {
		t.Fatal("no session should exist")
	}

	returned := make(chan error, 1)
	go func() { returned <- h.coordinator.Cancel(context.Background()) }()

	select {
	case <-returned:
		t.Fatal("Cancel returned while the operation lease was still held")
	case <-time.After(50 * time.Millisecond):
	}

	h.coordinator.lease <- struct{}{}
	select {
	case err := <-returned:
		if err != nil {
			t.Errorf("Cancel returned %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel never returned after the lease was released")
	}
	if h.coordinator.leaseHeld() {
		t.Error("the lease is still held after Cancel returned")
	}
	// Cancel found the lease already held, so awaitLeaseBounded took its slow
	// path and armed its own bound -- a legitimate part of the wait itself,
	// not an adapter call, and excluded here the same way portCalls excludes
	// entropy.Read.
	if got := adapterCalls(h); len(got) != 0 {
		t.Errorf("Cancel from IDLE called %v, want no adapter at all", got)
	}
	if h.bounds.armed() != 1 || h.bounds.stops() != 1 {
		t.Errorf("the lease wait armed %d bound(s) and stopped %d, want exactly one armed and stopped",
			h.bounds.armed(), h.bounds.stops())
	}
	if events := h.observer.published(); len(events) != 0 {
		t.Errorf("Cancel from IDLE published %+v, want nothing", events)
	}
}

// A scheduler that schedules without handing back a way to stop leaves the
// reset uncancellable. The coordinator must record that and carry on, not call
// a nil function on the drainer goroutine, where nothing recovers.
func TestASchedulerThatWithholdsItsStopFunctionDoesNotKillTheDrainer(t *testing.T) {
	h := newHarness(t)
	h.timer.withoutStop = true
	metadata := h.transferring()
	before := len(h.coordinator.diagnostics.snapshot())

	// No racing Cancel here, deliberately. withoutStop already guarantees
	// afterFunc hands back nil, so armReset reaches the nil-stop branch on its
	// own; adding a Cancel could only retire the session first and skip the
	// branch entirely. This assertion used to be raced against that Cancel and
	// failed roughly one run in a hundred, reporting "0 diagnostics recorded"
	// -- a real outcome of the race, not a defect in the code under test.
	h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
	h.awaitDrainer()

	if got := len(h.coordinator.diagnostics.snapshot()); got <= before {
		t.Errorf("%d diagnostics recorded, want more than the %d before", got, before)
	}
	// The drainer survived: it closed its own done channel rather than dying
	// mid-callback, which awaitDrainer above already proved by returning.
	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Errorf("Cancel after the withheld stop returned %v, want nil", err)
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
}

/*
A Cancel landing while the reset is being armed is survivable either way.

This is the race the test above used to carry. Both orderings are correct:
Cancel may retire the session before armReset checks whether a reset is due,
in which case no timer is armed and no diagnostic is recorded, or armReset
may get there first and record the withheld-stop diagnostic. Asserting the
diagnostic here would be asserting which way a race resolved. What must hold
under both is that the drainer survives, Cancel completes, and the session
lands idle.
*/
func TestACancelRacingTheResetArmingLeavesTheCoordinatorIdle(t *testing.T) {
	h := newHarness(t)
	h.timer.withoutStop = true
	metadata := h.transferring()

	h.observer.publish = func(event Event) {
		if event.Kind == TransferComplete {
			go h.coordinator.Cancel(context.Background())
		}
	}
	h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
	h.awaitDrainer()

	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Errorf("Cancel after the withheld stop returned %v, want nil", err)
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
}

// --- Story 3.4: bounded waits and quiescence -------------------------------

// TestCancelReportsACodedFailureWhenTheDrainerNeverEnds is D-027 and D-032:
// ServerPort.Stop is documented to close its event channel on every return,
// but this drives the one case that violates it -- Stop returns and the lane
// stays open -- and proves the drainer join no longer waits on that promise
// forever. The bound is forced deterministically through the bounds seam,
// never by sleeping.
func TestCancelReportsACodedFailureWhenTheDrainerNeverEnds(t *testing.T) {
	h := newHarness(t)
	h.transferring()
	h.server.keepLaneOpenOnStop = true

	cancelDone := make(chan error, 1)
	go func() { cancelDone <- h.coordinator.Cancel(context.Background()) }()

	// Two adapter calls precede the join (StopBeacon, then Stop) and settle
	// almost instantly on this fake; the drainer join is the one that stays
	// pending, which is exactly what awaitBoundsPending waits for.
	h.awaitBoundsPending()
	h.bounds.fire()

	select {
	case err := <-cancelDone:
		if err == nil {
			t.Fatal("Cancel succeeded, want a coded failure: the drainer never ended")
		}
		if code := ErrorCodeOf(err); code != ErrTransferFailed {
			t.Errorf("Cancel error code = %q, want %q", code, ErrTransferFailed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel never returned")
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q -- the coordinator must reach a usable state despite the stuck drainer", got, stateIdle)
	}
	if h.coordinator.leaseHeld() {
		t.Error("the lease is still held after Cancel returned")
	}

	// The lane was never actually closed by the fake Stop above; close it now
	// so the leaked drainer goroutine finishes and the harness's own cleanup
	// does not hang joining it.
	h.server.closeEvents()
}

// TestAuthorizeClaimCommitsWhenStopBeaconNeverReturns is D-024: the claim
// handshake calls StopBeacon synchronously while holding the lease, and this
// is the one unlocked call inside it. An mDNS shutdown that never returns
// must not hang the handshake -- the claim still commits, on its bound, with
// a diagnostic recorded rather than a claim that hangs forever.
func TestAuthorizeClaimCommitsWhenStopBeaconNeverReturns(t *testing.T) {
	h := newHarness(t)
	metadata := h.stageSuccessfully()
	before := len(h.coordinator.diagnostics.snapshot())

	blocked := make(chan struct{})
	unblock := make(chan struct{})
	var once sync.Once
	h.network.stopBeacon = func() error {
		once.Do(func() { close(blocked) })
		<-unblock
		return nil
	}
	t.Cleanup(func() { close(unblock) })

	claimDone := make(chan error, 1)
	go func() { claimDone <- h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID) }()

	<-blocked
	h.awaitBoundsPending()
	h.bounds.fire()

	select {
	case err := <-claimDone:
		if err != nil {
			t.Fatalf("AuthorizeClaim returned %v, want success once the bound elapses", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("AuthorizeClaim never returned")
	}
	if got := h.state(); got != stateTransferring {
		t.Errorf("state is %q, want %q -- a stuck StopBeacon must not block the commit", got, stateTransferring)
	}
	events := h.observer.published()
	if !slices.Equal(kindsOf(events), []EventKind{TransferStarted}) {
		t.Errorf("published %v, want exactly [started]", kindsOf(events))
	}
	if got := len(h.coordinator.diagnostics.snapshot()); got <= before {
		t.Errorf("%d diagnostics recorded, want more than the %d before: the bound timeout must leave a trace", got, before)
	}
}

// TestCancelHonoursAnAbandonedCallerContext is D-036: Cancel now takes a
// context and must honour it, not merely accept it. Cancel is made to
// genuinely wait for a lease someone else holds, and the caller's own context
// is cancelled mid-wait -- the case the contract calls "Cancel abandoned".
// Cancel must return promptly with a coded failure rather than waiting out
// its full bound.
func TestCancelHonoursAnAbandonedCallerContext(t *testing.T) {
	h := newHarness(t)

	// Nothing staged, and somebody else owns the lease -- the IDLE branch of
	// Cancel, which still has to wait for it.
	<-h.coordinator.lease
	t.Cleanup(func() {
		select {
		case h.coordinator.lease <- struct{}{}:
		default:
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancelDone := make(chan error, 1)
	go func() { cancelDone <- h.coordinator.Cancel(ctx) }()

	h.awaitBoundsPending()
	cancel()

	select {
	case err := <-cancelDone:
		if err == nil {
			t.Fatal("Cancel succeeded, want a coded failure: the caller's context was cancelled")
		}
		if code := ErrorCodeOf(err); code != ErrTransferFailed {
			t.Errorf("Cancel error code = %q, want %q", code, ErrTransferFailed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Cancel did not honour the cancelled context; it waited for the full bound instead")
	}
}

// TestShutdownReportsACodedFailureWhenTheLeaseNeverFrees is the other half of
// D-032 and D-036's "honour a context" requirement, proven through the fixed
// bound rather than the caller's context this time: Shutdown must not wait
// forever to join a teardown that never finishes, and on a timeout it must
// not have taken ownership of a lease it never actually acquired.
func TestShutdownReportsACodedFailureWhenTheLeaseNeverFrees(t *testing.T) {
	h := newHarness(t)

	<-h.coordinator.lease
	t.Cleanup(func() {
		select {
		case h.coordinator.lease <- struct{}{}:
		default:
		}
	})

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- h.coordinator.Shutdown(context.Background()) }()

	h.awaitBoundsPending()
	h.bounds.fire()

	select {
	case err := <-shutdownDone:
		if err == nil {
			t.Fatal("Shutdown succeeded, want a coded failure: the lease never freed before its bound")
		}
		if code := ErrorCodeOf(err); code != ErrTransferFailed {
			t.Errorf("Shutdown error code = %q, want %q", code, ErrTransferFailed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown never returned")
	}
	// Shutdown never acquired the lease, so it must not have released one
	// either -- releaseLease on a lease this call does not own would hand a
	// phantom token to whoever asks next.
	if !h.coordinator.leaseHeld() {
		t.Error("the lease was released by a Shutdown that never acquired it")
	}
}

// TestCancelInsideArmResetsWindowLeavesExactlyOneTimerStoppedNotLeaked is
// D-037, forced deterministically through the afterArm test hook rather than
// left to scheduling: a Cancel that marks the session cancelled between
// armReset creating its own reset timer and re-checking the session must see
// the mark on the re-check and stop the timer it just created, rather than
// leaving a callback armed for a session that is already gone.
func TestCancelInsideArmResetsWindowLeavesExactlyOneTimerStoppedNotLeaked(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()
	live := h.liveSession()

	hookDone := make(chan struct{})
	cancelDone := make(chan error, 1)
	h.coordinator.afterArm = func() {
		go func() { cancelDone <- h.coordinator.Cancel(context.Background()) }()
		// Proves Cancel reached the marker -- not that Cancel has returned,
		// which cannot happen yet: Cancel's own unwind joins this very
		// drainer goroutine, which is parked right here.
		h.awaitCancelled()
		close(hookDone)
	}

	h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))

	select {
	case <-hookDone:
	case <-time.After(mutexProbeTimeout):
		t.Fatal("armReset's window hook never ran")
	}

	deadline := time.Now().Add(mutexProbeTimeout)
	for h.timer.stops() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("armReset never stopped the timer it armed for an already-cancelled session")
		}
		time.Sleep(200 * time.Microsecond)
	}

	if got := h.timer.armed(); got != 1 {
		t.Fatalf("%d resets were armed, want exactly one", got)
	}
	if got := h.timer.stops(); got != 1 {
		t.Fatalf("%d armed resets were stopped, want exactly one", got)
	}

	// Let the drainer -- and therefore Cancel's own join -- finish, the same
	// way a real Stop closing the lane would.
	h.server.closeEvents()

	select {
	case err := <-cancelDone:
		if err != nil {
			t.Errorf("Cancel returned %v, want nil", err)
		}
	case <-time.After(mutexProbeTimeout):
		t.Fatal("Cancel never returned")
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
	if live.ctx.Err() == nil {
		t.Error("Cancel left the session context uncancelled")
	}
}

// TestTheBoundsAreTheDocumentedDurationsNotJustSeams pins the values production
// actually uses.
//
// Every other bounded-wait test in this package drives timeouts through the
// injected BoundTimer, a fake that never sleeps and fires only when a test says
// so. That is the right way to test the mechanism and it is completely blind to
// the duration: cutting leaseBound from 45 seconds to 45 milliseconds leaves
// every one of those tests green while making a healthy teardown on a slow
// machine report failure. Found by review, confirmed by mutation.
//
// The relationship is pinned too, not just the numbers. leaseBound's comment
// claims it covers every bounded step below it running to its own bound in
// sequence; that claim is arithmetic, so it can be checked rather than trusted.
func TestTheBoundsAreTheDocumentedDurationsNotJustSeams(t *testing.T) {
	t.Parallel()

	for _, bound := range []struct {
		name string
		got  time.Duration
		want time.Duration
	}{
		{"adapterCallBound", adapterCallBound, 10 * time.Second},
		{"drainerJoinBound", drainerJoinBound, 10 * time.Second},
		{"leaseBound", leaseBound, 45 * time.Second},
	} {
		if bound.got != bound.want {
			t.Errorf("%s = %v, want %v -- production uses this value, and no seam-driven test can see it",
				bound.name, bound.got, bound.want)
		}
		// A bound in milliseconds would be reached by a healthy teardown on a
		// loaded machine, which turns a working cancel into a reported failure.
		if bound.got < time.Second {
			t.Errorf("%s = %v, which is short enough for a healthy teardown to hit", bound.name, bound.got)
		}
	}

	// leaseBound must outlast the sequence it is documented to cover: a
	// StopBeacon and a ServerPort.Stop, each to adapterCallBound, then the
	// drainer join. Without margin a command could give up on a teardown that
	// was still making progress within its own bounds.
	sequential := 2*adapterCallBound + drainerJoinBound
	if leaseBound <= sequential {
		t.Errorf("leaseBound = %v but the steps it covers can take %v in sequence: "+
			"a command would abandon a teardown that was still inside its own bounds",
			leaseBound, sequential)
	}
}

// TestCancelReportsACodedFailureWhenServerStopNeverReturns drives the one
// bounded wait the story left unexercised.
//
// Its sibling on the beacon side is TestAuthorizeClaimCommitsWhenStopBeaconNeverReturns.
// Both go through callBounded, so the shared helper was covered -- but
// stopServerBounded builds its own coded failure and records its own
// diagnostic, and no fake server.stop hook in this package ever blocked, so
// replacing that whole branch with `return nil` left the package green. Found
// by review, confirmed by mutation.
//
// This is the case D-017 named and the one the story exists for: the real
// ServerPort.Stop holds no lock the coordinator can see, so a Stop that never
// returns is exactly how Cancel used to wedge forever.
func TestCancelReportsACodedFailureWhenServerStopNeverReturns(t *testing.T) {
	h := newHarness(t)
	h.transferring()
	before := len(h.coordinator.diagnostics.snapshot())

	blocked := make(chan struct{})
	unblock := make(chan struct{})
	var once sync.Once
	// The lane closes first, then the call hangs. That ordering is what
	// isolates this bound: a Stop that blocks without closing the lane also
	// strands the drainer, so Cancel would report the drainer's bound instead
	// and this branch could be deleted with the test still green -- which is
	// exactly what the first version of this test did.
	h.server.stop = func() error {
		once.Do(func() {
			h.server.closeEvents()
			close(blocked)
		})
		<-unblock
		return nil
	}
	t.Cleanup(func() { close(unblock) })

	cancelDone := make(chan error, 1)
	go func() { cancelDone <- h.coordinator.Cancel(context.Background()) }()

	<-blocked

	// A stuck ServerPort.Stop cascades: the bound on the call itself elapses
	// first, and then the drainer join has its own bound, because the drainer
	// only ends when Stop closes the event lane -- which this Stop never does.
	// So the teardown arms bounds in sequence and each one has to be driven.
	// Firing once and expecting Cancel to return was this test's own first
	// mistake, not the code's.
	var cancelErr error
	deadline := time.Now().Add(10 * time.Second)
	for {
		select {
		case cancelErr = <-cancelDone:
		default:
			if time.Now().After(deadline) {
				t.Fatal("Cancel never returned: some bound did not end its wait")
			}
			// fireIfArmed rather than fire: the cascade arms its bounds one
			// after another, so between two of them there is a moment with
			// nothing left unfired, and fire() treats that as fatal.
			if !h.bounds.fireIfArmed() {
				time.Sleep(200 * time.Microsecond)
			}
			continue
		}
		break
	}

	if cancelErr == nil {
		t.Fatal("Cancel returned success while ServerPort.Stop never came back: " +
			"a bound that elapses must never report the resource gone")
	}
	if code := ErrorCodeOf(cancelErr); code != ErrTransferFailed {
		t.Errorf("Cancel returned code %q, want %q", code, ErrTransferFailed)
	}
	// Named, not merely coded: the drainer's bound carries the same public
	// code, so without this the failure could be coming from the wrong wait.
	if !strings.Contains(cancelErr.Error(), "server did not confirm it stopped") {
		t.Errorf("Cancel reported %q, want the failure to name the server stop that never returned", cancelErr)
	}

	if got := len(h.coordinator.diagnostics.snapshot()); got <= before {
		t.Errorf("%d diagnostics recorded, want more than the %d before: a bound that elapsed must leave a trace",
			got, before)
	}
}
