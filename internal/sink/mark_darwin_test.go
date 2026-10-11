//go:build darwin

package sink

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

// Proven here and on the macOS CI job: a saved file really carries the
// quarantine attribute Gatekeeper reads, and its absence is not reported when it
// was applied.
func TestSavedFilesCarryTheQuarantineAttributeOnMacOS(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)
	save(t, dest, "photo.jpg", "bytes")

	buffer := make([]byte, 256)
	size, err := unix.Getxattr(filepath.Join(dir, fixedSubfolder, "photo.jpg"), "com.apple.quarantine", buffer)
	if err != nil {
		t.Fatalf("Getxattr(com.apple.quarantine) = %v, want the attribute present on a saved file", err)
	}
	value := string(buffer[:size])
	if !strings.HasPrefix(value, "0083;") || !strings.HasSuffix(value, ";FairDrop;") {
		t.Fatalf("quarantine value = %q, want 0083;<time>;FairDrop;", value)
	}
	if dest.Snapshot().MarkingFailed {
		t.Fatal("marking succeeded but the destination reports a marking failure")
	}
}
