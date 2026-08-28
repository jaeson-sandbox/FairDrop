package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fairdrop/internal/transfer"

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

// The formatter is what turns a command failure into something the frontend
// can act on. Without it Wails sends err.Error() -- raw adapter text with no
// stable code -- and every rejection collapses into one indistinguishable
// string. Compilation cannot catch its absence, so it is pinned here beside
// the drop and window options.
func TestAppOptionsRegistersTheErrorFormatter(t *testing.T) {
	opts := appOptions(NewApp())

	if opts.ErrorFormatter == nil {
		t.Fatal("ErrorFormatter is nil: rejections would carry raw adapter text and no stable code")
	}

	coded := transfer.WrapError(
		transfer.ErrBusy,
		`staging C:\Users\sender\Documents\quarterly report.pdf`,
		errors.New("internal cause"),
	)
	got, ok := opts.ErrorFormatter(coded).(string)
	if !ok {
		t.Fatalf("ErrorFormatter returned %T, want a JSON string", opts.ErrorFormatter(coded))
	}

	const want = `{"code":"busy","message":"Finish or cancel the current transfer before choosing another item."}`
	if got != want {
		t.Errorf("ErrorFormatter produced\n %s\nwant\n %s", got, want)
	}
}

// The unknown-failure row of the matrix, and the pin that keeps the
// hand-written fallback constant equal to what the formatter really produces.
func TestUnknownFailuresBecomeTheFixedTransferFailedCopy(t *testing.T) {
	const want = `{"code":"transfer_failed","message":"The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link."}`

	got, ok := formatCommandError(errors.New(`open C:\Users\sender\secret.pdf: permission denied`)).(string)
	if !ok {
		t.Fatal("formatCommandError returned a non-string for an unrecognized error")
	}
	if got != want {
		t.Errorf("an unrecognized error formatted as\n %s\nwant\n %s", got, want)
	}
	if unknownCommandError != want {
		t.Errorf("the fallback constant is\n %s\nwant\n %s", unknownCommandError, want)
	}
}

// The frontend's code list is the other half of the error contract, and
// nothing in either toolchain compiles across the boundary. This is the pin:
// a code added or renamed in Go without the matching frontend change fails
// here rather than at runtime, where it would simply become transfer_failed.
func TestEveryStableCodeIsRecognizedByTheFrontend(t *testing.T) {
	parser, err := os.ReadFile(filepath.Join("frontend", "src", "transfer", "errors.ts"))
	if err != nil {
		t.Fatalf("the frontend error parser is missing: %v", err)
	}

	// Spelled out, not ranged over a Go slice built from the same constants
	// the formatter uses: a literal list is what makes a renamed code visible.
	for _, code := range []string{
		"invalid_selection",
		"busy",
		"cancelled",
		"path_not_found",
		"path_unsupported",
		"source_changed",
		"network_unavailable",
		"server_start_failed",
		"qr_failed",
		"beacon_warning",
		"transfer_failed",
		"shutting_down",
	} {
		if !strings.Contains(string(parser), `'`+code+`'`) {
			t.Errorf("frontend/src/transfer/errors.ts does not recognize %q", code)
		}

		formatted, ok := formatCommandError(transfer.NewError(transfer.ErrorCode(code), "diagnostic")).(string)
		if !ok {
			t.Fatalf("formatCommandError returned a non-string for %q", code)
		}
		if !strings.Contains(formatted, `"code":"`+code+`"`) {
			t.Errorf("formatCommandError turned %q into %s", code, formatted)
		}
	}
}
