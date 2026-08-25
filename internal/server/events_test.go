package server

import (
	"testing"

	"fairdrop/internal/transfer"
)

// TestLaneDropsProgressRatherThanBlocking is the deadlock the whole lane
// exists to prevent: the coordinator's drainer may not be running yet, and a
// handler that blocked on a send would hold a payload and a connection open
// waiting for a reader that does not exist.
func TestLaneDropsProgressRatherThanBlocking(t *testing.T) {
	t.Parallel()

	lane := newEventLane()
	for index := range laneCapacity {
		if !lane.publishProgress(snapshotEvent(int64(index))) {
			t.Fatalf("progress %d was dropped before the lane was full", index)
		}
	}
	if lane.publishProgress(snapshotEvent(laneCapacity)) {
		t.Fatal("a full lane accepted another progress event")
	}
}

// TestTerminalEventMakesRoomForItself pins the reserved-capacity rule: a
// terminal outcome is not droppable, so it evicts the oldest expendable
// snapshot rather than waiting for a consumer.
func TestTerminalEventMakesRoomForItself(t *testing.T) {
	t.Parallel()

	lane := newEventLane()
	for index := range laneCapacity {
		lane.publishProgress(snapshotEvent(int64(index)))
	}

	if !lane.publishTerminal(completeEvent(testSession, transfer.ProgressSnapshot{BytesSent: 99})) {
		t.Fatal("the terminal event was dropped by a full lane")
	}

	lane.close()
	var collected []transfer.ServerEvent
	for event := range lane.channel() {
		collected = append(collected, event)
	}
	if len(collected) != laneCapacity {
		t.Fatalf("lane held %d events, want %d", len(collected), laneCapacity)
	}
	last := collected[len(collected)-1]
	if last.Kind != transfer.ServerComplete {
		t.Fatalf("last event = %s, want %s", last.Kind, transfer.ServerComplete)
	}
	// The oldest snapshot was the one discarded.
	if first := collected[0]; first.Progress.BytesSent != 1 {
		t.Fatalf("oldest surviving snapshot = %d bytes, want the second one", first.Progress.BytesSent)
	}
}

// TestNothingFollowsTheTerminalEvent covers the grammar rule directly: exactly
// one terminal event per outcome, and nothing after it.
func TestNothingFollowsTheTerminalEvent(t *testing.T) {
	t.Parallel()

	lane := newEventLane()
	if !lane.publishTerminal(completeEvent(testSession, transfer.ProgressSnapshot{})) {
		t.Fatal("the first terminal event was dropped")
	}
	if lane.publishTerminal(failedEvent(testSession, transfer.ProgressSnapshot{}, errStub)) {
		t.Fatal("a second terminal event was delivered")
	}
	if lane.publishProgress(snapshotEvent(1)) {
		t.Fatal("progress was delivered after the terminal event")
	}

	lane.close()
	count := 0
	for range lane.channel() {
		count++
	}
	if count != 1 {
		t.Fatalf("lane delivered %d events, want exactly 1", count)
	}
}

// TestClosingTheLaneIsIdempotentAndPermanent backs the Stop postcondition: the
// channel is closed once and never reopened, and a late publisher cannot panic
// Stop by sending into it.
func TestClosingTheLaneIsIdempotentAndPermanent(t *testing.T) {
	t.Parallel()

	lane := newEventLane()
	lane.close()
	lane.close()

	if lane.publishProgress(snapshotEvent(1)) {
		t.Fatal("a closed lane accepted progress")
	}
	if lane.publishTerminal(completeEvent(testSession, transfer.ProgressSnapshot{})) {
		t.Fatal("a closed lane accepted a terminal event")
	}
	if _, open := <-lane.channel(); open {
		t.Fatal("the lane reopened")
	}
}

// TestFailedEventCarriesProgressOnlyWhenBytesMoved keeps a failure that never
// sent a byte from being published as a transfer that stalled at zero.
func TestFailedEventCarriesProgressOnlyWhenBytesMoved(t *testing.T) {
	t.Parallel()

	none := failedEvent(testSession, transfer.ProgressSnapshot{}, errStub)
	if none.Progress != nil {
		t.Fatalf("failure before the first byte carried %+v", none.Progress)
	}
	if none.Err == nil || none.Kind != transfer.ServerFailed || none.SessionID != testSession {
		t.Fatalf("failed event = %+v, want a coded failure for the session", none)
	}

	some := failedEvent(testSession, transfer.ProgressSnapshot{BytesSent: 7, TotalKnown: true, TotalBytes: 10}, errStub)
	if some.Progress == nil || some.Progress.BytesSent != 7 {
		t.Fatalf("failure after 7 bytes carried %+v", some.Progress)
	}
}

var errStub = transfer.NewError(transfer.ErrTransferFailed, "stub failure")

func snapshotEvent(bytesSent int64) transfer.ServerEvent {
	return progressEvent(testSession, transfer.ProgressSnapshot{BytesSent: bytesSent})
}
