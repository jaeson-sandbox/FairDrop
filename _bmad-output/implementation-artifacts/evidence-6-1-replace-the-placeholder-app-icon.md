# Evidence: Story 6.1: Replace the Placeholder App Icon

Baseline: `de16807eedb8482aac82e9862161fb89aabfb7b2`. This file covers review loop 1: the P1-P11 patch
list three review layers produced against the story's first pass, and the re-verification that
followed. `scripts/verify-native-mutations.sh` mutates `.go` sources only, so — per the spec's Code
Map — the asset mutations below are recorded here by hand rather than by that harness.

## P1: the shipped master applied alpha twice

`scripts/build-appicon.py` pasted the plaque onto the canvas passing the plaque itself as the paste
*mask*, a second time, on top of alpha the plaque already carried correctly:

```python
canvas.paste(plaque, (offset_x, offset_y), plaque)  # buggy: plaque used as its own mask
```

PIL treats an RGBA mask's alpha band as a blend weight applied to **all four** destination channels,
not just RGB. Reproduced directly: `Image.new('RGBA', (4,1), (255,0,0,128))` pasted this way onto a
transparent canvas yields `(128,0,0,64)` — both RGB and alpha scaled by `128/255`, i.e. output alpha
becomes `alpha²/255` and output RGB is premultiplied by `alpha/255` a second time, on top of the
already-correct straight alpha the plaque carried. In the committed master this measured as a dark
fringe: row 512, x=9 read `(43,26,20,109)` where the opaque edge colour at x=12 was `(63,35,31,255)`
— straight (non-premultiplied) RGB must not vary with alpha, and it did.

**Fix:** a maskless paste (`canvas.paste(plaque, (offset_x, offset_y))`) — the destination region is
fully transparent before the paste, so this is a direct copy of the plaque's own already-correct
pixels, not a second composite. Re-verified at the same coordinates after the fix: x=9 now reads
`(66,39,30,167)` against the opaque edge's `(63,35,31,255)` at x=12 — RGB close regardless of alpha,
as it should be. `build/appicon.png` was regenerated and `build/windows/icon.ico` was deleted and
regenerated via `wails build` (the `FileExists` short-circuit is otherwise the only reason it would
not pick up the fix).

## P2-P4: three smaller defects in the same script

- **P2 (off-by-one erosion).** PIL's `rounded_rectangle` treats its box as inclusive of both
  endpoints, so `(inset, inset, big-inset, big-inset)` drew one supersampled pixel past the intended
  right/bottom edge — an asymmetric erosion, contradicting the "exact Minkowski erosion" comment.
  Fixed to `(inset, inset, big[0]-inset-1, big[1]-inset-1)`.
- **P3 (cwd-relative paths).** `SOURCE`/`OUTPUT` were plain relative strings, so the script only
  worked run from the repository root. Now resolved from `Path(__file__).resolve().parents[1]`.
- **P4 (unguarded inputs).** The script now asserts the source JPEG is exactly 2816x1536 (the size
  `CROP_BOX`/`CORNER_RADIUS` were fitted against — PIL crops out-of-range silently rather than
  erroring) and fails if the cropped plaque would not fit inside `CANVAS_SIZE` (PIL's `paste` would
  otherwise truncate it silently). The offset comment now states the top/bottom transparent band is
  exactly 15px / 16px for this crop, not "~15-16px". The PNG is saved with `optimize=True`
  (1,458,849 bytes unoptimized vs 1,400,496 optimized for the corrected master — measured directly,
  not the byte count from before the P1 fix, which changed the pixel data too).

## Freshness and geometry numbers `appicon_test.go` is calibrated against

Measured directly against this story's own final shipped assets (`build/appicon.png` box-downsampled
to 256x256 vs `build/windows/icon.ico`'s 256x256 entry, and the master's own pixels):

| Measurement | Genuine value | Notes |
|---|---|---|
| Freshness: mean per-channel distance, master vs `.ico` (0-255 scale) | 0.62 | tolerance set at 18.0 |
| Alpha-applied-once: low-alpha (< 128) mean deviation from local opaque reference | 3.19 (3,379 samples, 969 runs) | tolerance set at 8.0 |
| Alpha-applied-once: high-alpha (>= 128) mean deviation | 2.47 (3,547 samples) | for comparison only |
| Master: contiguous fully-transparent rows, top edge | 21 | minimum set at 12 |
| Master: contiguous fully-transparent rows, bottom edge | 22 | minimum set at 12 |
| Master: border-ring (12px) transparent fraction | 84.8% | minimum set at 75% |
| `.ico` 256 entry: border-ring (3px, scaled) transparent fraction | 80.2% | minimum set at 75% |

**Two of these needed a design change, not just a number, and both are recorded here because a
future reviewer will otherwise re-propose the simpler version that already failed:**

- **The "No opaque backdrop" ring cannot use one fixed pixel width across assets of different
  resolution.** A first version applied the master's 12px ring unscaled to the `.ico`'s 256x256 entry
  (a literal 4x downsample) and measured only 45.3% transparent — not a defect: inspecting that
  entry directly at y=128 shows the transition to fully opaque completing by x=3-4, with x=8 onward
  already genuine copper glyph colour `(147,89,63,255)` climbing to `(189,114,77,255)`. The fix scales
  the ring to the asset's own resolution (`ringWidthFor`: `12 * assetSize / 1024`), giving 3px for the
  256 entry.
- **"Alpha applied once" cannot compare one partial-alpha pixel against the first fully-opaque pixel
  found scanning further along its row.** A first version did exactly that and failed on the
  **correct**, already-fixed master at rows near the top-left corner (e.g. row 54): the scan's "first
  opaque pixel" landed on a different nearby feature (this artwork has photographic/JPEG noise of
  roughly 10-15 RGB units even among already-opaque neighbouring pixels — confirmed by inspecting
  row 54, x=105-129 directly), which a single-pixel tolerance could not tell apart from the actual
  defect. The fix pools every short (<= 20px), cleanly bounded transition run in the whole image —
  one row crosses the plaque's outer boundary at most once, and 20px was chosen from the run-length
  distribution itself: straight-edge runs measure 6-10px (median 6, p90 10), corner-crossing rows
  jump to 23-48px, so 20 sits in the gap — comparing each run's own pixels only to that run's own
  local opaque reference, then buckets by low/high alpha and compares the pooled means. That
  aggregation is what separates 3.19 (correct) from 27.96 (P1's defect reproduced) below, while
  staying near noise level on the correct master.
- **Relatedly, "Master geometry" cannot check only the single outermost row.** A first version
  checked y=0 and y=h-1 alone and it **passed** on the "stretch the plaque square to fill" mutation
  (see M5 below) — the plaque's own 9px erosion leaves a few-pixel transparent margin at its own
  edges regardless of how it is placed on the canvas, so the single outermost row was still
  transparent even with the deliberate 15/16px centring band removed. Counting a contiguous run of
  transparent rows instead (21-22 genuine vs 6 stretched) tells them apart.

## Mutation table

Eight mutations: the seven the amended Acceptance Criteria name, plus one unnamed but easy
cross-check for the "Toolchain provenance" criterion. Each applied alone against the real shipped
files, run through `go test -count=1 -run TestAppIcon -v .`, then reverted (byte-for-byte, sha256
verified) before the next.

| # | Criterion | Mutation | Result |
|---|---|---|---|
| M1 | Freshness | hue-swap the master (R/B channels), leave `.ico` untouched | **KILLED** — `TestAppIconMasterMatchesIcoFreshness`: "differs ... by a mean 26.00 per channel ..., want <= 18.00". Cleanly isolated: the other six tests still passed. |
| M2 | No opaque backdrop | re-derive with `ERODE_PX = 0` | **KILLED** — `TestAppIconNoOpaqueBackdrop`: "2056 near-white pixel(s) ... e.g. (11,160) rgba=(254,254,254,1)" and "only 67.0% ... fully transparent, want at least 75%". Also failed `TestAppIconMasterAppliesAlphaOnce` as a reasonable side effect (the un-eroded mask reaches into the JPEG's own soft bloom, producing a much longer, noisier transition). |
| M3 | No opaque backdrop | make the backdrop opaque everywhere except the four corner pixels | **KILLED** — `TestAppIconNoOpaqueBackdrop`: "9740 near-white pixel(s) ..." and "only 0.0% ... fully transparent". Also failed `TestAppIconMasterGeometry` (no transparent band survives) and `TestAppIconMasterAppliesAlphaOnce` (correctly reports "found only 0 usable transition run(s)" rather than passing vacuously). |
| M4 | Real colour | set the master's alpha to 0 everywhere, keep the RGB colour | **KILLED** — `TestAppIconCarriesRealColour`: "every pixel in the central 40% is fully transparent, so no colour could be measured". Also failed Freshness and AppliesAlphaOnce as expected side effects of erasing the whole image's alpha. |
| M5 | Alpha applied once | composite the plaque onto the canvas using itself as the mask (P1's original defect, regenerated through the real script) | **KILLED**, cleanly isolated — `TestAppIconMasterAppliesAlphaOnce`: "deviate ... by a mean of 27.96 ...; alpha >= 128 pixels deviate by only 3.93 ..., want <= 8.00". All six other tests passed. |
| M6 | Master geometry | stretch the plaque (`Image.resize`) to fill the full 1024x1024 square instead of centring it | **KILLED** — `TestAppIconMasterGeometry`: "only 6 contiguous fully-transparent row(s) from the top edge, want >= 12" (and bottom). Also failed No-opaque-backdrop and AppliesAlphaOnce as reasonable side effects of removing the centring band. This mutation is the one that exposed the single-row-check design flaw recorded above; it passed the first version of this test entirely. |
| M7 | Small sizes | rebuild `icon.ico` with its 16x16 entry replaced by opaque white, all other entries untouched | **KILLED**, cleanly isolated — `TestAppIconSmallIcoEntriesCarryRealColour`: "(16x16 entry): mean central-40% HSV saturation over 49 opaque pixels = 0.000, want > 0.25". All six other tests passed. |
| M8 | Toolchain provenance (no mutation named in the spec; added for coverage) | swap in the Wails 2.15.0 scaffold's own `icon.ico` (entries 256/128/64/24/32/16) | **KILLED** — `TestAppIconIcoCarriesTheFullWailsSizeSet`: "is missing size(s) [48] from the set {[256 128 64 48 32 16]}". Also failed Real-colour, Small-sizes and Freshness, since the scaffold file is wholly different content — expected. |

Every named test's failure message states the property it is checking and the measured number, per
the spec's "fails by name" requirement. Restoration was verified by sha256 after every mutation
(`build/appicon.png` and `build/windows/icon.ico` both matched their pre-mutation hashes before the
next mutation ran).

## Verification

Read stage by stage, all against the final working tree:

- `go test -count=1 -run TestAppIcon -v .` — all 7 tests pass by name.
- `gofmt -l .` — clean.
- `go vet ./...` — clean.
- `go tool staticcheck ./...` — clean.
- `go test -count=1 ./...` — 8 packages ok.
- `go test -count=1 -race ./...` — 8 packages ok (`CGO_ENABLED=1` confirmed before the run).
- `wails build` — with `icon.ico` present, the packager's `FileExists` short-circuit correctly
  skipped regeneration (sha256 of both shipped assets unchanged before/after); the exe's actually
  embedded icon was independently extracted (`System.Drawing.Icon.ExtractAssociatedIcon` against
  `build/bin/fairdrop.exe`) and confirmed to be the copper mark, not the Wails "W", both before and
  after this review loop's fixes.
- `cd frontend && npm test` — 17 files, 547 tests passing.
- `npm run test:browser` — 1 file, 17 tests passing.
- `npm run build` — clean; no binding or drift changes (this story touches no frontend source).

## Nothing else left open

- The Never clause (no image pipeline in `verify.yml`) is unchanged: `scripts/build-appicon.py` still
  runs only by hand.
- [[D-129]] and [[D-130]] (verification-gap findings from this same review loop, about the built exe
  never being read directly and about `verify.yml` running `wails build` before `go test`) are
  recorded in `deferred-work.md`, not fixed here: both require touching `verify.yml`, which this
  story's Never clause forbids, and are deliberately left for a shared follow-up step rather than
  built twice. [[D-128]] (the exe's version-resource strings) was investigated as part of the same
  manual-check pass, found to be a false alarm from a tool that cannot read a language-neutral string
  table, and is recorded, not touched, by this file.
- Byte-identity of `scripts/build-appicon.py`'s output across Pillow versions remains explicitly
  unasserted, per the spec's Design Notes — that would pin the runner's Pillow build, not this
  repo's code.
