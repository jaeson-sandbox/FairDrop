package transfer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// Covers the coordinator half of the I/O & Edge-Case Matrix in
// _bmad-output/implementation-artifacts/spec-phone-to-desktop-receiving.md:
// receive admission, the shared lifecycle, cancel races, and the event grammar a
// receive session adds. Every expectation is a literal written out here.

// --- Admission -----------------------------------------------------------

func TestStartReceiveCommitsAWaitingSessionAndWritesNothing(t *testing.T) {
	h := newReceiveHarness(t)

	metadata, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder)
	if err != nil {
		t.Fatalf("StartReceive = %v", err)
	}

	if metadata.SessionID != testSessionID {
		t.Errorf("SessionID = %q, want %q", metadata.SessionID, testSessionID)
	}
	if metadata.Destination != "Phone Photos" {
		t.Errorf("Destination = %q, want the basename %q", metadata.Destination, "Phone Photos")
	}
	if metadata.URL != "http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20" {
		t.Errorf("URL = %q, want the upload capability URL", metadata.URL)
	}
	if png, err := base64.StdEncoding.DecodeString(metadata.QR); err != nil || string(png) != string(testPNG) {
		t.Errorf("QR = %q, want the standard base64 of the encoded PNG", metadata.QR)
	}
	if metadata.Warnings == nil || len(metadata.Warnings) != 0 {
		t.Errorf("Warnings = %#v, want a non-nil empty slice", metadata.Warnings)
	}

	if got := h.state(); got != stateStaged {
		t.Errorf("state = %q, want STAGED", got)
	}
	if got := h.observer.published(); len(got) != 0 {
		t.Errorf("admission published %v, want no lifecycle event", kindsOf(got))
	}
	if got := h.sink.opened(); len(got) != 1 || got[0] != testReceiveFolder {
		t.Errorf("the sink opened %v, want the chosen folder once", got)
	}
	if got := h.qr.encoded(); len(got) != 1 || got[0] != testUploadURL {
		t.Errorf("the QR encoded %v, want the upload URL", got)
	}
	if h.sink.destination(0).closeCalls() != 0 {
		t.Error("the destination was closed while the session is waiting")
	}
	if n := len(h.source.inspected()); n != 0 {
		t.Errorf("source.Inspect ran %d times for a receive session, want 0", n)
	}

	requests := h.server.startRequests()
	if len(requests) != 1 {
		t.Fatalf("server.Start ran %d times, want 1", len(requests))
	}
	if requests[0].Destination != ReceiveDestination(h.sink.destination(0)) {
		t.Error("the server was not handed the opened destination")
	}
	if requests[0].Item.Path != "" || requests[0].Item.Name != "" {
		t.Errorf("the receive request carried an item: %+v", requests[0].Item)
	}
	if requests[0].Token != testToken || requests[0].SessionID != testSessionID {
		t.Errorf("the receive request identity = %q/%q", requests[0].SessionID, requests[0].Token)
	}
}

// The destination must be validated before any network resource exists: a
// refused folder leaves no listener, no beacon and no QR code.
func TestStartReceiveOrdersTheDestinationBeforeEveryNetworkResource(t *testing.T) {
	h := newReceiveHarness(t)
	h.receiveWaiting()

	var order []string
	for _, call := range h.calls.snapshot() {
		switch call {
		case "sink.Open", "network.GetLocalIP", "server.Start", "qr.EncodePNG", "network.StartBeacon":
			order = append(order, call)
		}
	}
	want := []string{"sink.Open", "network.GetLocalIP", "server.Start", "qr.EncodePNG", "network.StartBeacon"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("acquisition order = %v, want %v", order, want)
	}
}

func TestStartReceiveRefusesAnUnusableFolderBeforeTouchingTheNetwork(t *testing.T) {
	for _, testCase := range []struct {
		name string
		err  error
		want ErrorCode
	}{
		{"a missing folder", NewError(ErrPathNotFound, "missing"), ErrPathNotFound},
		{"a file or link-like folder", NewError(ErrPathUnsupported, "not plain"), ErrPathUnsupported},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			h := newReceiveHarness(t)
			h.sink.open = func(context.Context, string) (ReceiveDestination, error) { return nil, testCase.err }

			_, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder)
			if got := ErrorCodeOf(err); got != testCase.want {
				t.Fatalf("StartReceive code = %q (%v), want %q", got, err, testCase.want)
			}
			for _, call := range h.calls.snapshot() {
				switch call {
				case "network.GetLocalIP", "server.Start", "qr.EncodePNG", "network.StartBeacon":
					t.Errorf("a refused folder still reached %s", call)
				}
			}
			if got := h.state(); got != stateIdle {
				t.Errorf("state = %q after a refusal, want IDLE", got)
			}
			if got := h.observer.published(); len(got) != 0 {
				t.Errorf("a refusal published %v", kindsOf(got))
			}
			// The lease was handed back: the very next command is admitted.
			if _, err := h.stage(); err != nil {
				t.Errorf("Stage after a refused receive = %v, want it admitted", err)
			}
		})
	}
}

func TestStartReceiveIsAdmittedOnlyFromIdle(t *testing.T) {
	setups := []struct {
		name  string
		setup func(h *harness)
	}{
		{"a staged send", func(h *harness) { h.stageSuccessfully() }},
		{"a waiting receive", func(h *harness) { h.receiveWaiting() }},
		{"a transferring receive", func(h *harness) { h.receiveTransferring() }},
		{"a transferring send", func(h *harness) { h.transferring() }},
		{"a completed receive that has not been left", func(h *harness) {
			metadata := h.receiveTransferring()
			h.emit(completeEvent(metadata.SessionID, receiveProgressSnapshot(10, 10, 100)))
			h.awaitEvents(3)
			h.awaitDrainer()
		}},
	}
	for _, testCase := range setups {
		t.Run(testCase.name, func(t *testing.T) {
			h := newReceiveHarness(t)
			testCase.setup(h)
			opened := len(h.sink.opened())
			state := h.state()

			_, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder)
			if got := ErrorCodeOf(err); got != ErrBusy {
				t.Fatalf("StartReceive code = %q (%v), want busy", got, err)
			}
			if got := len(h.sink.opened()); got != opened {
				t.Errorf("a busy refusal opened %d more destinations", got-opened)
			}
			if got := h.state(); got != state {
				t.Errorf("state changed from %q to %q on a busy refusal", state, got)
			}
		})
	}
}

func TestStageIsRefusedWhileAReceiveSessionIsLive(t *testing.T) {
	h := newReceiveHarness(t)
	h.receiveWaiting()
	if _, err := h.stage(); ErrorCodeOf(err) != ErrBusy {
		t.Fatalf("Stage during a receive = %v, want busy", err)
	}
	if n := len(h.source.inspected()); n != 0 {
		t.Fatalf("Stage during a receive inspected %d paths, want 0", n)
	}
}

func TestStartReceiveRejectsBadInputsBeforeAnyStateChange(t *testing.T) {
	h := newReceiveHarness(t)
	if _, err := h.coordinator.StartReceive(context.Background(), ""); ErrorCodeOf(err) != ErrInvalidSelection {
		t.Errorf("an empty folder = %v, want invalid_selection", err)
	}
	var noContext context.Context
	if _, err := h.coordinator.StartReceive(noContext, testReceiveFolder); ErrorCodeOf(err) != ErrSetupFailed {
		t.Errorf("a nil context = %v, want setup_failed", err)
	}
	if len(h.sink.opened()) != 0 || h.state() != stateIdle {
		t.Error("a rejected input reached the sink or left IDLE")
	}
}

func TestACoordinatorBuiltWithoutASinkRefusesToReceiveButStillSends(t *testing.T) {
	h := newHarness(t)
	_, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder)
	if ErrorCodeOf(err) != ErrNotReady {
		t.Fatalf("StartReceive without a sink = %v, want not_ready", err)
	}
	if h.state() != stateIdle {
		t.Fatalf("state = %q, want IDLE", h.state())
	}
	if _, err := h.stage(); err != nil {
		t.Fatalf("Stage on a sinkless coordinator = %v, want the send path untouched", err)
	}
}

func TestStartReceiveWhileClosingIsRefused(t *testing.T) {
	h := newReceiveHarness(t)
	if err := h.coordinator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown = %v", err)
	}
	if _, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder); ErrorCodeOf(err) != ErrShuttingDown {
		t.Fatalf("StartReceive after Shutdown = %v, want shutting_down", err)
	}
	if len(h.sink.opened()) != 0 {
		t.Fatal("a closing application opened a destination")
	}
}

// Every acquired resource unwinds on a late setup failure, the destination
// included: a stranded destination handle would be a leaked descriptor.
func TestAFailedStartAfterTheDestinationOpenedClosesItExactlyOnce(t *testing.T) {
	cases := map[string]func(h *harness){
		"the server fails to start": func(h *harness) {
			h.server.start = func(context.Context, ServerStartRequest, ClaimAuthorizer) (ServerHandle, error) {
				return ServerHandle{}, NewError(ErrServerStartFailed, "bind failed")
			}
		},
		"the QR encoder fails": func(h *harness) {
			h.qr.encode = func(context.Context, string) ([]byte, error) { return nil, NewError(ErrQRFailed, "no image") }
		},
		"no address is available": func(h *harness) {
			h.network.getLocalIP = func(context.Context) (netip.Addr, error) {
				return netip.Addr{}, NewError(ErrNetworkUnavailable, "offline")
			}
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			h := newReceiveHarness(t)
			arrange(h)

			if _, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder); err == nil {
				t.Fatal("StartReceive succeeded with a failing dependency")
			}
			if got := h.sink.destination(0).closeCalls(); got != 1 {
				t.Errorf("the destination was closed %d times, want exactly 1", got)
			}
			if got := h.state(); got != stateIdle {
				t.Errorf("state = %q, want IDLE", got)
			}
			if got := h.observer.published(); len(got) != 0 {
				t.Errorf("a failed start published %v", kindsOf(got))
			}
		})
	}
}

func TestABeaconFailureIsAWarningForAReceiveSessionToo(t *testing.T) {
	h := newReceiveHarness(t)
	h.network.startBeacon = func(context.Context, BeaconRequest) error { return NewError(ErrBeaconWarning, "mdns down") }

	metadata := h.receiveWaiting()
	if len(metadata.Warnings) != 1 || metadata.Warnings[0].Code != WarnBeaconUnavailable {
		t.Fatalf("Warnings = %+v, want the single beacon warning", metadata.Warnings)
	}
	if h.state() != stateStaged {
		t.Fatalf("state = %q, want a usable STAGED session", h.state())
	}
}

// --- Cancel before the claim ----------------------------------------------

func TestCancellingAWaitingReceiveWritesNothingAndResets(t *testing.T) {
	h := newReceiveHarness(t)
	h.receiveWaiting()

	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatalf("Cancel = %v", err)
	}
	events := h.observer.published()
	if got := kindsOf(events); len(got) != 1 || got[0] != TransferReset {
		t.Fatalf("events = %v, want exactly one reset", got)
	}
	if events[0].Seq != 1 || events[0].Receive != nil || events[0].Error != nil {
		t.Errorf("reset event = %+v, want seq 1 and no outcome", events[0])
	}
	if got := h.sink.destination(0).closeCalls(); got != 1 {
		t.Errorf("the destination was closed %d times, want 1", got)
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state = %q, want IDLE", got)
	}
	// Teardown releases in reverse acquisition order: the destination last.
	var teardown []string
	for _, call := range h.calls.snapshot() {
		switch call {
		case "network.StopBeacon", "server.Stop", "destination.Close":
			teardown = append(teardown, call)
		}
	}
	if strings.Join(teardown, ",") != "network.StopBeacon,server.Stop,destination.Close" {
		t.Errorf("teardown order = %v, want beacon, server, destination", teardown)
	}
}

// --- The terminal outcome --------------------------------------------------

func TestACompleteUploadPublishesProgressThenAnOutcomeAndHoldsIt(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveTransferring()
	id := metadata.SessionID
	h.sink.destination(0).finalResult(ReceiveResult{FilesSaved: 3, SubfolderExists: true})

	h.emit(receiveProgressEvent(id, receiveProgressSnapshot(1024, 4096, 25), 2))
	h.awaitEvents(2)
	h.emit(completeEvent(id, receiveProgressSnapshot(4096, 4096, 100)))
	events := h.awaitEvents(4)
	h.awaitDrainer()

	want := []EventKind{TransferStarted, TransferProgress, TransferProgress, TransferComplete}
	if got := kindsOf(events); strings.Join(kindStrings(got), ",") != strings.Join(kindStrings(want), ",") {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for index, event := range events {
		if event.Seq != uint64(index+1) {
			t.Errorf("event %d has seq %d, want %d", index, event.Seq, index+1)
		}
	}
	if got := events[1].Receive; got == nil || *got != (ReceiveStatus{FilesSaved: 2}) {
		t.Errorf("mid-upload progress Receive = %+v, want filesSaved 2 only", got)
	}
	if got := events[2].Receive; got == nil || *got != (ReceiveStatus{FilesSaved: 3}) {
		t.Errorf("final progress Receive = %+v, want the destination's final count 3", got)
	}
	outcome := events[3].Receive
	if outcome == nil || *outcome != (ReceiveStatus{FilesSaved: 3, Result: "complete", SubfolderExists: true}) {
		t.Errorf("complete Receive = %+v, want 3 files, complete, subfolder exists", outcome)
	}
	if events[3].Progress == nil || events[3].Progress.BytesSent != 4096 || events[3].Progress.TotalBytes != 4096 {
		t.Errorf("complete Progress = %+v, want 4096 of 4096", events[3].Progress)
	}

	if got := h.state(); got != stateDone {
		t.Errorf("state = %q, want DONE", got)
	}
	if got := h.timer.armed(); got != 0 {
		t.Errorf("a receive outcome armed %d resets, want it held until the user leaves it", got)
	}
	if got := h.sink.destination(0).closeCalls(); got != 1 {
		t.Errorf("the destination was closed %d times, want 1", got)
	}
}

func kindStrings(kinds []EventKind) []string {
	out := make([]string, len(kinds))
	for index, kind := range kinds {
		out[index] = string(kind)
	}
	return out
}

func TestAHeldReceiveOutcomeServesShowInFolderUntilTheUserLeavesIt(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveTransferring()
	h.sink.destination(0).finalResult(ReceiveResult{FilesSaved: 1, SubfolderExists: true})
	h.emit(completeEvent(metadata.SessionID, receiveProgressSnapshot(10, 10, 100)))
	h.awaitEvents(3)
	h.awaitDrainer()

	folder, err := h.coordinator.ReceivedFolder()
	if err != nil || folder != testSubfolderPath {
		t.Fatalf("ReceivedFolder = %q, %v, want %q", folder, err, testSubfolderPath)
	}

	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatalf("Cancel = %v", err)
	}
	events := h.observer.published()
	if last := events[len(events)-1]; last.Kind != TransferReset || last.Seq != 4 {
		t.Fatalf("last event = %+v, want reset with seq 4", last)
	}
	if got := h.state(); got != stateIdle {
		t.Fatalf("state = %q, want IDLE", got)
	}
	if _, err := h.coordinator.ReceivedFolder(); ErrorCodeOf(err) != ErrPathNotFound {
		t.Fatalf("ReceivedFolder after leaving = %v, want path_not_found", err)
	}
	// And the coordinator admits the next command.
	if _, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder); err != nil {
		t.Fatalf("StartReceive after leaving the outcome = %v", err)
	}
}

func TestReceivedFolderRefusesWhenThereIsNoSubfolder(t *testing.T) {
	h := newReceiveHarness(t)
	if _, err := h.coordinator.ReceivedFolder(); ErrorCodeOf(err) != ErrPathNotFound {
		t.Fatalf("ReceivedFolder with no session = %v, want path_not_found", err)
	}
	h.receiveWaiting()
	h.sink.destination(0).mu.Lock()
	h.sink.destination(0).folderGone = true
	h.sink.destination(0).mu.Unlock()
	if _, err := h.coordinator.ReceivedFolder(); ErrorCodeOf(err) != ErrPathNotFound {
		t.Fatalf("ReceivedFolder before any file was saved = %v, want path_not_found", err)
	}
	h2 := newReceiveHarness(t)
	h2.stageSuccessfully()
	if _, err := h2.coordinator.ReceivedFolder(); ErrorCodeOf(err) != ErrPathNotFound {
		t.Fatalf("ReceivedFolder for a send session = %v, want path_not_found", err)
	}
}

func TestAnIncompleteUploadPublishesAnErrorWithTheExactSavedCount(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveTransferring()
	h.sink.destination(0).finalResult(ReceiveResult{FilesSaved: 2, SubfolderExists: true, MarkingFailed: true})

	snapshot := receiveProgressSnapshot(3000, 9000, 33.3)
	h.emit(failedEvent(metadata.SessionID, &snapshot, NewError(ErrTransferFailed, "connection dropped")))
	events := h.awaitEvents(3)
	h.awaitDrainer()

	if got := strings.Join(kindStrings(kindsOf(events)), ","); got != "transfer-started,transfer-progress,transfer-error" {
		t.Fatalf("events = %s, want started, the final progress, then the error", got)
	}
	last := events[len(events)-1]
	if last.Kind != TransferError {
		t.Fatalf("last event = %q, want transfer-error", last.Kind)
	}
	if last.Error == nil || last.Error.Code != ErrTransferFailed {
		t.Errorf("Error = %+v, want transfer_failed", last.Error)
	}
	want := ReceiveStatus{FilesSaved: 2, Result: "incomplete", SubfolderExists: true, MarkingWarning: true}
	if last.Receive == nil || *last.Receive != want {
		t.Errorf("Receive = %+v, want %+v", last.Receive, want)
	}
	if h.timer.armed() != 0 {
		t.Error("an incomplete receive armed a reset, want it held")
	}
	if folder, err := h.coordinator.ReceivedFolder(); err != nil || folder == "" {
		t.Errorf("ReceivedFolder after an incomplete upload = %q, %v, want the kept files reachable", folder, err)
	}
}

func TestAnUploadThatSavedNothingReportsIncompleteWithoutASubfolder(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveTransferring()
	h.sink.destination(0).finalResult(ReceiveResult{})

	h.emit(failedEvent(metadata.SessionID, nil, NewError(ErrTransferFailed, "dropped before the first file")))
	events := h.awaitEvents(2)
	h.awaitDrainer()

	last := events[len(events)-1]
	want := ReceiveStatus{Result: "incomplete"}
	if last.Kind != TransferError || last.Receive == nil || *last.Receive != want {
		t.Fatalf("last event = %+v, want a transfer-error with %+v", last, want)
	}
}

func TestALaneThatClosesBeforeAnyClaimFailsAWaitingReceive(t *testing.T) {
	h := newReceiveHarness(t)
	h.receiveWaiting()
	h.server.closeEvents()
	events := h.awaitEvents(1)
	h.awaitDrainer()

	if events[0].Kind != TransferError || events[0].Receive == nil || events[0].Receive.Result != "incomplete" || events[0].Receive.FilesSaved != 0 {
		t.Fatalf("event = %+v, want an incomplete outcome with nothing saved", events[0])
	}
}

// --- Desktop Cancel mid-upload ---------------------------------------------

func TestCancellingAnUploadInFlightPublishesACancelledOutcomeAndHoldsIt(t *testing.T) {
	h := newReceiveHarness(t)
	h.receiveTransferring()
	h.sink.destination(0).finalResult(ReceiveResult{FilesSaved: 2, SubfolderExists: true})

	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatalf("Cancel = %v", err)
	}
	events := h.observer.published()
	if got := strings.Join(kindStrings(kindsOf(events)), ","); got != "transfer-started,transfer-error" {
		t.Fatalf("events = %s, want started then one error and no reset", got)
	}
	outcome := events[1]
	if outcome.Seq != 2 {
		t.Errorf("outcome seq = %d, want 2", outcome.Seq)
	}
	if outcome.Error == nil || outcome.Error.Code != ErrCancelled || outcome.Error.Message != "Transfer canceled." {
		t.Errorf("Error = %+v, want the cancelled code and its fixed copy", outcome.Error)
	}
	want := ReceiveStatus{FilesSaved: 2, Result: "cancelled", SubfolderExists: true}
	if outcome.Receive == nil || *outcome.Receive != want {
		t.Errorf("Receive = %+v, want %+v", outcome.Receive, want)
	}
	if got := h.state(); got != stateError {
		t.Errorf("state = %q, want the outcome held in ERROR", got)
	}
	if h.timer.armed() != 0 {
		t.Error("a cancelled receive armed a reset, want it held")
	}
	if got := h.sink.destination(0).closeCalls(); got != 1 {
		t.Errorf("the destination was closed %d times, want 1", got)
	}
	if folder, err := h.coordinator.ReceivedFolder(); err != nil || folder != testSubfolderPath {
		t.Errorf("ReceivedFolder = %q, %v, want the saved files reachable after a cancel", folder, err)
	}

	// Leaving the held outcome is a second Cancel, which resets.
	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatalf("second Cancel = %v", err)
	}
	events = h.observer.published()
	if last := events[len(events)-1]; last.Kind != TransferReset || last.Seq != 3 {
		t.Fatalf("last event = %+v, want a reset with seq 3", last)
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state = %q, want IDLE", got)
	}
	if got := h.sink.destination(0).closeCalls(); got != 1 {
		t.Errorf("the destination was closed %d times after leaving, want still 1", got)
	}
}

// Cancel racing a terminal report: whichever linearizes first wins, and the
// session publishes exactly one terminal outcome either way. The distribution is
// logged; each branch is forced deterministically elsewhere.
func TestACancelRacingACompletionPublishesExactlyOneTerminalOutcome(t *testing.T) {
	complete, cancelled := 0, 0
	for round := 0; round < 40; round++ {
		h := newReceiveHarness(t)
		// The completion is published through the fake's synchronized producer,
		// as a live handler publishes through the real lane: a raw send on the
		// lane raced Cancel's close of it, which the race detector reports.
		h.bufferLane(1)
		metadata := h.receiveTransferring()
		h.sink.destination(0).finalResult(ReceiveResult{FilesSaved: 1, SubfolderExists: true})

		done := make(chan error, 1)
		go func() { done <- h.coordinator.Cancel(context.Background()) }()
		h.server.publish(completeEvent(metadata.SessionID, receiveProgressSnapshot(10, 10, 100)))
		if err := <-done; err != nil {
			t.Fatalf("Cancel = %v", err)
		}

		terminals := 0
		for _, event := range h.observer.published() {
			switch event.Kind {
			case TransferComplete:
				terminals++
				complete++
			case TransferError:
				terminals++
				cancelled++
				if event.Receive == nil || event.Receive.Result != "cancelled" {
					t.Fatalf("an error outcome was %+v, want only a cancelled one in this race", event.Receive)
				}
			}
		}
		if terminals > 1 {
			t.Fatalf("round %d published %d terminal outcomes, want at most 1", round, terminals)
		}
		if got := h.sink.destination(0).closeCalls(); got != 1 {
			t.Fatalf("round %d closed the destination %d times, want 1", round, got)
		}
	}
	t.Logf("race distribution over 40 rounds: complete=%d cancelled=%d", complete, cancelled)
}

// A cancel that lands while the claim is still committing wrote nothing, so it
// resets as a plain cancel does and publishes no outcome.
func TestCancellingDuringTheClaimPublishesNoOutcome(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveWaiting()

	h.clock.onNow = func() {
		h.clock.onNow = nil
		go func() { _ = h.coordinator.Cancel(context.Background()) }()
		h.awaitCancelled()
	}
	err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID)
	if ErrorCodeOf(err) != ErrCancelled {
		t.Fatalf("AuthorizeClaim during a cancel = %v, want cancelled", err)
	}
	h.eventually("the cancel's reset", func() bool {
		for _, event := range h.observer.published() {
			if event.Kind == TransferReset {
				return true
			}
		}
		return false
	})
	for _, event := range h.observer.published() {
		if event.Kind == TransferError || event.Kind == TransferComplete || event.Receive != nil {
			t.Errorf("a cancel before the claim committed published %+v", event)
		}
	}
}

func TestShutdownDuringAnUploadClosesTheDestinationAndPublishesNothing(t *testing.T) {
	h := newReceiveHarness(t)
	h.receiveTransferring()

	if err := h.coordinator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown = %v", err)
	}
	if got := kindsOf(h.observer.published()); len(got) != 1 || got[0] != TransferStarted {
		t.Fatalf("events = %v, want only the started event", got)
	}
	if got := h.sink.destination(0).closeCalls(); got != 1 {
		t.Fatalf("the destination was closed %d times, want 1", got)
	}
	if got := h.state(); got != stateIdle {
		t.Fatalf("state = %q, want IDLE", got)
	}
}

func TestShutdownFromAHeldReceiveOutcomeIsQuietAndIdempotent(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveTransferring()
	h.emit(completeEvent(metadata.SessionID, receiveProgressSnapshot(10, 10, 100)))
	h.awaitEvents(3)
	h.awaitDrainer()

	before := len(h.observer.published())
	for round := 0; round < 2; round++ {
		if err := h.coordinator.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown round %d = %v", round, err)
		}
	}
	if after := len(h.observer.published()); after != before {
		t.Fatalf("Shutdown published %d events, want none", after-before)
	}
}

// --- Notices ---------------------------------------------------------------

func TestARefusedUploadNoticeReachesTheDesktopWhileTheSessionKeepsWaiting(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveWaiting()

	h.emit(ServerEvent{SessionID: metadata.SessionID, Kind: ServerNotice, Notice: NoticeReceiveTooLarge})
	events := h.awaitEvents(1)

	if events[0].Kind != TransferNotice || events[0].Seq != 1 || events[0].Notice != "receive_too_large" {
		t.Fatalf("event = %+v, want a seq 1 transfer-notice receive_too_large", events[0])
	}
	if events[0].Progress != nil || events[0].Error != nil || events[0].Receive != nil {
		t.Errorf("notice carried a payload it should not: %+v", events[0])
	}
	if got := h.state(); got != stateStaged {
		t.Errorf("state = %q after a notice, want STAGED", got)
	}

	// A second refusal is a second, strictly sequenced notice.
	h.emit(ServerEvent{SessionID: metadata.SessionID, Kind: ServerNotice, Notice: NoticeReceiveTooLarge})
	events = h.awaitEvents(2)
	if events[1].Seq != 2 {
		t.Errorf("second notice seq = %d, want 2", events[1].Seq)
	}
	// The session is still claimable.
	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		t.Fatalf("AuthorizeClaim after notices = %v", err)
	}
}

func TestANoticeAfterTheClaimIsDroppedAndAnUnknownOneIsADiagnostic(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveTransferring()
	h.emit(ServerEvent{SessionID: metadata.SessionID, Kind: ServerNotice, Notice: NoticeReceiveTooLarge})
	h.emit(receiveProgressEvent(metadata.SessionID, receiveProgressSnapshot(1, 10, 10), 0))
	events := h.awaitEvents(2)
	if got := kindsOf(events); got[1] != TransferProgress {
		t.Fatalf("events = %v, want the notice dropped and the progress published", got)
	}

	h2 := newReceiveHarness(t)
	waiting := h2.receiveWaiting()
	h2.emit(ServerEvent{SessionID: waiting.SessionID, Kind: ServerNotice, Notice: "something_else"})
	h2.emit(receiveProgressEvent(waiting.SessionID, receiveProgressSnapshot(1, 10, 10), 0)) // taken once the notice was processed
	if got := h2.observer.published(); len(got) != 0 {
		t.Errorf("an unrecognized notice published %v", kindsOf(got))
	}
	if len(h2.diagnosed.snapshot()) == 0 {
		t.Error("an unrecognized notice left no diagnostic")
	}
}

func TestASendSessionNeverPublishesANotice(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.stageSuccessfully()
	h.emit(ServerEvent{SessionID: metadata.SessionID, Kind: ServerNotice, Notice: NoticeReceiveTooLarge})
	h.emit(progressEvent(metadata.SessionID, testProgress(1, 1))) // taken once the notice was processed
	if got := h.observer.published(); len(got) != 0 {
		t.Fatalf("a send session published %v for a receive notice", kindsOf(got))
	}
}

func TestReceiveProgressCarriesOnlyANonNegativeSavedCount(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveTransferring()
	h.emit(receiveProgressEvent(metadata.SessionID, receiveProgressSnapshot(5, 10, 50), -7))
	events := h.awaitEvents(2)
	if got := events[1].Receive; got == nil || got.FilesSaved != 0 {
		t.Fatalf("Receive = %+v, want a negative count clamped to 0", got)
	}
}

// --- Disclosure -----------------------------------------------------------

func TestNothingThatLeavesTheCoordinatorNamesTheFolder(t *testing.T) {
	h := newReceiveHarness(t)
	metadata := h.receiveTransferring()
	h.sink.destination(0).finalResult(ReceiveResult{FilesSaved: 1, SubfolderExists: true})
	h.emit(receiveProgressEvent(metadata.SessionID, receiveProgressSnapshot(5, 10, 50), 1))
	h.awaitEvents(2)
	h.emit(failedEvent(metadata.SessionID, nil, NewError(ErrTransferFailed, testReceiveFolder+" "+string(testToken))))
	h.awaitEvents(3)
	h.awaitDrainer()

	encoded, err := json.Marshal(h.observer.published())
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"/Users/receiver", "Phone Photos", "FairDrop 2026", string(testToken)} {
		if strings.Contains(string(encoded), leak) {
			t.Errorf("an event disclosed %q: %s", leak, encoded)
		}
	}
	for _, entry := range h.diagnosed.snapshot() {
		if strings.Contains(entry.message, "Users") || strings.Contains(entry.message, "Phone") || strings.Contains(entry.message, string(testToken)) {
			t.Errorf("a diagnostic disclosed the folder or token: %q", entry.message)
		}
	}
}

// --- Bounds -----------------------------------------------------------------

// A destination whose Close never returns must be reported, never absorbed, and
// must not stop the coordinator reaching IDLE.
func TestADestinationThatNeverClosesIsReportedNotAbsorbed(t *testing.T) {
	h := newReceiveHarness(t)
	h.receiveWaiting()
	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) })
	dest := h.sink.destination(0)
	dest.mu.Lock()
	dest.closeBlocked = blocked
	dest.mu.Unlock()

	baseline := h.bounds.armed()
	done := make(chan error, 1)
	go func() { done <- h.coordinator.Cancel(context.Background()) }()

	// The beacon's and the server's bounds resolve on their own; the
	// destination's is the third, and the one forced here.
	h.awaitCalls("destination.Close")
	h.awaitBoundResolvedAt(baseline + 1)
	h.awaitBoundPending(baseline + 2)
	h.bounds.fire()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "destination folder did not confirm") {
			t.Fatalf("Cancel = %v, want a failure naming the destination", err)
		}
		if ErrorCodeOf(err) != ErrCleanupUnconfirmed {
			t.Errorf("Cancel code = %q, want cleanup_unconfirmed for a session that never began", ErrorCodeOf(err))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Cancel never returned")
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state = %q, want the coordinator IDLE despite the stuck destination", got)
	}
}

// A hung chooser path (a dead network volume) must not leave the user unable to
// cancel, and a destination that eventually opens late must not be leaked.
func TestADestinationThatNeverOpensTimesOutAndALateOneIsClosed(t *testing.T) {
	h := newReceiveHarness(t)
	release := make(chan struct{})
	entered := make(chan struct{})
	h.sink.open = func(context.Context, string) (ReceiveDestination, error) {
		close(entered)
		<-release
		return h.sink.newDestination(), nil
	}

	baseline := h.bounds.armed()
	done := make(chan error, 1)
	go func() {
		_, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder)
		done <- err
	}()
	<-entered
	h.awaitBoundPending(baseline)
	h.bounds.fire()

	select {
	case err := <-done:
		if ErrorCodeOf(err) != ErrSetupFailed {
			t.Fatalf("StartReceive = %v, want setup_failed", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("StartReceive never returned")
	}
	if got := h.state(); got != stateIdle {
		t.Fatalf("state = %q, want IDLE", got)
	}

	close(release)
	h.eventually("the late destination to be closed", func() bool {
		h.sink.mu.Lock()
		defer h.sink.mu.Unlock()
		return len(h.sink.dests) == 1 && h.sink.dests[0].closeCalls() == 1
	})
}

// Cancel landing while the folder is still being validated must stop the start,
// close whatever the validation produced, and publish nothing: the session never
// reached its acknowledgement.
func TestCancellingWhileTheFolderIsBeingValidatedAbortsTheStart(t *testing.T) {
	h := newReceiveHarness(t)
	entered := make(chan struct{})
	h.sink.open = func(ctx context.Context, _ string) (ReceiveDestination, error) {
		close(entered)
		<-ctx.Done()
		dest := h.sink.newDestination()
		return dest, nil
	}

	started := make(chan error, 1)
	go func() {
		_, err := h.coordinator.StartReceive(context.Background(), testReceiveFolder)
		started <- err
	}()
	<-entered

	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatalf("Cancel = %v", err)
	}
	select {
	case err := <-started:
		if ErrorCodeOf(err) != ErrCancelled {
			t.Fatalf("StartReceive = %v, want cancelled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("StartReceive never returned after Cancel")
	}
	if got := h.sink.destination(0).closeCalls(); got != 1 {
		t.Errorf("the destination was closed %d times, want 1", got)
	}
	if got := h.observer.published(); len(got) != 0 {
		t.Errorf("an abandoned start published %v", kindsOf(got))
	}
	if got := h.state(); got != stateIdle {
		t.Errorf("state = %q, want IDLE", got)
	}
	for _, call := range h.calls.snapshot() {
		switch call {
		case "network.GetLocalIP", "server.Start", "qr.EncodePNG", "network.StartBeacon":
			t.Errorf("an abandoned start still reached %s", call)
		}
	}
}

// A caller whose own context ends mid-start gets the same unwind: the session
// context outlives the call, but setup does not.
func TestAnAbandonedCommandContextAbortsTheStartAfterTheFolderOpens(t *testing.T) {
	h := newReceiveHarness(t)
	ctx, cancel := context.WithCancel(context.Background())
	h.sink.open = func(context.Context, string) (ReceiveDestination, error) {
		cancel()
		return h.sink.newDestination(), nil
	}

	_, err := h.coordinator.StartReceive(ctx, testReceiveFolder)
	if ErrorCodeOf(err) != ErrCancelled {
		t.Fatalf("StartReceive = %v, want cancelled", err)
	}
	if got := h.sink.destination(0).closeCalls(); got != 1 {
		t.Errorf("the destination was closed %d times, want 1", got)
	}
	if h.state() != stateIdle || len(h.server.startRequests()) != 0 {
		t.Errorf("state %q with %d server starts, want IDLE and none", h.state(), len(h.server.startRequests()))
	}
}
