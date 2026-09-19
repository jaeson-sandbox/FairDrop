package main

// appicon_test.go pins the last three rows of the icon derivation's I/O &
// Edge-Case matrix (see
// _bmad-output/implementation-artifacts/spec-6-1-replace-the-placeholder-app-icon.md)
// against both shipped icon assets: build/appicon.png, the 1024x1024 RGBA
// master, and build/windows/icon.ico, which `wails build` generates from it.
//
//   - "Windows build, stale .ico": wails build's generateIcoFile
//     (packager.go:202, github.com/wailsapp/wails/v2@v2.15.0) wraps
//     generation in `if !fs.FileExists(icoFile)`, so it silently skips
//     regeneration whenever icon.ico already exists -- the only way past it
//     is deleting the file first. A 48x48 entry is the proof this .ico was
//     actually produced by that toolchain: leaanthony/winicon@v1.0.0 emits
//     256/128/64/48/32/16, and the Wails 2.15.0 scaffold default this repo
//     shipped until this story carries 256/128/64/24/32/16 -- 24, never 48.
//   - "Naive JPEG->PNG conversion": none of the three candidate renders the
//     derivation started from carries an alpha channel -- including the one
//     whose backdrop is drawn as a transparency checkerboard, which is
//     opaque pixels like any other -- so a straight format conversion would
//     ship that checkerboard as a grey backdrop. Each asset's four corner
//     pixels must be fully transparent.
//   - "Revert to scaffold default": the Wails scaffold's white-field,
//     black-"W" placeholder already has transparent corners, so the corner
//     check alone would not catch a revert. Mean HSV saturation over the
//     central 40% does: the placeholder measures ~0.01, the derived master
//     ~0.57.
//
// icon.ico's own entries are PNG-compressed -- winicon's GenerateIcon calls
// image/png.Encode per size -- so after this file parses the 6-byte
// ICONDIR header and the 16-byte-per-entry directory by hand, it decodes an
// entry's image data with the standard library's image/png, exactly as a
// general-purpose ICO reader would. No new dependency.
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

var (
	appIconPath    = filepath.Join("build", "appicon.png")
	windowsIcoPath = filepath.Join("build", "windows", "icon.ico")
)

const (
	// wantIcoEntrySize is the entry size leaanthony/winicon emits that the
	// Wails 2.15.0 scaffold default's icon.ico lacks -- see the file
	// comment above.
	wantIcoEntrySize = 48

	// minCentralSaturation sits strictly between the scaffold placeholder's
	// measured 0.01 and the derived master's measured 0.57 (see the spec's
	// Acceptance Criteria), so it discriminates a reverted placeholder from
	// real artwork without being tuned to either exact value.
	minCentralSaturation = 0.25
)

// decodePNGFile reads path and decodes it as a PNG image, failing the test
// on either error.
func decodePNGFile(t *testing.T, path string) image.Image {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode %s as PNG: %v", path, err)
	}
	return img
}

// icoEntry is one parsed ICONDIRENTRY plus the image bytes it points at.
type icoEntry struct {
	width, height int
	data          []byte
}

// readICOEntries parses path as an ICO file: a 6-byte ICONDIR header
// (2 reserved + imagetype + imagecount) followed by one 16-byte
// ICONDIRENTRY per image (width, height, colours, reserved, planes(2),
// bpp(2), size(4), offset(4)) -- see
// github.com/leaanthony/winicon@v1.0.0/internal/winicon/structs.go, which
// this mirrors by hand rather than importing, since only the directory is
// needed here, not generation.
func readICOEntries(t *testing.T, path string) []icoEntry {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if len(raw) < 6 {
		t.Fatalf("%s is %d bytes, too short for a 6-byte ICONDIR header", path, len(raw))
	}
	count := int(binary.LittleEndian.Uint16(raw[4:6]))
	if count == 0 {
		t.Fatalf("%s's ICONDIR declares zero images, so this test would pass vacuously", path)
	}

	const dirEntrySize = 16
	entries := make([]icoEntry, 0, count)
	for i := 0; i < count; i++ {
		off := 6 + i*dirEntrySize
		if off+dirEntrySize > len(raw) {
			t.Fatalf("%s: directory entry %d runs past the end of the file", path, i)
		}
		entry := raw[off : off+dirEntrySize]

		// A stored 0 means 256, per the ICO format -- a single byte cannot
		// hold 256, so it is reserved to mean the maximum size instead.
		width := int(entry[0])
		if width == 0 {
			width = 256
		}
		height := int(entry[1])
		if height == 0 {
			height = 256
		}

		size := binary.LittleEndian.Uint32(entry[8:12])
		dataOffset := binary.LittleEndian.Uint32(entry[12:16])
		if dataOffset > uint32(len(raw)) || size > uint32(len(raw))-dataOffset {
			t.Fatalf("%s: directory entry %d's image data (offset %d, size %d) runs past the end of the file", path, i, dataOffset, size)
		}

		entries = append(entries, icoEntry{
			width:  width,
			height: height,
			data:   raw[dataOffset : dataOffset+size],
		})
	}
	return entries
}

// largestEntry returns the entry with the greatest pixel area, so the
// pixel-level assertions below run against the most detailed image the ICO
// carries rather than an arbitrarily chosen small one.
func largestEntry(t *testing.T, entries []icoEntry) icoEntry {
	t.Helper()
	if len(entries) == 0 {
		t.Fatal("no ICO entries to choose from")
	}
	best := entries[0]
	for _, e := range entries[1:] {
		if e.width*e.height > best.width*best.height {
			best = e
		}
	}
	return best
}

// assertCornersAreTransparent pins the matrix's "naive JPEG->PNG
// conversion" row.
func assertCornersAreTransparent(t *testing.T, label string, img image.Image) {
	t.Helper()
	bounds := img.Bounds()
	corners := map[string][2]int{
		"top-left":     {bounds.Min.X, bounds.Min.Y},
		"top-right":    {bounds.Max.X - 1, bounds.Min.Y},
		"bottom-left":  {bounds.Min.X, bounds.Max.Y - 1},
		"bottom-right": {bounds.Max.X - 1, bounds.Max.Y - 1},
	}
	for name, xy := range corners {
		_, _, _, a := img.At(xy[0], xy[1]).RGBA()
		if a != 0 {
			t.Errorf("%s: %s corner (%d,%d) has alpha %d, want 0 -- an opaque backdrop survived the derivation", label, name, xy[0], xy[1], a)
		}
	}
}

// saturation returns HSV saturation (0..1) for an opaque colour:
// (max-min)/max over its 8-bit RGB channels, 0 when max is 0.
func saturation(c color.Color) float64 {
	nrgba := color.NRGBAModel.Convert(c).(color.NRGBA)
	max := maxByte(nrgba.R, nrgba.G, nrgba.B)
	min := minByte(nrgba.R, nrgba.G, nrgba.B)
	if max == 0 {
		return 0
	}
	return float64(max-min) / float64(max)
}

func maxByte(a, b, c uint8) uint8 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

func minByte(a, b, c uint8) uint8 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

// meanCentralSaturation returns the mean HSV saturation over the central
// 40% (by width and height) of img.
func meanCentralSaturation(img image.Image) float64 {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	x0 := bounds.Min.X + int(float64(w)*0.3)
	x1 := bounds.Min.X + int(float64(w)*0.7)
	y0 := bounds.Min.Y + int(float64(h)*0.3)
	y1 := bounds.Min.Y + int(float64(h)*0.7)

	var total float64
	var n int
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			total += saturation(img.At(x, y))
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return total / float64(n)
}

// assertCentralSaturationExceeds pins the matrix's "revert to scaffold
// default" row: the placeholder's corners are already transparent, so only
// a colour-based check catches a revert.
func assertCentralSaturationExceeds(t *testing.T, label string, img image.Image, min float64) {
	t.Helper()
	got := meanCentralSaturation(img)
	if got <= min {
		t.Errorf("%s: mean central-40%% HSV saturation = %.3f, want > %.2f -- "+
			"this is what the Wails scaffold's white-field, black-\"W\" placeholder (~0.01) measures",
			label, got, min)
	}
}

// TestAppIconWindowsIcoWasGeneratedFromTheMaster pins the matrix's "Windows
// build, stale .ico" row.
func TestAppIconWindowsIcoWasGeneratedFromTheMaster(t *testing.T) {
	entries := readICOEntries(t, windowsIcoPath)

	found := false
	sizes := make([]string, 0, len(entries))
	for _, e := range entries {
		sizes = append(sizes, fmt.Sprintf("%dx%d", e.width, e.height))
		if e.width == wantIcoEntrySize && e.height == wantIcoEntrySize {
			found = true
		}
	}
	if !found {
		t.Errorf("%s carries no %dx%d entry (has %v): this is what the stale Wails scaffold "+
			"default looks like -- delete it and run `wails build` to regenerate it from %s",
			windowsIcoPath, wantIcoEntrySize, wantIcoEntrySize, sizes, appIconPath)
	}
}

// TestAppIconShippedAssetsHaveNoOpaqueBackdrop pins the matrix's "naive JPEG->PNG
// conversion" row against both shipped assets.
func TestAppIconShippedAssetsHaveNoOpaqueBackdrop(t *testing.T) {
	appIcon := decodePNGFile(t, appIconPath)
	assertCornersAreTransparent(t, appIconPath, appIcon)

	entries := readICOEntries(t, windowsIcoPath)
	largest := largestEntry(t, entries)
	label := fmt.Sprintf("%s (%dx%d entry)", windowsIcoPath, largest.width, largest.height)
	icoImg, err := png.Decode(bytes.NewReader(largest.data))
	if err != nil {
		t.Fatalf("%s: decode as PNG: %v", label, err)
	}
	assertCornersAreTransparent(t, label, icoImg)
}

// TestAppIconShippedAssetsCarryRealColour pins the matrix's "revert to scaffold
// default" row against both shipped assets.
func TestAppIconShippedAssetsCarryRealColour(t *testing.T) {
	appIcon := decodePNGFile(t, appIconPath)
	assertCentralSaturationExceeds(t, appIconPath, appIcon, minCentralSaturation)

	entries := readICOEntries(t, windowsIcoPath)
	largest := largestEntry(t, entries)
	label := fmt.Sprintf("%s (%dx%d entry)", windowsIcoPath, largest.width, largest.height)
	icoImg, err := png.Decode(bytes.NewReader(largest.data))
	if err != nil {
		t.Fatalf("%s: decode as PNG: %v", label, err)
	}
	assertCentralSaturationExceeds(t, label, icoImg, minCentralSaturation)
}
