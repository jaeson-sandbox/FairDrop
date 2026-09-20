#!/usr/bin/env python3
"""Canonical mutation driver for Epic 6's icon assets.

`scripts/verify-native-mutations.sh` mutates Go sources; the icon criteria are
about binary assets, which that harness cannot express. This is their
equivalent, and it exists for the reason AGENTS.md gives under Story 3.8:
"Scoped mutation proof must be derived from the canonical script, not a
hand-copied case list." Review loop 2's table was hand-written and carried
three stale numbers into a merged evidence file; loop 3 found them.

Run it from anywhere:

    python scripts/verify-asset-mutations.py            # mutation table
    python scripts/verify-asset-mutations.py --measure  # also re-measure the
                                                        # calibration constants

Every mutation is applied to the real committed assets, the real test suite is
run against it, and the asset is restored from git and sha256-verified before
the next one. A restore that does not match aborts the run rather than
continuing against a corrupted tree.

Requires Pillow (see build/README.md). It is deliberately NOT wired into
verify.yml -- the spec's Never clause -- so nothing in CI depends on Pillow.
"""

import argparse
import hashlib
import io
import os
import struct
import subprocess
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFilter

REPO = Path(__file__).resolve().parents[1]
PNG = "build/appicon.png"
ICO = "build/windows/icon.ico"
SRC = "build/appicon-source.jpg"
ASSETS = (PNG, ICO, SRC)

# Mirrors scripts/build-appicon.py; imported by value rather than by import so
# a mutation of that script cannot quietly change what this driver compares to.
CROP_BOX = (896, 239, 1920, 1232)
CORNER_RADIUS = 226
SUPERSAMPLE = 4
FEATHER_PX = 1.2
CANVAS_SIZE = 1024

ENV = dict(os.environ)
ENV["PATH"] = r"C:\Program Files\Go\bin" + os.pathsep + \
    str(Path.home() / "go" / "bin") + os.pathsep + ENV.get("PATH", "")


def sha(path):
    return hashlib.sha256((REPO / path).read_bytes()).hexdigest()


BASELINE = {}


def restore():
    subprocess.run(["git", "checkout", "--", *ASSETS], cwd=REPO, check=True)
    for path in ASSETS:
        if sha(path) != BASELINE[path]:
            sys.exit("RESTORE FAILED for %s -- aborting rather than mutating further" % path)


def run_tests():
    """Return the set of failing TestAppIcon* names, minus the prefix."""
    proc = subprocess.run(
        ["go", "test", "-count=1", "-run", "^TestAppIcon", "-v", "."],
        capture_output=True, text=True, env=ENV, cwd=REPO,
    )
    return {
        line.split()[2].replace("TestAppIcon", "")
        for line in proc.stdout.splitlines()
        if line.startswith("--- FAIL:")
    }, proc.stdout


def load(path):
    data = (REPO / path).read_bytes()
    img = Image.open(io.BytesIO(data))
    img.load()
    return img.convert("RGBA")


def read_ico():
    raw = (REPO / ICO).read_bytes()
    count = struct.unpack("<H", raw[4:6])[0]
    out = []
    for i in range(count):
        entry = raw[6 + i * 16:22 + i * 16]
        size, off = struct.unpack("<II", entry[8:16])
        out.append({"w": entry[0] or 256, "hdr": bytearray(entry), "data": raw[off:off + size]})
    return out


def write_ico(entries):
    off = 6 + 16 * len(entries)
    directory = b""
    blob = b""
    for e in entries:
        hdr = bytearray(e["hdr"])
        struct.pack_into("<II", hdr, 8, len(e["data"]), off)
        directory += bytes(hdr)
        blob += e["data"]
        off += len(e["data"])
    (REPO / ICO).write_bytes(struct.pack("<HHH", 0, 1, len(entries)) + directory + blob)


def derive(erode=9, premultiply=0.0, stretch=False):
    """Re-run the real derivation with a deliberate defect.

    premultiply is how far toward a second alpha application to go: 1.0 is the
    defect review loop 1 found shipped (low-alpha deviation 27.96, which trips
    the absolute bound), and 0.20 a subtler one measuring 6.946 against a
    high-alpha 3.001 -- under the absolute bound of 8.0, so it is caught ONLY
    by the low/high ratio. Without that case the ratio constant is decoration.
    """
    src = Image.open(REPO / SRC).convert("RGB").crop(CROP_BOX)
    w, h = src.size
    big = (w * SUPERSAMPLE, h * SUPERSAMPLE)
    mask = Image.new("L", big, 0)
    inset = erode * SUPERSAMPLE
    radius = max((CORNER_RADIUS - erode) * SUPERSAMPLE, 0)
    ImageDraw.Draw(mask).rounded_rectangle(
        [inset, inset, big[0] - inset - 1, big[1] - inset - 1], radius=radius, fill=255)
    mask = mask.resize((w, h), Image.LANCZOS).filter(ImageFilter.GaussianBlur(FEATHER_PX))
    plaque = src.convert("RGBA")
    plaque.putalpha(mask)
    if stretch:
        plaque = plaque.resize((CANVAS_SIZE, CANVAS_SIZE), Image.LANCZOS)
    canvas = Image.new("RGBA", (CANVAS_SIZE, CANVAS_SIZE), (0, 0, 0, 0))
    canvas.paste(plaque, ((CANVAS_SIZE - plaque.width) // 2, (CANVAS_SIZE - plaque.height) // 2))
    if premultiply:
        px = canvas.load()
        for y in range(CANVAS_SIZE):
            for x in range(CANVAS_SIZE):
                r, g, b, a = px[x, y]
                if 0 < a < 255:
                    k = 1.0 - premultiply * (1.0 - a / 255.0)
                    px[x, y] = (int(r * k), int(g * k), int(b * k), a)
    canvas.save(REPO / PNG)


def brighten(delta):
    img = load(PNG)
    px = img.load()
    for y in range(img.height):
        for x in range(img.width):
            r, g, b, a = px[x, y]
            if a:
                px[x, y] = (min(r + delta, 255), min(g + delta, 255), min(b + delta, 255), a)
    img.save(REPO / PNG)


def flat_entry(size, colour=None):
    entries = read_ico()
    target = next(e for e in entries if e["w"] == size)
    fill = colour or (load(PNG).getpixel((512, 512))[:3] + (255,))
    buf = io.BytesIO()
    Image.new("RGBA", (size, size), fill).save(buf, "PNG")
    target["data"] = buf.getvalue()
    write_ico(entries)


def blank_master():
    Image.new("RGBA", (CANVAS_SIZE, CANVAS_SIZE), (0, 0, 0, 0)).save(REPO / PNG)


def scaffold_master():
    out = subprocess.run(["git", "show", "de16807:build/appicon.png"],
                         capture_output=True, cwd=REPO).stdout
    (REPO / PNG).write_bytes(out)


def scaffold_ico():
    path = Path.home() / "go/pkg/mod/github.com/wailsapp/wails/v2@v2.15.0/pkg/buildassets/build/windows/icon.ico"
    (REPO / ICO).write_bytes(path.read_bytes())


def extra_entry(size=24):
    entries = read_ico()
    donor = next(e for e in entries if e["w"] == 32)
    img = Image.open(io.BytesIO(donor["data"])).convert("RGBA").resize((size, size), Image.LANCZOS)
    buf = io.BytesIO()
    img.save(buf, "PNG")
    hdr = bytearray(donor["hdr"])
    hdr[0] = hdr[1] = size
    entries.append({"w": size, "hdr": hdr, "data": buf.getvalue()})
    write_ico(entries)


def drop_entry(size=48):
    write_ico([e for e in read_ico() if e["w"] != size])


def narrow_master(shrink=240):
    img = load(PNG)
    plaque = img.crop(img.getbbox())
    plaque = plaque.resize((plaque.width - shrink, plaque.height), Image.LANCZOS)
    canvas = Image.new("RGBA", (CANVAS_SIZE, CANVAS_SIZE), (0, 0, 0, 0))
    canvas.paste(plaque, ((CANVAS_SIZE - plaque.width) // 2, (CANVAS_SIZE - plaque.height) // 2))
    canvas.save(REPO / PNG)


CASES = [
    ("master brightened +2/255, .ico stale", lambda: brighten(2), "MasterMatchesIcoFreshness"),
    ("master hue-swapped, .ico stale", lambda: _hue(), "MasterMatchesIcoFreshness"),
    ("16x16 entry a flat coloured square", lambda: flat_entry(16), "MasterMatchesIcoFreshness"),
    ("48x48 entry a flat coloured square", lambda: flat_entry(48), "MasterMatchesIcoFreshness"),
    ("128x128 entry a flat coloured square", lambda: flat_entry(128), "MasterMatchesIcoFreshness"),
    ("16x16 entry opaque white", lambda: flat_entry(16, (255, 255, 255, 255)),
     "EveryIcoEntryCarriesTheArtwork"),
    ("re-derived with ERODE_PX = 0", lambda: derive(erode=0), "NoOpaqueBackdrop"),
    ("master alpha 0, colour intact", lambda: _alpha0(), "CarriesRealColour"),
    ("premultiply defect, full", lambda: derive(premultiply=1.0), "MasterAppliesAlphaOnce"),
    ("premultiply defect, 20% (ratio bound only)", lambda: derive(premultiply=0.20),
     "MasterAppliesAlphaOnce"),
    ("plaque stretched to fill canvas", lambda: derive(stretch=True), "MasterGeometry"),
    ("plaque narrowed 240px (columns)", narrow_master, "MasterGeometry"),
    ("master fully blank", blank_master, "MasterGeometry"),
    ("master is the Wails scaffold placeholder", scaffold_master, "MasterGeometry"),
    (".ico reverted to the Wails scaffold", scaffold_ico, "IcoCarriesTheFullWailsSizeSet"),
    (".ico 48x48 entry dropped", drop_entry, "IcoCarriesTheFullWailsSizeSet"),
    (".ico gains an extra 24x24 entry", extra_entry, "IcoCarriesTheFullWailsSizeSet"),
    ("source render deleted", lambda: os.remove(REPO / SRC), "SourceRenderIsThePinnedGeometry"),
    ("source render replaced, same size", lambda: _swap_source(), "SourceRenderIsThePinnedGeometry"),
]


def _hue():
    img = load(PNG)
    r, g, b, a = img.split()
    Image.merge("RGBA", (b, r, g, a)).save(REPO / PNG)


def _alpha0():
    img = load(PNG)
    r, g, b, _ = img.split()
    Image.merge("RGBA", (r, g, b, Image.new("L", img.size, 0))).save(REPO / PNG)


def _swap_source():
    Image.open(REPO / SRC).rotate(180).save(REPO / SRC, quality=95)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--measure", action="store_true",
                        help="also probe the freshness boundary size by size")
    args = parser.parse_args()

    for path in ASSETS:
        BASELINE[path] = sha(path)

    failing, _ = run_tests()
    if failing:
        sys.exit("baseline is not green (%s) -- fix that before trusting any mutation" % sorted(failing))
    print("baseline: all TestAppIcon* pass\n")

    print("| # | mutation | named test | killed |")
    print("|---|---|---|---|")
    bad = 0
    for i, (label, mutate, want) in enumerate(CASES, 1):
        restore()
        mutate()
        failing, _ = run_tests()
        named = want in failing
        if not named:
            bad += 1
        print("| %d | %s | `%s` | %s |" % (i, label, want, "yes" if named else "**NO**"))
    restore()
    print("\n%d of %d mutations failed their named test." % (len(CASES) - bad, len(CASES)))

    if args.measure:
        print("\nfreshness boundary, master brightened by +N with the .ico left stale:")
        for delta in (0, 1, 2, 3, 4):
            restore()
            if delta:
                brighten(delta)
            failing, _ = run_tests()
            print("  +%-3d %s" % (delta, "CAUGHT" if "MasterMatchesIcoFreshness" in failing else "missed"))
        restore()

    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main()
