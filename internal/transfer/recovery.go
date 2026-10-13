package transfer

import (
	"context"
	"errors"
	"strconv"
)

// RecoveryFailure marks a refusal after the old capability was retired.
// The App exposes only this fact and fixed public error copy.
type RecoveryFailure struct{ Cause error }

func (e *RecoveryFailure) Error() string { return e.Cause.Error() }
func (e *RecoveryFailure) Unwrap() error { return e.Cause }

// ListConnectionOptions inspects a waiting session's network without changing
// its selected address or announcement.
func (c *Coordinator) ListConnectionOptions(ctx context.Context, id SessionID) (ConnectionOptions, error) {
	if c == nil || ctx == nil {
		return ConnectionOptions{}, NewError(ErrNotReady, "FairDrop is not ready to list networks")
	}
	capability, ok := c.network.(ConnectionNetwork)
	if !ok {
		return ConnectionOptions{}, NewError(ErrNetworkUnavailable, "network choices are unavailable")
	}
	c.mu.Lock()
	err := c.revalidateLocked(ctx, id, 0, stateStaged)
	var generation uint64
	var address string
	if err == nil {
		generation, address = c.session.generation, c.session.address
	}
	c.mu.Unlock()
	if err != nil {
		return ConnectionOptions{}, err
	}
	choices, err := capability.Options(ctx)
	if err != nil {
		return ConnectionOptions{}, err
	}
	c.mu.Lock()
	err = c.revalidateLocked(ctx, id, generation, stateStaged)
	c.mu.Unlock()
	if err != nil {
		return ConnectionOptions{}, err
	}
	if choices == nil {
		choices = []ConnectionOption{}
	}
	return ConnectionOptions{SessionID: id, CurrentAddress: address, Choices: choices}, nil
}

// RefreshConnection reserves the waiting state under the same lock as claim
// authorization. Once reserved, the old capability is irrevocably retired.
func (c *Coordinator) RefreshConnection(ctx context.Context, id SessionID, networkID string) (ConnectionRefresh, error) {
	if c == nil || ctx == nil {
		return ConnectionRefresh{}, NewError(ErrNotReady, "FairDrop is not ready to refresh")
	}
	capability, ok := c.network.(ConnectionNetwork)
	if !ok {
		return ConnectionRefresh{}, NewError(ErrNetworkUnavailable, "network choices are unavailable")
	}
	c.mu.Lock()
	err := c.revalidateLocked(ctx, id, 0, stateStaged)
	var generation uint64
	if err == nil {
		generation = c.session.generation
	}
	c.mu.Unlock()
	if err != nil {
		return ConnectionRefresh{}, err
	}
	// Pure, fresh preflight. A vanished explicit ID never retires the old code.
	choices, err := capability.Options(ctx)
	if err != nil {
		return ConnectionRefresh{}, err
	}
	if networkID != "" {
		found := false
		for _, choice := range choices {
			if choice.ID == networkID {
				found = true
				break
			}
		}
		if !found {
			return ConnectionRefresh{}, NewError(ErrNetworkUnavailable, "the selected local network is no longer available")
		}
	} else if len(choices) == 0 {
		return ConnectionRefresh{}, NewError(ErrNetworkUnavailable, "no usable local network address was found")
	}

	c.mu.Lock()
	err = c.revalidateLocked(ctx, id, generation, stateStaged)
	if err == nil && !c.acquireLease() {
		err = NewError(ErrBusy, "the transfer is busy")
	}
	if err != nil {
		c.mu.Unlock()
		return ConnectionRefresh{}, err
	}
	old := c.session
	c.state = stateStaging // Claiming is excluded before any external work.
	c.mu.Unlock()

	// The receive destination remains in one ledger until HTTP and the drainer
	// have quiesced. A failed bound still closes it to fence any late writes.
	if old.receive != nil {
		old.release(resourceDestination)
	}
	old.stop()
	cleanupErr := c.unwind(old)
	if cleanupErr != nil {
		if old.receive != nil {
			cleanupErr = errors.Join(cleanupErr, c.closeDestinationBounded(old))
		}
		old.seq++
		c.publish(Event{SessionID: old.id, Seq: old.seq, Kind: TransferReset})
		return ConnectionRefresh{}, &RecoveryFailure{c.finishFailedRecovery(old, c.retireFailure(old, cleanupErr))}
	}

	old.seq++
	c.publish(Event{SessionID: old.id, Seq: old.seq, Kind: TransferReset})
	// Cancel and Shutdown can mark the old session while the old listener is
	// retiring. Check under mu before handing ownership to a new generation.
	c.mu.Lock()
	if c.closing || old.cancelled || ctx.Err() != nil {
		c.session = nil
		c.state = stateIdle
		c.mu.Unlock()
		closeErr := c.closeRetainedDestination(old)
		c.releaseLease()
		if closeErr != nil {
			return ConnectionRefresh{}, &RecoveryFailure{closeErr}
		}
		return ConnectionRefresh{}, &RecoveryFailure{NewError(ErrCancelled, "connection refresh was cancelled")}
	}
	c.mu.Unlock()

	newID, token, err := c.newIdentity()
	if err != nil {
		if closeErr := c.closeRetainedDestination(old); closeErr != nil {
			err = closeErr
		}
		return ConnectionRefresh{}, &RecoveryFailure{c.finishFailedRecovery(old, err)}
	}
	newCtx, stop := context.WithCancel(context.WithoutCancel(ctx))
	live := &session{id: newID, token: token, ctx: newCtx, cancel: stop, warnings: make([]Warning, 0, 1)}
	if old.receive != nil {
		live.receive = old.receive
		live.hold(resourceDestination)
	}
	c.mu.Lock()
	if c.closing || old.cancelled || ctx.Err() != nil {
		c.session = nil
		c.state = stateIdle
		c.mu.Unlock()
		stop()
		closeErr := c.closeRetainedDestination(old)
		c.releaseLease()
		if closeErr != nil {
			return ConnectionRefresh{}, &RecoveryFailure{closeErr}
		}
		return ConnectionRefresh{}, &RecoveryFailure{NewError(ErrCancelled, "connection refresh was cancelled")}
	}
	c.generation++
	live.generation = c.generation
	c.session = live
	c.mu.Unlock()
	setupCtx, stopSetup := context.WithCancel(live.ctx)
	defer stopSetup()
	stopCallerWatch := context.AfterFunc(ctx, stopSetup)
	defer stopCallerWatch()

	if live.receive != nil {
		dest := live.receive.dest
		ready, err := c.activate(ctx, setupCtx, live, ServerStartRequest{SessionID: newID, Token: token, Destination: dest}, uploadPathPrefix, networkID)
		if err != nil {
			_, failure := c.failReceive(live, err)
			return ConnectionRefresh{}, &RecoveryFailure{failure}
		}
		return ConnectionRefresh{Mode: "receive", Receive: &ReceiveMetadata{SessionID: newID, Destination: dest.Name(), URL: ready.url, QR: ready.qr, Warnings: ready.warnings}}, nil
	}
	item, paths, err := c.reinspectForRecovery(setupCtx, old.item)
	if err != nil {
		_, failure := c.failStage(live, err)
		return ConnectionRefresh{}, &RecoveryFailure{failure}
	}
	if err := c.afterStep(ctx, setupCtx, newID, live.generation); err != nil {
		_, failure := c.failStage(live, err)
		return ConnectionRefresh{}, &RecoveryFailure{failure}
	}
	live.item = item
	if item.UnportableNames > 0 {
		live.warnings = append(live.warnings, unportableNamesWarning())
	}
	ready, err := c.activate(ctx, setupCtx, live, ServerStartRequest{SessionID: newID, Token: token, Item: cloneStagedItem(item)}, downloadPathPrefix, networkID)
	if err != nil {
		_, failure := c.failStage(live, err)
		return ConnectionRefresh{}, &RecoveryFailure{failure}
	}
	return ConnectionRefresh{Mode: "send", Send: &FileMetadata{SessionID: newID, Name: item.Name, Size: item.LogicalSize, IsDir: item.Kind == ItemDirectory, IsCollection: item.Kind == ItemCollection, ItemCount: paths, URL: ready.url, QR: ready.qr, Warnings: ready.warnings}}, nil
}

func (c *Coordinator) finishFailedRecovery(old *session, cause error) error {
	c.mu.Lock()
	if c.session == old {
		c.session = nil
		c.state = stateIdle
	}
	c.mu.Unlock()
	c.releaseLease()
	return cause
}

func (c *Coordinator) closeRetainedDestination(old *session) error {
	if old.receive == nil {
		return nil
	}
	if err := c.closeDestinationBounded(old); err != nil {
		return WrapError(ErrCleanupUnconfirmed, "the receive destination did not confirm it closed", err)
	}
	return nil
}

func (c *Coordinator) reinspectForRecovery(ctx context.Context, previous StagedItem) (StagedItem, int, error) {
	if previous.Kind != ItemCollection {
		item, err := c.source.Inspect(ctx, previous.Path)
		if err != nil {
			return StagedItem{}, 0, err
		}
		if (item.Kind != ItemFile && item.Kind != ItemDirectory) || item.LogicalSize < 0 || item.LogicalSize > 9007199254740991 {
			return StagedItem{}, 0, NewError(ErrSetupFailed, "the selected item changed")
		}
		return item, 1, nil
	}
	if previous.Collection == nil || len(previous.Collection.Members) < 2 || len(previous.Collection.Members) > MaxCollectionItems {
		return StagedItem{}, 0, NewError(ErrSetupFailed, "the selected collection is invalid")
	}
	members := make([]StagedItem, 0, len(previous.Collection.Members))
	var total int64
	var unportable int
	overlaps := newOverlapIndex()
	for _, member := range previous.Collection.Members {
		item, err := c.source.Inspect(ctx, member.Path)
		if err != nil {
			return StagedItem{}, 0, err
		}
		if !overlaps.add(item.Path) {
			return StagedItem{}, 0, NewError(ErrInvalidSelection, "selected items overlap")
		}
		if item.Kind != ItemFile && item.Kind != ItemDirectory {
			return StagedItem{}, 0, NewError(ErrSetupFailed, "selection kind is unsupported")
		}
		if item.LogicalSize < 0 || item.LogicalSize > 9007199254740991-total {
			return StagedItem{}, 0, NewError(ErrPathUnsupported, "selection logical size is too large")
		}
		total += item.LogicalSize
		if item.UnportableNames > 0 {
			unportable = 1
		}
		members = append(members, item)
	}
	return StagedItem{Name: strconv.Itoa(len(members)) + " items", Kind: ItemCollection, LogicalSize: total, UnportableNames: unportable, Collection: &StagedCollection{Members: members}}, len(members), nil
}
