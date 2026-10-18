package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"fairdrop/internal/transfer"
)

// TestBrowserLiveFixture holds a real production HTTP handler open for the
// test-only Playwright runner. Ordinary Go runs skip it. The runner terminates
// this process when the browser assertion finishes.
func TestBrowserLiveFixture(t *testing.T) {
	if os.Getenv("FAIRDROP_BROWSER_RECEIVER_LIVE") != "1" {
		t.Skip("browser harness only")
	}
	server := newTestServer(t, payloadsReturning(&stubPayload{
		name: "quarterly report.pdf", size: 12, known: true,
		stream: bodyOf([]byte("hello world!"), 4),
	}))
	handle := startTestServer(t, server, &stubAuthorizer{})
	fmt.Println("FAIRDROP_RECEIVER_URL=" + downloadURL(handle.Port, string(testToken)))
	longServer := newTestServer(t, &stubPayloads{})
	longRequest := startRequest()
	longRequest.Item.Name = strings.Repeat("長", 240) + "שלום<unsafe>"
	longHandle, err := longServer.Start(context.Background(), longRequest, &stubAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = longServer.Stop() })
	fmt.Println("FAIRDROP_RECEIVER_LONG_URL=" + downloadURL(longHandle.Port, string(testToken)))
	textValue := "\nA <b>literal</b> URL: https://example.test/?a=1&b=2"
	textServer := newTestServer(t, nil)
	textHandle, err := textServer.Start(context.Background(), transfer.ServerStartRequest{SessionID: testSession, Token: testToken, Text: &textValue}, &stubAuthorizer{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = textServer.Stop() })
	fmt.Println("FAIRDROP_RECEIVER_TEXT_URL=" + baseURL(textHandle.Port) + "/text/" + string(testToken))
	receiveOwner := &receivedTextOwner{}
	receiveServer := newTestServer(t, nil)
	receiveHandle, err := receiveServer.Start(context.Background(), transfer.ServerStartRequest{
		SessionID: testSession, Token: testToken, ReceiveText: true,
	}, receiveOwner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = receiveServer.Stop() })
	fmt.Println("FAIRDROP_RECEIVER_RECEIVE_TEXT_URL=" + baseURL(receiveHandle.Port) + "/receive-text/" + string(testToken))
	finished := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		close(finished)
	}()
	select {
	case <-finished: // Harness closed stdin; cleanup closes both listeners.
	case <-time.After(2 * time.Minute):
		t.Fatal("browser harness did not release the fixture within two minutes")
	}
	if claims, values := receiveOwner.snapshot(); claims != 1 || len(values) != 1 ||
		values[0] != "First line\n雪 & <tag> https://example.test/?a=1&b=2" {
		t.Fatalf("browser form submitted claims=%d values=%q", claims, values)
	}
}
