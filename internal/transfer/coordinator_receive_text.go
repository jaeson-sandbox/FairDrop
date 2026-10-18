package transfer

import "context"

// receivedTextSession is owned by the coordinator mutex. The accepted value
// never enters a lifecycle event or a stage acknowledgement.
type receivedTextSession struct {
	value   *string
	success bool
}

// StartReceiveText creates an empty receiver without opening a source, sink or
// clipboard. Admission and setup use the same single-session lease as files.
func (c *Coordinator) StartReceiveText(ctx context.Context) (ReceiveTextMetadata, error) {
	if ctx == nil {
		return ReceiveTextMetadata{}, NewError(ErrSetupFailed, "receiving requires a context")
	}
	if err := c.ready(); err != nil {
		return ReceiveTextMetadata{}, err
	}
	id, token, err := c.newIdentity()
	if err != nil {
		return ReceiveTextMetadata{}, err
	}
	live, err := c.admit(ctx, id, token)
	if err != nil {
		return ReceiveTextMetadata{}, err
	}
	c.mu.Lock()
	live.receiveText = &receivedTextSession{}
	c.mu.Unlock()
	setupCtx, stopSetup := context.WithCancel(live.ctx)
	defer stopSetup()
	stopCallerWatch := context.AfterFunc(ctx, stopSetup)
	defer stopCallerWatch()
	ready, err := c.activate(ctx, setupCtx, live, ServerStartRequest{SessionID: id, Token: token, ReceiveText: true}, receiveTextPathPrefix)
	if err != nil {
		_, failure := c.failStage(live, err)
		return ReceiveTextMetadata{}, failure
	}
	return ReceiveTextMetadata{SessionID: id, URL: ready.url, QR: ready.qr, Warnings: ready.warnings}, nil
}

// CommitReceivedText accepts only the current claimed text receiver. The
// caller has already validated the value; the repeated check makes the memory
// boundary safe even if a different ServerPort implementation calls it.
func (c *Coordinator) CommitReceivedText(ctx context.Context, id SessionID, raw string) error {
	value, err := NormalizeText(raw)
	if err != nil {
		return err
	}
	if c == nil || ctx == nil || id == "" {
		return NewError(ErrCancelled, "the receive session is no longer available")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.revalidateLocked(ctx, id, 0, stateTransferring); err != nil {
		return err
	}
	received := c.session.receiveText
	if received == nil || received.value != nil || c.session.terminal {
		return NewError(ErrCancelled, "the receive session is no longer available")
	}
	received.value = &value
	return nil
}

// GetReceivedText exposes the value only after the current session's success
// is accepted. terminal is set before publication, so an event listener can
// fetch while the final state store is still pending.
func (c *Coordinator) GetReceivedText(id SessionID) (string, error) {
	if c == nil || id == "" {
		return "", NewError(ErrNotReady, "received text is unavailable")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.session == nil || c.session.id != id || c.session.cancelled || c.closing || c.session.receiveText == nil || !c.session.receiveText.success || c.session.receiveText.value == nil {
		return "", NewError(ErrNotReady, "received text is unavailable")
	}
	return *c.session.receiveText.value, nil
}
