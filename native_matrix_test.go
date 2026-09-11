package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fairdrop/internal/qr"
	"fairdrop/internal/server"
	"fairdrop/internal/source"
	"fairdrop/internal/stream"
	"fairdrop/internal/transfer"
)

type nativeMatrixNetwork struct{ calls *atomic.Int32 }

func nativeMatrixDirectoryLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	}
	if os.Getenv("CI") != "" {
		t.Fatal("native CI cannot create the required link fixture")
	}
	t.Skip("runner lacks directory symlink creation privilege; native CI requires this capability")
}

func (n nativeMatrixNetwork) GetLocalIP(context.Context) (netip.Addr, error) {
	if n.calls != nil {
		n.calls.Add(1)
	}
	return netip.MustParseAddr("127.0.0.1"), nil
}
func (nativeMatrixNetwork) StartBeacon(context.Context, transfer.BeaconRequest) error { return nil }
func (nativeMatrixNetwork) StopBeacon() error                                         { return nil }

type inspectedNativeSource struct {
	transfer.SourcePort
	mu           sync.Mutex
	first        string
	calls        atomic.Int32
	networkCalls atomic.Int32
	selection    *selectionSource
	events       chan transfer.Event
}

func (s *inspectedNativeSource) Inspect(ctx context.Context, path string) (transfer.StagedItem, error) {
	s.calls.Add(1)
	s.mu.Lock()
	if s.first == "" {
		s.first = path
	}
	s.mu.Unlock()
	return s.SourcePort.Inspect(ctx, path)
}

// Drive the bound command, real coordinator, source, server and payload over
// HTTP. Only network discovery is fixed to loopback; no external LAN required.
func nativeMatrixApp(t *testing.T) (*App, *inspectedNativeSource) {
	t.Helper()
	app := NewApp()
	app.logf = func(string, ...any) {}
	inspector := &inspectedNativeSource{SourcePort: source.New(), events: make(chan transfer.Event, 32)}
	inspector.selection = newSelectionSource(inspector)
	app.emit = func(_ context.Context, _ string, args ...any) { inspector.events <- args[0].(transfer.Event) }
	app.startup(context.Background())
	coordinator := transfer.NewCoordinator(transfer.Dependencies{
		Source: inspector.selection, Network: nativeMatrixNetwork{calls: &inspector.networkCalls}, Server: server.New(stream.New(inspector)),
		QR: qr.New(), Observer: appObserver{app: app},
	})
	app.useCoordinator(coordinator)
	t.Cleanup(func() {
		if err := coordinator.Shutdown(context.Background()); err != nil {
			t.Errorf("matrix shutdown code=%s", transfer.ErrorCodeOf(err))
		}
	})
	return app, inspector
}

func assertNativeDownload(t *testing.T, selected string, folder bool) {
	t.Helper()
	app, inspector := nativeMatrixApp(t)
	metadata, err := app.StageTransfer(selected)
	if err != nil {
		t.Fatalf("native stage refused: code=%s", transfer.ErrorCodeOf(err))
	}
	name := filepath.Base(selected)
	if metadata.Name != name || metadata.Size != 21 || metadata.IsDir != folder || metadata.SessionID == "" || metadata.QR == "" {
		t.Fatal("native returned metadata differs from selected fixture")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(metadata.URL)
	if err != nil {
		t.Fatal("native download request failed")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		// Keep the failure observable without logging a capability URL or a
		// selected path. A 200 alone does not prove HTTP framing completed.
		t.Fatalf("native download body incomplete: status=%d bytes=%d read_error_type=%T unexpected_eof=%t", response.StatusCode, len(body), err, errors.Is(err, io.ErrUnexpectedEOF))
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("native download incomplete: status=%d", response.StatusCode)
	}
	wantDownload := name
	if folder {
		wantDownload += ".zip"
	}
	mediaType, disposition, err := mime.ParseMediaType(response.Header.Get("Content-Disposition"))
	if err != nil || mediaType != "attachment" || disposition["filename"] != wantDownload {
		t.Fatal("native HTTP filename differs from selected leaf")
	}
	if folder {
		reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		if err != nil {
			t.Fatal("native folder ZIP invalid")
		}
		files := 0
		for _, entry := range reader.File {
			if entry.FileInfo().IsDir() {
				if entry.Name != name+"/" {
					t.Fatal("native ZIP root name differs from selected leaf")
				}
				continue
			}
			if entry.Name != name+"/"+nativeFixtureLeaf(selected) {
				t.Fatal("native ZIP entry name differs from Unicode/space leaf")
			}
			files++
			content, err := entry.Open()
			if err != nil {
				t.Fatalf("native fixture operation failed: %T", err)
			}
			got, err := io.ReadAll(content)
			content.Close()
			if err != nil || string(got) != "native matrix payload" {
				t.Fatal("native archive content differs or CRC failed")
			}
		}
		if files != 1 {
			t.Fatalf("native folder file count=%d, want 1", files)
		}
	} else if string(body) != "native matrix payload" {
		t.Fatal("native file content differs")
	}
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case event := <-inspector.events:
			if event.SessionID != metadata.SessionID {
				t.Fatal("native lifecycle session mismatch")
			}
			if event.Kind == transfer.TransferError || event.Kind == transfer.TransferReset {
				t.Fatal("native lifecycle ended without natural Complete")
			}
			if event.Kind == transfer.TransferComplete {
				if event.Progress == nil || event.Progress.BytesSent != int64(len(body)) {
					t.Fatal("native Complete byte count differs from received response")
				}
				return
			}
		case <-deadline.C:
			t.Fatal("native lifecycle omitted matching natural Complete before cleanup")
		}
	}
}

func nativeFixtureLeaf(directory string) string {
	// The path-class fixtures choose the child name independently of the parent.
	switch filepath.Base(directory) {
	case "a folder with spaces":
		return "a report with spaces.txt"
	case "日本語 résumé 🌍":
		return "报告 résumé 🌍.txt"
	default:
		return "report.txt"
	}
}

func TestNativePathClassesStageAndDownload(t *testing.T) {
	for _, name := range []string{"spaces", "non-ASCII", "over-260"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			switch name {
			case "spaces":
				base = filepath.Join(base, "a folder with spaces")
			case "non-ASCII":
				base = filepath.Join(base, "日本語 résumé 🌍")
			case "over-260":
				for len(base) <= 280 {
					base = filepath.Join(base, "long-native-component")
				}
			}
			if err := os.MkdirAll(base, 0o700); err != nil {
				t.Fatal("native path fixture creation failed")
			}
			file := filepath.Join(base, nativeFixtureLeaf(base))
			if err := os.WriteFile(file, []byte("native matrix payload"), 0o600); err != nil {
				t.Fatal("native path fixture write failed")
			}
			t.Run("file", func(t *testing.T) { assertNativeDownload(t, file, false) })
			t.Run("folder", func(t *testing.T) { assertNativeDownload(t, base, true) })
		})
	}
}

func TestNativeUNCStageAndDownload(t *testing.T) {
	if runtime.GOOS != "windows" {
		app, _ := nativeMatrixApp(t)
		if _, err := app.StageTransfer(`\\server\share\report.txt`); transfer.ErrorCodeOf(err) != transfer.ErrInvalidSelection {
			t.Fatal("Windows UNC syntax on POSIX must be invalid_selection")
		}
		t.Log("UNC syntax is Windows-only; POSIX refusal checked. Mounted volumes use native absolute paths.")
		return
	}
	file, folder := os.Getenv("FAIRDROP_TEST_UNC_FILE"), os.Getenv("FAIRDROP_TEST_UNC_DIRECTORY")
	if file == "" || folder == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("native Windows CI must provision the UNC file and directory fixtures")
		}
		t.Skip("local UNC fixture unavailable: set FAIRDROP_TEST_UNC_FILE and FAIRDROP_TEST_UNC_DIRECTORY; CI provisions both")
	}
	t.Run("file", func(t *testing.T) { assertNativeDownload(t, file, false) })
	t.Run("folder", func(t *testing.T) { assertNativeDownload(t, folder, true) })
}

func TestStageTransferResolvesAncestorsBeforeInspect(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	if err := os.WriteFile(filepath.Join(target, "report.txt"), []byte("native matrix payload"), 0o600); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	alias := filepath.Join(base, "alias")
	nativeMatrixDirectoryLink(t, target, alias)
	selected := filepath.Join(alias, "report.txt")
	if _, err := source.New().Inspect(context.Background(), selected); transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
		t.Fatal("source's ancestor link refusal was weakened")
	}
	app, inspector := nativeMatrixApp(t)
	if _, err := app.StageTransfer(selected); err != nil {
		t.Fatalf("resolved stage failed: %s", transfer.ErrorCodeOf(err))
	}
	inspector.mu.Lock()
	first := inspector.first
	inspector.mu.Unlock()
	if first != filepath.Join(target, "report.txt") {
		t.Fatal("Inspect saw an unresolved path: resolution must follow admission and precede Inspect")
	}
	assertNativeDownload(t, selected, false)
	assertNativeDownload(t, alias+string(os.PathSeparator)+"."+string(os.PathSeparator)+"report.txt", false)
}

func TestStageTransferRefusesSelectedSymlinkAndPreservesTraversalRefusals(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	file := filepath.Join(target, "report.txt")
	if err := os.WriteFile(file, []byte("native matrix payload"), 0o600); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	link := filepath.Join(base, "selected-link")
	nativeMatrixDirectoryLink(t, target, link)
	sep := string(os.PathSeparator)
	for index, selected := range []string{link, link + sep, link + sep + ".", link + sep + "..", file + sep, file + sep + ".." + sep + "report.txt"} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			app, _ := nativeMatrixApp(t)
			if _, err := app.StageTransfer(selected); err == nil {
				t.Fatal("entry resolution bypassed an existing traversal refusal")
			}
		})
	}
}

func TestNativeEmptySelectionRemainsInvalid(t *testing.T) {
	app, _ := nativeMatrixApp(t)
	if _, err := app.StageTransfer(""); transfer.ErrorCodeOf(err) != transfer.ErrInvalidSelection {
		t.Fatal("empty selection must remain invalid_selection")
	}
}

func TestNativeWindowsDeviceNamespaceRefusalSurvivesResolution(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows namespace grammar is Windows-only")
	}
	file := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(file, []byte("native matrix payload"), 0o600); err != nil {
		t.Fatalf("native fixture operation failed: %T", err)
	}
	for _, prefix := range []string{`\\.\`, `\??\`, `\\?\GLOBALROOT\`} {
		selected := prefix + file
		if resolveSelectionAncestors(selected) != selected {
			t.Fatal("entry resolution changed a forbidden device namespace")
		}
		app, _ := nativeMatrixApp(t)
		if _, err := app.StageTransfer(selected); transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
			t.Fatal("unsupported Windows namespace must remain path_unsupported")
		}
	}
}

func TestNativeSingleInstanceDegradationReportsNoPathOrCause(t *testing.T) {
	app := NewApp()
	var diagnostic string
	app.logf = func(format string, args ...any) { diagnostic = fmt.Sprintf(format, args...) }
	if singleInstanceOption(app, func() bool { return false }) != nil {
		t.Fatal("unusable lock must not reach Wails")
	}
	if diagnostic != "fairdrop: single-instance protection unavailable (temporary lock file unusable); launching without protection" {
		t.Fatal("degraded launch must report only its fixed diagnostic")
	}
	if option := singleInstanceOption(app, func() bool { return true }); option == nil || option.UniqueId != "d1766c78-45cf-4e6d-9f04-c3700ab32024" {
		t.Fatal("usable lock must retain fixed Wails identity")
	}
}

func TestDarwinSystemTemporaryAncestorStageAndDownload(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS system-directory alias coverage runs on macOS")
	}
	for _, root := range []string{"/tmp", "/var/tmp"} {
		t.Run(root, func(t *testing.T) {
			base, err := os.MkdirTemp(root, "fairdrop-native-matrix-")
			if err != nil {
				t.Fatal("system temp fixture unavailable")
			}
			t.Cleanup(func() { _ = os.RemoveAll(base) })
			selected := filepath.Join(base, "report.txt")
			if err := os.WriteFile(selected, []byte("native matrix payload"), 0o600); err != nil {
				t.Fatalf("native fixture operation failed: %T", err)
			}
			if !strings.HasPrefix(selected, root+"/") {
				t.Fatal("fixture lost the system alias")
			}
			assertNativeDownload(t, selected, false)
			assertNativeDownload(t, base, true)
		})
	}
}
