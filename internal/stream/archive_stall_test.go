package stream

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// stallingReader is a legal io.Reader that never progresses. Returning (0, nil)
// is permitted by the interface, and a SourcePort is injectable, so this is a
// reachable shape rather than a contrived one.
type stallingReader struct{ reads int }

func (s *stallingReader) Read([]byte) (int, error) {
	s.reads++
	return 0, nil
}

/*
A source that never progresses fails the transfer instead of spinning.

The archive path carries its own copy of the empty-read guard, and nothing
exercised it: every scripted fixture hands the visitor a strings.Reader,
which always either progresses or reports io.EOF. Deleting the branch left
the suite green while a non-progressing reader would busy-loop inside the
response goroutine until the receiver gave up.
*/
func TestArchiveFailsASourceThatNeverProgresses(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	staged := transfer.StagedItem{Path: root, Name: "root", Kind: transfer.ItemDirectory}
	reader := &stallingReader{}

	src := &scriptedSource{
		inspect: func(context.Context, string) (transfer.StagedItem, error) { return staged, nil },
		walk: func(_ context.Context, _ string, visit transfer.SourceVisitor) error {
			return visit(transfer.SourceEntry{
				RelativePath: "stalled.bin",
				Kind:         transfer.ItemFile,
				Size:         64,
				ModTime:      time.Unix(1700000000, 0),
			}, reader)
		},
	}

	prepared, err := (&Payloads{source: src, bufferSize: defaultBufferSize}).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	defer func() { _ = prepared.Close() }()

	done := make(chan error, 1)
	go func() { done <- prepared.WriteTo(context.Background(), io.Discard) }()

	select {
	case err := <-done:
		assertCode(t, err, transfer.ErrTransferFailed)
	case <-time.After(5 * time.Second):
		t.Fatal("WriteTo never returned for a source that never progresses")
	}
	// Bounded, not merely finite: the guard is a fixed count, so an
	// implementation that spun a million times before giving up would still
	// have returned above.
	if reader.reads > maxEmptyReads+1 {
		t.Errorf("read %d times before failing, want at most %d", reader.reads, maxEmptyReads+1)
	}
}

/*
No temporary archive is ever staged on disk.

Counting the shared temp directory around a stream cannot settle this:
other tests create and remove their own temp directories concurrently, so
the count moves in both directions for reasons unrelated to the archive --
an earlier version of this check failed exactly that way, reporting a
directory that had shrunk. The claim is about what this package can do, so
it is settled against the package's own source.

The banned list covers every spelling that creates or truncates a file, not
only the temp-file helpers: os.Create and os.WriteFile would stage an
archive just as effectively as os.CreateTemp.
*/
func TestStreamPackageNeverWritesToDisk(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{
		"os.CreateTemp", "os.MkdirTemp", "ioutil.TempFile", "ioutil.TempDir",
		"os.Create(", "os.WriteFile", "os.OpenFile", "os.Mkdir", "os.MkdirAll",
	}

	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		scanned++
		for _, call := range banned {
			if strings.Contains(string(source), call) {
				t.Errorf("%s calls %s: the payload path must never stage bytes on disk", name, call)
			}
		}
	}
	// Without this the loop could scan nothing and still pass, which is the
	// shape a green test takes when it has quietly stopped looking.
	if scanned == 0 {
		t.Fatal("scanned no production files; the guard proved nothing")
	}
}
