package transfer

import "time"

// resetDelay is the terminal UI lease: how long a DONE or ERROR session stays
// on screen before the coordinator clears it and returns to IDLE. It is a
// backend lease on purpose -- no frontend timer removes a terminal view --
// which is why it is scheduled through an injected seam rather than a bare
// time.AfterFunc.
const resetDelay = 3 * time.Second

// Cancel abandons whatever transfer is in flight and returns the coordinator
// to IDLE.
//
// It wins from any state, and it never starts a second cleanup: marking the
// generation cancelled and cancelling the data-plane context is the whole of
// the request, and waiting for the operation lease is how it joins the
// teardown that is already running. It returns only once the resources it
// names are quiescent -- listener, beacon, drainer and session context -- and
// it reports success even when a cleanup step recorded a diagnostic, because
// every Stop is quiescent on return and a completed cancellation is not a
// failed command.
func (c *Coordinator) Cancel() error {
	c.mu.Lock()
	if c.closing {
		// A command after Shutdown changes nothing and touches nothing.
		c.mu.Unlock()
		return NewError(ErrShuttingDown, "FairDrop is closing")
	}
	live := c.markCancelledLocked()
	c.mu.Unlock()

	if live == nil {
		// IDLE: no session, no adapter call, no event, no state change.
		return nil
	}
	live.stop()

	c.awaitLease()
	c.retire(live, true)
	return nil
}

// Shutdown closes the application lifetime: it refuses every later command,
// cancels the live session, quiesces every resource, and suppresses the UI
// events a Cancel would publish -- a closing application has no UI left to
// tell. It returns only when everything is gone, and repeating it is
// idempotent: the second call finds the session already retired, joins nothing,
// and publishes nothing.
func (c *Coordinator) Shutdown() error {
	live := c.beginClosing()

	// Taken even with nothing staged: the lease being free is the proof that
	// no setup or teardown is still running, and Shutdown may not return
	// before that is true.
	c.awaitLease()
	if live == nil {
		c.releaseLease()
		return nil
	}
	c.retire(live, false)
	return nil
}

// retire drives one marked session to IDLE and hands the operation lease back.
// The caller owns the lease and must not hold the state mutex.
//
// announce asks for the reset event the UI needs to leave its terminal view.
// Shutdown passes false, and a session that never reached its STAGED
// acknowledgement publishes nothing either way: nothing was acknowledged, so
// there is no UI session to terminate and the command's own error is the whole
// outcome.
func (c *Coordinator) retire(live *session, announce bool) {
	c.mu.Lock()
	if c.session != live {
		// The session we marked finished its own teardown while we waited for
		// the lease. Nothing is left to release, and publishing here would be
		// a second reset for one session.
		c.mu.Unlock()
		c.releaseLease()
		return
	}
	announce = announce && c.state != stateStaging
	stopReset := live.stopReset
	live.stopReset = nil
	c.mu.Unlock()

	// Stopping the armed reset before publishing our own is what makes the
	// timer/Cancel race produce exactly one reset. A timer that already fired
	// is not stopped by this, which is why the timer revalidates on the far
	// side: it will find the session cleared and publish nothing.
	if stopReset != nil {
		stopReset()
	}

	// In a terminal state acquired is already empty, so this releases nothing
	// and only joins the drainer the terminal path could not join itself.
	c.unwind(live)
	live.stop()

	var reset *Event
	c.mu.Lock()
	if announce {
		live.seq++
		reset = &Event{SessionID: live.id, Seq: live.seq, Kind: TransferReset}
	}
	c.session = nil
	c.state = stateIdle
	c.mu.Unlock()

	if reset != nil {
		c.publish(*reset)
	}
	c.releaseLease()
}

// fireReset is the armed reset timer's callback: it ends the terminal UI lease
// by publishing one reset, clearing the session, and returning to IDLE.
//
// It is generation-checked rather than trusted. A timer cannot be un-fired,
// only outrun, so a session that was replaced, cancelled, or already cleared
// between the arming and the firing must publish nothing and mutate nothing.
func (c *Coordinator) fireReset(live *session) {
	c.mu.Lock()
	if err := c.revalidateLocked(live.ctx, live.id, live.generation, stateDone, stateError); err != nil {
		c.mu.Unlock()
		return
	}
	if !c.acquireLease() {
		// A Cancel or Shutdown holds the lease, so it owns this session's
		// outcome and will publish -- or deliberately suppress -- the reset
		// itself. Waiting for it would be the second cleanup the lease exists
		// to prevent.
		c.mu.Unlock()
		return
	}
	live.stopReset = nil
	c.mu.Unlock()

	// The terminal path released every resource but could not join its own
	// drainer. This is the first place that join is safe, and IDLE must not be
	// announced while a goroutine of the finished session is still running.
	c.joinDrainer(live)

	c.mu.Lock()
	live.seq++
	reset := Event{SessionID: live.id, Seq: live.seq, Kind: TransferReset}
	c.session = nil
	c.state = stateIdle
	c.mu.Unlock()

	c.publish(reset)
	c.releaseLease()
	live.stop()
}

// awaitLease blocks until the operation lease is free and then takes it.
//
// Waiting is the join: whoever holds the lease is the one cleanup in flight
// for this session, so a command that waits joins that teardown instead of
// racing a second one. Only commands may wait. The drainer never does -- a
// drainer blocked here while a teardown waited on the drainer is the exact
// deadlock this design exists to make unrepresentable.
func (c *Coordinator) awaitLease() {
	<-c.lease
}
