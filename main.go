package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

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

		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	}
}

func main() {
	if err := wails.Run(appOptions(NewApp())); err != nil {
		// log.Fatal, not println: a bare print would fall off the end of main
		// and exit 0, reporting success to CI after a failed launch.
		log.Fatalf("fairdrop: %v", err)
	}
}
