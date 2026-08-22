package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
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
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
