package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"fairdrop/internal/qr"
	"fairdrop/internal/server"
	"fairdrop/internal/sink"
	"fairdrop/internal/source"
	"fairdrop/internal/stream"
	"fairdrop/internal/transfer"
)

// These drive the bound commands over the real coordinator, sink, server, QR
// encoder and a real directory. Only network discovery is fixed to loopback, as
// in the send matrix: no external LAN is required.

const nativeBoundary = "nativeboundary61c2"

func nativeReceiveApp(t *testing.T) (*App, chan transfer.Event, *[]string) {
	t.Helper()
	app := NewApp()
	app.logf = func(string, ...any) {}
	events := make(chan transfer.Event, 128)
	app.emit = func(_ context.Context, _ string, args ...any) { events <- args[0].(transfer.Event) }
	opened := &[]string{}
	app.openFolder = func(dir string) error {
		*opened = append(*opened, dir)
		return nil
	}
	app.startup(context.Background())
	inspector := source.New()
	coordinator := transfer.NewCoordinator(transfer.Dependencies{
		Source: inspector, Network: nativeMatrixNetwork{}, Server: server.New(stream.New(inspector)),
		QR: qr.New(), Sink: sink.New(), Observer: appObserver{app: app},
	})
	app.useCoordinator(coordinator)
	t.Cleanup(func() {
		if err := coordinator.Shutdown(context.Background()); err != nil {
			t.Errorf("receive shutdown code=%s", transfer.ErrorCodeOf(err))
		}
	})
	return app, events, opened
}

func nativeNextEvent(t *testing.T, events <-chan transfer.Event) transfer.Event {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(15 * time.Second):
		t.Fatal("no lifecycle event within 15s")
		return transfer.Event{}
	}
}

func nativeMultipart(parts ...[2]string) []byte {
	var body bytes.Buffer
	for _, part := range parts {
		fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"files\"; filename=%q\r\nContent-Type: application/octet-stream\r\n\r\n%s\r\n", nativeBoundary, part[0], part[1])
	}
	fmt.Fprintf(&body, "--%s--\r\n", nativeBoundary)
	return body.Bytes()
}

func nativeListing(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func nativeFixture(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "Phone Photos")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNativeReceiveSavesFilesReportsTheExactCountAndShowsTheFolder(t *testing.T) {
	app, events, opened := nativeReceiveApp(t)
	dir := nativeFixture(t)

	metadata, err := app.StartReceive(dir)
	if err != nil {
		t.Fatalf("StartReceive refused: code=%s", transfer.ErrorCodeOf(err))
	}
	if metadata.Destination != "Phone Photos" || metadata.SessionID == "" || metadata.QR == "" || !strings.Contains(metadata.URL, "/upload/") || strings.Contains(metadata.URL, "/download/") {
		t.Fatalf("native receive metadata invalid: destination=%q", metadata.Destination)
	}
	if strings.Contains(metadata.URL, dir) || strings.Contains(metadata.QR, "Phone") {
		t.Fatal("the capability link or QR disclosed the folder")
	}
	if _, err := app.StageTransfer(dir); transfer.ErrorCodeOf(err) != transfer.ErrBusy {
		t.Fatalf("a send was admitted during a receive: code=%s", transfer.ErrorCodeOf(err))
	}
	if _, err := app.StartReceive(dir); transfer.ErrorCodeOf(err) != transfer.ErrBusy {
		t.Fatalf("a second receive was admitted: code=%s", transfer.ErrorCodeOf(err))
	}
	if err := app.ShowReceivedFolder(); transfer.ErrorCodeOf(err) != transfer.ErrPathNotFound || len(*opened) != 0 {
		t.Fatalf("Show in Folder before any file: code=%s opened=%v", transfer.ErrorCodeOf(err), *opened)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	page, err := client.Get(metadata.URL)
	if err != nil {
		t.Fatal("native inspection request failed")
	}
	pageBody, _ := io.ReadAll(page.Body)
	_ = page.Body.Close()
	if page.StatusCode != http.StatusOK || !bytes.Contains(pageBody, []byte(`name="files" multiple`)) || bytes.Contains(pageBody, []byte(dir)) {
		t.Fatalf("native upload page invalid: status=%d", page.StatusCode)
	}
	select {
	case event := <-events:
		t.Fatalf("GET published lifecycle event %q", event.Kind)
	default:
	}
	if got := nativeListing(t, dir); len(got) != 0 {
		t.Fatalf("a GET wrote %v", got)
	}

	body := nativeMultipart([2]string{"IMG_1.jpg", strings.Repeat("a", 90_000)}, [2]string{"IMG_1.jpg", "second"}, [2]string{"notes.txt", "third"})
	response, err := client.Post(metadata.URL, "multipart/form-data; boundary="+nativeBoundary, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("native upload failed: %v", err)
	}
	resultPage, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Contains(resultPage, []byte("3 files saved to this computer.")) {
		t.Fatalf("native result page invalid: status=%d", response.StatusCode)
	}

	var sequence []transfer.EventKind
	var complete transfer.Event
	for complete.Kind == "" {
		event := nativeNextEvent(t, events)
		sequence = append(sequence, event.Kind)
		if event.Kind == transfer.TransferComplete {
			complete = event
		}
		if event.Kind == transfer.TransferError {
			t.Fatalf("native upload published an error: %+v", event.Error)
		}
	}
	if sequence[0] != transfer.TransferStarted || sequence[len(sequence)-2] != transfer.TransferProgress {
		t.Fatalf("event grammar = %v, want started first and a final progress before complete", sequence)
	}
	want := transfer.ReceiveStatus{FilesSaved: 3, Result: transfer.ReceiveComplete, SubfolderExists: true}
	if complete.Receive == nil || *complete.Receive != want {
		t.Fatalf("complete outcome = %+v, want %+v", complete.Receive, want)
	}
	if complete.Progress == nil || complete.Progress.BytesSent != int64(len(body)) || complete.Progress.TotalBytes != int64(len(body)) {
		t.Fatalf("complete progress = %+v, want %d of %d", complete.Progress, len(body), len(body))
	}

	subs := nativeListing(t, dir)
	if len(subs) != 1 || !strings.HasPrefix(subs[0], "FairDrop ") {
		t.Fatalf("destination holds %v, want one FairDrop subfolder", subs)
	}
	sub := filepath.Join(dir, subs[0])
	if got := nativeListing(t, sub); strings.Join(got, "|") != "IMG_1 (1).jpg|IMG_1.jpg|notes.txt" {
		t.Fatalf("subfolder holds %v", got)
	}

	// The outcome is held for Show in Folder rather than reset after three seconds.
	if err := app.ShowReceivedFolder(); err != nil {
		t.Fatalf("ShowReceivedFolder: code=%s", transfer.ErrorCodeOf(err))
	}
	if len(*opened) != 1 || (*opened)[0] != sub {
		t.Fatalf("Show in Folder opened %v, want the subfolder", *opened)
	}
	select {
	case event := <-events:
		t.Fatalf("a held outcome published %q on its own", event.Kind)
	case <-time.After(300 * time.Millisecond):
	}

	if err := app.CancelTransfer(); err != nil {
		t.Fatalf("leaving the outcome: code=%s", transfer.ErrorCodeOf(err))
	}
	if event := nativeNextEvent(t, events); event.Kind != transfer.TransferReset {
		t.Fatalf("leaving the outcome published %q, want a reset", event.Kind)
	}
	if err := app.ShowReceivedFolder(); transfer.ErrorCodeOf(err) != transfer.ErrPathNotFound {
		t.Fatalf("Show in Folder after leaving: code=%s", transfer.ErrorCodeOf(err))
	}
}

func TestNativeReceiveCancelMidFileKeepsCompletedFilesAndReportsCancelled(t *testing.T) {
	app, events, opened := nativeReceiveApp(t)
	dir := nativeFixture(t)
	metadata, err := app.StartReceive(dir)
	if err != nil {
		t.Fatalf("StartReceive refused: code=%s", transfer.ErrorCodeOf(err))
	}
	parsed, err := url.Parse(metadata.URL)
	if err != nil {
		t.Fatal(err)
	}

	full := nativeMultipart([2]string{"done.bin", "COMPLETE FILE"}, [2]string{"cut.bin", strings.Repeat("P", 300_000)})
	cut := bytes.Index(full, []byte("cut.bin")) + 5000
	conn, err := net.DialTimeout("tcp", parsed.Host, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: %s\r\nContent-Type: multipart/form-data; boundary=%s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n",
		parsed.Path, parsed.Host, nativeBoundary, len(full))
	if _, err := conn.Write(full[:cut]); err != nil {
		t.Fatal(err)
	}

	if started := nativeNextEvent(t, events); started.Kind != transfer.TransferStarted {
		t.Fatalf("first event = %q, want started", started.Kind)
	}
	// Wait for the partial file to exist, so the cancel lands genuinely mid-file.
	deadline := time.Now().Add(10 * time.Second)
	for {
		subs := nativeListing(t, dir)
		if len(subs) == 1 {
			inner := nativeListing(t, filepath.Join(dir, subs[0]))
			if len(inner) == 2 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the partial file never appeared; destination holds %v", subs)
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := app.CancelTransfer(); err != nil {
		t.Fatalf("CancelTransfer: code=%s", transfer.ErrorCodeOf(err))
	}
	var outcome transfer.Event
	for outcome.Kind == "" {
		event := nativeNextEvent(t, events)
		switch event.Kind {
		case transfer.TransferProgress:
		case transfer.TransferError:
			outcome = event
		default:
			t.Fatalf("cancelling mid-upload published %q, want progress then one cancelled outcome", event.Kind)
		}
	}
	want := transfer.ReceiveStatus{FilesSaved: 1, Result: transfer.ReceiveCancelled, SubfolderExists: true}
	if outcome.Receive == nil || *outcome.Receive != want {
		t.Fatalf("cancelled outcome = %+v, want %+v", outcome.Receive, want)
	}
	if outcome.Error == nil || outcome.Error.Code != transfer.ErrCancelled {
		t.Fatalf("cancelled outcome error = %+v, want the cancelled code", outcome.Error)
	}

	subs := nativeListing(t, dir)
	if len(subs) != 1 {
		t.Fatalf("destination holds %v, want one subfolder", subs)
	}
	sub := filepath.Join(dir, subs[0])
	if got := nativeListing(t, sub); strings.Join(got, "|") != "done.bin" {
		t.Fatalf("subfolder holds %v, want the completed file kept and the partial one gone", got)
	}
	if err := app.ShowReceivedFolder(); err != nil || len(*opened) != 1 || (*opened)[0] != sub {
		t.Fatalf("Show in Folder after a cancel: err=%v opened=%v", err, *opened)
	}

	if err := app.CancelTransfer(); err != nil {
		t.Fatalf("leaving the cancelled outcome: code=%s", transfer.ErrorCodeOf(err))
	}
	if event := nativeNextEvent(t, events); event.Kind != transfer.TransferReset {
		t.Fatalf("leaving published %q, want a reset", event.Kind)
	}
}

func TestNativeReceiveRefusesUnusableFoldersBeforeAnyNetworkResource(t *testing.T) {
	app, events, _ := nativeReceiveApp(t)
	base := filepath.Dir(nativeFixture(t))

	file := filepath.Join(base, "plain.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link")
	linked := os.Symlink(base, link) == nil

	for name, testCase := range map[string]struct {
		path string
		want transfer.ErrorCode
		skip bool
	}{
		"missing":   {filepath.Join(base, "missing"), transfer.ErrPathNotFound, false},
		"a file":    {file, transfer.ErrPathUnsupported, false},
		"a symlink": {link, transfer.ErrPathUnsupported, !linked},
	} {
		t.Run(name, func(t *testing.T) {
			if testCase.skip {
				t.Skip("this runner cannot create the symbolic link fixture")
			}
			if _, err := app.StartReceive(testCase.path); transfer.ErrorCodeOf(err) != testCase.want {
				t.Fatalf("StartReceive code=%s, want %s", transfer.ErrorCodeOf(err), testCase.want)
			}
			select {
			case event := <-events:
				t.Fatalf("a refusal published %q", event.Kind)
			default:
			}
		})
	}
	if _, err := app.StartReceive(base); err != nil {
		t.Fatalf("a refusal left the coordinator unusable: code=%s", transfer.ErrorCodeOf(err))
	}
}
