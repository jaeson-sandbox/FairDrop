package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

func TestSelectionResolutionHonoursAdmissionAndCancellation(t *testing.T) {
	for _, ending := range []string{"caller", "cancel", "shutdown"} {
		t.Run(ending, func(t *testing.T) {
			app, inspector := nativeMatrixApp(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			app.startup(ctx)
			entered, release := make(chan struct{}), make(chan struct{})
			unblock := sync.OnceFunc(func() { close(release) })
			defer unblock()
			enteredOnce := sync.OnceFunc(func() { close(entered) })
			inspector.selection.resolve = func(path string) string {
				enteredOnce()
				<-release
				return path
			}
			staged := make(chan error, 1)
			go func() { _, err := app.StageTransfer("private selection"); staged <- err }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("resolver was not entered")
			}
			if _, err := app.StageTransfer("another selection"); transfer.ErrorCodeOf(err) != transfer.ErrBusy {
				t.Fatal("busy admission reached filesystem resolution")
			}
			switch ending {
			case "caller":
				cancel()
			case "cancel":
				if err := app.CancelTransfer(); err != nil {
					t.Fatal("Cancel did not release blocked selection")
				}
			case "shutdown":
				if err := app.transfers.Shutdown(context.Background()); err != nil {
					t.Fatal("Shutdown did not release blocked selection")
				}
			}
			select {
			case err := <-staged:
				if err == nil {
					t.Fatal("abandoned selection committed")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("selection waited for the blocked filesystem call")
			}
			app.startup(context.Background())
			want := transfer.ErrBusy
			if ending == "shutdown" {
				want = transfer.ErrShuttingDown
			}
			for range 3 {
				retry := make(chan error, 1)
				go func() { _, err := app.StageTransfer("retry"); retry <- err }()
				select {
				case err := <-retry:
					if transfer.ErrorCodeOf(err) != want {
						t.Fatal("retry admitted additional unresolved filesystem work")
					}
				case <-time.After(time.Second):
					t.Fatal("retry admitted additional unresolved filesystem work")
				}
			}
			if inspector.calls.Load() != 0 || inspector.networkCalls.Load() != 0 {
				t.Fatal("abandoned resolver reached Inspect or network")
			}
			unblock()
			deadline := time.After(5 * time.Second)
			for inspector.selection.resolving.Load() {
				select {
				case <-deadline:
					t.Fatal("returned resolver retained busy ownership")
				default:
					runtime.Gosched()
				}
			}
			if inspector.calls.Load() != 0 || inspector.networkCalls.Load() != 0 {
				t.Fatal("late resolver performed Inspect or network work")
			}
		})
	}
}

func TestSelectionResolutionCancelledBeforeEntryDoesNoFilesystemWork(t *testing.T) {
	source := newSelectionSource(nil)
	source.resolve = func(string) string { t.Error("cancelled context reached resolver"); return "" }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := source.Inspect(ctx, "private selection"); transfer.ErrorCodeOf(err) != transfer.ErrCancelled {
		t.Fatal("pre-cancelled resolution was not refused")
	}
}

func TestSelectionResolutionRetryAfterFilesystemReturns(t *testing.T) {
	app, inspector := nativeMatrixApp(t)
	file := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(file, []byte("native matrix payload"), 0o600); err != nil {
		t.Fatal("fixture write failed")
	}
	entered, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	defer unblock()
	inspector.selection.resolve = func(path string) string {
		close(entered)
		<-release
		return resolveSelectionAncestors(path)
	}
	staged := make(chan error, 1)
	go func() { _, err := app.StageTransfer(file); staged <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("abandoned resolver never entered")
	}
	if err := app.CancelTransfer(); err != nil {
		t.Fatal("blocked selection cancellation failed")
	}
	select {
	case err := <-staged:
		if err == nil {
			t.Fatal("abandoned selection staged")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled Stage waited for filesystem")
	}
	if _, err := app.StageTransfer(file); transfer.ErrorCodeOf(err) != transfer.ErrBusy {
		t.Fatal("retry did not remain busy while resolver outstanding")
	}
	unblock()
	deadline := time.After(5 * time.Second)
	for inspector.selection.resolving.Load() {
		select {
		case <-deadline:
			t.Fatal("returned resolver retained ownership")
		default:
			runtime.Gosched()
		}
	}
	if inspector.calls.Load() != 0 || inspector.networkCalls.Load() != 0 {
		t.Fatal("abandoned resolution reached Inspect or network")
	}
	// The worker has returned; a fresh Stage must now traverse the production
	// resolver and complete a real HTTP transfer on this same coordinator.
	inspector.selection.resolve = resolveSelectionAncestors
	// Ignore lifecycle events belonging to the abandoned attempt.
	for len(inspector.events) != 0 {
		<-inspector.events
	}
	assertNativeDownloadWithApp(t, app, inspector, file, false)
	// Stage inspects once; the real payload preparation revalidates once.
	if inspector.calls.Load() != 2 || inspector.networkCalls.Load() != 1 {
		t.Fatal("fresh selection and real download did not inspect and revalidate exactly once")
	}
}

func TestSelectionResolutionCancellationImmediatelyBeforeResult(t *testing.T) {
	_, inspector := nativeMatrixApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancel()
	// Drive the exact production result-acceptance gate with a real cancelled
	// context. This deterministically tests that gate, not select's random arm.
	_, err := inspector.selection.inspectResolved(ctx, "private selection")
	if transfer.ErrorCodeOf(err) != transfer.ErrCancelled || inspector.calls.Load() != 0 || inspector.networkCalls.Load() != 0 {
		t.Fatal("cancelled result reached raw Inspect or network instead of cancelled refusal")
	}
	data, err := os.ReadFile("selection_source.go")
	if err != nil {
		t.Fatal("selection result wiring source unavailable")
	}
	if !strings.Contains(strings.ReplaceAll(string(data), "\r\n", "\n"), "case canonical := <-result:\n\t\treturn s.inspectResolved(ctx, canonical)") {
		t.Fatal("production result arm bypasses cancellation acceptance gate")
	}
}

func TestSelectionResolutionCancellationAtResolverReturn(t *testing.T) {
	_, inspector := nativeMatrixApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	inspector.selection.resolve = func(path string) string { cancel(); return path }
	_, err := inspector.selection.Inspect(ctx, "private selection")
	if transfer.ErrorCodeOf(err) != transfer.ErrCancelled || inspector.calls.Load() != 0 || inspector.networkCalls.Load() != 0 {
		t.Fatal("cancelled result reached raw Inspect or network instead of cancelled refusal")
	}
}

/*
TestAncestorResolutionKeepsTheSelectionWhenEvalFails pins what an unreadable
ancestor leaves behind.

resolveAncestorsWith exists to spell a selection's ancestors through their real
directories -- macOS reaches its temporary directory through /var, a symlink --
and it must not invent a path when it cannot. On an eval failure it returns the
selection exactly as the picker gave it, so the source layer below refuses it
with its own coded error rather than this decorator refusing something the user
never chose.

Every other test here stubs the outer resolve field or supplies a filesystem
where eval succeeds, so this branch was never executed: the Epic 3
retrospective replaced it with a path built from a zero-value parent and the
whole repository stayed green (B5). The window is real and the function's own
comment names it -- an ancestor can be removed or become unreadable between the
picker returning and Stage resolving.
*/
func TestAncestorResolutionKeepsTheSelectionWhenEvalFails(t *testing.T) {
	t.Parallel()

	selected := filepath.Join(absoluteRoot()+"Users", "someone", "Documents", "Travel Notes.pdf")

	calls := 0
	got := resolveAncestorsWith(selected, func(string) (string, error) {
		calls++
		return "", os.ErrNotExist
	})

	if calls != 1 {
		t.Fatalf("eval was called %d times, want 1 -- this test can only pin a branch it reaches", calls)
	}
	if got != selected {
		t.Errorf("an unreadable ancestor resolved to %q, want the selection unchanged (%q): a path built "+
			"from a parent that never resolved is a path the user did not choose", got, selected)
	}
}

/*
TestAncestorResolutionSpellsTheAncestorItResolved is the other half, and the
reason the test above cannot stand alone: a resolveAncestorsWith that ignored
eval entirely and always returned its argument would satisfy it.
*/
func TestAncestorResolutionSpellsTheAncestorItResolved(t *testing.T) {
	t.Parallel()

	leaf := "Travel Notes.pdf"
	parent := filepath.Join(absoluteRoot()+"real", "documents")
	selected := filepath.Join(parent+"-link", leaf)

	got := resolveAncestorsWith(selected, func(ancestor string) (string, error) {
		if !strings.HasSuffix(strings.TrimRight(ancestor, string(os.PathSeparator)), "documents-link") {
			t.Errorf("eval was asked for %q, want the selection's ancestor", ancestor)
		}
		return parent, nil
	})

	if want := filepath.Join(parent, leaf); got != want {
		t.Errorf("resolved to %q, want %q -- the leaf must be rejoined onto the resolved ancestor", got, want)
	}
}

// absoluteRoot is the prefix that makes a joined test path absolute on this
// platform. filepath.Join("C:", "Users") produces the drive-RELATIVE "C:Users",
// which resolveAncestorsWith correctly declines to treat as an ancestor -- so
// a test that used it would silently never reach the branch it names.
func absoluteRoot() string {
	if runtime.GOOS == "windows" {
		return `C:\`
	}
	return "/"
}
