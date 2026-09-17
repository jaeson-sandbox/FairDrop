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
			for inspector.selection.resolving.Load() != 0 {
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
	for inspector.selection.resolving.Load() != 0 {
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

// stubSource lets TestSelectionResolutionBoundRecoversTheFlagWithoutCorruptingALaterCall
// complete a real Inspect call past the decorator without touching the
// filesystem; PrepareDirectory and Walk are never reached from that test.
type stubSource struct{}

func (stubSource) Inspect(context.Context, string) (transfer.StagedItem, error) {
	return transfer.StagedItem{}, nil
}
func (stubSource) PrepareDirectory(context.Context, string) (transfer.PreparedDirectory, error) {
	panic("stubSource.PrepareDirectory is not used by this test")
}
func (stubSource) Walk(context.Context, string, transfer.SourceVisitor) error {
	panic("stubSource.Walk is not used by this test")
}

// stubBoundTimer lets a test fire a selectionSource's bound deterministically
// instead of sleeping past the real one. It captures whatever run was armed
// most recently; a test calls fire to invoke it on demand.
type stubBoundTimer struct {
	mu      sync.Mutex
	run     func()
	stopped bool
}

func (b *stubBoundTimer) after(_ time.Duration, run func()) transfer.StopTimer {
	b.mu.Lock()
	b.run = run
	b.stopped = false
	b.mu.Unlock()
	return func() bool {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.stopped {
			return false
		}
		b.stopped = true
		return true
	}
}

// fire honours stop, because a real timer does. A stub that ran its callback
// after the code under test had cancelled it would report a flag recovered by
// a timer that, in production, was never going to fire -- which is exactly the
// case this file has to be able to tell apart.
func (b *stubBoundTimer) fire() {
	b.mu.Lock()
	run, stopped := b.run, b.stopped
	b.mu.Unlock()
	if stopped {
		return
	}
	run()
}

// TestSelectionResolutionBoundRecoversTheFlagWithoutCorruptingALaterCall
// drives the D-024-shaped fix directly: a resolver that never returns must
// not make every later Stage busy forever, and the worker abandoned by the
// bound must not corrupt a call that has since taken the flag when it
// eventually, harmlessly, finishes.
func TestSelectionResolutionBoundRecoversTheFlagWithoutCorruptingALaterCall(t *testing.T) {
	source := newSelectionSource(stubSource{})
	bound := &stubBoundTimer{}
	source.boundTimer = bound.after

	staleEntered, staleRelease := make(chan struct{}), make(chan struct{})
	staleEnteredOnce := sync.OnceFunc(func() { close(staleEntered) })
	source.resolve = func(path string) string {
		staleEnteredOnce()
		<-staleRelease
		return path
	}

	staleErr := make(chan error, 1)
	go func() {
		_, err := source.Inspect(context.Background(), "private stale selection")
		staleErr <- err
	}()
	select {
	case <-staleEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("stale resolver was not entered")
	}

	// Firing the bound must recover the flag immediately and report that the
	// call did not come back in time -- never success, never busy forever.
	bound.fire()
	select {
	case err := <-staleErr:
		if transfer.ErrorCodeOf(err) != transfer.ErrSetupFailed {
			t.Fatalf("bound-elapsed resolution returned code=%s, want setup_failed", transfer.ErrorCodeOf(err))
		}
		if strings.Contains(err.Error(), "stale selection") {
			t.Fatal("bound-elapsed diagnostic named the selection it was resolving (AD-9)")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("bound-elapsed call did not return")
	}
	if got := source.resolving.Load(); got != 0 {
		t.Fatalf("flag stayed held (gen=%d) after its bound elapsed", got)
	}

	// A fresh call must be admitted right away -- proving the flag was really
	// cleared, not merely that the stale worker happened to finish already:
	// the stale worker is still blocked on staleRelease at this point.
	freshEntered, freshRelease := make(chan struct{}), make(chan struct{})
	freshEnteredOnce := sync.OnceFunc(func() { close(freshEntered) })
	source.resolve = func(path string) string {
		freshEnteredOnce()
		<-freshRelease
		return path
	}
	freshErr := make(chan error, 1)
	go func() {
		_, err := source.Inspect(context.Background(), "fresh selection")
		freshErr <- err
	}()
	select {
	case <-freshEntered:
	case <-time.After(5 * time.Second):
		t.Fatal("later Stage did not recover from the wedged ancestor resolution")
	}
	freshGen := source.resolving.Load()
	if freshGen == 0 {
		t.Fatal("fresh call did not record itself as the flag's new owner")
	}

	// Now let the abandoned stale worker finish late. Its release call must
	// be a no-op: the flag belongs to the fresh call now, and this hook fires
	// deterministically right after that worker's own release attempt.
	staleCleared := make(chan struct{})
	source.afterResolverCleared = sync.OnceFunc(func() { close(staleCleared) })
	close(staleRelease)
	select {
	case <-staleCleared:
	case <-time.After(5 * time.Second):
		t.Fatal("stale worker never reached its release attempt")
	}
	if got := source.resolving.Load(); got != freshGen {
		t.Fatalf("stale worker's late release corrupted the fresh call's ownership: gen=%d, want %d", got, freshGen)
	}

	// A third call while the fresh one is still outstanding must still be
	// refused busy -- the stale worker's late release did not free it.
	if _, err := source.Inspect(context.Background(), "third selection"); transfer.ErrorCodeOf(err) != transfer.ErrBusy {
		t.Fatal("stale worker's late release let a third call in while the fresh one was still outstanding")
	}

	close(freshRelease)
	select {
	case err := <-freshErr:
		if err != nil {
			t.Fatalf("fresh call failed: code=%s", transfer.ErrorCodeOf(err))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("fresh call did not complete after release")
	}
	if got := source.resolving.Load(); got != 0 {
		t.Fatalf("flag stayed held (gen=%d) after the fresh call completed", got)
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
	if !strings.Contains(strings.ReplaceAll(string(data), "\r\n", "\n"), "case canonical := <-result:\n\t\tstop()\n\t\treturn s.inspectResolved(ctx, canonical)") {
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

/*
TestACancelledResolutionStillRecoversAWedgedFlag closes the second route to the
defect the bound exists to remove.

The bound recovers the flag when it elapses. The cancellation arm returns
before that, while the worker is still wedged -- so if stopping the timer were
part of leaving on a cancel, nothing would ever take the flag back and every
later Stage would be busy for the life of the process. That is the same
permanent busy, reached by a user pressing Cancel rather than by waiting.

The fix is that the timer releases the flag itself rather than the timedOut
arm doing it, and the cancellation arm deliberately leaves the timer armed.
Found reviewing the bound, and not by the test above: that one never cancels.
*/
func TestACancelledResolutionStillRecoversAWedgedFlag(t *testing.T) {
	source := newSelectionSource(stubSource{})
	bound := &stubBoundTimer{}
	source.boundTimer = bound.after

	entered, wedged := make(chan struct{}), make(chan struct{})
	defer close(wedged)
	enteredOnce := sync.OnceFunc(func() { close(entered) })
	source.resolve = func(path string) string {
		enteredOnce()
		<-wedged // never returns while this test runs
		return path
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelled := make(chan error, 1)
	go func() {
		_, err := source.Inspect(ctx, "private wedged selection")
		cancelled <- err
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("the resolver was never entered")
	}
	cancel()

	select {
	case err := <-cancelled:
		if transfer.ErrorCodeOf(err) != transfer.ErrCancelled {
			t.Fatalf("cancelled Inspect = %q, want %q", transfer.ErrorCodeOf(err), transfer.ErrCancelled)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a cancelled Inspect did not return")
	}

	// The worker is still wedged and still holds the flag, which is correct:
	// the decorator allows one outstanding resolution and there is one.
	if source.resolving.Load() == 0 {
		t.Fatal("the flag was released while the resolver was still running")
	}

	// The bound is what must still be able to take it back. If leaving on a
	// cancel had stopped the timer, firing it here would do nothing.
	bound.fire()

	if got := source.resolving.Load(); got != 0 {
		t.Fatalf("the flag is still held (gen=%d) after the bound fired on a cancelled resolution: "+
			"every later Stage returns busy for the life of the process", got)
	}
	// The observable consequence, not just the field. The resolver is swapped
	// first: the wedged one is still parked on <-wedged and would hang this
	// call forever, which proves nothing about the flag and hangs the package
	// instead of failing it. The abandoned worker keeps its stale generation
	// either way, so it cannot disturb what this call takes.
	source.resolve = func(path string) string { return path }
	if _, err := source.Inspect(context.Background(), "private later selection"); transfer.ErrorCodeOf(err) == transfer.ErrBusy {
		t.Error("a later Stage was refused busy by a resolution its caller had already abandoned")
	}
}
