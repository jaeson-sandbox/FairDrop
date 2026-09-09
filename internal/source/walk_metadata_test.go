package source

import (
	"context"
	"io"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

/*
Every field the visitor is handed carries the entry it describes.

`RelativePath`, `Kind` and the borrowed content were pinned; `Size` and
`ModTime` were not, on either kind. Zeroing either one left the whole suite
green, because nothing downstream read them back: the archive writes both
into the ZIP header and no test opened a header to look. A field the port
promises and nothing checks is a field that can quietly stop being true.
*/
func TestWalkReportsEachEntrysSizeAndModTime(t *testing.T) {
	t.Parallel()

	fileTime := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	dirTime := time.Date(2025, 11, 12, 13, 14, 15, 0, time.UTC)

	root := fakeDirectory("root")
	child := fakeDirectory("child")
	child.modTime = dirTime
	payload := fakeRegular("payload.bin", 4242)
	payload.modTime = fileTime
	child.add("payload.bin", payload)
	root.add("child", child)

	factory := newFakeFactory(pathPlan{anchor: "root", rootLabel: "root"}, root)

	sizes := map[string]int64{}
	times := map[string]time.Time{}
	kinds := map[string]transfer.ItemKind{}
	err := (&Inspector{handles: factory, sameFile: sameFakeFile}).Walk(
		context.Background(), "original",
		func(entry transfer.SourceEntry, _ io.Reader) error {
			sizes[entry.RelativePath] = entry.Size
			times[entry.RelativePath] = entry.ModTime
			kinds[entry.RelativePath] = entry.Kind
			return nil
		},
	)
	if err != nil {
		t.Fatalf("Walk() error = %v", err)
	}

	if got := kinds["child"]; got != transfer.ItemDirectory {
		t.Fatalf("child kind = %q, want %q", got, transfer.ItemDirectory)
	}
	if got := times["child"]; !got.Equal(dirTime) {
		t.Errorf("child ModTime = %v, want %v", got, dirTime)
	}
	if got := sizes["child/payload.bin"]; got != 4242 {
		t.Errorf("payload Size = %d, want 4242", got)
	}
	if got := times["child/payload.bin"]; !got.Equal(fileTime) {
		t.Errorf("payload ModTime = %v, want %v", got, fileTime)
	}
	assertFakeClosed(t, factory)
}
