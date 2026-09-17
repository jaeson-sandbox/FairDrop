package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"fairdrop/internal/transfer"
)

// selectionSource sits behind coordinator admission. The stream adapter keeps
// the raw inspector and uses the canonical path returned by this decorator.
type selectionSource struct {
	transfer.SourcePort
	resolve func(string) string

	// resolving is 0 when idle. A nonzero value is the generation token of the
	// one outstanding resolution, assigned from nextGen and never reused, so
	// releasing it is a CompareAndSwap keyed to that token rather than an
	// unconditional Store. A worker the bound has given up on still runs to
	// completion eventually and still tries to release the flag when it does;
	// keying the release to gen is what stops that late release from clearing
	// a different, later call's ownership out from under it.
	resolving atomic.Uint64
	nextGen   atomic.Uint64

	// boundTimer arms selectionResolutionBound. It defaults to a real timer;
	// a test overrides it to fire the bound deterministically instead of
	// sleeping past a real one, the same seam shape as Coordinator's
	// boundTimer in internal/transfer.
	boundTimer func(delay time.Duration, run func()) transfer.StopTimer

	// afterResolverCleared is a test-only hook, nil in production. It runs on
	// the resolver's own goroutine immediately after that goroutine's release
	// attempt, whatever the outcome -- the exact point a test needs to observe
	// to prove a stale worker's late release cannot corrupt a call that has
	// since taken the flag for itself.
	afterResolverCleared func()
}

func newSelectionSource(raw transfer.SourcePort) *selectionSource {
	return &selectionSource{
		SourcePort: raw,
		resolve:    resolveSelectionAncestors,
		boundTimer: func(delay time.Duration, run func()) transfer.StopTimer {
			return time.AfterFunc(delay, run).Stop
		},
	}
}

// selectionResolutionBound ceils how long one ancestor resolution may hold
// the busy flag before Inspect gives up on it and recovers. EvalSymlinks
// cannot be cancelled -- an unresponsive network-mounted ancestor blocks it
// forever -- so without this bound, one wedged call would leave every later
// Stage returning busy for the rest of the process. This reuses
// AdapterCleanupBound rather than inventing a second magic number: it is the
// same category of wait -- one uncancellable external call the coordinator
// is willing to give up on -- that the coordinator already applies to
// ServerPort.Stop and NetworkPort.StopBeacon (D-024 is the same shape: an
// mDNS shutdown that never returns must not hang the claim).
const selectionResolutionBound = transfer.AdapterCleanupBound

func (s *selectionSource) Inspect(ctx context.Context, path string) (transfer.StagedItem, error) {
	if ctx == nil {
		return transfer.StagedItem{}, transfer.NewError(transfer.ErrSetupFailed, "selection resolution requires a context")
	}
	if err := ctx.Err(); err != nil {
		return transfer.StagedItem{}, selectionResolutionCancelled()
	}
	gen := s.nextGen.Add(1)
	if !s.resolving.CompareAndSwap(0, gen) {
		return transfer.StagedItem{}, transfer.NewError(transfer.ErrBusy, "a previous selection resolution is still outstanding")
	}
	// An OS filesystem call cannot be cancelled, so this worker cannot be
	// stopped, only abandoned -- the same shape as callBounded in
	// internal/transfer/bounded.go. Arm the bound before launching the worker,
	// for the same reason callBounded does: arming after would race the worker
	// for which one a test observes first. No lock is held across any of this,
	// and only the waiting caller may proceed into a fresh Inspect once the
	// flag is released, never this worker itself.
	timedOut := make(chan struct{})
	// The timer releases the flag itself rather than leaving that to whichever
	// select arm happens to win. It has to: the cancellation arm below returns
	// while the worker is still wedged, and if recovery lived in the timedOut
	// arm instead, a caller who cancelled would stop the timer and leave the
	// flag held by a call that never comes back -- every later Stage busy for
	// the rest of the process, which is the exact defect this bound exists to
	// remove, reached by a different route. Keyed to gen, so firing after the
	// worker already released, or after a later call took the flag, is a no-op
	// rather than a corruption.
	stop := s.boundTimer(selectionResolutionBound, func() {
		s.release(gen)
		close(timedOut)
	})
	result := make(chan string, 1)
	go func() {
		canonical := s.resolve(path)
		s.release(gen)
		if s.afterResolverCleared != nil {
			s.afterResolverCleared()
		}
		result <- canonical
	}()
	select {
	case <-ctx.Done():
		// Deliberately not stopped. The worker is still running and still
		// holds the flag; the bound is the only thing that will take it back
		// if the worker never finishes. A timer outliving its caller by at
		// most one bound is the price of that, and it holds nothing else.
		return transfer.StagedItem{}, selectionResolutionCancelled()
	case canonical := <-result:
		stop()
		return s.inspectResolved(ctx, canonical)
	case <-timedOut:
		// The flag is already recovered, by the timer above. The worker's
		// eventual result, if it ever arrives, lands in a buffered channel
		// nobody reads again.
		return transfer.StagedItem{}, selectionResolutionBoundElapsed()
	}
}

// release clears the busy flag only if it is still held by gen. See the
// resolving field comment: a worker abandoned by the bound still calls this
// when it eventually finishes, and the keyed compare-and-swap is what keeps
// that late call from erasing a different, later resolution's ownership.
func (s *selectionSource) release(gen uint64) {
	s.resolving.CompareAndSwap(gen, 0)
}

// Cancellation can race result delivery. Keep this acceptance gate separate
// from select's nondeterministic choice when both channels are ready.
func (s *selectionSource) inspectResolved(ctx context.Context, canonical string) (transfer.StagedItem, error) {
	if ctx.Err() != nil {
		return transfer.StagedItem{}, selectionResolutionCancelled()
	}
	return s.SourcePort.Inspect(ctx, canonical)
}

func selectionResolutionCancelled() error {
	return transfer.NewError(transfer.ErrCancelled, "selection resolution was cancelled")
}

// selectionResolutionBoundElapsed reports that the bound elapsed -- never
// that the resolution succeeded or that it was cancelled. AD-9: name no path,
// even the one this abandoned worker was resolving when the bound gave up.
func selectionResolutionBoundElapsed() error {
	return transfer.NewError(transfer.ErrSetupFailed, "selection resolution did not finish before its bound")
}

// Resolve only the ancestors at the selection boundary. In particular macOS
// spells its temporary directory through /var, a symlink. Inspect and Walk
// still refuse every link they encounter, including the selected leaf.
func resolveSelectionAncestors(selection string) string {
	return resolveAncestorsWith(selection, filepath.EvalSymlinks)
}

func resolveAncestorsWith(selection string, eval func(string) (string, error)) string {
	if !filepath.IsAbs(selection) {
		return selection // The source owns invalid/missing-path classifications.
	}
	// EvalSymlinks clamps '..' at the root. The source deliberately refuses
	// that spelling, so never canonicalize away an attempted root escape.
	depth := 0
	for _, component := range strings.FieldsFunc(selection[len(filepath.VolumeName(selection)):], func(r rune) bool {
		return r < 128 && os.IsPathSeparator(uint8(r))
	}) {
		switch component {
		case ".":
		case "..":
			if depth == 0 {
				return selection
			}
			depth--
		default:
			depth++
		}
	}
	if os.PathSeparator == '\\' {
		// Device/extended namespaces have a stricter source grammar. Do not
		// let EvalSymlinks turn a spelling the source refuses into a drive path.
		lower := strings.ToLower(selection)
		if strings.HasPrefix(lower, `\\.\`) || strings.HasPrefix(lower, `\??\`) || strings.HasPrefix(lower, `\\?\`) {
			return selection
		}
	}
	// A trailing separator must not turn the selected leaf into an ancestor.
	leafPath := strings.TrimRightFunc(selection, func(r rune) bool { return r < 128 && os.IsPathSeparator(uint8(r)) })
	if leafPath == "" || leafPath == filepath.VolumeName(selection) {
		return selection
	}
	leaf := filepath.Base(leafPath)
	if leaf == "." || leaf == ".." {
		return selection // Preserve explicit dot traversal for the handle walker.
	}
	// Do not use filepath.Dir here: its lexical Clean can erase a link or a
	// non-directory before '..' without ever asking the filesystem about it.
	ancestorPath := leafPath[:len(leafPath)-len(leaf)]
	trimmed := strings.TrimRightFunc(ancestorPath, func(r rune) bool { return r < 128 && os.IsPathSeparator(uint8(r)) })
	if trimmed != "" && trimmed != filepath.VolumeName(ancestorPath) {
		// Preserve root separators without lexically cleaning any component.
		ancestorPath = trimmed
	}
	parent, err := eval(ancestorPath)
	if err != nil {
		return selection // Preserve the source's coded refusal; log no path.
	}
	return filepath.Join(parent, leaf) + selection[len(leafPath):]
}
