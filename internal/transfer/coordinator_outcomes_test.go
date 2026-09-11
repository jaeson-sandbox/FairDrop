package transfer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"testing"
	"time"
)

func TestProgressPublishesContiguousSequences(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	h.emit(progressEvent(metadata.SessionID, testProgress(1024, 25)))
	h.emit(progressEvent(metadata.SessionID, testProgress(2048, 50)))
	events := h.awaitEvents(3)

	if len(events) != 3 {
		t.Fatalf("published %+v, want started and two progress events", events)
	}
	assertEventGrammar(t, metadata.SessionID, events)

	want := []EventKind{TransferStarted, TransferProgress, TransferProgress}
	for index, kind := range want {
		if events[index].Kind != kind {
			t.Errorf("event %d is %q, want %q", index, events[index].Kind, kind)
		}
		if !h.observer.leaseHeldAt(index) {
			t.Errorf("event %d was published without the operation lease", index)
		}
	}
	if got := *events[1].Progress; got != testProgress(1024, 25) {
		t.Errorf("first snapshot is %+v, want the server's %+v", got, testProgress(1024, 25))
	}
	if got := *events[2].Progress; got != testProgress(2048, 50) {
		t.Errorf("second snapshot is %+v, want the server's %+v", got, testProgress(2048, 50))
	}
	if got := h.state(); got != stateTransferring {
		t.Errorf("state is %q, want %q", got, stateTransferring)
	}
	if h.coordinator.leaseHeld() {
		t.Error("the operation lease was not returned after a progress publication")
	}
}

// A snapshot that cannot take the lease is dropped, and dropping it must not
// consume a sequence number: a gap in seq is indistinguishable from an event
// the UI lost, and the frontend discards everything at or below its last seq.
func TestProgressIsDroppedWhileTheLeaseIsHeld(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	h.coordinator.mu.Lock()
	took := h.coordinator.acquireLease()
	h.coordinator.mu.Unlock()
	if !took {
		t.Fatal("the lease was still held after a committed claim")
	}

	h.emit(progressEvent(metadata.SessionID, testProgress(1024, 25)))
	// The drainer can only receive this one once it has finished handling the
	// snapshot above, so that snapshot is provably dropped while the lease is
	// held rather than merely late. This one is refused whenever it is handled,
	// because it names another session, so it needs no timing of its own.
	h.emit(progressEvent("some-other-session", testProgress(2048, 50)))
	h.coordinator.releaseLease()
	h.emit(progressEvent(metadata.SessionID, testProgress(3072, 75)))

	events := h.awaitEvents(2)
	if len(events) != 2 {
		t.Fatalf("published %+v, want started and the one snapshot that took the lease", events)
	}
	if events[1].Kind != TransferProgress || events[1].Seq != 2 {
		t.Errorf("published %+v, want a progress event at seq 2 -- a dropped snapshot must consume no sequence", events[1])
	}
	if got := *events[1].Progress; got != testProgress(3072, 75) {
		t.Errorf("snapshot is %+v, want the one published after the lease was free", got)
	}
	assertEventGrammar(t, metadata.SessionID, events)
}

func TestProgressIsRefusedOutsideAMatchingTransfer(t *testing.T) {
	for _, testCase := range []struct {
		name string
		run  func(t *testing.T, h *harness) SessionID
	}{
		{
			name: "before the transfer is claimed",
			run: func(t *testing.T, h *harness) SessionID {
				metadata := h.stageSuccessfully()
				h.emit(progressEvent(metadata.SessionID, testProgress(1024, 25)))
				h.emit(progressEvent(metadata.SessionID, testProgress(2048, 50)))
				// emit blocks until each event is handled, so both refusals are
				// already resolved here. The shared check below only looks for
				// the second snapshot's BytesSent, which would still pass if the
				// first (1024) snapshot leaked through -- assert no progress
				// event reached the observer at all.
				for _, event := range h.observer.published() {
					if event.Kind == TransferProgress {
						t.Errorf("a pre-claim snapshot was published: %+v", event)
					}
				}
				return metadata.SessionID
			},
		},
		{
			name: "another session's snapshot",
			run: func(t *testing.T, h *harness) SessionID {
				metadata := h.transferring()
				h.emit(progressEvent("some-other-session", testProgress(1024, 25)))
				h.emit(progressEvent("some-other-session", testProgress(2048, 50)))
				// A matching snapshot follows, so the refusals are provably
				// handled and their cost is measurable: the survivor must
				// still be seq 2.
				h.emit(progressEvent(metadata.SessionID, testProgress(3072, 75)))
				events := h.awaitEvents(2)
				if events[1].Seq != 2 {
					t.Errorf("the surviving snapshot is at seq %d, want 2 -- a refused event consumes no sequence", events[1].Seq)
				}
				return metadata.SessionID
			},
		},
		{
			name: "after the terminal outcome",
			run: func(_ *testing.T, h *harness) SessionID {
				h.bufferLane(4)
				metadata := h.transferring()
				h.server.publish(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
				h.server.publish(progressEvent(metadata.SessionID, testProgress(2048, 50)))
				h.awaitDrainer()
				return metadata.SessionID
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)

			id := testCase.run(t, h)

			for _, event := range h.observer.published() {
				if event.Kind == TransferProgress && event.Progress != nil && event.Progress.BytesSent == 2048 {
					t.Errorf("a refused snapshot was published: %+v", event)
				}
			}
			assertEventGrammar(t, id, h.observer.published())
		})
	}
}

func TestServerCompleteDrivesTheSuccessGrammar(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()
	final := testProgress(testSize, 100)
	teardownBefore := len(h.calls.teardownCalls())

	h.emit(completeEvent(metadata.SessionID, final))
	h.awaitDrainer()

	events := h.observer.published()
	if len(events) != 3 {
		t.Fatalf("published %+v, want started, the final progress, and complete", events)
	}
	assertEventGrammar(t, metadata.SessionID, events)
	if events[1].Kind != TransferProgress || *events[1].Progress != final {
		t.Errorf("event 1 is %+v, want the authoritative final progress %+v", events[1], final)
	}
	if events[2].Kind != TransferComplete {
		t.Fatalf("event 2 is %q, want %q", events[2].Kind, TransferComplete)
	}
	if events[2].Progress == nil || *events[2].Progress != final {
		t.Errorf("complete carries %+v, want the authoritative snapshot %+v", events[2].Progress, final)
	}
	for index := range events {
		if !h.observer.leaseHeldAt(index) {
			t.Errorf("event %d was published without the operation lease", index)
		}
	}

	// The listener is closed before the UI is told the transfer finished.
	if got := h.calls.teardownCalls()[teardownBefore:]; !slices.Equal(got, []string{"server.Stop"}) {
		t.Errorf("the terminal path released %v, want exactly [server.Stop] -- the beacon went at the claim", got)
	}
	// Ordering, not just presence. teardownCalls filters the log down to the
	// release calls, so on its own it can only show that Stop ran -- never that
	// it ran before the UI was told. Moving the release after the publications
	// leaves every other assertion in this test unchanged.
	unfiltered := h.calls.snapshot()
	stopAt := slices.Index(unfiltered, "server.Stop")
	if stopAt < 0 {
		t.Fatalf("server.Stop is missing from the call log %v", unfiltered)
	}
	publishes := 0
	terminalPublishAt := -1
	for index, call := range unfiltered {
		if call != "observer.Publish" {
			continue
		}
		publishes++
		// Publication 1 is the started event, from the claim. The terminal
		// path's own publications are the ones that must follow the release.
		if publishes == 2 {
			terminalPublishAt = index
			break
		}
	}
	if terminalPublishAt < 0 {
		t.Fatalf("the terminal path published nothing: %v", unfiltered)
	}
	if stopAt > terminalPublishAt {
		t.Errorf("server.Stop ran at %d, after the terminal publication at %d -- the UI was told "+
			"the transfer finished while the listener was still accepting", stopAt, terminalPublishAt)
	}
	if got := h.state(); got != stateDone {
		t.Errorf("state is %q, want %q", got, stateDone)
	}
	if h.coordinator.leaseHeld() {
		t.Error("the terminal path kept the operation lease")
	}
	if h.timer.armed() != 1 {
		t.Fatalf("%d resets are armed, want exactly one", h.timer.armed())
	}
	if delay := h.timer.calls()[0].delay; delay != 3*time.Second {
		t.Errorf("the reset is armed for %v, want 3s", delay)
	}
}

func TestServerFailureWithBytesPublishesFinalProgressThenError(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()
	final := testProgress(1024, 25)

	h.emit(failedEvent(metadata.SessionID, &final,
		WrapError(ErrSourceChanged, "the item moved", errors.New(testPath))))
	h.awaitDrainer()

	events := h.observer.published()
	if len(events) != 3 {
		t.Fatalf("published %+v, want started, the final progress, and the error", events)
	}
	assertEventGrammar(t, metadata.SessionID, events)
	if events[1].Kind != TransferProgress || *events[1].Progress != final {
		t.Errorf("event 1 is %+v, want the final written-byte count %+v", events[1], final)
	}
	failure := events[2]
	if failure.Kind != TransferError || failure.Error == nil {
		t.Fatalf("event 2 is %+v, want an error event", failure)
	}
	if failure.Error.Code != ErrSourceChanged {
		t.Errorf("error code is %q, want the recognized %q", failure.Error.Code, ErrSourceChanged)
	}
	if want := "The item changed after it was prepared. Cancel and create a fresh link."; failure.Error.Message != want {
		t.Errorf("error message is %q, want the fixed copy %q", failure.Error.Message, want)
	}
	if failure.Progress == nil || *failure.Progress != final {
		t.Errorf("the error carries %+v, want the written-byte snapshot %+v", failure.Progress, final)
	}
	if got := h.state(); got != stateError {
		t.Errorf("state is %q, want %q", got, stateError)
	}
	if h.timer.armed() != 1 {
		t.Errorf("%d resets are armed after a failure, want exactly one", h.timer.armed())
	}
}

// A failure before the first byte must not be dressed up as a zero-byte
// progress event: "nothing was sent" and "zero bytes have been sent so far"
// are different claims about the wire.
func TestServerFailureBeforeAnyBytePublishesNoProgress(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	h.emit(failedEvent(metadata.SessionID, nil, nil))
	h.awaitDrainer()

	events := h.observer.published()
	if len(events) != 2 {
		t.Fatalf("published %+v, want started and the error only", events)
	}
	assertEventGrammar(t, metadata.SessionID, events)
	if events[1].Kind != TransferError || events[1].Seq != 2 {
		t.Fatalf("event 1 is %+v, want the error at seq 2", events[1])
	}
	if events[1].Progress != nil {
		t.Errorf("the error carries %+v, want no snapshot at all", events[1].Progress)
	}
	if events[1].Error == nil || events[1].Error.Code != ErrTransferFailed {
		t.Errorf("error is %+v, want the %q fallback for an unclassified cause", events[1].Error, ErrTransferFailed)
	}
}

// A server-reported cancellation with no coordinator teardown behind it is
// still a failure to the UI. Cancellation copy belongs to reset, and must
// never appear inside an error event.
func TestServerFailureCodedCancelledIsPublishedAsATransferFailure(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	h.emit(failedEvent(metadata.SessionID, nil, NewError(ErrCancelled, "the receiver went away")))
	h.awaitDrainer()

	events := h.observer.published()
	if len(events) != 2 {
		t.Fatalf("published %+v, want started and the error", events)
	}
	failure := events[1]
	if failure.Error == nil || failure.Error.Code != ErrTransferFailed {
		t.Fatalf("error is %+v, want %q", failure.Error, ErrTransferFailed)
	}
	// No separate message assertion here: PublicErrorOf sources the message
	// from the code alone, so a check that the message is not the cancellation
	// copy cannot fail independently of the code check above -- it is already
	// redundant with TestATerminalFailureOnlyPublishesCodesThatDescribeIt's
	// "a cancellation" case, which pins the whole code-to-copy table.
	if got := h.state(); got != stateError {
		t.Errorf("state is %q, want %q", got, stateError)
	}
}

func TestASecondTerminalEventIsDiscarded(t *testing.T) {
	h := newHarness(t)
	h.bufferLane(4)
	metadata := h.transferring()
	final := testProgress(testSize, 100)

	// The second outcome is queued from inside the first one's teardown. That is
	// the only moment it can be queued deterministically: the first has been
	// accepted, the drainer is busy quiescing rather than reading, and the lane
	// is still open because the fake closes it last. Publishing both up front
	// instead raced the drainer, which took the first and closed the lane before
	// the second was ever offered -- a flake about one run in forty.
	queued := make(chan bool, 1)
	h.server.stop = func() error {
		select {
		case queued <- h.server.publish(failedEvent(
			metadata.SessionID, nil, NewError(ErrTransferFailed, "a second outcome"))):
		default:
		}
		return nil
	}

	if !h.server.publish(completeEvent(metadata.SessionID, final)) {
		t.Fatal("the complete event was not queued")
	}
	h.awaitDrainer()

	select {
	case ok := <-queued:
		if !ok {
			t.Fatal("the second terminal event was not queued")
		}
	default:
		t.Fatal("the first outcome never reached teardown, so no second outcome was offered")
	}

	events := h.observer.published()
	if len(events) != 3 {
		t.Fatalf("published %+v, want started, final progress and complete only", events)
	}
	assertEventGrammar(t, metadata.SessionID, events)
	if events[2].Kind != TransferComplete {
		t.Errorf("the accepted outcome is %q, want %q", events[2].Kind, TransferComplete)
	}
	if got := h.state(); got != stateDone {
		t.Errorf("state is %q, want %q -- the second outcome changed it", got, stateDone)
	}
	if h.timer.armed() != 1 {
		t.Errorf("%d resets are armed, want exactly one", h.timer.armed())
	}
}

// Only ServerPort.Stop closes the lane. A closure with no terminal event and
// no teardown pending means the server went away mid-transfer, which the UI
// would otherwise never hear about.
func TestLaneClosureWithoutAnOutcomeSynthesizesAFailure(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	h.server.closeEvents()
	h.awaitDrainer()

	events := h.observer.published()
	if len(events) != 2 {
		t.Fatalf("published %+v, want started and one synthesized error", events)
	}
	assertEventGrammar(t, metadata.SessionID, events)
	if events[1].Kind != TransferError || events[1].Error == nil {
		t.Fatalf("event 1 is %+v, want an error event", events[1])
	}
	if events[1].Error.Code != ErrTransferFailed {
		t.Errorf("error code is %q, want %q", events[1].Error.Code, ErrTransferFailed)
	}
	if events[1].Progress != nil {
		t.Errorf("the synthesized error invented a snapshot: %+v", events[1].Progress)
	}
	if got := h.state(); got != stateError {
		t.Errorf("state is %q, want %q", got, stateError)
	}
	if h.calls.count("server.Stop") != 1 {
		t.Errorf("server.Stop ran %d times, want once -- the synthesized outcome still quiesces", h.calls.count("server.Stop"))
	}
}

// The same closure during a teardown is normal and silent: the Cancel that
// asked for the Stop owns that outcome.
func TestLaneClosureDuringATeardownIsSilent(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	// Cancel is the real teardown that asks for the Stop: its own unwind calls
	// ServerPort.Stop, which is what closes the lane here -- there is no
	// separate h.server.closeEvents() because that call is exactly the one
	// under test. The drainer notices the same closure concurrently and must
	// find the session already cancelled and stay silent rather than
	// synthesizing a second outcome.
	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatalf("Cancel returned %v, want success", err)
	}
	h.awaitDrainer()

	events := h.observer.published()
	if !slices.Equal(kindsOf(events), []EventKind{TransferStarted, TransferReset}) {
		t.Fatalf("published %v, want exactly [started, reset] -- a synthesized failure would mean the lane closure was not silent", kindsOf(events))
	}
	assertEventGrammar(t, metadata.SessionID, events)
	if got := h.state(); got != stateIdle {
		t.Errorf("state is %q, want %q", got, stateIdle)
	}
	if got := h.calls.count("server.Stop"); got != 1 {
		t.Errorf("server.Stop ran %d times, want exactly one -- the drainer's own closure handling must call it none", got)
	}
	if h.timer.armed() != 0 {
		t.Errorf("%d resets are armed, want none for a cancelled session", h.timer.armed())
	}
}

// The producing adapter owes finite, clamped values. This is the boundary that
// cannot afford to trust that: one NaN would fail JSON marshalling and cost the
// UI the whole event rather than a field.
func TestProgressValuesAreForcedIntoTheirContractRange(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	h.emit(progressEvent(metadata.SessionID, ProgressSnapshot{
		BytesSent:        1024,
		TotalBytes:       testSize,
		TotalKnown:       true,
		Percent:          math.NaN(),
		SpeedBytesPerSec: math.Inf(-1),
	}))
	h.emit(progressEvent(metadata.SessionID, ProgressSnapshot{
		BytesSent:        2048,
		TotalBytes:       testSize,
		TotalKnown:       true,
		Percent:          150,
		SpeedBytesPerSec: math.Inf(1),
	}))
	events := h.awaitEvents(3)

	if got := events[1].Progress.Percent; got != 0 {
		t.Errorf("a NaN percent published as %v, want 0", got)
	}
	if got := events[1].Progress.SpeedBytesPerSec; got != 0 {
		t.Errorf("a negative infinite speed published as %v, want 0", got)
	}
	if got := events[2].Progress.Percent; got != 100 {
		t.Errorf("a percent of 150 published as %v, want the clamp at 100", got)
	}
	// The positive-infinity arm of the same clamp. Without this the only thing
	// standing between +Inf and the UI is the json.Marshal loop below, which
	// fails on any infinity and so cannot tell a clamped value from an
	// unclamped one that happens to marshal.
	if got := events[2].Progress.SpeedBytesPerSec; got != math.MaxFloat64 {
		t.Errorf("an infinite speed published as %v, want the clamp at %v", got, math.MaxFloat64)
	}
	for _, event := range events {
		if _, err := json.Marshal(event); err != nil {
			t.Errorf("event %+v does not marshal: %v", event, err)
		}
	}
}

func TestTerminalOutcomesNeverCarryTheTokenOrThePath(t *testing.T) {
	for _, testCase := range []struct {
		name string
		emit func(h *harness, id SessionID)
	}{
		{"complete", func(h *harness, id SessionID) {
			h.emit(completeEvent(id, testProgress(testSize, 100)))
		}},
		{"failure carrying an adapter cause", func(h *harness, id SessionID) {
			final := testProgress(1024, 25)
			h.emit(failedEvent(id, &final, WrapError(ErrTransferFailed,
				"streaming "+testPath+" failed", errors.New(string(testToken)))))
		}},
		{"a stop that reports a diagnostic", func(h *harness, id SessionID) {
			h.server.stop = func() error {
				return WrapError(ErrTransferFailed, "cleanup of "+testPath+" reported a problem",
					errors.New(string(testToken)))
			}
			h.emit(completeEvent(id, testProgress(testSize, 100)))
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			metadata := h.transferring()
			token := string(h.liveSession().token)

			testCase.emit(h, metadata.SessionID)
			h.awaitDrainer()

			for _, event := range h.observer.published() {
				assertSafe(t, "published event", fmt.Sprintf("%+v", event), token)
				if event.Error != nil {
					assertSafe(t, "public error", fmt.Sprintf("%+v", *event.Error), token)
				}
			}
			for _, entry := range h.coordinator.diagnostics.snapshot() {
				assertSafe(t, "diagnostic", fmt.Sprintf("%s %s", entry.code, entry.message), token)
			}
		})
	}
}

// assertEventGrammar checks what the contract fixes for every published event
// of one session: the sequence starts at 1 and increments by exactly one, no
// event belongs to another session, at most one terminal outcome is present,
// and each kind carries exactly the payload its row of the table allows.
func assertEventGrammar(t *testing.T, id SessionID, events []Event) {
	t.Helper()

	terminals := 0
	terminalAt := -1
	for index, event := range events {
		switch event.Kind {
		case TransferStarted:
			if index != 0 {
				t.Errorf("started is at index %d, want 0 -- nothing precedes it", index)
			}
		case TransferReset:
			if index != len(events)-1 {
				t.Errorf("reset is at index %d of %d, want last -- it terminates the session",
					index, len(events)-1)
			}
		case TransferProgress:
			if terminalAt >= 0 {
				t.Errorf("progress at index %d follows the terminal event at %d", index, terminalAt)
			}
		}
		if terminalAt >= 0 && event.Kind != TransferReset {
			t.Errorf("event %d (%s) follows the terminal event at %d; only reset may",
				index, event.Kind, terminalAt)
		}
		if event.SessionID != id {
			t.Errorf("event %d belongs to %q, want %q", index, event.SessionID, id)
		}
		if want := uint64(index + 1); event.Seq != want {
			t.Errorf("event %d has seq %d, want %d -- sequences start at 1 and never gap", index, event.Seq, want)
		}
		switch event.Kind {
		case TransferStarted, TransferReset:
			if event.Progress != nil || event.Error != nil {
				t.Errorf("%s carries a payload: %+v", event.Kind, event)
			}
		case TransferProgress:
			if event.Progress == nil || event.Error != nil {
				t.Errorf("%s must carry progress and no error: %+v", event.Kind, event)
			}
		case TransferComplete:
			terminals++
			terminalAt = index
			if event.Progress == nil || event.Error != nil {
				t.Errorf("%s must carry progress and no error: %+v", event.Kind, event)
			}
		case TransferError:
			terminals++
			terminalAt = index
			if event.Error == nil {
				t.Errorf("%s must carry a public error: %+v", event.Kind, event)
			}
		default:
			t.Errorf("event %d has unknown kind %q", index, event.Kind)
		}
		if event.Progress != nil {
			if math.IsNaN(event.Progress.Percent) || event.Progress.Percent < 0 || event.Progress.Percent > 100 {
				t.Errorf("event %d reports percent %v, want a finite value in [0,100]", index, event.Progress.Percent)
			}
		}
	}
	if terminals > 1 {
		t.Errorf("%d terminal events published for one session, want at most one", terminals)
	}
}

// A terminal outcome that arrives while the lease is held belongs to whichever
// teardown holds it. The drainer discards it, and -- the load-bearing half --
// never waits for the lease: a drainer blocked there while a Cancel waits for
// the drainer is the deadlock this design exists to make unrepresentable.
func TestATerminalOutcomeIsDiscardedWhileTheLeaseIsHeld(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	h.coordinator.mu.Lock()
	took := h.coordinator.acquireLease()
	h.coordinator.mu.Unlock()
	if !took {
		t.Fatal("the lease was still held after a committed claim")
	}

	h.emit(completeEvent(metadata.SessionID, testProgress(testSize, 100)))
	// The drainer can only take this one once it has finished with the outcome
	// above. A drainer that waited for the lease would never take it, and this
	// call fails by name instead of hanging.
	h.emit(progressEvent("some-other-session", testProgress(2048, 50)))
	h.coordinator.releaseLease()

	events := h.observer.published()
	if len(events) != 1 || events[0].Kind != TransferStarted {
		t.Fatalf("published %+v, want the started event and nothing else", events)
	}
	if got := h.state(); got != stateTransferring {
		t.Errorf("state is %q, want %q -- a discarded outcome must settle nothing", got, stateTransferring)
	}
	if h.timer.armed() != 0 {
		t.Errorf("%d resets are armed, want none", h.timer.armed())
	}
	if got := h.calls.count("server.Stop"); got != 0 {
		t.Errorf("a discarded outcome tore the server down %d times, want none", got)
	}
}

// A terminal snapshot goes through the same clamp as a progress one. The suite
// used to feed out-of-range values only through ServerProgress, so replacing
// terminalSnapshot's sanitize with a bare passthrough left everything green --
// and a terminal event is not coalescable, so a snapshot that fails JSON
// marshalling costs the UI the outcome permanently rather than one update.
func TestTerminalSnapshotsAreForcedIntoTheirContractRange(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		build func(id SessionID, snapshot ProgressSnapshot) ServerEvent
		// failedEvent attaches a snapshot only when bytes actually reached the
		// receiver, so the failure case has to send a positive count for its
		// snapshot to be published at all.
		snapshot ProgressSnapshot
	}{
		{"complete", func(id SessionID, snapshot ProgressSnapshot) ServerEvent {
			return completeEvent(id, snapshot)
		}, ProgressSnapshot{
			BytesSent:        -512,
			TotalBytes:       -1,
			TotalKnown:       false,
			Percent:          math.NaN(),
			SpeedBytesPerSec: math.Inf(1),
		}},
		// In-range values that only the unknown-total rule can zero. The case
		// above cannot pin that rule: its total and percent are already driven
		// to zero by the negative-count and NaN clamps, so dropping the rule
		// would change nothing observable.
		{"complete with an unknown total", func(id SessionID, snapshot ProgressSnapshot) ServerEvent {
			return completeEvent(id, snapshot)
		}, ProgressSnapshot{
			BytesSent:        4096,
			TotalBytes:       9000,
			TotalKnown:       false,
			Percent:          42,
			SpeedBytesPerSec: 1024,
		}},
		{"failure", func(id SessionID, snapshot ProgressSnapshot) ServerEvent {
			return failedEvent(id, &snapshot, NewError(ErrTransferFailed, "the stream broke"))
		}, ProgressSnapshot{
			BytesSent:        -512,
			TotalBytes:       -1,
			TotalKnown:       false,
			Percent:          math.Inf(-1),
			SpeedBytesPerSec: math.NaN(),
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			metadata := h.transferring()

			h.emit(testCase.build(metadata.SessionID, testCase.snapshot))
			h.awaitDrainer()

			events := h.observer.published()
			assertEventGrammar(t, metadata.SessionID, events)
			for index, event := range events {
				if event.Progress == nil {
					continue
				}
				got := *event.Progress
				if got.BytesSent < 0 || got.TotalBytes < 0 {
					t.Errorf("event %d reports %d/%d bytes, want no negative counts",
						index, got.BytesSent, got.TotalBytes)
				}
				if !got.TotalKnown && (got.TotalBytes != 0 || got.Percent != 0) {
					t.Errorf("event %d declares an unknown total but reports %d bytes at %v%%",
						index, got.TotalBytes, got.Percent)
				}
				if math.IsNaN(got.Percent) || math.IsInf(got.SpeedBytesPerSec, 0) {
					t.Errorf("event %d carries non-finite values: %+v", index, got)
				}
				if _, err := json.Marshal(event); err != nil {
					t.Errorf("event %d does not marshal: %v", index, err)
				}
			}
		})
	}
}

// The public copy on a transfer-error has to describe a transfer that began and
// then failed. Every other registered code describes something else, and the
// registry's fixed copy would then contradict the event carrying it -- the
// beacon warning literally says the link still works.
func TestATerminalFailureOnlyPublishesCodesThatDescribeIt(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		cause error
		want  ErrorCode
	}{
		{"a generic stream failure", NewError(ErrTransferFailed, "the stream broke"), ErrTransferFailed},
		{"the source vanished", NewError(ErrPathNotFound, "gone"), ErrPathNotFound},
		{"the source changed", NewError(ErrSourceChanged, "changed"), ErrSourceChanged},
		{"the source became unsupported", NewError(ErrPathUnsupported, "a junction"), ErrPathUnsupported},
		{"a beacon warning", NewError(ErrBeaconWarning, "discovery is down"), ErrTransferFailed},
		{"busy", NewError(ErrBusy, "already running"), ErrTransferFailed},
		{"shutting down", NewError(ErrShuttingDown, "closing"), ErrTransferFailed},
		{"an invalid selection", NewError(ErrInvalidSelection, "no path"), ErrTransferFailed},
		{"a QR failure", NewError(ErrQRFailed, "no image"), ErrTransferFailed},
		{"a cancellation", NewError(ErrCancelled, "stopped"), ErrTransferFailed},
		{"no cause at all", nil, ErrTransferFailed},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			metadata := h.transferring()

			h.emit(failedEvent(metadata.SessionID, nil, testCase.cause))
			h.awaitDrainer()

			events := h.observer.published()
			failure := events[len(events)-1]
			if failure.Kind != TransferError {
				t.Fatalf("the last event is %q, want %q", failure.Kind, TransferError)
			}
			if failure.Error == nil {
				t.Fatal("the error event carries no public error")
			}
			if failure.Error.Code != testCase.want {
				t.Errorf("published code %q, want %q", failure.Error.Code, testCase.want)
			}
			// The copy is the registry's, chosen by the code -- never the
			// adapter's text, and never copy belonging to a different code.
			if want := publicMessages[testCase.want]; failure.Error.Message != want {
				t.Errorf("published copy %q, want %q", failure.Error.Message, want)
			}
		})
	}
}

// An event kind this build does not know, and progress carrying no measurement,
// are both adapter defects. Discarding them is the only safe action; discarding
// them without a trace would hide the defect that produced them.
func TestUnusableServerEventsAreDiscardedWithADiagnostic(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		event func(id SessionID) ServerEvent
	}{
		{"an unrecognized kind", func(id SessionID) ServerEvent {
			return ServerEvent{SessionID: id, Kind: ServerEventKind("bogus")}
		}},
		{"progress with no snapshot", func(id SessionID) ServerEvent {
			return ServerEvent{SessionID: id, Kind: ServerProgress}
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newHarness(t)
			metadata := h.transferring()
			before := len(h.coordinator.diagnostics.snapshot())

			h.emit(testCase.event(metadata.SessionID))

			// A second, well-formed snapshot proves the drainer survived the
			// first and is still forwarding.
			h.emit(progressEvent(metadata.SessionID, testProgress(1024, 25)))
			events := h.awaitEvents(2)

			assertEventGrammar(t, metadata.SessionID, events)
			if len(events) != 2 || events[1].Kind != TransferProgress {
				t.Fatalf("published %+v, want started then one progress event", events)
			}
			if got := h.state(); got != stateTransferring {
				t.Errorf("state is %q, want %q", got, stateTransferring)
			}
			if got := len(h.coordinator.diagnostics.snapshot()); got != before+1 {
				t.Errorf("%d diagnostics recorded, want one more than the %d before", got, before)
			}
		})
	}
}

// eventsFor filters a published stream down to one session's events, in
// order. assertEventGrammar pins seq to start at 1 and never gap for whatever
// slice it is given, so this is what lets a second session's own grammar be
// checked in isolation from the first session's events still sitting in the
// same observer log.
func eventsFor(id SessionID, events []Event) []Event {
	var filtered []Event
	for _, event := range events {
		if event.SessionID == id {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

// The frontend's discard rule treats seq as scoped to one session: a lower or
// equal seq than the last one seen is dropped, and a session id change is
// exactly what tells it to stop comparing against the old high-water mark. No
// other test in this file ever emits on a second session, so seq restarting
// at 1 under a new id -- the other half of that contract -- was unproven.
func TestASecondSessionsSequenceRestartsAtOneUnderItsOwnId(t *testing.T) {
	h := newHarness(t)

	first := h.transferring()
	h.emit(completeEvent(first.SessionID, testProgress(testSize, 100)))
	h.awaitDrainer()
	h.timer.fire()

	firstEvents := eventsFor(first.SessionID, h.observer.published())
	assertEventGrammar(t, first.SessionID, firstEvents)
	if len(firstEvents) != 4 {
		t.Fatalf("first session published %+v, want started, progress, complete and reset", firstEvents)
	}

	// The reset returned the coordinator to IDLE and closed the first
	// session's lane; a fresh lane is what a real second Stage gets from
	// ServerPort.Start.
	h.server.events = make(chan ServerEvent)
	h.server.closed = false

	second := h.transferring()
	if second.SessionID == first.SessionID {
		t.Fatal("the second session reused the first session's id -- seq restarting at 1 would prove nothing")
	}
	h.emit(progressEvent(second.SessionID, testProgress(1024, 25)))
	secondEvents := eventsFor(second.SessionID, h.awaitEvents(len(firstEvents)+2))
	if len(secondEvents) != 2 {
		t.Fatalf("second session published %+v, want started and one progress event", secondEvents)
	}
	assertEventGrammar(t, second.SessionID, secondEvents)
	if secondEvents[0].Seq != 1 {
		t.Errorf("the second session's started event has seq %d, want 1 -- seq is scoped per session", secondEvents[0].Seq)
	}

	// The first session's own events are untouched by the second session
	// existing: no event was renumbered, relabeled or duplicated across them.
	if got := eventsFor(first.SessionID, h.observer.published()); !slices.Equal(got, firstEvents) {
		t.Errorf("the first session's events changed after the second session ran: got %+v, want %+v", got, firstEvents)
	}
}

// terminalSnapshot's Complete arm is documented as refusing to downgrade a
// success: a Complete with no snapshot is a port defect, the bytes did arrive,
// and the outcome must still be DONE carrying the unknown-total zero snapshot.
// completeEvent always builds a snapshot, so no test ever drove that arm.
func TestACompleteCarryingNoSnapshotStillSucceeds(t *testing.T) {
	h := newHarness(t)
	metadata := h.transferring()

	h.emit(ServerEvent{SessionID: metadata.SessionID, Kind: ServerComplete})
	h.awaitDrainer()

	events := h.observer.published()
	if !slices.Equal(kindsOf(events), []EventKind{TransferStarted, TransferComplete}) {
		t.Fatalf("published %v, want exactly [started, complete] -- a missing snapshot must not add a progress event", kindsOf(events))
	}
	assertEventGrammar(t, metadata.SessionID, events)

	final := events[1].Progress
	if final == nil {
		t.Fatal("the complete event carries no progress payload; the contract's payload table requires one")
	}
	if final.TotalKnown || final.TotalBytes != 0 || final.BytesSent != 0 || final.Percent != 0 {
		t.Errorf("the complete event reports %+v, want the unknown-total zero snapshot", *final)
	}
	if got := h.state(); got != stateDone {
		t.Errorf("state is %q, want %q -- a port defect must not downgrade a success", got, stateDone)
	}
}

// TestEveryRecordedDiagnosticAlsoReachesTheSeam is the property Story 3.6
// exists for, asserted on the one path that produces a diagnostic without any
// adapter failing: an observer that panics.
//
// The sink has always been written. What did not exist was a way for anything
// outside this package to see it -- the contract cites "recorded as a
// diagnostic" wherever it swallows a failure, and in a shipped binary that
// record went nowhere. Deleting the seam call from recordDiagnostic left this
// whole package green until this test existed, because every other test reads
// the sink.
func TestEveryRecordedDiagnosticAlsoReachesTheSeam(t *testing.T) {
	h := newHarness(t)
	h.observer.publish = func(Event) { panic("a defective observer") }

	// transferring(), not stageSuccessfully(): Stage publishes no event at
	// all, so a panicking observer there produces nothing to observe. The
	// started event is the first one that reaches the observer.
	h.transferring()

	sunk := h.coordinator.diagnostics.snapshot()
	seen := h.diagnosed.snapshot()
	if len(sunk) == 0 {
		t.Fatal("no diagnostic was recorded at all, so this test would pass vacuously")
	}
	if len(seen) != len(sunk) {
		t.Fatalf("the sink holds %d diagnostics and the seam saw %d: a record that reaches only the "+
			"sink is invisible to a running FairDrop", len(sunk), len(seen))
	}
	for i := range sunk {
		if seen[i] != sunk[i] {
			t.Errorf("diagnostic %d reached the seam as %+v and the sink as %+v", i, seen[i], sunk[i])
		}
	}
}

// TestTheDiagnosticSinkSaysWhenItStoppedRecording is D-031.
//
// The sink's cap is what keeps a long-running session from accumulating an
// unbounded slice, and the silent `return` that enforced it made a truncated
// sink indistinguishable from a complete one to the only thing that reads it.
// Reserving the last slot for a marker costs one entry and turns "these are
// the diagnostics" into a claim the sink can actually support.
//
// Driven through recordDiagnostic rather than the sink's own method so the
// seam count is covered too: an overflow that never reached logDiagnostic
// would be as invisible as the drop it replaced.
func TestTheDiagnosticSinkSaysWhenItStoppedRecording(t *testing.T) {
	h := newHarness(t)

	for i := range maxDiagnostics + 8 {
		h.coordinator.recordDiagnostic(
			NewError(ErrTransferFailed, "a bound elapsed"),
			fmt.Sprintf("diagnostic %d", i),
		)
	}

	entries := h.coordinator.diagnostics.snapshot()
	if len(entries) != maxDiagnostics {
		t.Fatalf("the sink holds %d entries, want exactly its %d cap", len(entries), maxDiagnostics)
	}
	if last := entries[len(entries)-1]; last != diagnosticOverflow {
		t.Errorf("the last entry is %+v, want the overflow marker %+v -- a full sink that says "+
			"nothing reads as a complete one", last, diagnosticOverflow)
	}
	// The marker replaces the entry it dropped; it does not repeat forever.
	for i, entry := range entries[:len(entries)-1] {
		if entry == diagnosticOverflow {
			t.Errorf("entry %d is the overflow marker, which belongs only in the last slot", i)
		}
	}
	if seen := h.diagnosed.snapshot(); len(seen) != maxDiagnostics+8 {
		t.Errorf("the seam saw %d diagnostics, want all %d: the sink's cap bounds what is kept, "+
			"never what a running FairDrop is told", len(seen), maxDiagnostics+8)
	}
}

// TestATerminalFailureRecordsItsCauseBeforeRewritingIt is D-092.
//
// terminalPublicError answers with one of four codes whatever the adapter
// actually said, which is right for a user and destroys the only evidence of
// what happened for anyone debugging it. The code is recorded first. The
// message never is: an adapter's own text is where a path or a token would be
// (AD-9), and recordDiagnostic reads only ErrorCodeOf its cause.
func TestATerminalFailureRecordsItsCauseBeforeRewritingIt(t *testing.T) {
	h := newHarness(t)
	h.transferring()
	token := string(h.liveSession().token)

	h.emit(ServerEvent{
		SessionID: testSessionID,
		Kind:      ServerFailed,
		Err:       WrapError(ErrNetworkUnavailable, "the interface went away", errors.New(testPath)),
	})

	events := h.awaitEvents(2)
	failure := events[len(events)-1]
	if failure.Kind != TransferError || failure.Error == nil {
		t.Fatalf("published %+v, want a terminal error", failure)
	}
	if failure.Error.Code != ErrTransferFailed {
		t.Errorf("the user was told %q, want the public rewrite %q", failure.Error.Code, ErrTransferFailed)
	}

	var recorded bool
	for _, entry := range h.diagnosed.snapshot() {
		if entry.code == ErrNetworkUnavailable {
			recorded = true
		}
		assertSafe(t, "diagnostic", entry.message, token)
	}
	if !recorded {
		t.Errorf("no diagnostic carried the original %q code: after the rewrite there is nothing "+
			"left anywhere that says what actually failed, diagnostics were %+v",
			ErrNetworkUnavailable, h.diagnosed.snapshot())
	}
}
