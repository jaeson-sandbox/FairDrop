package transfer

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func receiveTextWaiting(t *testing.T, h *harness) ReceiveTextMetadata {
	t.Helper()
	metadata, err := h.coordinator.StartReceiveText(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if live := h.liveSession(); live != nil {
		h.track(live)
	}
	return metadata
}

func TestReceiveTextCallerCancellationAtClaimCommitLeavesSessionReusable(t *testing.T) {
	h := newHarness(t)
	metadata := receiveTextWaiting(t, h)
	ctx, cancel := context.WithCancel(context.Background())
	h.clock.onNow = cancel // Runs after CLAIMING admission, before its second context check.
	if err := h.coordinator.AuthorizeClaim(ctx, metadata.SessionID); ErrorCodeOf(err) != ErrCancelled {
		t.Fatalf("cancelled claim = %v, want cancelled", err)
	}
	h.clock.onNow = nil
	if got := h.state(); got != stateStaged {
		t.Fatalf("cancelled claim left state %s, want reusable Staged", got)
	}
	if h.coordinator.leaseHeld() || len(h.observer.published()) != 0 {
		t.Fatal("cancelled claim retained lease or published a started event")
	}
	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		t.Fatalf("next claim of still-live session = %v", err)
	}
}

func TestReceiveTextStartsEmptyWithoutSourceSinkOrPayloadMetadata(t *testing.T) {
	h := newHarness(t)
	metadata := receiveTextWaiting(t, h)
	if metadata.SessionID != testSessionID || metadata.URL != "http://192.168.1.50:45678/receive-text/1112131415161718191a1b1c1d1e1f20" ||
		metadata.Warnings == nil || len(metadata.Warnings) != 0 {
		t.Fatalf("metadata = %+v", metadata)
	}
	if h.state() != stateStaged || h.calls.count("sink.Open") != 0 || h.calls.count("source.Inspect") != 0 ||
		h.calls.count("source.Walk") != 0 {
		t.Fatalf("state=%s calls=%v", h.state(), h.calls.snapshot())
	}
	request := h.server.startRequests()[0]
	if !request.ReceiveText || request.Destination != nil || request.Text != nil || request.Item != (StagedItem{}) {
		t.Fatalf("server mode = %+v", request)
	}
	if _, err := h.coordinator.GetReceivedText(metadata.SessionID); err == nil {
		t.Fatal("uncommitted text was readable")
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "value") || strings.Contains(string(encoded), "text\":") {
		t.Fatalf("metadata disclosed a value: %s", encoded)
	}
}

func TestReceiveTextCommitReadHeldOutcomeAndDoneClear(t *testing.T) {
	h := newHarness(t)
	metadata := receiveTextWaiting(t, h)
	if err := h.coordinator.CommitReceivedText(context.Background(), metadata.SessionID, "premature"); err == nil {
		t.Fatal("commit before claim succeeded")
	}
	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		t.Fatal(err)
	}
	value := "\r\n<unsafe>\r雪"
	if err := h.coordinator.CommitReceivedText(context.Background(), metadata.SessionID, value); err != nil {
		t.Fatal(err)
	}
	if _, err := h.coordinator.GetReceivedText(metadata.SessionID); err == nil {
		t.Fatal("value visible before terminal receipt")
	}
	earlyRead := make(chan struct {
		value string
		err   error
	}, 1)
	h.observer.publish = func(event Event) {
		if event.Kind == TransferComplete {
			value, err := h.coordinator.GetReceivedText(metadata.SessionID)
			earlyRead <- struct {
				value string
				err   error
			}{value, err}
		}
	}
	h.emit(ServerEvent{SessionID: metadata.SessionID, Kind: ServerComplete, Progress: &ProgressSnapshot{}})
	h.awaitEvents(2)
	h.awaitDrainer()
	select {
	case read := <-earlyRead:
		if read.err != nil || read.value != "\n<unsafe>\n雪" {
			t.Fatalf("read at terminal publication = %q, %v", read.value, read.err)
		}
	default:
		t.Fatal("terminal observer did not fetch the committed value")
	}
	got, err := h.coordinator.GetReceivedText(metadata.SessionID)
	if err != nil || got != "\n<unsafe>\n雪" {
		t.Fatalf("GetReceivedText = %q, %v", got, err)
	}
	if h.state() != stateDone || h.timer.armed() != 0 {
		t.Fatalf("outcome not held: state=%s", h.state())
	}
	for _, event := range h.observer.published() {
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "unsafe") || strings.Contains(string(encoded), "雪") {
			t.Fatalf("event disclosed text: %s", encoded)
		}
	}
	live := h.liveSession()
	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.coordinator.GetReceivedText(metadata.SessionID); err == nil {
		t.Fatal("Done retained readable text")
	}
	if live.receiveText.value != nil || live.receiveText.success {
		t.Fatal("retired session retained RAM value")
	}
}

func TestReceiveTextFailureAndCancellationNeverExposeValue(t *testing.T) {
	for _, failAfterCommit := range []bool{false, true} {
		h := newHarness(t)
		metadata := receiveTextWaiting(t, h)
		if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
			t.Fatal(err)
		}
		if failAfterCommit {
			if err := h.coordinator.CommitReceivedText(context.Background(), metadata.SessionID, "never expose me"); err != nil {
				t.Fatal(err)
			}
		}
		h.emit(ServerEvent{SessionID: metadata.SessionID, Kind: ServerFailed, Err: NewError(ErrTransferFailed, "failed")})
		h.awaitEvents(2)
		h.awaitDrainer()
		if _, err := h.coordinator.GetReceivedText(metadata.SessionID); err == nil {
			t.Fatal("failed text was readable")
		}
		if live := h.liveSession(); live.receiveText.value != nil {
			t.Fatal("failure retained value")
		}
	}
	h := newHarness(t)
	metadata := receiveTextWaiting(t, h)
	if err := h.coordinator.Cancel(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := h.coordinator.CommitReceivedText(context.Background(), metadata.SessionID, "late"); err == nil {
		t.Fatal("late commit succeeded")
	}
}

func TestReceiveTextServerCompleteWithoutCommitBecomesFailure(t *testing.T) {
	h := newHarness(t)
	metadata := receiveTextWaiting(t, h)
	if err := h.coordinator.AuthorizeClaim(context.Background(), metadata.SessionID); err != nil {
		t.Fatal(err)
	}
	h.emit(ServerEvent{SessionID: metadata.SessionID, Kind: ServerComplete, Progress: &ProgressSnapshot{}})
	events := h.awaitEvents(2)
	h.awaitDrainer()
	if events[len(events)-1].Kind != TransferError || h.state() != stateError {
		t.Fatalf("empty completion was accepted: %v %s", kindsOf(events), h.state())
	}
	if _, err := h.coordinator.GetReceivedText(metadata.SessionID); err == nil {
		t.Fatal("empty completion exposed text")
	}
}

func TestReceiveTextRecoveryRetiresOldCodeAndStartsEmpty(t *testing.T) {
	h := newHarness(t)
	metadata := receiveTextWaiting(t, h)
	result, err := h.coordinator.RefreshConnection(context.Background(), metadata.SessionID, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.Mode != "receive-text" || result.ReceiveText == nil || result.Send != nil || result.Receive != nil || result.Text != nil ||
		result.ReceiveText.SessionID == metadata.SessionID || result.ReceiveText.URL == metadata.URL {
		t.Fatalf("recovery = %+v", result)
	}
	if _, err := h.coordinator.GetReceivedText(metadata.SessionID); err == nil {
		t.Fatal("old ID was readable")
	}
	if _, err := h.coordinator.GetReceivedText(result.ReceiveText.SessionID); err == nil {
		t.Fatal("new empty receiver was readable")
	}
}
