package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"fairdrop/internal/server"
	"fairdrop/internal/transfer"

	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestCoordinatorCleanupOutlastsServerTeardown(t *testing.T) {
	if server.TeardownBound != 10*time.Second {
		t.Fatalf("server.TeardownBound = %v, want 10s", server.TeardownBound)
	}
	if transfer.AdapterCleanupBound != 15*time.Second {
		t.Fatalf("transfer.AdapterCleanupBound = %v, want 15s", transfer.AdapterCleanupBound)
	}
	if transfer.AdapterCleanupBound <= server.TeardownBound {
		t.Fatalf("coordinator cleanup bound %v must outlast server teardown bound %v", transfer.AdapterCleanupBound, server.TeardownBound)
	}
}

// Phase 1 exists to produce a window that receives native OS file drops.
// Every other check in this repo -- go build, go vet, npm test, npm run build,
// wails build -- stays green with EnableFileDrop flipped to false, so this
// assertion is the only thing between a regression and a binary that silently
// discards every drop.
func TestAppOptionsEnablesNativeFileDrop(t *testing.T) {
	opts := appOptionsWithLockProbe(NewApp(), func() bool { return true })

	if opts.DragAndDrop == nil {
		t.Fatal("DragAndDrop is nil: native file drop is not configured at all")
	}
	if !opts.DragAndDrop.EnableFileDrop {
		t.Error("DragAndDrop.EnableFileDrop = false, want true: dropped files would be silently discarded")
	}
}

func TestAppOptionsWindowContract(t *testing.T) {
	opts := appOptionsWithLockProbe(NewApp(), func() bool { return true })

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
	stylesheet, err := os.ReadFile(filepath.Join("frontend", "src", "style.css"))
	if err != nil {
		t.Fatalf("read stylesheet: %v", err)
	}
	declared := string(stylesheet)

	// Both modes, because Wails paints one colour and the OS decides which one
	// it should be. Before D-055 only the light token was pinned and a
	// dark-mode machine got a light frame at every launch -- a defect no test
	// could see, since the dark token was never read on the Go side at all.
	for _, theme := range []struct {
		name  string
		dark  bool
		token string
		want  options.RGBA
	}{
		{"light", false, "--color-canvas: #F7F0E7;", options.RGBA{R: 0xF7, G: 0xF0, B: 0xE7, A: 1}},
		{"dark", true, "--color-canvas: #1C1916;", options.RGBA{R: 0x1C, G: 0x19, B: 0x16, A: 1}},
	} {
		t.Run(theme.name, func(t *testing.T) {
			if !strings.Contains(declared, theme.token) {
				t.Fatalf("style.css no longer declares %q -- update this test and canvasFor together", theme.token)
			}

			got := appOptionsWith(NewApp(), func() bool { return true }, func() bool { return theme.dark }).BackgroundColour
			if got == nil {
				t.Fatal("BackgroundColour is nil: the window would paint the platform default, not the canvas")
			}
			if *got != theme.want {
				t.Errorf("BackgroundColour = %+v, want %+v (the %s --color-canvas)", *got, theme.want, theme.name)
			}
		})
	}

	// The dark token lives inside the prefers-color-scheme block, so a
	// stylesheet that declared it at :root would satisfy the loop above while
	// meaning something else entirely.
	_, darkBlock, found := strings.Cut(declared, "@media (prefers-color-scheme: dark)")
	if !found {
		t.Fatal("style.css has no dark-scheme block, so the dark token above is not the dark theme's")
	}
	if !strings.Contains(darkBlock, "--color-canvas: #1C1916;") {
		t.Error("the dark canvas token is not declared inside the prefers-color-scheme block")
	}
}

// TestTheNativeThemeProbeIsWiredToTheOptions pins the half a fake cannot: that
// the composed options ask the operating system at all.
//
// appOptionsWith takes the probe, so every test above can drive both themes
// without either OS -- and that is exactly what would let the production path
// keep a hardcoded light canvas while the suite stayed green, the same shape as
// Story 3.3's workflow_call and Story 3.6's diagnostic seam.
func TestTheNativeThemeProbeIsWiredToTheOptions(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	found := false
	for _, line := range splitLines(source) {
		if strings.Contains(line, "return appOptionsWith(app, usable, nativeOSPrefersDarkTheme)") {
			found = true
		}
	}
	if !found {
		t.Error("appOptionsWithLockProbe does not pass nativeOSPrefersDarkTheme, so a shipped FairDrop " +
			"never reads the OS theme however well the seam is tested")
	}
}

func TestAppOptionsRegistersLifecycleHooks(t *testing.T) {
	opts := appOptionsWithLockProbe(NewApp(), func() bool { return true })

	if opts.OnStartup == nil {
		t.Error("OnStartup is nil: a.ctx would never be captured, so runtime.EventsEmit fails in later phases")
	}
	if opts.OnShutdown == nil {
		t.Error("OnShutdown is nil: later phases need it to tear down the listener and mDNS beacon")
	}
}

// Exactly one FairDrop process may run: a second launch must recognize the
// first rather than starting a competing coordinator, listener and beacon.
// The UniqueId is spelled out as a literal, not a reference to the constant
// under test, so a build that quietly changed the UUID fails here rather than
// only inside main.go, and OnSecondInstanceLaunch is asserted present so a
// second launch always has somewhere to hand its window off to.
func TestAppOptionsEnforcesSingleInstance(t *testing.T) {
	opts := appOptionsWithLockProbe(NewApp(), func() bool { return true })

	if opts.SingleInstanceLock == nil {
		t.Fatal("SingleInstanceLock is nil: a second launch would start a competing coordinator, listener and beacon")
	}

	const want = "d1766c78-45cf-4e6d-9f04-c3700ab32024"
	if opts.SingleInstanceLock.UniqueId != want {
		t.Errorf("SingleInstanceLock.UniqueId = %q, want %q", opts.SingleInstanceLock.UniqueId, want)
	}
	if opts.SingleInstanceLock.OnSecondInstanceLaunch == nil {
		t.Fatal("OnSecondInstanceLaunch is nil: a second launch would have nothing to hand the window to")
	}
}

func TestAppOptionsDefaultUsesNativeLockProbe(t *testing.T) {
	data, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ReplaceAll(string(data), "\r\n", "\n"), "return appOptionsWithLockProbe(app, nativeSingleInstanceLockUsable)") {
		t.Fatal("production appOptions bypasses the native lock probe")
	}
	app := NewApp()
	var logged int
	app.logf = func(string, ...any) { logged++ }
	calls := 0
	opts := appOptionsWithLockProbe(app, func() bool { calls++; return false })
	if calls != 1 || opts.SingleInstanceLock != nil || logged != 1 {
		t.Fatal("degraded options must run the probe, disable locking and report once")
	}
}

// The non-nil check above is satisfied by any callback, including a stub
// func(options.SecondInstanceData) {} that restores nothing -- appOptions
// could swap in one and every test would stay green. This drives the callback
// appOptions actually wires, through the options value itself, so only the
// real restoreWindow -- with the real fake seams a harness installed on this
// App -- can pass.
func TestAppOptionsSecondInstanceCallbackRestoresTheWindow(t *testing.T) {
	h := newHarness(t)
	opts := appOptionsWithLockProbe(h.app, func() bool { return true })

	opts.SingleInstanceLock.OnSecondInstanceLaunch(options.SecondInstanceData{Args: []string{testPath}})

	got := h.windowActionsLogged()
	if len(got) != 2 || got[0].name != "unminimise" || got[1].name != "show" {
		t.Fatalf("the callback produced %+v, want exactly [unminimise, show]", got)
	}
	if got[0].ctx != h.ctx || got[1].ctx != h.ctx {
		t.Error("the callback did not use the stored application-lifetime context")
	}
	if calls := h.coordinator.log(); len(calls) != 0 {
		t.Errorf("the callback reached the coordinator: %v", calls)
	}
	if events := h.emitted(); len(events) != 0 {
		t.Errorf("the callback emitted %+v", events)
	}
}

// The pre-startup half of the same proof, so a stub could not pass this path
// either by, say, answering "" or panicking instead of counting the drop.
func TestAppOptionsSecondInstanceCallbackBeforeStartupIsSafe(t *testing.T) {
	h := newUnstartedHarness(t)
	opts := appOptionsWithLockProbe(h.app, func() bool { return true })

	opts.SingleInstanceLock.OnSecondInstanceLaunch(options.SecondInstanceData{})

	if got := h.windowActionsLogged(); len(got) != 0 {
		t.Errorf("the callback called the runtime before startup: %+v", got)
	}
	if got := h.app.undelivered.Load(); got != 1 {
		t.Errorf("undelivered = %d, want 1", got)
	}
	if lines := h.logged(); len(lines) != 1 {
		t.Errorf("logged %d lines, want exactly 1: %q", len(lines), lines)
	}
}

// The formatter is what turns a command failure into something the frontend
// can act on. Without it Wails sends err.Error() -- raw adapter text with no
// stable code -- and every rejection collapses into one indistinguishable
// string. Compilation cannot catch its absence, so it is pinned here beside
// the drop and window options.
func TestAppOptionsRegistersTheErrorFormatter(t *testing.T) {
	opts := appOptionsWithLockProbe(NewApp(), func() bool { return true })

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

	const want = `{"code":"busy","message":"FairDrop is still finishing the last item. If it doesn’t finish, close FairDrop and reopen it."}`
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

// registryEntries is the complete cross-language error contract: every
// stable code and the exact PublicError.message it must produce everywhere a
// user can see it. It is spelled out here -- not derived from any file under
// test, including the Go map itself, which is unexported and in a different
// package anyway -- because a literal is what turns a code or message that
// silently drifted in exactly one of the four places (EXPERIENCE.md's
// registry, docs/fairdrop-contracts.md's binding code list, the Go table in
// internal/transfer/errors.go, and the TypeScript mirror in
// frontend/src/transfer/errors.ts) into a named failure instead of two
// out-of-date things quietly agreeing with each other.
var registryEntries = []struct {
	code    string
	message string
}{
	{"invalid_selection", "Choose exactly one file or folder."},
	{"busy", "FairDrop is still finishing the last item. If it doesn’t finish, close FairDrop and reopen it."},
	{"cancelled", "Transfer canceled."},
	{"path_not_found", "That file or folder is no longer available. Choose it again."},
	{"path_unsupported", "FairDrop can use regular files and folders only. Choose another item."},
	{"source_changed", "The item changed after it was prepared. Cancel and create a fresh link."},
	{"network_unavailable", "FairDrop couldn’t find a usable local network. Connect to local Wi-Fi, then try again."},
	{"server_start_failed", "FairDrop couldn’t open a local transfer connection. Check firewall access, then try again."},
	{"qr_failed", "FairDrop couldn’t create the QR code. Prepare the item again."},
	{"setup_failed", "FairDrop couldn’t prepare that item. Nothing was sent. Choose it again."},
	{"beacon_warning", "Device discovery isn’t available. The QR code and download link still work."},
	{"transfer_failed", "The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link."},
	{"cleanup_unconfirmed", "FairDrop couldn’t confirm it released the connection. Nothing was sent. Close FairDrop and reopen it before sending again."},
	{"not_ready", "FairDrop isn’t ready to send. Another copy may already be running. Close this window and use that one."},
	{"clipboard_failed", "FairDrop couldn’t copy the link. Select the link and copy it yourself."},
	{"name_unsupported", "One name inside that folder can’t be sent safely. Rename it, then choose the folder again."},
	{"name_warning", "Some names in this folder can’t be saved on Windows — usually a colon, an asterisk, or a trailing dot or space. They’re sent unchanged; a Windows receiver may not be able to extract those items."},
	{"shutting_down", "FairDrop is closing. Reopen it to start a transfer."},
	{"chooser_failed", "FairDrop couldn’t open the chooser. Try again, or drag the item onto the window."},
}

// TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage proves the four
// places that describe FairDrop's fixed public errors cannot silently
// disagree, extending the older code-only pin to cover every exact message
// too (Epic 1 retrospective item 6, Story 3.5):
//
//   - EXPERIENCE.md's UX registry -- the source the other three follow.
//   - docs/fairdrop-contracts.md's binding code list -- codes only, no
//     messages, so only the code half is checked here.
//   - The Go table in internal/transfer/errors.go, read through the real
//     formatCommandError rather than by parsing source: this is what a
//     rejected command actually carries across the Wails boundary.
//   - The TypeScript mirror in frontend/src/transfer/errors.ts.
//
// A code or message edited in exactly one of these four fails here, naming
// the file and the code that disagrees with the literal registryEntries
// above.
// TestTheRegistryLiteralCoversEveryCodeTheDomainDefines closes the hole under
// the pin above.
//
// registryEntries is a hand-written literal, and it is the sole driver of every
// cross-file comparison in this file. A code dropped from it is not reported as
// missing -- it simply stops being checked anywhere, in every file at once,
// while the suite stays green. That is precisely the failure this story exists
// to fix, one level up: two places quietly agreeing because nothing compares
// them. Found by review.
func TestTheRegistryLiteralCoversEveryCodeTheDomainDefines(t *testing.T) {
	// The domain's own list, read from the Go source rather than from a slice
	// built out of the same constants the literal uses -- a shared helper
	// would make both sides wrong together.
	source, err := os.ReadFile(filepath.Join("internal", "transfer", "errors.go"))
	if err != nil {
		t.Fatalf("read errors.go: %v", err)
	}
	declared := regexp.MustCompile(`ErrorCode = "([a-z_]+)"`).FindAllStringSubmatch(string(source), -1)
	if len(declared) == 0 {
		t.Fatal("no ErrorCode constants parsed from errors.go, so this test would pass vacuously")
	}

	pinned := map[string]bool{}
	for _, entry := range registryEntries {
		pinned[entry.code] = true
	}

	for _, match := range declared {
		if !pinned[match[1]] {
			t.Errorf("%q is a declared ErrorCode but is absent from registryEntries, so nothing checks "+
				"its copy in any of the four files", match[1])
		}
	}
	if len(pinned) != len(declared) {
		t.Errorf("registryEntries has %d codes and errors.go declares %d: the literal and the domain disagree",
			len(pinned), len(declared))
	}
}

func TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage(t *testing.T) {
	registry, err := os.ReadFile(filepath.Join(
		"_bmad-output", "planning-artifacts", "ux-designs", "ux-FairDrop-2026-08-23", "EXPERIENCE.md",
	))
	if err != nil {
		t.Fatalf("read EXPERIENCE.md: %v", err)
	}
	registryTable := tableSection(t, string(registry), "### Stable public error and warning copy", "\n## ")

	contract, err := os.ReadFile(filepath.Join("docs", "fairdrop-contracts.md"))
	if err != nil {
		t.Fatalf("read fairdrop-contracts.md: %v", err)
	}
	contractTable := tableSection(t, string(contract), "Stable domain error codes are:", "\n## ")
	contractConstants := tableSection(t, string(contract), "type ErrorCode string", "\n)")

	mirror, err := os.ReadFile(filepath.Join("frontend", "src", "transfer", "errors.ts"))
	if err != nil {
		t.Fatalf("the frontend error parser is missing: %v", err)
	}
	mirrorText := string(mirror)
	codeList := tableSection(t, mirrorText, "export const transferErrorCodes = [", "]")
	messagesBlock := tableSection(t, mirrorText, "export const fixedErrorMessages", "\n}")

	for _, entry := range registryEntries {
		t.Run(entry.code, func(t *testing.T) {
			// The Go table (internal/transfer/errors.go), through the real
			// formatter -- what a rejected command actually carries.
			formatted, ok := formatCommandError(transfer.NewError(transfer.ErrorCode(entry.code), "diagnostic")).(string)
			if !ok {
				t.Fatalf("formatCommandError returned a non-string for %q", entry.code)
			}
			// Marshalled, not concatenated. encoding/json HTML-escapes <, > and
			// & by default, so a hand-built string compares unequal to a
			// correct message that happens to contain one -- which cost this
			// story a false failure on copy that was right. The registry
			// literal above is still the source of truth; only the encoding
			// of the expectation is borrowed, and never from the table under
			// test.
			want, err := json.Marshal(struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}{Code: entry.code, Message: entry.message})
			if err != nil {
				t.Fatalf("marshal the expected payload for %q: %v", entry.code, err)
			}
			wantJSON := string(want)
			if formatted != wantJSON {
				t.Errorf("errors.go produced\n %s\nwant\n %s", formatted, wantJSON)
			}

			// docs/fairdrop-contracts.md's binding code list: codes only.
			if !strings.Contains(contractTable, "`"+entry.code+"`") {
				t.Errorf("docs/fairdrop-contracts.md's stable domain error codes table does not list %q", entry.code)
			}
			// The same document restates the ErrorCode constants as Go source,
			// which is a second statement of the same fact. Removing a code
			// from that block alone left this test green, so the contract
			// could disagree with itself inside one file.
			if !strings.Contains(contractConstants, `"`+entry.code+`"`) {
				t.Errorf("docs/fairdrop-contracts.md's ErrorCode constant block does not declare %q", entry.code)
			}

			// frontend/src/transfer/errors.ts: the code list, inside the
			// exported array rather than merely somewhere in the file, and
			// the exact message.
			if !strings.Contains(codeList, `'`+entry.code+`'`) {
				t.Errorf("frontend/src/transfer/errors.ts does not list %q in transferErrorCodes", entry.code)
			}
			mirrorMessage := quotedValueAfter(t, messagesBlock, entry.code+":")
			if mirrorMessage != entry.message {
				t.Errorf("frontend/src/transfer/errors.ts's fixedErrorMessages.%s = %q, want %q",
					entry.code, mirrorMessage, entry.message)
			}

			// EXPERIENCE.md's registry: the code and its exact message,
			// between the curly quotes the table already uses.
			if !strings.Contains(registryTable, "`"+entry.code+"`") {
				t.Errorf("EXPERIENCE.md's stable public error table does not list %q", entry.code)
			}
			registryMessage := registryMessageFor(t, registryTable, string(registry), entry.code)
			if registryMessage != entry.message {
				t.Errorf("EXPERIENCE.md's %s row message = %q, want %q", entry.code, registryMessage, entry.message)
			}
		})
	}
}

// TestEveryWarningCodeIsAcceptedByTheFrontendParser closes Epic 1
// retrospective item 3. Warning.Code is now transfer.WarningCode, a
// narrower type than ErrorCode, so a Warning carrying some other recognized
// failure code no longer compiles -- but nothing yet proved the set of
// WarningCode constants this package declares is exactly the set
// frontend/src/transfer/validation.ts's parseWarning accepts. Before the
// type existed, a mismatch there destroyed the whole Stage acknowledgement
// rather than just the one warning: parseWarning rejects an unrecognized
// code, and parseFileMetadata rejects the entire metadata when any one
// warning fails to parse, cancelling a perfectly good session over a
// warning nobody needed to see.
//
// The list is read from the package rather than restated here. It used to be
// a literal, and this comment used to admit what that cost: "a WarningCode
// added to types.go without also being added here is not reported as missing."
// That is the self-referential shape this project keeps rediscovering -- the
// expectation and the value under test maintained separately, so the test
// stays green while the contract drifts. transfer.WarningCodes() is now the
// one list, so a code that reaches the wire without reaching validation.ts
// fails here (Epic 3 retrospective, A7).
func TestEveryWarningCodeIsAcceptedByTheFrontendParser(t *testing.T) {
	everyWarningCode := transfer.WarningCodes()
	if len(everyWarningCode) == 0 {
		t.Fatal("the package reports no warning codes at all, so this pin would pass having checked nothing")
	}

	mirror, err := os.ReadFile(filepath.Join("frontend", "src", "transfer", "validation.ts"))
	if err != nil {
		t.Fatalf("read validation.ts: %v", err)
	}
	parseWarningBody := tableSection(t, string(mirror), "export function parseWarning", "\n}\n")

	for _, code := range everyWarningCode {
		t.Run(string(code), func(t *testing.T) {
			if !strings.Contains(parseWarningBody, "'"+string(code)+"'") {
				t.Errorf("validation.ts's parseWarning does not accept the Go WarningCode %q", code)
			}
		})
	}
}

// tableSection returns the file content between the first occurrence of
// start and the following occurrence of end, so a per-code search is scoped
// to the one table or block it names rather than the whole file -- a code
// name surviving only in a comment or an unrelated section must not satisfy
// this pin.
func tableSection(t *testing.T, content, start, end string) string {
	t.Helper()
	from := strings.Index(content, start)
	if from < 0 {
		t.Fatalf("marker %q not found", start)
	}
	rest := content[from+len(start):]
	to := strings.Index(rest, end)
	if to < 0 {
		t.Fatalf("closing marker %q not found after %q", end, start)
	}
	return rest[:to]
}

// quotedValueAfter finds label (e.g. "busy:") and returns the contents of the
// single-quoted string that follows it, across any whitespace or line break
// in between -- fixedErrorMessages wraps its longest value onto its own
// line, so the label and its string are not always on the same one.
func quotedValueAfter(t *testing.T, block, label string) string {
	t.Helper()
	pattern := regexp.MustCompile(regexp.QuoteMeta(label) + `\s*'([^']*)'`)
	match := pattern.FindStringSubmatch(block)
	if match == nil {
		t.Fatalf("%q not found in errors.ts's fixedErrorMessages", label)
	}
	return match[1]
}

// registryMessageFor finds code's row in EXPERIENCE.md's error table and
// returns its message. Most rows spell the message out between curly quotes;
// beacon_warning's cell instead names the stable Voice and Tone key
// `copy.discovery.warning`, so that indirection is resolved against the full
// registry text rather than skipped.
func registryMessageFor(t *testing.T, table, fullRegistry, code string) string {
	t.Helper()
	rowPattern := regexp.MustCompile(
		"`" + regexp.QuoteMeta(code) + "`[^|]*\\|[^|]*\\|\\s*(`[^`]*`|“[^”]*”)",
	)
	match := rowPattern.FindStringSubmatch(table)
	if match == nil {
		t.Fatalf("row for %q not found in EXPERIENCE.md's error table", code)
	}
	cell := match[1]
	if strings.HasPrefix(cell, "`") {
		return voiceToneMessageFor(t, fullRegistry, strings.Trim(cell, "`"))
	}
	return strings.Trim(cell, "“”")
}

// voiceToneMessageFor resolves a stable copy key (e.g. copy.discovery.warning)
// against EXPERIENCE.md's "Voice and Tone" registry table.
func voiceToneMessageFor(t *testing.T, fullRegistry, key string) string {
	t.Helper()
	pattern := regexp.MustCompile(
		"`" + regexp.QuoteMeta(key) + "`[^|]*\\|[^|]*\\|\\s*“([^”]*)”",
	)
	match := pattern.FindStringSubmatch(fullRegistry)
	if match == nil {
		t.Fatalf("stable key %q not found in EXPERIENCE.md's Voice and Tone table", key)
	}
	return match[1]
}

// Bind is what makes the App callable at all. Emptying it ships a binary with
// zero commands while go build, go vet, go test and wails build all pass --
// the same argument the file already makes for EnableFileDrop, and it became
// load-bearing only when this story gave the App its first exported method.
func TestAppOptionsBindsTheApp(t *testing.T) {
	app := NewApp()
	opts := appOptionsWithLockProbe(app, func() bool { return true })

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

	stories := storyStatuses(t, sprint)

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
			case stories[owner] != "":
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

// deferredIDPattern matches the stable ids deferred-work.md assigns and
// epics.md cites.
var deferredIDPattern = regexp.MustCompile(`D-\d{3}`)

// Read each entry independently: an absent id must not inherit its neighbour's
// id or silently bypass the owning-story checks. Field order is not a contract.
type deferredEntry struct{ id, owner string }

func deferredEntries(t *testing.T, content []byte) []deferredEntry {
	t.Helper()
	blocks := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n- source_spec:")
	if len(blocks) < 2 {
		t.Fatal("no deferred entries parsed")
	}
	seen := map[string]bool{}
	var entries []deferredEntry
	for index, block := range blocks[1:] {
		var entry deferredEntry
		var ids, owners int
		for _, line := range strings.Split(block, "\n") {
			if value, ok := strings.CutPrefix(line, "  id:"); ok {
				entry.id = strings.TrimSpace(value)
				ids++
			}
			if value, ok := strings.CutPrefix(line, "  owner:"); ok {
				entry.owner = strings.TrimSpace(value)
				owners++
			}
		}
		if ids != 1 || len(entry.id) != 5 || deferredIDPattern.FindString(entry.id) != entry.id {
			t.Fatalf("deferred entry %d must have exactly one stable D-NNN id (got %q, count %d)", index+1, entry.id, ids)
		}
		if seen[entry.id] {
			t.Fatalf("duplicate deferred id %s", entry.id)
		}
		seen[entry.id] = true
		if owners != 1 || entry.owner == "" {
			t.Fatalf("%s must have exactly one non-empty owner", entry.id)
		}
		entries = append(entries, entry)
	}
	return entries
}

// splitLines normalises CRLF so a Windows checkout parses the same as a macOS
// one. The workflow's line-ending check keeps the repository LF, and this makes
// these tests independent of that check rather than quietly dependent on it.
func splitLines(content []byte) []string {
	return strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
}

// storyStatuses reads the `key: status` lines inside development_status, and
// only those: reading every indented line in the file would let an unrelated
// key satisfy an owner, and would leave the vacuity guards in both callers
// unable to fire when the block itself is renamed away.
func storyStatuses(t *testing.T, sprint []byte) map[string]string {
	t.Helper()

	statuses := map[string]string{}
	inBlock := false
	for _, line := range splitLines(sprint) {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(line, " ") && trimmed != "" {
			inBlock = trimmed == "development_status:"
			continue
		}
		if !inBlock || trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if found && key != "" {
			statuses[key] = strings.TrimSpace(value)
		}
	}
	if len(statuses) == 0 {
		t.Fatal("no story keys parsed from sprint-status.yaml, so the caller would pass vacuously")
	}
	return statuses
}

// TestEveryOpenDeferredEntryIsCitedByItsOwningStory closes the gap that
// TestEveryDeferredEntryHasALiveOwner leaves open.
//
// That test proves an entry names a story that exists. It cannot prove the
// story knows. A build session reads its story's acceptance criteria in
// epics.md, and the house rule is that the D-ids named there are in scope --
// so an entry owned by Story 3.4 whose id appears nowhere in Story 3.4's
// Closes line is invisible to the only session that would ever resolve it.
// Five entries were in exactly that state when this test was written, all
// added in the same session that added them to deferred-work.md and forgot
// epics.md. The failure mode is silence: nothing breaks, the work is simply
// never done.
//
// The second half is the same loss from the other end. An entry still open
// while the story that owns it is already done belongs to nobody, and no
// future session has a reason to look at it.
func TestEveryOpenDeferredEntryIsCitedByItsOwningStory(t *testing.T) {
	artifacts := filepath.Join("_bmad-output", "implementation-artifacts")

	deferred, err := os.ReadFile(filepath.Join(artifacts, "deferred-work.md"))
	if err != nil {
		t.Fatalf("read deferred-work.md: %v", err)
	}
	epics, err := os.ReadFile(filepath.Join("_bmad-output", "planning-artifacts", "epics.md"))
	if err != nil {
		t.Fatalf("read epics.md: %v", err)
	}
	sprint, err := os.ReadFile(filepath.Join(artifacts, "sprint-status.yaml"))
	if err != nil {
		t.Fatalf("read sprint-status.yaml: %v", err)
	}

	status := storyStatuses(t, sprint)

	// Story {epic}-{number} -> the ids its Closes line names. Keyed by the
	// numeric prefix because that is all a story key and a story heading share.
	cited := map[string]map[string]bool{}
	var heading string
	for _, line := range splitLines(epics) {
		if rest, found := strings.CutPrefix(line, "### Story "); found {
			number, _, ok := strings.Cut(rest, ":")
			if ok {
				heading = strings.ReplaceAll(strings.TrimSpace(number), ".", "-")
			}
			continue
		}
		if heading == "" || !strings.HasPrefix(line, "**Closes:**") {
			continue
		}
		if cited[heading] == nil {
			cited[heading] = map[string]bool{}
		}
		for _, id := range deferredIDPattern.FindAllString(line, -1) {
			cited[heading][id] = true
		}
	}
	if len(cited) == 0 {
		t.Fatal("no Closes lines parsed from epics.md, so this test would pass vacuously")
	}

	entries := deferredEntries(t, deferred)
	if len(entries) == 0 {
		t.Fatal("no deferred entries parsed, so this test would pass vacuously")
	}

	// Zero *open* entries is a legitimate state -- Story 4.1 discharged the
	// last two (D-113, D-114) and left every remaining entry discharged or
	// accepted -- so the vacuity guard above counts every entry the parser
	// found, not merely the open ones: a broken deferredEntries parse is what
	// this guards against, not a backlog that happens to be empty right now.
	for _, entry := range entries {
		id, owner := entry.id, entry.owner
		if owner == "discharged" || owner == "accepted" {
			continue
		}

		prefix := storyPrefix(owner)
		if !cited[prefix][id] {
			t.Errorf("%s is owned by %q, but Story %s's Closes line in epics.md does not name it: "+
				"the session that builds that story reads its acceptance criteria and would never "+
				"learn this entry exists", id, owner, strings.ReplaceAll(prefix, "-", "."))
		}
		if status[owner] == "done" {
			t.Errorf("%s is still open but its owner %q is already done: "+
				"the finding belongs to nobody", id, owner)
		}
	}
}

// storyPrefix reduces a sprint-status story key to the {epic}-{number} pair it
// shares with an epics.md heading: 3-4-bound-every-... becomes 3-4.
func storyPrefix(key string) string {
	parts := strings.SplitN(key, "-", 3)
	if len(parts) < 2 {
		return key
	}
	return parts[0] + "-" + parts[1]
}

// TestComposeWiresTheDiagnosticSeam pins the half of D-098 that lives outside
// internal/transfer.
//
// The coordinator calls its Diagnose seam on every recorded diagnostic, and
// the coordinator's own tests wire a fake one, so that side is well covered.
// None of that says compose passes anything: with the field omitted the seam
// defaults to a no-op, every test in every package stays green, and a shipped
// FairDrop goes back to recording diagnostics nowhere -- which is the exact
// state this story exists to end. The same two-halves shape as Story 3.3's
// workflow_call: one side asked, nothing pinned the other side offering.
//
// Read as text rather than by constructing a coordinator because the seam is
// not readable back off the built value, and a test that reconstructs
// Dependencies would be pinning its own literal instead of main.go's.
func TestComposeWiresTheDiagnosticSeam(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}

	compose, _, found := strings.Cut(string(source), "app.useCoordinator(coordinator)")
	if !found {
		t.Fatal("main.go no longer calls app.useCoordinator, so this test would pass vacuously")
	}
	if !strings.Contains(compose, "Diagnose: app.logDiagnostic") {
		t.Error("compose does not pass Diagnose: app.logDiagnostic, so every internal diagnostic " +
			"a shipped binary records reaches nothing a person can read")
	}
}

// TestTheReleaseRecordCannotClaimAnUnevidencedPass is the one mechanical guard
// on a file that is otherwise prose.
//
// `release-evidence.md` is where this project says what it actually verified,
// and its whole value is that a reader can trust the word "pass". Two ways that
// erodes, both cheap to check and neither caught by any other gate:
//
// A row that reads "pending" says someone owes the check. Under
// docs/release-policy.md nobody does -- manual observation is optional -- so
// "pending" is a promise the project has stopped making, and a row carrying it
// invites a later editor to discharge it by writing "pass".
//
// And a pass with nothing behind it is the failure mode every one of this
// repo's own lessons points at: a green suite has hidden a defect here more
// than once, and `gh run watch --exit-status` once exited 0 over a failed run.
// So a claimed pass must carry either a named reviewer, in a table that has
// that column, or a workflow run this file cites by id.
func TestTheReleaseRecordCannotClaimAnUnevidencedPass(t *testing.T) {
	path := filepath.Join("_bmad-output", "implementation-artifacts", "release-evidence.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	document := string(content)

	if !strings.Contains(document, "actions/runs/") {
		t.Error("the release record cites no workflow run at all, so nothing in it can be re-checked at its source")
	}

	var header []string
	rows, passes := 0, 0
	for _, line := range splitLines(content) {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			header = nil
			continue
		}
		cells := markdownCells(trimmed)
		if header == nil {
			header = cells
			continue
		}
		// The `|---|` separator under every header.
		if strings.HasPrefix(strings.TrimLeft(cells[0], " "), "---") || strings.Contains(cells[0], "---:") {
			continue
		}
		rows++

		result := strings.ToLower(cellNamed(header, cells, "Result"))
		if result == "" {
			continue
		}
		if strings.Contains(result, "pending") {
			t.Errorf("a row's result reads %q: the policy replaced pending with optional / unverified, "+
				"because nobody owes an optional check", result)
		}
		if !strings.Contains(result, "pass") && !strings.Contains(result, "success") {
			continue
		}
		passes++

		// Which evidence a pass owes depends on who could have produced it.
		// A table with a Reviewer column records what a person observed, so
		// the person is the evidence; the machine table has no such column,
		// and its evidence is the run. Accepting either everywhere would let
		// the word "run" appearing anywhere in a prose cell discharge a
		// manual row -- which is the shape of the mistake this test exists
		// to stop, not a spelling of it.
		if columnIndex(header, "Reviewer") >= 0 {
			reviewer := strings.TrimSpace(cellNamed(header, cells, "Reviewer"))
			if reviewer == "" || reviewer == "-" || reviewer == "—" {
				t.Errorf("a row claims %q without naming who observed it: %s", result, trimmed)
			}
			continue
		}
		joined := strings.ToLower(strings.Join(cells, " "))
		if !strings.Contains(joined, "actions/runs/") && !strings.Contains(joined, "the same run") {
			t.Errorf("a row claims %q without citing the run that produced it: %s", result, trimmed)
		}
	}

	// Vacuity: a file whose tables stopped parsing would satisfy every rule
	// above by having nothing to check.
	if rows < 15 {
		t.Errorf("parsed %d table rows, want the record's own rows: the parser or the file shape changed", rows)
	}
	if passes == 0 {
		t.Error("parsed no claimed pass at all, so this test proved nothing about how a pass is evidenced")
	}
}

// markdownCells splits one table row into its cells, dropping the empty
// fields the leading and trailing pipes produce.
func markdownCells(row string) []string {
	parts := strings.Split(strings.Trim(row, "|"), "|")
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, strings.TrimSpace(part))
	}
	return cells
}

// cellNamed returns the cell under a header, or "" when this table has no such
// column -- which is how one pass of the file covers tables of different shapes.
func cellNamed(header, cells []string, name string) string {
	index := columnIndex(header, name)
	if index < 0 || index >= len(cells) {
		return ""
	}
	return cells[index]
}

// columnIndex distinguishes a column this table does not have from one whose
// cell is empty. The two owe different evidence, so the test cannot conflate
// them the way a bare lookup would.
func columnIndex(header []string, name string) int {
	for index, column := range header {
		if strings.EqualFold(strings.TrimSpace(column), name) {
			return index
		}
	}
	return -1
}

// TestTheInstanceLockIsExclusiveWithinThisUser drives the backstop against
// itself: the second acquisition of the same file must fail while the first
// still holds it, and succeed once it is released.
//
// This is the whole of D-088's guarantee that a test can reach. Wails' two
// Windows fallthroughs need an elevated process or a scheduling race to
// produce, and neither is something a unit test should try to stage; what it
// can prove is that when a second process does get past Wails, this lock
// refuses it.
func TestTheInstanceLockIsExclusiveWithinThisUser(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, instanceLockName)

	first, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open the lock file: %v", err)
	}
	if !lockFileExclusive(first) {
		t.Fatal("the first holder could not take an uncontended lock")
	}

	second, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("reopen the lock file: %v", err)
	}
	if lockFileExclusive(second) {
		t.Error("a second holder took the lock while the first still held it: a competing coordinator would start")
	}
	if err := second.Close(); err != nil {
		t.Errorf("close the second descriptor: %v", err)
	}

	// Released by closing the descriptor, which is also what a crashed process
	// gets from the operating system for free -- the reason this is an advisory
	// lock rather than a file whose existence means "running".
	if err := first.Close(); err != nil {
		t.Fatalf("close the first descriptor: %v", err)
	}

	third, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("reopen the lock file after release: %v", err)
	}
	t.Cleanup(func() { _ = third.Close() })
	if !lockFileExclusive(third) {
		t.Error("the lock was not released when its holder closed: every later launch would be refused")
	}
}

// TestAnUnheldInstanceLockLeavesTheAppUncomposed pins what the backstop is for.
// An App without a coordinator refuses every command, which is exactly the
// outcome D-088 wants for a process Wails failed to stop: no listener, no
// beacon, nothing competing for a port.
func TestAnUnheldInstanceLockLeavesTheAppUncomposed(t *testing.T) {
	app := buildApp(false)

	app.mu.RLock()
	installed := app.transfers
	app.mu.RUnlock()

	if installed != nil {
		t.Error("a process that lost the instance lock still composed a coordinator, so it would start a " +
			"second listener and a second beacon")
	}

	if got := buildApp(true); got == nil {
		t.Fatal("holding the lock produced no App at all")
	}
}

// TestAWiringPanicReachesTheUserBeforeItKillsTheProcess is D-107.
//
// The dialog is a seam so this can drive it; the panic continuing is asserted
// because swallowing it would leave a half-composed process running, and the
// runtime's stack trace is what a developer needs even though a release build
// has no console to print it to.
func TestAWiringPanicReachesTheUserBeforeItKillsTheProcess(t *testing.T) {
	var shown []string
	continued := false

	func() {
		defer func() {
			if recover() != nil {
				continued = true
			}
		}()
		defer reportWiringPanic(func(message string) { shown = append(shown, message) })
		panic("transfer: dependencies are incomplete: " + testPath)
	}()

	if !continued {
		t.Error("the panic was swallowed: a half-composed FairDrop would keep running")
	}
	if len(shown) != 1 {
		t.Fatalf("showed %v, want exactly one message", shown)
	}
	if shown[0] != fatalWiringMessage {
		t.Errorf("showed %q, want the fixed message", shown[0])
	}
	// AD-9: the panic carried a path, and nothing the user sees may.
	if strings.Contains(shown[0], testPath) {
		t.Errorf("the dialog disclosed what the panic carried: %q", shown[0])
	}
}

// TestNoWiringPanicShowsNothing is the other half: the deferred reporter runs
// on every composition, and a healthy one must be silent.
func TestNoWiringPanicShowsNothing(t *testing.T) {
	shown := 0

	func() {
		defer reportWiringPanic(func(string) { shown++ })
	}()

	if shown != 0 {
		t.Errorf("showed %d dialogs with nothing wrong", shown)
	}
}

/*
TestEveryPreWindowSubprocessIsBounded reads the source on every platform,
because the defect it pins is reachable on one and fixable on none of the
others' CI.

nativeOSPrefersDarkTheme is per-platform and build-tagged: the darwin
implementation shells out, the windows one reads a registry key, and only the
first can hang. So the behavioural test for it compiles on macOS alone, and a
Windows or Linux runner that reintroduced a bare exec.Command there would
report green. This does not: it parses the file as text, which every runner
can do for every platform's implementation.

What it refuses is the shape, not a particular call. Anything before wails.Run
that waits on a process without a deadline puts FairDrop back where the Epic 3
retrospective found it (B8) -- no window, no fatal dialog, no log line, and
nothing D-107 built able to reach it, because none of that machinery is
running yet.
*/
func TestEveryPreWindowSubprocessIsBounded(t *testing.T) {
	// Every file that runs before wails.Run and may reach outside the process.
	// A new one belongs here; that is the point of naming them rather than
	// globbing, so adding an unbounded reader is a deliberate act.
	for _, name := range []string{"theme_darwin.go", "theme_windows.go", "theme_other.go"} {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Errorf("read %s: %v -- a pre-window reader this test cannot see is one it cannot pin", name, err)
			continue
		}
		text := string(source)

		if !strings.Contains(text, "exec.") {
			continue // no subprocess at all: nothing here can hang on one.
		}
		if strings.Contains(text, "exec.Command(") {
			t.Errorf("%s calls exec.Command without a context: a subprocess that never returns blocks "+
				"main() before wails.Run, so FairDrop shows no window, no fatal dialog and no log line",
				name)
		}
		if !strings.Contains(text, "exec.CommandContext(") {
			t.Errorf("%s reaches exec without CommandContext, so nothing bounds the call", name)
		}
		if !strings.Contains(text, "context.WithTimeout(") {
			t.Errorf("%s passes a context to its subprocess but never gives one a deadline, which bounds "+
				"nothing: cancellation has to come from somewhere", name)
		}
	}
}

/*
TestEveryMutationTheScriptNamesPointsAtATestThatExists closes a failure this
repository keeps repeating.

scripts/verify-native-mutations.sh drives its proof by name: `baseline` runs a
test to establish it passes unmutated, and `expect_named_failure` requires a
specific test to fail once a mutation is applied. Both take a test name and a
package, and neither is checked by the compiler. Rename or delete a test and
the script does not fail where you can see it -- it fails on a runner, with
`no tests to run`, after the change has already been pushed.

It has landed three times now: once when five tests were renamed, once when a
test the script named was deleted with the feature it covered, and once when a
reverted change took its test with it and left the baseline line behind. The
script also dies at the first problem, so a second stale name stays invisible
until a later run trips on it -- which is why this checks every reference
rather than stopping at one.

Names only. Whether each mutation's regex still matches its target is the other
half and is genuinely harder to check portably; this covers the half that has
actually broken, and it runs in milliseconds where CI takes minutes.
*/
func TestEveryMutationTheScriptNamesPointsAtATestThatExists(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("scripts", "verify-native-mutations.sh"))
	if err != nil {
		t.Fatalf("read the mutation script: %v", err)
	}

	baseline := regexp.MustCompile(`(?m)^baseline\s+(\S+)\s+(\S+)`)
	expect := regexp.MustCompile(`(?m)^expect_named_failure\s+'[^']*'\s+(\S+)\s+(\S+)`)

	type reference struct{ test, pkg string }
	var refs []reference
	for _, match := range baseline.FindAllStringSubmatch(string(source), -1) {
		refs = append(refs, reference{match[1], match[2]})
	}
	for _, match := range expect.FindAllStringSubmatch(string(source), -1) {
		refs = append(refs, reference{match[1], match[2]})
	}

	// Vacuity: a script whose shape changed would otherwise satisfy this test
	// by parsing to nothing at all.
	if len(refs) < 50 {
		t.Fatalf("parsed %d test references, want the script's own count: the parser or the script "+
			"shape changed, and this test can only pin references it can find", len(refs))
	}

	declared := regexp.MustCompile(`(?m)^func (Test\w+)\(`)
	tests := map[string]map[string]bool{}
	for _, pkg := range []string{".", "./internal/transfer", "./internal/server", "./internal/source",
		"./internal/stream", "./internal/network", "./internal/qr"} {
		names := map[string]bool{}
		entries, err := os.ReadDir(filepath.Clean(pkg))
		if err != nil {
			t.Fatalf("read %s: %v", pkg, err)
		}
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			body, err := os.ReadFile(filepath.Join(filepath.Clean(pkg), entry.Name()))
			if err != nil {
				t.Fatalf("read %s: %v", entry.Name(), err)
			}
			for _, match := range declared.FindAllStringSubmatch(string(body), -1) {
				names[match[1]] = true
			}
		}
		tests[pkg] = names
	}

	for _, ref := range refs {
		names, known := tests[ref.pkg]
		if !known {
			t.Errorf("the mutation script names package %q, which this test does not scan: a reference "+
				"it cannot check is one it cannot pin", ref.pkg)
			continue
		}
		if !names[ref.test] {
			t.Errorf("the mutation script names %s in %s and no such test exists: the native proof would "+
				"fail on a runner with \"no tests to run\" rather than here", ref.test, ref.pkg)
		}
	}
}
