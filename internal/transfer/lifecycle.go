package transfer

import (
	"context"
	"time"
)

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
// teardown that is already running. On the healthy path it returns nil only
// once the resources it names are quiescent -- listener, beacon, drainer and
// session context -- and it reports success even when a cleanup step recorded
// a diagnostic, because every Stop that actually returns in time is quiescent
// on return and a completed cancellation is not a failed command. That is
// unchanged from before this type learned to bound anything.
//
// What changes is the unhealthy path. ctx bounds how long Cancel will wait to
// join a teardown some other operation already owns; every wait inside the
// teardown Cancel itself drives is bounded on its own documented seam
// regardless of ctx. Either kind of bound elapsing is reported as a coded
// failure naming what did not return, never as success: Cancel is honest
// about what it could not confirm rather than claiming the resource is gone.
// A nil ctx is treated as context.Background, so an existing caller that has
// not been updated to supply one still gets the documented bound rather than
// a panic.
func (c *Coordinator) Cancel(ctx context.Context) error {
	if c == nil {
		// Stage and AuthorizeClaim answer a missing coordinator with a coded
		// error rather than a panic; a command may not be the odd one out.
		return NewError(ErrTransferFailed, "FairDrop is not ready to cancel a transfer")
	}
	c.mu.Lock()
	if c.closing {
		// A command after Shutdown changes nothing and touches nothing.
		c.mu.Unlock()
		return NewError(ErrShuttingDown, "FairDrop is closing")
	}
	live := c.markCancelledLocked()
	c.mu.Unlock()

	if live == nil {
		// IDLE, but not necessarily quiet: retire and fireReset both clear the
		// session several lines before they hand the lease back, so returning
		// here without waiting would report IDLE while a reset was still being
		// published. Waiting calls no adapter and changes no state, which is
		// what the command table asks of Cancel from IDLE.
		if err := c.awaitLeaseBounded(ctx); err != nil {
			return err
		}
		c.releaseLease()
		return nil
	}
	live.stop()

	if err := c.awaitLeaseBounded(ctx); err != nil {
		return err
	}
	return c.retire(live, true)
}

// Shutdown closes the application lifetime: it refuses every later command,
// cancels the live session, quiesces every resource, and suppresses the UI
// events a Cancel would publish -- a closing application has no UI left to
// tell. On the healthy path it returns nil only when everything is gone, and
// repeating it is idempotent: the second call finds the session already
// retired, joins nothing, and publishes nothing.
//
// ctx bounds the same wait Cancel's does, and for the same reason: joining a
// teardown some other operation already owns must not be an unbounded wait
// just because this caller is Shutdown rather than Cancel. A nil ctx is
// treated as context.Background.
func (c *Coordinator) Shutdown(ctx context.Context) error {
	if c == nil {
		// Unlike Cancel this reports success: Shutdown promises that nothing
		// is left running, and for a coordinator that never existed that is
		// already true.
		return nil
	}
	live := c.beginClosing()

	// Taken even with nothing staged: the lease being free is the proof that
	// no setup or teardown is still running, and Shutdown may not return
	// before that is true.
	if err := c.awaitLeaseBounded(ctx); err != nil {
		return err
	}
	if live == nil {
		c.releaseLease()
		return nil
	}
	return c.retire(live, false)
}

// retire drives one marked session to IDLE and hands the operation lease back.
// The caller owns the lease and must not hold the state mutex.
//
// announce asks for the reset event the UI needs to leave its terminal view.
// Shutdown passes false.
//
// A session that never reached its STAGED acknowledgement publishes nothing
// either way, and the `c.session != live` return below is the whole of what
// carries that. STAGING exists only while Stage holds the operation lease, and
// this runs after awaiting it, so by now Stage has either reached failStage --
// which cleared the session -- or committed STAGED. An explicit STAGING check
// here would be unreachable, and an unreachable guard reads to the next person
// like a live one.
//
// retire always reaches IDLE and always hands the lease back, whether or not
// unwind's own bounds were all hit: a stuck adapter costs its own resource's
// quiescence, never the coordinator's ability to serve the next command. The
// returned error -- nil on the healthy path -- is unwind's first bound
// failure, if any, so Cancel and Shutdown can report it instead of claiming
// success they cannot back up.
func (c *Coordinator) retire(live *session, announce bool) error {
	c.mu.Lock()
	if c.session != live {
		// The session we marked finished its own teardown while we waited for
		// the lease. Nothing is left to release, and publishing here would be
		// a second reset for one session.
		c.mu.Unlock()
		c.releaseLease()
		return nil
	}
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
	unwindErr := c.unwind(live)
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
	return unwindErr
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
	// The bound's own failure is already a recorded diagnostic; there is no
	// caller here to report it to, so the timer proceeds to IDLE regardless --
	// a leaked drainer costs its own quiescence, not the terminal UI lease.
	_ = c.joinDrainerBounded(live)

	c.mu.Lock()
	if c.closing {
		// Shutdown raised the flag while this reset was joining the drainer,
		// and is now waiting for the lease to retire the session itself.
		c.mu.Unlock()
		c.releaseLease()
		return
	}
	live.seq++
	reset := Event{SessionID: live.id, Seq: live.seq, Kind: TransferReset}
	c.session = nil
	c.state = stateIdle
	c.mu.Unlock()

	// Cancelled before the lease is handed back, exactly as retire does it. A
	// Cancel or Shutdown parked in awaitLease is released by that hand-back,
	// and it must not be able to return "everything is gone" while this
	// session's context is still live.
	live.stop()
	c.publish(reset)
	c.releaseLease()
}

// awaitLeaseBounded waits, up to leaseBound or ctx (whichever elapses
// first), for the operation lease to become free, and takes it the instant it
// does.
//
// Waiting is the join: whoever holds the lease is the one cleanup in flight
// for this session, so a command that waits joins that teardown instead of
// racing a second one. Only commands may wait. The drainer never does -- a
// drainer blocked here while a teardown waited on the drainer is the exact
// deadlock this design exists to make unrepresentable.
//
// The fast path checks the lease without arming anything: the lease is free
// far more often than not, and arming a timer only to stop it immediately
// would cost every healthy Cancel and Shutdown an entry in every call log
// that counts adapter interactions. The bound is armed only once the caller
// has genuinely found something else already holding the lease.
//
// On a timeout the lease is NOT taken -- this caller never held it, so it
// must not release what it does not own, and the token stays exactly where
// its real holder will find it when that holder's own bounded teardown ends.
func (c *Coordinator) awaitLeaseBounded(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case <-c.lease:
		return nil
	default:
	}

	timedOut := make(chan struct{})
	stop := c.boundTimer(leaseBound, func() { close(timedOut) })
	defer stop()

	select {
	case <-c.lease:
		return nil
	case <-ctx.Done():
		return WrapError(
			ErrTransferFailed,
			"the caller abandoned the wait for the previous transfer's cleanup to finish",
			ctx.Err(),
		)
	case <-timedOut:
		return NewError(
			ErrTransferFailed,
			"the previous transfer's cleanup did not finish releasing its resources before the bound elapsed",
		)
	}
}
