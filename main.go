package main

import (
	"embed"
	"encoding/json"
	"log"

	"fairdrop/internal/network"
	"fairdrop/internal/qr"
	"fairdrop/internal/server"
	"fairdrop/internal/source"
	"fairdrop/internal/stream"
	"fairdrop/internal/transfer"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// The real coordinator has to satisfy the App's view of it. Asserted here
// rather than in app.go so app.go keeps naming no concrete implementation.
var _ transferCoordinator = (*transfer.Coordinator)(nil)

// unknownCommandError is the serialized public error a command failure falls
// back to when it cannot be marshalled. It is spelled out rather than derived
// so a change to the public copy has to be made deliberately in both places --
// main_test.go pins the two together.
const unknownCommandError = `{"code":"transfer_failed","message":"The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link."}`

// compose builds the one coordinator this application runs on and closes the
// App/coordinator cycle.
//
// It is the only place a concrete adapter is named. app.go translates Wails
// calls and the coordinator owns the lifecycle, so neither of them may know
// that discovery is mDNS, that the QR code comes from a barcode library, or
// that a payload is a descriptor on a local disk.
func compose(app *App) *transfer.Coordinator {
	// One inspector, reached twice on purpose: the coordinator validates the
	// admitted selection through the ancestor-resolving boundary at Stage,
	// and the payload adapter re-validates the canonical root directly before
	// it opens a descriptor. Two inspectors would be
	// two independent answers to "is this path acceptable".
	inspector := source.New()

	coordinator := transfer.NewCoordinator(transfer.Dependencies{
		Source:   newSelectionSource(inspector),
		Network:  network.NewManager(),
		Server:   server.New(stream.New(inspector)),
		QR:       qr.New(),
		Observer: appObserver{app: app},
		// Diagnose is what carries an internal diagnostic out of the process.
		// Without it the coordinator's sink is written by production and read
		// only by tests, which is what D-098 found: the contract's stated
		// honesty mechanism did not exist in a shipped binary.
		Diagnose: app.logDiagnostic,
		// Entropy, Now and AfterFunc stay defaulted: the process CSPRNG, the
		// process clock and time.AfterFunc are the production sources, and
		// only coordinator tests replace them.
	})

	app.useCoordinator(coordinator)
	return coordinator
}

// formatCommandError is the only path a command failure takes to the frontend.
//
// It serializes the public error -- a stable code and the fixed copy that code
// selects -- as a JSON string, because that string becomes Error.message in
// the rejection the generated binding produces, and that message is what
// parseCommandError reads. PublicErrorOf decides the code, not a second
// mapping here: it recognizes a coded error through its wrappers, maps
// everything else to transfer_failed, and never copies adapter text.
func formatCommandError(err error) any {
	if err == nil {
		// Wails only calls this for a real failure, but PublicErrorOf(nil) is
		// the zero value, which would serialize an empty code the frontend
		// could only treat as unknown anyway. Answering with the fallback
		// keeps the function total and its output always a valid public error.
		return unknownCommandError
	}

	encoded, marshalErr := json.Marshal(transfer.PublicErrorOf(err))
	if marshalErr != nil {
		// PublicError is two strings, so this cannot happen today. Falling
		// back rather than reaching for err.Error() is what keeps the
		// disclosure rule true if that shape ever grows: adapter text may not
		// reach the frontend even on a serialization failure.
		return unknownCommandError
	}
	return string(encoded)
}

// singleInstanceLockUniqueID identifies FairDrop to Wails' single-instance
// lock. It is a fixed, arbitrary UUID -- not a secret and not tied to any
// build -- so every launch that carries it recognizes every other launch as
// the same application. On Windows the lock is a named mutex with no
// "Global\" prefix, so it is session-local: it recognizes every launch inside
// the same logged-in session, not a launch from a different user or session.
// main_test.go pins this exact literal: a value that drifted between builds
// would silently let two processes run at once.
//
// A second launch does still compose before it is turned away. main() calls
// newBoundApp -- which builds a coordinator, network manager, server and QR
// encoder -- before wails.Run is even called, and Wails only checks this lock
// once it is running, inside Frontend.Run. None of that construction binds a
// listener or starts a beacon: NewCoordinator only assigns fields,
// network.NewManager and qr.New only close over dependencies, and server.New
// stores a listen closure it does not call. That construction is therefore
// inert, and the second process's os.Exit(0) discards it before anything the
// coordinator owns is ever started -- see app.go's restoreWindow for what the
// first process's window does in response.
const singleInstanceLockUniqueID = "d1766c78-45cf-4e6d-9f04-c3700ab32024"

// appOptions builds the Wails configuration for FairDrop.
//
// This is deliberately separate from main: wails.Run opens a real window and
// cannot be called from a test, so without this seam nothing would assert the
// options contract. Flipping DragAndDrop.EnableFileDrop to false leaves every
// build and lint check green while shipping a binary that silently discards
// every drop -- see main_test.go.
func appOptions(app *App) *options.App {
	return appOptionsWithLockProbe(app, nativeSingleInstanceLockUsable)
}

func appOptionsWithLockProbe(app *App, usable func() bool) *options.App {
	return appOptionsWith(app, usable, nativeOSPrefersDarkTheme)
}

// appOptionsWith adds the theme probe to the lock probe above. Two seams
// rather than one signature change: appOptionsWithLockProbe is named in a
// source-text pin and in every existing options assertion, and there is no
// reason a theme test should rewrite those.
func appOptionsWith(app *App, usable func() bool, prefersDark func() bool) *options.App {
	return &options.App{
		Title:     "FairDrop",
		Width:     1024,
		Height:    768,
		MinWidth:  640,
		MinHeight: 480,

		// Standard OS window chrome.
		Frameless:        false,
		WindowStartState: options.Normal,

		// Native OS file drop: hands the frontend absolute paths for
		// dropped files and directories.
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
		},

		AssetServer: &assetserver.Options{
			Assets: assets,
		},

		// The shade the native window paints before the webview renders. It
		// must track --color-canvas in frontend/src/style.css, or the window
		// flashes one theme and repaints in another; main_test.go pins the two
		// together. Wails takes a single value and offers no per-theme one, so
		// the OS preference is read here, before the options exist, and the
		// matching canvas chosen (D-055).
		BackgroundColour: canvasFor(prefersDark()),

		// Without this, a rejected command carries err.Error() -- raw adapter
		// text -- and the frontend has no stable code to switch on.
		ErrorFormatter: formatCommandError,

		// Exactly one FairDrop process runs. A second launch hands Wails a
		// SecondInstanceData carrying its own Args and WorkingDirectory; the
		// callback is app.go's restoreWindow, which ignores both by design and
		// restores the existing window instead -- see its comment for why it
		// touches neither the coordinator nor a lifecycle event, and for what a
		// second launch's inert composition does before it ever gets here.
		SingleInstanceLock: singleInstanceOption(app, usable),

		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	}
}

func singleInstanceOption(app *App, usable func() bool) *options.SingleInstanceLock {
	if !usable() {
		// Fixed diagnostic only: filesystem errors can contain private paths.
		app.logf("fairdrop: single-instance protection unavailable (temporary lock file unusable); launching without protection")
		return nil
	}
	return &options.SingleInstanceLock{UniqueId: singleInstanceLockUniqueID, OnSecondInstanceLaunch: app.restoreWindow}
}

// canvasFor is the native window's pre-paint colour for each OS theme, and the
// only place either token is written in Go.
//
// Both must equal --color-canvas in frontend/src/style.css for their mode. A
// value that disagrees is a visible flash on every launch of that theme, and
// nothing in the frontend suite can see a Go constant -- which is how the light
// one once tracked a Tailwind class Story 1.9 had deleted.
func canvasFor(dark bool) *options.RGBA {
	if dark {
		return &options.RGBA{R: 0x1C, G: 0x19, B: 0x16, A: 1}
	}
	return &options.RGBA{R: 0xF7, G: 0xF0, B: 0xE7, A: 1}
}

// newBoundApp builds the App exactly as main does: wired to the real Wails
// runtime and holding the composed coordinator.
//
// It is a seam for the same reason appOptions is one. main cannot be called
// from a test, so without this nothing asserted that main composes at all --
// deleting the compose call shipped a binary whose every command answers "not
// ready" while the whole suite stayed green.
func newBoundApp() *App {
	app := NewApp()
	compose(app)
	return app
}

func main() {
	release, held := acquireInstanceLock()
	if held {
		defer release()
	}

	app := buildApp(held)

	if err := wails.Run(appOptions(app)); err != nil {
		// log.Fatal, not println: a bare print would fall off the end of main
		// and exit 0, reporting success to CI after a failed launch.
		log.Fatalf("fairdrop: %v", err)
	}
}

// buildApp returns the App main runs: composed when this process holds the
// user's instance lock, inert when it does not.
//
// Inert rather than absent, because Wails' own single-instance handoff has not
// run yet and it is the thing that restores the existing window. On the
// ordinary path Wails sees the first instance and exits this process before the
// window appears, so nothing uncomposed is ever shown.
//
// This window reaches a user only when Wails' own lock misses the first
// instance, and there are three such paths, not the two this comment named
// until the Epic 3 retrospective (B12). Two are the Windows fallthroughs D-088
// describes: SetupSingleInstance reads any CreateMutex error other than
// ERROR_ALREADY_EXISTS as "nobody is running", which an elevated first instance
// produces, and it falls through when FindWindowW has not yet found the first
// instance's event window, which a tight double launch produces. The third is
// macOS, added later: singleInstanceOption passes Wails no lock at all when
// nativeSingleInstanceLockUsable fails its probe, which disables the handoff
// the same way.
//
// On every one of them each command answers that FairDrop is not ready, which
// is the point: a second listener and a second beacon are what must not happen
// (D-088). Whether an inert window is the right answer on macOS specifically is
// an open question the retrospective routed rather than settled -- D-088 chose
// it for Windows deliberately, and nobody has decided it applies here.
func buildApp(held bool) *App {
	if !held {
		app := NewApp()
		// Fixed diagnostic: a lock path is a filesystem path (AD-9).
		app.logf("fairdrop: another FairDrop already holds this user's instance lock; this window starts no transfer")
		return app
	}

	defer reportWiringPanic(showFatalDialog)
	return newBoundApp()
}

// fatalWiringMessage is the whole of what a user is told when composition
// fails. It names no port, no path and nothing the panic carried: a wiring
// defect is not a situation a user can act on, and AD-9 does not relax because
// the process is dying.
const fatalWiringMessage = "FairDrop could not start because of an internal defect in this build. " +
	"Nothing was sent, and no transfer was started."

// reportWiringPanic shows a panic to the user before letting it continue.
//
// NewCoordinator panics when a port is nil, which is the right answer for a
// wiring defect and is what makes ready()'s nil-port branch unreachable. What
// had no answer was the shape of that failure in a release build: compose runs
// before wails.Run, no recover covered it, and a release build has no console,
// so the process would vanish with no window, no dialog and no message (D-107).
//
// The panic continues rather than being swallowed. A recovered wiring defect
// would leave a half-composed process running, and the stack trace the runtime
// prints is what a developer needs -- the dialog is for the person who
// double-clicked, not instead of the diagnosis.
func reportWiringPanic(show func(string)) {
	recovered := recover()
	if recovered == nil {
		return
	}
	show(fatalWiringMessage)
	panic(recovered)
}
