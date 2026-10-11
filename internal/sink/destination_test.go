package sink

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// --- Opening a destination ----------------------------------------------

func TestOpenDestinationRefusesWhatItMustAndCreatesNothing(t *testing.T) {
	root := fixtureDir(t)
	file := filepath.Join(root, "plain.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	linked := os.Symlink(real, link) == nil
	through := filepath.Join(link, "inner")
	if linked {
		if err := os.Mkdir(filepath.Join(real, "inner"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name string
		path string
		want transfer.ErrorCode
		skip bool
	}{
		{"a missing folder is path_not_found", filepath.Join(root, "missing"), transfer.ErrPathNotFound, false},
		{"a file is path_unsupported", file, transfer.ErrPathUnsupported, false},
		{"a symbolic link is path_unsupported", link, transfer.ErrPathUnsupported, !linked},
		{"a path through a symbolic link is path_unsupported", through, transfer.ErrPathUnsupported, !linked},
		{"a relative path is invalid_selection", "relative/folder", transfer.ErrInvalidSelection, false},
		{"an empty path is invalid_selection", "", transfer.ErrInvalidSelection, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.skip {
				t.Skip("this runner cannot create the symbolic link fixture")
			}
			before := listing(t, root)
			dest, err := New().OpenDestination(context.Background(), testCase.path)
			if dest != nil {
				t.Fatalf("OpenDestination(%q) returned a destination", testCase.path)
			}
			if got := transfer.ErrorCodeOf(err); got != testCase.want {
				t.Fatalf("OpenDestination(%q) code = %q (%v), want %q", testCase.path, got, err, testCase.want)
			}
			if after := listing(t, root); !equalStrings(before, after) {
				t.Fatalf("a refused open changed the fixture: %v -> %v", before, after)
			}
		})
	}
}

func TestOpenDestinationHonoursACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := New().OpenDestination(ctx, fixtureDir(t)); transfer.ErrorCodeOf(err) != transfer.ErrCancelled {
		t.Fatalf("OpenDestination with a cancelled context = %v, want a cancelled error", err)
	}
	var noContext context.Context
	if _, err := New().OpenDestination(noContext, fixtureDir(t)); transfer.ErrorCodeOf(err) != transfer.ErrSetupFailed {
		t.Fatalf("OpenDestination with a nil context = %v, want a setup_failed error", err)
	}
}

// The seam tests below replace parts of the sink; this one drives the defaults
// New() actually ships, end to end, over a real directory.
func TestTheProductionDefaultsSaveARealFile(t *testing.T) {
	dir := fixtureDir(t)
	dest, err := New().OpenDestination(context.Background(), dir)
	if err != nil {
		t.Fatalf("OpenDestination = %v", err)
	}
	if _, err := dest.SaveFile(context.Background(), "real.txt", strings.NewReader("hello")); err != nil {
		t.Fatalf("SaveFile = %v", err)
	}
	folder, exists := dest.Folder()
	if !exists || filepath.Dir(folder) != dir || !strings.HasPrefix(filepath.Base(folder), "FairDrop ") {
		t.Fatalf("Folder() = %q, %v, want a FairDrop subfolder directly inside the destination", folder, exists)
	}
	if got := readFile(t, filepath.Join(folder, "real.txt")); got != "hello" {
		t.Fatalf("saved content = %q, want %q", got, "hello")
	}
	if _, err := dest.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
	if New().entropy != rand.Reader {
		t.Fatal("the production sink does not draw temporary names from crypto/rand")
	}
}

func TestTheDestinationNamesItselfByBasenameOnly(t *testing.T) {
	dir := filepath.Join(fixtureDir(t), "Phone Photos")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := openTest(t, newTestSink(), dir)
	if got := dest.Name(); got != "Phone Photos" {
		t.Fatalf("Name() = %q, want the basename %q", got, "Phone Photos")
	}
}

// --- Saving --------------------------------------------------------------

func TestNothingIsCreatedUntilTheFirstFile(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)

	if got := listing(t, dir); len(got) != 0 {
		t.Fatalf("opening a destination created %v", got)
	}
	if _, exists := dest.Folder(); exists {
		t.Fatal("Folder() reports a subfolder before any file")
	}
	result, err := dest.Close()
	if err != nil {
		t.Fatalf("Close = %v", err)
	}
	if result != (transfer.ReceiveResult{}) {
		t.Fatalf("Close() = %+v, want the zero result", result)
	}
	if got := listing(t, dir); len(got) != 0 {
		t.Fatalf("an abandoned session left %v behind", got)
	}
}

func TestSaveFileWritesExactBytesIntoALocalTimeNamedSubfolder(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)

	payload := bytes.Repeat([]byte("0123456789abcdef"), 70000) // 1.1 MB, several buffers
	written, err := dest.SaveFile(context.Background(), "big.bin", bytes.NewReader(payload))
	if err != nil || written != int64(len(payload)) {
		t.Fatalf("SaveFile = %d, %v, want %d, nil", written, err, len(payload))
	}

	if got := listing(t, dir); !equalStrings(got, []string{"FairDrop 2026-10-10 14.05"}) {
		t.Fatalf("destination holds %v, want exactly the subfolder", got)
	}
	sub := filepath.Join(dir, "FairDrop 2026-10-10 14.05")
	if got := listing(t, sub); !equalStrings(got, []string{"big.bin"}) {
		t.Fatalf("subfolder holds %v, want exactly big.bin and no temporary file", got)
	}
	if got := readFile(t, filepath.Join(sub, "big.bin")); got != string(payload) {
		t.Fatal("the saved bytes differ from the uploaded bytes")
	}
	snapshot := dest.Snapshot()
	if snapshot.FilesSaved != 1 || !snapshot.SubfolderExists || snapshot.MarkingFailed {
		t.Fatalf("Snapshot() = %+v, want 1 file, a subfolder, no marking failure", snapshot)
	}
	if folder, exists := dest.Folder(); !exists || folder != sub {
		t.Fatalf("Folder() = %q, %v, want %q, true", folder, exists, sub)
	}
}

// The folder name is the wall clock of the time it is given, not a UTC
// conversion of it: the contract says local time.
func TestTheSubfolderNameUsesTheClockWallTime(t *testing.T) {
	sink := newTestSink()
	sink.now = func() time.Time { return time.Date(2026, 1, 2, 23, 59, 0, 0, time.FixedZone("UTC+5:30", 5*3600+1800)) }
	dir := fixtureDir(t)
	dest := openTest(t, sink, dir)
	save(t, dest, "a.txt", "a")
	if got := listing(t, dir); !equalStrings(got, []string{"FairDrop 2026-01-02 23.59"}) {
		t.Fatalf("destination holds %v, want %q", got, "FairDrop 2026-01-02 23.59")
	}
}

func TestAnExistingSubfolderNameGetsANumberAndNothingInItChanges(t *testing.T) {
	dir := fixtureDir(t)
	for _, taken := range []string{fixedSubfolder, fixedSubfolder + " (1)"} {
		if err := os.Mkdir(filepath.Join(dir, taken), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, taken, "keep.txt"), []byte("ORIGINAL "+taken), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dest := openTest(t, newTestSink(), dir)
	save(t, dest, "new.txt", "new")

	want := []string{fixedSubfolder, fixedSubfolder + " (1)", fixedSubfolder + " (2)"}
	if got := listing(t, dir); !equalStrings(got, want) {
		t.Fatalf("destination holds %v, want %v", got, want)
	}
	if got := readFile(t, filepath.Join(dir, fixedSubfolder+" (2)", "new.txt")); got != "new" {
		t.Fatalf("new file content = %q", got)
	}
	for _, taken := range []string{fixedSubfolder, fixedSubfolder + " (1)"} {
		if got := listing(t, filepath.Join(dir, taken)); !equalStrings(got, []string{"keep.txt"}) {
			t.Fatalf("%q was written into: %v", taken, got)
		}
		if got := readFile(t, filepath.Join(dir, taken, "keep.txt")); got != "ORIGINAL "+taken {
			t.Fatalf("%q/keep.txt was modified: %q", taken, got)
		}
	}
}

// A link sitting on the subfolder's would-be name is a name taken, not a
// directory to write through.
func TestASymbolicLinkOnTheSubfolderNameIsNeverFollowed(t *testing.T) {
	dir := fixtureDir(t)
	elsewhere := fixtureDir(t)
	if err := os.Symlink(elsewhere, filepath.Join(dir, fixedSubfolder)); err != nil {
		t.Skip("this runner cannot create the symbolic link fixture")
	}
	dest := openTest(t, newTestSink(), dir)
	save(t, dest, "a.txt", "a")

	if got := listing(t, elsewhere); len(got) != 0 {
		t.Fatalf("the upload followed a link and wrote %v elsewhere", got)
	}
	if got := readFile(t, filepath.Join(dir, fixedSubfolder+" (1)", "a.txt")); got != "a" {
		t.Fatalf("saved content = %q, want it in the numbered subfolder", got)
	}
}

func TestDuplicateAndUnsafeNamesStayInsideTheSubfolder(t *testing.T) {
	parent := fixtureDir(t)
	dir := filepath.Join(parent, "dest")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := openTest(t, newTestSink(), dir)

	names := []string{"x.jpg", "X.JPG", "x.jpg", "../../escape.txt", `..\..\win.txt`, "CON.txt", "", "a:b.txt"}
	for index, name := range names {
		save(t, dest, name, "body "+string(rune('A'+index)))
	}

	want := []string{"CON.txt", "X (1).JPG", "a_b.txt", "escape.txt", "file", "win.txt", "x (2).jpg", "x.jpg"}
	// "CON.txt" is device-prefixed: the sorted list uses its final spelling.
	want[0] = "_CON.txt"
	want = sortedCopy(want)
	sub := filepath.Join(dir, fixedSubfolder)
	if got := listing(t, sub); !equalStrings(got, want) {
		t.Fatalf("subfolder holds %v, want %v", got, want)
	}
	if got := listing(t, dir); !equalStrings(got, []string{fixedSubfolder}) {
		t.Fatalf("destination holds %v, want only the subfolder", got)
	}
	if got := listing(t, parent); !equalStrings(got, []string{"dest"}) {
		t.Fatalf("something was written outside the destination: %v", got)
	}
	if got := readFile(t, filepath.Join(sub, "x.jpg")); got != "body A" {
		t.Fatalf("x.jpg = %q, want the first upload's bytes", got)
	}
	if got := readFile(t, filepath.Join(sub, "X (1).JPG")); got != "body B" {
		t.Fatalf("X (1).JPG = %q", got)
	}
}

func sortedCopy(in []string) []string {
	out := append([]string(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// The rename must never replace. A file that lands on the final name between the
// write and the rename -- the only moment a race can put one there -- survives,
// and the new file takes the next number.
func TestAFinalNameTakenBeforeTheRenameIsNeverReplaced(t *testing.T) {
	dir := fixtureDir(t)
	sink := newTestSink()
	planted := 0
	sink.beforeRename = func(directory, finalName string) {
		planted++
		if planted == 1 {
			if err := os.WriteFile(filepath.Join(directory, finalName), []byte("ORIGINAL"), 0o644); err != nil {
				t.Errorf("planting the conflicting file: %v", err)
			}
		}
	}
	dest := openTest(t, sink, dir)
	save(t, dest, "doc.txt", "NEW")

	sub := filepath.Join(dir, fixedSubfolder)
	if got := readFile(t, filepath.Join(sub, "doc.txt")); got != "ORIGINAL" {
		t.Fatalf("doc.txt = %q, want the file that was already there left untouched", got)
	}
	if got := readFile(t, filepath.Join(sub, "doc (1).txt")); got != "NEW" {
		t.Fatalf("doc (1).txt = %q, want the new bytes under the next free name", got)
	}
	if got := listing(t, sub); !equalStrings(got, []string{"doc (1).txt", "doc.txt"}) {
		t.Fatalf("subfolder holds %v, want exactly the two files and no temporary file", got)
	}
	if dest.Snapshot().FilesSaved != 1 {
		t.Fatalf("FilesSaved = %d, want 1", dest.Snapshot().FilesSaved)
	}
}

func TestFilesAndFoldersAreNeverExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX permission bits to inspect")
	}
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)
	save(t, dest, "run.sh", "#!/bin/sh\necho hi\n")

	sub := filepath.Join(dir, fixedSubfolder)
	file, err := os.Stat(filepath.Join(sub, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := file.Mode().Perm(); perm&^0o644 != 0 {
		t.Fatalf("the file mode is %o, want no bit beyond 0644", perm)
	}
	folder, err := os.Stat(sub)
	if err != nil {
		t.Fatal(err)
	}
	if perm := folder.Mode().Perm(); perm&^0o755 != 0 {
		t.Fatalf("the folder mode is %o, want no bit beyond 0755", perm)
	}
}

// --- Failure keeps completed files and removes only the partial one -------

func TestAReadErrorMidFileRemovesOnlyThePartialFile(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)
	save(t, dest, "first.txt", "complete")

	_, err := dest.SaveFile(context.Background(), "second.txt", &failingReader{prefix: "half of a file", err: errDropped})
	if err == nil {
		t.Fatal("SaveFile of a dropped upload returned nil")
	}
	if !errors.Is(err, errDropped) {
		t.Fatalf("SaveFile error = %v, want the read failure preserved for the caller", err)
	}

	sub := filepath.Join(dir, fixedSubfolder)
	if got := listing(t, sub); !equalStrings(got, []string{"first.txt"}) {
		t.Fatalf("subfolder holds %v, want only the completed file", got)
	}
	if got := readFile(t, filepath.Join(sub, "first.txt")); got != "complete" {
		t.Fatalf("the completed file was changed: %q", got)
	}
	if got := dest.Snapshot().FilesSaved; got != 1 {
		t.Fatalf("FilesSaved = %d after a failed second file, want 1", got)
	}
}

func TestAFirstFileFailureLeavesNoSubfolderOnceTheDestinationCloses(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)

	if _, err := dest.SaveFile(context.Background(), "only.txt", &failingReader{prefix: "partial", err: errDropped}); err == nil {
		t.Fatal("SaveFile of a dropped upload returned nil")
	}
	result, err := dest.Close()
	if err != nil {
		t.Fatalf("Close = %v", err)
	}
	if result.FilesSaved != 0 || result.SubfolderExists {
		t.Fatalf("Close() = %+v, want nothing saved and no subfolder", result)
	}
	if got := listing(t, dir); len(got) != 0 {
		t.Fatalf("a session that saved nothing left %v behind", got)
	}
	if _, exists := dest.Folder(); exists {
		t.Fatal("Folder() reports a subfolder that was removed")
	}
}

func TestACancelledContextMidFileRemovesThePartialFile(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)
	save(t, dest, "kept.txt", "kept")

	ctx, cancel := context.WithCancel(context.Background())
	reader := &cancellingReader{cancel: cancel}
	_, err := dest.SaveFile(ctx, "partial.txt", reader)
	if transfer.ErrorCodeOf(err) != transfer.ErrCancelled {
		t.Fatalf("SaveFile after cancellation = %v, want a cancelled error", err)
	}
	if got := listing(t, filepath.Join(dir, fixedSubfolder)); !equalStrings(got, []string{"kept.txt"}) {
		t.Fatalf("subfolder holds %v, want only the completed file", got)
	}
}

// cancellingReader cancels the context after delivering its first chunk, then
// keeps delivering: SaveFile must notice on its own.
type cancellingReader struct {
	cancel context.CancelFunc
	reads  int
}

func (r *cancellingReader) Read(p []byte) (int, error) {
	r.reads++
	if r.reads == 1 {
		r.cancel()
	}
	return copy(p, "more bytes"), nil
}

// --- Close ---------------------------------------------------------------

func TestCloseKeepsSavedFilesIsIdempotentAndRefusesLaterWrites(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)
	save(t, dest, "a.txt", "a")
	save(t, dest, "b.txt", "b")

	first, err := dest.Close()
	if err != nil {
		t.Fatalf("Close = %v", err)
	}
	want := transfer.ReceiveResult{FilesSaved: 2, SubfolderExists: true}
	if first != want {
		t.Fatalf("Close() = %+v, want %+v", first, want)
	}
	second, _ := dest.Close()
	if second != first {
		t.Fatalf("a second Close() = %+v, want the same final result %+v", second, first)
	}
	if _, err := dest.SaveFile(context.Background(), "late.txt", strings.NewReader("late")); err == nil {
		t.Fatal("SaveFile after Close returned nil")
	}
	if got := listing(t, filepath.Join(dir, fixedSubfolder)); !equalStrings(got, []string{"a.txt", "b.txt"}) {
		t.Fatalf("subfolder holds %v after Close, want exactly the two saved files", got)
	}
}

// Close while a file is half written: the in-flight file must not be counted or
// left behind, and the empty subfolder must go once its writer leaves.
func TestCloseWhileAFileIsBeingWrittenLeavesNothingBehind(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)

	reader := &blockingReader{prefix: "in flight", started: make(chan struct{}), release: make(chan struct{})}
	finished := make(chan error, 1)
	go func() {
		_, err := dest.SaveFile(context.Background(), "slow.bin", reader)
		finished <- err
	}()
	select {
	case <-reader.started:
	case <-time.After(5 * time.Second):
		t.Fatal("the writer never started")
	}

	result, err := dest.Close()
	if err != nil {
		t.Fatalf("Close = %v", err)
	}
	if result.FilesSaved != 0 || result.SubfolderExists {
		t.Fatalf("Close() = %+v while a file was in flight, want nothing saved", result)
	}

	close(reader.release)
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("the in-flight SaveFile succeeded after Close")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the in-flight SaveFile never returned")
	}
	if got := listing(t, dir); len(got) != 0 {
		t.Fatalf("a closed session left %v behind", got)
	}
	if got := dest.Snapshot().FilesSaved; got != 0 {
		t.Fatalf("FilesSaved = %d, want the in-flight file never counted", got)
	}
}

func TestSaveFileIsSerialPerDestination(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)

	reader := &blockingReader{prefix: "x", started: make(chan struct{}), release: make(chan struct{})}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = dest.SaveFile(context.Background(), "one.bin", reader)
	}()
	<-reader.started
	if _, err := dest.SaveFile(context.Background(), "two.bin", strings.NewReader("two")); err == nil {
		t.Error("a second concurrent SaveFile was accepted")
	}
	close(reader.release)
	wg.Wait()
}

// --- Marking --------------------------------------------------------------

func TestAMarkingFailureKeepsTheFileAndIsReported(t *testing.T) {
	dir := fixtureDir(t)
	sink := newTestSink()
	sink.markOverride = func() error { return errors.New("test: the volume cannot hold the marking") }
	dest := openTest(t, sink, dir)
	save(t, dest, "kept.txt", "still saved")

	if got := readFile(t, filepath.Join(dir, fixedSubfolder, "kept.txt")); got != "still saved" {
		t.Fatalf("the file was not kept: %q", got)
	}
	snapshot := dest.Snapshot()
	if snapshot.FilesSaved != 1 || !snapshot.MarkingFailed {
		t.Fatalf("Snapshot() = %+v, want one saved file and a marking failure", snapshot)
	}
	result, _ := dest.Close()
	if !result.MarkingFailed {
		t.Fatalf("Close() = %+v, want the marking failure reported", result)
	}
}

// --- Space ---------------------------------------------------------------

func TestTheSpaceReserveBoundary(t *testing.T) {
	const reserve = uint64(3 << 30)
	cases := []struct {
		name      string
		available uint64
		declared  int64
		want      bool
	}{
		{"exactly the reserve remains", reserve + 1000, 1000, true},
		{"one byte short of the reserve", reserve + 999, 1000, false},
		{"one byte more than the reserve", reserve + 1001, 1000, true},
		{"a zero byte upload needs only the reserve", reserve, 0, true},
		{"a zero byte upload one byte short", reserve - 1, 0, false},
		{"declared larger than the whole volume", 100, 1 << 40, false},
		{"declared equal to available leaves no reserve", 5 << 30, 5 << 30, false},
		{"a negative declared size is refused", reserve * 2, -1, false},
		{"the largest declared size does not overflow", math.MaxUint64, math.MaxInt64, true},
	}
	for _, testCase := range cases {
		if got := fitsWithReserve(testCase.available, testCase.declared); got != testCase.want {
			t.Errorf("%s: fitsWithReserve(%d, %d) = %v, want %v", testCase.name, testCase.available, testCase.declared, got, testCase.want)
		}
	}
}

func TestCheckSpaceUsesTheVolumeAnswerAndTheThreeGiBReserve(t *testing.T) {
	if ReserveBytes != 3*1024*1024*1024 {
		t.Fatalf("ReserveBytes = %d, want exactly 3 GiB", ReserveBytes)
	}
	sink := newTestSink()
	available := uint64(10<<30) + 123
	sink.available = func(dirHandle) (uint64, error) { return available, nil }
	dest := openTest(t, sink, fixtureDir(t))

	if err := dest.CheckSpace(7<<30 + 123); err != nil {
		t.Fatalf("CheckSpace at the exact boundary = %v, want it accepted", err)
	}
	if err := dest.CheckSpace(7<<30 + 124); !errors.Is(err, transfer.ErrInsufficientSpace) {
		t.Fatalf("CheckSpace one byte past the boundary = %v, want ErrInsufficientSpace", err)
	}
	if err := dest.CheckSpace(-5); !errors.Is(err, transfer.ErrInsufficientSpace) {
		t.Fatalf("CheckSpace of a negative size = %v, want ErrInsufficientSpace", err)
	}

	sink.available = func(dirHandle) (uint64, error) { return 0, errors.New("test: statfs failed") }
	if err := dest.CheckSpace(1); err == nil || errors.Is(err, transfer.ErrInsufficientSpace) {
		t.Fatalf("CheckSpace with an unanswerable volume = %v, want a distinct error a caller treats as a refusal", err)
	}
}

// The test above replaces the volume query; this one drives the real one.
func TestCheckSpaceAsksTheRealVolume(t *testing.T) {
	dest := openTest(t, New(), fixtureDir(t))
	available, err := dest.root.availableBytes()
	if err != nil || available == 0 {
		t.Fatalf("availableBytes() = %d, %v, want a positive answer from the real volume", available, err)
	}
	if err := dest.CheckSpace(math.MaxInt64); !errors.Is(err, transfer.ErrInsufficientSpace) {
		t.Fatalf("CheckSpace(MaxInt64) = %v, want ErrInsufficientSpace on every real volume", err)
	}
	if err := dest.CheckSpace(0); err != nil && !errors.Is(err, transfer.ErrInsufficientSpace) {
		t.Fatalf("CheckSpace(0) = %v, want nil or ErrInsufficientSpace from the real volume", err)
	}
}

// --- Memory and OS temp storage -------------------------------------------

// maxReadReader records the largest buffer SaveFile ever offers it.
type maxReadReader struct {
	remaining int
	largest   int
}

func (r *maxReadReader) Read(p []byte) (int, error) {
	if len(p) > r.largest {
		r.largest = len(p)
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.remaining)
	for index := range p[:n] {
		p[index] = 'z'
	}
	r.remaining -= n
	return n, nil
}

func TestAFileIsCopiedThroughOneBoundedBufferAndNothingGoesToOSTempStorage(t *testing.T) {
	osTemp := t.TempDir()
	t.Setenv("TMPDIR", osTemp)
	t.Setenv("TMP", osTemp)
	t.Setenv("TEMP", osTemp)

	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)
	reader := &maxReadReader{remaining: 24 << 20}
	written, err := dest.SaveFile(context.Background(), "large.bin", reader)
	if err != nil || written != 24<<20 {
		t.Fatalf("SaveFile = %d, %v, want 24 MiB, nil", written, err)
	}
	if reader.largest != 64<<10 {
		t.Fatalf("SaveFile offered a %d byte buffer, want exactly 64 KiB", reader.largest)
	}
	if got := listing(t, osTemp); len(got) != 0 {
		t.Fatalf("the upload wrote %v to OS temp storage", got)
	}
}
