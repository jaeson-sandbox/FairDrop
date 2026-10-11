package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"fairdrop/internal/transfer"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Covers the app.go half of phone-to-desktop receiving: the three bound
// commands, the Show in Folder argument vector, and the wire shape Story B's
// frontend reads. As with every App test, the Wails runtime is replaced by seams
// and the coordinator by a fake; the real stack is driven in
// receive_native_test.go.

// receivingCoordinator is the fake coordinator that also receives.
type receivingCoordinator struct {
	*fakeCoordinator

	startMetadata transfer.ReceiveMetadata
	startErr      error
	startFolder   string
	startCtx      context.Context

	folder    string
	folderErr error
}

func (r *receivingCoordinator) StartReceive(ctx context.Context, folder string) (transfer.ReceiveMetadata, error) {
	r.record("StartReceive")
	r.mu.Lock()
	r.startFolder, r.startCtx = folder, ctx
	r.mu.Unlock()
	return r.startMetadata, r.startErr
}

func (r *receivingCoordinator) ReceivedFolder() (string, error) {
	r.record("ReceivedFolder")
	return r.folder, r.folderErr
}

func receiveMetadata() transfer.ReceiveMetadata {
	return transfer.ReceiveMetadata{
		SessionID:   testSessionID,
		Destination: "Phone Photos",
		URL:         "http://192.168.1.50:45678/upload/" + testToken,
		QR:          "iVBORw0KGgo=",
		Warnings:    []transfer.Warning{},
	}
}

func newReceivingHarness(t *testing.T) (*harness, *receivingCoordinator) {
	t.Helper()
	h := newHarness(t)
	coordinator := &receivingCoordinator{fakeCoordinator: h.coordinator, startMetadata: receiveMetadata()}
	h.app.useCoordinator(coordinator)
	return h, coordinator
}

// --- SelectReceiveFolder ---------------------------------------------------

func TestSelectReceiveFolderOpensTheDirectoryChooserAtDownloads(t *testing.T) {
	h := newHarness(t)
	home := t.TempDir()
	downloads := filepath.Join(home, "Downloads")
	if err := os.Mkdir(downloads, 0o755); err != nil {
		t.Fatal(err)
	}
	h.app.homeDir = func() (string, error) { return home, nil }

	var mu sync.Mutex
	var got []wailsruntime.OpenDialogOptions
	h.app.openDirectory = func(_ context.Context, options wailsruntime.OpenDialogOptions) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, options)
		return "/chosen/folder", nil
	}
	h.app.openFile = func(context.Context, wailsruntime.OpenDialogOptions) (string, error) {
		t.Error("SelectReceiveFolder opened the file chooser")
		return "", nil
	}

	folder, err := h.app.SelectReceiveFolder()
	if err != nil || folder != "/chosen/folder" {
		t.Fatalf("SelectReceiveFolder = %q, %v, want the chosen folder", folder, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("the directory chooser opened %d times, want 1", len(got))
	}
	if got[0].DefaultDirectory != downloads {
		t.Errorf("the chooser opened at %q, want the Downloads folder %q", got[0].DefaultDirectory, downloads)
	}
	if got[0].Title != "Choose where to save received files" {
		t.Errorf("chooser title = %q", got[0].Title)
	}
	if !got[0].CanCreateDirectories {
		t.Error("the receive chooser cannot create a folder")
	}
	if calls := h.coordinator.log(); len(calls) != 0 {
		t.Errorf("choosing a folder reached the coordinator: %v", calls)
	}
}

func TestSelectReceiveFolderFallsBackToHomeThenToWherever(t *testing.T) {
	home := t.TempDir()
	for name, testCase := range map[string]struct {
		homeDir func() (string, error)
		want    string
	}{
		"no Downloads folder opens at home": {func() (string, error) { return home, nil }, home},
		"an unreadable home opens anywhere": {func() (string, error) { return "", errors.New("no home") }, ""},
		"a missing home opens anywhere":     {func() (string, error) { return filepath.Join(home, "gone"), nil }, ""},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.app.homeDir = testCase.homeDir
			if _, err := h.app.SelectReceiveFolder(); err != nil {
				t.Fatalf("SelectReceiveFolder = %v", err)
			}
			h.mu.Lock()
			defer h.mu.Unlock()
			if len(h.dialogFolders) != 1 || h.dialogFolders[0] != testCase.want {
				t.Fatalf("the chooser opened at %v, want %q", h.dialogFolders, testCase.want)
			}
		})
	}
}

func TestDismissingTheReceiveChooserIsANoOp(t *testing.T) {
	h, coordinator := newReceivingHarness(t)
	h.dialogPath = ""
	folder, err := h.app.SelectReceiveFolder()
	if folder != "" || err != nil {
		t.Fatalf("a dismissed chooser = %q, %v, want an empty selection and no error", folder, err)
	}
	if calls := coordinator.log(); len(calls) != 0 {
		t.Fatalf("a dismissed chooser reached the coordinator: %v", calls)
	}
	if got := h.emitted(); len(got) != 0 {
		t.Fatalf("a dismissed chooser emitted %d events", len(got))
	}
}

func TestSelectReceiveFolderReportsAChooserFailureWithTheChooserCode(t *testing.T) {
	h := newHarness(t)
	h.dialogErr = errors.New(`dialog failed for /Users/secret/path`)
	_, err := h.app.SelectReceiveFolder()
	if transfer.ErrorCodeOf(err) != transfer.ErrChooserFailed {
		t.Fatalf("SelectReceiveFolder error = %v, want chooser_failed", err)
	}
	if strings.Contains(err.Error(), "/Users/secret") {
		t.Fatalf("the chooser error disclosed a path: %v", err)
	}

	unstarted := newUnstartedHarness(t)
	if _, err := unstarted.app.SelectReceiveFolder(); transfer.ErrorCodeOf(err) != transfer.ErrNotReady {
		t.Fatalf("SelectReceiveFolder before startup = %v, want not_ready", err)
	}
	if titles := unstarted.dialogs(); len(titles) != 0 {
		t.Fatalf("a dialog was opened without a window context: %v", titles)
	}
}

// --- StartReceive ------------------------------------------------------------

func TestStartReceiveDelegatesWithTheCommandContextAndReturnsMetadataUnchanged(t *testing.T) {
	h, coordinator := newReceivingHarness(t)

	metadata, err := h.app.StartReceive("/Users/receiver/Phone Photos")
	if err != nil {
		t.Fatalf("StartReceive = %v", err)
	}
	if !reflect.DeepEqual(*metadata, receiveMetadata()) {
		t.Fatalf("StartReceive returned %+v, want the coordinator's metadata unchanged", *metadata)
	}
	if coordinator.startFolder != "/Users/receiver/Phone Photos" {
		t.Errorf("the coordinator was handed %q", coordinator.startFolder)
	}
	if coordinator.startCtx != h.app.commands {
		t.Error("StartReceive did not run on the cancellable command context")
	}
}

func TestStartReceivePropagatesARefusalWithNoMetadata(t *testing.T) {
	h, coordinator := newReceivingHarness(t)
	coordinator.startErr = transfer.NewError(transfer.ErrBusy, "busy")

	metadata, err := h.app.StartReceive("/x")
	if metadata != nil || transfer.ErrorCodeOf(err) != transfer.ErrBusy {
		t.Fatalf("StartReceive = %v, %v, want no metadata and busy", metadata, err)
	}
}

func TestStartReceiveBeforeCompositionOrWithoutAReceivingCoordinatorIsNotReady(t *testing.T) {
	for name, build := range map[string]func(t *testing.T) *App{
		"before startup":        func(t *testing.T) *App { return newUnstartedHarness(t).app },
		"without a coordinator": func(t *testing.T) *App { h := newHarness(t); h.app.useCoordinator(nil); return h.app },
		"a coordinator that cannot receive": func(t *testing.T) *App {
			return newHarness(t).app // the plain fake has no StartReceive
		},
	} {
		t.Run(name, func(t *testing.T) {
			metadata, err := build(t).StartReceive("/x")
			if metadata != nil || transfer.ErrorCodeOf(err) != transfer.ErrNotReady {
				t.Fatalf("StartReceive = %v, %v, want no metadata and not_ready", metadata, err)
			}
		})
	}
}

// --- ShowReceivedFolder -------------------------------------------------------

func TestShowReceivedFolderHandsTheSubfolderToTheOSAsOneArgument(t *testing.T) {
	h, coordinator := newReceivingHarness(t)
	coordinator.folder = "/Users/receiver/Phone Photos/FairDrop 2026-10-10 14.05"
	var opened []string
	h.app.openFolder = func(dir string) error {
		opened = append(opened, dir)
		return nil
	}

	if err := h.app.ShowReceivedFolder(); err != nil {
		t.Fatalf("ShowReceivedFolder = %v", err)
	}
	if len(opened) != 1 || opened[0] != "/Users/receiver/Phone Photos/FairDrop 2026-10-10 14.05" {
		t.Fatalf("the OS was handed %v, want the subfolder exactly once", opened)
	}
}

func TestShowReceivedFolderRefusesWhenThereIsNoSubfolder(t *testing.T) {
	h, coordinator := newReceivingHarness(t)
	coordinator.folderErr = transfer.NewError(transfer.ErrPathNotFound, "none")
	h.app.openFolder = func(string) error {
		t.Error("the OS was asked to open a folder that does not exist")
		return nil
	}
	if err := h.app.ShowReceivedFolder(); transfer.ErrorCodeOf(err) != transfer.ErrPathNotFound {
		t.Fatalf("ShowReceivedFolder = %v, want path_not_found", err)
	}
}

func TestAFailedOSLaunchIsACodedErrorThatNamesNoPath(t *testing.T) {
	h, coordinator := newReceivingHarness(t)
	coordinator.folder = "/Users/receiver/Phone Photos/FairDrop 2026-10-10 14.05"
	h.app.openFolder = func(dir string) error { return errors.New("exec: failed for " + dir) }

	err := h.app.ShowReceivedFolder()
	if transfer.ErrorCodeOf(err) != transfer.ErrPathNotFound {
		t.Fatalf("ShowReceivedFolder = %v, want path_not_found", err)
	}
	if strings.Contains(err.Error(), "Phone Photos") {
		t.Fatalf("the launch error disclosed the folder: %v", err)
	}
}

func TestShowReceivedFolderWithoutACoordinatorIsNotReady(t *testing.T) {
	if err := newUnstartedHarness(t).app.ShowReceivedFolder(); transfer.ErrorCodeOf(err) != transfer.ErrNotReady {
		t.Fatalf("ShowReceivedFolder on a coordinator that cannot show = %v, want not_ready", err)
	}
	h := newHarness(t)
	h.app.useCoordinator(nil)
	if err := h.app.ShowReceivedFolder(); transfer.ErrorCodeOf(err) != transfer.ErrNotReady {
		t.Fatalf("ShowReceivedFolder with no coordinator = %v, want not_ready", err)
	}
}

// The argument vector is the whole of the no-shell guarantee: a program name and
// one argument per path, whatever the path contains.
func TestTheFileManagerCommandIsAnArgumentVectorNeverAShellLine(t *testing.T) {
	hostile := []string{
		"/Users/receiver/Phone Photos/FairDrop 2026-10-10 14.05",
		`/tmp/a b; rm -rf ~ & echo "x" | cat $(whoami) ` + "`id`" + ` > /tmp/owned`,
		"/tmp/--help",
		"/tmp/it's a 'quoted' dir",
		`C:\Users\x\Phone Photos & More\FairDrop 2026-10-10 14.05`,
	}
	for _, goos := range []struct {
		name string
		want string
	}{
		{"darwin", "open"},
		{"windows", "explorer.exe"},
		{"linux", "xdg-open"},
		{"freebsd", "xdg-open"},
	} {
		for _, dir := range hostile {
			if !filepath.IsAbs(dir) {
				continue // a Windows path on a POSIX host is not an absolute path to it
			}
			name, args, err := fileManagerCommand(goos.name, dir)
			if err != nil {
				t.Fatalf("%s %q: %v", goos.name, dir, err)
			}
			if name != goos.want {
				t.Errorf("%s: program = %q, want %q", goos.name, name, goos.want)
			}
			if len(args) != 1 || args[0] != dir {
				t.Errorf("%s: arguments = %q, want the path as exactly one untouched argument", goos.name, args)
			}
			for _, shell := range []string{"sh", "bash", "zsh", "cmd", "cmd.exe", "powershell", "powershell.exe", "pwsh"} {
				if name == shell {
					t.Errorf("%s: the program is the shell %q", goos.name, shell)
				}
			}
		}
	}
}

func TestTheFileManagerCommandRefusesAnUnusablePath(t *testing.T) {
	for _, dir := range []string{"", "relative/folder", "FairDrop 2026", "\x00/tmp/x", "/tmp/x\x00y"} {
		if _, _, err := fileManagerCommand("darwin", dir); transfer.ErrorCodeOf(err) != transfer.ErrPathNotFound {
			t.Errorf("fileManagerCommand(%q) = %v, want a path_not_found refusal", dir, err)
		}
	}
}

func TestNewAppShipsTheNativeFileManagerLauncher(t *testing.T) {
	app := NewApp()
	if app.openFolder == nil {
		t.Fatal("NewApp left the Show in Folder launcher unset")
	}
	if reflect.ValueOf(app.openFolder).Pointer() != reflect.ValueOf(openFolderNatively).Pointer() {
		t.Fatal("NewApp does not use the native launcher")
	}
	if err := openFolderNatively("relative"); transfer.ErrorCodeOf(err) != transfer.ErrPathNotFound {
		t.Fatalf("the native launcher accepted a relative path: %v", err)
	}
}

// --- Events and the wire shape -------------------------------------------------

func TestPublishEmitsTheNoticeEventUnderItsOwnName(t *testing.T) {
	h := newHarness(t)
	h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 2, Kind: transfer.TransferNotice, Notice: transfer.NoticeReceiveTooLarge})

	events := h.emitted()
	if len(events) != 1 || events[0].name != "transfer-notice" {
		t.Fatalf("emissions = %+v, want one transfer-notice", events)
	}
	if got := h.app.undelivered.Load(); got != 0 {
		t.Fatalf("the notice was counted undelivered %d times", got)
	}
}

// The frontend reads these exact keys. Spelled out as literals: deriving them
// from the structs would let a rename change the wire with the whole suite green.
func TestTheReceiveWireShapesAreExactlyWhatTheFrontendReads(t *testing.T) {
	metadata, err := json.Marshal(receiveMetadata())
	if err != nil {
		t.Fatal(err)
	}
	wantMetadata := `{"sessionId":"0102030405060708090a0b0c0d0e0f10","destination":"Phone Photos","url":"http://192.168.1.50:45678/upload/1112131415161718191a1b1c1d1e1f20","qrBase64":"iVBORw0KGgo=","warnings":[]}`
	if string(metadata) != wantMetadata {
		t.Errorf("ReceiveMetadata = %s\nwant %s", metadata, wantMetadata)
	}

	cancelled := transfer.PublicErrorOf(transfer.NewError(transfer.ErrCancelled, "x"))
	for name, testCase := range map[string]struct {
		event transfer.Event
		want  string
	}{
		"progress": {
			transfer.Event{SessionID: testSessionID, Seq: 3, Kind: transfer.TransferProgress,
				Progress: &transfer.ProgressSnapshot{BytesSent: 100, TotalBytes: 400, TotalKnown: true, Percent: 25},
				Receive:  &transfer.ReceiveStatus{FilesSaved: 2}},
			`{"sessionId":"0102030405060708090a0b0c0d0e0f10","seq":3,"progress":{"bytesSent":100,"totalBytes":400,"totalKnown":true,"percent":25,"speedBytesPerSec":0},"receive":{"filesSaved":2,"subfolderExists":false,"markingWarning":false}}`,
		},
		"complete": {
			transfer.Event{SessionID: testSessionID, Seq: 4, Kind: transfer.TransferComplete,
				Receive: &transfer.ReceiveStatus{FilesSaved: 3, Result: transfer.ReceiveComplete, SubfolderExists: true}},
			`{"sessionId":"0102030405060708090a0b0c0d0e0f10","seq":4,"receive":{"filesSaved":3,"result":"complete","subfolderExists":true,"markingWarning":false}}`,
		},
		"cancelled": {
			transfer.Event{SessionID: testSessionID, Seq: 5, Kind: transfer.TransferError, Error: &cancelled,
				Receive: &transfer.ReceiveStatus{FilesSaved: 1, Result: transfer.ReceiveCancelled, SubfolderExists: true, MarkingWarning: true}},
			`{"sessionId":"0102030405060708090a0b0c0d0e0f10","seq":5,"error":{"code":"cancelled","message":"Transfer canceled."},"receive":{"filesSaved":1,"result":"cancelled","subfolderExists":true,"markingWarning":true}}`,
		},
		"notice": {
			transfer.Event{SessionID: testSessionID, Seq: 1, Kind: transfer.TransferNotice, Notice: transfer.NoticeReceiveTooLarge},
			`{"sessionId":"0102030405060708090a0b0c0d0e0f10","seq":1,"notice":"receive_too_large"}`,
		},
		"a send event is unchanged": {
			transfer.Event{SessionID: testSessionID, Seq: 1, Kind: transfer.TransferStarted},
			`{"sessionId":"0102030405060708090a0b0c0d0e0f10","seq":1}`,
		},
	} {
		encoded, err := json.Marshal(testCase.event)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if string(encoded) != testCase.want {
			t.Errorf("%s event = %s\nwant %s", name, encoded, testCase.want)
		}
	}
}

func TestEventLogLinesCarryCountsAndFixedWordsOnly(t *testing.T) {
	h := newHarness(t)
	h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 4, Kind: transfer.TransferComplete,
		Receive: &transfer.ReceiveStatus{FilesSaved: 3, Result: transfer.ReceiveComplete, SubfolderExists: true}})
	h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 1, Kind: transfer.TransferNotice, Notice: transfer.NoticeReceiveTooLarge})

	logs := h.logged()
	want := []string{
		"fairdrop: event transfer-complete seq=4 session=" + string(testSessionID) + " files=3 result=complete",
		"fairdrop: event transfer-notice seq=1 session=" + string(testSessionID) + " notice=receive_too_large",
	}
	if strings.Join(logs, "\n") != strings.Join(want, "\n") {
		t.Fatalf("log lines =\n%s\nwant\n%s", strings.Join(logs, "\n"), strings.Join(want, "\n"))
	}
}
