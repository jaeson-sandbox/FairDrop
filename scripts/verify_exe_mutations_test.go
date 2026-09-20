package main

import (
	"bytes"
	"encoding/binary"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestHueICOMutatesTheCommittedIconAndKeepsEveryEntryValid(t *testing.T) {
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
	count := int(binary.LittleEndian.Uint16(mutated[4:6]))
	changed64 := false
	for i := 0; i < count; i++ {
		entry := 6 + i*16
		size := int(binary.LittleEndian.Uint32(mutated[entry+8 : entry+12]))
		offset := int(binary.LittleEndian.Uint32(mutated[entry+12 : entry+16]))
		if offset > len(mutated) || size > len(mutated)-offset {
			t.Fatalf("entry %d points outside repacked ICO: offset=%d size=%d length=%d", i, offset, size, len(mutated))
		}
		width := int(mutated[entry])
		if width != 64 {
			continue
		}
		beforeSize := int(binary.LittleEndian.Uint32(raw[entry+8 : entry+12]))
		beforeOffset := int(binary.LittleEndian.Uint32(raw[entry+12 : entry+16]))
		before, err := png.Decode(bytes.NewReader(raw[beforeOffset : beforeOffset+beforeSize]))
		if err != nil {
			t.Fatal(err)
		}
		after, err := png.Decode(bytes.NewReader(mutated[offset : offset+size]))
		if err != nil {
			t.Fatal(err)
		}
		for y := before.Bounds().Min.Y; y < before.Bounds().Max.Y && !changed64; y++ {
			for x := before.Bounds().Min.X; x < before.Bounds().Max.X; x++ {
				old := color.NRGBAModel.Convert(before.At(x, y)).(color.NRGBA)
				if old.A == 0 || (old.R == old.G && old.G == old.B) {
					continue
				}
				got := color.NRGBAModel.Convert(after.At(x, y)).(color.NRGBA)
				want := color.NRGBA{R: old.B, G: old.R, B: old.G, A: old.A}
				if got != want {
					t.Fatalf("64px pixel (%d,%d) = %#v, want hue rotation %#v", x, y, got, want)
				}
				changed64 = true
				break
			}
		}
	}
	if !changed64 {
		t.Fatal("64px entry had no non-grey opaque pixel whose hue mutation could be verified")
	}
}

func TestRepackICOAllowsPayloadGrowthAndRewritesFollowingOffsets(t *testing.T) {
	header := []byte{0, 0, 1, 0, 2, 0}
	firstHeader := make([]byte, 16)
	secondHeader := make([]byte, 16)
	firstHeader[0], secondHeader[0] = 16, 32
	entries := []icoEntry{
		{header: firstHeader, payload: bytes.Repeat([]byte{0xaa}, 37)},
		{header: secondHeader, payload: bytes.Repeat([]byte{0xbb}, 11)},
	}
	repacked := repackICO(header, entries)
	firstOffset := int(binary.LittleEndian.Uint32(repacked[18:22]))
	secondOffset := int(binary.LittleEndian.Uint32(repacked[34:38]))
	wantFirst := 6 + 2*16
	if firstOffset != wantFirst || secondOffset != wantFirst+37 {
		t.Fatalf("repacked offsets = %d,%d, want %d,%d", firstOffset, secondOffset, wantFirst, wantFirst+37)
	}
	if got := repacked[firstOffset:secondOffset]; !bytes.Equal(got, entries[0].payload) {
		t.Fatal("grown first payload was not preserved")
	}
}
