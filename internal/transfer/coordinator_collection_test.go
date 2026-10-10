package transfer

import (
	"context"
	"reflect"
	"runtime"
	"strconv"
	"testing"
)

func TestCollectionCountRefusalPrecedesFilesystemAndNetwork(t *testing.T) {
	for _, count := range []int{0, 17} {
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

func TestSelectionOverlapRespectsRootAndComponentBoundaries(t *testing.T) {
	if !selectionsOverlap("/", "/a/b") {
		t.Fatal("filesystem root did not contain child")
	}
	if selectionsOverlap("/a/b", "/a/b2") {
		t.Fatal("component prefix mistaken for ancestor")
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
		if !selectionsOverlap(pair[0], pair[1]) {
			t.Fatalf("Windows alias overlap missed: %q %q", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]string{
		{`C:\Share`, `C:\Shared`},
		{`C:\Share`, `D:\Share\file.txt`},
		{`\\server\share`, `\\server\share2\file.txt`},
		{`\\server\share`, `\\server2\share\file.txt`},
	} {
		if selectionsOverlap(pair[0], pair[1]) {
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
