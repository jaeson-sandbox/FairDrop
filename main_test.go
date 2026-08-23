package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

// Phase 1 exists to produce a window that receives native OS file drops.
// Every other check in this repo -- go build, go vet, npm test, npm run build,
// wails build -- stays green with EnableFileDrop flipped to false, so this
// assertion is the only thing between a regression and a binary that silently
// discards every drop.
func TestAppOptionsEnablesNativeFileDrop(t *testing.T) {
	opts := appOptions(NewApp())

	if opts.DragAndDrop == nil {
		t.Fatal("DragAndDrop is nil: native file drop is not configured at all")
	}
	if !opts.DragAndDrop.EnableFileDrop {
		t.Error("DragAndDrop.EnableFileDrop = false, want true: dropped files would be silently discarded")
	}
}

func TestAppOptionsWindowContract(t *testing.T) {
	opts := appOptions(NewApp())

	if opts.Title != "FairDrop" {
		t.Errorf("Title = %q, want %q", opts.Title, "FairDrop")
	}
	if opts.Frameless {
		t.Error("Frameless = true, want false: the approved decision was a standard OS frame")
	}
	if opts.WindowStartState != options.Normal {
		t.Errorf("WindowStartState = %v, want options.Normal", opts.WindowStartState)
	}
}

func TestAppOptionsRegistersLifecycleHooks(t *testing.T) {
	opts := appOptions(NewApp())

	if opts.OnStartup == nil {
		t.Error("OnStartup is nil: a.ctx would never be captured, so runtime.EventsEmit fails in later phases")
	}
	if opts.OnShutdown == nil {
		t.Error("OnShutdown is nil: later phases need it to tear down the listener and mDNS beacon")
	}
}
