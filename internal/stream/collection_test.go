package stream

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

type trackedCollectionFile struct {
	*os.File
	closed *int
}

type trackedCollectionDirectory struct {
	transfer.PreparedDirectory
	closes   *int
	closeErr error
}

func (d *trackedCollectionDirectory) Close() error {
	*d.closes++
	return errors.Join(d.PreparedDirectory.Close(), d.closeErr)
}

type trackingCollectionSource struct {
	transfer.SourcePort
	directoryCloses []*int
	closeFailureAt  int
}

func (s *trackingCollectionSource) PrepareDirectory(ctx context.Context, path string) (transfer.PreparedDirectory, error) {
	pin, err := s.SourcePort.PrepareDirectory(ctx, path)
	if err != nil {
		return nil, err
	}
	closes := new(int)
	s.directoryCloses = append(s.directoryCloses, closes)
	var closeErr error
	if len(s.directoryCloses) == s.closeFailureAt {
		closeErr = errors.New("injected directory close failure")
	}
	return &trackedCollectionDirectory{PreparedDirectory: pin, closes: closes, closeErr: closeErr}, nil
}

type mutatingCollectionSource struct {
	transfer.SourcePort
	mutate func()
}

type cancellingCollectionSource struct {
	transfer.SourcePort
	calls  int
	cancel context.CancelFunc
}

func (s *cancellingCollectionSource) Inspect(ctx context.Context, path string) (transfer.StagedItem, error) {
	s.calls++
	if s.calls == 2 {
		s.cancel()
	}
	return s.SourcePort.Inspect(ctx, path)
}

func (s *mutatingCollectionSource) Inspect(ctx context.Context, path string) (transfer.StagedItem, error) {
	if s.mutate != nil {
		mutate := s.mutate
		s.mutate = nil
		mutate()
	}
	return s.SourcePort.Inspect(ctx, path)
}

func (f *trackedCollectionFile) Stat() (fs.FileInfo, error) { return f.File.Stat() }
func (f *trackedCollectionFile) Close() error               { *f.closed++; return f.File.Close() }

func collectionFixture(t *testing.T) (transfer.StagedItem, string) {
	t.Helper()
	root := fixtureDir(t)
	first := filepath.Join(root, "a", "report.txt")
	second := filepath.Join(root, "b", "report.txt")
	folder := filepath.Join(root, "folder")
	for _, path := range []string{filepath.Dir(first), filepath.Dir(second), filepath.Join(folder, "empty")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{first: "first", second: "second", filepath.Join(folder, "nested.txt"): "nested"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	inspector := source.New()
	members := make([]transfer.StagedItem, 0, 3)
	for _, path := range []string{first, second, folder} {
		member, err := inspector.Inspect(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		members = append(members, member)
	}
	return transfer.StagedItem{Kind: transfer.ItemCollection, Name: "3 items", LogicalSize: 17, Collection: &transfer.StagedCollection{Members: members}}, folder
}

func twoDirectoryCollection(t *testing.T) transfer.StagedItem {
	t.Helper()
	item, folder := collectionFixture(t)
	second := filepath.Join(filepath.Dir(folder), "second-folder")
	if err := os.Mkdir(second, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "report.txt"), []byte("second folder"), 0o644); err != nil {
		t.Fatal(err)
	}
	other, err := source.New().Inspect(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	item.Collection.Members = []transfer.StagedItem{item.Collection.Members[2], item.Collection.Members[0], other}
	return item
}

func TestCollectionDirectoryPinsCloseExactlyOnceOnEveryExit(t *testing.T) {
	// Members are folder, file, folder. Members are prepared lazily, so each
	// exit prepares and closes exactly the members it reached: a stream that
	// never starts has prepared only the first.
	for _, test := range []struct {
		scenario     string
		wantPins     int
		wantFileOpen int
	}{
		{"success", 2, 1},
		{"close before write", 1, 0},
		{"cancelled write", 1, 0},
		{"later prepare failure", 1, 1},
		{"close failure", 2, 1},
	} {
		t.Run(test.scenario, func(t *testing.T) {
			item := twoDirectoryCollection(t)
			raw := &trackingCollectionSource{SourcePort: source.New()}
			if test.scenario == "close failure" {
				raw.closeFailureAt = 2
			}
			if test.scenario == "later prepare failure" {
				item.Collection.Members[2].Path = filepath.Join(fixtureDir(t), "missing-folder")
			}
			fileCloses := 0
			payloads := New(raw)
			payloads.open = func(path string) (payloadFile, error) {
				file, err := os.Open(path)
				if err != nil {
					return nil, err
				}
				return &trackedCollectionFile{File: file, closed: &fileCloses}, nil
			}
			prepared, err := payloads.Prepare(context.Background(), item)
			if err != nil {
				t.Fatalf("Prepare failed for a first member that is present: %v", err)
			}
			switch test.scenario {
			case "success", "close failure":
				if err := prepared.WriteTo(context.Background(), io.Discard); err != nil {
					t.Fatal(err)
				}
			case "later prepare failure":
				assertCode(t, prepared.WriteTo(context.Background(), io.Discard), transfer.ErrPathNotFound)
			case "cancelled write":
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if err := prepared.WriteTo(ctx, io.Discard); err == nil {
					t.Fatal("cancelled write succeeded")
				}
			}
			err = prepared.Close()
			if test.scenario == "close failure" {
				if err == nil || !strings.Contains(err.Error(), "injected directory close failure") {
					t.Fatalf("close failure = %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if err := prepared.Close(); err != nil {
				t.Fatalf("repeated close = %v", err)
			}
			if len(raw.directoryCloses) != test.wantPins {
				t.Fatalf("prepared %d directory pins, want %d", len(raw.directoryCloses), test.wantPins)
			}
			for index, count := range raw.directoryCloses {
				if *count != 1 {
					t.Fatalf("directory %d Close called %d times, want once", index, *count)
				}
			}
			if fileCloses != test.wantFileOpen {
				t.Fatalf("mixed top-level file Close called %d times, want %d", fileCloses, test.wantFileOpen)
			}
		})
	}
}

func TestCollectionStreamsAllMembersWithNumberedRoots(t *testing.T) {
	item, _ := collectionFixture(t)
	prepared, err := New(source.New()).Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	if prepared.DownloadName() != "FairDrop.zip" {
		t.Fatalf("download name = %q", prepared.DownloadName())
	}
	if size, known := prepared.Size(); known || size != 0 {
		t.Fatalf("ZIP wire size = %d, %v", size, known)
	}
	var body bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &body); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"FairDrop/01-report.txt":        "first",
		"FairDrop/02-report.txt":        "second",
		"FairDrop/03-folder/nested.txt": "nested",
		"FairDrop/03-folder/empty/":     "",
	}
	for _, entry := range reader.File {
		if entry.Name == "FairDrop/" || entry.Name == "FairDrop/03-folder/" {
			continue
		}
		expected, ok := want[entry.Name]
		if !ok {
			t.Errorf("unexpected ZIP entry %q", entry.Name)
			continue
		}
		delete(want, entry.Name)
		opened, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(opened)
		_ = opened.Close()
		if err != nil || string(content) != expected {
			t.Errorf("entry %q = %q, %v", entry.Name, content, err)
		}
	}
	if len(want) != 0 {
		t.Errorf("missing ZIP entries: %v", want)
	}
	if err := prepared.WriteTo(context.Background(), io.Discard); err == nil {
		t.Fatal("second write succeeded")
	}
}

func TestCollectionCloseBeforeWriteReleasesEveryMember(t *testing.T) {
	item, _ := collectionFixture(t)
	payloads, counter, _ := instrumentedPayloads(source.New())
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	// Only the first member exists before the stream starts.
	if got := counter.snapshot(); got.held != 1 || got.acquired != 1 {
		t.Fatalf("after Prepare held=%d acquired=%d; want only the first member", got.held, got.acquired)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if got := counter.snapshot(); got.held != 0 || got.acquired != 1 || got.closeCalls != 1 {
		t.Fatalf("after Close held=%d acquired=%d closes=%d; want 0, 1, 1", got.held, got.acquired, got.closeCalls)
	}
	err = prepared.WriteTo(context.Background(), io.Discard)
	if err == nil || errors.Is(err, io.EOF) {
		t.Fatalf("write after close = %v", err)
	}
	if got := counter.snapshot(); got.acquired != 1 {
		t.Fatalf("a closed collection prepared %d members", got.acquired)
	}
}

func TestCollectionLaterFilePrepareFailureClosesEarlierDescriptor(t *testing.T) {
	item, _ := collectionFixture(t)
	item.Collection.Members = item.Collection.Members[:2]
	item.Collection.Members[1].Path = filepath.Join(fixtureDir(t), "missing.txt")
	closed := 0
	payloads := New(source.New())
	payloads.open = func(path string) (payloadFile, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return &trackedCollectionFile{File: file, closed: &closed}, nil
	}
	// The first member is present, so Prepare succeeds; the missing second
	// member is found when the stream reaches it, and the first member's
	// descriptor is closed by then.
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatalf("Prepare refused a present first member: %v", err)
	}
	defer prepared.Close()
	assertCode(t, prepared.WriteTo(context.Background(), io.Discard), transfer.ErrPathNotFound)
	if closed != 1 {
		t.Fatalf("earlier descriptor close count = %d, want 1 before the collection is closed", closed)
	}
}

func TestCollectionCancellationBetweenMembersClosesEarlierDescriptor(t *testing.T) {
	item, _ := collectionFixture(t)
	item.Collection.Members = item.Collection.Members[:2]
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	tracked := &cancellingCollectionSource{SourcePort: source.New(), cancel: cancel}
	closed := 0
	payloads := New(tracked)
	payloads.open = func(path string) (payloadFile, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		return &trackedCollectionFile{File: file, closed: &closed}, nil
	}
	prepared, err := payloads.Prepare(ctx, item)
	if err != nil {
		t.Fatalf("Prepare failed before the cancellation point: %v", err)
	}
	defer prepared.Close()
	// Cancellation lands as the second member begins preparing, during the
	// stream, so the stream stops and the first descriptor is already closed.
	if err := prepared.WriteTo(ctx, io.Discard); transfer.ErrorCodeOf(err) != transfer.ErrCancelled {
		t.Fatalf("cancelled stream = %v, want cancelled", err)
	}
	if tracked.calls != 2 || closed != 1 {
		t.Fatalf("cancelled after %d source calls and %d descriptor closes; want 2 and 1", tracked.calls, closed)
	}
}

func TestCollectionPreparedFileLengthIsEnforced(t *testing.T) {
	for _, test := range []struct {
		name, replacement string
		succeeds          bool
	}{
		{"truncate", "x", false},
		{"grow", "first plus unexpected bytes", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			item, _ := collectionFixture(t)
			item.Collection.Members = item.Collection.Members[:2]
			prepared, err := New(source.New()).Prepare(context.Background(), item)
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.Close()
			path := item.Collection.Members[0].Path
			if err := os.WriteFile(path, []byte(test.replacement), 0o644); err != nil {
				t.Fatal(err)
			}
			var body bytes.Buffer
			err = prepared.WriteTo(context.Background(), &body)
			if test.succeeds {
				if err != nil {
					t.Fatal(err)
				}
				reader, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len()))
				if err != nil {
					t.Fatal(err)
				}
				opened, err := reader.File[1].Open()
				if err != nil {
					t.Fatal(err)
				}
				content, err := io.ReadAll(opened)
				_ = opened.Close()
				if err != nil || string(content) != "first" {
					t.Fatalf("grown source member = %q, %v", content, err)
				}
			} else {
				if err == nil {
					t.Fatal("short source yielded successful collection")
				}
				if _, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len())); err == nil {
					t.Fatal("short source finalized ZIP")
				}
			}
		})
	}
}

func TestCollectionPreparationSnapshotsCallerMembersBeforeExternalWork(t *testing.T) {
	item, _ := collectionFixture(t)
	item.Collection.Members = item.Collection.Members[:2]
	second := item.Collection.Members[1].Path
	source := &mutatingCollectionSource{SourcePort: source.New()}
	source.mutate = func() { item.Collection.Members[1].Path = filepath.Join(fixtureDir(t), "absent.txt") }
	prepared, err := New(source).Prepare(context.Background(), item)
	if err != nil {
		t.Fatalf("caller mutation replaced admitted member %q: %v", second, err)
	}
	defer prepared.Close()
	if item.Collection.Members[1].Path == second {
		t.Fatal("mutation seam never ran")
	}
	var body bytes.Buffer
	// Member 2 is prepared lazily during the stream, after the caller's
	// mutation; it must still be the admitted path.
	if err := prepared.WriteTo(context.Background(), &body); err != nil {
		t.Fatalf("caller mutation replaced admitted member %q during the stream: %v", second, err)
	}
	reader, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 3 || reader.File[2].Name != "FairDrop/02-report.txt" {
		t.Fatalf("prepared archive members = %v", reader.File)
	}
}

func TestCollectionRootNameFallbackAndPostPrefixLimit(t *testing.T) {
	item, _ := collectionFixture(t)
	item.Collection.Members = item.Collection.Members[:2]
	item.Collection.Members[0].Name = "   "
	item.Collection.Members[1].Name = strings.Repeat("x", 196) + ".suffix"
	prepared, err := New(source.New()).Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	var body bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &body); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if reader.File[1].Name != "FairDrop/01-report.txt" {
		t.Fatalf("empty sanitized name did not use basename fallback: %q", reader.File[1].Name)
	}
	second := strings.TrimPrefix(reader.File[2].Name, "FairDrop/")
	if !strings.HasPrefix(second, "02-") || strings.HasSuffix(second, ".") || len([]rune(second)) > maxDownloadNameRunes {
		t.Fatalf("numbered root exceeded safe limit or ended in dot: %q", second)
	}
}

func TestCollectionNumberedRootFitsMultibyteFilesystemLimit(t *testing.T) {
	item, _ := collectionFixture(t)
	item.Collection.Members = item.Collection.Members[:2]
	item.Collection.Members[0].Name = strings.Repeat("界", 85)
	prepared, err := New(source.New()).Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	var body bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &body); err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	name := strings.TrimPrefix(reader.File[1].Name, "FairDrop/")
	if len(name) > 255 || !utf8.ValidString(name) || !strings.HasPrefix(name, "01-") || len([]rune(name)) > maxDownloadNameRunes {
		t.Fatalf("numbered multibyte root exceeded bound: %q (%d bytes)", name, len(name))
	}
	if name != "01-"+strings.Repeat("界", 84) {
		t.Fatalf("root clipped at wrong rune boundary: %q", name)
	}
}
