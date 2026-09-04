package stream

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

func TestPrepareDirectoryIsLazyAndReportsAnUnknownLength(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{"a.txt": "a", "nested/b.txt": "b"})
	staged := stage(t, root)

	counted := &countingSource{inner: source.New()}
	prepared, err := (&Payloads{source: counted}).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	defer func() {
		if err := prepared.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	// Preparation runs before the response headers, and a full traversal there
	// would stall the claim for the whole tree while learning nothing that can
	// be put in a header anyway.
	if inspects, walks := counted.counts(); inspects != 0 || walks != 0 {
		t.Fatalf("Prepare inspected %d times and walked %d times, want a lazy 0 and 0", inspects, walks)
	}
	total, known := prepared.Size()
	if known || total != 0 {
		t.Fatalf("Size() = (%d, %t), want the unknown (0, false) a streamed archive has", total, known)
	}
	if got, want := prepared.DownloadName(), filepath.Base(root)+".zip"; got != want {
		t.Fatalf("DownloadName() = %q, want %q", got, want)
	}
}

func TestWriteToProducesOneTopLevelRootWithAValidCentralDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"readme.txt":            "hello folder",
		"empty.txt":             "",
		"nested/deep/file.bin":  "deep bytes",
		"unicode ünï 🌍/note.md": "unicode entry",
	})
	if err := os.MkdirAll(filepath.Join(root, "empty-directory"), 0o700); err != nil {
		t.Fatal(err)
	}

	body := streamArchive(t, root)
	reader := openArchive(t, body)
	names := archiveNamesOf(reader)

	rootName := filepath.Base(root)
	for _, name := range names {
		if !strings.HasPrefix(name, rootName+"/") {
			t.Fatalf("entry %q is not under the single top-level root %q", name, rootName)
		}
		assertSafeEntryName(t, name)
	}
	if !contains(names, rootName+"/") {
		t.Fatalf("names = %v, want the top-level root entry itself", names)
	}
	if !contains(names, rootName+"/empty-directory/") {
		t.Fatalf("names = %v, want the empty directory preserved as an entry", names)
	}
	assertEntryContents(t, reader, rootName+"/readme.txt", "hello folder")
	assertEntryContents(t, reader, rootName+"/empty.txt", "")
	assertEntryContents(t, reader, rootName+"/nested/deep/file.bin", "deep bytes")
	assertEntryContents(t, reader, rootName+"/unicode ünï 🌍/note.md", "unicode entry")
}

// A ZIP that only archive/zip can open proves less than one a second
// implementation accepts, because both would share the same misreading.
func TestStreamedArchiveOpensWithASecondImplementation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{"one.txt": "first", "two/three.txt": "third"})
	body := streamArchive(t, root)

	archivePath := filepath.Join(t.TempDir(), "streamed.zip")
	if err := os.WriteFile(archivePath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	tool, args := secondZipReader(t, archivePath)
	output, err := exec.Command(tool, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s could not read the streamed archive: %v\n%s", tool, err, output)
	}
	listing := string(output)
	rootName := filepath.Base(root)
	for _, want := range []string{rootName, "one.txt", "three.txt"} {
		if !strings.Contains(listing, want) {
			t.Fatalf("%s listing missing %q:\n%s", tool, want, listing)
		}
	}
}

func TestWriteToArchivesAnEmptyRootAsAFolder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	body := streamArchive(t, root)
	reader := openArchive(t, body)

	names := archiveNamesOf(reader)
	rootName := filepath.Base(root)
	if len(names) != 1 || names[0] != rootName+"/" {
		t.Fatalf("names = %v, want exactly the single root entry %q", names, rootName+"/")
	}
}

func TestWriteToAbortsOnAnEntryThatBecomesUnsafeMidStream(t *testing.T) {
	t.Parallel()

	entries := []transfer.SourceEntry{
		{RelativePath: "first.txt", Kind: transfer.ItemFile, Size: 5},
		{RelativePath: "second.txt", Kind: transfer.ItemFile, Size: 5},
	}
	emitted := 0
	walker := &scriptedSource{walk: func(_ context.Context, _ string, visit transfer.SourceVisitor) error {
		for index, entry := range entries {
			if index == 1 {
				// The second entry became a junction between preflight and now:
				// the walk refuses it rather than following it.
				return transfer.NewError(transfer.ErrPathUnsupported, "entry became link-like")
			}
			emitted++
			if err := visit(entry, strings.NewReader("first")); err != nil {
				return err
			}
		}
		return nil
	}}

	payload := newTestArchive(t, walker, "folder")
	destination := &recordingWriter{}
	err := payload.WriteTo(context.Background(), destination)
	assertCode(t, err, transfer.ErrPathUnsupported)
	if emitted != 1 {
		t.Fatalf("emitted %d entries, want the stream to stop before the unsafe one", emitted)
	}
	// The bytes already on the wire cannot be recalled, but the central
	// directory must never follow them: an archive that opens cleanly and
	// silently omits the refused entry is worse than one that fails to open.
	if _, err := zip.NewReader(bytes.NewReader(destination.bytes()), int64(destination.length())); err == nil {
		t.Fatal("aborted stream still produced a readable archive, so a refused entry would look like a complete download")
	}
}

func TestWriteToPropagatesAWalkFailureWithoutAppendingToTheBody(t *testing.T) {
	t.Parallel()

	for name, injected := range map[string]error{
		"missing":     transfer.NewError(transfer.ErrPathNotFound, "entry disappeared"),
		"unsupported": transfer.NewError(transfer.ErrPathUnsupported, "entry became special"),
	} {
		injected := injected
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			walker := &scriptedSource{walk: func(context.Context, string, transfer.SourceVisitor) error {
				return injected
			}}
			payload := newTestArchive(t, walker, "folder")
			destination := &recordingWriter{}

			err := payload.WriteTo(context.Background(), destination)
			assertCode(t, err, transfer.ErrorCodeOf(injected))
			if strings.Contains(string(destination.bytes()), "PK\x05\x06") {
				t.Fatal("a failed stream still emitted an end-of-central-directory record")
			}
		})
	}
}

func TestWriteToJoinsItsWorkerOnEveryExit(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"a.bin": strings.Repeat("a", 512*1024),
		"b.bin": strings.Repeat("b", 512*1024),
	})
	staged := stage(t, root)
	adapter := &Payloads{source: source.New(), bufferSize: 4096}

	baseline := runtime.NumGoroutine()

	completed, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if err := completed.WriteTo(context.Background(), io.Discard); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	if err := completed.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	cancelled, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	assertCode(t, cancelled.WriteTo(ctx, &countingWriter{onWrite: cancel}), transfer.ErrCancelled)
	if err := cancelled.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	disconnected, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	assertCode(t,
		disconnected.WriteTo(context.Background(), &countingWriter{err: errors.New("receiver went away")}),
		transfer.ErrTransferFailed,
	)
	if err := disconnected.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// The join is what this measures: a worker still building the archive would
	// keep its goroutine, its pipe, and its open entry alive past WriteTo.
	deadline := time.Now().Add(2 * time.Second)
	for {
		running := runtime.NumGoroutine()
		if running <= baseline {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d goroutines still running, want no more than the %d at baseline", running, baseline)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWriteToClosesEveryBorrowedEntryBeforeReturning(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{"a.txt": "aa", "b.txt": "bb", "nested/c.txt": "cc"})

	tracker := &borrowTracker{inner: source.New()}
	payload := newTestArchive(t, tracker, filepath.Base(root))
	payload.path = root

	if err := payload.WriteTo(context.Background(), io.Discard); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}

	if tracker.lent.Load() == 0 {
		t.Fatal("no reader was borrowed, so this proves nothing about releasing one")
	}
	// Each reader is invalidated when its visit returns, in the order the walk
	// visited: nothing the archive kept can still reach a live descriptor.
	for index, reader := range tracker.readers() {
		if _, err := reader.Read(make([]byte, 1)); err == nil {
			t.Fatalf("borrowed reader %d still reads after WriteTo returned", index)
		}
	}
}

func TestWriteToRefusesASecondCallAndACallAfterClose(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{"a.txt": "a"})
	staged := stage(t, root)

	streamed, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if err := streamed.WriteTo(context.Background(), io.Discard); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	assertCode(t, streamed.WriteTo(context.Background(), io.Discard), transfer.ErrTransferFailed)
	if err := streamed.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	released, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if err := released.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	assertCode(t, released.WriteTo(context.Background(), io.Discard), transfer.ErrTransferFailed)
	// A repeated Close neither races nor re-reports: the server owns exactly
	// one, and a payload that reported a second one would turn an ordinary
	// teardown into a transfer failure.
	if err := released.Close(); err != nil {
		t.Fatalf("repeated Close() error = %v", err)
	}
}

func TestCloseIsSafeConcurrentlyForADirectoryPayload(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTree(t, root, map[string]string{"a.txt": "a"})
	staged := stage(t, root)
	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	var group sync.WaitGroup
	errs := make([]error, 8)
	for index := range errs {
		group.Add(1)
		go func() {
			defer group.Done()
			errs[index] = prepared.Close()
		}()
	}
	group.Wait()
	for index, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Close() %d error = %v", index, err)
		}
	}
}

func TestWriteToStopsPromptlyWhenTheReceiverDisconnects(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	files := map[string]string{}
	for index := range 24 {
		files[string(rune('a'+index))+".bin"] = strings.Repeat("x", 128*1024)
	}
	writeTree(t, root, files)
	staged := stage(t, root)

	prepared, err := (&Payloads{source: source.New(), bufferSize: 4096}).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	defer func() { _ = prepared.Close() }()

	destination := &countingWriter{err: errors.New("receiver disconnected")}
	done := make(chan error, 1)
	go func() { done <- prepared.WriteTo(context.Background(), destination) }()
	select {
	case err := <-done:
		assertCode(t, err, transfer.ErrTransferFailed)
	case <-time.After(5 * time.Second):
		t.Fatal("WriteTo did not return after the destination failed")
	}
	if destination.writes != 1 {
		t.Fatalf("destination accepted %d writes after failing the first, want 1", destination.writes)
	}
}

func TestWriteToRejectsMissingContextOrDestinationForADirectory(t *testing.T) {
	t.Parallel()

	payload := newTestArchive(t, &scriptedSource{}, "folder")
	assertCode(t, payload.WriteTo(nil, io.Discard), transfer.ErrTransferFailed) //nolint:staticcheck // the nil context is the case under test
	assertCode(t, payload.WriteTo(context.Background(), nil), transfer.ErrTransferFailed)
}

func TestArchiveEntryNamesAreRelativeAndNeverEscapeTheRoot(t *testing.T) {
	t.Parallel()

	for name, relative := range map[string]string{
		"empty":            "",
		"absolute":         "/etc/passwd",
		"backslash":        `nested\file.txt`,
		"dot-dot":          "../escape.txt",
		"nested dot-dot":   "nested/../../escape.txt",
		"single dot":       "nested/./file.txt",
		"trailing slash":   "nested/",
		"double slash":     "nested//file.txt",
		"volume qualified": "C:/escape.txt",
		"nul":              "nested/\x00.txt",
	} {
		relative := relative
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if got, err := archiveEntryName("root", relative); err == nil {
				t.Fatalf("archiveEntryName(%q) = %q, want a refusal", relative, got)
			} else {
				assertCode(t, err, transfer.ErrPathUnsupported)
			}
		})
	}

	got, err := archiveEntryName("root", "nested/deep/file.txt")
	if err != nil {
		t.Fatalf("archiveEntryName(safe) error = %v", err)
	}
	if got != "root/nested/deep/file.txt" {
		t.Fatalf("archiveEntryName(safe) = %q, want it placed under the root", got)
	}
}

func TestArchiveRefusesAnEntryNameTheSourceShouldNeverEmit(t *testing.T) {
	t.Parallel()

	walker := &scriptedSource{walk: func(_ context.Context, _ string, visit transfer.SourceVisitor) error {
		return visit(transfer.SourceEntry{
			RelativePath: "../escaped.txt",
			Kind:         transfer.ItemFile,
		}, strings.NewReader("escape"))
	}}
	payload := newTestArchive(t, walker, "folder")
	assertCode(t, payload.WriteTo(context.Background(), io.Discard), transfer.ErrPathUnsupported)
}

func TestArchiveDownloadNameIsCappedAfterTheExtensionIsAppended(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		staged transfer.StagedItem
		want   string
	}{
		"ordinary": {
			staged: transfer.StagedItem{Name: "photos", Path: `C:\Users\a\photos`},
			want:   "photos.zip",
		},
		"sanitizes to nothing": {
			staged: transfer.StagedItem{Name: "..", Path: ".."},
			want:   fallbackDownloadName + archiveExtension,
		},
		"at the rune cap": {
			staged: transfer.StagedItem{Name: strings.Repeat("f", maxDownloadNameRunes)},
			want:   strings.Repeat("f", maxDownloadNameRunes-len(archiveExtension)) + archiveExtension,
		},
		"over the rune cap in wide runes": {
			staged: transfer.StagedItem{Name: strings.Repeat("🌍", maxDownloadNameRunes+50)},
			want:   strings.Repeat("🌍", maxDownloadNameRunes-len(archiveExtension)) + archiveExtension,
		},
	} {
		test := test
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root, download := archiveNames(test.staged)
			if download != test.want {
				t.Fatalf("download name = %q, want %q", download, test.want)
			}
			// The cap exists to bound the header value, so it is the name with
			// ".zip" on it that has to fit -- not the base before it.
			if runes := []rune(download); len(runes) > maxDownloadNameRunes {
				t.Fatalf("download name is %d runes, want at most %d", len(runes), maxDownloadNameRunes)
			}
			if !strings.HasSuffix(download, archiveExtension) {
				t.Fatalf("download name %q lost its extension to the cap", download)
			}
			assertHeaderSafeName(t, download)
			if root+archiveExtension != download {
				t.Fatalf("archive root %q and download name %q disagree", root, download)
			}
			if root == "" || strings.ContainsAny(root, `/\`) {
				t.Fatalf("archive root %q is not a single safe top-level name", root)
			}
		})
	}
}

func TestPrepareRejectsARootThatIsNoLongerADirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	staged := stage(t, root)
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, []byte("now a file"), 0o600); err != nil {
		t.Fatal(err)
	}

	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrSourceChanged)
	assertNoDisclosure(t, err, root)
}

func TestPrepareRejectsARootThatDisappeared(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	staged := stage(t, root)
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}

	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrPathNotFound)
	assertNoDisclosure(t, err, root)
}

// The claim-time Lstat is the only check a directory gets before headers, so a
// root swapped for a link between staging and claim has to be refused there,
// with the code the contract promises rather than one that merely happens to
// stop the transfer.
func TestPrepareRejectsALinkLikeRootWithPathUnsupported(t *testing.T) {
	t.Parallel()

	for name, kind := range map[string]struct{ symlink, reparse bool }{
		"symlink": {symlink: true},
		"reparse": {reparse: true},
	} {
		kind := kind
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			staged := transfer.StagedItem{
				Path: filepath.Join(t.TempDir(), "folder"),
				Name: "folder",
				Kind: transfer.ItemDirectory,
			}
			mode := fs.ModeDir
			if kind.symlink {
				mode = fs.ModeSymlink
			}
			adapter := &Payloads{
				source: sourceFunc(matchingSource(staged)),
				lstat: func(string) (fs.FileInfo, error) {
					return fakeFileInfo{name: staged.Name, mode: mode, reparse: kind.reparse}, nil
				},
			}

			prepared, err := adapter.Prepare(context.Background(), staged)
			assertNoPayload(t, prepared, err, transfer.ErrPathUnsupported)
		})
	}
}

func TestPrepareRejectsALinkLikeFileRootWithPathUnsupported(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "report.pdf", 4)
	adapter := &Payloads{
		source: matchingSource(staged),
		lstat: func(string) (fs.FileInfo, error) {
			// Kind, size, and modtime all still match the staged snapshot: only
			// the native reparse attribute reveals the swap.
			return fakeFileInfo{
				name:    staged.Name,
				size:    staged.LogicalSize,
				modTime: staged.ModTime,
				reparse: true,
			}, nil
		},
		open: func(string) (payloadFile, error) {
			t.Error("open ran for a link-like source")
			return nil, errors.New("must not run")
		},
		sameFile: matchedIdentity,
	}

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrPathUnsupported)
}

func TestPrepareHonorsCancellationForADirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	staged := stage(t, root)
	prepared, err := New(source.New()).Prepare(cancelledContext(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrCancelled)
}

func TestArchiveStreamingErrorsDoNotDiscloseTheSourcePath(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "private folder name")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTree(t, root, map[string]string{"secret.txt": "secret"})
	staged := stage(t, root)

	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	defer func() { _ = prepared.Close() }()

	err = prepared.WriteTo(context.Background(), &countingWriter{err: errors.New("destination gone")})
	assertNoDisclosure(t, err, root)
}

// ---------------------------------------------------------------- test seams

// scriptedSource drives the archive through the port instead of the filesystem,
// so a test can produce an entry sequence the filesystem would not hold still
// for -- an entry that turns unsafe halfway through, or a name the source
// should never emit at all.
type scriptedSource struct {
	inspect func(ctx context.Context, absolutePath string) (transfer.StagedItem, error)
	walk    func(ctx context.Context, absolutePath string, visit transfer.SourceVisitor) error
}

func (s *scriptedSource) Inspect(ctx context.Context, absolutePath string) (transfer.StagedItem, error) {
	if s.inspect != nil {
		return s.inspect(ctx, absolutePath)
	}
	return transfer.StagedItem{}, errors.New("inspect is not scripted")
}

func (s *scriptedSource) Walk(ctx context.Context, absolutePath string, visit transfer.SourceVisitor) error {
	if s.walk != nil {
		return s.walk(ctx, absolutePath, visit)
	}
	return nil
}

// countingSource records how often the real port is reached without changing
// what it does, so laziness is measured rather than asserted from structure.
type countingSource struct {
	inner transfer.SourcePort

	mu       sync.Mutex
	inspects int
	walks    int
}

func (c *countingSource) Inspect(ctx context.Context, absolutePath string) (transfer.StagedItem, error) {
	c.mu.Lock()
	c.inspects++
	c.mu.Unlock()
	return c.inner.Inspect(ctx, absolutePath)
}

func (c *countingSource) Walk(ctx context.Context, absolutePath string, visit transfer.SourceVisitor) error {
	c.mu.Lock()
	c.walks++
	c.mu.Unlock()
	return c.inner.Walk(ctx, absolutePath, visit)
}

func (c *countingSource) counts() (inspects, walks int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.inspects, c.walks
}

// borrowTracker keeps every reader the walk lent out so a test can prove each
// one stopped working when its visit returned.
type borrowTracker struct {
	inner transfer.SourcePort
	lent  atomic.Int64

	mu   sync.Mutex
	kept []io.Reader
}

func (b *borrowTracker) Inspect(ctx context.Context, absolutePath string) (transfer.StagedItem, error) {
	return b.inner.Inspect(ctx, absolutePath)
}

func (b *borrowTracker) Walk(ctx context.Context, absolutePath string, visit transfer.SourceVisitor) error {
	return b.inner.Walk(ctx, absolutePath, func(entry transfer.SourceEntry, content io.Reader) error {
		if content != nil {
			b.lent.Add(1)
			b.mu.Lock()
			b.kept = append(b.kept, content)
			b.mu.Unlock()
		}
		return visit(entry, content)
	})
}

func (b *borrowTracker) readers() []io.Reader {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]io.Reader(nil), b.kept...)
}

type recordingWriter struct {
	mu      sync.Mutex
	written []byte
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.written = append(w.written, p...)
	return len(p), nil
}

func (w *recordingWriter) bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.written...)
}

func (w *recordingWriter) length() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.written)
}

// -------------------------------------------------------------- test helpers

func newTestArchive(t *testing.T, port transfer.SourcePort, root string) *archive {
	t.Helper()
	return &archive{
		name:       root + archiveExtension,
		root:       root,
		path:       filepath.Join(t.TempDir(), root),
		modTime:    time.Unix(1_700_000_000, 0),
		source:     port,
		bufferSize: defaultBufferSize,
	}
}

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for relative, contents := range files {
		full := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func streamArchive(t *testing.T, root string) []byte {
	t.Helper()
	staged := stage(t, root)
	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	defer func() {
		if err := prepared.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()
	var body bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &body); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	return body.Bytes()
}

func openArchive(t *testing.T, body []byte) *zip.Reader {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("streamed archive has no readable central directory: %v", err)
	}
	return reader
}

func archiveNamesOf(reader *zip.Reader) []string {
	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		names = append(names, file.Name)
	}
	sort.Strings(names)
	return names
}

func assertEntryContents(t *testing.T, reader *zip.Reader, name, want string) {
	t.Helper()
	for _, file := range reader.File {
		if file.Name != name {
			continue
		}
		opened, err := file.Open()
		if err != nil {
			t.Fatalf("open %q: %v", name, err)
		}
		defer func() { _ = opened.Close() }()
		got, err := io.ReadAll(opened)
		if err != nil {
			t.Fatalf("read %q: %v", name, err)
		}
		if string(got) != want {
			t.Fatalf("entry %q = %q, want %q", name, got, want)
		}
		return
	}
	t.Fatalf("entry %q is missing from the archive", name)
}

func assertSafeEntryName(t *testing.T, name string) {
	t.Helper()
	if name == "" {
		t.Fatal("archive holds an empty entry name")
	}
	if strings.HasPrefix(name, "/") || strings.Contains(name, `\`) || filepath.VolumeName(name) != "" {
		t.Fatalf("entry name %q is absolute or volume-qualified", name)
	}
	for _, segment := range strings.Split(strings.TrimSuffix(name, "/"), "/") {
		if segment == "" || segment == "." || segment == ".." {
			t.Fatalf("entry name %q carries a traversal element", name)
		}
	}
	if path.Clean(name) != strings.TrimSuffix(name, "/") {
		t.Fatalf("entry name %q is not already in its cleaned form", name)
	}
}

func contains(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}

func cancelledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// secondZipReader finds a ZIP implementation that is not archive/zip, so the
// central-directory proof does not rest on the same code that wrote it.
func secondZipReader(t *testing.T, archivePath string) (string, []string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if shell, err := exec.LookPath("powershell.exe"); err == nil {
			script := "$ErrorActionPreference='Stop';" +
				"Add-Type -AssemblyName System.IO.Compression.FileSystem;" +
				"$zip=[System.IO.Compression.ZipFile]::OpenRead('" + archivePath + "');" +
				"$zip.Entries | ForEach-Object { $_.FullName };" +
				"$zip.Dispose()"
			return shell, []string{"-NoProfile", "-NonInteractive", "-Command", script}
		}
	}
	if unzip, err := exec.LookPath("unzip"); err == nil {
		return unzip, []string{"-l", archivePath}
	}
	t.Skip("no second ZIP implementation available on this host")
	return "", nil
}
