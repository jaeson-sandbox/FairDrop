package server

import (
	"context"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"

	"fairdrop/internal/transfer"
)

type textOnlySource struct{}

func (textOnlySource) Inspect(context.Context, string) (transfer.StagedItem, error) {
	panic("text inspected source")
}
func (textOnlySource) PrepareDirectory(context.Context, string) (transfer.PreparedDirectory, error) {
	panic("text prepared directory")
}
func (textOnlySource) Walk(context.Context, string, transfer.SourceVisitor) error {
	panic("text walked source")
}

type textLoopbackNetwork struct{}

func (textLoopbackNetwork) GetLocalIP(context.Context) (netip.Addr, error) {
	return netip.MustParseAddr("127.0.0.1"), nil
}
func (textLoopbackNetwork) StartBeacon(context.Context, transfer.BeaconRequest) error { return nil }
func (textLoopbackNetwork) StopBeacon() error                                         { return nil }
func (textLoopbackNetwork) Options(context.Context) ([]transfer.ConnectionOption, error) {
	return []transfer.ConnectionOption{{ID: "loopback", Name: "Loopback", Address: "127.0.0.1"}}, nil
}
func (textLoopbackNetwork) SelectEndpoint(context.Context, string) (netip.Addr, error) {
	return netip.MustParseAddr("127.0.0.1"), nil
}

type textQR struct{}

func (textQR) EncodePNG(context.Context, string) ([]byte, error) { return []byte{1, 2, 3}, nil }

type textObserver struct{}

func (textObserver) Publish(transfer.Event) {}

func TestTextRefreshRetiresActualHTTPRoute(t *testing.T) {
	server := newTestServer(t, nil)
	coordinator := transfer.NewCoordinator(transfer.Dependencies{Source: textOnlySource{}, Network: textLoopbackNetwork{}, Server: server, QR: textQR{}, Observer: textObserver{}})
	t.Cleanup(func() { _ = coordinator.Shutdown(context.Background()) })
	first, err := coordinator.StageText(context.Background(), "one\r\ntwo")
	if err != nil {
		t.Fatal(err)
	}
	response := do(t, http.MethodGet, first.URL)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("first GET=%d", response.StatusCode)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	result, err := coordinator.RefreshConnection(context.Background(), first.SessionID, "")
	if err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if result.Text == nil || result.Text.URL == first.URL {
		t.Fatalf("no fresh text URL: %#v", result)
	}
	oldToken := first.URL[strings.LastIndexByte(first.URL, '/')+1:]
	newURLWithOldToken := result.Text.URL[:strings.LastIndexByte(result.Text.URL, '/')+1] + oldToken
	retired := do(t, http.MethodGet, newURLWithOldToken)
	if retired.StatusCode != http.StatusNotFound {
		t.Fatalf("retired token authorized at new endpoint: %d", retired.StatusCode)
	}
	oldResponse, oldErr := testClient().Get(first.URL)
	if oldErr == nil {
		defer oldResponse.Body.Close()
		if oldResponse.StatusCode == http.StatusOK {
			t.Fatal("old HTTP endpoint still serves GET")
		}
	}
	newResponse := do(t, http.MethodPost, result.Text.URL)
	body, err := io.ReadAll(newResponse.Body)
	if err != nil {
		t.Fatal(err)
	}
	if newResponse.StatusCode != http.StatusOK || textAreaValue(t, string(body)) != "one\ntwo" {
		t.Fatalf("replacement reveal=%d body=%q", newResponse.StatusCode, body)
	}
}
