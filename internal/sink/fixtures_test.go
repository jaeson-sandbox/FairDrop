package sink

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// fixtureDir is the only way a test in this package gets a directory. A raw
// t.TempDir() is a path macOS spells through a /var symlink, and OpenDestination
// refuses a link-like component anywhere on its path by design -- so a raw one
// would hand the code a path it is required to refuse and fail a test for a
// reason unrelated to what the test checks. This is the same helper the source
// and stream packages carry, for the same reason.
func fixtureDir(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolving the fixture directory: %v", err)
	}
	return resolved
}

// fixedClock pins the wall clock the subfolder name is taken from.
var fixedClock = time.Date(2026, 10, 10, 14, 5, 33, 0, time.UTC)

const fixedSubfolder = "FairDrop 2026-10-10 14.05"

// newTestSink is the production sink with only the clock replaced.
func newTestSink() *Sink {
	s := New()
	s.now = func() time.Time { return fixedClock }
	return s
}

// openTest opens a destination over dir.
func openTest(t *testing.T, s *Sink, dir string) *destination {
	t.Helper()
	dest, err := s.OpenDestination(context.Background(), dir)
	if err != nil {
		t.Fatalf("OpenDestination(%q) = %v, want a destination", dir, err)
	}
	t.Cleanup(func() { _, _ = dest.Close() })
	return dest.(*destination)
}

// save writes one file part and fails the test if it is refused.
func save(t *testing.T, dest *destination, name, content string) {
	t.Helper()
	written, err := dest.SaveFile(context.Background(), name, strings.NewReader(content))
	if err != nil {
		t.Fatalf("SaveFile(%q) = %v, want it saved", name, err)
	}
	if written != int64(len(content)) {
		t.Fatalf("SaveFile(%q) wrote %d bytes, want %d", name, written, len(content))
	}
}

// listing returns the sorted names directly inside dir.
func listing(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %q: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %q: %v", path, err)
	}
	return string(data)
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

// failingReader delivers prefix and then fails with err, which is what a dropped
// connection looks like to SaveFile.
type failingReader struct {
	prefix string
	err    error
	done   bool
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.done {
		return 0, r.err
	}
	r.done = true
	return copy(p, r.prefix), nil
}

var errDropped = errors.New("test: connection dropped")

// blockingReader delivers prefix, then blocks until release is closed, then
// reports EOF. It is how a test holds a file open mid-write.
type blockingReader struct {
	prefix   string
	started  chan struct{}
	release  chan struct{}
	sentOnce bool
}

func (r *blockingReader) Read(p []byte) (int, error) {
	if !r.sentOnce {
		r.sentOnce = true
		n := copy(p, r.prefix)
		close(r.started)
		return n, nil
	}
	<-r.release
	return 0, io.EOF
}
