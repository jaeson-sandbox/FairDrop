package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"fairdrop/internal/transfer"
)

type textReceivingCoordinator struct {
	*fakeCoordinator
	metadata transfer.ReceiveTextMetadata
	startErr error
	value    string
	readErr  error
	readID   transfer.SessionID
	startCtx context.Context
}

func (f *textReceivingCoordinator) StartReceiveText(ctx context.Context) (transfer.ReceiveTextMetadata, error) {
	f.record("StartReceiveText")
	f.startCtx = ctx
	return f.metadata, f.startErr
}

func (f *textReceivingCoordinator) GetReceivedText(id transfer.SessionID) (string, error) {
	f.record("GetReceivedText")
	f.readID = id
	return f.value, f.readErr
}

func TestAppReceiveTextCommandsOwnNativeClipboardAndExposeNoValueInMetadata(t *testing.T) {
	h := newHarness(t)
	f := &textReceivingCoordinator{fakeCoordinator: h.coordinator, value: strings.Repeat("雪", 2000),
		metadata: transfer.ReceiveTextMetadata{SessionID: testSessionID,
			URL: "http://192.168.1.50:45678/receive-text/1112131415161718191a1b1c1d1e1f20", QR: "iVBORw0KGgo=", Warnings: []transfer.Warning{}}}
	h.app.useCoordinator(f)
	metadata, err := h.app.StartReceiveText()
	if err != nil || metadata == nil || metadata.SessionID != testSessionID || f.startCtx == nil {
		t.Fatalf("StartReceiveText = %+v, %v", metadata, err)
	}
	if len(h.clipboardWrites()) != 0 {
		t.Fatal("start wrote the clipboard")
	}
	value, err := h.app.GetReceivedText(string(testSessionID))
	if err != nil || value != f.value || f.readID != testSessionID {
		t.Fatalf("GetReceivedText = %q, %v", value, err)
	}
	if err := h.app.CopyReceivedText(string(testSessionID)); err != nil {
		t.Fatal(err)
	}
	if got := h.clipboardWrites(); len(got) != 1 || got[0] != f.value {
		t.Fatalf("native writes = %q", got)
	}
	// This value exceeds CopyToClipboard's link limit; the received-text command
	// must retain the separate session-owned limit without widening that command.
	if err := h.app.CopyToClipboard(f.value); transfer.ErrorCodeOf(err) != transfer.ErrClipboardFailed {
		t.Fatalf("generic copy accepted long value: %v", err)
	}
}

func TestAppReceiveTextReadRefusalAndClipboardFailureAreSafe(t *testing.T) {
	h := newHarness(t)
	f := &textReceivingCoordinator{fakeCoordinator: h.coordinator, readErr: transfer.NewError(transfer.ErrNotReady, "received text is unavailable")}
	h.app.useCoordinator(f)
	if err := h.app.CopyReceivedText("wrong"); transfer.ErrorCodeOf(err) != transfer.ErrNotReady || len(h.clipboardWrites()) != 0 {
		t.Fatalf("stale copy = %v, writes=%q", err, h.clipboardWrites())
	}
	f.readErr = nil
	f.value = "payload <secret>"
	h.clipboardErr = errors.New("platform secret: <secret>")
	if err := h.app.CopyReceivedText(string(testSessionID)); transfer.ErrorCodeOf(err) != transfer.ErrClipboardFailed || strings.Contains(err.Error(), "platform secret") {
		t.Fatalf("clipboard refusal = %v", err)
	}
}
