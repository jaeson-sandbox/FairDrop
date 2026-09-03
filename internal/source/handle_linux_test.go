//go:build linux

package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fairdrop/internal/transfer"
	"golang.org/x/sys/unix"
)

func TestLinuxMetadataHandleUsesOPathWithoutReadAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata-only.bin")
	if err := os.WriteFile(path, []byte("metadata"), 0o000); err != nil {
		t.Fatal(err)
	}
	metadata, ancestors := openPOSIXMetadataForSelection(t, path)
	defer func() {
		_ = metadata.Close()
		_ = closeMetadataHandles(context.Background(), ancestors, nil)
	}()
	info, err := metadata.Stat()
	if err != nil {
		t.Fatalf("O_PATH Stat() error = %v", err)
	}
	if info.Size() != 8 {
		t.Fatalf("metadata size = %d, want 8", info.Size())
	}
	node, ok := metadata.(*posixNode)
	if !ok {
		t.Fatalf("metadata handle type = %T, want *posixNode", metadata)
	}
	if _, err := node.file.Read(make([]byte, 1)); !errors.Is(err, unix.EBADF) {
		t.Fatalf("O_PATH Read() error = %v, want EBADF proving no read access", err)
	}
}

func TestLinuxInspectRejectsFIFOWithoutOpeningForRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "named-pipe")
	if err := unix.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	type result struct {
		item transfer.StagedItem
		err  error
	}
	resultCh := make(chan result, 1)
	go func() {
		item, err := New().Inspect(context.Background(), path)
		resultCh <- result{item: item, err: err}
	}()
	select {
	case got := <-resultCh:
		if got.item != (transfer.StagedItem{}) {
			t.Fatalf("item = %+v, want zero metadata", got.item)
		}
		if code := transfer.ErrorCodeOf(got.err); code != transfer.ErrPathUnsupported {
			t.Fatalf("error = %v, code = %q, want %q", got.err, code, transfer.ErrPathUnsupported)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Inspect blocked opening a FIFO; metadata must use O_PATH")
	}
}
