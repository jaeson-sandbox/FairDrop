package transfer

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestPublicErrorOfExactRegistryCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code    ErrorCode
		message string
	}{
		{ErrInvalidSelection, "Choose exactly one file or folder."},
		{ErrBusy, "FairDrop is still finishing the last item. If it doesn’t finish, close FairDrop and reopen it."},
		{ErrCancelled, "Transfer canceled."},
		{ErrPathNotFound, "That file or folder is no longer available. Choose it again."},
		{ErrPathUnsupported, "FairDrop can use regular files and folders only. Choose another item."},
		{ErrSourceChanged, "The item changed after it was prepared. Cancel and create a fresh link."},
		{ErrNetworkUnavailable, "FairDrop couldn’t find a usable local network. Connect to local Wi-Fi, then try again."},
		{ErrServerStartFailed, "FairDrop couldn’t open a local transfer connection. Check firewall access, then try again."},
		{ErrQRFailed, "FairDrop couldn’t create the QR code. Prepare the item again."},
		{ErrSetupFailed, "FairDrop couldn’t prepare that item. Nothing was sent. Choose it again."},
		{ErrBeaconWarning, "Device discovery isn’t available. The QR code and download link still work."},
		{ErrTransferFailed, "The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link."},
		{ErrCleanupUnconfirmed, "FairDrop couldn’t confirm it released the connection. Nothing was sent. Close FairDrop and reopen it before sending again."},
		{ErrNotReady, "FairDrop isn’t ready to send. Another copy may already be running. Close this window and use that one."},
		{ErrClipboardFailed, "FairDrop couldn’t copy the link. Select the link and copy it yourself."},
		{ErrNameUnsupported, "One name inside that folder can’t be sent safely. Rename it, then choose the folder again."},
		{ErrNameWarning, "Some names in this folder can’t be saved on Windows — usually a colon, an asterisk, or a trailing dot or space. They’re sent unchanged; a Windows receiver may not be able to extract those items."},
		{ErrShuttingDown, "FairDrop is closing. Reopen it to start a transfer."},
		{ErrChooserFailed, "FairDrop couldn’t open the chooser. Try again, or drag the item onto the window."},
	}

	for _, test := range tests {
		test := test
		t.Run(string(test.code), func(t *testing.T) {
			t.Parallel()
			got := PublicErrorOf(NewError(test.code, "adapter detail that must be ignored"))
			if got.Code != test.code || got.Message != test.message {
				t.Fatalf("PublicErrorOf() = %#v, want code %q and message %q", got, test.code, test.message)
			}
		})
	}
}

func TestDomainErrorWrappingPreservesCodeAndCause(t *testing.T) {
	t.Parallel()

	cause := errors.New("filesystem failure")
	domainErr := WrapError(ErrPathUnsupported, "metadata unavailable", cause)
	err := fmt.Errorf("outer one: %w", fmt.Errorf("outer two: %w", domainErr))

	if got := ErrorCodeOf(err); got != ErrPathUnsupported {
		t.Fatalf("ErrorCodeOf() = %q, want %q", got, ErrPathUnsupported)
	}
	if !errors.Is(err, cause) {
		t.Fatal("wrapped cause is not reachable with errors.Is")
	}
	var target *DomainError
	if !errors.As(err, &target) {
		t.Fatal("DomainError is not reachable with errors.As")
	}
	if target.Code() != ErrPathUnsupported {
		t.Fatalf("DomainError.Code() = %q, want %q", target.Code(), ErrPathUnsupported)
	}
}

func TestPublicErrorOfMultiplyWrappedDomainErrorIsExactAndSafe(t *testing.T) {
	t.Parallel()

	const secret = `C:\private\payroll.txt?token=secret`
	err := fmt.Errorf("outer: %w", fmt.Errorf("middle: %w", WrapError(
		ErrPathNotFound,
		"selection does not exist",
		errors.New(secret),
	)))
	want := PublicError{
		Code:    ErrPathNotFound,
		Message: "That file or folder is no longer available. Choose it again.",
	}
	got := PublicErrorOf(err)
	if got != want {
		t.Fatalf("PublicErrorOf() = %#v, want %#v", got, want)
	}
	if strings.Contains(got.Message, secret) || strings.Contains(got.Message, "payroll.txt") {
		t.Fatalf("PublicErrorOf() leaked wrapped detail: %#v", got)
	}
}

func TestIndependentCodedErrorSurvivesWrapping(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("outer: %w", independentCodedError{code: ErrBusy})
	if got := ErrorCodeOf(err); got != ErrBusy {
		t.Fatalf("ErrorCodeOf() = %q, want %q", got, ErrBusy)
	}
	want := PublicError{
		Code:    ErrBusy,
		Message: "FairDrop is still finishing the last item. If it doesn’t finish, close FairDrop and reopen it.",
	}
	if got := PublicErrorOf(err); got != want {
		t.Fatalf("PublicErrorOf() = %#v, want %#v", got, want)
	}
}

func TestUnknownErrorsUseFixedFallback(t *testing.T) {
	t.Parallel()

	err := errors.New("unknown adapter detail")
	if got := ErrorCodeOf(err); got != ErrTransferFailed {
		t.Fatalf("ErrorCodeOf() = %q, want %q", got, ErrTransferFailed)
	}
	got := PublicErrorOf(err)
	want := PublicError{
		Code:    ErrTransferFailed,
		Message: "The transfer stopped before FairDrop finished sending. Check the local network and create a fresh link.",
	}
	if got != want {
		t.Fatalf("PublicErrorOf() = %#v, want %#v", got, want)
	}
}

func TestUnknownCodedErrorUsesPublicFallback(t *testing.T) {
	t.Parallel()

	err := NewError(ErrorCode("not_registered"), "unknown detail")
	if got := ErrorCodeOf(err); got != ErrTransferFailed {
		t.Fatalf("ErrorCodeOf() = %q, want transfer_failed fallback", got)
	}
	got := PublicErrorOf(err)
	if got.Code != ErrTransferFailed || got.Message != publicMessages[ErrTransferFailed] {
		t.Fatalf("PublicErrorOf() = %#v, want transfer_failed fallback", got)
	}
}

func TestNilErrorsAreSafe(t *testing.T) {
	t.Parallel()

	if got := ErrorCodeOf(nil); got != "" {
		t.Fatalf("ErrorCodeOf(nil) = %q, want empty", got)
	}
	if got := PublicErrorOf(nil); got != (PublicError{}) {
		t.Fatalf("PublicErrorOf(nil) = %#v, want zero value", got)
	}

	var domain *DomainError
	var typedNil error = domain
	if got := ErrorCodeOf(typedNil); got != ErrTransferFailed {
		t.Fatalf("ErrorCodeOf(typed nil) = %q, want %q", got, ErrTransferFailed)
	}
	if got := PublicErrorOf(typedNil); got.Code != ErrTransferFailed {
		t.Fatalf("PublicErrorOf(typed nil) = %#v, want transfer_failed", got)
	}
	if got := domain.Error(); got != string(ErrTransferFailed) {
		t.Fatalf("nil DomainError.Error() = %q, want %q", got, ErrTransferFailed)
	}
	if got := domain.Code(); got != ErrTransferFailed {
		t.Fatalf("nil DomainError.Code() = %q, want %q", got, ErrTransferFailed)
	}
	if cause := domain.Unwrap(); cause != nil {
		t.Fatalf("nil DomainError.Unwrap() = %v, want nil", cause)
	}
}

func TestDomainErrorStringDoesNotRenderWrappedSecrets(t *testing.T) {
	t.Parallel()

	const path = `C:\Users\sender\private report.txt`
	const token = "capability-token-that-must-not-leak"
	err := WrapError(
		ErrPathUnsupported,
		"selection metadata could not be read",
		fmt.Errorf("open %s with %s: denied", path, token),
	)

	for _, rendered := range []string{err.Error(), PublicErrorOf(err).Message} {
		if strings.Contains(rendered, path) || strings.Contains(rendered, token) {
			t.Fatalf("rendered error leaked path or token: %q", rendered)
		}
	}
	if got, want := err.Error(), "path_unsupported: selection metadata could not be read"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestNewErrorHasNoCause(t *testing.T) {
	t.Parallel()

	var domain *DomainError
	if !errors.As(NewError(ErrBusy, "already active"), &domain) {
		t.Fatal("NewError did not return a DomainError")
	}
	if domain.Unwrap() != nil {
		t.Fatalf("NewError cause = %v, want nil", domain.Unwrap())
	}
}

func TestPublicErrorJSONWireShape(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(PublicError{Code: ErrCancelled, Message: "Transfer canceled."})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"code":"cancelled","message":"Transfer canceled."}`; got != want {
		t.Fatalf("json.Marshal(PublicError) = %s, want %s", got, want)
	}
}

type independentCodedError struct{ code ErrorCode }

func (e independentCodedError) Error() string   { return "independent coded error detail" }
func (e independentCodedError) Code() ErrorCode { return e.code }

// The copy test above walks a hand-written list, so it catches a renamed or
// removed code and misses an added one entirely -- and an added code reaches
// the Wails boundary as unrecognized, degrading to transfer_failed with the
// wrong copy and no test failing anywhere. This pins the registry as a set.
func TestTheCodeRegistryIsExactlyThisSet(t *testing.T) {
	t.Parallel()

	want := map[ErrorCode]bool{
		ErrInvalidSelection:   true,
		ErrBusy:               true,
		ErrCancelled:          true,
		ErrPathNotFound:       true,
		ErrPathUnsupported:    true,
		ErrSourceChanged:      true,
		ErrNetworkUnavailable: true,
		ErrServerStartFailed:  true,
		ErrQRFailed:           true,
		ErrSetupFailed:        true,
		ErrBeaconWarning:      true,
		ErrTransferFailed:     true,
		ErrCleanupUnconfirmed: true,
		ErrNotReady:           true,
		ErrClipboardFailed:    true,
		ErrNameUnsupported:    true,
		ErrNameWarning:        true,
		ErrShuttingDown:       true,
		ErrChooserFailed:      true,
	}

	for code := range publicMessages {
		if !want[code] {
			t.Errorf("publicMessages gained %q. Add it to this test, to the copy test above, "+
				"to main_test.go's cross-language list, to frontend/src/transfer/errors.ts and its "+
				"own backendCodes list, to docs/fairdrop-contracts.md in both the prose table and the "+
				"Go constant block, and to EXPERIENCE.md -- otherwise it reaches the UI as an "+
				"unrecognized code.", code)
		}
	}
	for code := range want {
		if _, found := publicMessages[code]; !found {
			t.Errorf("publicMessages no longer defines %q", code)
		}
	}
}

// TestEveryDiagnosticMessageIsAFixedLiteral is where AD-9's diagnostic
// guarantee actually lives.
//
// app.go's logDiagnostic takes a string and prints it; nothing inside it could
// reliably recognise every shape an absolute path or a capability token can
// take, and a guarantee that depends on guessing is not one. What can be
// guaranteed is that no caller ever hands it anything but text this package
// wrote itself -- so this parses the package's own source and requires every
// recordDiagnostic call to pass a string literal as its message, never a
// variable, a format, or a concatenation that could carry an adapter's words.
//
// Parsed with go/ast rather than matched with a regular expression: these
// calls span lines and nest another call in their first argument, which a
// pattern gets wrong in exactly the direction that makes a test pass while
// reading nothing -- the first version of this test did.
func TestEveryDiagnosticMessageIsAFixedLiteral(t *testing.T) {
	t.Parallel()

	// ParseFile over a glob rather than ParseDir, which is deprecated because
	// it ignores build tags. Every file this package needs to check is
	// unconditional Go, so the glob is exact here and carries no dependency.
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob the package: %v", err)
	}

	set := token.NewFileSet()
	calls := 0
	for _, name := range sources {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(set, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		{
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "recordDiagnostic" {
					return true
				}
				calls++
				if len(call.Args) != 2 {
					t.Errorf("%s:%d: recordDiagnostic called with %d arguments, want 2",
						filepath.Base(name), set.Position(call.Pos()).Line, len(call.Args))
					return true
				}
				if literal, ok := call.Args[1].(*ast.BasicLit); !ok || literal.Kind != token.STRING {
					t.Errorf("%s:%d: the diagnostic message is not a string literal -- adapter text is "+
						"exactly where a path or a capability token would be (AD-9)",
						filepath.Base(name), set.Position(call.Pos()).Line)
				}
				return true
			})
		}
	}

	if len(sources) == 0 {
		t.Fatal("no package sources found, so this test would pass vacuously")
	}
	if calls == 0 {
		t.Fatal("no recordDiagnostic calls parsed, so this test would pass vacuously")
	}
}

/*
TestDrainAcceptsTerminalAtTheStatesEachCallSiteIsFor closes the half of
D-042/D-091 that had no guard, and pins the distinction the two call sites
exist to make.

drain accepts a terminal outcome in two places and they are deliberately not
the same. The in-loop arm forwards a real report from the server, which only
ever describes a claimed, in-flight transfer, so it stays gated to
TRANSFERRING: widening it would let a stray event end a session nobody claimed.
The post-loop synthesis is the opposite case -- the server left without
reporting anything -- so it must reach STAGED and CLAIMING too, or a session
keeps showing a QR code for a listener that no longer exists.

Three tests appear to cover this and none pins CLAIMING.
TestALaneThatClosesWhileStagedStillEndsTheSession drives only STAGED,
TestLaneClosureWithoutAnOutcomeSynthesizesAFailure only TRANSFERRING, and
TestTheDrainerMayEndASessionFromEveryStateOneCanBeIn declares its own
three-state list and hands it to the guard -- which proves the guard honours
what it is given, a different claim. The Epic 3 retrospective narrowed the
production call site to stateStaged, stateTransferring and the package stayed
green (B2).

CLAIMING cannot be driven deterministically, and that is not an oversight: it
exists only inside AuthorizeClaim, which holds the operation lease the
synthesis itself needs, so a behavioural test would race for it and usually
prove nothing. The claim here is structural -- *this call site names this
state* -- so it is checked structurally, the way
TestEveryBoundedCallArmsItsBoundBeforeLaunching below checks an ordering a
comment could not hold.
*/
func TestDrainAcceptsTerminalAtTheStatesEachCallSiteIsFor(t *testing.T) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "outcomes.go", nil, 0)
	if err != nil {
		t.Fatalf("parse outcomes.go: %v", err)
	}

	// The two call sites are told apart by what they pass as the event: the
	// in-loop arm forwards the one it received, the synthesis builds its own.
	// Nothing else distinguishes them, and a test that matched on order would
	// break the first time someone moved a line.
	var forwarded, synthesised []string
	sites := 0
	for _, declaration := range parsed.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Name.Name != "drain" || function.Body == nil {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			selector, isSelector := call.Fun.(*ast.SelectorExpr)
			if !isSelector || selector.Sel.Name != "acceptTerminal" || len(call.Args) < 2 {
				return true
			}
			sites++

			states := make([]string, 0, len(call.Args))
			for _, argument := range call.Args[2:] {
				if identifier, isIdentifier := argument.(*ast.Ident); isIdentifier {
					states = append(states, identifier.Name)
				}
			}
			if _, builtHere := call.Args[1].(*ast.CompositeLit); builtHere {
				synthesised = states
			} else {
				forwarded = states
			}
			return true
		})
	}

	// Vacuity: a renamed function, a moved call site or a changed signature
	// would otherwise satisfy every assertion below by finding nothing.
	if sites != 2 {
		t.Fatalf("found %d acceptTerminal call sites in drain, want 2 (the forwarded report and the "+
			"synthesised one) -- this test can only pin call sites it can find", sites)
	}

	// A real server report describes a transfer that was claimed and started.
	if !slices.Equal(forwarded, []string{"stateTransferring"}) {
		t.Errorf("drain forwards a real server report at %v, want exactly [stateTransferring]: a report "+
			"accepted at STAGED or CLAIMING would let a stray event end a session nobody claimed",
			forwarded)
	}

	// The synthesis is for a server that left without reporting anything.
	for _, state := range []string{"stateStaged", "stateClaiming", "stateTransferring"} {
		if !slices.Contains(synthesised, state) {
			t.Errorf("drain's synthesis does not name %s, so a session whose server dies in that state "+
				"gets no outcome at all: the window keeps a QR code for a listener that is gone "+
				"(D-042, D-091). Named: %v", state, synthesised)
		}
	}
}

// TestEveryBoundedCallArmsItsBoundBeforeLaunching pins an ordering that was
// written down, explained, and then quietly reversed one function later.
//
// callBounded arms its timer before it launches the call, and says why: arming
// afterwards races the spawned goroutine for which one reaches a test's call
// log first. Story 3.8's callAdapterBounded launched first and armed second,
// which reintroduced exactly that race -- and no local run found it. A Windows
// CI runner did, after the change had already merged, with
// TestAuthorizeClaimCommitsAndPublishesStarted reporting the two calls in the
// other order.
//
// A comment could not stop that and did not. This reads the source: in each of
// these functions the BoundTimer call must appear before the `go` statement.
// It is a structural claim, so it is checked structurally -- the behavioural
// alternative is a test that fails only on the scheduler's bad days, which is
// how this defect reached main in the first place.
//
// Both functions moved to bounded.go with the rest of the bounded-call
// subsystem (Epic 3 retrospective item 23), so this parses bounded.go rather
// than coordinator.go -- the file, not the ordering claim, is what moved.
func TestEveryBoundedCallArmsItsBoundBeforeLaunching(t *testing.T) {
	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, "bounded.go", nil, 0)
	if err != nil {
		t.Fatalf("parse bounded.go: %v", err)
	}

	checked := map[string]bool{"callBounded": false, "callAdapterBounded": false}
	for _, declaration := range parsed.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Body == nil {
			continue
		}
		if _, wanted := checked[function.Name.Name]; !wanted {
			continue
		}
		checked[function.Name.Name] = true

		armed, launched := token.NoPos, token.NoPos
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.CallExpr:
				selector, isSelector := typed.Fun.(*ast.SelectorExpr)
				if isSelector && selector.Sel.Name == "boundTimer" && armed == token.NoPos {
					armed = typed.Pos()
				}
			case *ast.GoStmt:
				if launched == token.NoPos {
					launched = typed.Pos()
				}
			}
			return true
		})

		if armed == token.NoPos {
			t.Errorf("%s arms no bound at all, so nothing here is bounded", function.Name.Name)
			continue
		}
		if launched == token.NoPos {
			t.Errorf("%s launches no call, so this test would pass vacuously for it", function.Name.Name)
			continue
		}
		if armed > launched {
			t.Errorf("%s launches its call at %s before arming its bound at %s: the call's position in a "+
				"test's log becomes a coin flip, and the call runs unbounded until the timer is armed",
				function.Name.Name, fileSet.Position(launched), fileSet.Position(armed))
		}
	}

	for name, found := range checked {
		if !found {
			t.Errorf("%s was not found in bounded.go, so this test silently stopped covering it", name)
		}
	}
}
