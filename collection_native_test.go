package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

// readCollectionZIP downloads a staged collection over real HTTP and returns
// every file entry's exact content keyed by archive name.
func readCollectionZIP(t *testing.T, observed *inspectedNativeSource, url string, wantEntries int) map[string]string {
	t.Helper()
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Post(url, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("collection download failed: status=%d body error=%v", response.StatusCode, err)
	}
	if response.Header.Get("Content-Length") != "" {
		t.Fatal("collection ZIP claimed known wire length")
	}
	mediaType, params, err := mime.ParseMediaType(response.Header.Get("Content-Disposition"))
	if err != nil || mediaType != "attachment" || params["filename"] != "FairDrop.zip" {
		t.Fatalf("collection attachment = %q, %v", response.Header.Get("Content-Disposition"), err)
	}
	archive, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatalf("collection download failed: the ZIP is unreadable: %v", err)
	}
	files := map[string]string{}
	for _, entry := range archive.File {
		if strings.HasSuffix(entry.Name, "/") {
			continue
		}
		opened, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(opened)
		_ = opened.Close()
		if err != nil {
			t.Fatalf("ZIP entry %q: %v", entry.Name, err)
		}
		files[entry.Name] = string(content)
	}
	if len(files) != wantEntries {
		t.Fatalf("collection ZIP holds %d files, want %d", len(files), wantEntries)
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case event := <-observed.events:
			if event.Kind == transfer.TransferError {
				t.Fatalf("collection failed after HTTP completion: %v", event)
			}
			if event.Kind == transfer.TransferComplete {
				if event.Progress == nil || event.Progress.BytesSent != int64(len(body)) {
					t.Fatalf("completion progress = %+v", event.Progress)
				}
				return files
			}
		case <-deadline:
			t.Fatal("collection HTTP completion event missing")
		}
	}
}

// The directory-handle budget belongs to ONE folder at a time. A folder at the
// deepest lexical depth the single-folder path admits must stream, and so must
// a dozen of them in one collection, because members are prepared one after
// another and the selected-item count never consumes the budget. A folder one
// level deeper is still refused at Stage, exactly as when it is sent alone.
func TestNativeCollectionManyFoldersAtTheSingleFolderBoundaryStreamTogether(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	inspector := source.New()
	var components []string
	var admittedDepth int
	for depth := 1; depth <= 64; depth++ {
		components = append(components, "d")
		deep := filepath.Join(append([]string{base}, components...)...)
		if err := os.MkdirAll(deep, 0o700); err != nil {
			t.Fatal(err)
		}
		_, inspectErr := inspector.Inspect(context.Background(), deep)
		if transfer.ErrorCodeOf(inspectErr) == transfer.ErrPathUnsupported {
			break
		}
		if inspectErr != nil {
			t.Fatalf("unexpected pre-boundary refusal: %v", inspectErr)
		}
		admittedDepth = depth
	}
	if admittedDepth == 0 || admittedDepth >= 64 {
		t.Fatal("native fixture missed the single-folder handle boundary")
	}
	refused := filepath.Join(append([]string{base}, components...)...)
	if err := os.WriteFile(filepath.Join(refused, "too-deep.txt"), []byte("too deep"), 0o600); err != nil {
		t.Fatal(err)
	}
	topFile := filepath.Join(base, "top.txt")
	if err := os.WriteFile(topFile, []byte("top"), 0o600); err != nil {
		t.Fatal(err)
	}
	shallow := filepath.Join(base, "shallow")
	if err := os.Mkdir(shallow, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shallow, "one.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A folder one level beyond the single-folder boundary is refused whether
	// it is sent alone or inside a collection, and nothing reaches the network.
	failedApp, failedSource := nativeMatrixApp(t)
	if _, err := failedApp.StageTransfer(refused); transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
		t.Fatalf("singleton folder beyond the boundary admitted: %v", err)
	}
	if _, err := failedApp.StageTransfers([]string{shallow, refused, topFile}); transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
		t.Fatalf("collection folder beyond the boundary admitted: %v", err)
	}
	if failedSource.networkCalls.Load() != 0 {
		t.Fatal("directory budget refusal reached network")
	}

	// The over-deep folder is the admitted one's only child; remove it so the
	// admitted folder is an unchanged tree at exactly the boundary.
	if err := os.RemoveAll(refused); err != nil {
		t.Fatal(err)
	}

	// A dozen roots at exactly the admitted depth: the same number of path
	// components, a different first component each.
	const deepRoots = 12
	admitted := filepath.Join(append([]string{base}, components[:admittedDepth]...)...)
	if err := os.WriteFile(filepath.Join(admitted, "deep.txt"), []byte("deep-0"), 0o600); err != nil {
		t.Fatal(err)
	}
	roots := []string{shallow, admitted}
	want := map[string]string{
		"FairDrop/01-shallow/one.txt": "one",
		"FairDrop/02-d/deep.txt":      "deep-0",
	}
	for index := 1; index < deepRoots; index++ {
		leaf := filepath.Join(append([]string{base, fmt.Sprintf("x%d", index)}, components[:admittedDepth-1]...)...)
		if err := os.MkdirAll(leaf, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(leaf, "deep.txt"), []byte(fmt.Sprintf("deep-%d", index)), 0o600); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, leaf)
		want[fmt.Sprintf("FairDrop/%02d-d/deep.txt", len(roots))] = fmt.Sprintf("deep-%d", index)
	}
	roots = append(roots, topFile)
	want[fmt.Sprintf("FairDrop/%02d-top.txt", len(roots))] = "top"

	app, observed := nativeMatrixApp(t)
	metadata, err := app.StageTransfers(roots)
	if err != nil {
		t.Fatalf("unchanged collection of folders at the boundary refused: %v", err)
	}
	if !metadata.IsCollection || metadata.ItemCount != len(roots) || metadata.Name != fmt.Sprintf("%d items", len(roots)) {
		t.Fatalf("collection metadata = %+v", metadata)
	}
	got := readCollectionZIP(t, observed, metadata.URL, len(want))
	for name, content := range want {
		if got[name] != content {
			t.Fatalf("ZIP entry %q = %q, want %q", name, got[name], content)
		}
	}
}

// A collection far larger than the old 16-root cap goes through the real
// bound command, coordinator, source, server and payload adapter, then HTTP.
func TestNativeCollectionOfHundredsOfItemsStagesAndDownloads(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const files, folders = 120, 60
	var roots []string
	want := map[string]string{}
	for index := 0; index < files; index++ {
		path := filepath.Join(base, fmt.Sprintf("file-%03d.txt", index))
		content := fmt.Sprintf("file %d", index)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, path)
		want[fmt.Sprintf("FairDrop/%03d-file-%03d.txt", len(roots), index)] = content
	}
	for index := 0; index < folders; index++ {
		path := filepath.Join(base, fmt.Sprintf("folder-%03d", index))
		if err := os.MkdirAll(filepath.Join(path, "inner"), 0o700); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf("folder %d", index)
		if err := os.WriteFile(filepath.Join(path, "inner", "x.txt"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		roots = append(roots, path)
		want[fmt.Sprintf("FairDrop/%03d-folder-%03d/inner/x.txt", len(roots), index)] = content
	}
	app, observed := nativeMatrixApp(t)
	metadata, err := app.StageTransfers(roots)
	if err != nil {
		t.Fatalf("180-item collection refused: %v", err)
	}
	if !metadata.IsCollection || metadata.ItemCount != 180 || metadata.Name != "180 items" {
		t.Fatalf("collection metadata = %+v", metadata)
	}
	got := readCollectionZIP(t, observed, metadata.URL, len(want))
	for name, content := range want {
		if got[name] != content {
			t.Fatalf("ZIP entry %q = %q, want %q", name, got[name], content)
		}
	}
	if _, err := app.StageTransfers(make([]string, 10001)); transfer.ErrorCodeOf(err) != transfer.ErrInvalidSelection {
		t.Fatalf("10,001 paths = %v, want invalid_selection", err)
	}
}

func TestNativeCollectionRejectsCanonicalAncestorAliasBeforeNetwork(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	actual := filepath.Join(base, "actual")
	if err := os.Mkdir(actual, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(actual, "report.txt")
	if err := os.WriteFile(file, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	nativeMatrixDirectoryLink(t, actual, alias)
	app, observed := nativeMatrixApp(t)
	if _, err := app.StageTransfers([]string{file, filepath.Join(alias, "report.txt")}); transfer.ErrorCodeOf(err) != transfer.ErrInvalidSelection {
		t.Fatalf("canonical alias pair = %v, want invalid_selection", err)
	}
	if observed.networkCalls.Load() != 0 {
		t.Fatal("canonical alias duplicate reached networking")
	}
}

func TestNativeCollectionReplacedLaterRootAbortsAuthorizedHTTP(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(base, "first.txt")
	if err := os.WriteFile(first, bytes.Repeat([]byte("initial bytes "), 5000), 0o600); err != nil {
		t.Fatal(err)
	}
	later := filepath.Join(base, "later")
	if err := os.Mkdir(later, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(later, "old.txt"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	app, observed := nativeMatrixApp(t)
	metadata, err := app.StageTransfers([]string{first, later})
	if err != nil {
		t.Fatal(err)
	}
	mutated := make(chan error, 1)
	observed.afterPrepareDirectory = func(path string) {
		if path != later {
			return
		}
		if err := os.Rename(later, later+"-original"); err != nil {
			mutated <- err
			return
		}
		if err := os.Mkdir(later, 0o700); err != nil {
			mutated <- err
			return
		}
		mutated <- os.WriteFile(filepath.Join(later, "substitute.txt"), []byte("substituted"), 0o600)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Post(metadata.URL, "", nil)
	if err == nil {
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr == nil {
			if _, zipErr := zip.NewReader(bytes.NewReader(body), int64(len(body))); zipErr == nil {
				t.Fatal("replaced later root finalized a readable ZIP")
			}
		}
	}
	select {
	case err := <-mutated:
		if err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("claim did not prepare and replace later directory")
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case event := <-observed.events:
			if event.Kind == transfer.TransferComplete {
				t.Fatal("replaced root published transfer completion")
			}
			if event.Kind == transfer.TransferError {
				return
			}
		case <-deadline:
			t.Fatal("replaced root published no terminal failure")
		}
	}
}
