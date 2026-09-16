package transfer

import "time"

// This file holds the bounded-call subsystem every quiescence wait in this
// package is built from: run one external call on its own goroutine, arm a
// bound before launching it, and report whether it returned in time rather
// than waiting on it forever. callAdapterBounded and cleanupPending add the
// one refinement idempotent cleanup calls get that a bare callBounded does
// not: two callers asking the same question -- is the server or the beacon
// stopped -- share one in-flight call instead of each starting their own.

type boundedCall struct {
	done chan struct{}
	err  error
}

// joinDrainerBounded waits, up to drainerJoinBound, for this session's
// drainer goroutine to end, which is what keeps a session's goroutine from
// outliving the session. Calling it twice is safe, and so is calling it after
// the drainer has already gone: Stop closed the event lane, so the loop is on
// its way out, and a closed done channel receives immediately.
//
// A drainer that never ends is now reported rather than waited on forever
// (D-027, D-032, D-090): the reasoning the old unbounded wait's comment gave
// -- a watchdog here would let Cancel report success while a publication was
// still in flight -- is answered by reporting failure instead of success, not
// by waiting without end. ServerPort.Stop remains documented as quiescent on
// every healthy return, so on a healthy adapter this still resolves the
// instant the lane closes, well inside the bound.
func (c *Coordinator) joinDrainerBounded(live *session) error {
	if live.drainerDone == nil {
		return nil
	}
	if c.awaitBounded(live.drainerDone, drainerJoinBound) {
		return nil
	}
	timeoutErr := NewError(ErrTransferFailed, "the transfer event drainer did not finish before its bound")
	c.recordDiagnostic(timeoutErr, "the transfer event drainer did not finish before its bound")
	return timeoutErr
}

// awaitBounded waits for ch to be ready or for bound to elapse, whichever
// comes first, and reports which happened. It never consumes ch's value when
// the bound wins, so a lease token or a closed-channel receive that arrives
// late is still there for whoever asks next.
func (c *Coordinator) awaitBounded(ch <-chan struct{}, bound time.Duration) bool {
	// Fast path, exactly like awaitLeaseBounded's: ch is a real, already-
	// settled fact here (unlike callBounded's call, which has not run yet),
	// so checking it without arming anything is safe rather than a race, and
	// it is what keeps a drainer that has already ended from costing every
	// caller a timer entry in its call log.
	select {
	case <-ch:
		return true
	default:
	}

	timedOut := make(chan struct{})
	stop := c.boundTimer(bound, func() { close(timedOut) })
	select {
	case <-ch:
		stop()
		return true
	case <-timedOut:
		return false
	}
}

// callBounded runs one external adapter call on its own goroutine and waits,
// up to bound, for it to return, reporting whether it did. An adapter that
// never returns leaves that goroutine running -- Go offers no way to force a
// function to stop -- so a bound that elapses reports exactly that: the call
// did not come back in time, never that it succeeded or that it failed. The
// abandoned goroutine's eventual result, if it ever arrives, is delivered
// into a buffered channel nobody is obliged to read again, so it cannot block
// anything further.
func (c *Coordinator) callBounded(bound time.Duration, call func() error) (result error, completed bool) {
	// Armed before the call is launched, deliberately: arming after would
	// race the spawned goroutine for which one logs first, making the
	// adapter-call order nondeterministic for no reason. Arming first costs
	// nothing on the healthy path -- the timer is stopped the instant the
	// call returns -- and keeps every bound's position in a test's call log
	// fixed rather than a coin flip.
	timedOut := make(chan struct{})
	stop := c.boundTimer(bound, func() { close(timedOut) })

	done := make(chan error, 1)
	go func() { done <- call() }()

	select {
	case err := <-done:
		stop()
		return err, true
	case <-timedOut:
		return nil, false
	}
}

// stopServerBounded stops the transfer server within AdapterCleanupBound. A
// returned error means the bound was hit and the server did not confirm it
// stopped in time -- the coded failure Cancel, Shutdown, or a claim must
// report. An adapter error that arrives within the bound is recorded as a
// diagnostic exactly as before and never surfaces as a command failure of its
// own: cleanup errors stay diagnostics on the healthy path.
func (c *Coordinator) stopServerBounded() error {
	err, completed := c.callAdapterBounded(&c.serverCleanup, AdapterCleanupBound, c.server.Stop)
	if !completed {
		timeoutErr := NewError(ErrTransferFailed, "the transfer server did not confirm it stopped before its bound")
		c.recordDiagnostic(timeoutErr, "transfer server cleanup did not finish before its bound")
		return timeoutErr
	}
	if err != nil {
		if IsUnquiescent(err) {
			c.recordDiagnostic(err, "transfer server cleanup did not prove quiescence")
			return err
		}
		c.recordDiagnostic(err, "transfer server cleanup reported a problem")
	}
	return nil
}

// stopBeaconBounded stops the discovery beacon within AdapterCleanupBound,
// mirroring stopServerBounded exactly: a bound hit is reported, an adapter
// error that arrives in time stays a diagnostic.
func (c *Coordinator) stopBeaconBounded() error {
	err, completed := c.callAdapterBounded(&c.beaconCleanup, AdapterCleanupBound, c.network.StopBeacon)
	if !completed {
		timeoutErr := NewError(ErrTransferFailed, "device discovery did not confirm it stopped before its bound")
		c.recordDiagnostic(timeoutErr, "device discovery cleanup did not finish before its bound")
		return timeoutErr
	}
	if err != nil {
		c.recordDiagnostic(err, "device discovery cleanup reported a problem")
	}
	return nil
}

func (c *Coordinator) callAdapterBounded(slot **boundedCall, bound time.Duration, call func() error) (error, bool) {
	c.cleanupMu.Lock()
	pending := *slot
	owned := pending == nil
	if owned {
		pending = &boundedCall{done: make(chan struct{})}
		*slot = pending
	}
	c.cleanupMu.Unlock()

	// Armed before the call is launched, for the reason callBounded gives
	// above: arming after races the spawned goroutine for which one reaches a
	// test's call log first, and that is not a detail a test should have to
	// tolerate. This function launched first and armed second until a Windows
	// runner caught it, so the claim is now in two places that must agree.
	// Registering the slot above and launching below is deliberate too -- a
	// joining caller may briefly find a call that has not started, and waits
	// on the same done channel either way.
	timedOut := make(chan struct{})
	stop := c.boundTimer(bound, func() { close(timedOut) })
	if owned {
		go func() {
			pending.err = call()
			close(pending.done)
		}()
	}
	select {
	case <-pending.done:
		stop()
		c.cleanupMu.Lock()
		if *slot == pending {
			*slot = nil
		}
		c.cleanupMu.Unlock()
		return pending.err, true
	case <-timedOut:
		return nil, false
	}
}

func (c *Coordinator) cleanupPending() bool {
	c.cleanupMu.Lock()
	defer c.cleanupMu.Unlock()
	pending := false
	for _, slot := range []**boundedCall{&c.serverCleanup, &c.beaconCleanup} {
		if *slot == nil {
			continue
		}
		select {
		case <-(*slot).done:
			*slot = nil
		default:
			pending = true
		}
	}
	return pending
}
