package main

import (
	"bufio"
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
	"sync/atomic"
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

func nativeReceiveApp(t *testing.T, boundTimer ...func(time.Duration, func()) transfer.StopTimer) (*App, chan transfer.Event, *[]string) {
	t.Helper()
	return nativeReceiveAppWithNetwork(t, nativeMatrixNetwork{}, boundTimer...)
}

func nativeReceiveAppWithNetwork(t *testing.T, network nativeMatrixNetwork, boundTimer ...func(time.Duration, func()) transfer.StopTimer) (*App, chan transfer.Event, *[]string) {
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
	var timer func(time.Duration, func()) transfer.StopTimer
	if len(boundTimer) != 0 {
		timer = boundTimer[0]
	}
	coordinator := transfer.NewCoordinator(transfer.Dependencies{
		Source: inspector, Network: network, Server: server.New(stream.New(inspector)),
		QR: qr.New(), Sink: sink.New(), Observer: appObserver{app: app}, BoundTimer: timer,
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

func TestNativeRefusalNoticeAndReservedUploadUseTheSameLink(t *testing.T) {
	app, events, _ := nativeReceiveApp(t, func(time.Duration, func()) transfer.StopTimer {
		return func() bool { return true }
	})
	entered := make(chan struct{})
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	app.emit = func(_ context.Context, _ string, args ...any) {
		event := args[0].(transfer.Event)
		if event.Kind == transfer.TransferNotice {
			close(entered)
			<-release
		}
		events <- event
	}
	dir := nativeFixture(t)
	metadata, err := app.StartReceive(dir)
	if err != nil {
		t.Fatalf("StartReceive = %v", err)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	// The declared length is deliberately impossible to reserve. The server
	// refuses it from headers, before reading the empty body or spending the
	// one-shot capability.
	parsed, err := url.Parse(metadata.URL)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: %s\r\nContent-Type: multipart/form-data; boundary=%s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", parsed.Path, parsed.Host, nativeBoundary, int64(1<<60))
	if err != nil {
		t.Fatal(err)
	}
	refusal, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("reading oversized refusal = %v", err)
	}
	_ = refusal.Body.Close()
	if refusal.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload = %d, want 413", refusal.StatusCode)
	}
	<-entered

	type uploadResult struct {
		status int
		body   []byte
		err    error
	}
	completed := make(chan uploadResult, 1)
	body := nativeMultipart([2]string{"saved.txt", "native exact bytes"})
	go func() {
		response, err := client.Post(metadata.URL, "multipart/form-data; boundary="+nativeBoundary, bytes.NewReader(body))
		if err != nil {
			completed <- uploadResult{err: err}
			return
		}
		page, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		completed <- uploadResult{status: response.StatusCode, body: page, err: readErr}
	}()

	// GET becomes 423 only after the valid POST has reserved the one-shot
	// capability. This proves it reached coordinator authorization while the
	// notice observer still holds publication open.
	deadline := time.Now().Add(10 * time.Second)
	for {
		probe, err := client.Get(metadata.URL)
		if err != nil {
			t.Fatalf("claim probe = %v", err)
		}
		_ = probe.Body.Close()
		if probe.StatusCode == http.StatusLocked {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("valid upload never reserved link; GET stayed %d", probe.StatusCode)
		}
		time.Sleep(time.Millisecond)
	}
	close(release)
	result := <-completed
	if result.err != nil || result.status != http.StatusOK {
		t.Fatalf("reserved upload after notice = status %d, err %v, page %q", result.status, result.err, result.body)
	}
	if notice := nativeNextEvent(t, events); notice.Kind != transfer.TransferNotice || notice.Seq != 1 {
		t.Fatalf("first event = %+v, want notice seq 1", notice)
	}
	if started := nativeNextEvent(t, events); started.Kind != transfer.TransferStarted || started.Seq != 2 {
		t.Fatalf("second event = %+v, want started seq 2", started)
	}
	for event := nativeNextEvent(t, events); event.Kind != transfer.TransferComplete; event = nativeNextEvent(t, events) {
		if event.Kind != transfer.TransferProgress {
			t.Fatalf("upload event = %+v, want progress or complete", event)
		}
	}
	subfolders := nativeListing(t, dir)
	if len(subfolders) != 1 {
		t.Fatalf("receive folders = %v, want one", subfolders)
	}
	saved, err := os.ReadFile(filepath.Join(dir, subfolders[0], "saved.txt"))
	if err != nil || string(saved) != "native exact bytes" {
		t.Fatalf("saved file = %q, err %v", saved, err)
	}
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

// An unwritable folder is refused when it is chosen -- path_unsupported, no
// listener, no event -- so the one-time link is never shown for a folder that
// cannot take a file and then burned by the first upload.
func TestNativeReceiveRefusesAnUnwritableFolderBeforeAnyNetworkResource(t *testing.T) {
	app, events, _ := nativeReceiveApp(t)
	base := filepath.Dir(nativeFixture(t))
	unwritable := nativeUnwritableDir(t, base)

	if _, err := app.StartReceive(unwritable); transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
		t.Fatalf("StartReceive of an unwritable folder code=%s, want path_unsupported", transfer.ErrorCodeOf(err))
	}
	select {
	case event := <-events:
		t.Fatalf("a refusal published %q", event.Kind)
	default:
	}
	if entries, err := os.ReadDir(unwritable); err != nil || len(entries) != 0 {
		t.Fatalf("the refused folder holds %v (%v), want it untouched", entries, err)
	}
	if _, err := app.StartReceive(base); err != nil {
		t.Fatalf("a refusal left the coordinator unusable: code=%s", transfer.ErrorCodeOf(err))
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

// Receive Again deliberately has no new backend command: each cycle goes
// through CancelTransfer and ordinary StartReceive with the original parent.
func TestNativeReceiveAgainUsesFreshSessionsFoldersValidationAndSpaceChecks(t *testing.T) {
	var networkCalls atomic.Int32
	app, events, _ := nativeReceiveAppWithNetwork(t, nativeMatrixNetwork{calls: &networkCalls})
	dir := nativeFixture(t)
	client := &http.Client{Timeout: 15 * time.Second}
	var previous *transfer.ReceiveMetadata
	for cycle := 0; cycle < 2; cycle++ {
		metadata, err := app.StartReceive(dir)
		if err != nil {
			t.Fatal(err)
		}
		if cycle > 0 {
			if metadata.SessionID == previous.SessionID || metadata.URL == previous.URL || metadata.QR == previous.QR {
				t.Fatal("Receive Again reused a session or capability")
			}
			// Even if the OS reused the old listening port, the retired token
			// must not authorize against the new session's live listener.
			old, _ := url.Parse(previous.URL)
			fresh, _ := url.Parse(metadata.URL)
			fresh.Path = old.Path
			response, err := client.Get(fresh.String())
			if err != nil {
				t.Fatal(err)
			}
			_ = response.Body.Close()
			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("old code admitted by new listener: %d", response.StatusCode)
			}
		}
		// A fresh per-upload space check must run even though the parent
		// was already approved in the preceding cycle.
		parsed, err := url.Parse(metadata.URL)
		if err != nil {
			t.Fatal(err)
		}
		conn, err := net.DialTimeout("tcp", parsed.Host, 5*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatal(err)
		}
		_, err = fmt.Fprintf(conn, "POST %s HTTP/1.1\r\nHost: %s\r\nContent-Type: multipart/form-data; boundary=%s\r\nContent-Length: %d\r\nConnection: close\r\n\r\n", parsed.Path, parsed.Host, nativeBoundary, int64(1<<60))
		if err != nil {
			t.Fatal(err)
		}
		refusal, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Fatal(err)
		}
		_ = refusal.Body.Close()
		_ = conn.Close()
		if refusal.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("cycle %d skipped space refusal: %d", cycle, refusal.StatusCode)
		}
		body := nativeMultipart([2]string{"same.txt", fmt.Sprintf("cycle %d", cycle)})
		response, err := client.Post(metadata.URL, "multipart/form-data; boundary="+nativeBoundary, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK {
			t.Fatalf("upload cycle %d: status %d, error %v", cycle, response.StatusCode, readErr)
		}
		for event := nativeNextEvent(t, events); event.Kind != transfer.TransferComplete; event = nativeNextEvent(t, events) {
			if event.Kind == transfer.TransferError {
				t.Fatalf("upload failed: %+v", event)
			}
		}
		// Complete publication can precede the state commit, but the terminal
		// path still owns the operation lease. Cancel joins that lease through
		// Cancel/awaitLeaseBounded before returning and publishing reset.
		if err := app.CancelTransfer(); err != nil {
			t.Fatal(err)
		}
		if reset := nativeNextEvent(t, events); reset.Kind != transfer.TransferReset {
			t.Fatalf("no actual reset: %+v", reset)
		}
		previous = metadata
	}
	folders := nativeListing(t, dir)
	if len(folders) != 2 {
		t.Fatalf("repeat must create two exclusive folders: %v", folders)
	}
	contents := make(map[string]bool)
	for _, folder := range folders {
		data, err := os.ReadFile(filepath.Join(dir, folder, "same.txt"))
		if err != nil {
			t.Fatal(err)
		}
		contents[string(data)] = true
	}
	if !contents["cycle 0"] || !contents["cycle 1"] {
		t.Fatalf("repeat overwrote a previous upload: %v", contents)
	}
	moved := dir + " moved"
	if err := os.Rename(dir, moved); err != nil {
		t.Fatal(err)
	}
	before := networkCalls.Load()
	assertRefusal := func(t *testing.T, want transfer.ErrorCode) {
		t.Helper()
		if _, err := app.StartReceive(dir); transfer.ErrorCodeOf(err) != want {
			t.Fatalf("repeat parent refusal code=%s, want %s", transfer.ErrorCodeOf(err), want)
		}
		if networkCalls.Load() != before {
			t.Fatal("unsafe repeat destination reached network discovery")
		}
		select {
		case event := <-events:
			t.Fatalf("refusal published %q", event.Kind)
		default:
		}
	}
	assertRefusal(t, transfer.ErrPathNotFound)
	if err := os.WriteFile(dir, []byte("replacement file"), 0o644); err != nil {
		t.Fatal(err)
	}
	assertRefusal(t, transfer.ErrPathUnsupported)
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	t.Run("symlink replacement", func(t *testing.T) {
		nativeMatrixDirectoryLink(t, moved, dir)
		assertRefusal(t, transfer.ErrPathUnsupported)
	})
}
