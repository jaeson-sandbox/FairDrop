# Evidence: Story 6.1: Replace the Placeholder App Icon

Baseline: `de16807eedb8482aac82e9862161fb89aabfb7b2`. This file covers review loops 1 and 2. Loop 1 is the
P1-P11 patch list three review layers produced against the first pass; the sections below
document P1-P4 in detail because they were code defects, while P5-P11 (test rewrites,
citation fixes, docs, `.gitattributes`/`.gitignore`) are visible in the diff and left
undocumented here deliberately -- an earlier header claimed to cover all eleven and did
not. Loop 2 is the Q1-Q18 list, recorded in its own section at the end. `scripts/verify-native-mutations.sh` mutates `.go` sources only, so — per the spec's Code
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

## Review loop 2

Loop 1 hardened the assertions; loop 2 found that hardening still under-specified, in the same way
and for the same reason -- the acceptance criteria named a *sampling method* where they meant a
*property*. Every number below was measured against the committed assets before any change was
made, not taken from a reviewer's report.

### What the loop-1 assertions still permitted

| mutation of the committed assets | loop-1 result |
|---|---|
| master brightened +8/255, `.ico` left stale | all seven passed |
| master brightened +16/255, `.ico` left stale | all seven passed |
| master brightened +24/255, `.ico` left stale | all seven passed |
| master brightened +32/255, `.ico` left stale | caught |
| fully blank transparent 1024x1024 canvas | `MasterGeometry` **passed** |
| Wails scaffold placeholder as master | `MasterGeometry`, `NoOpaqueBackdrop` **passed** |
| master re-saved with no alpha channel | colour-model branch raised nothing |
| `build/appicon-source.jpg` deleted | whole suite passed |
| premultiply defect on the trailing edge only | not measurable -- see below |

The freshness tolerance was `18.0` where the same assets measure `0.62` when they genuinely
match: a pass corridor 29x the real signal, which let an artwork revision up to roughly 10% of
full scale ship a stale Windows icon green. That is the exact defect class Epic 6 exists to fix.

`MasterGeometry` checked a *floor* of 12 contiguous transparent rows with no ceiling. The
placeholder measures 115/97 and a blank canvas 1024/1024, so a test named "master geometry"
passed on an image containing no artwork at all. The genuine master measures 21/22.

`MasterAppliesAlphaOnce` measured half the boundary. Its comment claimed "one row can cross the
plaque's outer alpha boundary at most once"; a row crosses twice, entering and leaving, and the
implementation reset its run buffer on `alpha == 0`, discarding every trailing-edge transition
before a reference pixel was reached -- 969 leading runs used against 979 trailing runs dropped.

### Loop-2 mutation table

Every criterion's named mutation, run against the real files, each restored and sha256-verified
between runs. "Named test" is the one the criterion names; other tests failing alongside it are
listed where they did.

| # | mutation | named test that failed | also failed |
|---|---|---|---|
| 1 | master +8/255, `.ico` stale | `MasterMatchesIcoFreshness` | — |
| 2 | master +24/255, `.ico` stale | `MasterMatchesIcoFreshness` | — |
| 3 | hue-swapped master | `MasterMatchesIcoFreshness` | — |
| 4 | re-derive with `ERODE_PX = 0` | `NoOpaqueBackdrop` | `MasterAppliesAlphaOnce` |
| 5 | backdrop opaque except the 4 corners | `NoOpaqueBackdrop` | Geometry, Freshness, AlphaOnce |
| 6 | 16x16 entry all opaque white | `EveryIcoEntryCarriesTheArtwork` | — |
| 7 | master alpha 0, colour intact | `CarriesRealColour` | Geometry, Freshness, AlphaOnce |
| 8 | 16x16 entry, one opaque pixel | `EveryIcoEntryCarriesTheArtwork` | — |
| 9 | premultiply (whole image) | `MasterAppliesAlphaOnce` | — |
| 10 | **premultiply, trailing edge only** | `MasterAppliesAlphaOnce` | — |
| 11 | stretch plaque to fill | `MasterGeometry` | Backdrop, Freshness, AlphaOnce |
| 12 | fully blank transparent canvas | `MasterGeometry` | Colour, Freshness, AlphaOnce |
| 13 | scaffold placeholder as master | `MasterGeometry` | Colour, Freshness, AlphaOnce |
| 14 | master saved without alpha | `MasterGeometry` | Backdrop, Freshness, AlphaOnce |
| 15 | 48x48 entry dropped | `IcoCarriesTheFullWailsSizeSet` | EveryIcoEntry |
| 16 | **extra 24x24 entry added** | `IcoCarriesTheFullWailsSizeSet` | — |
| 17 | `.ico` reverted to scaffold | `IcoCarriesTheFullWailsSizeSet` | Colour, EveryIcoEntry, Freshness |
| 18 | `build/appicon-source.jpg` deleted | `SourceRenderIsThePinnedGeometry` | — |

18 of 18 failed their named test. Mutations 10, 12, 13, 14, 16 and 18 are new in loop 2 and had
no coverage at all before it; 1 and 2 passed before it.

### One false failure found and fixed during loop 2, not papered over

Extending the border-ring assertion to every `.ico` entry failed on the genuine assets: the 128,
64, 48, 32 and 16 entries measured 71.7%, 66.7%, 34.0%, 25.8% and 6.7% transparent against a 75%
floor. That is not a surviving backdrop -- it is geometry. `ringWidthFor` clamps to 1px at small
sizes, and a 1px ring on a 16px tile is almost entirely plaque edge, because the rounded corner
that makes the ring mostly-transparent at 1024px is only ~3px across at 16px.

The near-white property generalises; the transparent-fraction property does not. So the fraction
check is now gated to the master and the 256 entry, where the ring is wide relative to the corner
radius, and the near-white check runs on every entry. The limit this leaves is stated in the
assertion's own doc comment rather than implied away: a deliberately opaque *coloured* backdrop on
a small entry is outside what it can see.

### Calibration recorded rather than round-numbered

- `freshnessTolerance255` is now `8 * measuredSameArtworkDistance255` (0.62), not an absolute
  constant. The measured floor is its own named constant so a future Wails or winicon resampling
  change moves a number that says what it is, rather than being blamed on artwork drift.
- `maxTransparentBandRows = 40`, against the genuine master's 21/22 and the placeholder's 97.
- `maxLowToHighDeviationRatio = 2.5`: genuine master 3.19/2.47 = 1.3; with the premultiply defect
  27.96/3.93 = 7.1. This asserts the alpha-correlated asymmetry the test is named for, which the
  absolute bound alone left unchecked -- uniform noise raises both buckets together.
- `minCentralSaturation`'s comment said the master measures 0.57; recomputed exactly as the code
  does, it is **0.592**. The 0.57 predated loop 1's alpha fix.

## Review loop 3

Loop 2 hardened the assertions and still left three holes, each of the same shape as loop 1's and
loop 2's: an assertion pinned a sampling method rather than a property, and a tolerance was picked
for headroom rather than derived from a measured boundary.

### What loop 2's assertions still permitted

| mutation of the committed assets | loop-2 result |
|---|---|
| any of the 16/32/48/64/128 entries replaced by a flat coloured square | **entire suite passed** |
| master brightened +2, +4 or +6/255 with the `.ico` left stale | **all passed** (+8 was the first caught) |
| plaque narrowed 240px | `MasterGeometry` did not fire |
| `build/appicon-source.jpg` swapped for a different 2816x1536 render | **entire suite passed** |

Only the 256 entry was ever compared against the master, so five of six entries -- the sizes
Windows actually draws in the taskbar -- could hold no artwork at all. The freshness tolerance had
been changed from an absolute 18.0 to an absolute 4.96 and reported as closing the stale-icon gap;
the new boundary was never measured, and it sat at a 2.4% tonal revision.

Two constants were also wrong in their own comments. `maxLowToHighDeviationRatio` claimed the
genuine master measures 3.19 / 2.47 = 1.3; it measures **3.172 / 2.621 = 1.21**. And the ratio
branch sat in a `switch` after the absolute bound, which both premultiply mutations trip first, so
by this repo's own standard the constant was decoration.

### What changed

Freshness now compares **every** entry against the master, each against its own measured
same-artwork distance (0.616 at 256 rising to 4.109 at 16, because the two resampling filters
disagree more the further the downsample goes) times a factor of 1.5. A factor cannot drift away
from its measurement the way an absolute can. Geometry bounds columns as well as rows. The
alpha-once test counts leading and trailing runs separately, flushes a run that reaches the right
image edge, guards against a zero high-alpha mean, and evaluates its two bounds independently.
Sample floors moved from 5% of the measured values to roughly half.

Every measured value is now logged on each run (`go test -run TestAppIcon -v`), so drift toward a
bound is visible rather than discovered.

### Derived mutation table

Produced by `scripts/verify-asset-mutations.py`, which applies each mutation to the real committed
assets, runs the real suite, and restores from git with a sha256 check between cases. It is the
asset-side equivalent of `scripts/verify-native-mutations.sh`, and it exists because loop 2's table
was hand-written and carried three stale numbers into a merged evidence file -- exactly what
AGENTS.md's Story 3.8 lesson warns about.

```
baseline: all TestAppIcon* pass

| # | mutation | named test | killed |
|---|---|---|---|
| 1 | master brightened +2/255, .ico stale | `MasterMatchesIcoFreshness` | yes |
| 2 | master hue-swapped, .ico stale | `MasterMatchesIcoFreshness` | yes |
| 3 | 16x16 entry a flat coloured square | `MasterMatchesIcoFreshness` | yes |
| 4 | 48x48 entry a flat coloured square | `MasterMatchesIcoFreshness` | yes |
| 5 | 128x128 entry a flat coloured square | `MasterMatchesIcoFreshness` | yes |
| 6 | 16x16 entry opaque white | `EveryIcoEntryCarriesTheArtwork` | yes |
| 7 | re-derived with ERODE_PX = 0 | `NoOpaqueBackdrop` | yes |
| 8 | master alpha 0, colour intact | `CarriesRealColour` | yes |
| 9 | premultiply defect, full | `MasterAppliesAlphaOnce` | yes |
| 10 | premultiply defect, 20% (ratio bound only) | `MasterAppliesAlphaOnce` | yes |
| 11 | plaque stretched to fill canvas | `MasterGeometry` | yes |
| 12 | plaque narrowed 240px (columns) | `MasterGeometry` | yes |
| 13 | master fully blank | `MasterGeometry` | yes |
| 14 | master is the Wails scaffold placeholder | `MasterGeometry` | yes |
| 15 | .ico reverted to the Wails scaffold | `IcoCarriesTheFullWailsSizeSet` | yes |
| 16 | .ico 48x48 entry dropped | `IcoCarriesTheFullWailsSizeSet` | yes |
| 17 | .ico gains an extra 24x24 entry | `IcoCarriesTheFullWailsSizeSet` | yes |
| 18 | source render deleted | `SourceRenderIsThePinnedGeometry` | yes |
| 19 | source render replaced, same size | `SourceRenderIsThePinnedGeometry` | yes |

19 of 19 mutations failed their named test.

freshness boundary, master brightened by +N with the .ico left stale:
  +0   missed
  +1   CAUGHT
  +2   CAUGHT
  +3   CAUGHT
  +4   CAUGHT
```

Case 10 is the one that earns the ratio bound its place: a 20% premultiply measures 6.946 against
a high-alpha 3.001, which stays **under** the absolute bound of 8.0 and is caught only by the
ratio. The full premultiply trips both, so it proves nothing about the ratio on its own.

The freshness boundary is now measured rather than asserted: +0 (the genuine asset) passes and
+1/255 fails, against loop 2's first catch at +8.

