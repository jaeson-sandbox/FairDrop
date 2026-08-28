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
	// selection with it at Stage, and the payload adapter re-validates the
	// same root with it before it opens a descriptor. Two inspectors would be
	// two independent answers to "is this path acceptable".
	inspector := source.New()

	coordinator := transfer.NewCoordinator(transfer.Dependencies{
		Source:   inspector,
		Network:  network.NewManager(),
		Server:   server.New(stream.New(inspector)),
		QR:       qr.New(),
		Observer: app,
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

// appOptions builds the Wails configuration for FairDrop.
//
// This is deliberately separate from main: wails.Run opens a real window and
// cannot be called from a test, so without this seam nothing would assert the
// options contract. Flipping DragAndDrop.EnableFileDrop to false leaves every
// build and lint check green while shipping a binary that silently discards
// every drop -- see main_test.go.
func appOptions(app *App) *options.App {
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

		// Matches the frontend's bg-slate-900 (#0f172a) so the window does not
		// flash a different shade before the webview paints.
		BackgroundColour: &options.RGBA{R: 15, G: 23, B: 42, A: 1},

		// Without this, a rejected command carries err.Error() -- raw adapter
		// text -- and the frontend has no stable code to switch on.
		ErrorFormatter: formatCommandError,

		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	}
}

func main() {
	app := NewApp()
	compose(app)

	if err := wails.Run(appOptions(app)); err != nil {
		// log.Fatal, not println: a bare print would fall off the end of main
		// and exit 0, reporting success to CI after a failed launch.
		log.Fatalf("fairdrop: %v", err)
	}
}
