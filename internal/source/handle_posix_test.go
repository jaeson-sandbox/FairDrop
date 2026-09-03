//go:build linux || darwin

package source

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"fairdrop/internal/transfer"
)

func TestPOSIXReopenRefusesPostMetadataSymlinkSubstitution(t *testing.T) {
	for _, test := range []struct {
		name string
		open func(metadataHandle) (metadataHandle, error)
	}{
		{name: "search", open: func(handle metadataHandle) (metadataHandle, error) { return handle.OpenSearch() }},
		{name: "enumeration", open: func(handle metadataHandle) (metadataHandle, error) {
			return handle.OpenEnumeration()
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			selected := filepath.Join(base, "selected")
			target := filepath.Join(base, "target")
			if err := os.Mkdir(selected, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(target, 0o700); err != nil {
				t.Fatal(err)
			}

			metadata, ancestors := openPOSIXMetadataForSelection(t, selected)
			defer func() {
				_ = metadata.Close()
				_ = closeMetadataHandles(context.Background(), ancestors, nil)
			}()
			inspected, err := metadata.Stat()
			if err != nil || !inspected.IsDir() {
				t.Fatalf("selected metadata = %+v, %v", inspected, err)
			}

			original := filepath.Join(base, "original")
			if err := os.Rename(selected, original); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, selected); err != nil {
				if errorsIsPOSIXCapability(err) {
					t.Skipf("native symlink capability unavailable: %v", err)
				}
				t.Fatal(err)
			}
			opened, err := test.open(metadata)
			if opened != nil {
				_ = opened.Close()
			}
			if err == nil {
				t.Fatalf("Open%s followed a symlink substituted after metadata inspection", test.name)
			}
		})
	}
}

func TestPOSIXInspectUsesSearchOnlyAncestorRights(t *testing.T) {
	ancestor := filepath.Join(t.TempDir(), "search-only")
	if err := os.Mkdir(ancestor, 0o700); err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(ancestor, "ordinary.txt")
	if err := os.WriteFile(selected, []byte("ordinary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ancestor, 0o111); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ancestor, 0o700) })
	if _, err := os.ReadDir(ancestor); err == nil {
		t.Skip("runner privileges bypass the search-only directory restriction")
	} else if !errorsIsPermission(err) {
		t.Fatalf("search-only fixture ReadDir error = %v, want permission refusal", err)
	}

	item, err := New().Inspect(context.Background(), selected)
	if err != nil {
		t.Fatalf("Inspect(file through search-only ancestor) error = %v", err)
	}
	if item.Kind != transfer.ItemFile || item.Name != "ordinary.txt" || item.LogicalSize != 8 {
		t.Fatalf("item = %+v, want ordinary file metadata", item)
	}
}

func openPOSIXMetadataForSelection(t *testing.T, path string) (metadataHandle, []metadataHandle) {
	t.Helper()
	factory := nativeHandleFactory{}
	plan, err := factory.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	anchor, err := factory.OpenAnchor(plan)
	if err != nil {
		t.Fatal(err)
	}
	ancestors := []metadataHandle{anchor}
	current := anchor
	for index, component := range plan.components {
		metadata, err := current.OpenChildMetadata(component)
		if err != nil {
			_ = closeMetadataHandles(context.Background(), ancestors, nil)
			t.Fatal(err)
		}
		if index == len(plan.components)-1 {
			return metadata, ancestors
		}
		search, err := metadata.OpenSearch()
		_ = metadata.Close()
		if err != nil {
			_ = closeMetadataHandles(context.Background(), ancestors, nil)
			t.Fatal(err)
		}
		ancestors = append(ancestors, search)
		current = search
	}
	_ = closeMetadataHandles(context.Background(), ancestors, nil)
	t.Fatal("selection did not contain a final component")
	return nil, nil
}

func errorsIsPOSIXCapability(err error) bool {
	return errorsIsPermission(err) || err == fs.ErrInvalid
}

func errorsIsPermission(err error) bool {
	return os.IsPermission(err)
}
