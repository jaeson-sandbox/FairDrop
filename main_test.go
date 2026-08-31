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

// The window paints BackgroundColour before the webview renders anything, so a
// value that disagrees with the frontend's canvas is a visible flash on every
// launch -- and nothing in the frontend suite can see a Go constant. This
// caught exactly that: the constant tracked a Tailwind class that Story 1.9
// deleted, leaving every light-mode start painting slate-900 and repainting
// cream.
func TestAppOptionsBackgroundTracksTheCanvasToken(t *testing.T) {
	const token = "--color-canvas: #F7F0E7;"

	stylesheet, err := os.ReadFile(filepath.Join("frontend", "src", "style.css"))
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}
	if !strings.Contains(string(stylesheet), token) {
		t.Fatalf("style.css no longer declares %q -- update this test and the option together", token)
	}

	got := appOptions(NewApp()).BackgroundColour
	if got == nil {
		t.Fatal("BackgroundColour is nil: the window would paint the platform default, not the canvas")
	}
	want := options.RGBA{R: 0xF7, G: 0xF0, B: 0xE7, A: 1}
	if *got != want {
		t.Errorf("BackgroundColour = %+v, want %+v (the light --color-canvas)", *got, want)
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

	const listStart = "export const transferErrorCodes = ["
	start := strings.Index(string(parser), listStart)
	if start < 0 {
		t.Fatal("frontend/src/transfer/errors.ts no longer exports transferErrorCodes")
	}
	end := strings.Index(string(parser)[start:], "]")
	if end < 0 {
		t.Fatal("the transferErrorCodes array is unterminated")
	}
	codeList := string(parser)[start : start+end]

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
		// Inside the exported array, not merely somewhere in the file: a code
		// surviving only in a comment or an unused union would otherwise keep
		// this green while parseCommandError rejected it.
		if !strings.Contains(codeList, `'`+code+`'`) {
			t.Errorf("frontend/src/transfer/errors.ts does not list %q in transferErrorCodes", code)
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

// Bind is what makes the App callable at all. Emptying it ships a binary with
// zero commands while go build, go vet, go test and wails build all pass --
// the same argument the file already makes for EnableFileDrop, and it became
// load-bearing only when this story gave the App its first exported method.
func TestAppOptionsBindsTheApp(t *testing.T) {
	app := NewApp()
	opts := appOptions(app)

	if len(opts.Bind) != 1 {
		t.Fatalf("Bind holds %d entries, want exactly the App", len(opts.Bind))
	}
	if opts.Bind[0] != app {
		t.Error("Bind does not hold the App the options were built for: its commands would not be callable")
	}
}

// main composes before it runs. Deleting that call shipped a binary whose
// every command answered "not ready" with the whole suite green, because
// nothing reached past appOptions into how main builds its App.
func TestNewBoundAppInstallsACoordinator(t *testing.T) {
	app := newBoundApp()

	app.mu.RLock()
	installed := app.transfers
	app.mu.RUnlock()

	if installed == nil {
		t.Fatal("newBoundApp returned an App with no coordinator: every command would refuse")
	}
	if _, ok := installed.(*transfer.Coordinator); !ok {
		t.Errorf("the installed coordinator is %T, want *transfer.Coordinator", installed)
	}
}

// A nil error is not a failure, so the formatter must still answer with a
// valid public error rather than an empty code the frontend cannot switch on.
func TestFormatCommandErrorIsTotal(t *testing.T) {
	got, ok := formatCommandError(nil).(string)
	if !ok {
		t.Fatal("formatCommandError returned a non-string for a nil error")
	}
	if got != unknownCommandError {
		t.Errorf("formatCommandError(nil) = %s, want the fixed fallback", got)
	}
}

// Deferred work is where this project puts a real finding it is not fixing yet,
// and prose alone let those findings stop being anyone's problem: entries named
// an owning story inside their evidence text, or named none at all, and nothing
// noticed. Every entry now carries an owner, and this fails if one is missing or
// names a story that does not exist -- including a story key renamed in
// sprint-status.yaml without the entries that point at it.
func TestEveryDeferredEntryHasALiveOwner(t *testing.T) {
	artifacts := filepath.Join("_bmad-output", "implementation-artifacts")

	deferred, err := os.ReadFile(filepath.Join(artifacts, "deferred-work.md"))
	if err != nil {
		t.Fatalf("read deferred-work.md: %v", err)
	}
	sprint, err := os.ReadFile(filepath.Join(artifacts, "sprint-status.yaml"))
	if err != nil {
		t.Fatalf("read sprint-status.yaml: %v", err)
	}

	// Story keys are the indented `key: status` lines inside development_status,
	// and only those: reading every indented line in the file would let an
	// unrelated key satisfy an owner, and would leave the vacuity guard below
	// unable to fire when the block itself is renamed away.
	stories := map[string]bool{}
	inBlock := false
	for _, line := range strings.Split(strings.ReplaceAll(string(sprint), "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(line, " ") && trimmed != "" {
			inBlock = trimmed == "development_status:"
			continue
		}
		if !inBlock || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, _, found := strings.Cut(trimmed, ":")
		if found && key != "" {
			stories[key] = true
		}
	}
	if len(stories) == 0 {
		t.Fatal("no story keys parsed from sprint-status.yaml, so this test would pass vacuously")
	}

	var summaries, owners int
	for _, line := range strings.Split(strings.ReplaceAll(string(deferred), "\r\n", "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "  summary:"):
			summaries++
		case strings.HasPrefix(line, "  owner:"):
			owners++
			owner := strings.TrimSpace(strings.TrimPrefix(line, "  owner:"))
			switch {
			case owner == "discharged" || owner == "accepted":
			case stories[owner]:
			default:
				t.Errorf("deferred entry %d is owned by %q, which is not a sprint-status story: "+
					"the finding has no story that will resolve it", owners, owner)
			}
		}
	}

	if summaries == 0 {
		t.Fatal("no deferred entries parsed, so this test would pass vacuously")
	}
	if summaries != owners {
		t.Errorf("%d deferred entries carry %d owners: %d finding(s) belong to nobody",
			summaries, owners, summaries-owners)
	}
}
