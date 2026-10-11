package transfer

import (
	"context"
	"sync"
)

// This file is the receive half of the coordinator: admitting a receive
// session, releasing its destination, reading its outcome from that
// destination, and the one notice a waiting session can publish.
//
// A receive session is a session like any other. It is admitted from IDLE by
// the same gate as Stage, holds the same operation lease, walks the same
// STAGING -> STAGED -> CLAIMING -> TRANSFERRING -> DONE/ERROR states and is torn
// down by the same bounded unwind. What differs is the subject (an opened
// destination rather than a staged item) and one deliberate behaviour: a
// receive outcome is held until the user leaves it rather than reset after three
// seconds, because the desktop offers Show in Folder from it and a three second
// window to reach that is not a window.

// receiveSession is the receive-specific state of one session.
type receiveSession struct {
	// dest is set before the session is installed in the ledger and never
	// changes, so any goroutine may read it.
	dest ReceiveDestination

	// final and finalized are guarded by Coordinator.mu. final is the
	// destination's result as of the moment its Close returned (or, if that
	// bound elapsed, its non-blocking snapshot at that moment): the count the
	// outcome reports, and the count no later write can change because a closed
	// destination commits nothing.
	final     ReceiveResult
	finalized bool
}

// StartReceive validates the chosen folder, then admits a receive session
// through the same lifecycle as Stage: from IDLE only, under the operation
// lease, with every failure or cancellation unwound in reverse acquisition order
// to IDLE with no lifecycle event. Nothing is written to the folder, and the
// listener, QR code and beacon do not exist until the folder has been accepted.
func (c *Coordinator) StartReceive(ctx context.Context, absolutePath string) (ReceiveMetadata, error) {
	if absolutePath == "" {
		return ReceiveMetadata{}, NewError(ErrInvalidSelection, "choose a folder")
	}
	if ctx == nil {
		return ReceiveMetadata{}, NewError(ErrSetupFailed, "receiving requires a context")
	}
	if err := c.ready(); err != nil {
		return ReceiveMetadata{}, err
	}
	if c.sink == nil {
		return ReceiveMetadata{}, NewError(ErrNotReady, "FairDrop is not ready to receive files")
	}

	id, token, err := c.newIdentity()
	if err != nil {
		return ReceiveMetadata{}, err
	}
	live, err := c.admit(ctx, id, token)
	if err != nil {
		return ReceiveMetadata{}, err
	}
	generation := live.generation

	setupCtx, stopSetup := context.WithCancel(live.ctx)
	defer stopSetup()
	stopCallerWatch := context.AfterFunc(ctx, stopSetup)
	defer stopCallerWatch()

	// 1. Validate and pin the destination. Nothing on the network is touched
	//    until the folder has proven itself.
	dest, err := c.openDestinationBounded(setupCtx, absolutePath)
	if err != nil {
		return c.failReceive(live, err)
	}
	if dest == nil {
		return c.failReceive(live, NewError(ErrSetupFailed, "the destination folder could not be opened"))
	}
	c.mu.Lock()
	live.receive = &receiveSession{dest: dest}
	c.mu.Unlock()
	live.hold(resourceDestination)
	if err := c.afterStep(ctx, setupCtx, id, generation); err != nil {
		return c.failReceive(live, err)
	}

	// 2. Everything from the address to the commit is shared with Stage.
	ready, err := c.activate(ctx, setupCtx, live, ServerStartRequest{SessionID: id, Token: token, Destination: dest}, uploadPathPrefix)
	if err != nil {
		return c.failReceive(live, err)
	}

	return ReceiveMetadata{
		SessionID:   id,
		Destination: dest.Name(),
		URL:         ready.url,
		QR:          ready.qr,
		Warnings:    ready.warnings,
	}, nil
}

func (c *Coordinator) failReceive(live *session, cause error) (ReceiveMetadata, error) {
	_, err := c.failStage(live, cause)
	return ReceiveMetadata{}, err
}

// openDestinationBounded opens the destination within AdapterCleanupBound. A
// hung network volume must not leave the user unable to cancel, so a bound that
// elapses is a coded setup failure; whatever the abandoned call eventually
// returns is closed rather than leaked, so a late success cannot strand a
// destination handle.
func (c *Coordinator) openDestinationBounded(ctx context.Context, absolutePath string) (ReceiveDestination, error) {
	var (
		mu        sync.Mutex
		opened    ReceiveDestination
		abandoned bool
	)
	err, completed := c.callBounded(AdapterCleanupBound, func() error {
		dest, openErr := c.sink.OpenDestination(ctx, absolutePath)
		mu.Lock()
		defer mu.Unlock()
		if abandoned {
			if dest != nil {
				_, _ = dest.Close()
			}
			return openErr
		}
		opened = dest
		return openErr
	})
	mu.Lock()
	defer mu.Unlock()
	if !completed {
		abandoned = true
		if opened != nil {
			// The call finished between the bound elapsing and this lock.
			_, _ = opened.Close()
		}
		timeoutErr := NewError(ErrSetupFailed, "the destination folder did not answer before its bound")
		c.recordDiagnostic(timeoutErr, "receive destination validation did not finish before its bound")
		return nil, timeoutErr
	}
	if err != nil {
		if opened != nil {
			_, _ = opened.Close()
		}
		return nil, err
	}
	return opened, nil
}

// closeDestinationBounded closes a receive session's destination within
// AdapterCleanupBound and records its final result. It runs from the unwind's
// reverse release, after the server has stopped, so no handler is still writing
// unless the server's own bound elapsed -- in which case the destination's Close
// is exactly what refuses that handler's remaining commits.
//
// A bound hit is returned, never absorbed. The result is still recorded from the
// destination's non-blocking snapshot, which is final because Close marks the
// destination closed before it does any filesystem work.
func (c *Coordinator) closeDestinationBounded(live *session) error {
	c.mu.Lock()
	recv := live.receive
	c.mu.Unlock()
	if recv == nil {
		return nil
	}
	err, completed := c.callBounded(AdapterCleanupBound, func() error {
		final, closeErr := recv.dest.Close()
		c.mu.Lock()
		recv.final, recv.finalized = final, true
		c.mu.Unlock()
		return closeErr
	})
	if !completed {
		snapshot := recv.dest.Snapshot()
		c.mu.Lock()
		if !recv.finalized {
			recv.final, recv.finalized = snapshot, true
		}
		c.mu.Unlock()
		timeoutErr := NewError(ErrTransferFailed, "the destination folder did not confirm it closed before its bound")
		c.recordDiagnostic(timeoutErr, "receive destination cleanup did not finish before its bound")
		return timeoutErr
	}
	if err != nil {
		c.recordDiagnostic(err, "receive destination cleanup reported a problem")
	}
	return nil
}

// receiveStatus reads a receive session's outcome. It returns nil for a send
// session, which is how the shared terminal path stays one path.
func (c *Coordinator) receiveStatus(live *session, result ReceiveResultKind) *ReceiveStatus {
	c.mu.Lock()
	recv := live.receive
	var final ReceiveResult
	finalized := false
	if recv != nil {
		final, finalized = recv.final, recv.finalized
	}
	c.mu.Unlock()
	if recv == nil {
		return nil
	}
	if !finalized {
		// A terminal path always releases the destination first, so this is the
		// defensive branch: the destination's own non-blocking view.
		final = recv.dest.Snapshot()
	}
	return &ReceiveStatus{
		FilesSaved:      max(final.FilesSaved, 0),
		Result:          result,
		SubfolderExists: final.SubfolderExists,
		MarkingWarning:  final.MarkingFailed,
	}
}

// receiveProgress is the receive payload of a progress event: only the saved
// count, taken from the destination's final result for the last snapshot.
func (c *Coordinator) receiveProgress(live *session) *ReceiveStatus {
	status := c.receiveStatus(live, "")
	if status == nil {
		return nil
	}
	return &ReceiveStatus{FilesSaved: status.FilesSaved}
}

// ReceivedFolder returns the absolute path of the live receive session's
// subfolder for the desktop's Show in Folder, or a coded refusal when there is
// no receive session or its subfolder does not exist (nothing was saved, or it
// was removed). The path leaves this method only for the OS file manager call;
// it is never put in an event, a diagnostic or an HTTP response.
func (c *Coordinator) ReceivedFolder() (string, error) {
	if c == nil {
		return "", NewError(ErrNotReady, "FairDrop is not ready to show a folder")
	}
	c.mu.Lock()
	var dest ReceiveDestination
	if c.session != nil && c.session.receive != nil {
		dest = c.session.receive.dest
	}
	c.mu.Unlock()
	if dest == nil {
		return "", NewError(ErrPathNotFound, "there is no received folder to show")
	}
	folder, exists := dest.Folder()
	if !exists || folder == "" {
		return "", NewError(ErrPathNotFound, "the session has not saved a folder")
	}
	return folder, nil
}

// forwardNotice publishes one notice for a receive session that is still
// waiting, or drops it. Like progress it is droppable: it consumes a sequence
// number only when it is actually published, and a claim or teardown that owns
// the lease right now has made it moot.
func (c *Coordinator) forwardNotice(live *session, event ServerEvent) {
	if live.receive == nil || event.Notice != NoticeReceiveTooLarge {
		c.recordDiagnostic(
			NewError(ErrTransferFailed, "unrecognized server notice"),
			"the transfer server reported a notice this session does not know",
		)
		return
	}

	c.mu.Lock()
	if !c.drainerMayActLocked(live, event.SessionID, stateStaged) {
		c.mu.Unlock()
		return
	}
	if !c.acquireLease() {
		c.mu.Unlock()
		return
	}
	live.seq++
	published := Event{SessionID: live.id, Seq: live.seq, Kind: TransferNotice, Notice: event.Notice}
	c.mu.Unlock()

	c.publish(published)
	c.releaseLease()
}

// settleCancelledReceive ends a receive session the desktop cancelled while an
// upload was in flight. It runs inside retire, after the unwind has stopped the
// server and closed the destination, and publishes the one terminal outcome the
// user is owed -- "cancelled, N saved" -- then holds it like any other receive
// outcome rather than resetting. The caller owns the operation lease.
//
// It returns the unwind's own failure through retireFailure exactly as the plain
// retire does: an elapsed bound is reported, never absorbed.
func (c *Coordinator) settleCancelledReceive(live *session, unwindErr error) error {
	cancelled := PublicErrorOf(NewError(ErrCancelled, "the transfer was cancelled"))
	c.publishNext(live, Event{
		Kind:    TransferError,
		Error:   &cancelled,
		Receive: c.receiveStatus(live, ReceiveCancelled),
	})

	c.mu.Lock()
	c.state = stateError
	c.mu.Unlock()

	c.releaseLease()
	return c.retireFailure(live, unwindErr)
}
