package server

import (
	"sync"

	"fairdrop/internal/transfer"
)

// laneCapacity is the buffered depth of the event channel.
//
// It is a latency budget, not a queue: progress is capped at 4 Hz, so eight
// slots hold two seconds of snapshots for a coordinator drainer that has not
// been scheduled yet. Depth is not what makes the lane safe -- publishing is
// non-blocking at any depth, and the terminal event reserves room by
// discarding a queued progress snapshot rather than by waiting for one.
const laneCapacity = 8

// eventLane is the server's side of ServerHandle.Events.
//
// Its job is to make delivery impossible to block on. The coordinator's
// drainer may not exist yet, may be slow, or may already be gone, and a
// handler that blocked on a send would hold a payload descriptor and a
// connection open until someone read from a channel nobody owns -- which is
// exactly the deadlock the "Stop is quiescent on every return" postcondition
// forbids. So every publish is non-blocking, progress is droppable by
// contract, and the terminal event makes room for itself.
//
// The mutex is what lets close() be unconditionally safe: a send racing a
// close would panic on a closed channel, and Stop must never panic.
type eventLane struct {
	mu         sync.Mutex
	events     chan transfer.ServerEvent
	closed     bool
	terminated bool
}

func newEventLane() *eventLane {
	return &eventLane{events: make(chan transfer.ServerEvent, laneCapacity)}
}

// channel is the receive-only view handed to the coordinator.
func (l *eventLane) channel() <-chan transfer.ServerEvent {
	return l.events
}

// publishProgress delivers a snapshot if the lane has room, and drops it
// otherwise. Dropping is allowed: progress is a coalesced view of a byte
// counter that keeps counting, so the next snapshot supersedes this one.
func (l *eventLane) publishProgress(event transfer.ServerEvent) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.terminated {
		return false
	}
	select {
	case l.events <- event:
		return true
	default:
		return false
	}
}

// publishTerminal delivers the one Complete or Failed event for this server.
//
// Exactly one wins: a later terminal event, and any progress that raced it,
// are refused rather than queued behind it. Delivery does not depend on a
// consumer -- if the buffer is full, the oldest queued progress snapshot is
// discarded to free the slot, because a stale snapshot is expendable and a
// terminal outcome is not.
func (l *eventLane) publishTerminal(event transfer.ServerEvent) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed || l.terminated {
		return false
	}
	// The flag commits only on a delivery that succeeded. Setting it before the
	// sends would let a lane that failed to deliver any terminal event refuse
	// every later one, losing the outcome permanently instead of retrying.

	select {
	case l.events <- event:
		l.terminated = true
		return true
	default:
	}
	// Only progress snapshots can be queued here: this is the first terminal
	// event and no event follows it. Receiving from the producer side races
	// the drainer benignly, since either reader is dropping the same
	// expendable snapshot.
	select {
	case <-l.events:
	default:
	}
	select {
	case l.events <- event:
		l.terminated = true
		return true
	default:
		return false
	}
}

// close ends the lane permanently. It is idempotent, and after it returns the
// channel is closed and no publish will ever reopen or feed it.
func (l *eventLane) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	close(l.events)
}

func progressEvent(sessionID transfer.SessionID, snapshot transfer.ProgressSnapshot) transfer.ServerEvent {
	return transfer.ServerEvent{
		SessionID: sessionID,
		Kind:      transfer.ServerProgress,
		Progress:  &snapshot,
	}
}

func completeEvent(sessionID transfer.SessionID, snapshot transfer.ProgressSnapshot) transfer.ServerEvent {
	return transfer.ServerEvent{
		SessionID: sessionID,
		Kind:      transfer.ServerComplete,
		Progress:  &snapshot,
	}
}

// failedEvent reports a terminal failure. snapshot is attached only when bytes
// actually reached the receiver: a failure before the first byte carries no
// progress at all, so the coordinator cannot publish a "0 bytes sent" update
// for a transfer that never started sending.
func failedEvent(sessionID transfer.SessionID, snapshot transfer.ProgressSnapshot, cause error) transfer.ServerEvent {
	event := transfer.ServerEvent{
		SessionID: sessionID,
		Kind:      transfer.ServerFailed,
		Err:       cause,
	}
	if snapshot.BytesSent > 0 {
		event.Progress = &snapshot
	}
	return event
}
