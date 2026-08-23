package main

import (
	"context"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved so the Wails
// runtime methods (notably runtime.EventsEmit) can be called from later phases.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called when the app terminates. Later phases tear down the
// ephemeral HTTP listener and the mDNS beacon here.
func (a *App) shutdown(ctx context.Context) {}
