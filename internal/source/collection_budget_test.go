package source

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"fairdrop/internal/transfer"
)

func TestCollectionPinsShareRetainedDirectoryBudget(t *testing.T) {
	root := fixtureDir(t)
	shallow := filepath.Join(root, "shallow")
	if err := os.Mkdir(shallow, 0o755); err != nil {
		t.Fatal(err)
	}
	inspector := New()
	pin, err := inspector.PrepareDirectoryWithRetained(context.Background(), shallow, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Close()
	deep := root
	found := false
	for depth := 0; depth < maxRetainedDirectoryHandles; depth++ {
		deep = filepath.Join(deep, "d")
		if err := os.Mkdir(deep, 0o755); err != nil {
			t.Fatal(err)
		}
		_, aloneErr := inspector.InspectWithRetained(context.Background(), deep, 0)
		_, sharedErr := inspector.InspectWithRetained(context.Background(), deep, 1)
		if aloneErr == nil && transfer.ErrorCodeOf(sharedErr) == transfer.ErrPathUnsupported {
			found = true
			prepared, err := inspector.PrepareDirectoryWithRetained(context.Background(), deep, 1)
			if err == nil {
				walkErr := prepared.Walk(context.Background(), func(transfer.SourceEntry, io.Reader) error { return nil })
				_ = prepared.Close()
				if transfer.ErrorCodeOf(walkErr) != transfer.ErrPathUnsupported {
					t.Fatalf("deep root walk with another live pin = %v, want path_unsupported", walkErr)
				}
			} else if transfer.ErrorCodeOf(err) != transfer.ErrPathUnsupported {
				t.Fatalf("deep root preparation with another live pin = %v", err)
			}
			break
		}
		if aloneErr != nil {
			t.Fatalf("single root refused before shared budget boundary: %v", aloneErr)
		}
	}
	if !found {
		t.Fatal("did not reach shared budget boundary")
	}
}
