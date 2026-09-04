package source

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"fairdrop/internal/transfer"
)

// visited is one recorded visitor call, flattened so a whole traversal can be
// compared as data rather than through nested assertions.
type visited struct {
	relative string
	kind     transfer.ItemKind
	contents string
}

func recordWalk(t *testing.T, inspector *Inspector, path string) ([]visited, error) {
	t.Helper()
	var seen []visited
	err := inspector.Walk(context.Background(), path, func(entry transfer.SourceEntry, content io.Reader) error {
		record := visited{relative: entry.RelativePath, kind: entry.Kind}
		if content != nil {
			bytes, readErr := io.ReadAll(content)
			if readErr != nil {
				return readErr
			}
			record.contents = string(bytes)
		}
		seen = append(seen, record)
		return nil
	})
	return seen, err
}

func TestWalkEmitsRootRelativeNamesWithParentsBeforeChildren(t *testing.T) {
	t.Parallel()

	root := fakeDirectory("root")
	nested := fakeDirectory("nested")
	deeper := fakeDirectory("deeper")
	root.add("top.txt", fakeFile("top.txt", "top"))
	root.add("nested", nested)
	nested.add("inner.txt", fakeFile("inner.txt", "inner"))
	nested.add("deeper", deeper)
	deeper.add("leaf.txt", fakeFile("leaf.txt", "leaf"))
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

	seen, err := recordWalk(t, &Inspector{handles: factory, sameFile: sameFakeFile}, "original")
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	want := []visited{
		{relative: "top.txt", kind: transfer.ItemFile, contents: "top"},
		{relative: "nested", kind: transfer.ItemDirectory},
		{relative: "nested/inner.txt", kind: transfer.ItemFile, contents: "inner"},
		{relative: "nested/deeper", kind: transfer.ItemDirectory},
		{relative: "nested/deeper/leaf.txt", kind: transfer.ItemFile, contents: "leaf"},
	}
	if !slices.Equal(seen, want) {
		t.Fatalf("walk = %+v, want %+v", seen, want)
	}
	// The root is never emitted: it has no relative name, and a consumer places
	// every entry under whatever top-level name it chooses.
	for _, entry := range seen {
		if entry.relative == "" || entry.relative == "." || strings.HasPrefix(entry.relative, "/") {
			t.Fatalf("entry name %q is not a relative name under the root", entry.relative)
		}
	}
	assertFakeClosed(t, factory)
}

func TestWalkEmitsAnEmptyDirectoryAndAnEmptyFile(t *testing.T) {
	t.Parallel()

	root := fakeDirectory("root")
	root.add("empty-dir", fakeDirectory("empty-dir"))
	root.add("empty.txt", fakeFile("empty.txt", ""))
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

	seen, err := recordWalk(t, &Inspector{handles: factory, sameFile: sameFakeFile}, "original")
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	want := []visited{
		{relative: "empty-dir", kind: transfer.ItemDirectory},
		{relative: "empty.txt", kind: transfer.ItemFile},
	}
	if !slices.Equal(seen, want) {
		t.Fatalf("walk = %+v, want %+v", seen, want)
	}
	assertFakeClosed(t, factory)
}

func TestWalkOpensContentThroughTheParentAndClosesItBeforeTheNextEntry(t *testing.T) {
	t.Parallel()

	root := fakeDirectory("root")
	root.add("file.txt", fakeFile("file.txt", "bytes"))
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

	if _, err := recordWalk(t, &Inspector{handles: factory, sameFile: sameFakeFile}, "original"); err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	// The content open names the parent it is relative to, so a reconstructed
	// absolute path could not produce this sequence.
	want := []string{
		"metadata:root:file.txt",
		"stat:file.txt",
		"content:root:file.txt",
		"stat-content:file.txt",
		"read-content:file.txt",
		"read-content:file.txt",
		"close-content:file.txt",
		"close-metadata:file.txt",
	}
	if !containsSubsequence(factory.ops, want) {
		t.Fatalf("ops = %v, want the parent-relative open/verify/read/close order %v", factory.ops, want)
	}
	assertFakeClosed(t, factory)
}

func TestWalkRefusesAnEntryWhoseDescriptorIsNotTheInspectedObject(t *testing.T) {
	t.Parallel()

	for name, prepare := range map[string]func(*fakeNode){
		"identity swapped": func(node *fakeNode) { node.contentIdentity = 987_654 },
		"became a link":    func(node *fakeNode) { node.contentReparse = true },
		"became special":   func(node *fakeNode) { node.contentMode = os.ModeNamedPipe },
		"became a device":  func(node *fakeNode) { node.contentMode = os.ModeDevice },
	} {
		prepare := prepare
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := fakeDirectory("root")
			entry := fakeFile("entry.txt", "bytes")
			prepare(entry)
			root.add("entry.txt", entry)
			root.add("later.txt", fakeFile("later.txt", "later"))
			factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

			seen, err := recordWalk(t, &Inspector{handles: factory, sameFile: sameFakeFile}, "original")
			if code := transfer.ErrorCodeOf(err); code != transfer.ErrPathUnsupported && code != transfer.ErrSourceChanged {
				t.Fatalf("error = %v, code = %q, want a refusal", err, code)
			}
			// The stream must stop, not skip: a later entry emitted after a
			// refusal would ship an archive missing exactly what was refused.
			if len(seen) != 0 {
				t.Fatalf("walk emitted %+v after refusing the entry", seen)
			}
			assertFakeClosed(t, factory)
		})
	}
}

func TestWalkRefusesASelectionThatIsNotADirectory(t *testing.T) {
	t.Parallel()

	root := fakeDirectory("root")
	root.add("file.txt", fakeFile("file.txt", "bytes"))
	factory := newFakeFactory(pathPlan{
		anchor: "root", components: []string{"file.txt"}, rootLabel: "root",
	}, root)

	seen, err := recordWalk(t, &Inspector{handles: factory, sameFile: sameFakeFile}, "original")
	assertCode(t, err, transfer.ErrPathUnsupported)
	if len(seen) != 0 {
		t.Fatalf("walk emitted %+v for a regular-file selection", seen)
	}
	assertFakeClosed(t, factory)
}

func TestWalkRequiresAVisitorAndAContext(t *testing.T) {
	t.Parallel()

	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, fakeDirectory("root"))
	inspector := &Inspector{handles: factory, sameFile: sameFakeFile}

	assertCode(t, inspector.Walk(context.Background(), "original", nil), transfer.ErrTransferFailed)
	assertCode(t, inspector.Walk(nil, "original", func(transfer.SourceEntry, io.Reader) error { return nil }), transfer.ErrTransferFailed) //nolint:staticcheck // the nil context is the case under test
	if len(factory.ops) != 0 {
		t.Fatalf("ops = %v, want no adapter call for a refused walk", factory.ops)
	}
}

func TestWalkReturnsTheVisitorFailureAndClosesEveryHandle(t *testing.T) {
	t.Parallel()

	for name, stopAt := range map[string]transfer.ItemKind{
		"during a file entry":      transfer.ItemFile,
		"during a directory entry": transfer.ItemDirectory,
	} {
		stopAt := stopAt
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := fakeDirectory("root")
			nested := fakeDirectory("nested")
			nested.add("inner.txt", fakeFile("inner.txt", "inner"))
			root.add("nested", nested)
			root.add("top.txt", fakeFile("top.txt", "top"))
			factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

			refusal := transfer.NewError(transfer.ErrTransferFailed, "fixture destination failure")
			err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Walk(
				context.Background(), "original",
				func(entry transfer.SourceEntry, _ io.Reader) error {
					if entry.Kind == stopAt {
						return refusal
					}
					return nil
				},
			)
			if !errors.Is(err, refusal) {
				t.Fatalf("Walk() error = %v, want the visitor's own error preserved", err)
			}
			assertFakeClosed(t, factory)
		})
	}
}

func TestWalkBorrowedReaderStopsWorkingOnceTheVisitReturns(t *testing.T) {
	t.Parallel()

	root := fakeDirectory("root")
	root.add("first.txt", fakeFile("first.txt", "first"))
	root.add("second.txt", fakeFile("second.txt", "second"))
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

	var kept []io.Reader
	err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Walk(
		context.Background(), "original",
		func(_ transfer.SourceEntry, content io.Reader) error {
			if content != nil {
				kept = append(kept, content)
			}
			// Every previously borrowed reader is already dead by now.
			for index, reader := range kept[:max(0, len(kept)-1)] {
				if _, err := reader.Read(make([]byte, 1)); err == nil {
					t.Errorf("reader %d borrowed by an earlier visit still reads", index)
				}
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if len(kept) != 2 {
		t.Fatalf("borrowed %d readers, want 2", len(kept))
	}
	for index, reader := range kept {
		if _, err := reader.Read(make([]byte, 1)); err == nil {
			t.Fatalf("reader %d still reads after Walk returned", index)
		}
	}
	assertFakeClosed(t, factory)
}

func TestWalkHoldsOneHandlePerActiveDepthPlusTheEntryBeingVisited(t *testing.T) {
	t.Parallel()

	const depth = 12
	root := fakeDirectory("root")
	current := root
	for range depth {
		child := fakeDirectory("level")
		current.add("level", child)
		for index := range 6 {
			name := "file" + string(rune('a'+index)) + ".txt"
			current.add(name, fakeFile(name, "bytes"))
		}
		current = child
	}
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

	seen, err := recordWalk(t, &Inspector{handles: factory, sameFile: sameFakeFile}, "original")
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}
	if len(seen) < depth*6 {
		t.Fatalf("walk emitted %d entries, want the whole %d-deep tree", len(seen), depth)
	}
	// One enumeration handle per active depth, plus the anchor, plus the one
	// entry being visited as both a metadata and a content handle. Nothing here
	// is a function of how many entries the tree holds.
	bound := depth + 4
	if factory.maxActive > bound {
		t.Fatalf("held %d handles at once for a depth-%d tree of %d entries, want at most %d",
			factory.maxActive, depth, len(seen), bound)
	}
	for _, size := range factory.readSizes {
		if size != directoryReadBatchSize {
			t.Fatalf("ReadDir batch = %d, want the fixed %d", size, directoryReadBatchSize)
		}
	}
	assertFakeClosed(t, factory)
}

func TestWalkCancellationStopsBeforeTheNextEntry(t *testing.T) {
	t.Parallel()

	for _, cancelAt := range []string{
		"read:root", "metadata:root:first.txt", "content:root:first.txt",
		"stat-content:first.txt", "close-content:first.txt",
	} {
		cancelAt := cancelAt
		t.Run(cancelAt, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			root := fakeDirectory("root")
			root.add("first.txt", fakeFile("first.txt", "first"))
			root.add("second.txt", fakeFile("second.txt", "second"))
			factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
			factory.onOperation = func(op string) {
				if op == cancelAt {
					cancel()
				}
			}

			err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Walk(
				ctx, "original",
				func(transfer.SourceEntry, io.Reader) error { return nil },
			)
			assertCode(t, err, transfer.ErrCancelled)
			seenCancellationPoint := false
			for _, operation := range factory.ops {
				if operation == cancelAt {
					seenCancellationPoint = true
					continue
				}
				if seenCancellationPoint && !strings.HasPrefix(operation, "close") {
					t.Fatalf("operation %q ran after cancellation point %q; ops = %v", operation, cancelAt, factory.ops)
				}
			}
			assertFakeClosed(t, factory)
		})
	}
}

func TestWalkPropagatesAContentOpenFailureWithItsCode(t *testing.T) {
	t.Parallel()

	root := fakeDirectory("root")
	root.add("entry.txt", fakeFile("entry.txt", "bytes"))
	// contentOpenErr lives on the parent that opens the child, so it goes on
	// the directory the walk actually calls. Setting it on the entry too would
	// be inert, and would imply a per-entry failure mode that is not covered.
	root.contentOpenErr = os.ErrNotExist
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

	_, err := recordWalk(t, &Inspector{handles: factory, sameFile: sameFakeFile}, "original")
	assertCode(t, err, transfer.ErrPathNotFound)
	assertFakeClosed(t, factory)
}

func TestWalkPropagatesAContentReadFailureAndStillCloses(t *testing.T) {
	t.Parallel()

	root := fakeDirectory("root")
	entry := fakeFile("entry.txt", "bytes")
	entry.readContentErr = errors.New("fixture read failure")
	root.add("entry.txt", entry)
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

	_, err := recordWalk(t, &Inspector{handles: factory, sameFile: sameFakeFile}, "original")
	// Not merely non-nil: any failure at all would satisfy that, including one
	// raised before the read the fixture is arranging.
	assertCode(t, err, transfer.ErrTransferFailed)
	if !errors.Is(err, entry.readContentErr) {
		t.Fatalf("Walk() error = %v, want it to wrap the fixture read failure", err)
	}
	assertFakeClosed(t, factory)
}

func TestWalkRejectsAnEntryNameThatIsNotASingleComponent(t *testing.T) {
	t.Parallel()

	for name, entryName := range map[string]string{
		"dot-dot":        "..",
		"dot":            ".",
		"forward slash":  "nested/escape.txt",
		"backslash":      `nested\escape.txt`,
		"nul":            "bad\x00name.txt",
		"absolute posix": "/etc/passwd",
	} {
		entryName := entryName
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := fakeDirectory("root")
			root.add(entryName, fakeFile(entryName, "bytes"))
			factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

			seen, err := recordWalk(t, &Inspector{handles: factory, sameFile: sameFakeFile}, "original")
			assertCode(t, err, transfer.ErrPathUnsupported)
			if len(seen) != 0 {
				t.Fatalf("walk emitted %+v for entry name %q", seen, entryName)
			}
			assertFakeClosed(t, factory)
		})
	}
}

func TestChildRelativeNameAccumulatesSlashSeparatedNames(t *testing.T) {
	t.Parallel()

	got, err := childRelativeName("", "top.txt")
	if err != nil || got != "top.txt" {
		t.Fatalf("childRelativeName(root) = %q, %v", got, err)
	}
	got, err = childRelativeName("nested/deeper", "leaf.txt")
	if err != nil || got != "nested/deeper/leaf.txt" {
		t.Fatalf("childRelativeName(nested) = %q, %v", got, err)
	}
	if filepath.IsAbs(got) || strings.Contains(got, `\`) {
		t.Fatalf("relative name %q is not slash-separated and relative", got)
	}
}

// Inspection must stay metadata-only: the size sum has no reason to touch a
// byte, and a preflight that opened content would need read rights on every
// file in the tree before the receiver has even scanned the code.
func TestInspectNeverOpensEntryContent(t *testing.T) {
	t.Parallel()

	root := fakeDirectory("root")
	nested := fakeDirectory("nested")
	nested.add("inner.txt", fakeFile("inner.txt", "inner"))
	root.add("nested", nested)
	root.add("top.txt", fakeFile("top.txt", "top"))
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

	item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if item.Kind != transfer.ItemDirectory || item.LogicalSize != 8 {
		t.Fatalf("item = %+v, want a directory summing 8 bytes", item)
	}
	for _, operation := range factory.ops {
		if strings.HasPrefix(operation, "content:") || strings.HasPrefix(operation, "read-content:") {
			t.Fatalf("Inspect opened entry content: ops = %v", factory.ops)
		}
	}
	assertFakeClosed(t, factory)
}

// The fallback after the lexical walk is reachable because a ".." pop re-Stats
// the ancestor it lands on without re-running the component checks. The opened
// handle still reports a directory, so only the popped Stat can catch it.
func TestInspectRefusesAPoppedAncestorThatIsNoLongerADirectory(t *testing.T) {
	t.Parallel()

	root := fakeDirectory("root")
	root.openMode = os.ModeDir
	root.add("descend", fakeDirectory("descend"))
	factory := newFakeFactory(pathPlan{
		anchor: "root", components: []string{"descend", ".."}, rootLabel: "root",
	}, root)
	stats := 0
	factory.onOperation = func(op string) {
		if op != "stat:root" {
			return
		}
		stats++
		// Stat one is the anchor check on the way down; stat two is the pop.
		// Between them the ancestor is replaced by a device node.
		if stats == 2 {
			root.mode = os.ModeDevice
		}
	}

	item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrPathUnsupported)
	if item != (transfer.StagedItem{}) {
		t.Fatalf("item = %+v, want zero metadata", item)
	}
	assertFakeClosed(t, factory)
}

func containsSubsequence(haystack, needle []string) bool {
	index := 0
	for _, operation := range haystack {
		if index < len(needle) && operation == needle[index] {
			index++
		}
	}
	return index == len(needle)
}
