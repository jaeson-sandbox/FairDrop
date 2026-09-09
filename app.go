package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"fairdrop/internal/transfer"

	"github.com/wailsapp/wails/v2/pkg/options"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// transferCoordinator is the App's whole view of the transfer implementation.
//
// It is an interface for one reason: app.go is a translation layer, and a
// translation layer that names the concrete coordinator could not be driven
// through its own edge cases -- a real coordinator refusing a command needs a
// listener, a beacon and a selected file to refuse it with. main.go supplies
// the real one and is the only file that names it.
type transferCoordinator interface {
	Stage(ctx context.Context, absolutePath string) (transfer.FileMetadata, error)
	Cancel() error
	Shutdown() error
}

// emittableKinds is the closed set of lifecycle events this adapter knows how
// to route. It is spelled out rather than derived so that a kind added to the
// contract has to be added here too, deliberately, instead of silently
// emitting under a name the frontend never subscribes to.
var emittableKinds = map[transfer.EventKind]bool{
	transfer.TransferStarted:  true,
	transfer.TransferProgress: true,
	transfer.TransferComplete: true,
	transfer.TransferError:    true,
	transfer.TransferReset:    true,
}

// dialogFunc is the shape both native open dialogs share.
type dialogFunc func(ctx context.Context, dialogOptions wailsruntime.OpenDialogOptions) (string, error)

// emitFunc is the shape of the Wails runtime event emission.
type emitFunc func(ctx context.Context, eventName string, optionalData ...interface{})

// clipboardFunc is the shape of the Wails runtime clipboard write.
type clipboardFunc func(ctx context.Context, text string) error

// windowActionFunc is the shape both window-restoration seams share:
// WindowUnminimise and WindowShow in the real Wails runtime each take only
// the application-lifetime context.
type windowActionFunc func(ctx context.Context)

// App is FairDrop's Wails boundary: it translates the five bound commands into
// coordinator calls, translates coordinator lifecycle events into runtime
// emissions, and owns nothing else. Every decision about what a transfer is,
// what state it is in, and what may happen next belongs to the coordinator.
type App struct {
	// mu guards the two fields written after construction: ctx, which startup
	// installs on the main goroutine, and transfers, which main.go installs
	// once. Publish reads ctx from whichever goroutine owns the coordinator's
	// operation lease, so the read is genuinely concurrent with the write.
	mu        sync.RWMutex
	ctx       context.Context
	transfers transferCoordinator

	// undelivered counts lifecycle events the window could not be told about.
	// The recovered panic value is deliberately dropped rather than logged: a
	// value that escapes an adapter is adapter text, and adapter text is
	// exactly where an absolute path or a capability token would be.
	//
	// Nothing in production reads this counter yet -- it is assertable in
	// tests and otherwise inert. It is kept because a drop is exactly the
	// event nobody else can observe, and Story 1.10's recovery contract is
	// where a degraded-state surface would consume it.
	undelivered atomic.Uint64

	// The Wails runtime seams, set once at construction and never written
	// again. They exist because the real ones cannot run under `go test`:
	// EventsEmit, both dialogs, the clipboard write, and both window-restoration
	// calls answer a context that did not come from a running window with
	// log.Fatalf -- not panic -- which would take the test binary down with it.
	emit          emitFunc
	openFile      dialogFunc
	openDirectory dialogFunc
	setClipboard  clipboardFunc
	unminimise    windowActionFunc
	show          windowActionFunc

	// logf is the diagnostic seam. A transfer otherwise leaves no record at
	// all -- FairDrop persists nothing, by contract -- so this is the one place
	// a live failure can be explained afterwards: one stderr line per
	// lifecycle event, visible when the app is launched from a shell. It
	// carries the kind, sequence, session id, byte count and error code, and
	// never the capability token, the selected name, or a path, which AD-9
	// keeps out of diagnostics. Progress is not logged; at four a second it
	// would bury the lines that matter, and the terminal events carry the
	// final count.
	logf func(format string, args ...any)

	// homeDir is where a chooser opens. It is a seam for the same reason the
	// dialogs are: the test binary must be able to answer it without depending
	// on the machine running the tests.
	homeDir func() (string, error)
}

// appObserver adapts the App to transfer.Observer.
//
// It exists so that Publish is NOT a method on App. Wails binds every exported
// method of a bound struct, so an exported Publish would be generated as a
// fifth callable command. The contract fixes the public API at four commands,
// and this is what keeps the generated bindings equal to it.
//
// What this does NOT do is make forged lifecycle events impossible. Wails'
// frontend EventsEmit notifies same-webview listeners itself before it
// forwards anything to Go, so any script in the window can already deliver a
// transfer-complete to every EventsOn subscriber without this process being
// involved. That is inherent to the event system, and the defence against it
// is the frontend rule the contract already fixes: initialize
// (sessionId, lastSeq) only from a successful Stage result and ignore events
// carrying another session or a seq at or below the last one. Story 1.8 owns
// that reducer. This type narrows the command surface; it is not a trust
// boundary.
type appObserver struct{ app *App }

var _ transfer.Observer = appObserver{}

func (o appObserver) Publish(event transfer.Event) {
	if o.app == nil {
		return
	}
	o.app.publish(event)
}

// NewApp creates a new App wired to the real Wails runtime.
func NewApp() *App {
	return &App{
		emit:          wailsruntime.EventsEmit,
		openFile:      wailsruntime.OpenFileDialog,
		openDirectory: wailsruntime.OpenDirectoryDialog,
		setClipboard:  wailsruntime.ClipboardSetText,
		unminimise:    wailsruntime.WindowUnminimise,
		show:          wailsruntime.WindowShow,
		homeDir:       os.UserHomeDir,
		logf:          log.Printf,
	}
}

// useCoordinator hands the App the coordinator its commands delegate to.
//
// The App is the coordinator's Observer and the coordinator is the App's
// delegate, so one of the two has to be built first. main.go builds the App,
// builds the coordinator with the App as its observer, and closes the cycle
// here. That single setter is why the cycle needs no channel, no registry and
// no lazily initialized global.
func (a *App) useCoordinator(coordinator transferCoordinator) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.transfers = coordinator
}

// StageTransfer stages the one selected absolute path and returns the session
// acknowledgement the UI needs.
//
// The metadata crosses unchanged: remapping it here would be a second
// definition of the same shape, and the two would drift.
func (a *App) StageTransfer(absolutePath string) (*transfer.FileMetadata, error) {
	ctx, coordinator := a.delegate()
	if coordinator == nil {
		return nil, errNotComposed()
	}

	metadata, err := coordinator.Stage(ctx, absolutePath)
	if err != nil {
		// No metadata on failure, and the coordinator's code is preserved for
		// the error formatter to serialize.
		return nil, err
	}
	return &metadata, nil
}

// CancelTransfer abandons whatever transfer is in flight.
//
// Cancel takes no context on purpose: it returns only once the listener,
// beacon, drainer and session context are quiescent, so a caller-supplied
// deadline would be a parameter it had to ignore. A cleanup diagnostic never
// becomes a command failure -- the coordinator already decided that -- so
// there is nothing to translate on the success path.
func (a *App) CancelTransfer() error {
	_, coordinator := a.delegate()
	if coordinator == nil {
		return errNotComposed()
	}
	return coordinator.Cancel()
}

// SelectFile opens the native file chooser and returns the chosen path.
//
// It stages nothing. The frontend decides what to do with the selection, which
// is what keeps the keyboard path and the drop path on the same single
// StageTransfer entry point. A dismissed dialog is an empty selection, not an
// error, and produces no lifecycle event.
func (a *App) SelectFile() (string, error) {
	return a.chooseWith(a.openFile, "Choose a file to send")
}

// SelectDirectory opens the native folder chooser and returns the chosen path.
// Like SelectFile it stages nothing.
func (a *App) SelectDirectory() (string, error) {
	return a.chooseWith(a.openDirectory, "Choose a folder to send")
}

// maxClipboardText bounds what the window may put on the user's clipboard.
// The only legitimate payload is one capability URL, which is well under a
// hundred characters; the bound keeps a defect in the view from handing the
// platform an unbounded string.
const maxClipboardText = 2048

// CopyToClipboard writes text to the system clipboard through the Wails
// runtime rather than through the webview.
//
// The webview cannot do this itself on every platform: WKWebView serves the
// frontend from the custom `wails://` scheme, which is not a secure context, so
// `navigator.clipboard` is undefined there and a browser-side copy silently
// does nothing on macOS. WebView2 serves `http://wails.localhost`, which is
// trustworthy, so the browser API works on Windows -- the asymmetry is exactly
// why this goes through Go, where one path serves both.
func (a *App) CopyToClipboard(text string) error {
	if len(text) > maxClipboardText {
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"the text to copy is larger than the clipboard command accepts",
		)
	}

	ctx := a.runtimeContext()
	if ctx == nil {
		// Same reasoning as the dialogs: the real runtime answers a context
		// that never came from a window by taking the process down.
		return transfer.NewError(
			transfer.ErrTransferFailed,
			"FairDrop is not ready to use the clipboard",
		)
	}

	if err := a.setClipboard(ctx, text); err != nil {
		// The platform's own clipboard diagnostic is adapter text; only the
		// code and its fixed copy cross the boundary.
		return transfer.WrapError(
			transfer.ErrTransferFailed,
			"the clipboard could not be written",
			err,
		)
	}
	return nil
}

// chooseWith runs one native open dialog.
func (a *App) chooseWith(open dialogFunc, title string) (string, error) {
	ctx := a.runtimeContext()
	if ctx == nil {
		// The real dialog answers a context that did not come from a running
		// window with log.Fatalf, so this is the difference between a coded
		// refusal and a process that vanishes.
		return "", transfer.NewError(
			transfer.ErrTransferFailed,
			"FairDrop is not ready to open a chooser",
		)
	}

	selection, err := open(ctx, wailsruntime.OpenDialogOptions{
		Title:            title,
		DefaultDirectory: a.startingDirectory(),
	})
	if err != nil {
		// A dialog's own diagnostic text names directories, so it stays behind
		// Unwrap: what crosses the boundary is the code and the fixed copy.
		return "", transfer.WrapError(
			transfer.ErrTransferFailed,
			"the chooser could not be opened",
			err,
		)
	}
	return selection, nil
}

/*
startingDirectory is where a chooser opens, and "" means "wherever Windows
left it last".

The empty default is what made the folder chooser tedious: it reopens on the
last location the OS remembers, which is often nowhere near the folder being
sent, and the platform picker selects the directory you have navigated *into*
rather than one you click. Opening at home shortens that walk.

It is deliberately not the last folder chosen, which would be the nicer
behaviour. FairDrop persists nothing between runs, and a remembered path is
persistence -- of exactly the kind the product promises not to keep, since it
records something about what the user shared.

Every failure returns "": an unreadable home directory, or one that does not
exist. That last case is not hypothetical caution. The runtime rejects a
DefaultDirectory that is missing by returning an error instead of opening
anything, so guessing here would replace a merely tedious chooser with one
that cannot open at all.
*/
func (a *App) startingDirectory() string {
	if a == nil || a.homeDir == nil {
		return ""
	}
	home, err := a.homeDir()
	if err != nil || home == "" {
		return ""
	}
	info, err := os.Stat(home)
	if err != nil || !info.IsDir() {
		return ""
	}
	return home
}

// publish turns one coordinator lifecycle event into the matching Wails
// runtime emission. It is unexported so Wails cannot bind it; appObserver is
// what satisfies the port.
//
// It is called while the coordinator holds its operation lease, which is what
// orders events causally -- and what makes this method the riskiest code in
// the file. Every later Cancel and Shutdown waits for that lease, so a panic
// escaping here strands it and wedges the application for good. The adapter
// therefore owns that risk instead of exporting it: it guards the context,
// recovers around the emission, calls nothing back into the coordinator, and
// returns.
func (a *App) publish(event transfer.Event) {
	ctx := a.runtimeContext()
	if ctx == nil {
		// Published before the window exists. Dropping it is the only safe
		// answer: the real runtime would call log.Fatalf on a nil context and
		// end the process rather than return.
		a.undelivered.Add(1)
		a.logEvent("undelivered (no window yet)", event)
		return
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			a.undelivered.Add(1)
			a.logEvent("undelivered (emit panicked)", event)
		}
	}()

	if !emittableKinds[event.Kind] {
		// The event name is the whole routing decision, so an empty or unknown
		// kind would emit under a name no listener subscribes to -- lost, and
		// indistinguishable from never having happened. Counting it is what
		// makes a coordinator that grows a sixth kind visible here rather than
		// silent in the UI.
		a.undelivered.Add(1)
		a.logEvent("undelivered (unknown kind)", event)
		return
	}
	if event.Kind != transfer.TransferProgress {
		a.logEvent("event", event)
	}

	// Event.Kind is json:"-", so the kind travels as the event name and the
	// payload carries sessionId, seq, and whichever of progress and error this
	// event kind is allowed to have.
	a.emit(ctx, string(event.Kind), event)
}

// logEvent writes the one line a lifecycle event leaves behind. Everything in
// it is either a fixed word, a number, or a value the contract already allows
// on the wire to the window; nothing here can name a path or a token.
func (a *App) logEvent(what string, event transfer.Event) {
	if a == nil || a.logf == nil {
		return
	}
	detail := ""
	if event.Progress != nil {
		detail += " bytes=" + strconv.FormatInt(event.Progress.BytesSent, 10)
	}
	if event.Error != nil {
		detail += " code=" + string(event.Error.Code)
	}
	a.logf("fairdrop: %s %s seq=%d session=%s%s", what, event.Kind, event.Seq, event.SessionID, detail)
}

// startup is called when the app starts. The context is saved so the Wails
// runtime methods -- EventsEmit and the two dialogs -- can be called later.
// This is the application-lifetime context and the only one the App stores.
func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ctx = ctx
}

// shutdown is called when the app terminates. It blocks until every resource
// the coordinator owns is gone, which is the intended trade: the alternative
// is a process that exits while its listener is still accepting connections.
//
// The hook's own context is discarded rather than stored. Wails hands
// OnShutdown a shutting-down context, and installing it over the
// application-lifetime one would leave the App holding a context it could
// never emit through again.
func (a *App) shutdown(_ context.Context) {
	_, coordinator := a.delegate()
	if coordinator == nil {
		return
	}
	// Shutdown is idempotent and reports cleanup diagnostics it has already
	// made safe; there is no UI left to tell, and no caller above this one.
	_ = coordinator.Shutdown()
}

// restoreWindow is the Wails OnSecondInstanceLaunch callback: a second launch
// -- a double-click, "Open with", a stale shortcut -- hands the already-running
// process this instead of starting a competing coordinator, listener and
// beacon. It reaches the window through the same two runtime seams NewApp
// wires to WindowUnminimise and WindowShow, unminimising before showing so a
// minimised window is not left minimised by a Show that cannot see past it;
// the runtime marshals the actual work to the UI thread itself.
//
// The second launch's Args and WorkingDirectory are ignored: nothing here
// stages a path, so a second launch pointed at a file cannot compete with a
// live transfer. Nothing here calls the coordinator or emits a lifecycle
// event either -- the window's session, retained outcome and focus are left
// exactly as they were, and the coordinator's operation lease, which a live
// transfer may hold, is never waited on.
//
// Wails invokes this from a goroutine of its own that OnStartup never
// synchronises with, so a second launch can win the race before a.ctx is
// installed. The nil-context guard is not defensive: the real runtime
// functions answer a context that never came from a window with log.Fatalf,
// not an error, so calling them here would take the whole process down
// instead of just failing this restoration.
func (a *App) restoreWindow(_ options.SecondInstanceData) {
	ctx := a.runtimeContext()
	if ctx == nil {
		a.undelivered.Add(1)
		if a.logf != nil {
			a.logf("fairdrop: undelivered (second instance before startup)")
		}
		return
	}
	a.unminimise(ctx)
	a.show(ctx)
}

// delegate reads the two fields a command needs. The context is the
// application-lifetime one, falling back to a background context only before
// startup has run, so a command can never be handed a nil context.
func (a *App) delegate() (context.Context, transferCoordinator) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return ctx, a.transfers
}

// runtimeContext returns the stored Wails context, or nil before startup. The
// nil is meaningful to both callers, so unlike delegate it is not defaulted.
func (a *App) runtimeContext() context.Context {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.ctx
}

// errNotComposed is the coded refusal a command returns when it is reached
// before main.go has handed the App its coordinator. It cannot happen in a
// composed binary; it exists so a command answers with a code rather than a
// nil dereference if composition is ever reordered.
func errNotComposed() error {
	return transfer.NewError(
		transfer.ErrTransferFailed,
		"FairDrop is not ready to run a transfer command",
	)
}
