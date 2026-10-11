package server

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"
)

// TestBrowserUploadFixture holds a real receive server, sink and directory open
// for the test-only Playwright runner (frontend/browser/upload-live.mjs). Ordinary
// Go runs skip it. The runner closes stdin when the browser assertions finish.
func TestBrowserUploadFixture(t *testing.T) {
	if os.Getenv("FAIRDROP_BROWSER_UPLOAD_LIVE") != "1" {
		t.Skip("browser harness only")
	}
	rig := startReceiveRig(t)
	fmt.Println("FAIRDROP_UPLOAD_URL=" + rig.uploadURL())
	fmt.Println("FAIRDROP_UPLOAD_DIR=" + rig.dir)
	finished := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, os.Stdin)
		close(finished)
	}()
	select {
	case <-finished: // The harness closed stdin; cleanup stops the server.
	case <-time.After(2 * time.Minute):
		t.Fatal("browser harness did not release the fixture within two minutes")
	}
}
