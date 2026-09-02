package transfer

import "math"

// This file gives one session's server events their lifecycle meaning: which
// snapshots become progress events, which report becomes the session's single
// terminal outcome, and what an unexpected lane closure means.
//
// Everything here runs on the drainer goroutine, and two rules keep that safe:
//
//   - the drainer never *waits* for the operation lease. A lease held while
//     this session is TRANSFERRING can only be a teardown, and a teardown
//     already owns the outcome the drainer would otherwise publish. Waiting
//     would deadlock outright, because a Cancel holding the lease waits for
//     the drainer it would be waiting on.
//   - the drainer never joins itself. Terminal handling releases resources
//     with releaseAcquired rather than unwind, and lets the loop end on its
//     own when Stop closes the lane.

// drain consumes one session's server event lane until it closes, so the lane
// can never block a teardown, and turns each event into the lifecycle event it
// means. It owns nothing: every decision is revalidated against the live state
// under the mutex, so a session that has been replaced, cancelled or already
// terminated silently discards whatever is still in flight for it.
func (c *Coordinator) drain(live *session, events <-chan ServerEvent) {
	defer close(live.drainerDone)

	for event := range events {
		switch event.Kind {
		case ServerProgress:
			c.forwardProgress(live, event)
		case ServerComplete, ServerFailed:
			c.acceptTerminal(live, event)
		default:
			// The port defines three kinds and this coordinator does not own
			// the adapter that produces them. Discarding an unrecognized one
			// is the only safe action, but discarding it silently would hide
			// the adapter defect that produced it -- the same reasoning that
			// makes the drainer re-check the event's own session id.
			c.recordDiagnostic(
				NewError(ErrTransferFailed, "unrecognized server event"),
				"the transfer server reported an event kind this build does not know",
			)
		}
	}

	// The lane closed. Only ServerPort.Stop closes it, so either a teardown
	// asked for that Stop -- in which case the session is cancelled, closing,
	// or already terminal, and this is refused silently -- or the server went
	// away without reporting an outcome while a transfer was live, which is a
	// failure the UI would otherwise never hear about.
	c.acceptTerminal(live, ServerEvent{
		SessionID: live.id,
		Kind:      ServerFailed,
		Err:       NewError(ErrTransferFailed, "the transfer server stopped before reporting an outcome"),
	})
}

// forwardProgress publishes one snapshot, or drops it.
//
// Dropping is not a failure: the contract makes progress coalescable, so a
// snapshot that cannot be published is superseded by the next one. What must
// not happen is a gap -- so the sequence number is assigned in the same
// critical section as the decision to publish, and a dropped snapshot consumes
// nothing.
func (c *Coordinator) forwardProgress(live *session, event ServerEvent) {
	if event.Progress == nil {
		// ServerProgress carries a snapshot by the port's definition, so a nil
		// one is an adapter defect. Returning here is what keeps it from
		// panicking the drainer goroutine, where nothing recovers and the
		// process dies; the diagnostic is what keeps it from vanishing.
		c.recordDiagnostic(
			NewError(ErrTransferFailed, "progress without a snapshot"),
			"the transfer server reported progress carrying no measurement",
		)
		return
	}

	c.mu.Lock()
	if !c.drainerMayActLocked(live, event.SessionID) {
		c.mu.Unlock()
		return
	}
	if !c.acquireLease() {
		// A teardown or the started publication owns the lane right now.
		// Progress is the one thing that may be thrown away rather than
		// waited for, and waiting here is what would deadlock.
		c.mu.Unlock()
		return
	}
	live.seq++
	snapshot := sanitizeProgress(*event.Progress)
	published := Event{SessionID: live.id, Seq: live.seq, Kind: TransferProgress, Progress: &snapshot}
	c.mu.Unlock()

	c.publish(published)
	c.releaseLease()
}

// acceptTerminal accepts this session's one Complete or Failed outcome and
// runs the whole terminal path: quiesce, publish the authoritative final
// progress when there is one, publish the outcome, hold the terminal UI lease,
// and arm the reset.
//
// It is refused -- silently, changing nothing -- when the session is not the
// live one, is not TRANSFERRING, has already accepted an outcome, has been
// marked cancelled, or when the operation lease is held by the teardown that
// owns this outcome instead.
func (c *Coordinator) acceptTerminal(live *session, event ServerEvent) {
	c.mu.Lock()
	if !c.drainerMayActLocked(live, event.SessionID) {
		c.mu.Unlock()
		return
	}
	if !c.acquireLease() {
		c.mu.Unlock()
		return
	}
	// Marked before the lease is ever handed back, so the second terminal
	// event of a session cannot be accepted even in the window where this one
	// is still quiescing and the state still reads TRANSFERRING.
	live.terminal = true
	c.mu.Unlock()

	// Resources go first. The UI must never be told a transfer finished while
	// its listener is still accepting connections or its beacon is still
	// advertising. This is the unwind variant that does not join the drainer,
	// because this code is the drainer.
	c.releaseAcquired(live)

	// Shutdown can raise the closing flag after this outcome took the lease,
	// and it then waits for that lease before retiring the session. Without
	// this re-check the outcome would publish into a UI Shutdown has already
	// declared gone. The state still settles, so the retire that follows sees
	// a session in its terminal state rather than one stuck TRANSFERRING.
	c.mu.Lock()
	closing := c.closing
	c.mu.Unlock()

	final, reported := terminalSnapshot(event)
	if reported && !closing {
		// The authoritative final snapshot is published as its own progress
		// event first, which is the grammar the contract fixes: a UI that
		// renders progress and outcome separately still sees the last byte
		// count before it sees the outcome.
		snapshot := final
		c.publishNext(live, Event{Kind: TransferProgress, Progress: &snapshot})
	}

	var settled sessionState
	switch {
	case closing:
		// Nothing is published, but the outcome is still accepted: which one
		// arrived first stays decided, so a second event cannot be taken.
		if event.Kind == ServerComplete {
			settled = stateDone
		} else {
			settled = stateError
		}
	case event.Kind == ServerComplete:
		// complete carries a progress payload by the contract's payload table.
		// A Complete without a snapshot is a port defect rather than a
		// transfer failure -- the bytes did arrive -- so it reports the
		// unknown-total zero snapshot rather than downgrading a success.
		snapshot := final
		c.publishNext(live, Event{Kind: TransferComplete, Progress: &snapshot})
		settled = stateDone
	default:
		failure := terminalPublicError(event.Err)
		var payload *ProgressSnapshot
		if reported {
			snapshot := final
			payload = &snapshot
		}
		c.publishNext(live, Event{Kind: TransferError, Progress: payload, Error: &failure})
		settled = stateError
	}

	c.mu.Lock()
	c.state = settled
	c.mu.Unlock()

	c.releaseLease()
	c.armReset(live)
}

// drainerMayActLocked reports whether this drainer's session is still the one
// the coordinator is running, is still transferring, and has not already
// settled. The caller holds c.mu.
//
// The event's own session id is checked too. ServerHandle.Events belongs to
// one session by construction, but the coordinator does not get to assume an
// adapter it does not own labels its events correctly.
func (c *Coordinator) drainerMayActLocked(live *session, reported SessionID) bool {
	if live.terminal {
		return false
	}
	if reported != live.id {
		return false
	}
	return c.revalidateLocked(live.ctx, live.id, live.generation, stateTransferring) == nil
}

// publishNext assigns the next sequence number and publishes. The caller owns
// the operation lease and must not hold the state mutex.
//
// Sequence assignment lives here, next to the publication it belongs to, so
// that "a number is consumed only by an event that is actually published"
// holds structurally rather than by review.
func (c *Coordinator) publishNext(live *session, event Event) {
	c.mu.Lock()
	live.seq++
	event.SessionID = live.id
	event.Seq = live.seq
	c.mu.Unlock()

	c.publish(event)
}

// armReset schedules the session's return to IDLE, three seconds after the
// terminal outcome the UI is holding.
//
// It runs after the lease has been handed back rather than under it. Arming
// under the lease would let a reset that fires immediately find the lease held
// by the very handler that armed it, fail its non-blocking acquisition, and
// vanish -- parking the session in DONE with no reset at all. Arming after the
// release trades that for a window in which a Cancel retires the session
// first, which this re-check sees: no timer is armed for a session that is
// already gone.
func (c *Coordinator) armReset(live *session) {
	c.mu.Lock()
	due := c.resetIsDueLocked(live)
	c.mu.Unlock()
	if !due {
		return
	}

	// Scheduled without the mutex, like every other injected seam.
	stop := c.afterFunc(resetDelay, func() { c.fireReset(live) })

	if stop == nil {
		// A seam that schedules without handing back a way to stop leaves the
		// reset uncancellable. Calling a nil StopTimer would panic on the
		// drainer goroutine, where nothing recovers.
		c.recordDiagnostic(
			NewError(ErrTransferFailed, "reset scheduler returned no stop function"),
			"the reset scheduler cannot be cancelled",
		)
		return
	}

	c.mu.Lock()
	due = c.resetIsDueLocked(live)
	if due {
		live.stopReset = stop
	}
	c.mu.Unlock()

	if !due {
		// A Cancel retired the session while the timer was being created, so
		// nobody would ever stop this one. Stopping it here is what keeps the
		// window from leaking a pending callback.
		stop()
	}
}

// resetIsDueLocked reports whether this session is still sitting in a terminal
// state with no reset pending. The caller holds c.mu.
func (c *Coordinator) resetIsDueLocked(live *session) bool {
	if c.closing || c.session != live || live.cancelled || live.stopReset != nil {
		return false
	}
	return c.state == stateDone || c.state == stateError
}

// terminalSnapshot reports the authoritative final byte count an outcome
// carries, if it carries one.
//
// A Failed event with no snapshot must not become a zero-byte progress event:
// "the transfer failed before the first byte" and "the transfer failed having
// sent zero bytes so far" are different claims about the wire, and only the
// server knows which one happened.
func terminalSnapshot(event ServerEvent) (ProgressSnapshot, bool) {
	if event.Progress == nil {
		return ProgressSnapshot{}, false
	}
	return sanitizeProgress(*event.Progress), true
}

// terminalFailureCodes are the only codes a transfer failure may publish.
//
// It is an allow-list rather than a deny-list because the question is not
// "which codes are wrong here" but "which codes describe a transfer that began
// and then failed". The contract gives the payload path exactly these: the
// applicable path or source code, or the generic failure. Every other
// registered code describes something else entirely -- beacon_warning says
// discovery is down but the link still works, busy says a transfer is already
// running, cancelled says the user stopped it -- and each would put copy in
// front of the sender that contradicts the event carrying it.
var terminalFailureCodes = map[ErrorCode]struct{}{
	ErrTransferFailed:  {},
	ErrPathNotFound:    {},
	ErrPathUnsupported: {},
	ErrSourceChanged:   {},
}

// terminalPublicError maps a server failure cause to fixed public copy,
// preserving a code that describes this failure and rewriting anything else to
// the generic one. A nil cause is a failure the server could not classify, and
// the contract sends everything unrecognized to transfer_failed.
func terminalPublicError(cause error) PublicError {
	if cause != nil {
		if code := ErrorCodeOf(cause); ok(terminalFailureCodes, code) {
			return PublicErrorOf(cause)
		}
	}
	return PublicErrorOf(NewError(ErrTransferFailed, "the transfer did not finish"))
}

func ok(set map[ErrorCode]struct{}, code ErrorCode) bool {
	_, found := set[code]
	return found
}

// sanitizeProgress makes a snapshot safe to marshal at the UI boundary. The
// producing adapter already owes finite, clamped values; this is the boundary
// that cannot afford to trust that, because a single NaN would fail JSON
// marshalling and silently cost the UI an event rather than a field.
func sanitizeProgress(snapshot ProgressSnapshot) ProgressSnapshot {
	if snapshot.BytesSent < 0 {
		snapshot.BytesSent = 0
	}
	if snapshot.TotalBytes < 0 {
		snapshot.TotalBytes = 0
	}
	// An unknown total has one representation, fixed by the contract: zero
	// bytes and zero percent. Letting a percentage through beside
	// TotalKnown=false would put a completion figure on a payload whose length
	// the sender has explicitly said it does not know.
	if !snapshot.TotalKnown {
		snapshot.TotalBytes = 0
		snapshot.Percent = 0
	}
	snapshot.Percent = clamp(snapshot.Percent, 0, 100)
	snapshot.SpeedBytesPerSec = clamp(snapshot.SpeedBytesPerSec, 0, math.MaxFloat64)
	return snapshot
}

// clamp forces a value into a finite range. NaN is tested first because it
// compares false against every bound, so an ordinary clamp would pass it
// through untouched.
func clamp(value, low, high float64) float64 {
	switch {
	case math.IsNaN(value):
		return low
	case value < low:
		return low
	case value > high:
		return high
	default:
		return value
	}
}
