package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fairdrop/internal/qr"
	"fairdrop/internal/server"
	"fairdrop/internal/sink"
	"fairdrop/internal/source"
	"fairdrop/internal/stream"
	"fairdrop/internal/transfer"
)

type recoveryLoopbackNetwork struct{}

func (recoveryLoopbackNetwork) GetLocalIP(context.Context) (netip.Addr, error) {
	return netip.MustParseAddr("127.0.0.1"), nil
}
func (recoveryLoopbackNetwork) StartBeacon(context.Context, transfer.BeaconRequest) error { return nil }
func (recoveryLoopbackNetwork) StopBeacon() error                                         { return nil }
func (recoveryLoopbackNetwork) Options(context.Context) ([]transfer.ConnectionOption, error) {
	return []transfer.ConnectionOption{{ID: "loopback", Name: "Test network", Address: "127.0.0.1"}}, nil
}
func (recoveryLoopbackNetwork) SelectEndpoint(context.Context, string) (netip.Addr, error) {
	return netip.MustParseAddr("127.0.0.1"), nil
}

type recoveryDiscardObserver struct{}

func (recoveryDiscardObserver) Publish(transfer.Event) {}

func TestRecoveredHTTPRejectsOldTokenAndSendsExactFile(t *testing.T) {
	root, err := os.MkdirTemp(".", ".fairdrop-recovery-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	path, err := filepath.Abs(filepath.Join(root, "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("fresh connection delivers these exact bytes\x00\xff")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	inspector := source.New()
	c := transfer.NewCoordinator(transfer.Dependencies{Source: newSelectionSource(inspector), Network: recoveryLoopbackNetwork{},
		Server: server.New(stream.New(inspector)), QR: qr.New(), Observer: recoveryDiscardObserver{}})
	t.Cleanup(func() { _ = c.Shutdown(context.Background()) })
	old, err := c.Stage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := c.RefreshConnection(context.Background(), old.SessionID, "loopback")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Send == nil || fresh.Send.URL == old.URL {
		t.Fatalf("replacement = %+v", fresh)
	}
	client := &http.Client{}
	// Exercise the exact URL the old code displayed, even when its retired
	// listener answers with a connection refusal instead of an HTTP response.
	originalResponse, originalErr := client.Get(old.URL)
	if originalErr == nil {
		_ = originalResponse.Body.Close()
		if originalResponse.StatusCode == http.StatusOK {
			t.Fatal("original retired URL still served a payload")
		}
	}
	oldToken := old.URL[strings.LastIndex(old.URL, "/")+1:]
	retiredURL := fresh.Send.URL[:strings.LastIndex(fresh.Send.URL, "/")+1] + oldToken
	oldResponse, err := client.Get(retiredURL)
	if err != nil {
		t.Fatal(err)
	}
	_ = oldResponse.Body.Close()
	if oldResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("old URL status = %d", oldResponse.StatusCode)
	}
	request, err := http.NewRequest(http.MethodPost, fresh.Send.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	got, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || string(got) != string(content) {
		t.Fatalf("response = %d %q", response.StatusCode, got)
	}
}

func TestRecoveredReceiveHTTPWritesOnlyAfterFreshClaimToApprovedFolder(t *testing.T) {
	dir := nativeFixture(t)
	inspector := source.New()
	app := NewApp()
	app.logf = func(string, ...any) {}
	app.startup(context.Background())
	c := transfer.NewCoordinator(transfer.Dependencies{Source: inspector, Network: recoveryLoopbackNetwork{},
		Server: server.New(stream.New(inspector)), QR: qr.New(), Sink: sink.New(), Observer: recoveryDiscardObserver{}})
	app.useCoordinator(c)
	t.Cleanup(func() { _ = c.Shutdown(context.Background()) })
	old, err := app.StartReceive(dir)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{}
	page, err := client.Get(old.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = page.Body.Close()
	if page.StatusCode != http.StatusOK || len(nativeListing(t, dir)) != 0 {
		t.Fatal("GET created receive output before claim")
	}
	fresh, err := app.RefreshConnection(string(old.SessionID), "loopback")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Receive == nil || fresh.Receive.SessionID == old.SessionID || len(nativeListing(t, dir)) != 0 {
		t.Fatalf("receive refresh = %+v; output=%v", fresh, nativeListing(t, dir))
	}
	oldToken := old.URL[strings.LastIndex(old.URL, "/")+1:]
	retiredURL := fresh.Receive.URL[:strings.LastIndex(fresh.Receive.URL, "/")+1] + oldToken
	bad, err := http.NewRequest(http.MethodPost, retiredURL, bytes.NewReader(nativeMultipart([2]string{"old.txt", "must not save"})))
	if err != nil {
		t.Fatal(err)
	}
	bad.Header.Set("Content-Type", "multipart/form-data; boundary="+nativeBoundary)
	badResponse, err := client.Do(bad)
	if err != nil {
		t.Fatal(err)
	}
	_ = badResponse.Body.Close()
	if badResponse.StatusCode != http.StatusNotFound || len(nativeListing(t, dir)) != 0 {
		t.Fatalf("retired upload status=%d output=%v", badResponse.StatusCode, nativeListing(t, dir))
	}
	good, err := http.NewRequest(http.MethodPost, fresh.Receive.URL, bytes.NewReader(nativeMultipart([2]string{"new.txt", "fresh exact bytes"})))
	if err != nil {
		t.Fatal(err)
	}
	good.Header.Set("Content-Type", "multipart/form-data; boundary="+nativeBoundary)
	goodResponse, err := client.Do(good)
	if err != nil {
		t.Fatal(err)
	}
	_ = goodResponse.Body.Close()
	if goodResponse.StatusCode != http.StatusOK {
		t.Fatalf("fresh upload status=%d", goodResponse.StatusCode)
	}
	subfolders := nativeListing(t, dir)
	if len(subfolders) != 1 {
		t.Fatalf("approved destination = %v", subfolders)
	}
	content, err := os.ReadFile(filepath.Join(dir, subfolders[0], "new.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "fresh exact bytes" {
		t.Fatalf("saved bytes = %q", content)
	}
}
