package main

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
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

// Exactly one FairDrop process may run: a second launch must recognize the
// first rather than starting a competing coordinator, listener and beacon.
// The UniqueId is spelled out as a literal, not a reference to the constant
// under test, so a build that quietly changed the UUID fails here rather than
// only inside main.go, and OnSecondInstanceLaunch is asserted present so a
// second launch always has somewhere to hand its window off to.
func TestAppOptionsEnforcesSingleInstance(t *testing.T) {
	opts := appOptions(NewApp())

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

// The non-nil check above is satisfied by any callback, including a stub
// func(options.SecondInstanceData) {} that restores nothing -- appOptions
// could swap in one and every test would stay green. This drives the callback
// appOptions actually wires, through the options value itself, so only the
// real restoreWindow -- with the real fake seams a harness installed on this
// App -- can pass.
func TestAppOptionsSecondInstanceCallbackRestoresTheWindow(t *testing.T) {
	h := newHarness(t)
	opts := appOptions(h.app)

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
	opts := appOptions(h.app)

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

	const want = `{"code":"busy","message":"FairDrop is still finishing the last transfer. Wait a moment, or cancel it, then choose another item."}`
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
	{"busy", "FairDrop is still finishing the last transfer. Wait a moment, or cancel it, then choose another item."},
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
	{"shutting_down", "FairDrop is closing. Reopen it to start a transfer."},
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
			wantJSON := `{"code":"` + entry.code + `","message":"` + entry.message + `"}`
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

	var open int
	var id string
	for _, line := range splitLines(deferred) {
		if rest, found := strings.CutPrefix(line, "  id:"); found {
			id = strings.TrimSpace(rest)
			continue
		}
		rest, found := strings.CutPrefix(line, "  owner:")
		if !found {
			continue
		}
		owner := strings.TrimSpace(rest)
		if id == "" || owner == "discharged" || owner == "accepted" {
			id = ""
			continue
		}
		open++

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
		id = ""
	}

	if open == 0 {
		t.Fatal("no open deferred entries parsed, so this test would pass vacuously")
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
