package transfer

import (
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
			run: func(_ *testing.T, h *harness) SessionID {
				metadata := h.stageSuccessfully()
				h.emit(progressEvent(metadata.SessionID, testProgress(1024, 25)))
				h.emit(progressEvent(metadata.SessionID, testProgress(2048, 50)))
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
	if want := "Transfer canceled."; failure.Error.Message == want {
		t.Errorf("the error event carries cancellation copy %q", want)
	}
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

	h.coordinator.cancelSession()
	h.server.closeEvents()
	h.awaitDrainer()

	events := h.observer.published()
	if len(events) != 1 || events[0].Kind != TransferStarted {
		t.Fatalf("published %+v, want the started event and nothing else", events)
	}
	if got := h.state(); got == stateError {
		t.Error("a cancelled session synthesized a transfer failure")
	}
	if h.timer.armed() != 0 {
		t.Errorf("%d resets are armed, want none for a cancelled session", h.timer.armed())
	}
	_ = metadata
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
	for index, event := range events {
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
			if event.Progress == nil || event.Error != nil {
				t.Errorf("%s must carry progress and no error: %+v", event.Kind, event)
			}
		case TransferError:
			terminals++
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
