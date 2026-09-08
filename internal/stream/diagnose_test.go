package stream

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"fairdrop/internal/source"
	"fairdrop/internal/transfer"
)

/*
Support tool, not a regression test. It skips unless FAIRDROP_DIAGNOSE names
a folder, and then runs the exact production path a live transfer runs --
Inspect, Walk, Prepare, WriteTo -- against that folder and reports the first
stage that refuses, with the entry it was on.

FairDrop logs nothing about a transfer, so when a live download fails the
only questions worth asking first are "does this folder stage" and "does it
archive", and this answers both in seconds:

	FAIRDROP_DIAGNOSE='C:\path	oolder' go test -count=1 -run TestDiagnoseRealFolder -v ./internal/stream/

On this machine it cleared a 2.5 GB, 3,274-file tree and OneDrive-backed
folders, which is what ruled the archive out of the first live failure.
*/
func TestDiagnoseRealFolder(t *testing.T) {
	root := os.Getenv("FAIRDROP_DIAGNOSE")
	if root == "" {
		t.Skip("set FAIRDROP_DIAGNOSE to a folder path")
	}
	t.Logf("folder: %s", root)

	staged, err := source.New().Inspect(context.Background(), root)
	if err != nil {
		t.Fatalf("INSPECT REFUSED: %v  (code=%s)", err, transfer.ErrorCodeOf(err))
	}
	t.Logf("INSPECT ok: name=%q kind=%s logicalSize=%d", staged.Name, staged.Kind, staged.LogicalSize)

	files, dirs := 0, 0
	last := ""
	walkErr := source.New().Walk(context.Background(), root, func(e transfer.SourceEntry, _ io.Reader) error {
		last = e.RelativePath
		if e.Kind == transfer.ItemDirectory {
			dirs++
		} else {
			files++
		}
		return nil
	})
	if walkErr != nil {
		t.Errorf("WALK REFUSED after %d files / %d dirs (last entry reached: %q): %v  (code=%s)",
			files, dirs, last, walkErr, transfer.ErrorCodeOf(walkErr))
	} else {
		t.Logf("WALK ok: %d files, %d directories", files, dirs)
	}

	prepared, err := New(source.New()).Prepare(context.Background(), staged)
	if err != nil {
		t.Fatalf("PREPARE REFUSED: %v  (code=%s)", err, transfer.ErrorCodeOf(err))
	}
	defer func() { _ = prepared.Close() }()
	t.Logf("PREPARE ok: downloadName=%q", prepared.DownloadName())

	var body bytes.Buffer
	if err := prepared.WriteTo(context.Background(), &body); err != nil {
		t.Fatalf("WRITETO FAILED after %d bytes: %v  (code=%s)",
			body.Len(), err, transfer.ErrorCodeOf(err))
	}
	t.Logf("WRITETO ok: %d bytes on the wire", body.Len())

	reader, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len()))
	if err != nil {
		t.Fatalf("ARCHIVE UNREADABLE: %v", err)
	}
	t.Logf("ARCHIVE ok: %d entries", len(reader.File))
}
