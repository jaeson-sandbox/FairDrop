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
	if strings.Contains(prepared.DownloadName(), string(os.PathSeparator)) {
		t.Fatalf("DownloadName() disclosed a path: %q", prepared.DownloadName())
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
	adapter := &Payloads{
		source: matchingSource(staged),
		open:   func(string) (payloadFile, error) { return nil, cause },
	}

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrPathNotFound)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("open cause is not preserved through Unwrap")
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
	adapter := &Payloads{
		source: matchingSource(staged),
		open:   func(string) (payloadFile, error) { return decoy, nil },
	}

	prepared, err := adapter.Prepare(context.Background(), staged)
	assertNoPayload(t, prepared, err, transfer.ErrSourceChanged)
	if decoy.closeCount() != 1 {
		t.Fatalf("swapped descriptor closed %d times, want 1", decoy.closeCount())
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
			adapter := &Payloads{
				source: matchingSource(staged),
				open:   func(string) (payloadFile, error) { return opened, nil },
			}

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
	adapter := &Payloads{
		source: matchingSource(staged),
		open:   func(string) (payloadFile, error) { return nil, cause },
	}

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
	adapter := &Payloads{
		source: matchingSource(staged),
		open:   func(string) (payloadFile, error) { return opened, nil },
	}

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
		adapter := &Payloads{
			source: matchingSource(staged),
			open: func(string) (payloadFile, error) {
				cancel()
				return opened, nil
			},
		}

		prepared, err := adapter.Prepare(ctx, staged)
		assertNoPayload(t, prepared, err, transfer.ErrCancelled)
		if opened.closeCount() != 1 {
			t.Fatalf("descriptor closed %d times after cancellation, want 1", opened.closeCount())
		}
	})
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

func TestPreparePassesSelectedPathToTheOpenCallUnchanged(t *testing.T) {
	t.Parallel()

	path := writeFile(t, "spaced 🌍 name.bin", []byte("payload"))
	staged := stage(t, path)
	var opened []string
	adapter := &Payloads{source: source.New(), open: func(candidate string) (payloadFile, error) {
		opened = append(opened, candidate)
		return os.Open(candidate)
	}}

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	if len(opened) != 1 || opened[0] != path {
		t.Fatalf("open paths = %q, want exactly one byte-identical %q", opened, path)
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
	adapter := &Payloads{
		source:     matchingSource(staged),
		open:       func(string) (payloadFile, error) { return opened, nil },
		bufferSize: 8,
	}

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

func TestWriteToReportsShortWriteAsTransferFailure(t *testing.T) {
	t.Parallel()

	staged := stage(t, writeFile(t, "short-write.bin", bytes.Repeat([]byte("s"), 1024)))
	adapter := &Payloads{source: source.New(), bufferSize: 256}

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	destination := &countingWriter{short: true}
	assertCode(t, prepared.WriteTo(context.Background(), destination), transfer.ErrTransferFailed)
	if destination.writes != 1 {
		t.Fatalf("destination received %d writes after a short write, want exactly 1", destination.writes)
	}
}

func TestWriteToReportsSourceReadFailure(t *testing.T) {
	t.Parallel()

	staged := fabricatedItem(t, "unreadable-stream.bin", 6)
	cause := errors.New("source read failed")
	opened := &fakeFile{
		data:    []byte("abcdef"),
		readErr: cause,
		info:    fakeFileInfo{name: staged.Name, size: 6, modTime: staged.ModTime},
	}
	adapter := &Payloads{
		source:     matchingSource(staged),
		open:       func(string) (payloadFile, error) { return opened, nil },
		bufferSize: 4,
	}

	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	t.Cleanup(func() { _ = prepared.Close() })

	err = prepared.WriteTo(context.Background(), io.Discard)
	assertCode(t, err, transfer.ErrTransferFailed)
	if !errors.Is(err, cause) {
		t.Fatal("read cause is not preserved through Unwrap")
	}
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
			adapter := &Payloads{
				source: matchingSource(staged),
				open:   func(string) (payloadFile, error) { return opened, nil },
			}
			prepared, err := adapter.Prepare(context.Background(), staged)
			if err != nil {
				t.Fatalf("Prepare() error = %v", err)
			}
			concrete, ok := prepared.(*payload)
			if !ok {
				t.Fatalf("Prepare() returned %T, want *payload", prepared)
			}
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
	adapter := &Payloads{
		source: matchingSource(staged),
		open:   func(string) (payloadFile, error) { return opened, nil },
	}
	prepared, err := adapter.Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

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
// tests supply both the validation result and the descriptor.
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

type sourceFunc func(ctx context.Context, absolutePath string) (transfer.StagedItem, error)

func (f sourceFunc) Inspect(ctx context.Context, absolutePath string) (transfer.StagedItem, error) {
	return f(ctx, absolutePath)
}

// matchingSource reports the staged snapshot back unchanged, so path-level
// validation cannot be what catches a descriptor-level divergence.
func matchingSource(item transfer.StagedItem) sourceFunc {
	return func(context.Context, string) (transfer.StagedItem, error) { return item, nil }
}

type readBuffer struct {
	pointer *byte
	length  int
}

type fakeFile struct {
	data     []byte
	info     fs.FileInfo
	statErr  error
	readErr  error
	closeErr error

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
	if f.offset >= len(f.data) {
		if f.readErr != nil {
			return 0, f.readErr
		}
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

func assertNoDisclosure(t *testing.T, err error, path string) {
	t.Helper()
	public := transfer.PublicErrorOf(err).Message
	local := err.Error()
	for _, secret := range []string{path, filepath.Base(path), filepath.Dir(path)} {
		if strings.Contains(public, secret) {
			t.Fatalf("public message disclosed %q: %q", secret, public)
		}
		if strings.Contains(local, secret) {
			t.Fatalf("local error text disclosed %q: %q", secret, local)
		}
	}
}
