package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

func TestInspectProductionDefaultSafeTreesAndFiles(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "safe tree ü")
	empty := filepath.Join(root, "empty")
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(empty, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.bin"), []byte("abc"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "b.bin"), []byte("12345"), 0o600); err != nil {
		t.Fatal(err)
	}

	item, err := New().Inspect(context.Background(), root)
	if err != nil {
		t.Fatalf("Inspect(directory) error = %v", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if item.Path != root || item.Name != "safe tree ü" || item.Kind != transfer.ItemDirectory ||
		item.LogicalSize != 8 || !item.ModTime.Equal(info.ModTime()) {
		t.Fatalf("directory item = %+v, want exact path/name/kind/size/modtime", item)
	}

	file := filepath.Join(root, "a.bin")
	fileItem, err := New().Inspect(context.Background(), file)
	if err != nil {
		t.Fatalf("Inspect(file) error = %v", err)
	}
	if fileItem.Path != file || fileItem.Name != "a.bin" || fileItem.Kind != transfer.ItemFile || fileItem.LogicalSize != 3 {
		t.Fatalf("file item = %+v", fileItem)
	}
}

func TestInspectProductionDefaultZeroByteFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "empty.bin")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	item, err := New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if item.Path != path || item.Name != "empty.bin" || item.Kind != transfer.ItemFile || item.LogicalSize != 0 {
		t.Fatalf("item = %+v, want zero-byte file metadata", item)
	}
}

func TestInspectProductionDefaultEmptyAndTrailingDirectory(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "empty")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	selected := root + string(os.PathSeparator)
	item, err := New().Inspect(context.Background(), selected)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if item.Path != selected || item.Name != "empty" || item.Kind != transfer.ItemDirectory || item.LogicalSize != 0 {
		t.Fatalf("item = %+v, want byte-identical trailing path and empty directory metadata", item)
	}
}

func TestInspectProductionDefaultDotDotPreservesCallerPath(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "chosen")
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatal(err)
	}
	selected := child + string(os.PathSeparator) + ".."
	item, err := New().Inspect(context.Background(), selected)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if item.Path != selected || item.Name != "chosen" || item.Kind != transfer.ItemDirectory {
		t.Fatalf("item = %+v, want preserved dot-dot spelling and chosen root", item)
	}
}

func TestInspectRejectsLexicalEscapeAfterOpeningAnchorAndClosesIt(t *testing.T) {
	t.Parallel()
	for _, plan := range []pathPlan{
		{anchor: "root", components: []string{".."}, rootLabel: "root"},
		{anchor: "root", components: []string{".", ".."}, rootLabel: "root"},
	} {
		factory := newFakeFactory(plan, fakeDirectory("root"))
		_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
		assertCode(t, err, transfer.ErrPathUnsupported)
		if !slices.Contains(factory.ops, "open-anchor") {
			t.Fatalf("ops = %v, want anchor opened before lexical escape refusal", factory.ops)
		}
		if factory.active != 0 {
			t.Fatalf("active handles = %d, want zero", factory.active)
		}
	}
}

func TestInspectUsesNativeRootLabelsAfterDotDotReturnsToAnchor(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		plan pathPlan
		want string
	}{
		{name: "drive root", plan: pathPlan{anchor: "drive", rootLabel: "C"}, want: "C"},
		{name: "drive component dot-dot", plan: pathPlan{anchor: "drive", components: []string{"child", ".."}, rootLabel: "C"}, want: "C"},
		{name: "UNC root", plan: pathPlan{anchor: "unc", rootLabel: "share"}, want: "share"},
		{name: "UNC component dot-dot", plan: pathPlan{anchor: "unc", components: []string{"child", ".."}, rootLabel: "share"}, want: "share"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := fakeDirectory("synthetic-anchor-name")
			root.add("child", fakeDirectory("child"))
			factory := newFakeFactory(test.plan, root)
			item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "caller-spelling")
			if err != nil {
				t.Fatalf("Inspect() error = %v", err)
			}
			if item.Name != test.want || item.Path != "caller-spelling" || item.Kind != transfer.ItemDirectory {
				t.Fatalf("item = %+v, want root label %q and preserved path", item, test.want)
			}
			assertFakeClosed(t, factory)
		})
	}
}

func TestInspectDeferredCleanupErrorClearsReturnedMetadata(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	root.closeErrByKind = map[string]error{"search": errors.New("fixture deferred cleanup failure")}
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "private-path")
	assertCode(t, err, transfer.ErrTransferFailed)
	if item != (transfer.StagedItem{}) {
		t.Fatalf("item = %+v, want zero metadata when deferred cleanup fails", item)
	}
	assertFakeClosed(t, factory)
}

func TestInspectNilAndAlreadyCancelledContextsNeverCallAdapter(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		ctx  context.Context
		code transfer.ErrorCode
	}{
		{name: "nil", ctx: nil, code: transfer.ErrTransferFailed},
		{name: "already cancelled", ctx: cancelledContext(), code: transfer.ErrCancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, fakeDirectory("root"))
			item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(test.ctx, "original")
			assertCode(t, err, test.code)
			if item != (transfer.StagedItem{}) || len(factory.ops) != 0 {
				t.Fatalf("item/ops = %+v/%v, want zero item and no adapter call", item, factory.ops)
			}
		})
	}
}

func TestInspectCancellationFromParsePreventsAnchorOpen(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, fakeDirectory("root"))
	factory.parseErr = errors.New("fixture Parse error must lose to cancellation")
	factory.onOperation = func(op string) {
		if op == "parse" {
			cancel()
		}
	}
	item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(ctx, "original")
	assertCode(t, err, transfer.ErrCancelled)
	if item != (transfer.StagedItem{}) || !slices.Equal(factory.ops, []string{"parse"}) {
		t.Fatalf("item/ops = %+v/%v, want Parse only and zero metadata", item, factory.ops)
	}
}

func TestInspectPreservesCodedErrorsAcrossFilesystemClassifiers(t *testing.T) {
	t.Parallel()
	for _, code := range []transfer.ErrorCode{transfer.ErrTransferFailed, transfer.ErrSourceChanged} {
		code := code
		t.Run(string(code), func(t *testing.T) {
			for _, operation := range []string{"anchor open", "root stat", "search open", "enumeration open", "enumeration read", "child open", "child stat"} {
				operation := operation
				t.Run(operation, func(t *testing.T) {
					coded := transfer.NewError(code, "fixture coded failure")
					injected := fmt.Errorf("fixture wrapper: %w", coded)
					root := fakeDirectory("root")
					child := fakeRegular("child", 1)
					plan := pathPlan{anchor: "root", rootLabel: "root"}
					if operation == "search open" {
						child = fakeDirectory("child")
						child.searchErr = injected
						plan.components = []string{"child"}
					}
					root.add("child", child)
					factory := newFakeFactory(plan, root)
					switch operation {
					case "anchor open":
						factory.openErr = injected
					case "root stat":
						root.statErr = injected
					case "enumeration open":
						root.enumerateErr = injected
					case "enumeration read":
						root.readErr = injected
					case "child open":
						root.childOpenErr = injected
					case "child stat":
						child.statErr = injected
					}
					item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
					if err != injected {
						t.Fatalf("error = %#v, want original coded wrapper %#v", err, injected)
					}
					if item != (transfer.StagedItem{}) {
						t.Fatalf("item = %+v, want zero metadata", item)
					}
					assertFakeClosed(t, factory)
				})
			}
		})
	}
}

func TestInspectUsesParentRelativeOneEntryTraversalAndBoundedHandles(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	for index := 0; index < 200; index++ {
		name := fmt.Sprintf("file-%03d-%s", index, strings.Repeat("x", index%7))
		root.add(name, fakeRegular(name, int64(index)))
	}
	deep := fakeDirectory("deep")
	deeper := fakeDirectory("deeper")
	deeper.add("payload", fakeRegular("payload", 7))
	deep.add("deeper", deeper)
	root.add("deep", deep)

	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "unchanged")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	var want int64 = 7
	for index := 0; index < 200; index++ {
		want += int64(index)
	}
	if item.LogicalSize != want {
		t.Fatalf("LogicalSize = %d, want %d", item.LogicalSize, want)
	}
	if len(factory.readSizes) == 0 || !slices.Equal(slices.Compact(factory.readSizes), []int{1}) {
		t.Fatalf("ReadDir batch sizes = %v, want only literal 1", factory.readSizes)
	}
	if factory.maxActive > 8 {
		t.Fatalf("max active handles = %d, want depth-bounded handles", factory.maxActive)
	}
	if factory.active != 0 {
		t.Fatalf("active handles = %d, want zero", factory.active)
	}
	for _, op := range factory.ops {
		if strings.Contains(op, string(os.PathSeparator)) {
			t.Fatalf("operation %q reconstructed a child path", op)
		}
	}
}

func TestInspectLargeWideTreeKeepsConstantLiveHandleCeiling(t *testing.T) {
	const entries = 50000
	root := fakeDirectory("root")
	root.add("file", fakeRegular("file", 1))
	root.repeatEntries = entries
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	factory.recordOps = false

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if item.LogicalSize != entries {
		t.Fatalf("LogicalSize = %d, want %d", item.LogicalSize, entries)
	}
	if factory.maxActive > 3 || factory.active != 0 {
		t.Fatalf("max/current live handles = %d/%d, want constant ceiling 3/0", factory.maxActive, factory.active)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(root)
	runtime.KeepAlive(factory)
	runtime.KeepAlive(item)
	const retainedCeiling = uint64(4 << 20)
	if after.HeapAlloc > before.HeapAlloc+retainedCeiling {
		t.Fatalf("retained live heap grew by %d bytes for %d entries, want <= %d", after.HeapAlloc-before.HeapAlloc, entries, retainedCeiling)
	}
}

func TestInspectRefusesChangedOpenedIdentityBeforeEnumeration(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	root.openIdentity = root.identity + 100
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrSourceChanged)
	if slices.Contains(factory.ops, "read:root") {
		t.Fatal("changed directory was enumerated")
	}
	assertFakeClosed(t, factory)
}

func TestInspectRefusesPostOpenReparseBeforeEnumeration(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	root.openReparse = true
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrPathUnsupported)
	if slices.Contains(factory.ops, "read:root") {
		t.Fatal("link-like opened directory was enumerated")
	}
	assertFakeClosed(t, factory)
}

func TestInspectNestedLookupStaysRelativeToOpenedParentAfterRename(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	child := fakeRegular("kept", 9)
	root.add("kept", child)
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	factory.onOperation = func(op string) {
		if op == "read:root" {
			// A global path lookup would now see a replacement. The fake handle
			// still resolves "kept" through the opened root object.
			factory.root = fakeDirectory("replacement")
		}
	}
	item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if item.LogicalSize != 9 || !slices.Contains(factory.ops, "metadata:root:kept") {
		t.Fatalf("item/ops = %+v/%v, want parent-relative child lookup", item, factory.ops)
	}
}

func TestInspectRejectsLinksSpecialsAndStopsBeforeLaterEntries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		node *fakeNode
	}{
		{name: "symlink", node: fakeMode("unsafe", os.ModeSymlink)},
		{name: "regular-mode reparse", node: func() *fakeNode {
			node := fakeRegular("unsafe", 1)
			node.reparse = true
			return node
		}()},
		{name: "named pipe", node: fakeMode("unsafe", os.ModeNamedPipe)},
		{name: "device", node: fakeMode("unsafe", os.ModeDevice)},
		{name: "socket", node: fakeMode("unsafe", os.ModeSocket)},
		{name: "irregular", node: fakeMode("unsafe", os.ModeIrregular)},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := fakeDirectory("root")
			root.add("unsafe", test.node)
			root.add("later", fakeRegular("later", 99))
			factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
			_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
			assertCode(t, err, transfer.ErrPathUnsupported)
			if slices.Contains(factory.ops, "metadata:root:later") {
				t.Fatal("inspection continued after unsafe entry")
			}
			assertFakeClosed(t, factory)
		})
	}
}

/*
A *selected* special file, not only a nested one.

The frozen matrix refuses a selected symlink, reparse point, or special file,
but every case in the table above adds its unsafe node as a child of the
selected directory. That left the selected-root half of the row resting on two
guards -- the non-regular/non-directory clause in rejectUnsupportedInfo and the
not-a-directory fallback after the component walk -- which could both be
deleted with the suite still green, because each masked the other. Selecting
the special entry directly pins the refusal instead of the redundancy.
*/
func TestInspectRejectsSelectedSpecialFile(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		mode fs.FileMode
	}{
		{name: "named pipe", mode: os.ModeNamedPipe},
		{name: "device", mode: os.ModeDevice},
		{name: "socket", mode: os.ModeSocket},
		{name: "irregular", mode: os.ModeIrregular},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := fakeDirectory("root")
			root.add("chosen", fakeMode("chosen", test.mode))
			factory := newFakeFactory(pathPlan{anchor: "root", components: []string{"chosen"}, rootLabel: "root"}, root)
			_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
			assertCode(t, err, transfer.ErrPathUnsupported)
			assertFakeClosed(t, factory)
		})
	}
}

/*
A trailing separator names a directory, so a regular file behind one is refused.

parseWindowsPath and its POSIX peer were pinned to *report* hadTrailingSep, and
the directory side of the matrix row was covered, but nothing drove Inspect
with a trailing separator onto a file -- so the one branch that consumes the
flag could be removed without a failure.
*/
func TestInspectRejectsTrailingSeparatorOnRegularFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	item, err := New().Inspect(context.Background(), path+string(os.PathSeparator))
	if err == nil {
		t.Fatalf("Inspect() staged %+v, want a refusal for a file behind a trailing separator", item)
	}
	assertCode(t, err, transfer.ErrPathUnsupported)
}

/*
A nested directory that is a regular file by the time it is opened.

The identity check and this kind recheck both defend the "checked directory is
replaced before open" row, and identity alone caught every existing case -- so
the recheck could be disabled with the suite green. Windows reuses file IDs
after a delete, so a swap that preserves identity is the case only this guard
refuses. openMode changes what the *opened* handle reports while leaving the
inspected metadata and identity untouched, which is that swap exactly.
*/
func TestInspectRefusesNestedDirectoryOpenedAsFileWithUnchangedIdentity(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	sub := fakeDirectory("sub")
	sub.openMode = 0o644
	root.add("sub", sub)
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrSourceChanged)
	assertFakeClosed(t, factory)
}

func TestInspectRejectsActiveAncestorCycle(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	child := fakeDirectory("child")
	child.identity = root.identity
	root.add("child", child)
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrPathUnsupported)
	assertFakeClosed(t, factory)
}

func TestInspectRejectsMissingAndUnreadableEntriesWithoutPathDisclosure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		err  error
		code transfer.ErrorCode
	}{
		{name: "missing", err: fs.ErrNotExist, code: transfer.ErrPathNotFound},
		{name: "unreadable", err: fs.ErrPermission, code: transfer.ErrPathUnsupported},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := fakeDirectory("root")
			root.add("secret-name", fakeRegular("secret-name", 1))
			root.childOpenErr = test.err
			factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
			_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "private-original-path")
			assertCode(t, err, test.code)
			if strings.Contains(err.Error(), "secret-name") || strings.Contains(err.Error(), "private-original-path") {
				t.Fatalf("error disclosed a source path: %v", err)
			}
			assertFakeClosed(t, factory)
		})
	}
}

func TestInspectRejectsNegativeAndOverflowingLogicalSizes(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		sizes []int64
	}{
		{name: "negative", sizes: []int64{-1}},
		{name: "overflow", sizes: []int64{math.MaxInt64, 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := fakeDirectory("root")
			for index, size := range test.sizes {
				name := string(rune('a' + index))
				root.add(name, fakeRegular(name, size))
			}
			factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
			_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
			assertCode(t, err, transfer.ErrTransferFailed)
			assertFakeClosed(t, factory)
		})
	}
}

func TestInspectCancellationWinsAfterEveryActiveOperation(t *testing.T) {
	t.Parallel()
	operations := []string{
		"open-anchor", "stat:root", "enumerate:root", "stat-open:root",
		"read:root", "metadata:root:file", "stat:file", "close:file", "close:root",
	}
	for _, cancelAt := range operations {
		cancelAt := cancelAt
		t.Run(cancelAt, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			root := fakeDirectory("root")
			root.add("file", fakeRegular("file", 1))
			factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
			factory.onOperation = func(op string) {
				if op == cancelAt {
					cancel()
				}
			}
			_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(ctx, "original")
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

func TestInspectCancellationWinsDuringLexicalSearchOperations(t *testing.T) {
	t.Parallel()
	for _, cancelAt := range []string{"metadata:root:chosen", "stat:chosen", "search:chosen", "stat-open:chosen", "close:chosen"} {
		cancelAt := cancelAt
		t.Run(cancelAt, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			root := fakeDirectory("root")
			chosen := fakeDirectory("chosen")
			root.add("chosen", chosen)
			factory := newFakeFactory(pathPlan{anchor: "root", components: []string{"chosen"}, rootLabel: "root"}, root)
			factory.onOperation = func(op string) {
				if op == cancelAt {
					cancel()
				}
			}
			_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(ctx, "original")
			assertCode(t, err, transfer.ErrCancelled)
			assertFakeClosed(t, factory)
		})
	}
}

func TestInspectAncestorCloseFailureStillClosesEveryOwnedHandle(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	first := fakeDirectory("first")
	second := fakeDirectory("second")
	file := fakeRegular("file", 1)
	root.add("first", first)
	first.add("second", second)
	second.add("file", file)
	second.closeErrByKind = map[string]error{"search": errors.New("fixture ancestor close failure")}
	factory := newFakeFactory(pathPlan{
		anchor: "root", components: []string{"first", "second", "file"}, rootLabel: "root",
	}, root)
	_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrTransferFailed)
	assertFakeClosed(t, factory)
}

func TestInspectCancellationDuringAncestorCloseStillClosesEveryOwnedHandle(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	root := fakeDirectory("root")
	first := fakeDirectory("first")
	second := fakeDirectory("second")
	file := fakeRegular("file", 1)
	root.add("first", first)
	first.add("second", second)
	second.add("file", file)
	factory := newFakeFactory(pathPlan{
		anchor: "root", components: []string{"first", "second", "file"}, rootLabel: "root",
	}, root)
	factory.onOperation = func(op string) {
		if op == "close-search:second" {
			cancel()
		}
	}
	_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(ctx, "original")
	assertCode(t, err, transfer.ErrCancelled)
	assertFakeClosed(t, factory)
}

func TestInspectCloseFailureStillClosesEveryOwnedHandle(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	child := fakeDirectory("child")
	child.add("file", fakeRegular("file", 1))
	root.add("child", child)
	child.closeErr = errors.New("fixture close failure")
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrTransferFailed)
	assertFakeClosed(t, factory)
}

func TestInspectTraversalEnumerationCloseFailureClosesEveryHandle(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	root.closeErrByKind = map[string]error{"enumeration": errors.New("fixture enumeration close failure")}
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	item, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrTransferFailed)
	if item != (transfer.StagedItem{}) {
		t.Fatalf("item = %+v, want zero metadata on enumeration close failure", item)
	}
	assertFakeClosed(t, factory)
}

func TestInspectPrimaryErrorSurvivesCleanupFailure(t *testing.T) {
	t.Parallel()
	root := fakeDirectory("root")
	child := fakeRegular("child", 1)
	child.statErr = fs.ErrPermission
	child.closeErr = errors.New("secondary close failure")
	root.add("child", child)
	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)
	_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrPathUnsupported)
	assertFakeClosed(t, factory)
}

func TestFilesystemRootLabelsAreSeparatorFree(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		volume := filepath.VolumeName(t.TempDir())
		plan, err := (nativeHandleFactory{}).Parse(volume + string(os.PathSeparator))
		if err != nil {
			t.Fatalf("Parse(drive root) error = %v", err)
		}
		want := strings.TrimSuffix(volume, ":")
		if plan.rootLabel != want || strings.ContainsAny(plan.rootLabel, "/\\:") {
			t.Fatalf("root label = %q, want %q without punctuation", plan.rootLabel, want)
		}
		return
	}
	item, err := New().Inspect(context.Background(), "/")
	if err != nil {
		t.Fatalf("Inspect(/) error = %v", err)
	}
	if item.Name != "root" {
		t.Fatalf("root label = %q, want literal root", item.Name)
	}
}

type fakeFactory struct {
	plan        pathPlan
	root        *fakeNode
	active      int
	maxActive   int
	opened      int
	closed      int
	readSizes   []int
	ops         []string
	onOperation func(string)
	recordOps   bool
	parseErr    error
	openErr     error
}

func newFakeFactory(plan pathPlan, root *fakeNode) *fakeFactory {
	return &fakeFactory{plan: plan, root: root, recordOps: true}
}

func (f *fakeFactory) Parse(string) (pathPlan, error) {
	f.record("parse")
	return f.plan, f.parseErr
}
func (f *fakeFactory) OpenAnchor(pathPlan) (metadataHandle, error) {
	f.record("open-anchor")
	if f.openErr != nil {
		return nil, f.openErr
	}
	return f.open(f.root, "search", false), nil
}
func (f *fakeFactory) open(node *fakeNode, kind string, opened bool) *fakeHandle {
	f.active++
	f.opened++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	return &fakeHandle{factory: f, node: node, kind: kind, opened: opened}
}
func (f *fakeFactory) record(op string) {
	if f.recordOps {
		f.ops = append(f.ops, op)
	}
	if f.onOperation != nil {
		f.onOperation(op)
	}
}

type fakeHandle struct {
	factory *fakeFactory
	node    *fakeNode
	kind    string
	opened  bool
	index   int
	closed  bool
}

func (h *fakeHandle) Stat() (fs.FileInfo, error) {
	name := h.node.name
	if h.opened {
		h.factory.record("stat-open:" + name)
	} else {
		h.factory.record("stat:" + name)
	}
	if h.node.statErr != nil {
		return nil, h.node.statErr
	}
	info := h.node.info()
	if h.opened {
		if h.node.openIdentity != 0 {
			info.identity = h.node.openIdentity
		}
		if h.node.openMode != 0 {
			info.mode = h.node.openMode
		}
		if h.node.openReparse {
			info.reparse = true
		}
	}
	return info, nil
}

func (h *fakeHandle) OpenChildMetadata(name string) (metadataHandle, error) {
	h.factory.record("metadata:" + h.node.name + ":" + name)
	if h.node.childOpenErr != nil {
		return nil, h.node.childOpenErr
	}
	child := h.node.children[name]
	if child == nil {
		return nil, fs.ErrNotExist
	}
	return h.factory.open(child, "metadata", false), nil
}

func (h *fakeHandle) OpenSearch() (metadataHandle, error) {
	h.factory.record("search:" + h.node.name)
	if h.node.searchErr != nil {
		return nil, h.node.searchErr
	}
	return h.factory.open(h.node, "search", true), nil
}

func (h *fakeHandle) OpenEnumeration() (directoryHandle, error) {
	h.factory.record("enumerate:" + h.node.name)
	if h.node.enumerateErr != nil {
		return nil, h.node.enumerateErr
	}
	return h.factory.open(h.node, "enumeration", true), nil
}

func (h *fakeHandle) ReadDir(count int) ([]fs.DirEntry, error) {
	h.factory.record("read:" + h.node.name)
	if h.factory.recordOps {
		h.factory.readSizes = append(h.factory.readSizes, count)
	}
	if h.node.readErr != nil {
		return nil, h.node.readErr
	}
	if h.node.repeatEntries > 0 {
		if h.index >= h.node.repeatEntries {
			return nil, io.EOF
		}
		h.index++
		return []fs.DirEntry{fakeDirEntry{info: h.node.children[h.node.order[0]].info()}}, nil
	}
	if h.index >= len(h.node.order) {
		return nil, io.EOF
	}
	name := h.node.order[h.index]
	h.index++
	return []fs.DirEntry{fakeDirEntry{info: h.node.children[name].info()}}, nil
}

func (h *fakeHandle) Close() error {
	if h == nil || h.closed {
		return nil
	}
	h.factory.record("close-" + h.kind + ":" + h.node.name)
	h.factory.record("close:" + h.node.name)
	h.closed = true
	h.factory.active--
	h.factory.closed++
	if err := h.node.closeErrByKind[h.kind]; err != nil {
		return err
	}
	return h.node.closeErr
}

type fakeNode struct {
	name           string
	mode           fs.FileMode
	size           int64
	modTime        time.Time
	identity       int
	openIdentity   int
	openMode       fs.FileMode
	reparse        bool
	openReparse    bool
	children       map[string]*fakeNode
	order          []string
	childOpenErr   error
	searchErr      error
	enumerateErr   error
	statErr        error
	readErr        error
	closeErr       error
	closeErrByKind map[string]error
	repeatEntries  int
}

var nextFakeIdentity atomic.Int64

func fakeDirectory(name string) *fakeNode { return fakeMode(name, os.ModeDir) }
func fakeRegular(name string, size int64) *fakeNode {
	node := fakeMode(name, 0)
	node.size = size
	return node
}
func fakeMode(name string, mode fs.FileMode) *fakeNode {
	node := &fakeNode{name: name, mode: mode, identity: int(nextFakeIdentity.Add(1)), children: map[string]*fakeNode{}}
	return node
}
func (n *fakeNode) add(name string, child *fakeNode) {
	n.children[name] = child
	n.order = append(n.order, name)
}
func (n *fakeNode) info() fakeFileInfo {
	return fakeFileInfo{name: n.name, mode: n.mode, size: n.size, modTime: n.modTime, identity: n.identity, reparse: n.reparse}
}

type fakeFileInfo struct {
	name     string
	mode     fs.FileMode
	size     int64
	modTime  time.Time
	identity int
	sys      any
	reparse  bool
}

func (f fakeFileInfo) Name() string          { return f.name }
func (f fakeFileInfo) Size() int64           { return f.size }
func (f fakeFileInfo) Mode() fs.FileMode     { return f.mode }
func (f fakeFileInfo) ModTime() time.Time    { return f.modTime }
func (f fakeFileInfo) IsDir() bool           { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any              { return f.sys }
func (f fakeFileInfo) FairDropReparse() bool { return f.reparse }

type fakeDirEntry struct{ info fs.FileInfo }

func (f fakeDirEntry) Name() string               { return f.info.Name() }
func (f fakeDirEntry) IsDir() bool                { return f.info.IsDir() }
func (f fakeDirEntry) Type() fs.FileMode          { return f.info.Mode().Type() }
func (f fakeDirEntry) Info() (fs.FileInfo, error) { return f.info, nil }

func sameFakeFile(first, second fs.FileInfo) bool {
	left, lok := first.(fakeFileInfo)
	right, rok := second.(fakeFileInfo)
	return lok && rok && left.identity == right.identity
}

func assertFakeClosed(t *testing.T, factory *fakeFactory) {
	t.Helper()
	if factory.active != 0 || factory.closed != factory.opened {
		t.Fatalf("handles opened/closed/active = %d/%d/%d", factory.opened, factory.closed, factory.active)
	}
}

func assertCode(t *testing.T, err error, want transfer.ErrorCode) {
	t.Helper()
	if got := transfer.ErrorCodeOf(err); got != want {
		t.Fatalf("error = %v, code = %q, want %q", err, got, want)
	}
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}
