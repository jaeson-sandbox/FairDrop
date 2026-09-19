package main

// appicon_test.go pins the seven acceptance criteria in
// _bmad-output/implementation-artifacts/spec-6-1-replace-the-placeholder-app-icon.md
// against both shipped icon assets: build/appicon.png, the 1024x1024 RGBA
// master, and build/windows/icon.ico, which `wails build` generates from it.
//
// Review loop 1 found the original version of this file under-specified:
// each assertion pinned a *sampling method* ("four corner pixels", "central
// 40%") rather than the *property* the spec actually cares about ("no
// opaque backdrop survived", "this is real artwork"), and five mutations
// that should have failed all passed. Every assertion below states the
// property, the region that establishes it, and was confirmed by actually
// running the mutation the spec names -- see the evidence file for the
// mutation table.
//
// icon.ico's own entries are PNG-compressed -- winicon's GenerateIcon calls
// image/png.Encode per size -- so after this file parses the 6-byte
// ICONDIR header and the 16-byte-per-entry directory by hand, it decodes an
// entry's image data with the standard library's image/png, exactly as a
// general-purpose ICO reader would. No new dependency.
//
// Citations into github.com/wailsapp/wails/v2@v2.15.0/pkg/commands/build/packager.go:
//   - generateIcoFile's `if !fs.FileExists(icoFile)` short-circuit -- the
//     whole reason the Windows icon never updated on its own -- is at
//     packager.go:204.
//   - processDarwinIcon is defined at packager.go:134; its (only relevant)
//     call site is packager.go:99.
//   - The size set {256,128,64,48,32,16} is a Wails literal passed to
//     winicon.GenerateIcon at packager.go:217. winicon itself emits
//     whatever list it is handed -- it has no opinion on sizes -- so this
//     pins a Wails literal, not a winicon behaviour, and a maintainer
//     chasing a size-set change after a Wails upgrade should start there,
//     not in leaanthony/winicon.
import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

var (
	appIconPath    = filepath.Join("build", "appicon.png")
	windowsIcoPath = filepath.Join("build", "windows", "icon.ico")
)

const (
	// wantMasterSize is build/appicon.png's required width and height --
	// see the "Master geometry" criterion.
	wantMasterSize = 1024

	// minCentralSaturation sits strictly between the scaffold placeholder's
	// measured 0.01 and the derived master's measured 0.57 (see the spec's
	// Design Notes), so it discriminates a reverted placeholder from real
	// artwork without being tuned to either exact value. Used by the "Real
	// colour" and "Small sizes" criteria.
	minCentralSaturation = 0.25

	// borderRingWidth is the outer-ring width the "No opaque backdrop"
	// criterion measures, in the pixel space of whichever asset is being
	// checked (the master and the .ico's chosen entry are both measured at
	// their own native resolution, not rescaled to a shared width).
	borderRingWidth = 12

	// nearWhiteChannel is the per-channel threshold ("channels > 200")
	// naming a near-white backdrop pixel in the "No opaque backdrop"
	// criterion -- this is what a baked JPEG transparency-checkerboard or
	// its background bloom looks like if it survives the derivation.
	nearWhiteChannel = 200

	// minRingTransparentFraction is how much of the border ring must be
	// fully transparent for the ring to count as "overwhelmingly
	// transparent". The erosion (9px) plus feather (1.2px) geometry leaves
	// a real, expected few-pixel-wide partial-alpha fringe along the
	// plaque's straight edges within any ring this wide, which is why this
	// is not 1.0: the master measures ~0.85 here. Calibrated with headroom
	// below that measurement, and re-checked against the "opaque everywhere
	// except the four corners" mutation, which measures far below it.
	minRingTransparentFraction = 0.75

	// freshnessEntrySize is the shared .ico entry size the "Freshness"
	// criterion downsamples the master to and compares against. 256 is
	// chosen because it is the largest entry the toolchain-provenance size
	// set guarantees is present, which minimises resampling-filter noise
	// relative to a smaller shared size.
	freshnessEntrySize = 256

	// freshnessTolerance255 is the maximum allowed mean per-channel
	// distance (rescaled to an 8-bit 0-255 scale) between the master,
	// box-downsampled to freshnessEntrySize, and the .ico's matching entry.
	// This test's box averaging is not the same filter winicon's
	// Catmull-Rom scale uses, so some distance is expected between
	// same-artwork assets; the tolerance is sized to absorb that but not a
	// different image -- see the freshness test's doc comment for the
	// measured calibration numbers.
	freshnessTolerance255 = 18.0
)

// wantIcoSizes is the literal Wails passes to winicon.GenerateIcon at
// packager.go:217 -- see the "Toolchain provenance" criterion.
var wantIcoSizes = []int{256, 128, 64, 48, 32, 16}

// pngMagic is the 8-byte PNG signature. leaanthony/winicon@v1.0.0 always
// PNG-encodes ICO entries; checking this before attempting to decode turns
// a BMP/DIB-encoded classic ICO entry into a message naming the format
// instead of an opaque image/png decode error.
var pngMagic = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

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

	reserved := binary.LittleEndian.Uint16(raw[0:2])
	if reserved != 0 {
		t.Fatalf("%s's ICONDIR reserved word is %d, want 0: this does not parse as a well-formed ICO file", path, reserved)
	}
	imageType := binary.LittleEndian.Uint16(raw[2:4])
	if imageType != 1 {
		t.Fatalf("%s's ICONDIR image type is %d, want 1 (icon -- 2 would mean cursor): this does not parse as a well-formed ICO file", path, imageType)
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
		if size == 0 {
			t.Fatalf("%s: directory entry %d (%dx%d) declares zero-byte image data", path, i, width, height)
		}
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

// entryOfSize returns the square entry matching size, failing the test by
// name (naming the size and what was actually present) if none matches.
func entryOfSize(t *testing.T, path string, entries []icoEntry, size int) icoEntry {
	t.Helper()
	for _, e := range entries {
		if e.width == size && e.height == size {
			return e
		}
	}
	sizes := make([]string, 0, len(entries))
	for _, e := range entries {
		sizes = append(sizes, fmt.Sprintf("%dx%d", e.width, e.height))
	}
	t.Fatalf("%s carries no %dx%d entry (has %v)", path, size, size, sizes)
	return icoEntry{}
}

// decodeICOEntryImage decodes one ICO directory entry's image data,
// checking the PNG signature first so a non-PNG (classic BMP/DIB) entry
// fails naming that, rather than surfacing an opaque image/png error.
func decodeICOEntryImage(t *testing.T, path string, e icoEntry) image.Image {
	t.Helper()
	if len(e.data) < len(pngMagic) || !bytes.Equal(e.data[:len(pngMagic)], pngMagic) {
		n := len(e.data)
		if n > 8 {
			n = 8
		}
		t.Fatalf("%s: the %dx%d entry is not PNG-compressed (leaanthony/winicon@v1.0.0 always emits "+
			"PNG, never the classic BMP/DIB ICO format) -- first bytes are % x", path, e.width, e.height, e.data[:n])
	}
	img, err := png.Decode(bytes.NewReader(e.data))
	if err != nil {
		t.Fatalf("%s: decode the %dx%d entry as PNG: %v", path, e.width, e.height, err)
	}
	return img
}

// straightRGBA converts a Go premultiplied color.Color to straight
// (non-premultiplied) 8-bit channels -- the values a human editing the PNG
// would see, and the space every 8-bit threshold in this file (channels
// > 200, HSV saturation) is written in. Reuses the standard library's
// conversion rather than re-deriving the unpremultiply arithmetic by hand.
func straightRGBA(c color.Color) (r, g, b, a uint8) {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return n.R, n.G, n.B, n.A
}

// saturation returns HSV saturation (0..1) for an opaque colour:
// (max-min)/max over its 8-bit RGB channels, 0 when max is 0.
func saturation(r, g, b uint8) float64 {
	max := maxByte(r, g, b)
	min := minByte(r, g, b)
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
// 40% (by width and height) of img, skipping fully transparent pixels --
// an invisible icon must not be able to claim "real colour" by leaving
// fully-transparent, high-saturation pixel data behind alpha 0. opaqueCount
// is the number of pixels the mean was computed over.
func meanCentralSaturation(img image.Image) (mean float64, opaqueCount int) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	x0 := bounds.Min.X + int(float64(w)*0.3)
	x1 := bounds.Min.X + int(float64(w)*0.7)
	y0 := bounds.Min.Y + int(float64(h)*0.3)
	y1 := bounds.Min.Y + int(float64(h)*0.7)

	var total float64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, a := straightRGBA(img.At(x, y))
			if a == 0 {
				continue
			}
			total += saturation(r, g, b)
			opaqueCount++
		}
	}
	if opaqueCount == 0 {
		return 0, 0
	}
	return total / float64(opaqueCount), opaqueCount
}

// assertCentralSaturationExceeds pins the "Real colour" criterion.
func assertCentralSaturationExceeds(t *testing.T, label string, img image.Image, min float64) {
	t.Helper()
	mean, n := meanCentralSaturation(img)
	if n == 0 {
		t.Errorf("%s: every pixel in the central 40%% is fully transparent, so no colour could be "+
			"measured -- an invisible icon is not real colour", label)
		return
	}
	if mean <= min {
		t.Errorf("%s: mean central-40%% HSV saturation over %d opaque pixels = %.3f, want > %.2f -- "+
			"this is what the Wails scaffold's white-field, black-\"W\" placeholder (~0.01) measures",
			label, n, mean, min)
	}
}

// assertBorderRingHasNoOpaqueBackdrop pins the "No opaque backdrop"
// criterion over the outer ringWidth-pixel border of img, not just its four
// corner pixels: a centred rounded rect has transparent corners by
// construction, so a backdrop that is opaque everywhere else would still
// pass a corner-only check. Two properties are checked: no ring pixel with
// non-zero alpha has all three straight RGB channels above
// nearWhiteChannel (a baked JPEG checkerboard or background bloom would
// show up here), and the ring as a whole is overwhelmingly (not
// necessarily 100%) transparent, since a genuine derivation still carries a
// feathered transition zone with some low, non-zero alpha in the ring.
func assertBorderRingHasNoOpaqueBackdrop(t *testing.T, label string, img image.Image, ringWidth int) {
	t.Helper()
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if ringWidth*2 >= w || ringWidth*2 >= h {
		t.Fatalf("%s: %dpx border ring does not fit inside a %dx%d image", label, ringWidth, w, h)
	}

	inRing := func(x, y int) bool {
		return x < ringWidth || x >= w-ringWidth || y < ringWidth || y >= h-ringWidth
	}

	var total, transparent, nearWhiteOpaque int
	var firstNearWhite string
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !inRing(x, y) {
				continue
			}
			total++
			r, g, b, a := straightRGBA(img.At(bounds.Min.X+x, bounds.Min.Y+y))
			if a == 0 {
				transparent++
				continue
			}
			if r > nearWhiteChannel && g > nearWhiteChannel && b > nearWhiteChannel {
				nearWhiteOpaque++
				if firstNearWhite == "" {
					firstNearWhite = fmt.Sprintf("(%d,%d) rgba=(%d,%d,%d,%d)", x, y, r, g, b, a)
				}
			}
		}
	}

	if nearWhiteOpaque > 0 {
		t.Errorf("%s: the outer %dpx border ring has %d near-white pixel(s) (R,G,B all > %d), e.g. %s -- "+
			"a baked backdrop (checkerboard or background bloom) survived the derivation",
			label, ringWidth, nearWhiteOpaque, nearWhiteChannel, firstNearWhite)
	}

	transparentFraction := float64(transparent) / float64(total)
	if transparentFraction < minRingTransparentFraction {
		t.Errorf("%s: only %.1f%% of the outer %dpx border ring is fully transparent, want at least "+
			"%.0f%% -- the ring is not overwhelmingly transparent, so an opaque backdrop likely survived",
			label, transparentFraction*100, ringWidth, minRingTransparentFraction*100)
	}
}

// pixelGrid is a decoded image's premultiplied 16-bit-per-channel pixels,
// in row-major order -- Go's color.Color.RGBA() always returns
// alpha-premultiplied channels, which is also the space winicon's
// Catmull-Rom scale operates in (via image.RGBA), so averaging or diffing
// in this space does not fringe a partially transparent edge the way
// averaging straight RGB and alpha independently would.
type pixelGrid struct {
	w, h int
	px   [][4]uint32
}

func gridOf(img image.Image) pixelGrid {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	px := make([][4]uint32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			px[y*w+x] = [4]uint32{r, g, b, a}
		}
	}
	return pixelGrid{w: w, h: h, px: px}
}

func (g pixelGrid) at(x, y int) [4]uint32 { return g.px[y*g.w+x] }

// downsampleBoxAverage resizes g to exactly targetW x targetH by averaging
// each output pixel's source box in premultiplied space. It does not need
// to match winicon's Catmull-Rom filter exactly -- the freshness check's
// tolerance is sized to absorb the difference between box averaging and
// Catmull-Rom, not to require bit-identical resampling.
func downsampleBoxAverage(g pixelGrid, targetW, targetH int) pixelGrid {
	out := make([][4]uint32, targetW*targetH)
	for oy := 0; oy < targetH; oy++ {
		sy0 := oy * g.h / targetH
		sy1 := (oy + 1) * g.h / targetH
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		for ox := 0; ox < targetW; ox++ {
			sx0 := ox * g.w / targetW
			sx1 := (ox + 1) * g.w / targetW
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			var sum [4]uint64
			var n uint64
			for y := sy0; y < sy1; y++ {
				for x := sx0; x < sx1; x++ {
					p := g.at(x, y)
					sum[0] += uint64(p[0])
					sum[1] += uint64(p[1])
					sum[2] += uint64(p[2])
					sum[3] += uint64(p[3])
					n++
				}
			}
			out[oy*targetW+ox] = [4]uint32{
				uint32(sum[0] / n), uint32(sum[1] / n), uint32(sum[2] / n), uint32(sum[3] / n),
			}
		}
	}
	return pixelGrid{w: targetW, h: targetH, px: out}
}

// meanChannelDistance255 returns the mean absolute per-channel difference
// between two equal-sized premultiplied pixel grids, rescaled from Go's
// native 16-bit channel space to an 8-bit 0-255 scale.
func meanChannelDistance255(a, b pixelGrid) float64 {
	if a.w != b.w || a.h != b.h || len(a.px) == 0 {
		return math.MaxFloat64
	}
	var total uint64
	for i := range a.px {
		for c := 0; c < 4; c++ {
			d := int64(a.px[i][c]) - int64(b.px[i][c])
			if d < 0 {
				d = -d
			}
			total += uint64(d)
		}
	}
	return float64(total) / float64(len(a.px)*4) / 257.0
}

// minTransparentBandRows is the minimum number of contiguous fully
// transparent rows required from the master's top and bottom edge inward --
// see contiguousTransparentRows and the "Master geometry" criterion.
//
// Checking only the single outermost row (y=0) is not enough: the plaque's
// own erosion (9px) leaves a few-pixel transparent margin at ITS edges
// regardless of how it is placed on the canvas, so a stretch-to-fill
// mutation that removes the deliberate 15/16px centring band still leaves
// that single outermost row transparent, passing a single-row check
// vacuously (confirmed by running the mutation: it passed on the first
// version of this test). Measuring a run of rows instead tells the two
// apart: this repo's own master has 21-22 contiguous transparent rows top
// and bottom; the same plaque stretched to fill the full 1024x1024 canvas
// (Image.resize instead of centring on a transparent canvas) has only 6.
// 12 sits with real margin on both sides of that gap.
const minTransparentBandRows = 12

// contiguousTransparentRows counts consecutive fully-transparent rows
// starting from img's top (fromTop) or bottom edge and moving inward,
// stopping at the first row containing any non-transparent pixel.
func contiguousTransparentRows(img image.Image, fromTop bool) int {
	bounds := img.Bounds()
	h := bounds.Dy()
	count := 0
	for i := 0; i < h; i++ {
		y := bounds.Min.Y + i
		if !fromTop {
			y = bounds.Max.Y - 1 - i
		}
		allTransparent := true
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				allTransparent = false
				break
			}
		}
		if !allTransparent {
			break
		}
		count++
	}
	return count
}

// assertTransparentBand pins part of the "Master geometry" criterion: the
// master must carry a real transparent band top and bottom from centring
// the non-square plaque in the square canvas, not artwork stretched to
// fill it.
func assertTransparentBand(t *testing.T, label string, img image.Image, which string, fromTop bool, min int) {
	t.Helper()
	got := contiguousTransparentRows(img, fromTop)
	if got < min {
		t.Errorf("%s: only %d contiguous fully-transparent row(s) from the %s edge, want >= %d -- "+
			"the master must carry a real transparent band top and bottom from centring the non-square "+
			"plaque in the square canvas, not the plaque stretched to fill it (which leaves only the "+
			"erosion's own few-pixel margin)", label, got, which, min)
	}
}

// TestAppIconMasterGeometry pins the "Master geometry" criterion.
// Mutation: stretch the plaque square to fill -> must fail.
func TestAppIconMasterGeometry(t *testing.T) {
	data, err := os.ReadFile(appIconPath)
	if err != nil {
		t.Fatalf("read %s: %v", appIconPath, err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode %s's PNG header: %v", appIconPath, err)
	}
	if cfg.Width != wantMasterSize || cfg.Height != wantMasterSize {
		t.Errorf("%s is %dx%d, want exactly %dx%d", appIconPath, cfg.Width, cfg.Height, wantMasterSize, wantMasterSize)
	}
	switch cfg.ColorModel {
	case color.NRGBAModel, color.RGBAModel, color.NRGBA64Model, color.RGBA64Model:
	default:
		t.Errorf("%s decodes with colour model %T, want an RGBA model with an alpha channel", appIconPath, cfg.ColorModel)
	}

	img := decodePNGFile(t, appIconPath)
	assertTransparentBand(t, appIconPath, img, "top", true, minTransparentBandRows)
	assertTransparentBand(t, appIconPath, img, "bottom", false, minTransparentBandRows)
}

// ringWidthFor scales borderRingWidth to an asset's own resolution,
// relative to the wantMasterSize (1024px) master the erosion/feather
// geometry was actually designed at. winicon's 256x256 .ico entry is a
// literal 4x downsample of the master, so the same rounded-rect transition
// zone occupies a proportionally narrower strip there: applying the
// master's absolute 12px ring unscaled to a quarter-resolution asset reaches
// deep into legitimately opaque artwork (confirmed by inspecting the .ico's
// own 256x256 entry directly: by x=3-4 the pixels are already fully opaque,
// genuine copper/plaque colour, not backdrop) rather than the transition
// zone -- exactly the false failure this scaling avoids.
func ringWidthFor(assetSize int) int {
	w := borderRingWidth * assetSize / wantMasterSize
	if w < 1 {
		w = 1
	}
	return w
}

// TestAppIconNoOpaqueBackdrop pins the "No opaque backdrop" criterion
// against both shipped assets. Mutations: re-derive with ERODE_PX = 0 ->
// must fail; make the backdrop opaque everywhere except the four corners
// -> must fail.
func TestAppIconNoOpaqueBackdrop(t *testing.T) {
	master := decodePNGFile(t, appIconPath)
	assertBorderRingHasNoOpaqueBackdrop(t, appIconPath, master, borderRingWidth)

	entries := readICOEntries(t, windowsIcoPath)
	entry := entryOfSize(t, windowsIcoPath, entries, freshnessEntrySize)
	icoImg := decodeICOEntryImage(t, windowsIcoPath, entry)
	label := fmt.Sprintf("%s (%dx%d entry)", windowsIcoPath, entry.width, entry.height)
	assertBorderRingHasNoOpaqueBackdrop(t, label, icoImg, ringWidthFor(entry.width))
}

// TestAppIconCarriesRealColour pins the "Real colour" criterion against
// both shipped assets. Mutation: set the master's alpha to 0 everywhere,
// keeping colour -> must fail.
func TestAppIconCarriesRealColour(t *testing.T) {
	master := decodePNGFile(t, appIconPath)
	assertCentralSaturationExceeds(t, appIconPath, master, minCentralSaturation)

	entries := readICOEntries(t, windowsIcoPath)
	entry := entryOfSize(t, windowsIcoPath, entries, freshnessEntrySize)
	icoImg := decodeICOEntryImage(t, windowsIcoPath, entry)
	label := fmt.Sprintf("%s (%dx%d entry)", windowsIcoPath, entry.width, entry.height)
	assertCentralSaturationExceeds(t, label, icoImg, minCentralSaturation)
}

// TestAppIconSmallIcoEntriesCarryRealColour pins the "Small sizes"
// criterion: the .ico's 16x16 and 32x32 entries -- the sizes the artwork
// was actually chosen for, per the spec's edge-energy measurement -- must
// each independently satisfy the colour assertion. Mutation: replace a
// small entry with opaque white -> must fail.
func TestAppIconSmallIcoEntriesCarryRealColour(t *testing.T) {
	entries := readICOEntries(t, windowsIcoPath)
	for _, size := range []int{16, 32} {
		entry := entryOfSize(t, windowsIcoPath, entries, size)
		img := decodeICOEntryImage(t, windowsIcoPath, entry)
		label := fmt.Sprintf("%s (%dx%d entry)", windowsIcoPath, entry.width, entry.height)
		assertCentralSaturationExceeds(t, label, img, minCentralSaturation)
	}
}

// TestAppIconIcoCarriesTheFullWailsSizeSet pins the "Toolchain provenance"
// criterion: it does not, by itself, prove the .ico was generated from
// *this* master -- see TestAppIconMasterMatchesIcoFreshness for that -- it
// proves only that some winicon-produced .ico, carrying the full size set
// Wails asks for, is present rather than the stale scaffold default (which
// carries 256/128/64/24/32/16 -- 24, never 48).
func TestAppIconIcoCarriesTheFullWailsSizeSet(t *testing.T) {
	entries := readICOEntries(t, windowsIcoPath)
	have := map[int]bool{}
	sizes := make([]string, 0, len(entries))
	for _, e := range entries {
		sizes = append(sizes, fmt.Sprintf("%dx%d", e.width, e.height))
		if e.width == e.height {
			have[e.width] = true
		}
	}
	var missing []int
	for _, size := range wantIcoSizes {
		if !have[size] {
			missing = append(missing, size)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%s is missing size(s) %v from the set {%v} Wails passes to winicon.GenerateIcon at "+
			"packager.go:217 (has %v)", windowsIcoPath, missing, wantIcoSizes, sizes)
	}
}

// TestAppIconMasterMatchesIcoFreshness pins the "Freshness" criterion.
// Mutation: replace the master with hue-swapped artwork, leave the .ico ->
// must fail.
func TestAppIconMasterMatchesIcoFreshness(t *testing.T) {
	master := decodePNGFile(t, appIconPath)
	entries := readICOEntries(t, windowsIcoPath)
	entry := entryOfSize(t, windowsIcoPath, entries, freshnessEntrySize)
	icoImg := decodeICOEntryImage(t, windowsIcoPath, entry)

	downsampled := downsampleBoxAverage(gridOf(master), entry.width, entry.height)
	icoGrid := gridOf(icoImg)

	dist := meanChannelDistance255(downsampled, icoGrid)
	if dist > freshnessTolerance255 {
		t.Errorf("%s downsampled to %dx%d differs from %s's %dx%d entry by a mean %.2f per channel "+
			"(0-255 scale), want <= %.2f -- more than box-averaging-vs-Catmull-Rom resampling noise "+
			"explains, so the .ico does not look like it was generated from this master",
			appIconPath, entry.width, entry.height, windowsIcoPath, entry.width, entry.height, dist, freshnessTolerance255)
	}
}

// partialPixel is one sampled pixel inside a transition run: partial alpha,
// straight RGB.
type partialPixel struct{ r, g, b, a uint8 }

const (
	// maxTransitionRunLength bounds how long a run of consecutive
	// partial-alpha pixels (scanning one row) may be before this test
	// discards it rather than using it. Measured on this repo's own master:
	// a straight-edge transition (erosion + feather) runs 6-10px (median 6,
	// p90 10); rows crossing near the plaque's rounded corners run 23-48px,
	// because a shallow-angle crossing through a curve spans far more
	// pixels than the same transition would along a straight edge. 20 sits
	// in the gap between those two populations, keeping straight-edge runs
	// and excluding corner rows, whose "first opaque pixel after the run"
	// is not a reliable same-feature reference the way a straight edge's is.
	maxTransitionRunLength = 20

	// lowAlphaThreshold255 splits transition-run pixels into "low" and
	// "high" alpha buckets on the straight 8-bit scale.
	lowAlphaThreshold255 = 128

	// maxLowAlphaMeanDeviation is the maximum allowed mean per-channel
	// deviation (0-255 scale) of low-alpha pixels from their run's local
	// opaque reference pixel, averaged over every sampled low-alpha pixel
	// in the image. Calibrated against this repo's own master: the
	// correctly-derived master measures ~3.2 here (against ~2.5 for the
	// high-alpha bucket -- both near photographic/JPEG noise level, no
	// alpha-correlated trend); regenerating with the review-loop-1 defect
	// reintroduced (pasting the plaque onto the canvas using itself as the
	// mask) measures ~28.0 -- roughly 9x higher, and visibly asymmetric
	// against its own ~3.9 high-alpha bucket, which is exactly what
	// systematically darkening RGB in proportion to how low alpha is
	// produces. See the evidence file for both measurements.
	maxLowAlphaMeanDeviation = 8.0
)

// TestAppIconMasterAppliesAlphaOnce pins the "Alpha applied once" criterion.
// Mutation: composite the plaque onto the canvas using itself as the mask
// (canvas.paste(plaque, offset, plaque) instead of a maskless paste) ->
// must fail. This is a real defect review loop 1 found in the shipped
// master, not a hypothetical: it darkens the feather and halves its width,
// because PIL then alpha-blends using the plaque's own alpha a second time
// on top of its already-correct per-pixel alpha.
//
// A single partial-alpha pixel cannot be checked against one fixed
// tolerance: this artwork is a photographed/rendered relief with several
// RGB units of JPEG/photographic noise per pixel, which a first version of
// this test (comparing one partial pixel against "the first fully opaque
// pixel found scanning further along the row") mistook for the defect on
// corner rows, where that "first opaque pixel" can belong to a different
// nearby feature (a decorative frame line) rather than the same local
// transition.
//
// Instead this pools every short (<= maxTransitionRunLength), cleanly
// bounded transition run in the whole master -- one row can cross the
// plaque's outer alpha boundary at most once, so each row contributes at
// most one run -- and compares every pixel in a run against that run's own
// immediately-following opaque pixel, which is guaranteed to be the same
// local feature. Pooling thousands of samples this way averages out the
// photographic noise (a zero-mean, alpha-independent nuisance) while
// staying highly sensitive to the defect's actual signature: a mean
// deviation for low-alpha pixels that is dramatically larger than for
// high-alpha pixels, because darkening scales with (1 - alpha/255).
func TestAppIconMasterAppliesAlphaOnce(t *testing.T) {
	img := decodePNGFile(t, appIconPath)
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	var lowSamples, highSamples []float64
	runsUsed := 0

	for row := 0; row < h; row++ {
		y := bounds.Min.Y + row
		var run []partialPixel
		for col := 0; col < w; col++ {
			x := bounds.Min.X + col
			r, g, b, a := straightRGBA(img.At(x, y))
			switch {
			case a == 0:
				run = nil
			case a == 255:
				if len(run) > 0 {
					for _, p := range run {
						dev := (absDiffF(p.r, r) + absDiffF(p.g, g) + absDiffF(p.b, b)) / 3
						if p.a < lowAlphaThreshold255 {
							lowSamples = append(lowSamples, dev)
						} else {
							highSamples = append(highSamples, dev)
						}
					}
					runsUsed++
				}
				run = nil
			default:
				run = append(run, partialPixel{r, g, b, a})
				if len(run) > maxTransitionRunLength {
					// Likely crossing a rounded corner at a shallow angle
					// rather than the plaque's straight edge: bail on this
					// run's earlier pixels rather than eventually comparing
					// them against a reference pixel that may belong to a
					// different feature. Scanning continues from here, so a
					// short clean tail before the next alpha==0/255 is still
					// captured.
					run = nil
				}
			}
		}
	}

	if runsUsed < 100 || len(lowSamples) == 0 || len(highSamples) == 0 {
		t.Fatalf("%s: found only %d usable transition run(s) (%d low-alpha, %d high-alpha samples), "+
			"too few to measure reliably -- this assertion would be unreliable or vacuous",
			appIconPath, runsUsed, len(lowSamples), len(highSamples))
	}

	lowMean := meanFloat(lowSamples)
	highMean := meanFloat(highSamples)
	if lowMean > maxLowAlphaMeanDeviation {
		t.Errorf("%s: partial-alpha pixels with alpha < %d deviate from their transition run's local "+
			"opaque reference pixel by a mean of %.2f (0-255 scale, %d samples across %d runs); "+
			"alpha >= %d pixels deviate by only %.2f (%d samples) -- RGB drifting further from the "+
			"true colour as alpha drops is what applying alpha twice (pasting an RGBA image using its "+
			"own alpha as the paste mask) looks like, want <= %.2f",
			appIconPath, lowAlphaThreshold255, lowMean, len(lowSamples), runsUsed,
			lowAlphaThreshold255, highMean, len(highSamples), maxLowAlphaMeanDeviation)
	}
}

func meanFloat(xs []float64) float64 {
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func absDiffF(a, b uint8) float64 {
	if a > b {
		return float64(a - b)
	}
	return float64(b - a)
}
