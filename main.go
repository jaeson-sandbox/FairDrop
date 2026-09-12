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
		// together. Wails takes a single value, so this is the light canvas and
		// a dark-mode OS still gets one light frame -- deferred-work.md carries
		// the theme-aware version.
		BackgroundColour: &options.RGBA{R: 0xF7, G: 0xF0, B: 0xE7, A: 1},

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
	app := newBoundApp()

	if err := wails.Run(appOptions(app)); err != nil {
		// log.Fatal, not println: a bare print would fall off the end of main
		// and exit 0, reporting success to CI after a failed launch.
		log.Fatalf("fairdrop: %v", err)
	}
}
