package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// This crosses the real browser-form encoding, HTTP server, coordinator and
// bound desktop read/copy commands. Neither a filesystem destination nor a
// frontend-supplied clipboard value participates.
func TestNativeReceiveTextFormReachesHeldDesktopValue(t *testing.T) {
	app, events, _ := nativeReceiveApp(t)
	copied := []string{}
	app.setClipboard = func(_ context.Context, value string) error {
		copied = append(copied, value)
		return nil
	}
	metadata, err := app.StartReceiveText()
	if err != nil {
		t.Fatalf("StartReceiveText: %v", err)
	}
	if metadata.SessionID == "" || !strings.Contains(metadata.URL, "/receive-text/") || strings.Contains(metadata.URL, "<") {
		t.Fatalf("receive-text metadata invalid: %+v", metadata)
	}
	if _, err := app.GetReceivedText(string(metadata.SessionID)); transfer.ErrorCodeOf(err) != transfer.ErrNotReady {
		t.Fatalf("preclaim read code=%s", transfer.ErrorCodeOf(err))
	}
	client := &http.Client{Timeout: 15 * time.Second}
	page, err := client.Get(metadata.URL)
	if err != nil {
		t.Fatal(err)
	}
	form, _ := io.ReadAll(page.Body)
	_ = page.Body.Close()
	if page.StatusCode != http.StatusOK || !strings.Contains(string(form), `name="text"`) {
		t.Fatalf("form status=%d, body=%q", page.StatusCode, form)
	}
	value := "\r\n<em>literal</em> & café\rline"
	response, err := client.PostForm(metadata.URL, url.Values{"text": {value}})
	if err != nil {
		t.Fatal(err)
	}
	ack, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !strings.Contains(string(ack), "Text received by this computer") || strings.Contains(string(ack), "literal") {
		t.Fatalf("ack status=%d, body=%q", response.StatusCode, ack)
	}
	for event := nativeNextEvent(t, events); event.Kind != transfer.TransferComplete; event = nativeNextEvent(t, events) {
		if event.Kind != transfer.TransferStarted && event.Kind != transfer.TransferProgress {
			t.Fatalf("unexpected event: %+v", event)
		}
	}
	want := "\n<em>literal</em> & café\nline"
	got, err := app.GetReceivedText(string(metadata.SessionID))
	if err != nil || got != want {
		t.Fatalf("held value=%q, error=%v; want %q", got, err, want)
	}
	if len(copied) != 0 {
		t.Fatalf("clipboard was written automatically: %q", copied)
	}
	if err := app.CopyReceivedText(string(metadata.SessionID)); err != nil || len(copied) != 1 || copied[0] != want {
		t.Fatalf("explicit copy err=%v, writes=%q", err, copied)
	}
	if err := app.CancelTransfer(); err != nil {
		t.Fatal(err)
	}
	if _, err := app.GetReceivedText(string(metadata.SessionID)); transfer.ErrorCodeOf(err) != transfer.ErrNotReady {
		t.Fatalf("read after Done code=%s", transfer.ErrorCodeOf(err))
	}

	// A second successful receipt is required here: a guard that merely checks
	// for any held receipt would incorrectly serve the new value to the old ID.
	second, err := app.StartReceiveText()
	if err != nil || second.SessionID == metadata.SessionID {
		t.Fatalf("second StartReceiveText = %+v, %v", second, err)
	}
	response, err = client.PostForm(second.URL, url.Values{"text": {"second receipt 雪"}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("second POST status=%d", response.StatusCode)
	}
	for event := nativeNextEvent(t, events); event.Kind != transfer.TransferComplete || event.SessionID != second.SessionID; event = nativeNextEvent(t, events) {
		if event.Kind != transfer.TransferReset && event.Kind != transfer.TransferStarted && event.Kind != transfer.TransferProgress {
			t.Fatalf("unexpected event before second receipt: %+v", event)
		}
	}
	for _, id := range []string{string(metadata.SessionID), "wrong"} {
		if _, err := app.GetReceivedText(id); transfer.ErrorCodeOf(err) != transfer.ErrNotReady {
			t.Fatalf("stale read %q code=%s", id, transfer.ErrorCodeOf(err))
		}
		if err := app.CopyReceivedText(id); transfer.ErrorCodeOf(err) != transfer.ErrNotReady {
			t.Fatalf("stale copy %q code=%s", id, transfer.ErrorCodeOf(err))
		}
	}
	if len(copied) != 1 {
		t.Fatalf("stale copy wrote clipboard: %q", copied)
	}
	if got, err := app.GetReceivedText(string(second.SessionID)); err != nil || got != "second receipt 雪" {
		t.Fatalf("second held value = %q, %v", got, err)
	}
	if err := app.CopyReceivedText(string(second.SessionID)); err != nil || len(copied) != 2 || copied[1] != "second receipt 雪" {
		t.Fatalf("second copy err=%v writes=%q", err, copied)
	}
}
