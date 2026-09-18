package transfer

import (
	"context"
	"time"
)

// This file holds the session state machine: the lifecycle states a
// Coordinator moves a transfer through, the resource ledger that records
// acquisition order for reverse unwind, and the session struct itself along
// with the three methods that mutate its resource ledger and stop its
// data-plane context.

// sessionState is the coordinator's lifecycle state. STAGING and CLAIMING are
// internal: they exist so a long setup or handshake stays interruptible, and
// the UI never sees them. DONE and ERROR are terminal holding states: the
// session's resources are already released there, and only the reset that
// clears the session still has to happen.
type sessionState string

const (
	stateIdle         sessionState = "IDLE"
	stateStaging      sessionState = "STAGING"
	stateStaged       sessionState = "STAGED"
	stateClaiming     sessionState = "CLAIMING"
	stateTransferring sessionState = "TRANSFERRING"
	stateDone         sessionState = "DONE"
	stateError        sessionState = "ERROR"
)

// resource names one thing a session acquires from an adapter. Stage appends
// each in acquisition order and unwind walks the list backwards, so reverse
// release is structural rather than a comment somebody has to keep true.
type resource int

const (
	resourceServer resource = iota
	resourceBeacon
)

// session is everything one staged transfer owns.
//
// Field ownership splits three ways, and mixing them is what a race here would
// look like:
//
//   - id, token, generation, ctx and cancel are set before the session is
//     installed and never change, so any goroutine may read them.
//   - cancelled, terminal, stopReset, seq, stagedAt and startedAt are guarded
//     by Coordinator.mu.
//   - everything else belongs to whichever operation holds the lease. The
//     lease is a channel handoff, so it carries the happens-before edge that
//     lets the claim path read what Stage wrote.
type session struct {
	id         SessionID
	token      CapabilityToken
	generation uint64
	ctx        context.Context
	cancel     context.CancelFunc

	cancelled bool
	// terminal records that this session's one Complete or Failed outcome has
	// been accepted. It is set the moment the outcome is taken, before the
	// resources are released and the settled state is committed, so
	// exactly-once acceptance does not depend on where that transition lands.
	terminal bool
	// stopReset cancels the armed three-second reset. It is nil whenever no
	// reset is pending, so a Cancel that finds it nil has nothing to stop.
	stopReset StopTimer
	seq       uint64
	stagedAt  time.Time
	startedAt time.Time

	item     StagedItem
	url      string
	qrBase64 string
	warnings []Warning
	acquired []resource

	drainerDone chan struct{}
}

// hold records a resource as live, in acquisition order.
func (s *session) hold(held resource) {
	s.acquired = append(s.acquired, held)
}

// stop cancels the session's data-plane context. The caller must not hold the
// state mutex: cancelling runs whatever is waiting on the context, and the
// no-lock-across-a-call rule covers those continuations too.
func (s *session) stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// release forgets a resource the operation just gave up, so a later unwind
// does not try to release it twice.
func (s *session) release(freed resource) {
	kept := s.acquired[:0]
	for _, held := range s.acquired {
		if held != freed {
			kept = append(kept, held)
		}
	}
	s.acquired = kept
}
