package main

// appicon_test.go pins the seven asset acceptance criteria in
// _bmad-output/implementation-artifacts/spec-6-1-replace-the-placeholder-app-icon.md
// against both shipped icon assets: build/appicon.png, the 1024x1024 RGBA
// master, and build/windows/icon.ico, which `wails build` generates from it.
//
// Three review loops found successive versions of this file
// under-specified, each time the same way: an assertion pinned a *sampling
// method* rather than the *property* the spec cares about, and a constant was
// chosen for headroom rather than derived from a measurement.
//
//   - Loop 1: "four corner pixels" for "no opaque backdrop"; five mutations
//     that should have failed all passed.
//   - Loop 2: a transparent-row floor with no ceiling passed on a blank
//     canvas; the freshness tolerance was 29x the real signal.
//   - Loop 3: only the 256 entry was ever compared against the master, so the
//     other five could be flat coloured squares with the suite green; and
//     loop 2's replacement tolerance still let a 2.4% tonal revision ship a
//     stale .ico while reporting the gap closed.
//
// Every tolerance here is now a factor applied to a measured value, and every
// measured value is logged on each run so drift is visible rather than
// discovered. The mutation table is DERIVED by scripts/verify-asset-mutations.py,
// not hand-written -- loop 2's hand-written table carried three stale numbers
// into a merged evidence file.
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
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

var (
	appIconPath    = filepath.Join("build", "appicon.png")
	windowsIcoPath = filepath.Join("build", "windows", "icon.ico")
	appIconSource  = filepath.Join("build", "appicon-source.jpg")
)

const (
	// wantMasterSize is build/appicon.png's required width and height --
	// see the "Master geometry" criterion.
	wantMasterSize = 1024

	// minCentralSaturation sits strictly between the scaffold placeholder's
	// measured 0.010 and the derived master's measured 0.592 (both recomputed
	// exactly as meanCentralSaturation does -- central 40%, skipping alpha 0,
	// over 167,281 pixels -- after the review-loop-1 alpha fix changed the
	// master's pixel data; an earlier comment here said 0.57, measured before
	// it). It discriminates a reverted placeholder from real artwork without
	// being tuned to either exact value. Used by the "Real colour" and "Small
	// sizes" criteria.
	minCentralSaturation = 0.25

	// minCentralOpaqueFraction is how much of the central 40% must actually
	// be opaque for a saturation mean over it to mean anything. Without this,
	// a single opaque pixel anywhere in the central region satisfies the
	// "Real colour" criterion: the mean is taken over opaque pixels only, so
	// one saturated pixel in an otherwise empty field scores perfectly. The
	// master and every .ico entry are solid plaque across their whole centre,
	// so they measure 1.0 here.
	minCentralOpaqueFraction = 0.5

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

	// largestEntrySize is the .ico entry the border-ring and central-colour
	// assertions measure. 256 is the largest the toolchain-provenance size set
	// guarantees, and the only one whose ring is wide enough relative to the
	// corner radius for the transparent-fraction check to mean anything.
	//
	// It was named freshnessEntrySize until review loop 4: freshness now
	// compares EVERY entry and no longer uses it, so the old name and comment
	// described a caller that had stopped existing -- the same
	// comment-disagrees-with-code defect loop 3 was opened by.
	largestEntrySize = 256

	// freshnessToleranceFactor multiplies each entry's own measured
	// same-artwork distance (measuredEntryDistance255) to get its bound.
	//
	// Deriving the bound from the measurement, rather than picking a number
	// and hoping, is the point. Loop 2 replaced an absolute 18.0 with an
	// absolute 4.96 and reported the stale-icon gap closed; measuring the new
	// boundary afterwards showed a +6/255 revision (2.4% of full scale) still
	// passed. A factor cannot drift away from its measurement that way: if a
	// future toolchain change moves the real distance, the bound moves with
	// it, and a move beyond 50% fails loudly naming the constant to re-measure.
	//
	// 1.5 is what the measurements support. Per entry, genuine distance against
	// the distance when the entry holds a flat coloured square instead of the
	// artwork: 0.616/54.1 at 256, 1.031/53.8 at 128, 1.512/53.3 at 64,
	// 2.140/53.2 at 48, 2.552/52.8 at 32, 4.109/52.4 at 16 -- a 13x to 88x
	// separation, so content differences are never close to this bound.
	//
	// Measured boundary for the tonal case, which is the one loop 2 got wrong:
	// a uniform +1/255 brightening of the master with the .ico left stale is
	// CAUGHT, and +0 (the genuine asset) passes. Loop 2's bound first caught
	// it at +8.
	freshnessToleranceFactor = 1.5
)

// measuredEntryDistance255 is, per .ico entry size, the mean per-channel
// distance (0-255) this test's box averaging measures against winicon's
// Catmull-Rom scale when the entry genuinely was generated from the committed
// master. The distance grows as the entry shrinks because the two filters
// disagree more the further the downsample goes -- which is exactly why a
// single shared tolerance for all sizes was the wrong shape.
//
// Re-measure with scripts/verify-asset-mutations.py if a Wails or winicon
// upgrade changes the resampling filter.
var measuredEntryDistance255 = map[int]float64{
	256: 0.616,
	128: 1.031,
	64:  1.512,
	48:  2.140,
	32:  2.552,
	16:  4.109,
}

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
	var matches []icoEntry
	for _, e := range entries {
		if e.width == size && e.height == size {
			matches = append(matches, e)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	sizes := make([]string, 0, len(entries))
	for _, e := range entries {
		sizes = append(sizes, fmt.Sprintf("%dx%d", e.width, e.height))
	}
	if len(matches) == 0 {
		t.Fatalf("%s carries no %dx%d entry (has %v)", path, size, size, sizes)
	}
	// Two entries at one size means "the" entry at that size is ambiguous,
	// and every assertion below would silently measure whichever came first.
	t.Fatalf("%s declares %d separate %dx%d entries (has %v) -- which one ships is ambiguous",
		path, len(matches), size, size, sizes)
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
	// The directory's declared size is metadata; the embedded PNG carries its
	// own dimensions. If they disagree, every assertion that selects an entry
	// by declared size is measuring a different image than it names.
	if b := img.Bounds(); b.Dx() != e.width || b.Dy() != e.height {
		t.Fatalf("%s: the entry its directory declares as %dx%d decodes as %dx%d",
			path, e.width, e.height, b.Dx(), b.Dy())
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
func meanCentralSaturation(img image.Image) (mean float64, opaqueCount, centralCount int) {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	x0 := bounds.Min.X + int(float64(w)*0.3)
	x1 := bounds.Min.X + int(float64(w)*0.7)
	y0 := bounds.Min.Y + int(float64(h)*0.3)
	y1 := bounds.Min.Y + int(float64(h)*0.7)

	var total float64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			centralCount++
			r, g, b, a := straightRGBA(img.At(x, y))
			if a == 0 {
				continue
			}
			total += saturation(r, g, b)
			opaqueCount++
		}
	}
	if opaqueCount == 0 {
		return 0, 0, centralCount
	}
	return total / float64(opaqueCount), opaqueCount, centralCount
}

// assertCentralSaturationExceeds pins the "Real colour" criterion.
func assertCentralSaturationExceeds(t *testing.T, label string, img image.Image, min float64) {
	t.Helper()
	mean, n, central := meanCentralSaturation(img)
	if n == 0 {
		t.Errorf("%s: every pixel in the central 40%% is fully transparent, so no colour could be "+
			"measured -- an invisible icon is not real colour", label)
		return
	}
	// A mean taken over opaque pixels only is meaningless when almost none
	// are opaque: one saturated pixel in an empty central field would score
	// perfectly. Require the region to be substantially filled first.
	if fraction := float64(n) / float64(central); fraction < minCentralOpaqueFraction {
		t.Errorf("%s: only %d of %d central-40%% pixels (%.1f%%) are non-transparent, want at least "+
			"%.0f%% -- a saturation mean over a nearly empty region proves nothing",
			label, n, central, fraction*100, minCentralOpaqueFraction*100)
		return
	}
	if mean <= min {
		t.Errorf("%s: mean central-40%% HSV saturation over %d opaque pixels = %.3f, want > %.2f -- "+
			"this is what the Wails scaffold's white-field, black-\"W\" placeholder (~0.010) measures",
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
//
// checkTransparentFraction gates the second property, because it is
// geometry-dependent in a way the first is not. The ring counts as mostly
// transparent only where it is wide relative to the plaque's corner radius:
// measured on the genuine assets, the master's 12px ring is 85% transparent
// and the .ico's 256 entry clears the bar, but the smaller entries fall to
// 71.7% (128), 34.0% (48) and 6.7% (16) -- not because a backdrop survived
// but because a 1px ring on a 16px tile is almost entirely plaque edge. The
// near-white property holds at every size and is what actually detects a
// baked checkerboard or bloom; a deliberately opaque *coloured* backdrop on a
// small entry is outside what this assertion can see, and is stated here
// rather than implied away.
func assertBorderRingHasNoOpaqueBackdrop(t *testing.T, label string, img image.Image, ringWidth int, checkTransparentFraction bool) {
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

	if !checkTransparentFraction {
		return
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

// maxTransparentBandRows bounds the same band from above, and exists because
// a floor alone does not pin geometry at all -- it pins "there is some empty
// space at the edges", which artwork-free images satisfy best of all.
// Measured in review loop 2: the Wails scaffold placeholder this story
// replaces yields 115 top / 97 bottom, and a fully blank transparent
// 1024x1024 canvas yields 1024 / 1024. Both passed a floor-only check, so a
// test named "master geometry" passed on an image containing no artwork.
//
// The genuine master measures 21 / 22: the deliberate 15/16px centring band
// from the 1024x993 plaque, plus the plaque's own ~6px erosion margin. 40
// sits well above that and far below the placeholder's 97.
const maxTransparentBandRows = 40

// The master's plaque is 1024 wide -- the full canvas -- so horizontally
// there is no centring band at all, only the ~6px margin the 9px erosion
// leaves at the plaque's own edges. Measured: 6 left, 6 right. Checking rows
// alone pinned nothing about width: review loop 3 confirmed a plaque shrunk
// horizontally by 240px passed TestAppIconMasterGeometry (other tests caught
// it, but not the one named for geometry).
const (
	minTransparentBandCols = 1
	maxTransparentBandCols = 12
)

// contiguousTransparentCols is contiguousTransparentRows' vertical twin,
// counting fully-transparent columns inward from the left or right edge.
func contiguousTransparentCols(img image.Image, fromLeft bool) int {
	bounds := img.Bounds()
	w := bounds.Dx()
	count := 0
	for i := 0; i < w; i++ {
		x := bounds.Min.X + i
		if !fromLeft {
			x = bounds.Max.X - 1 - i
		}
		allTransparent := true
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
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
func assertTransparentBand(t *testing.T, label string, img image.Image, which string, fromTop bool, min, max int) {
	t.Helper()
	got := contiguousTransparentRows(img, fromTop)
	if got < min {
		t.Errorf("%s: only %d contiguous fully-transparent row(s) from the %s edge, want >= %d -- "+
			"the master must carry a real transparent band top and bottom from centring the non-square "+
			"plaque in the square canvas, not the plaque stretched to fill it (which leaves only the "+
			"erosion's own few-pixel margin)", label, got, which, min)
	}
	if got > max {
		t.Errorf("%s: %d contiguous fully-transparent row(s) from the %s edge, want <= %d -- the "+
			"plaque should fill the canvas but for its centring band, so a band this deep means the "+
			"artwork is shrunken, inset, or absent (the Wails scaffold placeholder measures 115/97 "+
			"here and a fully blank canvas 1024/1024, and both passed while this bound was missing)",
			label, got, which, max)
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
	// cfg.ColorModel cannot do this job: Go's image/png maps truecolour
	// *without* alpha to color.RGBAModel (reader.go, cbTC8) and 16-bit
	// truecolour to color.RGBA64Model, so a switch accepting those accepts an
	// alpha-less PNG -- confirmed in review loop 2 by re-saving the master as
	// mode RGB, which produced no colour-model error at all. Read the IHDR
	// colour-type byte instead, which states the fact directly.
	if ct, ok := pngColourType(data); !ok {
		t.Errorf("%s: could not read the PNG IHDR colour-type byte", appIconPath)
	} else if ct != pngTruecolourAlpha && ct != pngGreyscaleAlpha {
		t.Errorf("%s: PNG colour type is %d, want %d (truecolour with alpha) or %d "+
			"(greyscale with alpha) -- the master must carry a real alpha channel, and types "+
			"0/2/3 do not", appIconPath, ct, pngTruecolourAlpha, pngGreyscaleAlpha)
	}

	img := decodePNGFile(t, appIconPath)
	assertTransparentBand(t, appIconPath, img, "top", true, minTransparentBandRows, maxTransparentBandRows)
	assertTransparentBand(t, appIconPath, img, "bottom", false, minTransparentBandRows, maxTransparentBandRows)

	// The bands must also MATCH each other: a range alone lets the plaque sit
	// up to 28 rows off-centre while both ends stay inside 12-40. The genuine
	// master is 21/22, uneven by one only because 1024-993 is odd.
	if top, bottom := contiguousTransparentRows(img, true), contiguousTransparentRows(img, false); abs(top-bottom) > 1 {
		t.Errorf("%s: transparent bands are %d rows top and %d bottom, differing by %d -- the plaque "+
			"is off-centre; centring a %d-row plaque in %d leaves bands within one row of each other",
			appIconPath, top, bottom, abs(top-bottom), wantMasterSize-31, wantMasterSize)
	}

	for _, side := range []struct {
		name     string
		fromLeft bool
	}{{"left", true}, {"right", false}} {
		got := contiguousTransparentCols(img, side.fromLeft)
		if got < minTransparentBandCols || got > maxTransparentBandCols {
			t.Errorf("%s: %d contiguous fully-transparent column(s) from the %s edge, want %d-%d -- "+
				"the plaque spans the canvas's full width, leaving only the erosion's own ~6px "+
				"margin, so a wider band means the artwork is narrower than it should be and a "+
				"narrower one means it is being clipped",
				appIconPath, got, side.name, minTransparentBandCols, maxTransparentBandCols)
		}
	}
}

const (
	// PNG IHDR colour-type values that carry an alpha channel intrinsically.
	// Types 0 (greyscale), 2 (truecolour) and 3 (palette) do not; a palette
	// PNG can carry transparency via a tRNS chunk, which this master is not
	// and which is deliberately not accepted here.
	pngGreyscaleAlpha  = 4
	pngTruecolourAlpha = 6
)

// pngColourType returns the IHDR colour-type byte. A PNG is an 8-byte
// signature, then a 4-byte length, then "IHDR", then 13 bytes of header
// whose 10th byte is the colour type.
func pngColourType(data []byte) (byte, bool) {
	const colourTypeOffset = 8 + 4 + 4 + 9
	if len(data) <= colourTypeOffset || !bytes.Equal(data[:len(pngMagic)], pngMagic) ||
		!bytes.Equal(data[12:16], []byte("IHDR")) {
		return 0, false
	}
	return data[colourTypeOffset], true
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
	assertBorderRingHasNoOpaqueBackdrop(t, appIconPath, master, borderRingWidth, true)

	entries := readICOEntries(t, windowsIcoPath)
	entry := entryOfSize(t, windowsIcoPath, entries, largestEntrySize)
	icoImg := decodeICOEntryImage(t, windowsIcoPath, entry)
	label := fmt.Sprintf("%s (%dx%d entry)", windowsIcoPath, entry.width, entry.height)
	assertBorderRingHasNoOpaqueBackdrop(t, label, icoImg, ringWidthFor(entry.width), true)
}

// TestAppIconCarriesRealColour pins the "Real colour" criterion against
// both shipped assets. Mutation: set the master's alpha to 0 everywhere,
// keeping colour -> must fail.
func TestAppIconCarriesRealColour(t *testing.T) {
	master := decodePNGFile(t, appIconPath)
	assertCentralSaturationExceeds(t, appIconPath, master, minCentralSaturation)

	entries := readICOEntries(t, windowsIcoPath)
	entry := entryOfSize(t, windowsIcoPath, entries, largestEntrySize)
	icoImg := decodeICOEntryImage(t, windowsIcoPath, entry)
	label := fmt.Sprintf("%s (%dx%d entry)", windowsIcoPath, entry.width, entry.height)
	assertCentralSaturationExceeds(t, label, icoImg, minCentralSaturation)
}

// TestAppIconEveryIcoEntryCarriesTheArtwork pins the "Small sizes"
// criterion. Every entry is decoded and checked, not a chosen few: review
// loop 2 found three of the six (128, 64, 48) were read as directory
// metadata only and never decoded at all, so a stale or corrupt entry at any
// of those sizes shipped with the suite green. The 16 and 32 entries matter
// most -- they are the sizes the artwork was chosen for, per the spec's
// edge-energy measurement -- but "every" is what makes the criterion mean
// what it says. Mutation: replace any single entry with opaque white ->
// must fail, naming that size.
func TestAppIconEveryIcoEntryCarriesTheArtwork(t *testing.T) {
	entries := readICOEntries(t, windowsIcoPath)
	if len(entries) < len(wantIcoSizes) {
		t.Fatalf("%s carries %d entries, want at least %d -- a short set would make this loop "+
			"pass by checking fewer images than the .ico is supposed to hold",
			windowsIcoPath, len(entries), len(wantIcoSizes))
	}
	for _, entry := range entries {
		img := decodeICOEntryImage(t, windowsIcoPath, entry)
		label := fmt.Sprintf("%s (%dx%d entry)", windowsIcoPath, entry.width, entry.height)
		assertCentralSaturationExceeds(t, label, img, minCentralSaturation)
		assertBorderRingHasNoOpaqueBackdrop(t, label, img, ringWidthFor(entry.width), false)
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

	// Equality, not containment. A superset satisfies "carries the full set"
	// while carrying provenance this criterion is meant to reject -- a
	// hand-assembled .ico keeping the scaffold's 24x24 alongside a 48x48
	// would pass a missing-sizes check alone.
	want := map[int]bool{}
	for _, size := range wantIcoSizes {
		want[size] = true
	}
	var unexpected []string
	for _, e := range entries {
		if e.width != e.height || !want[e.width] {
			unexpected = append(unexpected, fmt.Sprintf("%dx%d", e.width, e.height))
		}
	}
	if len(unexpected) > 0 || len(entries) != len(wantIcoSizes) {
		t.Errorf("%s carries %d entries %v, want exactly the %d-entry set {%v} winicon.GenerateIcon "+
			"emits for the literal at packager.go:217 (unexpected: %v) -- an extra entry means this "+
			".ico was not produced by that call alone",
			windowsIcoPath, len(entries), sizes, len(wantIcoSizes), wantIcoSizes, unexpected)
	}
}

// TestAppIconMasterMatchesIcoFreshness pins the "Freshness" criterion.
// Mutation: replace the master with hue-swapped artwork, leave the .ico ->
// must fail.
func TestAppIconMasterMatchesIcoFreshness(t *testing.T) {
	master := decodePNGFile(t, appIconPath)
	masterGrid := gridOf(master)
	entries := readICOEntries(t, windowsIcoPath)

	// EVERY entry, not just the largest. Review loop 3 found that replacing
	// any of the 16, 32, 48, 64 or 128 entries with a flat opaque square of
	// the plaque's own colour -- no artwork at all -- passed the entire suite,
	// because only the 256 entry was ever compared against the master. Those
	// are the sizes Windows actually draws in the taskbar, and the sizes the
	// artwork was chosen for. This also subsumes the coloured-backdrop case
	// the border-ring check admits it cannot see.
	compared := map[int]bool{}
	for _, entry := range entries {
		if entry.width != entry.height {
			t.Errorf("%s declares a %dx%d entry; a non-square entry has no square size whose "+
				"tolerance applies", windowsIcoPath, entry.width, entry.height)
			continue
		}
		icoImg := decodeICOEntryImage(t, windowsIcoPath, entry)
		downsampled := downsampleBoxAverage(masterGrid, entry.width, entry.height)
		icoGrid := gridOf(icoImg)

		// Catch a dimension mismatch here rather than letting
		// meanChannelDistance255 return math.MaxFloat64, which would fail
		// reporting 1.8e308 per channel and hide the real cause.
		if downsampled.w != icoGrid.w || downsampled.h != icoGrid.h {
			t.Fatalf("%s: the %dx%d entry decoded at %dx%d, so it cannot be compared against the "+
				"master downsampled to %dx%d", windowsIcoPath, entry.width, entry.height,
				icoGrid.w, icoGrid.h, downsampled.w, downsampled.h)
		}

		measured, ok := measuredEntryDistance255[entry.width]
		if !ok {
			t.Errorf("%s carries a %dx%d entry with no measured same-artwork distance -- add one to "+
				"measuredEntryDistance255 (measure it, do not guess) before this size can be checked",
				windowsIcoPath, entry.width, entry.height)
			continue
		}
		tolerance := measured * freshnessToleranceFactor

		dist := meanChannelDistance255(downsampled, icoGrid)
		// Always logged, so drift toward the bound is visible in -v output
		// rather than only becoming apparent when it finally crosses.
		t.Logf("%dx%d: distance %.3f (measured %.3f, bound %.3f)",
			entry.width, entry.height, dist, measured, tolerance)
		compared[entry.width] = true

		// A bound derived from a measurement is only honest while the
		// measurement holds. If a toolchain change moved the real distance
		// well BELOW the recorded one, every bound would silently become
		// several times too wide -- the factor drifting exactly as an absolute
		// would. Catch that from underneath too, and name the constant.
		if dist < measured/2 {
			t.Errorf("%s: the %dx%d distance is %.3f, less than half the %.3f recorded in "+
				"measuredEntryDistance255 -- the recorded value is stale, so every bound derived "+
				"from it is too wide. Re-measure with scripts/verify-asset-mutations.py.",
				windowsIcoPath, entry.width, entry.height, dist, measured)
		}

		if dist > tolerance {
			t.Errorf("%s downsampled to %dx%d differs from %s's %dx%d entry by a mean %.3f per "+
				"channel (0-255 scale), want <= %.3f (%.1fx the %.3f these assets measure when they "+
				"genuinely match) -- the .ico was not generated from this master. If you changed the "+
				"artwork, delete %s and re-run `wails build`: Wails only generates it when absent.",
				appIconPath, entry.width, entry.height, windowsIcoPath, entry.width, entry.height,
				dist, tolerance, freshnessToleranceFactor, measured, windowsIcoPath)
		}
	}

	// Distinct SIZES, not a count of entries: two entries at one size with
	// another absent would otherwise reach six and leave a size never compared.
	for _, size := range wantIcoSizes {
		if !compared[size] {
			t.Errorf("%s: the %dx%d entry was never compared against the master", windowsIcoPath, size, size)
		}
	}
	if len(compared) != len(wantIcoSizes) {
		t.Errorf("%s: compared %d distinct sizes against the master, want %d",
			windowsIcoPath, len(compared), len(wantIcoSizes))
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

	// maxLowToHighDeviationRatio bounds the low-alpha bucket against the
	// high-alpha one, so the alpha-correlated asymmetry this test is named for
	// is asserted rather than merely printed.
	//
	// Measured, and re-measured after loop 2's comment here was found stale:
	// the genuine master gives 3.172 / 2.621 = **1.21** (loop 2's comment said
	// 3.19 / 2.47 = 1.3, numbers that predated its own changes). A full
	// premultiply defect gives 27.96 / 3.93 = 7.1 -- but that trips the
	// absolute bound too, so it proves nothing about this one. The case that
	// does is a 20% premultiply: 6.946 / 3.001 = 2.31, which stays UNDER
	// maxLowAlphaMeanDeviation (8.0) and is caught only here. It is case 10 in
	// scripts/verify-asset-mutations.py for exactly that reason.
	maxLowToHighDeviationRatio = 2.0
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
// bounded transition run in the whole master. A row crosses the plaque's
// outer alpha boundary TWICE -- entering on the left and leaving on the
// right -- so each row contributes up to two runs, and both are measured: an
// earlier version's comment claimed a single crossing and its code discarded
// every trailing run, leaving half the boundary unchecked. Each run's pixels
// are compared against the adjacent opaque pixel on the side the run meets
// it (the one it reaches on a leading edge, the one it left on a trailing
// edge), which is guaranteed to be the same local feature. Pooling
// thousands of samples this way averages out the
// photographic noise (a zero-mean, alpha-independent nuisance) while
// staying highly sensitive to the defect's actual signature: a mean
// deviation for low-alpha pixels that is dramatically larger than for
// high-alpha pixels, because darkening scales with (1 - alpha/255).
func TestAppIconMasterAppliesAlphaOnce(t *testing.T) {
	img := decodePNGFile(t, appIconPath)
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	var lowSamples, highSamples []float64
	runsUsed, leadingRuns, trailingRuns := 0, 0, 0

	flush := func(run []partialPixel, ref partialPixel) {
		for _, p := range run {
			dev := (absDiffF(p.r, ref.r) + absDiffF(p.g, ref.g) + absDiffF(p.b, ref.b)) / 3
			if p.a < lowAlphaThreshold255 {
				lowSamples = append(lowSamples, dev)
			} else {
				highSamples = append(highSamples, dev)
			}
		}
		runsUsed++
	}

	for row := 0; row < h; row++ {
		y := bounds.Min.Y + row
		var run []partialPixel
		var lastOpaque partialPixel
		haveOpaque, poisoned := false, false
		for col := 0; col < w; col++ {
			x := bounds.Min.X + col
			r, g, b, a := straightRGBA(img.At(x, y))
			switch {
			case a == 0:
				// Trailing edge. An earlier version discarded these runs
				// outright, which threw away roughly half the boundary
				// unmeasured (969 leading runs used against 979 trailing runs
				// dropped), so a defect confined to the plaque's right or
				// bottom edge would not have been seen. The reference is the
				// opaque pixel the run just left, not one it never reaches.
				if !poisoned && len(run) > 0 && haveOpaque {
					flush(run, lastOpaque)
					trailingRuns++
				}
				run, haveOpaque, poisoned = nil, false, false
			case a == 255:
				if !poisoned && len(run) > 0 {
					flush(run, partialPixel{r, g, b, a})
					leadingRuns++
				}
				run = nil
				lastOpaque, haveOpaque = partialPixel{r, g, b, a}, true
			default:
				run = append(run, partialPixel{r, g, b, a})
				if len(run) > maxTransitionRunLength {
					// Likely crossing a rounded corner at a shallow angle
					// rather than the plaque's straight edge, where the
					// nearest opaque pixel can belong to a different feature.
					// Poison the rest of the row rather than merely dropping
					// these pixels: clearing `run` alone still let the run's
					// final short tail reach the pool this guard exists to
					// keep out. The poison clears at the next alpha == 0,
					// which is a clean feature boundary.
					poisoned = true
					run = nil
				}
			}
		}
		// A row that reaches the right image edge mid-run still has a valid
		// reference behind it -- the opaque pixel the run left -- so it is
		// flushed rather than dropped. This master's plaque does not touch the
		// edge, so nothing is gained today; artwork that does would otherwise
		// go unmeasured along that edge.
		if !poisoned && len(run) > 0 && haveOpaque {
			flush(run, lastOpaque)
			trailingRuns++
		}
	}

	// Floors at roughly half the measured values, not a token non-zero check.
	// The committed master yields runsUsed 1902 (leading 951, trailing 951),
	// 6758 low-alpha and 6874 high-alpha samples. Loop 2's floors were 100 and
	// 200 -- 5% and 3% of those -- which is the same oversized pass corridor
	// this loop condemned in the freshness tolerance. The leading and trailing
	// counts are checked SEPARATELY on purpose: the defect loop 2 fixed was
	// that trailing runs were silently discarded, and a combined floor would
	// not notice that happening again.
	if leadingRuns < 400 || trailingRuns < 400 {
		t.Fatalf("%s: %d leading-edge and %d trailing-edge transition runs, want at least 400 of "+
			"each -- the master's own edges yield about 951 of each, and a collapse in either "+
			"direction means half the alpha boundary is going unmeasured, which is exactly the "+
			"defect this test was rewritten to fix",
			appIconPath, leadingRuns, trailingRuns)
	}
	if len(lowSamples) < 3000 || len(highSamples) < 3000 {
		t.Fatalf("%s: only %d low-alpha and %d high-alpha samples (master yields ~6758/~6874), "+
			"too few for the means below to be stable", appIconPath, len(lowSamples), len(highSamples))
	}

	lowMean := meanFloat(lowSamples)
	highMean := meanFloat(highSamples)
	// Logged always, so the calibration in the constants above can be checked
	// against reality instead of trusted. Loop 2's comment claimed 3.19/2.47
	// while the code measured something else, which is how a stale number
	// survives a review that reads it.
	t.Logf("runs %d (leading %d, trailing %d); low-alpha mean %.3f over %d, high-alpha mean %.3f over %d",
		runsUsed, leadingRuns, trailingRuns, lowMean, len(lowSamples), highMean, len(highSamples))
	if highMean == 0 {
		t.Fatalf("%s: high-alpha pixels deviate 0.00 from their references, so the ratio below is "+
			"undefined -- a fabricated flat-colour master reaches this, and it is not artwork",
			appIconPath)
	}

	// Two independent checks, not a switch. As a switch the ratio case only
	// evaluated when the absolute bound held, and both premultiply mutations
	// trip the absolute bound -- so the ratio constant was never executed by
	// any mutation, which by this repo's own standard made it decoration.
	if lowMean > maxLowAlphaMeanDeviation {
		t.Errorf("%s: partial-alpha pixels with alpha < %d deviate from their transition run's local "+
			"opaque reference pixel by a mean of %.2f (0-255 scale, %d samples across %d runs); "+
			"alpha >= %d pixels deviate by only %.2f (%d samples) -- RGB drifting further from the "+
			"true colour as alpha drops is what applying alpha twice (pasting an RGBA image using its "+
			"own alpha as the paste mask) looks like, want <= %.2f",
			appIconPath, lowAlphaThreshold255, lowMean, len(lowSamples), runsUsed,
			lowAlphaThreshold255, highMean, len(highSamples), maxLowAlphaMeanDeviation)
	}
	// The absolute bound alone leaves the asymmetry this test is named for
	// unchecked: noisier artwork could raise both buckets together and still be
	// correct, while a defect that scales with (1 - alpha/255) shows up as a
	// ratio no amount of uniform noise produces.
	if lowMean > highMean*maxLowToHighDeviationRatio {
		t.Errorf("%s: low-alpha pixels deviate %.3f from their local opaque reference while "+
			"high-alpha pixels deviate only %.3f -- a ratio of %.2f, want <= %.2f. Uniform "+
			"photographic noise raises both buckets together; deviation that grows as alpha falls "+
			"is the signature of alpha applied twice.",
			appIconPath, lowMean, highMean, lowMean/highMean, maxLowToHighDeviationRatio)
	}
}

const (
	// wantSourceWidth and wantSourceHeight are the render dimensions
	// scripts/build-appicon.py's CROP_BOX (896, 239, 1920, 1232) and
	// CORNER_RADIUS (226) were fitted against by least squares. Crop geometry
	// is meaningless against a different-sized source, and Pillow pads an
	// out-of-range crop silently rather than failing.
	wantSourceWidth  = 2816
	wantSourceHeight = 1536

	// wantSourceSHA256 is the same digest scripts/build-appicon.py pins in
	// EXPECTED_SOURCE_SHA256, asserted here too because the script's copy only
	// runs when a human runs the script. Dimensions alone cannot tell one
	// candidate render from another: the owner supplied three at identical
	// dimensions, and review loop 3 confirmed that replacing this file with a
	// different 2816x1536 render passed every assertion in this file.
	wantSourceSHA256 = "b0aeab41a13f4b81fd0eea778323979708594c7fd862f20bb415183be41f61b0"
)

// TestAppIconSourceRenderIsThePinnedGeometry pins the "Source provenance"
// criterion. build/appicon-source.jpg is the only committed copy of the
// owner's render -- .gitignore excludes the originals from the repo root --
// and both epics.md's reproducibility criterion and build/README.md's
// re-derivation instructions rest on it. Nothing verified it before: review
// loop 2 renamed the file out of the tree and the entire suite still passed,
// so the provenance could be deleted, truncated or replaced with a different
// render while the gate stayed green. The script's own size guard runs only
// when a human runs the script.
//
// Mutation: delete build/appicon-source.jpg -> must fail.
func TestAppIconSourceRenderIsThePinnedGeometry(t *testing.T) {
	data, err := os.ReadFile(appIconSource)
	if err != nil {
		t.Fatalf("read %s: %v -- this is the only committed copy of the render the master is "+
			"derived from, and scripts/build-appicon.py cannot run without it", appIconSource, err)
	}
	// Decode the whole image, not just the header: DecodeConfig reads only the
	// SOF marker, so a truncated or body-corrupt JPEG would satisfy it while
	// being unusable to the derivation this file claims it guarantees.
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode %s: %v -- the header may be intact while the image data is not",
			appIconSource, err)
	}
	if b := img.Bounds(); b.Dx() != wantSourceWidth || b.Dy() != wantSourceHeight {
		t.Errorf("%s is %dx%d, want %dx%d -- scripts/build-appicon.py's CROP_BOX and CORNER_RADIUS "+
			"were fitted against that exact render, so a different size means the committed master "+
			"cannot be re-derived from this file",
			appIconSource, b.Dx(), b.Dy(), wantSourceWidth, wantSourceHeight)
	}
	// scripts/build-appicon.py pins the same digest. Nothing but a README step
	// keeps the two in step, so assert their agreement here rather than trusting
	// that whoever updates one remembers the other.
	script, err := os.ReadFile(filepath.Join("scripts", "build-appicon.py"))
	if err != nil {
		t.Fatalf("read the derivation script: %v", err)
	}
	if want := fmt.Sprintf("EXPECTED_SOURCE_SHA256 = %q", wantSourceSHA256); !bytes.Contains(script, []byte(want)) {
		t.Errorf("scripts/build-appicon.py does not pin %s -- the test and the derivation would "+
			"accept different renders, and only a README step keeps them aligned", wantSourceSHA256)
	}

	if got := hex.EncodeToString(sha256Sum(data)); got != wantSourceSHA256 {
		t.Errorf("%s has sha256 %s, want %s -- this is not the render the crop geometry was fitted "+
			"against. If the artwork is deliberately changing, re-fit the geometry and update both "+
			"this constant and EXPECTED_SOURCE_SHA256 in scripts/build-appicon.py",
			appIconSource, got, wantSourceSHA256)
	}
}

func sha256Sum(b []byte) []byte {
	sum := sha256.Sum256(b)
	return sum[:]
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

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
