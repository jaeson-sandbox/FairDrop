package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Covers the I/O & Edge-Case Matrix in
// _bmad-output/implementation-artifacts/spec-1-7-expose-safe-transfer-commands-through-wails.md.
//
// app.go is a translation layer, so every test here drives it from both sides:
// a fake coordinator stands in for the transfer implementation, and fake Wails
// seams stand in for a running window. The real runtime cannot appear in a
// test at all -- EventsEmit and both dialogs answer a context that did not
// come from a window with log.Fatalf, which would end the test binary rather
// than fail a case -- so TestNewAppWiresTheRealWailsRuntime pins the
// production defaults separately.

const (
	// testPath is the sender-private absolute path every disclosure assertion
	// searches for. The space and the user directory are deliberate: a leak
	// anywhere is then detected rather than argued about.
	testPath = `C:\Users\sender\Documents\quarterly report.pdf`

	// testToken never legitimately leaves the coordinator, so it is what the
	// disclosure assertions look for beside the path.
	testToken = "1112131415161718191a1b1c1d1e1f20"

	testSessionID = transfer.SessionID("0102030405060708090a0b0c0d0e0f10")
)

// emission is one recorded call to the Wails runtime event emitter.
//
// The context is recorded because it is load-bearing and invisible otherwise:
// the real EventsEmit calls log.Fatalf for a context that did not come from a
// running window, so emitting with anything but the stored one takes the
// process down rather than returning an error.
type emission struct {
	ctx  context.Context
	name string
	data []interface{}
}

// fakeCoordinator is the transfer implementation the App delegates to. It
// records the ordered call log so "this command reached the coordinator" and
// "this command reached nothing" are both assertable.
type fakeCoordinator struct {
	mu    sync.Mutex
	calls []string

	stageMetadata transfer.FileMetadata
	stageErr      error
	stagePath     string
	stageCtx      context.Context

	cancelErr error

	// shutdownGate, when non-nil, holds Shutdown until it is closed. It is how
	// the blocking hook is proven to block.
	shutdownGate chan struct{}
}

func (f *fakeCoordinator) Stage(ctx context.Context, absolutePath string) (transfer.FileMetadata, error) {
	f.record("Stage")
	f.mu.Lock()
	f.stagePath = absolutePath
	f.stageCtx = ctx
	f.mu.Unlock()
	return f.stageMetadata, f.stageErr
}

func (f *fakeCoordinator) Cancel() error {
	f.record("Cancel")
	return f.cancelErr
}

func (f *fakeCoordinator) Shutdown() error {
	f.record("Shutdown")
	if f.shutdownGate != nil {
		<-f.shutdownGate
	}
	return nil
}

func (f *fakeCoordinator) record(call string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, call)
}

func (f *fakeCoordinator) log() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

// harness is one App with every Wails seam replaced and a fake coordinator
// installed, plus the recordings those seams produce.
type harness struct {
	app         *App
	coordinator *fakeCoordinator
	ctx         context.Context

	mu        sync.Mutex
	emissions []emission

	// dialogPath and dialogErr are what both fake dialogs answer with. They
	// are read under h.mu like every other recorded field: tests drive the App
	// from a second goroutine, and a bare read here would make the race
	// detector report the fixture instead of the code.
	dialogPath string
	dialogErr  error
	// dialogCtx records the context the dialog was handed.
	dialogCtx context.Context
	// dialogTitles records each opened dialog in order.
	dialogTitles []string

	// emitPanic, when non-nil, is what the fake emitter panics with.
	emitPanic error

	// clipboardErr is what the fake clipboard answers with; clipboardText and
	// clipboardCtx record what it was handed.
	clipboardErr  error
	clipboardText []string
	clipboardCtx  context.Context
}

// newHarness returns a started App: startup has run, so a.ctx is installed.
func newHarness(t *testing.T) *harness {
	t.Helper()
	h := newUnstartedHarness(t)
	h.app.startup(h.ctx)
	return h
}

// newUnstartedHarness returns an App whose seams are wired but whose startup
// hook has not run, so a.ctx is still nil.
func newUnstartedHarness(t *testing.T) *harness {
	t.Helper()

	h := &harness{
		coordinator: &fakeCoordinator{},
		// A value on the context makes it identifiable, so "the App passed the
		// application-lifetime context" is an equality rather than a guess.
		ctx: context.WithValue(context.Background(), runtimeContextKey{}, "wails"),
	}

	h.app = NewApp()
	h.app.useCoordinator(h.coordinator)
	h.app.emit = func(ctx context.Context, eventName string, optionalData ...interface{}) {
		h.mu.Lock()
		shouldPanic := h.emitPanic
		h.mu.Unlock()
		if shouldPanic != nil {
			panic(shouldPanic)
		}
		h.mu.Lock()
		defer h.mu.Unlock()
		h.emissions = append(h.emissions, emission{ctx: ctx, name: eventName, data: optionalData})
	}
	// Each seam records which one it is: a SelectDirectory that reached the
	// file chooser would otherwise be indistinguishable from a correct one.
	dialog := func(seam string) dialogFunc {
		return func(ctx context.Context, dialogOptions wailsruntime.OpenDialogOptions) (string, error) {
			h.mu.Lock()
			h.dialogCtx = ctx
			h.dialogTitles = append(h.dialogTitles, seam+": "+dialogOptions.Title)
			path, err := h.dialogPath, h.dialogErr
			h.mu.Unlock()
			return path, err
		}
	}
	h.app.openFile = dialog("openFile")
	h.app.openDirectory = dialog("openDirectory")
	h.app.setClipboard = func(ctx context.Context, text string) error {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.clipboardCtx = ctx
		h.clipboardText = append(h.clipboardText, text)
		return h.clipboardErr
	}

	return h
}

func (h *harness) clipboardWrites() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.clipboardText))
	copy(out, h.clipboardText)
	return out
}

type runtimeContextKey struct{}

func (h *harness) emitted() []emission {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]emission, len(h.emissions))
	copy(out, h.emissions)
	return out
}

func (h *harness) dialogs() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.dialogTitles))
	copy(out, h.dialogTitles)
	return out
}

func stagedMetadata() transfer.FileMetadata {
	return transfer.FileMetadata{
		SessionID: testSessionID,
		Name:      "quarterly report.pdf",
		Size:      4096,
		IsDir:     false,
		URL:       "http://192.168.1.50:45678/download/" + testToken,
		QR:        "iVBORw0KGgo=",
		Warnings:  []transfer.Warning{},
	}
}

// --- Composition ---------------------------------------------------------

// The three runtime seams exist for the tests below, which means the values
// production actually runs on are untested unless they are pinned here.
func TestNewAppWiresTheRealWailsRuntime(t *testing.T) {
	app := NewApp()

	for _, seam := range []struct {
		name string
		got  any
		want any
	}{
		{"emit", app.emit, wailsruntime.EventsEmit},
		{"openFile", app.openFile, wailsruntime.OpenFileDialog},
		{"openDirectory", app.openDirectory, wailsruntime.OpenDirectoryDialog},
	} {
		if seam.got == nil {
			t.Errorf("%s is nil: NewApp left a runtime seam unwired", seam.name)
			continue
		}
		if reflect.ValueOf(seam.got).Pointer() != reflect.ValueOf(seam.want).Pointer() {
			t.Errorf("%s does not point at the real Wails runtime function", seam.name)
		}
	}
}

// The matrix's Composition row. Reflection reaches unexported fields on
// purpose: nothing else can tell a coordinator wired to the real adapters from
// one wired to nil, and a build with a missing adapter compiles cleanly.
func TestComposeWiresTheSixRealAdapters(t *testing.T) {
	app := NewApp()
	coordinator := compose(app)

	if coordinator == nil {
		t.Fatal("compose returned no coordinator")
	}

	value := reflect.ValueOf(coordinator).Elem()
	for _, want := range []struct{ field, dynamic string }{
		{"source", "*source.Inspector"},
		{"network", "*network.Manager"},
		{"server", "*server.Server"},
		{"qr", "*qr.Encoder"},
		{"observer", "main.appObserver"},
	} {
		if got := dynamicTypeOf(t, value, want.field); got != want.dynamic {
			t.Errorf("coordinator field %q holds %s, want %s", want.field, got, want.dynamic)
		}
	}

	// The sixth adapter is the payload port, which the server consumes rather
	// than the coordinator, so it is one level further in.
	serverValue := value.FieldByName("server").Elem().Elem()
	if got := dynamicTypeOf(t, serverValue, "payloads"); got != "*stream.Payloads" {
		t.Errorf("server payload port holds %s, want *stream.Payloads", got)
	}
}

func TestComposeClosesTheAppCoordinatorCycle(t *testing.T) {
	app := NewApp()
	coordinator := compose(app)

	app.mu.RLock()
	installed := app.transfers
	app.mu.RUnlock()

	if installed != transferCoordinator(coordinator) {
		t.Error("compose did not hand the App the coordinator it built")
	}

	// The observer is a value type wrapping the App, so the identity check
	// reaches through it to the pointer it carries.
	observer := reflect.ValueOf(coordinator).Elem().FieldByName("observer").Elem()
	if observer.Type() != reflect.TypeOf(appObserver{}) {
		t.Fatalf("the coordinator observes a %s, want main.appObserver", observer.Type())
	}
	held := observer.FieldByName("app")
	if held.Pointer() != reflect.ValueOf(app).Pointer() {
		t.Error("the coordinator observes some other App than the one compose was given")
	}
}

// Wails binds every exported method of a bound struct, so the App's exported
// method set IS the public command surface. An exported Publish would become
// one more, letting the webview forge lifecycle events -- a transfer-complete
// for a session still running -- that the coordinator never produced and the
// reducer would believe. Every name here is a deliberate command; anything else
// appearing in this set is an accident.
func TestTheAppBindsExactlyTheContractCommands(t *testing.T) {
	want := map[string]bool{
		"StageTransfer":   true,
		"CancelTransfer":  true,
		"SelectFile":      true,
		"SelectDirectory": true,
		"CopyToClipboard": true,
	}

	appType := reflect.TypeOf(NewApp())
	got := make(map[string]bool, appType.NumMethod())
	for index := 0; index < appType.NumMethod(); index++ {
		got[appType.Method(index).Name] = true
	}

	for name := range want {
		if !got[name] {
			t.Errorf("the App no longer exports %s, so the frontend cannot call it", name)
		}
	}
	for name := range got {
		if !want[name] {
			t.Errorf("the App exports %s, which Wails will bind as a fifth command", name)
		}
	}
}

// --- CopyToClipboard -----------------------------------------------------

// The webview cannot do this itself on both platforms. WKWebView serves the
// frontend from the custom `wails://` scheme, which is not a secure context, so
// navigator.clipboard is undefined and a browser-side copy silently does
// nothing on macOS. Routing through Go is what makes one path work on both, so
// the command has to actually reach the runtime seam with the text unchanged.
func TestCopyToClipboardWritesTheTextThroughTheRuntime(t *testing.T) {
	h := newHarness(t)
	const link = "http://192.0.2.1:34123/download/fedcba9876543210fedcba9876543210"

	if err := h.app.CopyToClipboard(link); err != nil {
		t.Fatalf("CopyToClipboard returned %v, want no error", err)
	}

	if got := h.clipboardWrites(); len(got) != 1 || got[0] != link {
		t.Errorf("clipboard writes = %q, want exactly [%q]", got, link)
	}
}

// The real ClipboardSetText answers a context that never came from a running
// window by taking the process down, exactly as the dialogs do.
func TestCopyToClipboardPassesTheApplicationLifetimeContext(t *testing.T) {
	h := newHarness(t)

	if err := h.app.CopyToClipboard("link"); err != nil {
		t.Fatalf("CopyToClipboard returned %v", err)
	}

	h.mu.Lock()
	got := h.clipboardCtx
	h.mu.Unlock()
	if got != h.ctx {
		t.Errorf("clipboard context = %v, want the stored application-lifetime context %v", got, h.ctx)
	}
}

func TestCopyToClipboardRefusesBeforeStartup(t *testing.T) {
	h := newUnstartedHarness(t)

	err := h.app.CopyToClipboard("link")
	if err == nil {
		t.Fatal("CopyToClipboard succeeded before startup, so it reached the runtime with a nil context")
	}
	if got := len(h.clipboardWrites()); got != 0 {
		t.Errorf("clipboard was written %d times before startup, want 0", got)
	}
	if code := transfer.PublicErrorOf(err).Code; code != transfer.ErrTransferFailed {
		t.Errorf("code = %q, want %q", code, transfer.ErrTransferFailed)
	}
}

// A platform clipboard failure names the platform; only the code and its fixed
// copy may cross the boundary.
func TestCopyToClipboardKeepsThePlatformDiagnosticBehindTheCode(t *testing.T) {
	h := newHarness(t)
	h.clipboardErr = errors.New("NSPasteboard declineType: com.apple.pasteboard denied")

	err := h.app.CopyToClipboard("link")
	if err == nil {
		t.Fatal("CopyToClipboard returned no error for a failing clipboard")
	}
	public := transfer.PublicErrorOf(err)
	if public.Code != transfer.ErrTransferFailed {
		t.Errorf("code = %q, want %q", public.Code, transfer.ErrTransferFailed)
	}
	if strings.Contains(public.Message, "NSPasteboard") {
		t.Errorf("public message leaked the platform diagnostic: %q", public.Message)
	}
}

// The only legitimate payload is one capability URL. An unbounded string from
// a defective view should not reach the platform at all.
func TestCopyToClipboardRefusesTextBeyondTheBound(t *testing.T) {
	h := newHarness(t)

	if err := h.app.CopyToClipboard(strings.Repeat("a", maxClipboardText)); err != nil {
		t.Fatalf("CopyToClipboard refused text at the bound: %v", err)
	}
	if err := h.app.CopyToClipboard(strings.Repeat("a", maxClipboardText+1)); err == nil {
		t.Error("CopyToClipboard accepted text past the bound")
	}

	if got := len(h.clipboardWrites()); got != 1 {
		t.Errorf("clipboard was written %d times, want 1: the oversized text still reached the platform", got)
	}
}

// dynamicTypeOf reports the concrete type held by an unexported interface
// field. Reading the value would panic on an unexported field; reading its
// type does not.
func dynamicTypeOf(t *testing.T, value reflect.Value, field string) string {
	t.Helper()

	held := value.FieldByName(field)
	if !held.IsValid() {
		t.Fatalf("field %q no longer exists: this test pins composition through it", field)
	}
	if held.Kind() != reflect.Interface {
		t.Fatalf("field %q is %s, not an interface", field, held.Kind())
	}
	if held.IsNil() {
		t.Fatalf("field %q is nil: composition left an adapter out", field)
	}
	// A typed nil satisfies the interface and reports the right type, so the
	// type name alone would accept an adapter that panics on first use.
	if held.Elem().Kind() == reflect.Pointer && held.Elem().IsNil() {
		t.Fatalf("field %q holds a nil %s: composition wired a typed nil adapter", field, held.Elem().Type())
	}
	return held.Elem().Type().String()
}

// --- StageTransfer -------------------------------------------------------

func TestStageTransferReturnsTheCoordinatorMetadataUnchanged(t *testing.T) {
	h := newHarness(t)
	want := stagedMetadata()
	h.coordinator.stageMetadata = want

	got, err := h.app.StageTransfer(testPath)

	if err != nil {
		t.Fatalf("StageTransfer returned %v, want no error", err)
	}
	if got == nil {
		t.Fatal("StageTransfer returned nil metadata for a successful stage")
	}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("StageTransfer remapped the metadata:\n got %+v\nwant %+v", *got, want)
	}
	if h.coordinator.stagePath != testPath {
		t.Errorf("the coordinator was asked to stage %q, want %q", h.coordinator.stagePath, testPath)
	}
}

// The coordinator owns the non-null guarantee (it allocates the slice at
// stage time); this asserts the boundary does not undo it. Driving the fixture
// from nil is what makes that a claim about app.go rather than about the
// fixture: a boundary that re-marshalled the value would turn nil into null
// here, and the frontend would get null where it expects [].
func TestStageTransferDoesNotUndoTheWarningsGuarantee(t *testing.T) {
	h := newHarness(t)
	metadata := stagedMetadata()
	metadata.Warnings = []transfer.Warning{}
	h.coordinator.stageMetadata = metadata

	staged, err := h.app.StageTransfer(testPath)
	if err != nil {
		t.Fatalf("StageTransfer returned %v", err)
	}
	encoded, err := json.Marshal(staged)
	if err != nil {
		t.Fatalf("staged metadata does not marshal: %v", err)
	}
	if !strings.Contains(string(encoded), `"warnings":[]`) {
		t.Errorf("warnings serialized as %s, want an empty array", encoded)
	}

	// And the passthrough itself: whatever the coordinator returned is what
	// crosses, unremapped.
	if staged.URL != metadata.URL || staged.QR != metadata.QR || staged.SessionID != metadata.SessionID {
		t.Errorf("the boundary remapped the metadata: %+v", staged)
	}
}

func TestStageTransferPassesTheApplicationLifetimeContext(t *testing.T) {
	h := newHarness(t)

	if _, err := h.app.StageTransfer(testPath); err != nil {
		t.Fatalf("StageTransfer returned %v, want no error", err)
	}

	if h.coordinator.stageCtx == nil {
		t.Fatal("the coordinator was handed a nil context")
	}
	if got := h.coordinator.stageCtx.Value(runtimeContextKey{}); got != "wails" {
		t.Errorf("the coordinator was handed a context carrying %v, want the stored startup context", got)
	}
}

// Every refusal the coordinator can answer Stage with, spelled out as literals
// so the codes cannot move together with the constants under test.
func TestStageTransferPreservesEveryRefusalCode(t *testing.T) {
	for _, refusal := range []struct {
		name string
		code transfer.ErrorCode
		want string
	}{
		{"busy", transfer.ErrBusy, "busy"},
		{"invalid selection", transfer.ErrInvalidSelection, "invalid_selection"},
		{"path not found", transfer.ErrPathNotFound, "path_not_found"},
		{"path unsupported", transfer.ErrPathUnsupported, "path_unsupported"},
		{"network unavailable", transfer.ErrNetworkUnavailable, "network_unavailable"},
		{"server start failed", transfer.ErrServerStartFailed, "server_start_failed"},
		{"qr failed", transfer.ErrQRFailed, "qr_failed"},
		{"cancelled", transfer.ErrCancelled, "cancelled"},
		{"shutting down", transfer.ErrShuttingDown, "shutting_down"},
	} {
		t.Run(refusal.name, func(t *testing.T) {
			h := newHarness(t)
			// Non-empty metadata beside the error: a command that returned
			// both would be handing the UI a session that does not exist.
			h.coordinator.stageMetadata = stagedMetadata()
			h.coordinator.stageErr = transfer.NewError(refusal.code, "diagnostic naming "+testPath)

			metadata, err := h.app.StageTransfer(testPath)

			if metadata != nil {
				t.Errorf("StageTransfer returned metadata %+v with a refusal", *metadata)
			}
			if err == nil {
				t.Fatal("StageTransfer returned no error for a refused stage")
			}
			if got := string(transfer.ErrorCodeOf(err)); got != refusal.want {
				t.Errorf("the refusal crossed as %q, want %q", got, refusal.want)
			}
		})
	}
}

func TestStageTransferMapsAnUnrecognizedFailureToTransferFailed(t *testing.T) {
	h := newHarness(t)
	h.coordinator.stageErr = errors.New("dial tcp 192.168.1.50:45678: refused")

	metadata, err := h.app.StageTransfer(testPath)

	if metadata != nil {
		t.Error("StageTransfer returned metadata for an unrecognized failure")
	}
	if got := string(transfer.ErrorCodeOf(err)); got != "transfer_failed" {
		t.Errorf("an unrecognized failure crossed as %q, want %q", got, "transfer_failed")
	}
}

func TestStageTransferEmitsNoLifecycleEvent(t *testing.T) {
	h := newHarness(t)
	h.coordinator.stageMetadata = stagedMetadata()

	if _, err := h.app.StageTransfer(testPath); err != nil {
		t.Fatalf("StageTransfer returned %v, want no error", err)
	}

	if got := h.emitted(); len(got) != 0 {
		t.Errorf("StageTransfer emitted %+v: the coordinator owns every lifecycle event", got)
	}
}

// --- CancelTransfer ------------------------------------------------------

func TestCancelTransferDelegatesAndReturnsQuietly(t *testing.T) {
	h := newHarness(t)

	if err := h.app.CancelTransfer(); err != nil {
		t.Errorf("CancelTransfer returned %v, want nil", err)
	}
	if got := h.coordinator.log(); !reflect.DeepEqual(got, []string{"Cancel"}) {
		t.Errorf("CancelTransfer produced the call log %v, want exactly [Cancel]", got)
	}
	if got := h.emitted(); len(got) != 0 {
		t.Errorf("CancelTransfer emitted %+v itself", got)
	}
}

func TestCancelTransferAfterShutdownIsRefusedWithShuttingDown(t *testing.T) {
	h := newHarness(t)
	h.coordinator.cancelErr = transfer.NewError(transfer.ErrShuttingDown, "FairDrop is closing")

	err := h.app.CancelTransfer()

	if err == nil {
		t.Fatal("CancelTransfer returned nil while the application was closing")
	}
	if got := string(transfer.ErrorCodeOf(err)); got != "shutting_down" {
		t.Errorf("the refusal crossed as %q, want %q", got, "shutting_down")
	}
}

// --- Dialogs -------------------------------------------------------------

func TestSelectDialogsReturnTheChosenPathAndStageNothing(t *testing.T) {
	for _, dialog := range []struct {
		name   string
		call   func(*App) (string, error)
		title  string
		chosen string
	}{
		{"SelectFile", (*App).SelectFile, "openFile: Choose a file to send", testPath},
		{"SelectDirectory", (*App).SelectDirectory, "openDirectory: Choose a folder to send", `C:\Users\sender\Documents\quarter`},
	} {
		t.Run(dialog.name, func(t *testing.T) {
			h := newHarness(t)
			h.dialogPath = dialog.chosen

			got, err := dialog.call(h.app)

			if err != nil {
				t.Fatalf("%s returned %v, want no error", dialog.name, err)
			}
			if got != dialog.chosen {
				t.Errorf("%s returned %q, want %q", dialog.name, got, dialog.chosen)
			}
			if titles := h.dialogs(); !reflect.DeepEqual(titles, []string{dialog.title}) {
				t.Errorf("%s opened %v, want exactly one %q dialog", dialog.name, titles, dialog.title)
			}
			if calls := h.coordinator.log(); len(calls) != 0 {
				t.Errorf("%s reached the coordinator (%v): a dialog stages nothing", dialog.name, calls)
			}
			if h.dialogCtx == nil || h.dialogCtx.Value(runtimeContextKey{}) != "wails" {
				t.Error("the dialog was not handed the application-lifetime context")
			}
		})
	}
}

func TestDismissedDialogIsAnEmptySelectionNotAnError(t *testing.T) {
	h := newHarness(t)
	// What Wails answers a dismissed chooser with.
	h.dialogPath = ""
	h.dialogErr = nil

	got, err := h.app.SelectFile()

	if err != nil {
		t.Errorf("a dismissed dialog returned %v, want no error", err)
	}
	if got != "" {
		t.Errorf("a dismissed dialog returned %q, want an empty selection", got)
	}
	if events := h.emitted(); len(events) != 0 {
		t.Errorf("a dismissed dialog emitted %+v", events)
	}
	if calls := h.coordinator.log(); len(calls) != 0 {
		t.Errorf("a dismissed dialog reached the coordinator: %v", calls)
	}
}

func TestDialogFailureIsCodedAndDisclosesNoDialogText(t *testing.T) {
	h := newHarness(t)
	h.dialogErr = errors.New("default directory '" + testPath + "' does not exist")

	got, err := h.app.SelectFile()

	if got != "" {
		t.Errorf("a failed dialog returned the selection %q", got)
	}
	if err == nil {
		t.Fatal("a failed dialog returned no error")
	}
	if code := string(transfer.ErrorCodeOf(err)); code != "transfer_failed" {
		t.Errorf("a failed dialog crossed as %q, want %q", code, "transfer_failed")
	}
	if strings.Contains(err.Error(), testPath) {
		t.Errorf("the dialog's own text reached the command error: %q", err.Error())
	}
}

func TestDialogBeforeStartupIsRefusedRatherThanFatal(t *testing.T) {
	h := newUnstartedHarness(t)
	h.dialogPath = testPath

	got, err := h.app.SelectFile()

	if got != "" || err == nil {
		t.Fatalf("SelectFile before startup returned (%q, %v), want a coded refusal", got, err)
	}
	if code := string(transfer.ErrorCodeOf(err)); code != "transfer_failed" {
		t.Errorf("the refusal crossed as %q, want %q", code, "transfer_failed")
	}
	if titles := h.dialogs(); len(titles) != 0 {
		t.Errorf("a dialog was opened without a window context: %v", titles)
	}
}

// --- Publish -------------------------------------------------------------

// The name carries the kind, which is why Event.Kind is json:"-". Spelled out
// as literals: deriving them from the constants under test would let the wire
// names change with the whole suite green while the frontend listened for the
// old ones.
func TestPublishEmitsOneRuntimeEventPerKind(t *testing.T) {
	for _, published := range []struct {
		kind transfer.EventKind
		want string
	}{
		{transfer.TransferStarted, "transfer-started"},
		{transfer.TransferProgress, "transfer-progress"},
		{transfer.TransferComplete, "transfer-complete"},
		{transfer.TransferError, "transfer-error"},
		{transfer.TransferReset, "transfer-reset"},
	} {
		t.Run(published.want, func(t *testing.T) {
			h := newHarness(t)

			h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 7, Kind: published.kind})

			events := h.emitted()
			if len(events) != 1 {
				t.Fatalf("Publish produced %d emissions, want exactly 1", len(events))
			}
			if events[0].name != published.want {
				t.Errorf("the emission was named %q, want %q", events[0].name, published.want)
			}
			if len(events[0].data) != 1 {
				t.Fatalf("the emission carried %d payloads, want exactly 1", len(events[0].data))
			}
		})
	}
}

func TestPublishedPayloadCarriesSessionAndSeqAndNoKind(t *testing.T) {
	h := newHarness(t)

	h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 3, Kind: transfer.TransferStarted})

	encoded := marshalOnlyEmission(t, h)
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("the emitted payload is not a JSON object: %v", err)
	}

	if decoded["sessionId"] != string(testSessionID) {
		t.Errorf("the payload carried sessionId %v, want %q", decoded["sessionId"], testSessionID)
	}
	if decoded["seq"] != float64(3) {
		t.Errorf("the payload carried seq %v, want 3", decoded["seq"])
	}
	for _, forbidden := range []string{"Kind", "kind"} {
		if _, present := decoded[forbidden]; present {
			t.Errorf("the payload carried the internal %q field: %s", forbidden, encoded)
		}
	}
}

// The contract's per-event payload table, checked on the wire rather than in
// the struct: an adapter that emitted a remapped shape would still satisfy the
// struct.
func TestPublishedPayloadMatchesTheContractPayloadTable(t *testing.T) {
	progress := &transfer.ProgressSnapshot{
		BytesSent: 2048, TotalBytes: 4096, TotalKnown: true, Percent: 50, SpeedBytesPerSec: 1024,
	}
	failure := &transfer.PublicError{Code: transfer.ErrTransferFailed, Message: "stopped"}

	for _, published := range []struct {
		name         string
		event        transfer.Event
		wantProgress bool
		wantError    bool
	}{
		{"started", transfer.Event{Kind: transfer.TransferStarted}, false, false},
		{"progress", transfer.Event{Kind: transfer.TransferProgress, Progress: progress}, true, false},
		{"complete", transfer.Event{Kind: transfer.TransferComplete, Progress: progress}, true, false},
		{"error", transfer.Event{Kind: transfer.TransferError, Error: failure}, false, true},
		{"error with a final snapshot", transfer.Event{Kind: transfer.TransferError, Progress: progress, Error: failure}, true, true},
		{"reset", transfer.Event{Kind: transfer.TransferReset}, false, false},
	} {
		t.Run(published.name, func(t *testing.T) {
			h := newHarness(t)
			event := published.event
			event.SessionID = testSessionID
			event.Seq = 1

			h.app.publish(event)

			var decoded map[string]any
			if err := json.Unmarshal(marshalOnlyEmission(t, h), &decoded); err != nil {
				t.Fatalf("the emitted payload is not a JSON object: %v", err)
			}
			if _, present := decoded["progress"]; present != published.wantProgress {
				t.Errorf("progress present = %v, want %v", present, published.wantProgress)
			}
			if _, present := decoded["error"]; present != published.wantError {
				t.Errorf("error present = %v, want %v", present, published.wantError)
			}
		})
	}
}

func TestPublishBeforeStartupDropsTheEventWithoutEmitting(t *testing.T) {
	h := newUnstartedHarness(t)

	h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 1, Kind: transfer.TransferStarted})

	if events := h.emitted(); len(events) != 0 {
		t.Errorf("Publish emitted %+v with no window context", events)
	}
	if got := h.app.undelivered.Load(); got != 1 {
		t.Errorf("undelivered = %d, want 1: the drop must be recorded", got)
	}
	// The coordinator holds its operation lease across this call, so the only
	// thing that matters is that it got control back -- which reaching this
	// line proves.
	if err := h.app.CancelTransfer(); err != nil {
		t.Errorf("the next command returned %v after a dropped event", err)
	}
}

func TestPublishRecoversAnEmitPanicSoTheLeaseIsNotStranded(t *testing.T) {
	h := newHarness(t)
	h.emitPanic = errors.New("the webview is gone")

	// A panic escaping here would strand the coordinator's operation lease and
	// wedge every later Cancel and Shutdown, so returning at all is the whole
	// guarantee this asserts.
	h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 1, Kind: transfer.TransferProgress})

	if got := h.app.undelivered.Load(); got != 1 {
		t.Errorf("undelivered = %d, want 1: the failed emission must be recorded", got)
	}
	if calls := h.coordinator.log(); len(calls) != 0 {
		t.Errorf("Publish called back into the coordinator: %v", calls)
	}

	h.emitPanic = nil
	if err := h.app.CancelTransfer(); err != nil {
		t.Errorf("the next command returned %v after a panicking emission", err)
	}
	h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 2, Kind: transfer.TransferReset})
	if events := h.emitted(); len(events) != 1 || events[0].name != "transfer-reset" {
		t.Errorf("the emitter did not recover for the next event: %+v", events)
	}
}

func TestPublishNeverCallsBackIntoTheCoordinator(t *testing.T) {
	h := newHarness(t)

	for seq, kind := range []transfer.EventKind{
		transfer.TransferStarted,
		transfer.TransferProgress,
		transfer.TransferComplete,
		transfer.TransferError,
		transfer.TransferReset,
	} {
		h.app.publish(transfer.Event{SessionID: testSessionID, Seq: uint64(seq + 1), Kind: kind})
	}

	if calls := h.coordinator.log(); len(calls) != 0 {
		t.Errorf("Publish reached the coordinator: %v", calls)
	}
}

func marshalOnlyEmission(t *testing.T, h *harness) []byte {
	t.Helper()

	events := h.emitted()
	if len(events) != 1 {
		t.Fatalf("expected exactly one emission, got %d", len(events))
	}
	if len(events[0].data) != 1 {
		t.Fatalf("expected exactly one payload, got %d", len(events[0].data))
	}
	encoded, err := json.Marshal(events[0].data[0])
	if err != nil {
		t.Fatalf("the emitted payload does not serialize: %v", err)
	}
	return encoded
}

// --- Lifecycle hooks -----------------------------------------------------

func TestStartupStoresTheContextAndNothingElse(t *testing.T) {
	h := newUnstartedHarness(t)

	h.app.startup(h.ctx)

	if h.app.runtimeContext() != h.ctx {
		t.Error("startup did not store the context it was given")
	}
	if calls := h.coordinator.log(); len(calls) != 0 {
		t.Errorf("startup reached the coordinator: %v", calls)
	}
	if events := h.emitted(); len(events) != 0 {
		t.Errorf("startup emitted %+v", events)
	}
}

func TestShutdownDelegatesAndBlocksUntilQuiescent(t *testing.T) {
	h := newHarness(t)
	release := make(chan struct{})
	h.coordinator.shutdownGate = release

	returned := make(chan struct{})
	go func() {
		h.app.shutdown(context.Background())
		close(returned)
	}()

	select {
	case <-returned:
		t.Fatal("the shutdown hook returned while resources were still live")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("the shutdown hook never returned after Shutdown completed")
	}

	if calls := h.coordinator.log(); !reflect.DeepEqual(calls, []string{"Shutdown"}) {
		t.Errorf("the shutdown hook produced the call log %v, want exactly [Shutdown]", calls)
	}
}

func TestShutdownDoesNotReplaceTheRuntimeContext(t *testing.T) {
	h := newHarness(t)
	closing, cancel := context.WithCancel(context.Background())
	cancel()

	h.app.shutdown(closing)

	if h.app.runtimeContext() != h.ctx {
		t.Error("the shutdown hook replaced the application-lifetime context with its own")
	}
}

func TestShutdownIsRepeatableAndLeavesIdempotenceToTheCoordinator(t *testing.T) {
	h := newHarness(t)

	h.app.shutdown(context.Background())
	h.app.shutdown(context.Background())

	if calls := h.coordinator.log(); !reflect.DeepEqual(calls, []string{"Shutdown", "Shutdown"}) {
		t.Errorf("the shutdown hook produced %v, want both calls delegated", calls)
	}
}

// --- Composition failure and disclosure ----------------------------------

func TestCommandsRefuseBeforeCompositionRatherThanPanicking(t *testing.T) {
	app := NewApp()
	app.startup(context.Background())

	metadata, err := app.StageTransfer(testPath)
	if metadata != nil || err == nil {
		t.Fatalf("StageTransfer returned (%v, %v) with no coordinator", metadata, err)
	}
	if code := string(transfer.ErrorCodeOf(err)); code != "transfer_failed" {
		t.Errorf("the refusal crossed as %q, want %q", code, "transfer_failed")
	}

	if err := app.CancelTransfer(); err == nil {
		t.Error("CancelTransfer returned nil with no coordinator")
	}

	// The shutdown hook has nothing to do and must not dereference nil.
	app.shutdown(context.Background())
}

// The Disclosure row: nothing that crosses the boundary may name the source
// path or the capability token.
func TestNoCommandErrorOrEmittedEventDisclosesPathOrToken(t *testing.T) {
	h := newHarness(t)
	h.coordinator.stageMetadata = stagedMetadata()

	// Every command failure the boundary can produce, formatted the way the
	// frontend will actually see it.
	for _, failure := range []error{
		transfer.WrapError(transfer.ErrPathNotFound, "inspecting "+testPath, errors.New("open "+testPath+": not found")),
		errors.New("listen tcp: token " + testToken + " for " + testPath),
	} {
		formatted, ok := formatCommandError(failure).(string)
		if !ok {
			t.Fatalf("formatCommandError returned %T, want a string", formatCommandError(failure))
		}
		for _, secret := range []string{testPath, testToken, `C:\`} {
			if strings.Contains(formatted, secret) {
				t.Errorf("a command error disclosed %q: %s", secret, formatted)
			}
		}
	}

	// Every event kind, serialized the way Wails will serialize it.
	for seq, kind := range []transfer.EventKind{
		transfer.TransferStarted,
		transfer.TransferProgress,
		transfer.TransferComplete,
		transfer.TransferError,
		transfer.TransferReset,
	} {
		h.app.publish(transfer.Event{
			SessionID: testSessionID,
			Seq:       uint64(seq + 1),
			Kind:      kind,
			Progress:  &transfer.ProgressSnapshot{BytesSent: 1, TotalBytes: 2, TotalKnown: true, Percent: 50},
			Error:     &transfer.PublicError{Code: transfer.ErrTransferFailed, Message: "stopped"},
		})
	}
	for _, event := range h.emitted() {
		encoded, err := json.Marshal(event.data[0])
		if err != nil {
			t.Fatalf("an emitted payload does not serialize: %v", err)
		}
		for _, secret := range []string{testPath, testToken, `C:\`} {
			if strings.Contains(string(encoded), secret) {
				t.Errorf("event %q disclosed %q: %s", event.name, secret, encoded)
			}
		}
	}
}

// --- The adapter that actually satisfies the port ------------------------

// Every other publish test drives App.publish directly, which leaves the one
// type the coordinator really calls untested: emptying appObserver.Publish
// left the whole suite green while no lifecycle event reached the window at
// all. This is the only test that goes through the port.
func TestTheObserverPortReachesTheRuntimeEmitter(t *testing.T) {
	h := newHarness(t)

	var observer transfer.Observer = appObserver{app: h.app}
	observer.Publish(transfer.Event{
		SessionID: testSessionID,
		Seq:       4,
		Kind:      transfer.TransferComplete,
		Progress:  &transfer.ProgressSnapshot{BytesSent: 4096, TotalBytes: 4096, TotalKnown: true, Percent: 100},
	})

	emitted := h.emitted()
	if len(emitted) != 1 {
		t.Fatalf("the observer port produced %d emissions, want exactly one", len(emitted))
	}
	if emitted[0].name != "transfer-complete" {
		t.Errorf("emitted %q, want %q", emitted[0].name, "transfer-complete")
	}
	if emitted[0].ctx != h.ctx {
		t.Error("the emission did not carry the application-lifetime context")
	}
}

// The nil guard exists because a misordered composition would otherwise panic
// on the coordinator's goroutine, where the operation lease is held.
func TestTheObserverPortWithoutAnAppIsInert(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("an unwired observer panicked: %v", recovered)
		}
	}()
	appObserver{}.Publish(transfer.Event{SessionID: testSessionID, Seq: 1, Kind: transfer.TransferStarted})
}

// Every emission must carry the stored application-lifetime context. Nothing
// else asserted it, and the real EventsEmit answers a foreign context with
// log.Fatalf -- the process, not the call, is what fails.
func TestEveryEmissionCarriesTheApplicationLifetimeContext(t *testing.T) {
	h := newHarness(t)

	for _, kind := range []transfer.EventKind{
		transfer.TransferStarted,
		transfer.TransferProgress,
		transfer.TransferComplete,
		transfer.TransferError,
		transfer.TransferReset,
	} {
		h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 1, Kind: kind})
	}

	emitted := h.emitted()
	if len(emitted) != 5 {
		t.Fatalf("emitted %d events, want 5", len(emitted))
	}
	for index, got := range emitted {
		if got.ctx != h.ctx {
			t.Errorf("emission %d (%s) carried a different context", index, got.name)
		}
	}
}

// A kind this build cannot name has no event name to travel under, so it is
// refused and counted rather than emitted into the void.
func TestAnUnknownEventKindIsCountedRatherThanEmitted(t *testing.T) {
	h := newHarness(t)

	for _, kind := range []transfer.EventKind{"", "transfer-teleported"} {
		h.app.publish(transfer.Event{SessionID: testSessionID, Seq: 1, Kind: kind})
	}

	if emitted := h.emitted(); len(emitted) != 0 {
		t.Errorf("emitted %+v, want nothing for a kind this build does not know", emitted)
	}
	if got := h.app.undelivered.Load(); got != 2 {
		t.Errorf("undelivered = %d, want 2", got)
	}
}

// The App is driven from two goroutines in production: startup runs on the
// main one while publish runs on whichever goroutine owns the coordinator's
// operation lease. That is what a.mu is for, and nothing exercised it.
func TestStartupAndPublishAreSafeConcurrently(t *testing.T) {
	h := newUnstartedHarness(t)

	var wait sync.WaitGroup
	wait.Add(2)
	go func() {
		defer wait.Done()
		h.app.startup(h.ctx)
	}()
	go func() {
		defer wait.Done()
		for seq := 1; seq <= 50; seq++ {
			h.app.publish(transfer.Event{
				SessionID: testSessionID,
				Seq:       uint64(seq),
				Kind:      transfer.TransferProgress,
				Progress:  &transfer.ProgressSnapshot{BytesSent: int64(seq)},
			})
		}
	}()
	wait.Wait()

	// Every event either emitted or was counted; none may be lost silently.
	if got := uint64(len(h.emitted())) + h.app.undelivered.Load(); got != 50 {
		t.Errorf("%d of 50 events were accounted for", got)
	}
}

// The disclosure row of the matrix, driven through the one command that
// legitimately carries the capability token. The previous version built its
// own payloads out of two integers and a fixed string, so nothing it searched
// could ever have contained the path.
func TestStagedMetadataCarriesTheTokenButNeverThePath(t *testing.T) {
	h := newHarness(t)
	h.coordinator.stageMetadata = stagedMetadata()

	metadata, err := h.app.StageTransfer(testPath)
	if err != nil {
		t.Fatalf("StageTransfer returned %v", err)
	}

	encoded, err := json.Marshal(metadata)
	if err != nil {
		t.Fatalf("staged metadata does not marshal: %v", err)
	}
	serialized := string(encoded)

	// The token is allowed here and nowhere else: it is what the URL and the
	// QR are for. Asserting its presence is what stops this test passing
	// because the payload happened to be empty.
	if !strings.Contains(serialized, testToken) {
		t.Errorf("staged metadata carries no capability token: %s", serialized)
	}
	if strings.Contains(serialized, testPath) {
		t.Errorf("staged metadata leaked the source path: %s", serialized)
	}
	if strings.Contains(serialized, `\\Users\\sender`) {
		t.Errorf("staged metadata leaked part of the source path: %s", serialized)
	}
}
