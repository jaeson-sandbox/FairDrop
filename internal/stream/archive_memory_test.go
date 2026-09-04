package stream

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

/*
Archive memory is bounded by the buffer and the tree's depth, never by how
many entries the tree holds.

This is the assertion a retained file index would fail, and it is measured
rather than argued. The walk is scripted instead of written to disk so the
entry count can reach a scale no fixture could: fifty thousand entries would
cost several megabytes if any layer kept one record per entry.

Retained live heap is the right measure here, not cumulative TotalAlloc.
Per-entry allocation that is immediately garbage is legitimate -- a ZIP
writer builds and discards a header for every entry -- so TotalAlloc grows
with entry count for a correct implementation and would make this test
assert the opposite of what it means. HeapAlloc after a GC is what survives.

One thing genuinely is O(entries) and cannot be otherwise: archive/zip
retains a small central-directory record per entry, because the format
writes that directory at the end. The ceiling below therefore accommodates
the central directory while staying far under what retaining the entries
themselves -- names, metadata, or content -- would cost.
*/
func TestArchiveRetainedMemoryDoesNotGrowWithEntryCount(t *testing.T) {
	const entries = 50000

	root := t.TempDir()
	staged := transfer.StagedItem{Path: root, Name: "root", Kind: transfer.ItemDirectory}
	body := strings.Repeat("b", 64)

	walked := 0
	src := &scriptedSource{
		inspect: func(context.Context, string) (transfer.StagedItem, error) { return staged, nil },
		walk: func(ctx context.Context, _ string, visit transfer.SourceVisitor) error {
			for index := range entries {
				entry := transfer.SourceEntry{
					RelativePath: fmt.Sprintf("f%06d.bin", index),
					Kind:         transfer.ItemFile,
					Size:         int64(len(body)),
					ModTime:      time.Unix(1700000000, 0),
				}
				// A fresh reader per entry, borrowed for the call only, is what
				// the port promises the visitor -- the same shape the real
				// source hands over.
				if err := visit(entry, strings.NewReader(body)); err != nil {
					return err
				}
				walked++
			}
			return nil
		},
	}

	prepared, err := (&Payloads{source: src, bufferSize: defaultBufferSize}).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	defer func() {
		if err := prepared.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	}()

	runtime.GC()
	var heapBefore, heapAfter runtime.MemStats
	runtime.ReadMemStats(&heapBefore)
	if err := prepared.WriteTo(context.Background(), io.Discard); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	runtime.GC()
	runtime.ReadMemStats(&heapAfter)
	// Without these the payload and its source are dead by the second read, and
	// a retained index would be collected before it could be measured.
	runtime.KeepAlive(prepared)
	runtime.KeepAlive(src)

	if walked != entries {
		t.Fatalf("walked %d entries, want %d", walked, entries)
	}

	var retained uint64
	if heapAfter.HeapAlloc > heapBefore.HeapAlloc {
		retained = heapAfter.HeapAlloc - heapBefore.HeapAlloc
	}
	// Measured repeatedly at ~0.8 MiB for this tree, dominated by the
	// compressor. The ceiling sits above that and well below the ~4 MiB that
	// fifty thousand retained SourceEntry records cost, which is what makes
	// this assertion able to fail: a ceiling generous enough to admit the
	// index would look exactly like a passing test.
	const ceiling = uint64(2 << 20)
	if retained > ceiling {
		t.Fatalf("retained live heap grew by %d bytes streaming %d entries, want at most %d",
			retained, entries, ceiling)
	}
}

/*
No temporary archive is ever staged on disk.

Counting the shared temp directory around a stream cannot prove this: other
tests create and remove their own temp directories concurrently, so the
count moves in both directions for reasons that have nothing to do with the
archive. The claim is about what this package is capable of, so it is
settled against the package's own source instead -- deterministic, and it
fails the moment someone introduces a temp-file call rather than only when
a timing window happens to expose one.
*/
func TestStreamPackageNeverCreatesATemporaryFile(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"os.CreateTemp", "os.MkdirTemp", "ioutil.TempFile", "ioutil.TempDir"}

	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, err := os.ReadFile(name)
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
