package main

import (
	"archive/zip"
	"bytes"
	"context"
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

func TestNativeCollectionSharedDirectoryBoundaryAndHTTP(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	shallow := filepath.Join(base, "shallow")
	if err := os.Mkdir(shallow, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shallow, "one.txt"), []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	topFile := filepath.Join(base, "top.txt")
	if err := os.WriteFile(topFile, []byte("top"), 0o600); err != nil {
		t.Fatal(err)
	}
	inspector := source.New()
	deep := base
	var admitted, refused string
	for depth := 0; depth < 64; depth++ {
		deep = filepath.Join(deep, "d")
		if err := os.Mkdir(deep, 0o700); err != nil {
			t.Fatal(err)
		}
		_, aloneErr := inspector.InspectWithRetained(context.Background(), deep, 0)
		_, sharedErr := inspector.InspectWithRetained(context.Background(), deep, 1)
		if aloneErr == nil && transfer.ErrorCodeOf(sharedErr) == transfer.ErrPathUnsupported {
			refused = deep
			break
		}
		if sharedErr != nil {
			t.Fatalf("unexpected pre-boundary refusal: %v", sharedErr)
		}
		admitted = deep
	}
	if admitted == "" || refused == "" {
		t.Fatal("native fixture missed shared handle boundary")
	}
	if err := os.WriteFile(filepath.Join(admitted, "deep.txt"), []byte("deep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(refused, "too-deep.txt"), []byte("too deep"), 0o600); err != nil {
		t.Fatal(err)
	}

	failedApp, failedSource := nativeMatrixApp(t)
	if _, err := failedApp.StageTransfers([]string{shallow, refused, topFile}); transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
		t.Fatalf("unreservable third directory depth admitted: %v", err)
	}
	if failedSource.networkCalls.Load() != 0 {
		t.Fatal("directory budget refusal reached network")
	}
	if err := os.RemoveAll(refused); err != nil {
		t.Fatal(err)
	}

	app, observed := nativeMatrixApp(t)
	metadata, err := app.StageTransfers([]string{shallow, admitted, topFile})
	if err != nil {
		t.Fatalf("unchanged collection at boundary refused: %v", err)
	}
	if !metadata.IsCollection || metadata.ItemCount != 3 || metadata.Name != "3 items" {
		t.Fatalf("collection metadata = %+v", metadata)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Post(metadata.URL, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("collection HTTP status=%d body error=%v", response.StatusCode, err)
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
		t.Fatal(err)
	}
	want := map[string]string{"FairDrop/01-shallow/one.txt": "one", "FairDrop/02-d/deep.txt": "deep", "FairDrop/03-top.txt": "top"}
	for _, entry := range archive.File {
		if strings.HasSuffix(entry.Name, "/") {
			continue
		}
		expected, ok := want[entry.Name]
		if !ok {
			t.Fatalf("unexpected ZIP entry %q", entry.Name)
		}
		opened, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(opened)
		_ = opened.Close()
		if err != nil || string(content) != expected {
			t.Fatalf("ZIP entry %q = %q, %v", entry.Name, content, err)
		}
		delete(want, entry.Name)
	}
	if len(want) != 0 {
		t.Fatalf("missing collection entries: %v", want)
	}
	deadline := time.After(5 * time.Second)
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
				return
			}
		case <-deadline:
			t.Fatal("collection HTTP completion event missing")
		}
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
