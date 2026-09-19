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

from the repository root, then `git diff --stat build/appicon.png` to see
what changed, and delete + regenerate build/windows/icon.ico with
`wails build` (see build/README.md) since Wails only ever generates that
file when it is absent.
"""

from PIL import Image, ImageDraw, ImageFilter

SOURCE = "build/appicon-source.jpg"
OUTPUT = "build/appicon.png"

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
# approximation, and no new dependency (e.g. scipy) to get it.
ERODE_PX = 9

# Softens the eroded mask's hard edge so the alpha transition is a gradient,
# not a single aliased step -- the source JPEG has no alpha of its own to
# anti-alias against.
FEATHER_PX = 1.2

# Final canvas. The crop is 1024x993 -- not square -- and per the spec it is
# centred rather than stretched to fill 1024x1024: stretching would restyle
# the artwork (see CORNER_RADIUS's note -- the non-square crop is real
# geometry, not a foreshortening artefact to correct for). Centring leaves
# transparent bands of (1024 - 993) / 2 =~ 15-16px top and bottom.
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
    box = (inset, inset, big[0] - inset, big[1] - inset)
    draw.rounded_rectangle(box, radius=radius, fill=255)

    mask = mask.resize(size, Image.LANCZOS)
    if FEATHER_PX > 0:
        mask = mask.filter(ImageFilter.GaussianBlur(FEATHER_PX))
    return mask


def main():
    source = Image.open(SOURCE).convert("RGB")
    crop = source.crop(CROP_BOX)

    mask = build_mask(crop.size)

    plaque = crop.convert("RGBA")
    plaque.putalpha(mask)

    canvas = Image.new("RGBA", (CANVAS_SIZE, CANVAS_SIZE), (0, 0, 0, 0))
    offset_x = (CANVAS_SIZE - plaque.width) // 2
    offset_y = (CANVAS_SIZE - plaque.height) // 2
    canvas.paste(plaque, (offset_x, offset_y), plaque)

    canvas.save(OUTPUT)
    print(f"wrote {OUTPUT} ({canvas.size[0]}x{canvas.size[1]}, mode={canvas.mode})")


if __name__ == "__main__":
    main()
