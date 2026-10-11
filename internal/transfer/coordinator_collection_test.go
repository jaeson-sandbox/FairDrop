package transfer

import (
	"context"
	"reflect"
	"runtime"
	"strconv"
	"testing"
)

func TestCollectionCountRefusalPrecedesFilesystemAndNetwork(t *testing.T) {
	// Literal counts, not the exported constant: a test that compares the
	// constant to itself pins nothing.
	for _, count := range []int{0, 10001} {
		h := newHarness(t)
		paths := make([]string, count)
		for index := range paths {
			paths[index] = "/selected/item-" + strconv.Itoa(index)
		}
		_, err := h.coordinator.StageTransfers(context.Background(), paths)
		if ErrorCodeOf(err) != ErrInvalidSelection {
			t.Fatalf("count %d: %v", count, err)
		}
		if got := h.calls.snapshot(); len(got) != 0 {
			t.Fatalf("count %d reached external work: %v", count, got)
		}
	}
}

func TestCollectionCapIsExactlyTenThousand(t *testing.T) {
	if MaxCollectionItems != 10000 {
		t.Fatalf("collection cap = %d, want 10000", MaxCollectionItems)
	}
}

func TestCollectionAdmitsTenThousandRootsAndInspectsEachOnce(t *testing.T) {
	h := newHarness(t)
	h.source.inspect = func(_ context.Context, path string) (StagedItem, error) {
		item := testItem()
		item.Path = path
		item.Name = path[len("/selected/"):]
		return item, nil
	}
	paths := make([]string, 10000)
	for index := range paths {
		paths[index] = "/selected/item-" + strconv.Itoa(index)
	}
	metadata, err := h.coordinator.StageTransfers(context.Background(), paths)
	if err != nil {
		t.Fatalf("10,000 roots refused: %v", err)
	}
	if !metadata.IsCollection || metadata.ItemCount != 10000 || metadata.Name != "10000 items" || metadata.Size != testSize*10000 {
		t.Fatalf("10,000-root metadata = %+v", metadata)
	}
	// One Inspect per root, in order: no second count-aware pass.
	got := h.source.inspected()
	if len(got) != 10000 || got[0] != "/selected/item-0" || got[9999] != "/selected/item-9999" {
		t.Fatalf("Stage inspected %d paths (first %q); want exactly one per root", len(got), got[0])
	}
}

func TestCollectionStageInspectsEachFolderOnceWithoutAccumulatedReservations(t *testing.T) {
	h := newHarness(t)
	h.source.inspect = func(_ context.Context, path string) (StagedItem, error) {
		item := testItem()
		item.Path = path
		item.Kind = ItemDirectory
		return item, nil
	}
	paths := []string{"/a/one", "/b/two", "/c/three"}
	if _, err := h.coordinator.StageTransfers(context.Background(), paths); err != nil {
		t.Fatal(err)
	}
	if got := h.source.inspected(); !reflect.DeepEqual(got, paths) {
		t.Fatalf("three folders were inspected %v; want one Inspect each, in order", got)
	}
}

func TestCollectionRejectsDuplicateAndAncestorSelections(t *testing.T) {
	for _, paths := range [][]string{{"/a/report", "/a/report"}, {"/a", "/a/report"}, {"/a/report", "/a"}, {"/", "/a"}} {
		h := newHarness(t)
		h.source.inspect = func(_ context.Context, path string) (StagedItem, error) {
			item := testItem()
			item.Path = path
			return item, nil
		}
		_, err := h.coordinator.StageTransfers(context.Background(), paths)
		if ErrorCodeOf(err) != ErrInvalidSelection {
			t.Fatalf("paths %v: %v", paths, err)
		}
		if h.calls.count("network.GetLocalIP") != 0 || h.calls.count("server.Start") != 0 {
			t.Fatalf("paths %v acquired network resources", paths)
		}
	}
}

// overlaps reports whether b overlaps a once a has been admitted.
func overlaps(a, b string) bool {
	index := newOverlapIndex()
	if !index.add(a) {
		panic("first path cannot overlap an empty index")
	}
	return !index.add(b)
}

func TestSelectionOverlapRespectsRootAndComponentBoundaries(t *testing.T) {
	if !overlaps("/", "/a/b") || !overlaps("/a/b", "/") {
		t.Fatal("filesystem root did not contain child")
	}
	if overlaps("/a/b", "/a/b2") || overlaps("/a/b2", "/a/b") || overlaps("/a/b-x", "/a/b/c") {
		t.Fatal("component prefix mistaken for ancestor")
	}
	if !overlaps("/a/b", "/a/b/c/d") || !overlaps("/a/b/c/d", "/a/b") {
		t.Fatal("deep descendant missed")
	}
	if !overlaps("/a/b/", "/a/b") || !overlaps("/a/./b", "/a/b") {
		t.Fatal("equivalent spelling missed")
	}
}

func TestSelectionOverlapIndexIsOrderIndependentAcrossManyRoots(t *testing.T) {
	// A later ancestor must find an earlier descendant among thousands of
	// unrelated siblings, and a failed add must record nothing.
	index := newOverlapIndex()
	for i := 0; i < 5000; i++ {
		if !index.add("/data/sibling-" + strconv.Itoa(i) + "/file") {
			t.Fatalf("sibling %d falsely overlapped", i)
		}
	}
	if index.add("/data/sibling-4999") {
		t.Fatal("ancestor of an earlier root was admitted")
	}
	if !index.add("/data/sibling-4999-extra") {
		t.Fatal("a refused add left residue that blocked a legitimate sibling")
	}
	if index.add("/data/sibling-0/file") {
		t.Fatal("duplicate admitted")
	}
	if index.add("/data/sibling-1/file/deeper") {
		t.Fatal("descendant of an earlier root was admitted")
	}
}

func TestWindowsSelectionOverlapUsesCaseFoldAndVolumeComponents(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows filepath behavior")
	}
	for _, pair := range [][2]string{
		{`C:\`, `c:\share\reports`},
		{`C:\Share\Reports`, `c:\share\reports`},
		{`C:\Share`, `c:\share\Reports`},
		{`\\server\share\Reports`, `\\SERVER\SHARE\reports\file.txt`},
	} {
		if !overlaps(pair[0], pair[1]) || !overlaps(pair[1], pair[0]) {
			t.Fatalf("Windows alias overlap missed: %q %q", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{
		{`C:\Share`, `C:\Shared`},
		{`C:\Share`, `D:\Share\file.txt`},
		{`\\server\share`, `\\server\share2\file.txt`},
		{`\\server\share`, `\\server2\share\file.txt`},
	} {
		if overlaps(pair[0], pair[1]) || overlaps(pair[1], pair[0]) {
			t.Fatalf("Windows volume/component false overlap: %q %q", pair[0], pair[1])
		}
	}
}

func TestCollectionCheckedSumAndCallerSliceOwnership(t *testing.T) {
	h := newHarness(t)
	paths := []string{"/a/one", "/b/two"}
	h.source.inspect = func(_ context.Context, path string) (StagedItem, error) {
		if path == "/a/one" {
			paths[1] = "/malicious/changed"
		}
		item := testItem()
		item.Path = path
		item.LogicalSize = 4503599627370496
		return item, nil
	}
	_, err := h.coordinator.StageTransfers(context.Background(), paths)
	if ErrorCodeOf(err) != ErrPathUnsupported {
		t.Fatalf("overflow = %v", err)
	}
	if got := h.source.inspected(); !reflect.DeepEqual(got, []string{"/a/one", "/b/two"}) {
		t.Fatalf("caller mutated admitted paths: %v", got)
	}
	if h.calls.count("network.GetLocalIP") != 0 {
		t.Fatal("overflow reached network")
	}
}

func TestCollectionCopiesServerBoundaryAndPublishesAggregateOnly(t *testing.T) {
	h := newHarness(t)
	h.source.inspect = func(_ context.Context, path string) (StagedItem, error) {
		item := testItem()
		item.Path = path
		item.Name = "report.txt"
		return item, nil
	}
	metadata, err := h.coordinator.StageTransfers(context.Background(), []string{"/a/report.txt", "/b/report.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if !metadata.IsCollection || metadata.ItemCount != 2 || metadata.Name != "2 items" || metadata.Size != testSize*2 || metadata.IsDir {
		t.Fatalf("aggregate metadata = %+v", metadata)
	}
	h.server.mu.Lock()
	request := h.server.requests[0]
	request.Item.Collection.Members[0].Path = "/mutated/server"
	h.server.mu.Unlock()
	live := h.liveSession()
	if live.item.Collection.Members[0].Path != "/a/report.txt" {
		t.Fatalf("server mutated coordinator ownership: %q", live.item.Collection.Members[0].Path)
	}
}
