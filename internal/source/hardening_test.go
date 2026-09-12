package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"fairdrop/internal/transfer"
)

func TestDirectoryHandleBudgetIncludesAncestorsAndPreparedPin(t *testing.T) {
	for _, depth := range []int{53, 54} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			root := fakeDirectory("root")
			current := root
			var components []string
			for n := 0; n < 8; n++ {
				name := fmt.Sprintf("ancestor%d", n)
				child := fakeDirectory(name)
				current.add(name, child)
				current = child
				components = append(components, name)
			}
			for n := 1; n <= depth; n++ {
				name := fmt.Sprintf("depth%d", n)
				child := fakeDirectory(name)
				current.add(name, child)
				current = child
			}
			factory := newFakeFactory(pathPlan{rootLabel: "root", components: components}, root)
			inspector := &Inspector{handles: factory, sameFile: sameFakeFile}
			_, err := inspector.Inspect(context.Background(), "original")
			if depth == 54 {
				assertCode(t, err, transfer.ErrPathUnsupported)
				for _, op := range factory.ops {
					if op == "enumerate:depth54" {
						t.Fatal("depth guard acquired the forbidden enumeration handle")
					}
				}
			} else if err != nil {
				t.Fatalf("accepted depth failed inspection: %v", err)
			}
			assertFakeClosed(t, factory)
			prepared, err := inspector.PrepareDirectory(context.Background(), "original")
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			err = prepared.Walk(context.Background(), func(transfer.SourceEntry, io.Reader) error { seen++; return nil })
			if depth == 54 {
				assertCode(t, err, transfer.ErrPathUnsupported)
			} else if err != nil || seen != 53 {
				t.Fatalf("accepted unchanged tree did not stream: visits=%d error=%v", seen, err)
			}
			if err := prepared.Close(); err != nil {
				t.Fatal(err)
			}
			if factory.maxActive > 67 {
				t.Fatalf("handle consumption exceeded 64 retained plus three transient: %d", factory.maxActive)
			}
			assertFakeClosed(t, factory)
		})
	}
}

func TestLexicalHandleBudgetRefusesBeforeSearchOpen(t *testing.T) {
	root := fakeDirectory("root")
	current := root
	var components []string
	for n := 1; n <= 64; n++ {
		name := fmt.Sprintf("ancestor%d", n)
		child := fakeDirectory(name)
		current.add(name, child)
		current = child
		components = append(components, name)
	}
	factory := newFakeFactory(pathPlan{rootLabel: "root", components: components}, root)
	_, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Inspect(context.Background(), "original")
	assertCode(t, err, transfer.ErrPathUnsupported)
	for _, op := range factory.ops {
		if op == "search:ancestor63" {
			t.Fatal("lexical guard acquired a forbidden search handle")
		}
	}
	assertFakeClosed(t, factory)
}

func TestPreparedDirectoryOwnsOnlyLazyPinAndRejectsReplacement(t *testing.T) {
	root := fakeDirectory("root")
	root.add("old.txt", fakeFile("old.txt", "original"))
	factory := newFakeFactory(pathPlan{rootLabel: "root"}, root)
	i := &Inspector{handles: factory, sameFile: sameFakeFile}
	prepared, err := i.PrepareDirectory(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	if factory.active != 1 {
		t.Fatalf("Prepare retained %d handles, want one owned search pin", factory.active)
	}
	for _, op := range factory.ops {
		if strings.HasPrefix(op, "enumerate:") {
			t.Fatal("Prepare enumerated a directory")
		}
	}
	replacement := fakeDirectory("root")
	replacement.add("new.txt", fakeFile("new.txt", "replacement"))
	factory.root = replacement
	visits := 0
	err = prepared.Walk(context.Background(), func(transfer.SourceEntry, io.Reader) error { visits++; return nil })
	assertCode(t, err, transfer.ErrSourceChanged)
	if visits != 0 {
		t.Fatal("prepared walk visited replacement entries")
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	assertFakeClosed(t, factory)
	err = prepared.Walk(context.Background(), func(transfer.SourceEntry, io.Reader) error { return nil })
	if !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("Walk after Close = %v, want closed", err)
	}
}

func TestPreparedDirectoryNativeReplacementNeverReadsNewBytes(t *testing.T) {
	base := fixtureDir(t)
	path := filepath.Join(base, "selected")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	prepared, err := New().PrepareDirectory(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	if err := os.Rename(path, filepath.Join(base, "original")); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "secret"), []byte("replacement bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	visits := 0
	err = prepared.Walk(context.Background(), func(transfer.SourceEntry, io.Reader) error { visits++; return nil })
	assertCode(t, err, transfer.ErrSourceChanged)
	if visits != 0 {
		t.Fatal("native prepared walk visited replacement bytes")
	}
}

func TestSourceArithmeticAndBatchFaultsArePhaseCorrect(t *testing.T) {
	for _, fault := range []string{"negative", "overflow", "batch", "root-file"} {
		t.Run(fault, func(t *testing.T) {
			root := fakeDirectory("root")
			switch fault {
			case "negative":
				root.add("file", fakeRegular("file", -1))
			case "overflow":
				root.add("first", fakeRegular("first", 1<<63-1))
				root.add("second", fakeRegular("second", 1))
			case "batch":
				root.oversizedBatch = true
			case "root-file":
				root.add("file", fakeRegular("file", -1))
			}
			plan := pathPlan{rootLabel: "root"}
			if fault == "root-file" {
				plan.components = []string{"file"}
			}
			factory := newFakeFactory(plan, root)
			inspector := &Inspector{handles: factory, sameFile: sameFakeFile}
			_, err := inspector.Inspect(context.Background(), "original")
			assertCode(t, err, transfer.ErrSetupFailed)
			assertFakeClosed(t, factory)
			if fault != "root-file" {
				err = inspector.Walk(context.Background(), "original", func(transfer.SourceEntry, io.Reader) error { return nil })
				assertCode(t, err, transfer.ErrTransferFailed)
				assertFakeClosed(t, factory)
			}
		})
	}
}

type blockingBorrowedHandle struct {
	entered     chan struct{}
	resume      chan struct{}
	reading     atomic.Bool
	unsafeClose atomic.Bool
}

func (h *blockingBorrowedHandle) Stat() (fs.FileInfo, error) { return nil, nil }
func (h *blockingBorrowedHandle) Read(p []byte) (int, error) {
	h.reading.Store(true)
	close(h.entered)
	<-h.resume
	h.reading.Store(false)
	return 0, io.EOF
}
func (h *blockingBorrowedHandle) Close() error { h.unsafeClose.Store(h.reading.Load()); return nil }

func TestBorrowedReaderRevocationJoinsAnInFlightRead(t *testing.T) {
	h := &blockingBorrowedHandle{entered: make(chan struct{}), resume: make(chan struct{})}
	b := &borrowedContent{handle: h}
	readDone := make(chan struct{})
	go func() { _, _ = b.Read(make([]byte, 1)); close(readDone) }()
	<-h.entered
	if b.mu.TryLock() {
		b.mu.Unlock()
		close(h.resume)
		<-readDone
		t.Fatal("borrowed Read did not retain the revocation lock during native I/O")
	}
	released := make(chan struct{})
	go func() { b.release(); _ = h.Close(); close(released) }()
	close(h.resume)
	<-readDone
	<-released
	if h.unsafeClose.Load() {
		t.Fatal("owned close raced an in-flight borrowed read")
	}
	if n, err := b.Read(make([]byte, 1)); n != 0 || !errors.Is(err, fs.ErrClosed) {
		t.Fatalf("revoked reader returned %d, %v", n, err)
	}
}

func TestPreparedCloseJoinsActiveWalk(t *testing.T) {
	root := fakeDirectory("root")
	root.add("file", fakeFile("file", "content"))
	factory := newFakeFactory(pathPlan{rootLabel: "root"}, root)
	prepared, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).PrepareDirectory(context.Background(), "original")
	if err != nil {
		t.Fatal(err)
	}
	p := prepared.(*preparedDirectory)
	entered, resume, finished := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		finished <- p.Walk(context.Background(), func(transfer.SourceEntry, io.Reader) error { close(entered); <-resume; return nil })
	}()
	<-entered
	if p.mu.TryLock() {
		p.mu.Unlock()
		close(resume)
		<-finished
		t.Fatal("prepared Walk did not retain ownership through visitor return")
	}
	closed := make(chan error, 1)
	go func() { closed <- p.Close() }()
	close(resume)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	if err := <-closed; err != nil {
		t.Fatal(err)
	}
	assertFakeClosed(t, factory)
}

func TestWalkRevokesBorrowBeforeOwnedClose(t *testing.T) {
	root := fakeDirectory("root")
	root.add("file", fakeFile("file", "content"))
	factory := newFakeFactory(pathPlan{rootLabel: "root"}, root)
	var kept io.Reader
	checked := false
	factory.onOperation = func(op string) {
		if op == "close-content:file" {
			checked = true
			if kept == nil {
				t.Fatal("owned close preceded visitor")
			}
			if n, err := kept.Read(make([]byte, 1)); n != 0 || !errors.Is(err, fs.ErrClosed) {
				t.Fatalf("visitor-return wiring did not revoke before owned close: read=%d error=%v", n, err)
			}
		}
	}
	err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Walk(context.Background(), "original", func(_ transfer.SourceEntry, content io.Reader) error { kept = content; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Fatal("no owned close reached")
	}
	assertFakeClosed(t, factory)
}

func TestSourceRejectsPortableNamesDuringInspectionAndWalk(t *testing.T) {
	for _, name := range []string{"NUL .txt", "COM¹.txt", "LPT³", "bad\u202e.txt", "bad\x01.txt", "bad<", "bad>", "bad|", "bad?", "bad*", "bad:", `bad"`, "tail.", "tail "} {
		t.Run(name, func(t *testing.T) {
			root := fakeDirectory("root")
			root.add(name, fakeFile(name, "data"))
			factory := newFakeFactory(pathPlan{rootLabel: "root"}, root)
			i := &Inspector{handles: factory, sameFile: sameFakeFile}
			_, err := i.Inspect(context.Background(), "original")
			assertCode(t, err, transfer.ErrPathUnsupported)
			seen := 0
			err = i.Walk(context.Background(), "original", func(transfer.SourceEntry, io.Reader) error { seen++; return nil })
			assertCode(t, err, transfer.ErrPathUnsupported)
			if seen != 0 {
				t.Fatal("unsafe source entry reached a visitor")
			}
			assertFakeClosed(t, factory)
		})
	}
}

func TestPreparedDirectoryClosesWithoutWalkingAndOnPreparationFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		root := fakeDirectory("root")
		if fail {
			root.closeErr = errors.New("close fixture")
		}
		factory := newFakeFactory(pathPlan{rootLabel: "root"}, root)
		p, err := (&Inspector{handles: factory, sameFile: sameFakeFile}).PrepareDirectory(context.Background(), "original")
		if fail {
			assertCode(t, err, transfer.ErrTransferFailed)
			if p != nil {
				t.Fatal("failed Prepare returned a live capability")
			}
		} else {
			if err != nil {
				t.Fatal(err)
			}
			if factory.active != 1 {
				t.Fatal("Prepare did not retain exactly one pin")
			}
			if err := p.Close(); err != nil {
				t.Fatal(err)
			}
			if err := p.Close(); err != nil {
				t.Fatal(err)
			}
		}
		assertFakeClosed(t, factory)
	}
}
