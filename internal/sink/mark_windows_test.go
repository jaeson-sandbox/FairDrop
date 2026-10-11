//go:build windows

package sink

import (
	"os"
	"path/filepath"
	"testing"
)

// Proven on the Windows CI job: a saved file carries a Zone.Identifier alternate
// data stream with ZoneId=3, which is the Mark of the Web. The stream is written
// to the temporary name and must survive the rename onto the final name.
func TestSavedFilesCarryTheMarkOfTheWebOnWindows(t *testing.T) {
	dir := fixtureDir(t)
	dest := openTest(t, newTestSink(), dir)
	save(t, dest, "photo.jpg", "bytes")

	stream, err := os.ReadFile(filepath.Join(dir, fixedSubfolder, "photo.jpg") + ":Zone.Identifier")
	if err != nil {
		t.Fatalf("reading the Zone.Identifier stream = %v, want it present on a saved file", err)
	}
	if string(stream) != "[ZoneTransfer]\r\nZoneId=3\r\n" {
		t.Fatalf("Zone.Identifier = %q, want the Internet-zone marker", stream)
	}
	if dest.Snapshot().MarkingFailed {
		t.Fatal("marking succeeded but the destination reports a marking failure")
	}
}
