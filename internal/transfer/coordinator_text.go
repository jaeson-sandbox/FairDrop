package transfer

import "context"

// StageText offers one normalized, bounded in-memory text value. It never
// inspects a source path or opens a receive destination.
func (c *Coordinator) StageText(ctx context.Context, raw string) (TextMetadata, error) {
	value, err := NormalizeText(raw)
	if err != nil {
		return TextMetadata{}, err
	}
	if ctx == nil {
		return TextMetadata{}, NewError(ErrSetupFailed, "staging requires a context")
	}
	if err := c.ready(); err != nil {
		return TextMetadata{}, err
	}
	id, token, err := c.newIdentity()
	if err != nil {
		return TextMetadata{}, err
	}
	live, err := c.admit(ctx, id, token)
	if err != nil {
		return TextMetadata{}, err
	}
	live.text = &value
	setupCtx, stopSetup := context.WithCancel(live.ctx)
	defer stopSetup()
	stopCallerWatch := context.AfterFunc(ctx, stopSetup)
	defer stopCallerWatch()
	ready, err := c.activate(ctx, setupCtx, live, ServerStartRequest{SessionID: id, Token: token, Text: &value}, textPathPrefix)
	if err != nil {
		_, failure := c.failStage(live, err)
		return TextMetadata{}, failure
	}
	return TextMetadata{SessionID: id, ByteLength: len(value), URL: ready.url, QR: ready.qr, Warnings: ready.warnings}, nil
}
