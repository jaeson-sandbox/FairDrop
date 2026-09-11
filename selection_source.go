package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"fairdrop/internal/transfer"
)

// selectionSource sits behind coordinator admission. The stream adapter keeps
// the raw inspector and uses the canonical path returned by this decorator.
type selectionSource struct {
	transfer.SourcePort
	resolve   func(string) string
	resolving atomic.Bool
}

func newSelectionSource(raw transfer.SourcePort) *selectionSource {
	return &selectionSource{SourcePort: raw, resolve: resolveSelectionAncestors}
}

func (s *selectionSource) Inspect(ctx context.Context, path string) (transfer.StagedItem, error) {
	if ctx == nil {
		return transfer.StagedItem{}, transfer.NewError(transfer.ErrSetupFailed, "selection resolution requires a context")
	}
	if err := ctx.Err(); err != nil {
		return transfer.StagedItem{}, selectionResolutionCancelled()
	}
	if !s.resolving.CompareAndSwap(false, true) {
		return transfer.StagedItem{}, transfer.NewError(transfer.ErrBusy, "a previous selection resolution is still outstanding")
	}
	// An OS filesystem call cannot be cancelled. Bound abandoned work to one
	// call per decorator; no lock is held across it and only the waiting caller
	// may proceed into Inspect, never this worker after the caller has left.
	result := make(chan string, 1)
	go func() {
		canonical := s.resolve(path)
		s.resolving.Store(false)
		result <- canonical
	}()
	select {
	case <-ctx.Done():
		return transfer.StagedItem{}, selectionResolutionCancelled()
	case canonical := <-result:
		if ctx.Err() != nil {
			return transfer.StagedItem{}, selectionResolutionCancelled()
		}
		return s.SourcePort.Inspect(ctx, canonical)
	}
}

func selectionResolutionCancelled() error {
	return transfer.NewError(transfer.ErrCancelled, "selection resolution was cancelled")
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
