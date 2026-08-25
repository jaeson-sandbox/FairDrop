package stream

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

func TestPrepareOpensRegularFileAndReportsKnownLength(t *testing.T) {
	t.Parallel()

	path := writeFile(t, "report 🌍 日本語.txt", []byte("hello, FairDrop"))
	staged := stage(t, path)

	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	if got := prepared.DownloadName(); got != filepath.Base(path) {
		t.Fatalf("DownloadName() = %q, want %q", got, filepath.Base(path))
	}
	size, known := prepared.Size()
	if !known {
		t.Fatal("Size() reported an unknown length for a regular file")
	}
	if size != int64(len("hello, FairDrop")) {
		t.Fatalf("Size() = %d, want %d", size, len("hello, FairDrop"))
	}
}

func TestPrepareReportsKnownZeroLengthForEmptyFile(t *testing.T) {
	t.Parallel()

	staged := stage(t, writeFile(t, "empty.bin", nil))

	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	size, known := prepared.Size()
	if !known || size != 0 {
		t.Fatalf("Size() = (%d, %t), want (0, true)", size, known)
	}
}

func TestPrepareRejectsMissingSource(t *testing.T) {
	t.Parallel()

	path := writeFile(t, "removed-after-staging.bin", []byte("data"))
	staged := stage(t, path)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrPathNotFound)
	assertNoDisclosure(t, err, path)
}

func TestPrepareRejectsSourceThatDisappearsBetweenValidationAndOpen(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "raced.bin", 4)
	cause := &os.PathError{Op: "open", Path: staged.Path, Err: fs.ErrNotExist}
	adapter := syntheticAdapter(staged, failingOpen(cause))

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrPathNotFound)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("open cause is not preserved through Unwrap")
	}
	assertNoDisclosure(t, err, staged.Path)
}

func TestPrepareRejectsSourceWhoseIdentityCannotBePinned(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "unpinnable.bin", 4)
	cause := &os.PathError{Op: "lstat", Path: staged.Path, Err: fs.ErrPermission}
	adapter := syntheticAdapter(staged, func(string) (payloadFile, error) {
		t.Error("open ran after the identity Lstat failed")
		return nil, errors.New("must not run")
	})
	adapter.lstat = func(string) (fs.FileInfo, error) { return nil, cause }

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrTransferFailed)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatal("lstat cause is not preserved through Unwrap")
	}
	assertNoDisclosure(t, err, staged.Path)
}

func TestPrepareRejectsChangedSource(t *testing.T) {
	t.Parallel()

	tests := map[string]func(t *testing.T, path string){
		"size-and-content": func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("a much longer replacement"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"modtime-only": func(t *testing.T, path string) {
			shifted := time.Now().Add(-90 * time.Minute)
			if err := os.Chtimes(path, shifted, shifted); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := writeFile(t, "mutated.bin", []byte("original"))
			staged := stage(t, path)
			mutate(t, path)

			prepared, err := New(source.New()).Prepare(context.Background(), staged)
			assertNoPayload(t, prepared, err, transfer.ErrSourceChanged)
			assertNoDisclosure(t, err, path)
		})
	}
}

// The wire length must come from the opened descriptor, never from
// StagedItem.LogicalSize. Here the two diverge and the source port is seamed to
// report a match, so only the descriptor's own Stat can notice: an
// implementation that trusted LogicalSize would prepare successfully and
// promise a Content-Length the file cannot satisfy.
func TestPrepareDerivesWireLengthFromDescriptorNotStagedMetadata(t *testing.T) {
	t.Parallel()

	path := writeFile(t, "diverged.bin", bytes.Repeat([]byte("x"), 4096))
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	staged := transfer.StagedItem{
		Path:        path,
		Name:        filepath.Base(path),
		Kind:        transfer.ItemFile,
		LogicalSize: 11,
		ModTime:     info.ModTime(),
	}
	adapter := &Payloads{source: matchingSource(staged)}

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrSourceChanged)
}

// A swap inside the window between validation and open is caught by the
// descriptor, not by the path: the seam hands back a descriptor for different
// bytes than the validated path names.
func TestPrepareRejectsDescriptorSwappedAfterValidation(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "expected.bin", 8)
	decoy := &fakeFile{data: bytes.Repeat([]byte("y"), 512), info: fakeFileInfo{
		name:    "decoy.bin",
		size:    512,
		modTime: staged.ModTime,
	}}
	adapter := syntheticAdapter(staged, openFake(decoy))

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrSourceChanged)
	if decoy.closeCount() != 1 {
		t.Fatalf("swapped descriptor closed %d times, want 1", decoy.closeCount())
	}
}

// Kind, size, and modtime are forgeable together, so a replacement that copies
// all three is metadata-identical to the staged snapshot. Only the filesystem
// identity separates them. The open seam performs the swap inside the window an
// Lstat cannot cover; os.SameFile itself is the real one.
func TestPrepareRejectsReplacementThatForgesKindSizeAndModtime(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "original.bin")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged := stage(t, path)

	forged := filepath.Join(directory, "forged.bin")
	if err := os.WriteFile(forged, []byte("REPLACED"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(forged, staged.ModTime, staged.ModTime); err != nil {
		t.Fatal(err)
	}
	forgedInfo, err := os.Lstat(forged)
	if err != nil {
		t.Fatal(err)
	}
	if forgedInfo.Size() != staged.LogicalSize || !forgedInfo.ModTime().Equal(staged.ModTime) {
		t.Fatalf("fixture is not metadata-identical: %d/%v against staged %d/%v",
			forgedInfo.Size(), forgedInfo.ModTime(), staged.LogicalSize, staged.ModTime)
	}

	adapter := &Payloads{source: source.New(), open: func(string) (payloadFile, error) {
		file, err := os.Open(forged)
		if err != nil {
			return nil, err
		}
		return file, nil
	}}

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrSourceChanged)
}

// The identity compared must be the one that was pinned before the open and the
// one carried by the descriptor being streamed, not any other pair.
func TestPrepareComparesThePinnedIdentityAgainstTheDescriptor(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "identity.bin", 4)
	pinned := fakeFileInfo{name: "pinned", size: 4, modTime: staged.ModTime}
	opened := &fakeFile{data: []byte("abcd"), info: fakeFileInfo{
		name:    "descriptor",
		size:    4,
		modTime: staged.ModTime,
	}}
	var comparisons [][2]string
	adapter := syntheticAdapter(staged, openFake(opened))
	adapter.lstat = func(string) (fs.FileInfo, error) { return pinned, nil }
	adapter.sameFile = func(before, after fs.FileInfo) bool {
		comparisons = append(comparisons, [2]string{before.Name(), after.Name()})
		return false
	}

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrSourceChanged)
	if len(comparisons) != 1 || comparisons[0] != [2]string{"pinned", "descriptor"} {
		t.Fatalf("identity comparisons = %v, want one pinned-against-descriptor pair", comparisons)
	}
	if opened.closeCount() != 1 {
		t.Fatalf("descriptor closed %d times, want 1", opened.closeCount())
	}
}

func TestPrepareRejectsSourceThatBecameLinkLike(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "linked.bin")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	staged := stage(t, path)

	target := filepath.Join(directory, "target.bin")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}

	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrPathUnsupported)
	assertNoDisclosure(t, err, path)
}

func TestPrepareRejectsDescriptorThatIsNotRegular(t *testing.T) {
	t.Parallel()

	modes := map[string]fs.FileMode{
		"directory":  os.ModeDir,
		"symlink":    os.ModeSymlink,
		"device":     os.ModeDevice,
		"named-pipe": os.ModeNamedPipe,
		"irregular":  os.ModeIrregular,
	}
	for name, mode := range modes {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			staged := fabricatedItem(t, "special.bin", 0)
			opened := &fakeFile{info: fakeFileInfo{
				name:    staged.Name,
				mode:    mode,
				modTime: staged.ModTime,
			}}
			adapter := syntheticAdapter(staged, openFake(opened))

			prepared, err := adapter.Prepare(context.Background(), staged)
			assertNoPayload(t, prepared, err, transfer.ErrPathUnsupported)
			if opened.closeCount() != 1 {
				t.Fatalf("descriptor closed %d times, want 1", opened.closeCount())
			}
		})
	}
}

func TestPrepareRefusesDirectoryItemWithoutInspectingIt(t *testing.T) {
	t.Parallel()

	calls := 0
	adapter := &Payloads{
		source: sourceFunc(func(context.Context, string) (transfer.StagedItem, error) {
			calls++
			return transfer.StagedItem{}, errors.New("must not run")
		}),
		open: func(string) (payloadFile, error) {
			t.Error("open ran for a directory item")
			return nil, errors.New("must not run")
		},
	}
	staged := transfer.StagedItem{
		Path: filepath.Join(t.TempDir(), "folder"),
		Name: "folder",
		Kind: transfer.ItemDirectory,
	}

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrPathUnsupported)
	if calls != 0 {
		t.Fatalf("source inspected %d times for a directory item, want 0", calls)
	}
}

func TestPrepareRejectsUnreadableSource(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "unreadable.bin", 3)
	cause := &os.PathError{Op: "open", Path: staged.Path, Err: fs.ErrPermission}
	adapter := syntheticAdapter(staged, failingOpen(cause))

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrTransferFailed)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatal("open cause is not preserved through Unwrap")
	}
	assertNoDisclosure(t, err, staged.Path)
}

func TestPrepareReleasesDescriptorWhenDescriptorMetadataFails(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "statless.bin", 3)
	cause := errors.New("descriptor metadata unavailable")
	opened := &fakeFile{statErr: cause}
	adapter := syntheticAdapter(staged, openFake(opened))

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrTransferFailed)
	if !errors.Is(err, cause) {
		t.Fatal("stat cause is not preserved through Unwrap")
	}
	if opened.closeCount() != 1 {
		t.Fatalf("descriptor closed %d times, want 1", opened.closeCount())
	}
}

func TestPreparePropagatesSourcePortFailureUnchanged(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "unsupported.bin", 3)
	cause := transfer.NewError(transfer.ErrPathUnsupported, "selection traverses a reparse point")
	adapter := &Payloads{
		source: sourceFunc(func(context.Context, string) (transfer.StagedItem, error) {
			return transfer.StagedItem{}, cause
		}),
		lstat: func(string) (fs.FileInfo, error) {
			t.Error("identity was pinned after source validation failed")
			return nil, errors.New("must not run")
		},
		open: func(string) (payloadFile, error) {
			t.Error("open ran after source validation failed")
			return nil, errors.New("must not run")
		},
	}

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrPathUnsupported)
	if !errors.Is(err, cause) {
		t.Fatal("source port error was not preserved")
	}
}

func TestPrepareHonorsCancellation(t *testing.T) {
	t.Parallel()

	t.Run("before-filesystem", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		staged := fabricatedItem(t, "cancelled.bin", 3)
		adapter := &Payloads{
			source: sourceFunc(func(context.Context, string) (transfer.StagedItem, error) {
				t.Error("source inspected after cancellation")
				return transfer.StagedItem{}, errors.New("must not run")
			}),
			open: func(string) (payloadFile, error) {
				t.Error("open ran after cancellation")
				return nil, errors.New("must not run")
			},
		}

		prepared, err := adapter.Prepare(ctx, staged)
		assertNoPayload(t, prepared, err, transfer.ErrCancelled)
		if !errors.Is(err, context.Canceled) {
			t.Fatal("context cancellation cause is not preserved")
		}
	})

	t.Run("after-open", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		staged := fabricatedItem(t, "cancelled-late.bin", 3)
		opened := &fakeFile{data: []byte("abc"), info: fakeFileInfo{
			name:    staged.Name,
			size:    3,
			modTime: staged.ModTime,
		}}
		adapter := syntheticAdapter(staged, func(string) (payloadFile, error) {
			cancel()
			return opened, nil
		})

		prepared, err := adapter.Prepare(ctx, staged)
		assertNoPayload(t, prepared, err, transfer.ErrCancelled)
		if opened.closeCount() != 1 {
			t.Fatalf("descriptor closed %d times after cancellation, want 1", opened.closeCount())
		}
	})
}

// An expired deadline is FairDrop's own timeout, not the user's cancel, and a
// UI that treats cancellation as a non-error outcome would swallow it.
func TestPrepareDistinguishesDeadlineExpiryFromCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	staged := fabricatedItem(t, "expired.bin", 3)
	adapter := syntheticAdapter(staged, func(string) (payloadFile, error) {
		t.Error("open ran after the deadline expired")
		return nil, errors.New("must not run")
	})

	prepared, err := adapter.Prepare(ctx, staged)
	assertNoPayload(t, prepared, err, transfer.ErrTransferFailed)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("deadline cause is not preserved through Unwrap")
	}
}

func TestPrepareRejectsNilContextWithoutTouchingTheFilesystem(t *testing.T) {
	t.Parallel()

	adapter := &Payloads{
		source: sourceFunc(func(context.Context, string) (transfer.StagedItem, error) {
			t.Error("source inspected without a context")
			return transfer.StagedItem{}, errors.New("must not run")
		}),
		open: func(string) (payloadFile, error) {
			t.Error("open ran without a context")
			return nil, errors.New("must not run")
		},
	}

	//nolint:staticcheck // the nil context is the boundary under test.
	prepared, err := adapter.Prepare(nil, fabricatedItem(t, "no-context.bin", 3))
	assertNoPayload(t, prepared, err, transfer.ErrTransferFailed)
}

// The two-layer defense only holds if every layer names the same object. A
// validator handed a cleaned path while the open call gets the raw string would
// check one file and stream another, so the seams assert their arguments.
func TestPrepareValidatesPinsAndOpensTheSameByteIdenticalPath(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	separator := string(os.PathSeparator)
	if err := os.Mkdir(filepath.Join(directory, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := directory + separator + "sub" + separator + ".." + separator + "unclean 🌍 name.bin"
	if filepath.Clean(path) == path {
		t.Fatal("fixture path is already clean, so this test could not detect rewriting")
	}
	contents := []byte("payload")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	staged := stage(t, path)
	if staged.Path != path {
		t.Fatalf("staged path = %q, want byte-identical %q", staged.Path, path)
	}

	var inspected, pinned, opened []string
	adapter := &Payloads{
		source: sourceFunc(func(ctx context.Context, candidate string) (transfer.StagedItem, error) {
			inspected = append(inspected, candidate)
			return source.New().Inspect(ctx, candidate)
		}),
		lstat: func(candidate string) (fs.FileInfo, error) {
			pinned = append(pinned, candidate)
			return os.Lstat(candidate)
		},
		open: func(candidate string) (payloadFile, error) {
			opened = append(opened, candidate)
			file, err := os.Open(candidate)
			if err != nil {
				return nil, err
			}
			return file, nil
		},
	}

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	for name, calls := range map[string][]string{"inspect": inspected, "lstat": pinned, "open": opened} {
		if len(calls) != 1 {
			t.Fatalf("%s received %d calls, want exactly 1: %q", name, len(calls), calls)
		}
		if calls[0] != path {
			t.Fatalf("%s received %q, want byte-identical %q", name, calls[0], path)
		}
	}
}

func TestPrepareAndStreamSupportLongPaths(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	for len(directory) < 280 {
		directory = filepath.Join(directory, strings.Repeat("deep-ünïcode", 3))
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Skipf("long path fixture unavailable: %v", err)
	}
	path := filepath.Join(directory, "long path 🌍.bin")
	contents := bytes.Repeat([]byte("long"), 1024)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Skipf("long path fixture unavailable: %v", err)
	}

	staged, err := source.New().Inspect(context.Background(), path)
	if err != nil {
		t.Skipf("long path is not supported by this host: %v", err)
	}
	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v (code %q)", err, transfer.ErrorCodeOf(err))
	}
	t.Cleanup(func() { _ = prepared.Close() })

	var destination bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &destination); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	if !bytes.Equal(destination.Bytes(), contents) {
		t.Fatal("long-path payload did not arrive byte-identical")
	}
}

// StagedItem.Name is sender-controlled data that ends up in a response header,
// so it is reduced to a bare basename: no separator, no "..", no control
// character. CR/LF in particular is a header-injection primitive.
func TestDownloadNameIsReducedToASafeBasename(t *testing.T) {
	t.Parallel()

	sourcePath := filepath.Join(t.TempDir(), "actual name.bin")
	tests := map[string]struct {
		name string
		want string
	}{
		"windows-traversal":  {name: `..\..\Windows\System32\evil.exe`, want: "evil.exe"},
		"posix-traversal":    {name: "../../etc/passwd", want: "passwd"},
		"header-injection":   {name: "report.pdf\r\nX-Injected: 1", want: "report.pdfX-Injected: 1"},
		"embedded-null":      {name: "report\x00.pdf", want: "report.pdf"},
		"dot-dot-only":       {name: "..", want: filepath.Base(sourcePath)},
		"dot-only":           {name: ".", want: filepath.Base(sourcePath)},
		"empty":              {name: "", want: filepath.Base(sourcePath)},
		"trailing-dots":      {name: "archive.tar...", want: "archive.tar"},
		"leading-dot-is-ok":  {name: ".gitignore", want: ".gitignore"},
		"unicode-is-kept":    {name: "報告 🌍.pdf", want: "報告 🌍.pdf"},
		"separator-only":     {name: `\\`, want: filepath.Base(sourcePath)},
		"trailing-separator": {name: `folder\`, want: filepath.Base(sourcePath)},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := downloadName(transfer.StagedItem{Path: sourcePath, Name: test.name})
			if got != test.want {
				t.Fatalf("downloadName(%q) = %q, want %q", test.name, got, test.want)
			}
			assertHeaderSafeName(t, got)
		})
	}
}

func TestDownloadNameFallsBackWhenNothingUsableRemains(t *testing.T) {
	t.Parallel()

	got := downloadName(transfer.StagedItem{Path: `\\`, Name: ".."})
	if got != fallbackDownloadName {
		t.Fatalf("downloadName() = %q, want the %q fallback", got, fallbackDownloadName)
	}
	assertHeaderSafeName(t, got)
}

func TestPreparedPayloadExposesTheSanitizedName(t *testing.T) {
	t.Parallel()

	path := writeFile(t, "genuine.bin", []byte("payload"))
	staged := stage(t, path)
	staged.Name = `..\..\Windows\System32\evil.exe`

	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	if got := prepared.DownloadName(); got != "evil.exe" {
		t.Fatalf("DownloadName() = %q, want the sanitized %q", got, "evil.exe")
	}
	assertHeaderSafeName(t, prepared.DownloadName())
}

func TestWriteToStreamsExactBytesThroughTheReusedBuffer(t *testing.T) {
	t.Parallel()

	contents := make([]byte, 300*1024)
	for index := range contents {
		contents[index] = byte(index % 251)
	}
	path := writeFile(t, "exact 🌍.bin", contents)
	staged := stage(t, path)
	adapter := &Payloads{source: source.New(), bufferSize: 8 * 1024}

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	var destination bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &destination); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	if !bytes.Equal(destination.Bytes(), contents) {
		t.Fatalf("streamed %d bytes that differ from the source's %d", destination.Len(), len(contents))
	}
	size, _ := prepared.Size()
	if int64(destination.Len()) != size {
		t.Fatalf("streamed %d bytes, want the advertised wire length %d", destination.Len(), size)
	}
}

func TestWriteToUsesOneBufferForTheWholeStream(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "buffered.bin", 40)
	opened := &fakeFile{data: bytes.Repeat([]byte("z"), 40), info: fakeFileInfo{
		name:    staged.Name,
		size:    40,
		modTime: staged.ModTime,
	}}
	adapter := syntheticAdapter(staged, openFake(opened))
	adapter.bufferSize = 8

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })
	if err := prepared.WriteTo(context.Background(), io.Discard); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}

	buffers := opened.readBuffers()
	if len(buffers) < 5 {
		t.Fatalf("read %d times, want at least 5 chunks", len(buffers))
	}
	for index, buffer := range buffers {
		if buffer.pointer != buffers[0].pointer {
			t.Fatalf("read %d used a different buffer than the first read", index)
		}
		if buffer.length != 8 {
			t.Fatalf("read %d used a %d-byte buffer, want the fixed 8", index, buffer.length)
		}
	}
}

func TestWriteToEmptyPayloadWritesNothing(t *testing.T) {
	t.Parallel()

	staged := stage(t, writeFile(t, "empty-stream.bin", nil))
	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	destination := &countingWriter{}
	if err := prepared.WriteTo(context.Background(), destination); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	if destination.writes != 0 || destination.bytes != 0 {
		t.Fatalf("empty payload produced %d writes / %d bytes, want 0/0", destination.writes, destination.bytes)
	}
}

// Size is the promise Content-Length is built from: a source that grew after
// Prepare must be cut at the advertised length rather than overrunning the
// header the receiver is already parsing against.
func TestWriteToStopsAtTheAdvertisedLengthWhenTheSourceGrew(t *testing.T) {
	t.Parallel()

	path := writeFile(t, "grown.bin", []byte("SIXTEEN BYTES.\r\n"))
	staged := stage(t, path)
	adapter := &Payloads{source: source.New(), bufferSize: 4}

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })
	advertised, _ := prepared.Size()

	appended, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Skipf("append fixture unavailable: %v", err)
	}
	if _, err := appended.Write(bytes.Repeat([]byte("A"), 1024)); err != nil {
		t.Fatal(err)
	}
	if err := appended.Close(); err != nil {
		t.Fatal(err)
	}

	var destination bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &destination); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	if int64(destination.Len()) != advertised {
		t.Fatalf("streamed %d bytes under an advertised length of %d", destination.Len(), advertised)
	}
	if bytes.Contains(destination.Bytes(), []byte("A")) {
		t.Fatal("streamed bytes appended after Prepare")
	}
}

// The mirror case: a short body must never be reported as success, because the
// connection-abort defense Story 1.4 owns keys on a non-nil error.
func TestWriteToFailsWhenTheSourceDeliversFewerBytesThanAdvertised(t *testing.T) {
	t.Parallel()

	t.Run("truncated-on-disk", func(t *testing.T) {
		t.Parallel()
		path := writeFile(t, "truncated.bin", bytes.Repeat([]byte("t"), 4096))
		staged := stage(t, path)
		adapter := &Payloads{source: source.New(), bufferSize: 256}

		prepared, err := adapter.Prepare(context.Background(), staged)
		if err != nil {
			t.Fatalf("Prepare() error = %v", err)
		}
		t.Cleanup(func() { _ = prepared.Close() })
		advertised, _ := prepared.Size()

		if err := os.Truncate(path, 512); err != nil {
			t.Skipf("truncation fixture unavailable: %v", err)
		}

		destination := &countingWriter{}
		err = prepared.WriteTo(context.Background(), destination)
		assertCode(t, err, transfer.ErrTransferFailed)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("short body error = %v, want it to wrap io.ErrUnexpectedEOF", err)
		}
		if int64(destination.bytes) >= advertised {
			t.Fatalf("destination received %d bytes, want fewer than the advertised %d", destination.bytes, advertised)
		}
		assertNoDisclosure(t, err, path)
	})

	t.Run("descriptor-ends-early", func(t *testing.T) {
		t.Parallel()
		staged := fabricatedItem(t, "short.bin", 64)
		opened := &fakeFile{data: bytes.Repeat([]byte("s"), 20), info: fakeFileInfo{
			name:    staged.Name,
			size:    64,
			modTime: staged.ModTime,
		}}
		adapter := syntheticAdapter(staged, openFake(opened))
		adapter.bufferSize = 8

		prepared, err := adapter.Prepare(context.Background(), staged)
		if err != nil {
			t.Fatalf("Prepare() error = %v", err)
		}
		t.Cleanup(func() { _ = prepared.Close() })

		destination := &countingWriter{}
		err = prepared.WriteTo(context.Background(), destination)
		assertCode(t, err, transfer.ErrTransferFailed)
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("short body error = %v, want it to wrap io.ErrUnexpectedEOF", err)
		}
		if destination.bytes != 20 {
			t.Fatalf("destination received %d bytes, want the 20 the descriptor held", destination.bytes)
		}
	})
}

// A second copy would resume at the descriptor's current offset and, after a
// completed stream, write nothing while returning nil.
func TestWriteToRefusesASecondCall(t *testing.T) {
	t.Parallel()

	contents := []byte("streamed once")
	staged := stage(t, writeFile(t, "once.bin", contents))
	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	var first bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &first); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	if !bytes.Equal(first.Bytes(), contents) {
		t.Fatal("first stream did not arrive byte-identical")
	}

	second := &countingWriter{}
	assertCode(t, prepared.WriteTo(context.Background(), second), transfer.ErrTransferFailed)
	if second.writes != 0 {
		t.Fatalf("second WriteTo produced %d writes, want 0", second.writes)
	}
}

func TestWriteToStopsPromptlyOnCancellation(t *testing.T) {
	t.Parallel()

	contents := bytes.Repeat([]byte("c"), 64*1024)
	staged := stage(t, writeFile(t, "cancelled-stream.bin", contents))
	adapter := &Payloads{source: source.New(), bufferSize: 512}

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	destination := &countingWriter{onWrite: cancel}
	err = prepared.WriteTo(ctx, destination)

	assertCode(t, err, transfer.ErrCancelled)
	if !errors.Is(err, context.Canceled) {
		t.Fatal("context cancellation cause is not preserved")
	}
	if destination.writes != 1 {
		t.Fatalf("destination received %d writes after cancellation, want exactly 1", destination.writes)
	}
	if destination.bytes != 512 {
		t.Fatalf("destination received %d bytes, want one 512-byte chunk", destination.bytes)
	}
}

func TestWriteToDistinguishesDeadlineExpiryFromCancellation(t *testing.T) {
	t.Parallel()

	staged := stage(t, writeFile(t, "deadline.bin", bytes.Repeat([]byte("d"), 1024)))
	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	destination := &countingWriter{}
	err = prepared.WriteTo(ctx, destination)

	assertCode(t, err, transfer.ErrTransferFailed)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("deadline cause is not preserved through Unwrap")
	}
	if destination.writes != 0 {
		t.Fatalf("destination received %d writes after the deadline expired, want 0", destination.writes)
	}
}

func TestWriteToReportsDestinationFailureWithoutWritingFurther(t *testing.T) {
	t.Parallel()

	staged := stage(t, writeFile(t, "broken-destination.bin", bytes.Repeat([]byte("d"), 4096)))
	adapter := &Payloads{source: source.New(), bufferSize: 256}

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	cause := errors.New("connection reset by peer")
	destination := &countingWriter{err: cause}
	err = prepared.WriteTo(context.Background(), destination)

	assertCode(t, err, transfer.ErrTransferFailed)
	if !errors.Is(err, cause) {
		t.Fatal("destination cause is not preserved through Unwrap")
	}
	if destination.writes != 1 {
		t.Fatalf("destination received %d writes after failing, want exactly 1", destination.writes)
	}
}

func TestWriteToReportsShortWriteWithARecognizableCause(t *testing.T) {
	t.Parallel()

	staged := stage(t, writeFile(t, "short-write.bin", bytes.Repeat([]byte("s"), 1024)))
	adapter := &Payloads{source: source.New(), bufferSize: 256}

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	destination := &countingWriter{short: true}
	err = prepared.WriteTo(context.Background(), destination)
	assertCode(t, err, transfer.ErrTransferFailed)
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short-write error = %v, want it to wrap io.ErrShortWrite", err)
	}
	if destination.writes != 1 {
		t.Fatalf("destination received %d writes after a short write, want exactly 1", destination.writes)
	}
}

func TestWriteToReportsSourceReadFailureMidStream(t *testing.T) {
	t.Parallel()

	t.Run("error-after-partial-delivery", func(t *testing.T) {
		t.Parallel()
		staged := fabricatedItem(t, "unreadable-stream.bin", 24)
		cause := errors.New("source read failed")
		opened := &fakeFile{
			data:    bytes.Repeat([]byte("r"), 24),
			readErr: cause,
			failAt:  12,
			info:    fakeFileInfo{name: staged.Name, size: 24, modTime: staged.ModTime},
		}
		adapter := syntheticAdapter(staged, openFake(opened))
		adapter.bufferSize = 4

		prepared, err := adapter.Prepare(context.Background(), staged)
		if err != nil {
			t.Fatalf("Prepare() error = %v", err)
		}
		t.Cleanup(func() { _ = prepared.Close() })

		destination := &countingWriter{}
		err = prepared.WriteTo(context.Background(), destination)
		assertCode(t, err, transfer.ErrTransferFailed)
		if !errors.Is(err, cause) {
			t.Fatal("read cause is not preserved through Unwrap")
		}
		if destination.bytes != 12 {
			t.Fatalf("destination received %d bytes before the read failed, want 12", destination.bytes)
		}
	})

	// io.Reader is allowed to return bytes and an error together; those bytes
	// must still reach the destination before the failure is reported.
	t.Run("error-alongside-final-bytes", func(t *testing.T) {
		t.Parallel()
		staged := fabricatedItem(t, "bytes-with-error.bin", 24)
		cause := errors.New("source read failed with bytes in hand")
		opened := &fakeFile{
			data:      bytes.Repeat([]byte("r"), 24),
			readErr:   cause,
			failAt:    12,
			withBytes: true,
			info:      fakeFileInfo{name: staged.Name, size: 24, modTime: staged.ModTime},
		}
		adapter := syntheticAdapter(staged, openFake(opened))
		adapter.bufferSize = 4

		prepared, err := adapter.Prepare(context.Background(), staged)
		if err != nil {
			t.Fatalf("Prepare() error = %v", err)
		}
		t.Cleanup(func() { _ = prepared.Close() })

		destination := &countingWriter{}
		err = prepared.WriteTo(context.Background(), destination)
		assertCode(t, err, transfer.ErrTransferFailed)
		if !errors.Is(err, cause) {
			t.Fatal("read cause is not preserved through Unwrap")
		}
		if destination.bytes != 16 {
			t.Fatalf("destination received %d bytes, want the 16 delivered before and with the error", destination.bytes)
		}
	})
}

func TestWriteToRejectsMissingContextOrDestination(t *testing.T) {
	t.Parallel()

	staged := stage(t, writeFile(t, "guarded.bin", []byte("guard")))
	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	//nolint:staticcheck // the nil context is the boundary under test.
	assertCode(t, prepared.WriteTo(nil, io.Discard), transfer.ErrTransferFailed)
	assertCode(t, prepared.WriteTo(context.Background(), nil), transfer.ErrTransferFailed)
}

func TestWriteToAfterCloseFailsInsteadOfReadingAReleasedDescriptor(t *testing.T) {
	t.Parallel()

	staged := stage(t, writeFile(t, "closed-first.bin", []byte("closed")))
	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	destination := &countingWriter{}
	assertCode(t, prepared.WriteTo(context.Background(), destination), transfer.ErrTransferFailed)
	if destination.writes != 0 {
		t.Fatalf("destination received %d writes from a released payload, want 0", destination.writes)
	}
}

// Streaming and release failures wrap causes such as *os.PathError, which carry
// the selected path verbatim.
func TestStreamingAndReleaseErrorsDoNotDiscloseTheSourcePath(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "private-name.bin", 8)
	path := staged.Path

	t.Run("destination-failure", func(t *testing.T) {
		t.Parallel()
		opened := &fakeFile{data: []byte("12345678"), info: fakeFileInfo{
			name: staged.Name, size: 8, modTime: staged.ModTime,
		}}
		prepared := prepareSynthetic(t, staged, opened)
		destination := &countingWriter{err: &os.PathError{Op: "write", Path: path, Err: errors.New("boom")}}
		assertNoDisclosure(t, prepared.WriteTo(context.Background(), destination), path)
	})

	t.Run("source-read-failure", func(t *testing.T) {
		t.Parallel()
		opened := &fakeFile{
			data:    []byte("12345678"),
			readErr: &os.PathError{Op: "read", Path: path, Err: errors.New("boom")},
			info:    fakeFileInfo{name: staged.Name, size: 8, modTime: staged.ModTime},
		}
		prepared := prepareSynthetic(t, staged, opened)
		assertNoDisclosure(t, prepared.WriteTo(context.Background(), io.Discard), path)
	})

	t.Run("release-failure", func(t *testing.T) {
		t.Parallel()
		opened := &fakeFile{
			data:     []byte("12345678"),
			closeErr: &os.PathError{Op: "close", Path: path, Err: errors.New("boom")},
			info:     fakeFileInfo{name: staged.Name, size: 8, modTime: staged.ModTime},
		}
		prepared := prepareSynthetic(t, staged, opened)
		assertNoDisclosure(t, prepared.Close(), path)
	})
}

func TestCloseReleasesTheDescriptorExactlyOnce(t *testing.T) {
	t.Parallel()

	lifecycles := map[string]func(t *testing.T, prepared *payload){
		"without-writeto": func(*testing.T, *payload) {},
		"after-writeto": func(t *testing.T, prepared *payload) {
			if err := prepared.WriteTo(context.Background(), io.Discard); err != nil {
				t.Fatalf("WriteTo() error = %v", err)
			}
		},
		"after-failed-writeto": func(t *testing.T, prepared *payload) {
			err := prepared.WriteTo(context.Background(), &countingWriter{err: errors.New("gone")})
			assertCode(t, err, transfer.ErrTransferFailed)
		},
	}
	for name, run := range lifecycles {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			staged := fabricatedItem(t, "lifecycle.bin", 4)
			cause := errors.New("descriptor release failed")
			opened := &fakeFile{
				data:     []byte("abcd"),
				closeErr: cause,
				info:     fakeFileInfo{name: staged.Name, size: 4, modTime: staged.ModTime},
			}
			concrete := prepareSynthetic(t, staged, opened)
			run(t, concrete)

			first := concrete.Close()
			assertCode(t, first, transfer.ErrTransferFailed)
			if !errors.Is(first, cause) {
				t.Fatal("close cause is not preserved through Unwrap")
			}
			for repeat := range 3 {
				if err := concrete.Close(); err != nil {
					t.Fatalf("repeated Close() %d returned %v, want nil", repeat, err)
				}
			}
			if opened.closeCount() != 1 {
				t.Fatalf("descriptor closed %d times, want exactly 1", opened.closeCount())
			}
		})
	}
}

func TestCloseIsSafeWhenCalledConcurrently(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "concurrent-close.bin", 4)
	cause := errors.New("descriptor release failed")
	opened := &fakeFile{
		data:     []byte("abcd"),
		closeErr: cause,
		info:     fakeFileInfo{name: staged.Name, size: 4, modTime: staged.ModTime},
	}
	prepared := prepareSynthetic(t, staged, opened)

	const callers = 8
	results := make([]error, callers)
	var group sync.WaitGroup
	group.Add(callers)
	for index := range callers {
		go func() {
			defer group.Done()
			results[index] = prepared.Close()
		}()
	}
	group.Wait()

	reported := 0
	for _, result := range results {
		if result != nil {
			reported++
		}
	}
	if reported != 1 {
		t.Fatalf("%d concurrent Close calls reported the cause, want exactly 1", reported)
	}
	if opened.closeCount() != 1 {
		t.Fatalf("descriptor closed %d times, want exactly 1", opened.closeCount())
	}
}

// The adapter creates no goroutine, so neither a completed nor a cancelled
// stream may leave one behind. Deliberately sequential: goroutine counts are
// process-wide.
func TestStreamingLeavesNoGoroutineBehind(t *testing.T) {
	contents := bytes.Repeat([]byte("g"), 64*1024)
	staged := stage(t, writeFile(t, "goroutines.bin", contents))
	adapter := &Payloads{source: source.New(), bufferSize: 512}

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

func writeFile(t *testing.T, name string, contents []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func stage(t *testing.T, path string) transfer.StagedItem {
	t.Helper()
	item, err := source.New().Inspect(context.Background(), path)
	if err != nil {
		t.Fatalf("staging fixture failed: %v", err)
	}
	return item
}

// fabricatedItem is a staged snapshot for a path that is never touched: seam
// tests supply the validation result, the pinned identity, and the descriptor.
func fabricatedItem(t *testing.T, name string, size int64) transfer.StagedItem {
	t.Helper()
	return transfer.StagedItem{
		Path:        filepath.Join(t.TempDir(), name),
		Name:        name,
		Kind:        transfer.ItemFile,
		LogicalSize: size,
		ModTime:     time.Unix(1_700_000_000, 123),
	}
}

// syntheticAdapter drives Prepare entirely through seams: validation and the
// pinned identity report the staged snapshot back unchanged and the identities
// match, so a test overriding one seam isolates exactly the layer it is about.
func syntheticAdapter(item transfer.StagedItem, open openFunc) *Payloads {
	return &Payloads{
		source:   matchingSource(item),
		lstat:    matchingLstat(item),
		open:     open,
		sameFile: matchedIdentity,
	}
}

func prepareSynthetic(t *testing.T, item transfer.StagedItem, file *fakeFile) *payload {
	t.Helper()
	prepared, err := syntheticAdapter(item, openFake(file)).Prepare(context.Background(), item)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	concrete, ok := prepared.(*payload)
	if !ok {
		t.Fatalf("Prepare() returned %T, want *payload", prepared)
	}
	return concrete
}

type sourceFunc func(ctx context.Context, absolutePath string) (transfer.StagedItem, error)

func (f sourceFunc) Inspect(ctx context.Context, absolutePath string) (transfer.StagedItem, error) {
	return f(ctx, absolutePath)
}

// matchingSource reports the staged snapshot back unchanged, so path-level
// validation cannot be what catches a descriptor-level divergence.
func matchingSource(item transfer.StagedItem) sourceFunc {
	return func(context.Context, string) (transfer.StagedItem, error) { return item, nil }
}

func matchingLstat(item transfer.StagedItem) lstatFunc {
	return func(string) (fs.FileInfo, error) {
		return fakeFileInfo{name: item.Name, size: item.LogicalSize, modTime: item.ModTime}, nil
	}
}

// matchedIdentity stands in for os.SameFile, which cannot compare synthetic
// metadata. The real comparison is exercised against the filesystem in
// TestPrepareRejectsReplacementThatForgesKindSizeAndModtime.
func matchedIdentity(fs.FileInfo, fs.FileInfo) bool { return true }

func openFake(file *fakeFile) openFunc {
	return func(string) (payloadFile, error) { return file, nil }
}

func failingOpen(err error) openFunc {
	return func(string) (payloadFile, error) { return nil, err }
}

type readBuffer struct {
	pointer *byte
	length  int
}

type fakeFile struct {
	data      []byte
	info      fs.FileInfo
	statErr   error
	readErr   error
	closeErr  error
	failAt    int  // byte offset at which readErr is returned
	withBytes bool // deliver the pending chunk alongside readErr

	mu      sync.Mutex
	offset  int
	closes  int
	buffers []readBuffer
}

func (f *fakeFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(p) > 0 {
		f.buffers = append(f.buffers, readBuffer{pointer: &p[0], length: len(p)})
	}
	if f.readErr != nil && f.offset >= f.failAt {
		if !f.withBytes || f.offset >= len(f.data) {
			return 0, f.readErr
		}
		read := copy(p, f.data[f.offset:])
		f.offset += read
		return read, f.readErr
	}
	if f.offset >= len(f.data) {
		return 0, io.EOF
	}
	read := copy(p, f.data[f.offset:])
	f.offset += read
	return read, nil
}

func (f *fakeFile) Stat() (fs.FileInfo, error) {
	if f.statErr != nil {
		return nil, f.statErr
	}
	return f.info, nil
}

func (f *fakeFile) Close() error {
	f.mu.Lock()
	f.closes++
	f.mu.Unlock()
	return f.closeErr
}

func (f *fakeFile) closeCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closes
}

func (f *fakeFile) readBuffers() []readBuffer {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]readBuffer(nil), f.buffers...)
}

type fakeFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
}

func (f fakeFileInfo) Name() string       { return f.name }
func (f fakeFileInfo) Size() int64        { return f.size }
func (f fakeFileInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeFileInfo) ModTime() time.Time { return f.modTime }
func (f fakeFileInfo) IsDir() bool        { return f.mode.IsDir() }
func (f fakeFileInfo) Sys() any           { return nil }

type countingWriter struct {
	err     error
	short   bool
	onWrite func()

	writes int
	bytes  int
}

func (w *countingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.onWrite != nil {
		w.onWrite()
	}
	if w.err != nil {
		return 0, w.err
	}
	if w.short && len(p) > 0 {
		w.bytes += len(p) - 1
		return len(p) - 1, nil
	}
	w.bytes += len(p)
	return len(p), nil
}

func assertCode(t *testing.T, err error, want transfer.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want code %q", want)
	}
	if got := transfer.ErrorCodeOf(err); got != want {
		t.Fatalf("ErrorCodeOf(err) = %q, want %q (error: %v)", got, want, err)
	}
}

func assertNoPayload(t *testing.T, prepared any, err error, want transfer.ErrorCode) {
	t.Helper()
	assertCode(t, err, want)
	if prepared != nil {
		t.Fatalf("Prepare() returned a payload alongside %v", err)
	}
}

// assertNoDisclosure guards the error text FairDrop is willing to render. The
// public message comes from a fixed table, so the load-bearing half is the
// local text: it is what wraps causes such as *os.PathError, which carry the
// selected path verbatim.
func assertNoDisclosure(t *testing.T, err error, path string) {
	t.Helper()
	if err == nil {
		t.Fatal("error = nil, want a coded failure to inspect for disclosure")
	}
	public := transfer.PublicErrorOf(err).Message
	local := err.Error()
	for _, secret := range []string{path, filepath.Base(path), filepath.Dir(path)} {
		if secret == "" || secret == "." {
			continue
		}
		if strings.Contains(public, secret) {
			t.Fatalf("public message disclosed %q: %q", secret, public)
		}
		if strings.Contains(local, secret) {
			t.Fatalf("local error text disclosed %q: %q", secret, local)
		}
	}
}

func assertHeaderSafeName(t *testing.T, name string) {
	t.Helper()
	if name == "" {
		t.Fatal("download name is empty")
	}
	if strings.ContainsAny(name, `/\`) {
		t.Fatalf("download name %q still carries a path separator", name)
	}
	if name == "." || name == ".." {
		t.Fatalf("download name %q is a traversal element", name)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			t.Fatalf("download name %q still carries control character %U", name, r)
		}
	}
}
