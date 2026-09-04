package stream

import (
	"context"
	"fmt"
	"io"
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
assert the opposite of what it means. 	HeapAlloc after a GC is what survives.

It is sampled at the last entry, not after WriteTo returns. A first version
measured afterwards and was demonstrably weaker: an index rooted in the
payload was caught, but one accumulated in a local inside the archive worker
is unreachable by then and got collected before the measurement, so that
mutation passed. The peak this criterion is about happens while the walk is
still on the stack.

One thing genuinely is O(entries) and cannot be otherwise: archive/zip
retains a central-directory record per entry, because the format writes that
directory at the end. Measured here at a steady ~248 bytes per entry from
10,000 entries upward -- about 12 MiB at fifty thousand. So the bound that can
honestly be asserted is per-entry overhead, not a fixed ceiling: what must not
happen is this story's own layers keeping a second record per entry on top of
the one the format forces.
*/
func TestArchiveRetainedMemoryDoesNotGrowWithEntryCount(t *testing.T) {
	const entries = 50000

	root := t.TempDir()
	staged := transfer.StagedItem{Path: root, Name: "root", Kind: transfer.ItemDirectory}
	body := strings.Repeat("b", 64)

	walked := 0
	var peak uint64
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
				if walked == entries {
					// Still inside the walk: everything the stream is holding
					// is reachable here, including a worker-local index.
					runtime.GC()
					var atPeak runtime.MemStats
					runtime.ReadMemStats(&atPeak)
					peak = atPeak.HeapAlloc
				}
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
	var heapBefore runtime.MemStats
	runtime.ReadMemStats(&heapBefore)
	if err := prepared.WriteTo(context.Background(), io.Discard); err != nil {
		t.Fatalf("WriteTo() error = %v", err)
	}
	runtime.KeepAlive(prepared)
	runtime.KeepAlive(src)

	if walked != entries {
		t.Fatalf("walked %d entries, want %d", walked, entries)
	}
	if peak == 0 {
		t.Fatal("never sampled the heap during the walk; the ceiling proved nothing")
	}

	var retained uint64
	if peak > heapBefore.HeapAlloc {
		retained = peak - heapBefore.HeapAlloc
	}
	// 248 bytes per entry is the format's own cost, stable across entry counts.
	// A retained SourceEntry adds roughly another 96 -- a string header, its
	// bytes, a kind, a size and a timestamp -- so a budget of 300 sits above
	// what ZIP requires and below what a second index would cost. A ceiling
	// loose enough to admit that index would look exactly like a passing test.
	const perEntryBudget = 300
	budget := uint64(entries) * perEntryBudget
	if retained > budget {
		t.Fatalf("live heap at the last of %d entries grew by %d bytes (%d per entry), want at most %d",
			entries, retained, retained/uint64(entries), budget)
	}
}
