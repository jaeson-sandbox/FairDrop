#!/usr/bin/env python3
"""Canonical mutation driver for Epic 6's icon assets.

`scripts/verify-native-mutations.sh` mutates Go sources; the icon criteria are
about binary assets, which that harness cannot express. This is their
equivalent, and it exists for the reason AGENTS.md gives under Story 3.8:
"Scoped mutation proof must be derived from the canonical script, not a
hand-copied case list." Review loop 2's table was hand-written and carried
three stale numbers into a merged evidence file; loop 3 found them.

Run it from anywhere:

    # Run only inside a disposable worktree/check-out:
    FAIRDROP_MUTATION_DISPOSABLE=1 python scripts/verify-asset-mutations.py
    FAIRDROP_MUTATION_DISPOSABLE=1 python scripts/verify-asset-mutations.py --measure
    FAIRDROP_MUTATION_DISPOSABLE=1 python scripts/verify-asset-mutations.py --exe

Every mutation is applied to an isolated checkout's implemented baseline. The
driver restores its snapshots and sha256-verifies them before the next case.
Catchable interrupts restore too; SIGKILL cannot run process cleanup, which is
why mutation runs belong in disposable worktrees.

Requires Pillow (see build/README.md). It is deliberately NOT wired into
verify.yml -- the spec's Never clause -- so nothing in CI depends on Pillow.
"""

import argparse
import atexit
import hashlib
import io
import os
import re
import shutil
import signal
import struct
import subprocess
import sys
import tempfile
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
PNG = "build/appicon.png"
ICO = "build/windows/icon.ico"
SRC = "build/appicon-source.jpg"
WAILS_JSON = "wails.json"
EXE = "build/bin/fairdrop.exe"
APPICON_TEST = "appicon_test.go"
DERIVATION = "scripts/build-appicon.py"
WINDOWS_INFO = "build/windows/info.json"
ASSETS = (PNG, ICO, SRC, APPICON_TEST, DERIVATION)
EXE_ASSETS = (ICO, WAILS_JSON, WINDOWS_INFO)
EXE_CASE_SPECS = (
    ("committed .ico no longer matches the exe", "stale_ico", "EmbedTheCommittedIcon", "differs from"),
    ("wails.json productVersion bumped, no rebuild", "bump_version", "CarryTheCommittedIdentity", "built artifact disagrees"),
    ("one RT_ICON directory entry stripped from the built exe", "strip_embedded_icon", "EmbedTheCommittedIcon", "embeds no"),
    ("rebuilt with a language-neutral version string table", "neutral_version_table", "CarryTheCommittedIdentity", "language key has changed"),
)

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
BASELINE_BYTES = {}


class TestRunError(RuntimeError):
    def __init__(self, message, output):
        super().__init__(message + "\n" + output)
        self.output = output


def restore():
    for path in ASSETS:
        (REPO / path).write_bytes(BASELINE_BYTES[path])
    for path in ASSETS:
        if sha(path) != BASELINE[path]:
            sys.exit("RESTORE FAILED for %s -- aborting rather than mutating further" % path)


def run_tests():
    """Return the set of failing TestAppIcon* names, minus the prefix."""
    proc = subprocess.run(
        ["go", "test", "-count=1", "-run", "^TestAppIcon", "-v", "."],
        capture_output=True, text=True, env=ENV, cwd=REPO,
    )
    output = proc.stdout + proc.stderr
    results, blocks = validate_test_run(proc.returncode, proc.stdout, output, "TestAppIcon")
    failing = {name.removeprefix("TestAppIcon") for name, status in results.items() if status == "FAIL"}
    return failing, blocks, output


def parse_test_results(stdout, prefix):
    results, blocks = {}, {}
    current, lines = None, []
    for line in stdout.splitlines():
        if line.startswith("=== RUN   "):
            current = line.split()[2]
            lines = [line]
        elif current:
            lines.append(line)
        if line.startswith(("--- PASS:", "--- FAIL:", "--- SKIP:")):
            parts = line.split()
            name, status = parts[2], parts[1].rstrip(":")
            if name.startswith(prefix):
                results[name] = status
                blocks[name] = "\n".join(lines)
            current, lines = None, []
    return results, blocks


def validate_test_run(returncode, stdout, output, prefix):
    results, blocks = parse_test_results(stdout, prefix)
    if not results:
        raise TestRunError("go test produced no test result lines -- the package did not run:", output)
    failed = any(status == "FAIL" for status in results.values())
    if returncode != (1 if failed else 0):
        raise TestRunError("go test exit status disagrees with its parsed test results:", output)
    return results, blocks


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
    pos = ((CANVAS_SIZE - plaque.width) // 2, (CANVAS_SIZE - plaque.height) // 2)
    canvas = Image.new("RGBA", (CANVAS_SIZE, CANVAS_SIZE), (0, 0, 0, 0))
    canvas.paste(plaque, pos)
    if premultiply >= 1.0:
        # The defect exactly as it shipped: pasting the plaque using ITSELF as
        # the mask. PIL blends all four bands against the transparent-black
        # canvas, so RGB is premultiplied a second time AND alpha becomes
        # a^2/255. Scaling RGB alone reproduces only half of it, and the alpha
        # half is what moves the bands, the ring and the freshness distance.
        canvas = Image.new("RGBA", (CANVAS_SIZE, CANVAS_SIZE), (0, 0, 0, 0))
        canvas.paste(plaque, pos, plaque)
    elif premultiply:
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


def _opaque_backdrop():
    img = load(PNG)
    px = img.load()
    w, h = img.size
    for y in range(h):
        for x in range(w):
            if px[x, y][3] == 0:
                px[x, y] = (190, 190, 190, 255)
    for corner in ((0, 0), (w - 1, 0), (0, h - 1), (w - 1, h - 1)):
        px[corner] = (0, 0, 0, 0)
    img.save(REPO / PNG)


def _rgb_master():
    load(PNG).convert("RGB").save(REPO / PNG)


def _one_pixel(size):
    entries = read_ico()
    target = next(e for e in entries if e["w"] == size)
    img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    img.putpixel((size // 2, size // 2), (200, 60, 20, 255))
    buf = io.BytesIO()
    img.save(buf, "PNG")
    target["data"] = buf.getvalue()
    write_ico(entries)


def _off_centre_master(rows=5):
    img = load(PNG)
    shifted = Image.new("RGBA", img.size, (0, 0, 0, 0))
    shifted.paste(img, (0, rows))
    shifted.save(REPO / PNG)


def _non_square_entry(size=64):
    entries = read_ico()
    target = next(e for e in entries if e["w"] == size)
    target["hdr"][1] = size - 1
    write_ico(entries)


def _inflate_calibration(size=16):
    path = REPO / APPICON_TEST
    source = path.read_text(encoding="utf-8")
    pattern = r"(?m)^(\s*%d:\s*)([0-9.]+)(,\s*)$" % size
    mutated, count = re.subn(pattern, lambda m: m.group(1) + "99.0" + m.group(3), source)
    if count != 1:
        sys.exit("could not uniquely find measuredEntryDistance255[%d] in %s" % (size, APPICON_TEST))
    path.write_text(mutated, encoding="utf-8")


def _diverge_source_digest():
    path = REPO / DERIVATION
    source = path.read_text(encoding="utf-8")
    mutated, count = re.subn(
        r'(?m)^EXPECTED_SOURCE_SHA256 = "[0-9a-f]{64}"$',
        'EXPECTED_SOURCE_SHA256 = "' + "0" * 64 + '"', source,
    )
    if count != 1:
        sys.exit("could not uniquely find EXPECTED_SOURCE_SHA256 in %s" % DERIVATION)
    path.write_text(mutated, encoding="utf-8")


def declared_ico_sizes():
    """Read the canonical Go declaration rather than repeating its values."""
    source = (REPO / APPICON_TEST).read_text(encoding="utf-8")
    match = re.search(r"var\s+wantIcoSizes\s*=\s*\[\]int\s*\{([^}]*)\}", source)
    if not match:
        sys.exit("could not parse wantIcoSizes from %s" % APPICON_TEST)
    body = match.group(1).strip()
    if not re.fullmatch(r"\d+(?:\s*,\s*\d+)*\s*,?", body):
        sys.exit("wantIcoSizes must contain only positive integer literals")
    sizes = [int(value.strip()) for value in body.rstrip(",").split(",")]
    if not sizes or any(size <= 0 for size in sizes) or len(sizes) != len(set(sizes)):
        sys.exit("wantIcoSizes must declare a non-empty unique size inventory")
    return sizes


def _trailing_only():
    """The premultiply defect confined to the right half of every row."""
    derive()
    img = load(PNG)
    px = img.load()
    for y in range(CANVAS_SIZE):
        for x in range(CANVAS_SIZE // 2, CANVAS_SIZE):
            r, g, b, a = px[x, y]
            if 0 < a < 255:
                px[x, y] = (r * a // 255, g * a // 255, b * a // 255, a)
    img.save(REPO / PNG)


def asset_cases():
    sizes = declared_ico_sizes()
    return [
    # +1 is the boundary the acceptance criterion actually states, so it is an
    # asserted case rather than only an optional --measure probe.
    ("master brightened +1/255, .ico stale", lambda: brighten(1), "MasterMatchesIcoFreshness"),
    ("master brightened +2/255, .ico stale", lambda: brighten(2), "MasterMatchesIcoFreshness"),
    ("master hue-swapped, .ico stale", lambda: _hue(), "MasterMatchesIcoFreshness"),
    # Generated from wantIcoSizes rather than a hand-picked subset: the
    # criterion says "any single entry", and listing three of six sizes left
    # half of them unexercised by the script the spec calls canonical.
    *[("%dx%d entry a flat coloured square" % (n, n), (lambda n=n: flat_entry(n)),
       "MasterMatchesIcoFreshness") for n in sizes],
    ("16x16 entry opaque white", lambda: flat_entry(16, (255, 255, 255, 255)),
     "EveryIcoEntryCarriesTheArtwork"),
    ("re-derived with ERODE_PX = 0", lambda: derive(erode=0), "NoOpaqueBackdrop"),
    ("master alpha 0, colour intact", lambda: _alpha0(), "CarriesRealColour"),
    ("premultiply defect, full", lambda: derive(premultiply=1.0), "MasterAppliesAlphaOnce"),
    ("premultiply defect, 20% (ratio bound only)", lambda: derive(premultiply=0.20),
     "MasterAppliesAlphaOnce"),
    # Loop 2's hand-written table ran this and loop 3's derived table dropped it,
    # which is a regression in EXECUTED coverage for the exact machinery loop 2
    # added: without it, a trailing-edge regression would be caught only by the
    # run-count floor, and no case would demonstrate that.
    ("premultiply defect, trailing edge only", _trailing_only, "MasterAppliesAlphaOnce"),
    ("plaque stretched to fill canvas", lambda: derive(stretch=True), "MasterGeometry"),
    ("plaque narrowed 240px (columns)", narrow_master, "MasterGeometry"),
    ("master fully blank", blank_master, "MasterGeometry"),
    ("master is the Wails scaffold placeholder", scaffold_master, "MasterGeometry"),
    (".ico reverted to the Wails scaffold", scaffold_ico, "IcoCarriesTheFullWailsSizeSet"),
    (".ico 48x48 entry dropped", drop_entry, "IcoCarriesTheFullWailsSizeSet"),
    (".ico gains an extra 24x24 entry", extra_entry, "IcoCarriesTheFullWailsSizeSet"),
    ("source render deleted", lambda: os.remove(REPO / SRC), "SourceRenderIsThePinnedGeometry"),
    ("source render replaced, same size", lambda: _swap_source(), "SourceRenderIsThePinnedGeometry"),
    # Named by the criteria but absent from the table until review loop 4.
    ("backdrop opaque except the four corners", _opaque_backdrop, "NoOpaqueBackdrop"),
    ("master saved without an alpha channel", _rgb_master, "MasterGeometry"),
    ("16x16 entry, one opaque pixel in its centre", lambda: _one_pixel(16),
     "EveryIcoEntryCarriesTheArtwork"),
    ("master shifted down 5px while both bands remain in range", _off_centre_master,
     "MasterGeometry", "transparent bands are"),
    ("64x64 ICO directory entry declares a 64x63 rectangle", _non_square_entry,
     "MasterMatchesIcoFreshness", "non-square entry has no square size"),
    ("16x16 measured distance inflated above twice the real distance", _inflate_calibration,
     "MasterMatchesIcoFreshness", "less than half"),
    ("derivation script pins a digest different from the Go test", _diverge_source_digest,
     "SourceRenderIsThePinnedGeometry", "does not pin"),
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


def _exe_tests():
    """Failing and skipped TestExeResources* names."""
    proc = subprocess.run(
        ["go", "test", "-count=1", "-run", "^TestExeResources", "-v", "."],
        capture_output=True, text=True, env=ENV, cwd=REPO,
    )
    output = proc.stdout + proc.stderr
    results, blocks = validate_test_run(proc.returncode, proc.stdout, output, "TestExeResources")
    fail = {name.removeprefix("TestExeResources") for name, status in results.items() if status == "FAIL"}
    skip = {name.removeprefix("TestExeResources") for name, status in results.items() if status == "SKIP"}
    return fail, skip, blocks, output


def inventory():
    return {
        "sizes": declared_ico_sizes(),
        "asset_cases": [case[0] for case in asset_cases()],
        "exe_cases": [spec[0] for spec in EXE_CASE_SPECS] + ["exe absent"],
    }


def self_test():
    false_green = "=== RUN   TestAppIconGuard\n--- PASS: TestAppIconGuard (0.00s)\n"
    try:
        validate_test_run(1, false_green, false_green + "late command failure", "TestAppIcon")
    except TestRunError:
        pass
    else:
        raise AssertionError("nonzero go test with no failed test was accepted")
    scoped = ("=== RUN   TestAppIconWanted\n    wanted diagnostic\n--- FAIL: TestAppIconWanted (0.00s)\n"
              "=== RUN   TestAppIconOther\n    other diagnostic\n--- FAIL: TestAppIconOther (0.00s)\n")
    _, blocks = validate_test_run(1, scoped, scoped, "TestAppIcon")
    if "other diagnostic" in blocks["TestAppIconWanted"]:
        raise AssertionError("diagnostic leaked across test blocks")


def run_exe_table(log_dir=None):
    """Story 6.2: does the built-exe assertion actually bite?

    Mutates committed inputs for stale-resource cases and structurally patches
    a snapshotted PE for the missing-RT_ICON case. Every mutation is restored
    before the next case.
    """
    if not os.path.exists(REPO / EXE):
        sys.exit("%s is absent -- run `wails build` first; this table is about the built product" % EXE)

    dirty = subprocess.run(["git", "status", "--porcelain", "--", *EXE_ASSETS],
                           capture_output=True, text=True, cwd=REPO).stdout.strip()
    if dirty:
        sys.exit("commit or restore these first:\n" + dirty)
    baseline = {p: sha(p) for p in EXE_ASSETS}
    baseline_bytes = {p: (REPO / p).read_bytes() for p in EXE_ASSETS}
    descriptor, exe_name = tempfile.mkstemp(prefix="fairdrop-exe-", suffix=".exe")
    os.close(descriptor)
    exe_baseline = Path(exe_name)
    shutil.copy2(REPO / EXE, exe_baseline)
    exe_digest = hashlib.sha256(exe_baseline.read_bytes()).hexdigest()
    cleanup_state = {"active": True}
    command_output = []

    def run_checked(command):
        proc = subprocess.run(command, cwd=REPO, env=ENV, capture_output=True, text=True)
        transcript = "$ %s\n%s%s" % (" ".join(command), proc.stdout, proc.stderr)
        command_output.append(transcript)
        if proc.returncode:
            raise RuntimeError("mutation command failed with exit %d:\n%s" %
                               (proc.returncode, transcript))

    def restore_exe():
        for p in EXE_ASSETS:
            (REPO / p).write_bytes(baseline_bytes[p])
        shutil.copy2(exe_baseline, REPO / EXE)
        for p in EXE_ASSETS:
            if sha(p) != baseline[p]:
                sys.exit("RESTORE FAILED for " + p)
        if sha(EXE) != exe_digest:
            sys.exit("RESTORE FAILED for " + EXE)

    def cleanup_exe():
        if not cleanup_state["active"]:
            return
        try:
            restore_exe()
        finally:
            exe_baseline.unlink(missing_ok=True)

    atexit.register(cleanup_exe)

    def stale_ico():
        run_checked(["go", "run", "./scripts/verify-exe-mutations.go", "hue-ico", ICO])

    def bump_version():
        import json
        data = json.loads((REPO / WAILS_JSON).read_text(encoding="utf-8"))
        data["info"]["productVersion"] = "9.9.9"
        (REPO / WAILS_JSON).write_text(json.dumps(data, indent=2) + "\n", encoding="utf-8")

    def strip_embedded_icon():
        run_checked(["go", "run", "./scripts/verify-exe-mutations.go", "strip-icon", EXE])

    def neutral_version_table():
        path = REPO / WINDOWS_INFO
        source = path.read_text(encoding="utf-8")
        mutated, count = re.subn(r'"0409"\s*:', '"0000":', source)
        if count != 1:
            sys.exit("could not uniquely replace the 040904b0 version table in %s" % WINDOWS_INFO)
        path.write_text(mutated, encoding="utf-8")
        run_checked(["wails", "build"])

    mutations = {
        "stale_ico": stale_ico,
        "bump_version": bump_version,
        "strip_embedded_icon": strip_embedded_icon,
        "neutral_version_table": neutral_version_table,
    }
    cases = [(label, mutations[key], want, diagnostic)
             for label, key, want, diagnostic in EXE_CASE_SPECS]

    try:
        fail, skip, blocks, output = _exe_tests()
    except TestRunError as exc:
        if log_dir:
            (log_dir / "exe-000-baseline.log").write_text(exc.output, encoding="utf-8")
        raise
    if log_dir:
        (log_dir / "exe-000-baseline.log").write_text(output, encoding="utf-8")
    if fail or skip:
        sys.exit("baseline is not green (failing %s, skipped %s)" % (sorted(fail), sorted(skip)))
    print("baseline: all TestExeResources* pass against the built exe\n")

    print("| # | mutation | named test | killed |")
    print("|---|---|---|---|")
    bad = 0
    for i, (label, mutate, want, diagnostic) in enumerate(cases, 1):
        command_output.clear()
        try:
            restore_exe()
            mutate()
            fail, _, blocks, output = _exe_tests()
            if log_dir:
                (log_dir / ("exe-%03d.log" % i)).write_text(
                    "\n".join(command_output) + "\n" + output, encoding="utf-8")
        except BaseException as exc:
            if log_dir:
                if isinstance(exc, TestRunError):
                    command_output.append(exc.output)
                (log_dir / ("exe-%03d.log" % i)).write_text(
                    "\n".join(command_output), encoding="utf-8")
            restore_exe()
            raise
        named = want in fail and diagnostic in blocks.get("TestExeResources" + want, "")
        if not named:
            bad += 1
        print("| %d | %s | `%s` + `%s` | %s |" % (i, label, want, diagnostic, "yes" if named else "**NO**"))
    restore_exe()

    # The skip must be a skip: absent build must never pass silently.
    probe = Path(str(REPO / EXE) + ".probe")
    if probe.exists():
        sys.exit("refusing absent-exe probe because stale path exists: %s" % probe)
    os.rename(REPO / EXE, probe)
    try:
        fail, skip, blocks, output = _exe_tests()
        if log_dir:
            (log_dir / ("exe-%03d.log" % (len(cases) + 1))).write_text(output, encoding="utf-8")
    finally:
        os.rename(probe, REPO / EXE)
    expected_skips = {"EmbedTheCommittedIcon", "CarryTheCommittedIdentity"}
    ok = not fail and skip == expected_skips and all(
        "is absent, so there is no built artifact to inspect" in
        blocks.get("TestExeResources" + name, "") for name in expected_skips)
    if not ok:
        bad += 1
    print("| %d | exe absent | both tests | %s |"
          % (len(cases) + 1, "skipped, not passed" if ok else "**NO -- %s / %s**" % (sorted(fail), sorted(skip))))

    print("\n%d of %d exe mutations behaved as required." % (len(cases) + 1 - bad, len(cases) + 1))
    cleanup_exe()
    cleanup_state["active"] = False
    return 1 if bad else 0


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--measure", action="store_true",
                        help="also probe the freshness boundary size by size")
    parser.add_argument("--exe", action="store_true",
                        help="run Story 6.2's built-exe table instead (needs `wails build` first)")
    parser.add_argument("--log-dir", type=Path,
                        help="retain the complete unique go-test output for every case")
    parser.add_argument("--inventory", action="store_true", help="print canonical inventory JSON")
    parser.add_argument("--self-test", action="store_true", help="exercise parser false-green guards")
    args = parser.parse_args()

    if args.inventory:
        import json
        print(json.dumps(inventory(), sort_keys=True))
        return
    if args.self_test:
        self_test()
        return

    def terminate(_signum, _frame):
        raise KeyboardInterrupt("termination requested; restore from the disposable worktree if setup was incomplete")

    signal.signal(signal.SIGTERM, terminate)

    if ENV.get("GITHUB_ACTIONS") != "true" and ENV.get("FAIRDROP_MUTATION_DISPOSABLE") != "1":
        sys.exit("mutation runs require an isolated disposable checkout; set "
                 "FAIRDROP_MUTATION_DISPOSABLE=1 only inside one")

    log_dir = args.log_dir.resolve() if args.log_dir else None
    if log_dir:
        if log_dir.exists() and any(log_dir.iterdir()):
            sys.exit("log directory is not empty; use a fresh unique path: %s" % log_dir)
        log_dir.mkdir(parents=True, exist_ok=True)

    if args.exe:
        sys.exit(run_exe_table(log_dir))

    # Pillow belongs only to the source-asset mutation suite. The native
    # Windows --exe proof is intentionally standard-library-only in CI.
    global Image, ImageDraw, ImageFilter
    from PIL import Image, ImageDraw, ImageFilter

    dirty = subprocess.run(["git", "status", "--porcelain", "--", *ASSETS],
                           capture_output=True, text=True, cwd=REPO).stdout.strip()
    if dirty:
        sys.exit("these assets are modified; commit or restore them first, or a mutated file "
                 "becomes the baseline every case is scored against:\n" + dirty)
    for path in ASSETS:
        BASELINE[path] = sha(path)
        BASELINE_BYTES[path] = (REPO / path).read_bytes()

    try:
        failing, blocks, output = run_tests()
    except TestRunError as exc:
        if log_dir:
            (log_dir / "asset-000-baseline.log").write_text(exc.output, encoding="utf-8")
        raise
    if log_dir:
        (log_dir / "asset-000-baseline.log").write_text(output, encoding="utf-8")
    if failing:
        sys.exit("baseline is not green (%s) -- fix that before trusting any mutation" % sorted(failing))
    print("baseline: all TestAppIcon* pass\n")

    print("| # | mutation | named test | killed |")
    print("|---|---|---|---|")
    bad = 0
    cases = asset_cases()
    for i, case in enumerate(cases, 1):
        label, mutate, want = case[:3]
        diagnostic = case[3] if len(case) == 4 else None
        try:
            restore()
            mutate()
            if all(sha(p) == BASELINE[p] for p in ASSETS if os.path.exists(REPO / p)) and \
                    all(os.path.exists(REPO / p) for p in ASSETS):
                sys.exit("case %d (%s) changed nothing -- it would be scored without having run" % (i, label))
            failing, blocks, output = run_tests()
            if log_dir:
                (log_dir / ("asset-%03d.log" % i)).write_text(output, encoding="utf-8")
        except BaseException as exc:
            # Never leave a mutated binary asset in the tree, including on
            # KeyboardInterrupt.
            if log_dir and isinstance(exc, TestRunError):
                (log_dir / ("asset-%03d.log" % i)).write_text(exc.output, encoding="utf-8")
            restore()
            raise
        named = want in failing and (diagnostic is None or diagnostic in blocks.get("TestAppIcon" + want, ""))
        if not named:
            bad += 1
        target = "`%s`" % want if diagnostic is None else "`%s` + `%s`" % (want, diagnostic)
        print("| %d | %s | %s | %s |" % (i, label, target, "yes" if named else "**NO**"))
    restore()
    print("\n%d of %d mutations failed their named test." % (len(cases) - bad, len(cases)))

    if args.measure:
        print("\nfreshness boundary, master brightened by +N with the .ico left stale:")
        for delta in (0, 1, 2, 3, 4):
            restore()
            if delta:
                brighten(delta)
            failing, _, _ = run_tests()
            caught = "MasterMatchesIcoFreshness" in failing
            print("  +%-3d %s" % (delta, "CAUGHT" if caught else "missed"))
            # +0 is the genuine asset and must pass; every step above it must be
            # caught. A regression here is a real finding, not a printout.
            if (delta == 0) == caught:
                bad += 1
                print("       ^ unexpected: the boundary the spec states has moved")
        restore()

    sys.exit(1 if bad else 0)


if __name__ == "__main__":
    main()
