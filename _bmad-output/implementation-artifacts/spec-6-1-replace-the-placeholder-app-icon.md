---
title: 'Story 6.1: Replace the Placeholder App Icon'
type: 'feature'
created: '2026-09-19'
status: 'in-review'
baseline_commit: 'de16807eedb8482aac82e9862161fb89aabfb7b2'
review_loop_iteration: 1
context: []
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** `build/appicon.png` and `build/windows/icon.ico` are still byte-identical to the
Wails 2.15.0 scaffold defaults — v1.0.0 shipped a stock white "W" in the Windows taskbar, the
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
  `installer/project.nsi:53-54` (`MUI_ICON` = `..\icon.ico`) -- read-only; both pick up the
  regenerated files automatically. Nothing embeds the icon; it is purely a build asset.
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

**Acceptance Criteria:**
Each criterion names the mutation that must fail it. "Fails by name" means the named test, not
any test.

- **Freshness.** Given both shipped assets, when the master is downsampled to a shared entry
  size, then its mean per-channel RGBA distance from the matching `.ico` entry is within a
  tolerance absorbing winicon's Catmull-Rom resampling but not different artwork. *Mutation:*
  replace the master with hue-swapped artwork, leave the `.ico` **-> must fail.**
- **No opaque backdrop.** Given either asset, when the outer 12px border ring is measured, then
  it holds no opaque pixel whose channels all exceed 200, and the ring is overwhelmingly
  transparent. Four corner pixels are **not** sufficient: on a centred rounded rect they are
  transparent by construction. *Mutations:* re-derive with `ERODE_PX = 0`; make the backdrop
  opaque everywhere except the four corners **-> both must fail.**
- **Real colour.** Given either asset, when mean HSV saturation over the central 40% is measured
  **skipping fully transparent pixels**, then it exceeds 0.25, and the test fails if no opaque
  pixel remains there. *Mutation:* set the master's alpha to 0 everywhere, keeping colour
  **-> must fail** (an invisible icon is not real colour).
- **Alpha applied once.** Given the master, when pixels at partial alpha are compared with the
  opaque edge colour, then their RGB matches within tolerance. *Mutation:* composite the plaque
  onto the canvas using itself as the mask **-> must fail.** This is a real defect found in
  review, not a hypothetical: it darkens the feather and halves its width.
- **Master geometry.** Given the master, then it is exactly 1024x1024 RGBA carrying a fully
  transparent band top and bottom. *Mutation:* stretch the plaque square to fill **-> must fail.**
- **Small sizes.** Given the `.ico`, then its 16x16 and 32x32 entries decode and satisfy the
  colour assertion -- the sizes the artwork was chosen for. *Mutation:* replace a small entry
  with opaque white **-> must fail.**
- **Toolchain provenance.** Given the `.ico`, then it carries the full 256/128/64/48/32/16 set
  that **Wails** passes to `winicon.GenerateIcon` at `packager.go:217` -- winicon emits whatever
  list it is handed, so this pins a Wails literal, not a winicon behaviour.
- Given the full gate on all three platforms, when it runs, then it passes.

## Spec Change Log

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
