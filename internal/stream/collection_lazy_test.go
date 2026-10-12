package stream

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

// holdCounter counts the collection resources that are held concurrently: one
// top-level file descriptor or one prepared directory pin each. The central
// claim of lazy preparation is that this never exceeds one, whatever the item
// count, so the instrumentation sits at the two places a member acquires
// anything -- the payload adapter's open seam and the source port's
// PrepareDirectory -- and counts overlap directly.
type holdCounter struct {
	mu         sync.Mutex
	held       int
	max        int
	acquired   int
	closeCalls int
	inspected  []string
}

func (c *holdCounter) acquire() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.held++
	c.acquired++
	if c.held > c.max {
		c.max = c.held
	}
}

func (c *holdCounter) release() {
	c.mu.Lock()
	c.held--
	c.mu.Unlock()
}

func (c *holdCounter) closed() {
	c.mu.Lock()
	c.closeCalls++
	c.mu.Unlock()
}

type holdSnapshot struct {
	held, max, acquired, closeCalls int
	inspected                       []string
}

func (c *holdCounter) snapshot() holdSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	return holdSnapshot{c.held, c.max, c.acquired, c.closeCalls, append([]string(nil), c.inspected...)}
}

type heldFile struct {
	payloadFile
	counter *holdCounter
	once    sync.Once
}

func (f *heldFile) Close() error {
	f.counter.closed()
	f.once.Do(f.counter.release)
	return f.payloadFile.Close()
}

type heldPin struct {
	transfer.PreparedDirectory
	counter *holdCounter
	once    sync.Once
}

func (p *heldPin) Close() error {
	p.counter.closed()
	p.once.Do(p.counter.release)
	return p.PreparedDirectory.Close()
}

type heldSource struct {
	transfer.SourcePort
	counter *holdCounter
	// afterPrepare, when set, runs right after a directory pin is acquired.
	afterPrepare func(path string)
}

func (s *heldSource) Inspect(ctx context.Context, path string) (transfer.StagedItem, error) {
	s.counter.mu.Lock()
	s.counter.inspected = append(s.counter.inspected, path)
	s.counter.mu.Unlock()
	return s.SourcePort.Inspect(ctx, path)
}

func (s *heldSource) PrepareDirectory(ctx context.Context, path string) (transfer.PreparedDirectory, error) {
	pin, err := s.SourcePort.PrepareDirectory(ctx, path)
	if err != nil {
		return nil, err
	}
	s.counter.acquire()
	if s.afterPrepare != nil {
		s.afterPrepare(path)
	}
	return &heldPin{PreparedDirectory: pin, counter: s.counter}, nil
}

// instrumentedPayloads is the production adapter over the real filesystem with
// every member acquisition counted.
func instrumentedPayloads(raw transfer.SourcePort) (*Payloads, *holdCounter, *heldSource) {
	counter := &holdCounter{}
	held := &heldSource{SourcePort: raw, counter: counter}
	payloads := New(held)
	payloads.open = func(path string) (payloadFile, error) {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		counter.acquire()
		return &heldFile{payloadFile: file, counter: counter}, nil
	}
	return payloads, counter, held
}

// mixedCollection stages files file-0.. and folders folder-0.. interleaved
// (file, folder, file, folder ...) so ordinals alternate kinds. Each folder
// holds inner/x.txt and an empty directory.
func mixedCollection(t *testing.T, files, folders int) (transfer.StagedItem, map[string]string) {
	t.Helper()
	root := fixtureDir(t)
	inspector := source.New()
	want := map[string]string{"FairDrop/": ""}
	var members []transfer.StagedItem
	stageMember := func(path string) transfer.StagedItem {
		member, err := inspector.Inspect(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		return member
	}
	width := ordinalWidth(files + folders)
	for pair := 0; pair < max(files, folders); pair++ {
		if pair < files {
			path := filepath.Join(root, fmt.Sprintf("file-%d.txt", pair))
			content := fmt.Sprintf("file body %d", pair)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			members = append(members, stageMember(path))
			want[fmt.Sprintf("FairDrop/%0*d-file-%d.txt", width, len(members), pair)] = content
		}
		if pair < folders {
			path := filepath.Join(root, fmt.Sprintf("folder-%d", pair))
			if err := os.MkdirAll(filepath.Join(path, "inner"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(path, "empty"), 0o755); err != nil {
				t.Fatal(err)
			}
			content := fmt.Sprintf("folder body %d", pair)
			if err := os.WriteFile(filepath.Join(path, "inner", "x.txt"), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			members = append(members, stageMember(path))
			prefix := fmt.Sprintf("FairDrop/%0*d-folder-%d", width, len(members), pair)
			want[prefix+"/"] = ""
			want[prefix+"/inner/"] = ""
			want[prefix+"/inner/x.txt"] = content
			want[prefix+"/empty/"] = ""
		}
	}
	return transfer.StagedItem{
		Kind: transfer.ItemCollection, Name: fmt.Sprintf("%d items", len(members)),
		Collection: &transfer.StagedCollection{Members: members},
	}, want
}

// readZipIndependently reads a ZIP with archive/zip alone and returns every
// entry's exact content keyed by name, plus the order entries appear in.
func readZipIndependently(t *testing.T, body []byte) (map[string]string, []string) {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("collection ZIP is unreadable: %v", err)
	}
	got := map[string]string{}
	var order []string
	for _, entry := range reader.File {
		opened, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(opened)
		_ = opened.Close()
		if err != nil {
			t.Fatalf("reading %q: %v", entry.Name, err)
		}
		if _, duplicate := got[entry.Name]; duplicate {
			t.Fatalf("duplicate ZIP entry %q", entry.Name)
		}
		got[entry.Name] = string(content)
		order = append(order, entry.Name)
	}
	return got, order
}

func TestCollectionStreamsHundredFilesAndHundredFoldersWithThreeDigitOrdinals(t *testing.T) {
	item, want := mixedCollection(t, 100, 100)
	payloads, counter, _ := instrumentedPayloads(source.New())
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	var body bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &body); err != nil {
		t.Fatal(err)
	}
	got, order := readZipIndependently(t, body.Bytes())
	if len(got) != len(want) {
		t.Fatalf("ZIP has %d entries, want %d", len(got), len(want))
	}
	for name, content := range want {
		if actual, ok := got[name]; !ok || actual != content {
			t.Fatalf("entry %q = %q (present %v), want %q", name, actual, ok, content)
		}
	}
	// Ordinals 001..200 appear in selection order, and the first and last are
	// the literal names the contract promises.
	var tops []string
	for _, name := range order {
		trimmed := strings.TrimPrefix(name, "FairDrop/")
		if trimmed == "" || strings.Contains(strings.TrimSuffix(trimmed, "/"), "/") {
			continue
		}
		tops = append(tops, trimmed)
	}
	if len(tops) != 200 || tops[0] != "001-file-0.txt" || tops[1] != "002-folder-0/" || tops[198] != "199-file-99.txt" || tops[199] != "200-folder-99/" {
		t.Fatalf("top-level members = %d, first %q %q, last %q %q", len(tops), tops[0], tops[1], tops[198], tops[199])
	}
	for index, name := range tops {
		if !strings.HasPrefix(name, fmt.Sprintf("%03d-", index+1)) {
			t.Fatalf("member %d is %q, out of ordinal order", index+1, name)
		}
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	final := counter.snapshot()
	if final.max != 1 || final.held != 0 || final.acquired != 200 || final.closeCalls != 200 {
		t.Fatalf("200 members held at most %d at once, %d still held, %d acquired, %d closes; want 1, 0, 200, 200",
			final.max, final.held, final.acquired, final.closeCalls)
	}
}

// samplingWriter records how many member resources are held every time bytes
// reach the destination, sampling the invariant while the stream is live rather
// than only at its end.
type samplingWriter struct {
	buf     bytes.Buffer
	counter *holdCounter
	mu      sync.Mutex
	samples map[int]int
}

func (w *samplingWriter) Write(p []byte) (int, error) {
	held := w.counter.snapshot().held
	w.mu.Lock()
	if w.samples == nil {
		w.samples = map[int]int{}
	}
	w.samples[held]++
	w.mu.Unlock()
	return w.buf.Write(p)
}

func TestCollectionHoldsAtMostOneMemberAtAnyMoment(t *testing.T) {
	item, _ := mixedCollection(t, 25, 25)
	payloads, counter, _ := instrumentedPayloads(source.New())
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	// The first member is prepared before any response header: exactly one
	// resource is held and only that member was inspected.
	if before := counter.snapshot(); before.held != 1 || before.acquired != 1 || len(before.inspected) > 1 {
		t.Fatalf("after Prepare held=%d acquired=%d inspected=%v; want only member 1 prepared", before.held, before.acquired, before.inspected)
	}
	writer := &samplingWriter{counter: counter}
	if err := prepared.WriteTo(context.Background(), writer); err != nil {
		t.Fatalf("one-at-a-time collection streamed with error: %v", err)
	}
	final := counter.snapshot()
	if final.max != 1 {
		t.Fatalf("up to %d members were held at once; want at most 1", final.max)
	}
	if final.acquired != 50 {
		t.Fatalf("only %d of 50 members were ever prepared", final.acquired)
	}
	if final.held != 0 {
		t.Fatalf("%d members still held after the stream finished", final.held)
	}
	for held := range writer.samples {
		if held > 1 {
			t.Fatalf("destination write observed %d members held", held)
		}
	}
	if writer.samples[1] == 0 {
		t.Fatal("no write ever observed a held member; the sampling proved nothing")
	}
}

// gatedWriter lets a test stop the stream at its first byte on the wire and
// inspect what has been prepared by then.
type gatedWriter struct {
	buf     bytes.Buffer
	once    sync.Once
	onFirst func()
	reached chan struct{}
	proceed chan struct{}
}

func newGatedWriter(onFirst func()) *gatedWriter {
	return &gatedWriter{onFirst: onFirst, reached: make(chan struct{}), proceed: make(chan struct{})}
}

func (w *gatedWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		if w.onFirst != nil {
			w.onFirst()
		}
		close(w.reached)
		<-w.proceed
	})
	return w.buf.Write(p)
}

func randomFile(t *testing.T, dir, name string, size int) string {
	t.Helper()
	data := make([]byte, size)
	rand.New(rand.NewSource(7)).Read(data)
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func stageAll(t *testing.T, paths ...string) transfer.StagedItem {
	t.Helper()
	members := make([]transfer.StagedItem, 0, len(paths))
	for _, path := range paths {
		members = append(members, stage(t, path))
	}
	return transfer.StagedItem{Kind: transfer.ItemCollection, Name: fmt.Sprintf("%d items", len(members)), Collection: &transfer.StagedCollection{Members: members}}
}

func TestCollectionPreparesFirstMemberBeforeHeadersAndLaterMembersOnlyWhenReached(t *testing.T) {
	root := fixtureDir(t)
	big := randomFile(t, root, "big.bin", 4<<20)
	second := writeFileIn(t, root, "second.txt", "second")
	third := writeFileIn(t, root, "third.txt", "third")
	item := stageAll(t, big, second, third)
	payloads, counter, _ := instrumentedPayloads(source.New())
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	if got := counter.snapshot(); got.acquired != 1 || len(got.inspected) != 1 || got.inspected[0] != big {
		t.Fatalf("Prepare touched %v and acquired %d; want only the first member", got.inspected, got.acquired)
	}
	var atFirstByte holdSnapshot
	writer := newGatedWriter(func() { atFirstByte = counter.snapshot() })
	close(writer.proceed)
	if err := prepared.WriteTo(context.Background(), writer); err != nil {
		t.Fatal(err)
	}
	// While the first member's 4 MiB is still being written, neither later
	// member has been inspected or opened.
	if atFirstByte.acquired != 1 || len(atFirstByte.inspected) != 1 {
		t.Fatalf("at the first byte on the wire %d members were acquired and %v inspected; later members must wait their turn",
			atFirstByte.acquired, atFirstByte.inspected)
	}
	if final := counter.snapshot(); final.acquired != 3 || final.max != 1 {
		t.Fatalf("final acquired=%d max=%d; want 3 and 1", final.acquired, final.max)
	}
}

func writeFileIn(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCollectionLaterMemberChangedReplacedOrRemovedAbortsUnfinalized(t *testing.T) {
	for _, test := range []struct {
		name         string
		mutate       func(t *testing.T, paths [4]string)
		code         transfer.ErrorCode
		wantAcquired int
	}{
		{"later file removed", func(t *testing.T, p [4]string) {
			if err := os.Remove(p[1]); err != nil {
				t.Fatal(err)
			}
		}, transfer.ErrPathNotFound, 1},
		{"later file changed size", func(t *testing.T, p [4]string) {
			if err := os.WriteFile(p[1], []byte("a different length"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, transfer.ErrSourceChanged, 1},
		{"later folder removed", func(t *testing.T, p [4]string) {
			if err := os.RemoveAll(p[2]); err != nil {
				t.Fatal(err)
			}
		}, transfer.ErrPathNotFound, 2},
		{"later folder replaced by a file", func(t *testing.T, p [4]string) {
			if err := os.RemoveAll(p[2]); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p[2], []byte("now a file"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, transfer.ErrSourceChanged, 2},
		{"last file removed", func(t *testing.T, p [4]string) {
			if err := os.Remove(p[3]); err != nil {
				t.Fatal(err)
			}
		}, transfer.ErrPathNotFound, 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := fixtureDir(t)
			paths := [4]string{
				writeFileIn(t, root, "a.txt", "first member"),
				writeFileIn(t, root, "b.txt", "second"),
				filepath.Join(root, "c-folder"),
				writeFileIn(t, root, "d.txt", "fourth"),
			}
			if err := os.Mkdir(paths[2], 0o755); err != nil {
				t.Fatal(err)
			}
			writeFileIn(t, paths[2], "inside.txt", "inside")
			item := stageAll(t, paths[0], paths[1], paths[2], paths[3])
			payloads, counter, _ := instrumentedPayloads(source.New())
			prepared, err := payloads.Prepare(context.Background(), item)
			if err != nil {
				t.Fatal(err)
			}
			defer prepared.Close()
			test.mutate(t, paths)
			var body bytes.Buffer
			err = prepared.WriteTo(context.Background(), &body)
			assertCode(t, err, test.code)
			if _, zipErr := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len())); zipErr == nil {
				t.Fatal("failed collection has a valid ZIP central directory")
			}
			// Before Close is called: the worker alone already released every
			// earlier member, and never prepared those after the failure.
			after := counter.snapshot()
			if after.held != 0 || after.acquired != test.wantAcquired || after.max != 1 {
				t.Fatalf("after the abort held=%d acquired=%d max=%d; want 0, %d and 1", after.held, after.acquired, after.max, test.wantAcquired)
			}
			if err := prepared.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCollectionFirstMemberChangedIsAPreStreamRefusal(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(t *testing.T, first string)
		code   transfer.ErrorCode
	}{
		{"file removed", func(t *testing.T, first string) {
			if err := os.Remove(first); err != nil {
				t.Fatal(err)
			}
		}, transfer.ErrPathNotFound},
		{"file changed", func(t *testing.T, first string) {
			if err := os.WriteFile(first, []byte("a different length"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, transfer.ErrSourceChanged},
		{"folder removed", func(t *testing.T, first string) {
			if err := os.RemoveAll(first); err != nil {
				t.Fatal(err)
			}
		}, transfer.ErrPathNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := fixtureDir(t)
			first := filepath.Join(root, "first")
			if test.name == "folder removed" {
				if err := os.Mkdir(first, 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				first = writeFileIn(t, root, "first", "first")
			}
			second := writeFileIn(t, root, "second.txt", "second")
			item := stageAll(t, first, second)
			payloads, counter, _ := instrumentedPayloads(source.New())
			test.mutate(t, first)
			prepared, err := payloads.Prepare(context.Background(), item)
			assertNoPayload(t, prepared, err, test.code)
			got := counter.snapshot()
			if got.held != 0 || got.acquired > 1 || got.closeCalls != got.acquired {
				t.Fatalf("a refused first member left held=%d acquired=%d closes=%d", got.held, got.acquired, got.closeCalls)
			}
			for _, path := range got.inspected {
				if path == second {
					t.Fatal("the second member was touched although the first was refused")
				}
			}
		})
	}
}

func TestCollectionDirectoryIdentityIsPinnedAtItsOwnPreparation(t *testing.T) {
	build := func(t *testing.T) (item transfer.StagedItem, folder string) {
		root := fixtureDir(t)
		first := writeFileIn(t, root, "first.txt", "first")
		folder = filepath.Join(root, "folder")
		if err := os.Mkdir(folder, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFileIn(t, folder, "original.txt", "original")
		return stageAll(t, first, folder), folder
	}
	// swap may run on the stream's worker goroutine, so it reports with
	// Errorf rather than FailNow.
	swap := func(t *testing.T, folder, name, content string) {
		if err := os.Rename(folder, folder+"-old"); err != nil {
			t.Errorf("swap rename: %v", err)
			return
		}
		if err := os.Mkdir(folder, 0o755); err != nil {
			t.Errorf("swap mkdir: %v", err)
			return
		}
		if err := os.WriteFile(filepath.Join(folder, name), []byte(content), 0o644); err != nil {
			t.Errorf("swap write: %v", err)
		}
	}

	t.Run("replaced after Stage but before its turn", func(t *testing.T) {
		// Documented trade-off of lazy preparation: a later folder pins its
		// identity when it is prepared, not at Stage, and a folder carries no
		// staged size/mtime equality requirement, so a replacement that is
		// already in place at its turn is what gets streamed -- exactly as a
		// singleton folder replaced before the claim is.
		item, folder := build(t)
		payloads, _, _ := instrumentedPayloads(source.New())
		prepared, err := payloads.Prepare(context.Background(), item)
		if err != nil {
			t.Fatal(err)
		}
		defer prepared.Close()
		swap(t, folder, "substitute.txt", "substitute")
		var body bytes.Buffer
		if err := prepared.WriteTo(context.Background(), &body); err != nil {
			t.Fatal(err)
		}
		got, _ := readZipIndependently(t, body.Bytes())
		if got["FairDrop/02-folder/substitute.txt"] != "substitute" {
			t.Fatalf("replacement present at its turn was not what streamed: %v", got)
		}
	})

	t.Run("replaced between its preparation and its walk", func(t *testing.T) {
		item, folder := build(t)
		payloads, counter, held := instrumentedPayloads(source.New())
		held.afterPrepare = func(path string) {
			if path == folder {
				swap(t, folder, "substitute.txt", "substitute")
			}
		}
		prepared, err := payloads.Prepare(context.Background(), item)
		if err != nil {
			t.Fatal(err)
		}
		defer prepared.Close()
		var body bytes.Buffer
		err = prepared.WriteTo(context.Background(), &body)
		assertCode(t, err, transfer.ErrSourceChanged)
		if _, zipErr := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len())); zipErr == nil {
			t.Fatal("failed collection has a valid ZIP central directory")
		}
		if got := counter.snapshot(); got.held != 0 {
			t.Fatalf("%d members still held after the aborted walk", got.held)
		}
		if bytes.Contains(body.Bytes(), []byte("substitute")) {
			t.Fatal("substituted content reached the wire")
		}
	})
}

func TestCollectionCancelMidMemberReleasesOnlyTheCurrentMember(t *testing.T) {
	root := fixtureDir(t)
	big := randomFile(t, root, "big.bin", 4<<20)
	item := stageAll(t, big, writeFileIn(t, root, "second.txt", "second"), writeFileIn(t, root, "third.txt", "third"))
	payloads, counter, _ := instrumentedPayloads(source.New())
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := newGatedWriter(cancel)
	close(writer.proceed)
	err = prepared.WriteTo(ctx, writer)
	assertCode(t, err, transfer.ErrCancelled)
	got := counter.snapshot()
	if got.held != 0 || got.acquired != 1 || got.closeCalls != 1 {
		t.Fatalf("cancelled mid-member: held=%d acquired=%d closes=%d; want only the current member prepared and closed once", got.held, got.acquired, got.closeCalls)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if after := counter.snapshot(); after.closeCalls != 1 {
		t.Fatalf("Close re-closed a released member: %d close calls", after.closeCalls)
	}
}

func TestCollectionCloseMidStreamReleasesTheCurrentMemberOnly(t *testing.T) {
	root := fixtureDir(t)
	big := randomFile(t, root, "big.bin", 4<<20)
	item := stageAll(t, big, writeFileIn(t, root, "second.txt", "second"), writeFileIn(t, root, "third.txt", "third"))
	payloads, counter, _ := instrumentedPayloads(source.New())
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := newGatedWriter(nil)
	done := make(chan error, 1)
	go func() { done <- prepared.WriteTo(ctx, writer) }()
	select {
	case <-writer.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the stream never produced a byte")
	}
	// The stream is parked inside the first member. Closing now releases that
	// member and nothing else; later members were never prepared.
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if got := counter.snapshot(); got.held != 0 || got.acquired != 1 || got.closeCalls != 1 {
		t.Fatalf("Close mid-stream: held=%d acquired=%d closes=%d; want 0, 1, 1", got.held, got.acquired, got.closeCalls)
	}
	cancel()
	close(writer.proceed)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a collection closed mid-stream reported success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("WriteTo did not return after Close and cancellation")
	}
	if got := counter.snapshot(); got.acquired != 1 || got.closeCalls != 1 {
		t.Fatalf("after the stream unwound acquired=%d closes=%d; want 1 and 1 (no later member prepared)", got.acquired, got.closeCalls)
	}
}

// virtualSource serves a collection with no filesystem at all, so the cap can
// be exercised at its true size.
type virtualSource struct {
	transfer.SourcePort
	items map[string]transfer.StagedItem
}

func (v *virtualSource) Inspect(_ context.Context, path string) (transfer.StagedItem, error) {
	if item, ok := v.items[path]; ok {
		return item, nil
	}
	return transfer.StagedItem{}, transfer.NewError(transfer.ErrPathNotFound, "virtual member is absent")
}

func (v *virtualSource) PrepareDirectory(context.Context, string) (transfer.PreparedDirectory, error) {
	return nil, transfer.NewError(transfer.ErrTransferFailed, "virtual collection has no folders")
}

func (v *virtualSource) Walk(context.Context, string, transfer.SourceVisitor) error {
	return transfer.NewError(transfer.ErrTransferFailed, "virtual collection has no folders")
}

func virtualCollection(count int) (transfer.StagedItem, *virtualSource) {
	modTime := time.Unix(1_700_000_000, 0)
	virtual := &virtualSource{items: make(map[string]transfer.StagedItem, count)}
	members := make([]transfer.StagedItem, 0, count)
	for index := 0; index < count; index++ {
		name := fmt.Sprintf("item-%d.txt", index)
		member := transfer.StagedItem{
			Path: "/virtual/" + name, Name: name, Kind: transfer.ItemFile,
			LogicalSize: int64(len(fmt.Sprintf("body %d", index))), ModTime: modTime,
		}
		virtual.items[member.Path] = member
		members = append(members, member)
	}
	return transfer.StagedItem{Kind: transfer.ItemCollection, Name: fmt.Sprintf("%d items", count), Collection: &transfer.StagedCollection{Members: members}}, virtual
}

func TestCollectionOfTenThousandMembersStreamsOneAtATime(t *testing.T) {
	item, virtual := virtualCollection(10000)
	counter := &holdCounter{}
	payloads := New(virtual)
	payloads.lstat = func(path string) (fs.FileInfo, error) {
		member := virtual.items[path]
		return fakeFileInfo{name: member.Name, size: member.LogicalSize, modTime: member.ModTime}, nil
	}
	payloads.sameFile = matchedIdentity
	payloads.open = func(path string) (payloadFile, error) {
		member := virtual.items[path]
		var index int
		if _, err := fmt.Sscanf(member.Name, "item-%d.txt", &index); err != nil {
			return nil, err
		}
		counter.acquire()
		file := &fakeFile{
			data: []byte(fmt.Sprintf("body %d", index)),
			info: fakeFileInfo{name: member.Name, size: member.LogicalSize, modTime: member.ModTime},
		}
		return &heldFile{payloadFile: file, counter: counter}, nil
	}
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatalf("10,000 members refused at Prepare: %v", err)
	}
	defer prepared.Close()
	if got := counter.snapshot(); got.acquired != 1 {
		t.Fatalf("Prepare acquired %d of 10,000 members; want 1", got.acquired)
	}
	var body bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &body); err != nil {
		t.Fatal(err)
	}
	got, order := readZipIndependently(t, body.Bytes())
	if len(order) != 10001 || order[0] != "FairDrop/" || order[1] != "FairDrop/00001-item-0.txt" || order[10000] != "FairDrop/10000-item-9999.txt" {
		t.Fatalf("10,000-member ZIP has %d entries, first %q %q, last %q", len(order), order[0], order[1], order[len(order)-1])
	}
	if got["FairDrop/00001-item-0.txt"] != "body 0" || got["FairDrop/09999-item-9998.txt"] != "body 9998" || got["FairDrop/10000-item-9999.txt"] != "body 9999" {
		t.Fatal("10,000-member ZIP carried wrong content")
	}
	final := counter.snapshot()
	if final.max != 1 || final.held != 0 || final.acquired != 10000 {
		t.Fatalf("10,000 members: max held %d, still held %d, acquired %d; want 1, 0, 10000", final.max, final.held, final.acquired)
	}
}

func TestCollectionPrepareRefusesCountsOutsideTheContractBeforeTouchingTheSource(t *testing.T) {
	for _, count := range []int{1, 10001} {
		item, virtual := virtualCollection(count)
		counter := &holdCounter{}
		payloads := New(&heldSource{SourcePort: virtual, counter: counter})
		prepared, err := payloads.Prepare(context.Background(), item)
		assertNoPayload(t, prepared, err, transfer.ErrSetupFailed)
		if got := counter.snapshot(); len(got.inspected) != 0 || got.acquired != 0 {
			t.Fatalf("count %d reached the source: %+v", count, got)
		}
	}
}

func TestCollectionOrdinalsArePaddedToTheWidthOfTheCount(t *testing.T) {
	for _, test := range []struct {
		count, index int
		want         string
	}{
		{2, 0, "01-x"}, {2, 1, "02-x"},
		{16, 15, "16-x"},
		{99, 98, "99-x"},
		{100, 0, "001-x"}, {100, 99, "100-x"},
		{999, 0, "001-x"},
		{1000, 0, "0001-x"}, {1000, 999, "1000-x"},
		{9999, 9998, "9999-x"},
		{10000, 0, "00001-x"}, {10000, 9998, "09999-x"}, {10000, 9999, "10000-x"},
	} {
		member := transfer.StagedItem{Name: "x", Path: "/p/x"}
		if got := collectionMemberName(member, test.index, ordinalWidth(test.count)); got != test.want {
			t.Errorf("member %d of %d is %q, want %q", test.index+1, test.count, got, test.want)
		}
	}
}

func TestCollectionWideOrdinalStaysWithinTheRootNameBound(t *testing.T) {
	width := ordinalWidth(10000)
	long := collectionMemberName(transfer.StagedItem{Name: strings.Repeat("x", 300)}, 9999, width)
	if len([]rune(long)) != 200 || !strings.HasPrefix(long, "10000-") {
		t.Fatalf("long name = %d runes %q; want exactly 200 starting with the full ordinal", len([]rune(long)), long[:12])
	}
	multibyte := collectionMemberName(transfer.StagedItem{Name: strings.Repeat("界", 100)}, 0, width)
	if multibyte != "00001-"+strings.Repeat("界", 83) || len(multibyte) != 255 {
		t.Fatalf("multibyte name = %q (%d bytes); want the ordinal plus 83 runes in 255 bytes", multibyte, len(multibyte))
	}
	trimmed := collectionMemberName(transfer.StagedItem{Name: strings.Repeat("x", 193) + ".yyy"}, 0, width)
	if trimmed != "00001-"+strings.Repeat("x", 193) {
		t.Fatalf("clipped trailing dot/space survived: %q", trimmed[len(trimmed)-6:])
	}
	if got := collectionMemberName(transfer.StagedItem{Name: "..."}, 3, width); got != "00004-download" {
		t.Fatalf("fallback name = %q", got)
	}
}

func TestLazyCollectionRefusesToHoldASecondMember(t *testing.T) {
	root := fixtureDir(t)
	item := stageAll(t, writeFileIn(t, root, "a.txt", "a"), writeFileIn(t, root, "b.txt", "b"))
	payloads, counter, _ := instrumentedPayloads(source.New())
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	lazy := prepared.(*archive).lazy
	// Member 0 is held. Preparing member 1 without releasing member 0 must not
	// be installable, and the refused member must be closed rather than leaked.
	second, err := lazy.prepare(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := lazy.install(second); err == nil {
		t.Fatal("a second held member was installed beside the current one")
	}
	if got := counter.snapshot(); got.held != 1 || got.acquired != 2 {
		t.Fatalf("after the refused install held=%d acquired=%d; want the refused member closed (1 held of 2 acquired)", got.held, got.acquired)
	}
}

func TestLazyCollectionClosesAMemberThatFinishesPreparingAfterShutdown(t *testing.T) {
	root := fixtureDir(t)
	folder := filepath.Join(root, "folder")
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "inner.txt"), []byte("inner"), 0o644); err != nil {
		t.Fatal(err)
	}
	item := stageAll(t, writeFileIn(t, root, "a.txt", "a"), folder)
	payloads, counter, held := instrumentedPayloads(source.New())
	prepared, err := payloads.Prepare(context.Background(), item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	lazy := prepared.(*archive).lazy
	// The archive is closed in the window after member 1 acquires its pin and
	// before it is installed: exactly the race an in-flight Close can hit.
	// shutdown also releases member 0, so the held branch cannot be what
	// refuses the install -- only the closed flag can.
	held.afterPrepare = func(path string) {
		if path == folder {
			if err := lazy.shutdown(); err != nil {
				t.Errorf("shutdown: %v", err)
			}
		}
	}
	member, err := lazy.next(context.Background(), 1)
	assertCode(t, err, transfer.ErrTransferFailed)
	if member != nil {
		t.Fatal("a member prepared after shutdown was handed to the stream")
	}
	lazy.mu.Lock()
	installed := lazy.current
	lazy.mu.Unlock()
	if installed != nil {
		t.Fatal("a member prepared after shutdown was installed as current")
	}
	if got := counter.snapshot(); got.held != 0 || got.acquired != 2 || got.closeCalls != 2 {
		t.Fatalf("after shutdown mid-preparation held=%d acquired=%d closes=%d; want 0, 2, 2 (each member closed exactly once)", got.held, got.acquired, got.closeCalls)
	}
	if err := prepared.Close(); err != nil {
		t.Fatal(err)
	}
	if after := counter.snapshot(); after.closeCalls != 2 {
		t.Fatalf("Close re-closed a member: %d close calls, want 2", after.closeCalls)
	}
}

// cancelAfterSameFile cancels the context the moment the nth identity
// comparison has passed, which is the last step of a file member's
// preparation: the member is fully open when the cancellation lands.
func cancelAfterSameFile(payloads *Payloads, nth int, cancel context.CancelFunc) {
	calls := 0
	payloads.sameFile = func(before, after fs.FileInfo) bool {
		calls++
		if calls == nth {
			cancel()
		}
		return os.SameFile(before, after)
	}
}

func TestCollectionPrepareCancelledAfterTheFirstMemberOpensReleasesIt(t *testing.T) {
	root := fixtureDir(t)
	item := stageAll(t, writeFileIn(t, root, "a.txt", "a"), writeFileIn(t, root, "b.txt", "b"))
	payloads, counter, _ := instrumentedPayloads(source.New())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelAfterSameFile(payloads, 1, cancel)
	prepared, err := payloads.Prepare(ctx, item)
	assertNoPayload(t, prepared, err, transfer.ErrCancelled)
	if got := counter.snapshot(); got.held != 0 || got.acquired != 1 || got.closeCalls != 1 {
		t.Fatalf("a cancelled Prepare left the first member held=%d acquired=%d closes=%d; want 0, 1, 1", got.held, got.acquired, got.closeCalls)
	}
}

func TestCollectionCancelledJustAfterALaterMemberOpensClosesIt(t *testing.T) {
	root := fixtureDir(t)
	item := stageAll(t, writeFileIn(t, root, "a.txt", "a"), writeFileIn(t, root, "b.txt", "b"), writeFileIn(t, root, "c.txt", "c"))
	payloads, counter, _ := instrumentedPayloads(source.New())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelAfterSameFile(payloads, 2, cancel)
	prepared, err := payloads.Prepare(ctx, item)
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	assertCode(t, prepared.WriteTo(ctx, io.Discard), transfer.ErrCancelled)
	// Member 2 finished opening and was then abandoned; it must not stay held,
	// and member 3 must never have been touched.
	if got := counter.snapshot(); got.held != 0 || got.acquired != 2 || got.closeCalls != 2 {
		t.Fatalf("cancelled just after a later member opened: held=%d acquired=%d closes=%d; want 0, 2, 2", got.held, got.acquired, got.closeCalls)
	}
}
