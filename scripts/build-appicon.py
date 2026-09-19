#!/usr/bin/env python3
"""Derive build/appicon.png from the committed master JPEG.

Spec: _bmad-output/implementation-artifacts/spec-6-1-replace-the-placeholder-app-icon.md

build/appicon-source.jpg is a plain opaque JPEG -- none of the three
candidate renders the owner supplied carries an alpha channel, including the
one (this one) whose backdrop is drawn as a transparency checkerboard. That
checkerboard is decoration, not alpha, so a naive format conversion would
ship it as opaque grey pixels behind the icon. This script instead crops the
plaque out of the source render and synthesises real alpha from a measured
rounded-rect silhouette.

This is a derivation, not a pipeline: it runs by hand, on demand, and its
output is committed. It is deliberately not wired into verify.yml -- the
spec's Never clause -- so nothing in CI depends on Pillow or on this script
still producing byte-identical output across Pillow versions. Re-run it with:

    python scripts/build-appicon.py

from any directory (paths below resolve from this file's own location, not
the current working directory), then `git diff --stat build/appicon.png` to
see what changed, and delete + regenerate build/windows/icon.ico with
`wails build` (see build/README.md) since Wails only ever generates that
file when it is absent.
"""

from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter

# Resolved from this file's location rather than left relative, so the script
# behaves the same run from the repository root or from anywhere else --
# review loop 1 found a plain relative path fails, or silently writes into
# the wrong directory, the moment it is not run from repo root.
REPO_ROOT = Path(__file__).resolve().parents[1]
SOURCE = REPO_ROOT / "build" / "appicon-source.jpg"
OUTPUT = REPO_ROOT / "build" / "appicon.png"

# The source JPEG the geometry below was fitted against. Pillow pads or
# silently accepts an out-of-range crop box rather than erroring, so a
# differently-sized source (a re-export, a different candidate swapped in
# without updating CROP_BOX) would produce a silently wrong master instead of
# a loud failure. main() asserts this before cropping.
EXPECTED_SOURCE_SIZE = (2816, 1536)

# Measured by least-squares fit over 60 rows of the source render's
# rounded-rect silhouette, rms 1.74px (see the spec's Code Map). Box is
# (left, top, right, bottom) in the 2816x1536 source JPEG's own coordinates.
#
# The fitted top edge sits at 237, one pixel inside the crop below -- 239 is
# used instead because 237 sits inside the background's soft bloom around
# the plaque, and cropping there would bake ~133 near-white border pixels
# into the mask before erosion ever runs. See ERODE_PX.
CROP_BOX = (896, 239, 1920, 1232)

# The plaque's corners are circular in source space at this radius, from the
# same fit. A stretched (non-circular) corner from a 2.9% foreshortening
# would have shown roughly 6px of residual fit error; the fit lands at
# 1.74px, so the plaque is genuinely round, not a squashed square -- do not
# "fix" this back to looking square.
CORNER_RADIUS = 226

# Even at the corrected 239px top edge, the fitted silhouette still sits
# inside the background's bloom, leaving ~133 near-white pixels along the
# border if the alpha mask followed the crop exactly. Eroding the mask
# inward by 9px clears it (edge-ring p95 drops from 203 to 85; the residual
# near-white corner pixel count drops from 133 to 8).
#
# This is implemented as an inset + reduced corner radius rather than a
# morphological filter: for a convex shape whose boundary is straight edges
# and circular arcs, insetting the bounding box by e and reducing the arc
# radius by e (floored at 0) IS the exact Minkowski erosion by e -- no
# approximation, and no new dependency (e.g. scipy) to get it. See build_mask
# for the inclusive-endpoint correction PIL's rounded_rectangle needs to make
# that inset exact rather than one supersampled pixel too generous.
ERODE_PX = 9

# Softens the eroded mask's hard edge so the alpha transition is a gradient,
# not a single aliased step -- the source JPEG has no alpha of its own to
# anti-alias against.
FEATHER_PX = 1.2

# Final canvas. The crop is 1024x993 -- not square -- and per the spec it is
# centred rather than stretched to fill 1024x1024: stretching would restyle
# the artwork (see CORNER_RADIUS's note -- the non-square crop is real
# geometry, not a foreshortening artefact to correct for). Centring leaves a
# transparent band of exactly (1024 - 993) // 2 = 15px on top and the
# remaining 16px on the bottom (the split is uneven because 1024 - 993 = 31
# is odd) -- not "~15-16px" as an earlier draft of this comment said; both
# numbers are exact for this crop, not a range.
CANVAS_SIZE = 1024

# The rounded-rect mask geometry (and its erosion) is drawn at this multiple
# of final resolution, then downsampled with Lanczos resampling, so the arc
# is anti-aliased against a much finer grid than the 1024px output before it
# ever reaches pixel boundaries.
SUPERSAMPLE = 4


def build_mask(size):
    """Return an L-mode alpha mask: an eroded, feathered rounded rect."""
    width, height = size
    big = (width * SUPERSAMPLE, height * SUPERSAMPLE)
    mask = Image.new("L", big, 0)
    draw = ImageDraw.Draw(mask)

    inset = ERODE_PX * SUPERSAMPLE
    radius = max((CORNER_RADIUS - ERODE_PX) * SUPERSAMPLE, 0)
    # PIL's rounded_rectangle treats its box as inclusive of both endpoints
    # (a box of (0, 0, N, N) spans N+1 pixels), so a box built as
    # (inset, inset, big-inset, big-inset) draws one supersampled pixel past
    # the intended right/bottom edge -- an asymmetric erosion that quietly
    # falsified the "exact Minkowski erosion" claim above. The -1 makes the
    # box span exactly big - 2*inset pixels on every side, matching the left
    # and top edges exactly.
    box = (inset, inset, big[0] - inset - 1, big[1] - inset - 1)
    draw.rounded_rectangle(box, radius=radius, fill=255)

    mask = mask.resize(size, Image.LANCZOS)
    if FEATHER_PX > 0:
        mask = mask.filter(ImageFilter.GaussianBlur(FEATHER_PX))
    return mask


def main():
    source = Image.open(SOURCE).convert("RGB")
    if source.size != EXPECTED_SOURCE_SIZE:
        raise SystemExit(
            f"{SOURCE} is {source.size[0]}x{source.size[1]}, expected "
            f"{EXPECTED_SOURCE_SIZE[0]}x{EXPECTED_SOURCE_SIZE[1]}: CROP_BOX and CORNER_RADIUS "
            f"were fitted against that exact size, and Pillow crops out-of-range silently rather "
            f"than erroring, so a differently-sized source would produce a silently wrong master."
        )

    crop = source.crop(CROP_BOX)
    if crop.width > CANVAS_SIZE or crop.height > CANVAS_SIZE:
        raise SystemExit(
            f"the crop is {crop.width}x{crop.height}, which does not fit in the "
            f"{CANVAS_SIZE}x{CANVAS_SIZE} canvas: paste() would silently truncate it rather than "
            f"erroring. Check CROP_BOX and CANVAS_SIZE."
        )

    mask = build_mask(crop.size)

    plaque = crop.convert("RGBA")
    plaque.putalpha(mask)

    canvas = Image.new("RGBA", (CANVAS_SIZE, CANVAS_SIZE), (0, 0, 0, 0))
    offset_x = (CANVAS_SIZE - plaque.width) // 2
    offset_y = (CANVAS_SIZE - plaque.height) // 2
    # Maskless paste: the canvas is fully transparent everywhere the plaque
    # is not, so this is a direct pixel copy of the plaque's own (correct)
    # straight alpha, not a composite. Passing `plaque` a second time as the
    # mask argument -- the review-loop-1 defect -- tells PIL to alpha-blend
    # using the plaque's alpha *again* on top of its own per-pixel values,
    # which applies alpha twice (output alpha becomes alpha^2) and blends
    # RGB toward the transparent canvas's black, producing a dark fringe and
    # a feather roughly half its intended width. A maskless paste (or
    # equivalently Image.alpha_composite, since the destination region
    # starts fully transparent) copies the already-correct RGBA verbatim.
    canvas.paste(plaque, (offset_x, offset_y))

    canvas.save(OUTPUT, optimize=True)
    print(f"wrote {OUTPUT} ({canvas.size[0]}x{canvas.size[1]}, mode={canvas.mode})")


if __name__ == "__main__":
    main()
