package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestHueICORepackagesWhenMutatedPayloadGrows(t *testing.T) {
	source := filepath.Join("..", "build", "windows", "icon.ico")
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "icon.ico")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := hueICO(path); err != nil {
		t.Fatalf("hueICO rejected the committed icon: %v", err)
	}
	mutated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(mutated) <= len(raw) {
		t.Fatalf("mutated ICO is %d bytes, want growth beyond original %d-byte payload layout", len(mutated), len(raw))
	}
	count := int(binary.LittleEndian.Uint16(mutated[4:6]))
	for i := 0; i < count; i++ {
		entry := 6 + i*16
		size := int(binary.LittleEndian.Uint32(mutated[entry+8 : entry+12]))
		offset := int(binary.LittleEndian.Uint32(mutated[entry+12 : entry+16]))
		if offset > len(mutated) || size > len(mutated)-offset {
			t.Fatalf("entry %d points outside repacked ICO: offset=%d size=%d length=%d", i, offset, size, len(mutated))
		}
	}
}
