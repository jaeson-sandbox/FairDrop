---
title: 'Story 6.1: Replace the Placeholder App Icon'
type: 'feature'
created: '2026-09-19'
status: 'done'
baseline_commit: 'de16807eedb8482aac82e9862161fb89aabfb7b2'
review_loop_iteration: 3
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `build/appicon.png` and `build/windows/icon.ico` are still byte-identical to the
Wails 2.15.0 scaffold defaults — v1.0.0 shipped the stock placeholder, a white field
carrying a dark "W", in the Windows taskbar, the
exe's Properties pane, the NSIS installer and the macOS Dock. The owner supplied three candidate
renders. **None carries an alpha channel** — all are JPEGs, including the one whose backdrop is
drawn as a transparency checkerboard, which is opaque pixels like any other. A format conversion
would ship that backdrop.

**Approach:** Derive one 1024×1024 RGBA master from `fairdrop_logo2_revised.jpg` — the two-tone
render, a light tan mark on a dark brown field — by cropping the plaque and synthesising a
rounded-rect alpha. Commit the source and the derivation beside it, and force the Windows
`.ico` to regenerate, which it otherwise never does.

**Why logo 2 over logo 1.** Measured, not preferred: logo 2 carries 61–90% more edge energy
(mean |∇luminance| per opaque pixel) at every taskbar size — 60.3 vs 37.5 at 16px. Logo 1's
relief is copper-on-copper and resolves to a featureless blob below ~48px; logo 2's mark and
field differ in luminance, so the glyph survives. Hence no small-size variant is needed.

## Boundaries & Constraints

**Always:** `build/appicon.png` is the single art source; every other icon is derived from it by
`wails build`. Commit the original JPEG and the derivation so the master is reproducible rather
than a one-off nobody can regenerate. Both shipped assets carry real transparency.

**Ask First:** Any change to the artwork itself — recolouring, restyling, or a separate
simplified variant for small sizes. Logo 2 removes the need that would have motivated one.

**Never:** Redraw or restyle the owner's artwork. Add an image pipeline to `verify.yml` — the
derivation runs by hand; the committed assets are what CI checks. Touch the frontend, the window,
or any transfer behaviour. Introduce a system tray; this app has none.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| macOS package build | `build/appicon.png` present | `processDarwinIcon` re-encodes `iconfile.icns` on **every** build | decode failure surfaces from `wails build` |
| Windows build, no `.ico` | `build/windows/icon.ico` absent | Wails generates it from the master at 256/128/64/**48**/32/16 | as above |
| Windows build, stale `.ico` | `build/windows/icon.ico` present | Wails **skips** generation entirely; the stale icon ships | `appicon_test.go` fails, naming Windows |
| Naive JPEG→PNG conversion | corners hold the baked checkerboard | grey checker ships behind the icon | corner-alpha assertion fails |
| Revert to scaffold default | white field, black "W" | placeholder ships again | centre-saturation assertion fails |

</frozen-after-approval>

## Code Map

- `build/appicon.png` -- the master. Currently byte-identical to
  `wails/v2@v2.15.0/pkg/buildassets/build/appicon.png`.
- `build/windows/icon.ico` -- byte-identical to the Wails default; entries 256/128/64/**24**/32/16,
  not the full size set Wails asks `winicon.GenerateIcon` for, proving it came from the scaffold and
  never from a build.
- `wails/v2@v2.15.0/pkg/commands/build/packager.go:204` -- `generateIcoFile` wraps generation in
  `if !fs.FileExists(icoFile)`. This is the whole reason the Windows icon never updated.
- `.../packager.go:134` -- `processDarwinIcon`'s definition, unconditional, so macOS needs no
  equivalent step; called from `.../packager.go:99`.
- `.../packager.go:217` -- `winicon.GenerateIcon(..., []int{256, 128, 64, 48, 32, 16})`: the size set
  is a **Wails** literal passed in at the call site, not a winicon behaviour --
  `leaanthony/winicon@v1.0.0/generate.go` emits whatever list it is handed. A maintainer chasing a
  size-set change after a Wails upgrade should start here, not in winicon.
- `leaanthony/winicon@v1.0.0/generate.go` -- emits **PNG-compressed** ICO entries, so stdlib
  `image/png` decodes them after parsing the 6-byte ICONDIR + 16-byte entries. No new dependency.
- `release_identity_test.go` -- precedent for a root `package main` test reading shipped files as
  data. The new test is a sibling: image decoding is different mechanics from text pinning.
- `build/darwin/Info.plist:18` + `Info.dev.plist:18` (`CFBundleIconFile` = `iconfile`) and
  `installer/project.nsi:53` (`MUI_ICON`) and `:54` (`MUI_UNICON`, the **uninstaller** icon --
  a second user-visible surface this change updates) both = `..\icon.ico` -- read-only; all
  pick up the regenerated files automatically. Nothing embeds the icon; it is a build asset.
- `scripts/verify-native-mutations.sh` -- mutates `.go` sources only, so asset mutations are
  recorded manually in the evidence file instead.

**Measured source geometry**, fitted by least squares over 60 rows at **rms 1.74px**: crop
`(896, 239, 1920, 1232)`, corner radius 226. Two results the implementer must not "fix":

- **The plaque is 1024×993, not square**, with circular corners in source space (a 2.9%
  foreshortening would have shown ~6px error, not 1.7). It is therefore **centred** in the square
  canvas with 15px transparent bands, not stretched — stretching would restyle the artwork.
- **The fitted top edge, 237, sits inside the background's bloom**, leaving 133 near-white border
  pixels. 239 with a 9px erosion drops that to 8 and edge-ring p95 from 203 to 85.

## Tasks & Acceptance

**Execution:**
- [x] `build/appicon-source.jpg` -- add `fairdrop_logo2_revised.jpg` under this name -- provenance,
  so the master can be re-derived at higher fidelity later.
- [x] `scripts/build-appicon.py` -- add the derivation: crop `(896, 239, 1920, 1232)`, synthesise a
  rounded-rect alpha at radius 226, erode 9px and feather 1.2px so no bloom survives the JPEG's
  soft edge, then centre in a 1024×1024 transparent canvas -- so the transform is reviewable
  rather than a black box. Constants carry the reasoning above as comments.
- [x] `build/appicon.png` -- replace the scaffold default with the script's output.
- [x] `build/windows/icon.ico` -- delete, regenerate by running `wails build`, commit the result
  -- the only way past the `FileExists` short-circuit.
- [x] `appicon_test.go` -- new root test pinning the matrix's last three rows against both
  shipped assets.
- [x] `build/README.md` -- correct the `icon.ico` wording: regeneration on absence is the *only*
  trigger, not a fallback.
- [x] `_bmad-output/planning-artifacts/epics.md` -- add Epic 6 and Story 6.1, following Epic 5's
  single-story shape.
- [x] `_bmad-output/implementation-artifacts/sprint-status.yaml` -- register `epic-6`,
  `6-1-replace-the-placeholder-app-icon`, and `epic-6-retrospective`.
- [x] `.gitattributes` -- add `*.jpg` and `*.jpeg` to the binary list, which a 2MB committed
  JPEG otherwise sits outside.
- [x] `.gitignore` -- ignore the owner's candidate renders by shape (`**/fairdrop_logo*.jp*g`),
  since only the chosen one is committed, under `build/appicon-source.jpg`.
- [x] `_bmad-output/implementation-artifacts/deferred-work.md` -- D-128 (withdrawn), D-129 and
  D-130.
- [x] `_bmad-output/implementation-artifacts/evidence-6-1-replace-the-placeholder-app-icon.md`
  -- the mutation table, per the house rule that evidence never lives in the spec.

**Acceptance Criteria:**

Each criterion names the mutation that must fail it. The freshness bound is a factor applied to
a per-entry measured value; the remaining constants are absolutes, each bracketed by a measured
mutation on its far side and logged on every run so drift is visible. An absolute with a probed
boundary is honest; an absolute chosen for headroom is what loops 2 and 3 shipped -- loops 2 and 3
both shipped an absolute that had never had its boundary measured. The mutation table is derived by
`scripts/verify-asset-mutations.py`, not written by hand.

- **Freshness, every entry.** Given the master and **each** `.ico` entry, when the master is
  downsampled to that entry's size, then their mean per-channel distance is at most 1.5x the
  distance measured for that size when they genuinely match (0.616 at 256 through 4.109 at 16).
  *Mutations:* a hue-swapped master; a uniform **+1/255** brightening with the `.ico` left stale;
  any single entry replaced by a flat coloured square **-> all must fail.** Measured boundary: +0
  passes, +1 fails. Loop 3 found five of six entries could be flat squares with the suite green.
- **No opaque backdrop.** Given the master and every `.ico` entry, when the border ring is measured
  at a width scaled to that asset's resolution, then it holds no opaque near-white pixel; the
  ring's transparent *fraction* is additionally checked where the ring is wide relative to the
  corner radius (the master and the 256 entry), because at 16px a 1px ring is legitimately 93%
  plaque. A coloured backdrop is caught by Freshness, not here. *Mutations:* re-derive with
  `ERODE_PX = 0`; backdrop opaque except the four corners; a 16x16 entry of opaque white.
- **Real colour.** Given the master and every `.ico` entry, when mean HSV saturation over the
  central 40% is measured skipping transparent pixels, then it exceeds 0.25 and at least half that
  region is opaque. *Mutation:* master alpha 0 with colour intact **-> must fail.**
- **Alpha applied once.** Given the master, when partial-alpha pixels are compared with their local
  opaque reference **on both the leading and trailing edge of every row**, then mean low-alpha
  deviation is at most 8.0 **and** at most 2.0x the high-alpha deviation. Genuine: 3.172 / 2.621 =
  1.21. *Mutations:* a full premultiply (27.96, trips the absolute bound); and a **20%** one
  (6.946 / 3.001 = 2.31) which stays under the absolute bound and is caught only by the ratio
  **-> both must fail.** Without the second, the ratio constant is decoration.
- **Master geometry.** Given the master, then it is exactly 1024x1024, its PNG IHDR colour type
  carries an alpha channel, its transparent bands are **12-40 rows** top and bottom (the genuine
  master measures 21/22: the 15/16px centring band plus the erosion's own margin) and **1-12
  columns** left and right (genuine 6/6, since the plaque spans the full width). *Mutations:*
  stretch to fill; narrow the plaque 240px; a blank canvas; the scaffold placeholder; a master
  saved without alpha **-> all must fail.** A floor with no ceiling passed the first three.
- **Toolchain provenance.** Given the `.ico`, then its entry set **equals** {256,128,64,48,32,16},
  the literal Wails passes to `winicon.GenerateIcon` at `packager.go:217`. *Mutations:* drop the
  48; add a scaffold 24 **-> both must fail.**
- **Source provenance.** Given `build/appicon-source.jpg`, then it decodes fully (not just its
  header), is 2816x1536, and its sha256 equals the digest `scripts/build-appicon.py` pins.
  *Mutations:* delete it; replace it with a different 2816x1536 render **-> both must fail.** Loop
  3 found the whole suite passed with the file deleted, and again with it swapped.
- Given the full gate on all three platforms, when it runs, then it passes.

## Spec Change Log

- **2026-09-19 — review loop 3: the loop-2 criteria were under-specified the same way loop 1's
  were, and this is now a pattern rather than an incident.** Three findings, each measured before
  being believed. (1) Only the 256 `.ico` entry was ever compared against the master, so replacing
  any of the 16, 32, 48, 64 or 128 entries with a flat coloured square passed the entire suite —
  the sizes Windows actually draws. (2) Loop 2 replaced an absolute freshness tolerance of 18.0
  with an absolute 4.96 and reported the stale-icon gap closed; measuring the new boundary showed
  +6/255 (a 2.4% tonal revision) still passed. (3) `MasterGeometry` checked rows but never columns,
  so a plaque narrowed by 240px passed the test named for geometry. Root cause, again, is mine and
  outside the frozen block: a constant picked for headroom and confirmed by one mutation on its far
  side, rather than derived from a measurement with its boundary probed. Every tolerance is now a
  factor times a measured value, every measured value is logged each run, and the mutation table is
  derived by `scripts/verify-asset-mutations.py` — loop 2's hand-written table carried three
  stale numbers into a merged evidence file. **KEEP:** the per-entry freshness shape, which
  subsumes the coloured-backdrop case the ring check admits it cannot see; the measured crop
  geometry; and the refusal to assert Pillow byte-identity.

- **2026-09-19 — review loop 1: the acceptance criteria under-specified every assertion.**
  Triggered by all three review layers independently, and confirmed by mutation rather than
  argument: an invisible master, an opaque backdrop sparing only the four sampled corners, an
  opaque-white small `.ico` entry, a hue-swapped master against a stale `.ico`, and a
  re-derivation with `ERODE_PX = 0` leaving a bloom halo (1,327 opaque pixels with every channel
  above 200 in the outer 12px ring, against 0 for the committed asset; the review layer that
  raised it reported 1,276, measured before the alpha fix below, which darkened RGB and so
  cleared the threshold less often) — **all five passed the original criteria.** Root cause is
  mine and sits outside the frozen block: the criteria named the *sampling method* ("four
  corner pixels", "central 40%") as though it were the *property* ("no opaque backdrop
  survived", "this is real artwork"). Amended to state each property, the region that
  establishes it, and the mutation that must fail it. Also corrected: the claim that a 48x48
  entry proves generation "from the master" — it proves only that some winicon-produced `.ico`
  is present — and the attribution of the size set to winicon rather than to Wails. **KEEP:**
  the logo-2 artwork choice and its edge-energy justification; the measured crop geometry
  `(896, 239, 1920, 1232)` at radius 226; the analytic inset-plus-radius-reduction erosion; and
  the refusal to assert Pillow-byte-identity, which remains correct.

## Design Notes

**Candidate selection.** Three renders were supplied and all three measured. Logo 1 is
copper-on-copper and loses its glyph below ~48px. Logo 2 and its revision are the same artwork
and score the same edge energy (60.3 vs 60.4 at 16px); the revision wins purely on extractability
-- its plaque is dark-on-light rather than brown-on-brown, so the silhouette is measurable to
rms 1.74px instead of eyeballed, and the crop lands on an exact 1024px width. The checkerboard in
the revision is decoration, not alpha: every candidate is an opaque JPEG.

**Two metrics rejected before edge energy,** recorded so they are not re-proposed. *Luminance
standard deviation* ranks logo 1 above logo 2 at 16px (28.1 vs 26.0) -- it is blind to spatial
arrangement, scoring a smooth relief gradient the same as a sharp glyph boundary. *p10-p90
luminance spread* was drafted as an acceptance criterion and would have been vacuous: the
placeholder scores 232, above either real logo, because it is black on white, so the test would
have passed on the exact thing it claimed to detect. Centre saturation discriminates; it does not.

**Reproducibility is honest rather than absolute:** the script re-derives the master from the
committed JPEG, but byte-identity across Pillow versions is not asserted and is not a test --
that would pin the runner's Pillow build rather than this repo's code.

## Verification

**Commands:**
- `go test -count=1 -run TestAppIcon -v .` -- expected: the new assertions pass by name.
- `wails build` -- expected: regenerates `build/windows/icon.ico` (absent) and rebuilds the exe.
- `gofmt -l . && go vet ./... && go tool staticcheck ./...` -- expected: clean.
- `go test -count=1 ./... && go test -count=1 -race ./...` -- expected: no regression.
- `cd frontend && npm test && npm run test:browser && npm run build` -- expected: unaffected.

**Manual checks (if no CLI):**
- Launch `build/bin/fairdrop.exe` and confirm the Windows taskbar button and the exe's
  Properties → Details icon both show the copper mark, not the "W".
