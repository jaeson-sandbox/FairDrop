# Evidence: Story 9.5: Rebuild Sending Around a Ring

## Summary

Sending now reuses Staged's own card geometry verbatim: `.fd-packet`
wrapping `.fd-hero`'s fixed 216px/`minmax(0, 1fr)` grid, with a 216px
progress ring in the QR's own fixed slot instead of the QR bitmap. The
`fd-packet-tab` kind label and the standalone `.fd-transfer-view` card are
gone; the item row (kind glyph, name, meta) is the same one `StagedView.tsx`
renders, unstaggered, since it is the one thing that does not change moving
from Staged to Sending. The three progress modes -- known positive,
unknown total, known empty -- are rewritten against the ring rather than
the retired linear meter, each keeping its own mutation. `copy.label.sent`/
`copy.label.of` and progress speech (`progressSpeech.ts`) are byte-for-byte
unchanged.

## What changed

**`TransferringView.tsx` (full rewrite):**

- `.fd-staged-head` (reused, unmodified rule) centres the "Sending" heading
  alone -- no subtitle, per the story's own text.
- `.fd-packet` > `.fd-hero` > `ProgressRing` (the fixed 216px slot) +
  `.fd-hero__details` (item row, figures, Cancel), exactly Staged's DOM
  shape.
- `ProgressRing` renders one of four states from `ProgressSelection | null`:
  - `null` (before the first accepted snapshot): a plain, undecorated
    track, no `data-progress-mode`, no role -- the card's shape never jumps
    once the first snapshot lands.
  - `known-empty`: the track only, `aria-hidden="true"`, no `role`.
  - `unknown`: a static, non-directional dashed fill
    (`.fd-ring__fill--unknown`, `stroke-dasharray`, never a keyframe or a
    rotation of its own), `role="progressbar"`, no `aria-valuenow`,
    `copy.progress.unknown` beneath the ring via `aria-labelledby`.
  - `known-positive`: a determinate ring, `role="progressbar"` with
    `aria-valuenow`/`aria-valuemin`/`aria-valuemax`, the fill's
    `stroke-dashoffset` computed from the byte pair (never the wire's own
    rounded `percent` field, which `ProgressSelection` does not even
    expose), and the tabular-numeral percentage centred inside the ring.
- `TransferMetrics` unchanged in shape (two figure-over-caption pairs,
  known-empty omitting the speed pair) but captioned with two **new** keys,
  `copy.label.sentCaption`/`copy.label.speedCaption` ("Sent"/"Speed") --
  see "Cross-story collision" below for why these are new keys rather than
  a rename of the existing `wireBytes`/`throughput`.
- Cancel: unchanged behaviour (pending label, `aria-disabled`, focus kept),
  now inside `.fd-hero__details` with the item row and figures.
- `FileKindGlyph`/`FolderKindGlyph` and the `rise()` stagger helper are
  local, unexported duplicates of `StagedView.tsx`'s own (this repo's
  established convention for a helper too trivial to extract yet -- see
  that file's own comment on the same function).

**`copy.ts`:** two new `label` keys, `sentCaption: 'Sent'`,
`speedCaption: 'Speed'`. `wireBytes`/`throughput` ("Wire bytes"/
"Throughput") are untouched. Neither pair needs an EXPERIENCE.md Voice and
Tone table row -- both are `label` entries, the short functional words the
spine names in prose rather than tabulates (copy.ts's own header comment,
and `copy.test.ts`'s own worked example naming `throughput` specifically).

**`style.css`:** `.fd-transfer-view`, `.fd-progress-head`,
`.fd-progress-percent`, `.fd-meter`, `.fd-meter--unknown`,
`.fd-meter__fill`, `.fd-meter-label` (all Story 7.5, all now dead --
`.fd-meter` and friends were consumed only by the file this story rewrote)
removed. New: `.fd-ring-panel` (the 216px slot, `.fd-qr-panel`'s own
fade-plus-scale-from-0.94 entrance, same 759px reflow order swap),
`.fd-ring-frame` (the 216x216 square the SVG draws in, separate from the
panel so the unknown mode's caption can sit beneath it without stretching
the ring oval), `.fd-ring`/`.fd-ring__track`/`.fd-ring__edge`/
`.fd-ring__fill`/`.fd-ring__fill--unknown`/`.fd-ring__pct`/
`.fd-ring__status`. `.fd-empty-status`/`.fd-metrics`/`.fd-metric*` are
untouched (reused as-is). The 759px reflow breakpoint's `.fd-qr-panel`
order rule now also names `.fd-ring-panel`.

**`.fd-ring__track`/`.fd-ring__edge` -- two strokes, not one.** The ring's
track needed both a decorative fill *and* a separate functional-boundary
edge, the same two things the retired linear meter split across its
`background`/`border`, for a reason found by measuring rather than
assuming: `--color-track` alone is ~1.2:1 against `--color-surface`/
`--color-elevated` in both authored modes (measured with the same
relative-luminance formula `styles.test.ts` itself uses), nowhere near the
3:1 DESIGN.md's Colors table requires on "the progress-track outline".
Moving `.fd-ring__track` itself to `--color-control-border` instead was
considered and rejected: `styles.test.ts` already pins `primary` on `track`
at 4.56/4.95 (light/dark) -- the contrast between the determinate fill and
the track it partially covers, the boundary a viewer actually needs to see
-- and `primary` on `control-border` measures only ~1.53/1.86, which would
have made the filled and unfilled arcs hard to tell apart at exactly the
edge that matters. `.fd-ring__edge` is a second, thin (1px) concentric
stroke at the track band's own outer radius instead, in
`--color-control-border`, leaving the wide band's own color untouched.
`styles.test.ts`'s existing "functional boundary" `controls` list, which
used to include `.fd-meter`, now checks `.fd-ring__edge` in its place.

**Cross-story collision, resolved without touching the other story's
files.** The epics' own AC text names `copy.label.wireBytes` -> "Sent" and
`copy.label.throughput` -> "Speed" as the keys to rename. Running the full
suite after the first draft found `OutcomePanel.tsx` (Story 9.6's own scope,
explicitly off-limits to this session) reads those same two keys for its
Completion Receipt captions -- renaming them in place would have silently
changed the Done receipt's wording too and broken
`OutcomePanel.test.tsx`'s own literal "Wire bytes"/"Throughput"
assertions, a file this session was told not to touch. Resolved by adding
two new keys instead of renaming the shared ones: `wireBytes`/`throughput`
keep their original values for the receipt; `sentCaption`/`speedCaption`
are Sending's own pair. This is a deliberate, reported deviation from the
AC's literal key names -- the *displayed* strings match the AC exactly
("Sent"/"Speed"), only the registry key names differ from what the epics
text specified.

**`accessibility.test.tsx`:** new `renderTransferringInAppShell` helper
(mirrors `renderIdleInAppShell`'s pattern -- `TransferringView` inside a
real `.fd-app` shell with an explicit `height: 100vh`) and a new describe
block, "the sending card keeps the ring in Staged's fixed column (Story
9.5)", with four `it.each([[1024, 768], [640, 480]])` cases: the ring
column's width and its being narrower than the details column, a realistic
item name staying on one line, the percentage centred inside the ring, and
the whole card staying within the viewport with nothing overflowing.

### Files touched

- `frontend/src/ui/TransferringView.tsx`, `frontend/src/ui/TransferringView.test.tsx`
- `frontend/src/ui/copy.ts`, `frontend/src/ui/copy.test.ts`
- `frontend/src/style.css`, `frontend/src/ui/styles.test.ts`
- `frontend/browser/accessibility.test.tsx`
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md`
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (story -> `review`)

No file listed in the parallel Story 9.6 assignment (`OutcomePanel.tsx`,
`App.tsx`, `IdleView.tsx`, `OutcomePanel.test.tsx`, or the outcome/receipt
rules in `style.css`) was touched.

## Rendered measurements (1024x768 and 640x480, Chromium, `renderTransferringInAppShell`)

| Measurement | 1024x768 | 640x480 |
|---|---|---|
| `.fd-ring-panel` width | 216.0px | 216.0px |
| `.fd-hero__details` width | 438.0px | 550.0px |
| Item name ("dev-environment-guide.html") wraps? | No (1 line) | No (1 line) |
| Percentage horizontal offset from ring centre | 0.0px | 0.0px |
| Percentage vertical offset from ring centre | 0.0px | 0.0px |
| `.fd-packet` left/right edges | 152.0 / 872.0 (within 0..1024) | 24.0 / 616.0 (within 0..640) |

Details is wider at 640x480 than at 1024x768 because the card crosses the
759px reflow breakpoint below 640: the ring and details stack into one
column there (details takes the near-full card width), while at 1024 the
two-column grid still splits the card's own constrained width between the
216px ring track and the `minmax(0, 1fr)` details track. Both numbers
still clear the "details wider than ring" assertion the mutation table's
M9 exercises.

## Mutation table

Each mutation was applied to the real working tree, confirmed to fail
(naming the problem), then reverted -- confirmed clean afterward via
`git diff --stat` / re-running the affected suite green.

| # | Mutation | AC | File | Result |
|---|---|---|---|---|
| M1 | Removed `aria-valuenow={percent}` from the determinate ring | AC2 | `TransferringView.tsx` | KILLED -- 3 tests failed: "renders a determinate progress ring carrying the derived percentage" (`expected null` where `'69'` was wanted), "keeps the metrics readable while the cancellation is outstanding", "gives a known positive total a finite value between an explicit min and max" |
| M2 | Rounded the fill's `stroke-dashoffset` from the already-rounded `percent` instead of the exact `progress.value` | AC2 | `TransferringView.tsx` | KILLED -- "draws the ring from the byte pair, not from the wire-reported percent": `expected 186.987... to be close to 186.700...`, difference 0.287, tolerance 5e-7 |
| M3 | Added `role="progressbar"` to the known-empty ring | AC2 | `TransferringView.tsx` | KILLED -- "keeps a decorative ring that claims nothing": `expected true to be false` on `panel.hasAttribute('role')` |
| M4 | Dropped the folder meta line's `copy.label.logicalSize` suffix (used `formatBytes` alone) | AC1 (card geometry, item row) | `TransferringView.tsx` | KILLED -- "keeps the folder identity, its logical size distinct from the wire total, and its ZIP note visible": `expected 'Folder · 36.8 MB' to be 'Folder · 36.8 MB logical size'` |
| M5 | Removed `transition: stroke-dashoffset 400ms var(--ease-decelerate);` from `.fd-ring__fill` | AC2 | `style.css` | KILLED -- "transitions the determinate fill's stroke-dashoffset over 400ms, never a keyframe": rule no longer contained the transition |
| M6 | Removed `stroke-dasharray: 4 10;` from `.fd-ring__fill--unknown` | AC2 | `style.css` | KILLED -- "progress presentation > keeps the unknown ring static": the dasharray regex no longer matched |
| M7 | Changed `.fd-ring__edge`'s stroke from `--color-control-border` to `--color-separator` | AC2 (single-gradient/solid-stroke rule, and the pre-existing functional-boundary contrast guarantee) | `style.css` | KILLED -- 3 tests failed: "draws its own functional-boundary edge, distinct from the decorative track fill", plus both generic `controls`-list contrast tests ("%s draws its boundary with the functional token" and "never uses the decorative edge as the sole boundary") |
| M8 | Changed `copy.label.sentCaption` from `'Sent'` to `'Wire bytes'` | AC3 | `copy.ts` | KILLED -- 2 tests failed: `copy.test.ts`'s literal assertion, and `TransferringView.test.tsx`'s "shows zero wire bytes and omits the meaningless speed" (`'0 bytes sentWire bytes'` where `'0 bytes sentSent'` was wanted) |
| M9 | Reverted `.fd-hero`'s `grid-template-columns` to `minmax(0, 1fr) 216px` (fixed track second, the exact Story 9.4 defect) | AC1 | `style.css` | KILLED -- rendered (Chromium) "gives the ring a ~216px column narrower than the details beside it at 1024x768": `expected 216 to be greater than 216` |

M1-M3 prove the three progress modes' own ARIA contract. M2 proves the
determinate ring reads the authoritative byte pair rather than the wire's
own rounded percentage -- the same guarantee Epic 1 retrospective item 7
established for the linear meter, now re-proven for the ring at a
precision (1e-6) tight enough to catch even a one-step rounding
regression. M4 proves the item row's logical-size labelling survived the
move from the old packet-tab design. M5/M6 prove the two CSS-level motion
claims the acceptance criteria name explicitly (400ms transition; static,
non-directional unknown pattern). M7 proves the two-stroke boundary design
decision documented above is load-bearing, not decorative, and that the
pre-existing sheet-wide contrast-boundary sweep still covers the ring.
M8 proves the new caption keys are load-bearing copy, not dead strings. M9
is the rendered-layout mutation the story's own instructions require
("mutation-verify at least the column assertion"), reproducing the exact
Story 9.4 defect this story's card geometry shares, now caught at the
Sending card too.

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above and with the working tree back to its intended diff.

- `wails build`: **PASS** -- `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 12.995s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change.
- Bindings drift: **PASS** after `git checkout -- frontend/wailsjs` -- `git diff --stat` showed 0 insertions/0 deletions (mode-only churn). No exported `App` command surface changed in this story.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`go env CGO_ENABLED` confirmed `1` first; `internal/stream` ~98s under the race detector, the rest a few seconds each).
- `GOOS=darwin GOARCH=arm64 go build ./...`: **PASS**. `GOOS=linux GOARCH=amd64 go build ./...`: **PASS**. `GOOS=darwin GOARCH=arm64 staticcheck ./...` (bare binary): **PASS**. This story touched no Go code; these three are the AGENTS.md-mandated pre-flight, run for completeness.
- `cd frontend && npm test` (`npx vitest run`): **PASS** -- 19 files, **750 tests**.
- `cd frontend && npm run test:browser`: **PASS** -- 2 files, **49 tests** (8 new for this story's rendered layout describe block).
- `git checkout -- frontend/browser/captures/`: not needed -- `git status --short` showed no change to `frontend/browser/captures/` after either browser-test run.
- No content drift beyond the nine files this story intentionally touched, confirmed via `git status --short` after the full gate.

## One line per AC

1. Sending reuses Staged's exact card geometry (`.fd-hero`'s fixed
   216px/`minmax(0, 1fr)` grid): a ~216px ring in the QR's slot, the item
   row, two figures, and Cancel beside it; the packet tab is gone.
   **Done, mutation-verified (M9, rendered).**
2. The three progress modes render exactly as specified -- determinate
   ring with `role="progressbar"`/`aria-valuenow`, 400ms
   `stroke-dashoffset` transition, tabular-numeral percentage centred
   inside; static non-directional dashed ring for unknown, no
   `aria-valuenow`; no percentage-bearing progressbar at all for
   known-empty. The three existing mode-test describe blocks are rewritten
   against the ring (every prior mutation's intent is kept, now expressed
   against ring elements); the single-gradient rule holds (`.fd-ring__fill`
   is a solid `stroke`, never a gradient). **Done, mutation-verified
   (M1, M2, M3, M5, M6).**
3. `copy.label.sentCaption`/`copy.label.speedCaption` ("Sent"/"Speed") caption
   the two figure-over-caption pairs; the figures remain actual wire bytes
   and visual-only throughput. **Done, mutation-verified (M8) -- see
   "Cross-story collision" above for why these are new keys rather than a
   rename of `copy.label.wireBytes`/`copy.label.throughput`.**
4. `progressSpeech.ts` is untouched -- byte-for-byte identical to before
   this story, confirmed by `git diff` showing no change to that file, and
   its own test suite (`progressSpeech.test.ts`) passing unchanged. **Done.**
5. Reduced motion: the determinate fill's transition collapses to the
   sheet's universal 1ms under `prefers-reduced-motion: reduce` (the same
   generic `*, *::before, *::after` rule every other transition in the
   sheet already relies on); nothing on the ring rotates as an animation --
   its `-90deg` orientation is a fixed, static value declared once, never a
   transition or `@keyframes` target (pinned directly: "gives the ring a
   fixed, static drawing orientation rather than an animated rotation").
   **Done.**
6. Full gate passes (above). **Done.**

## Disagreements between the story text and the prototype

- **The owner-approved prototype's `t-sending` item row carries no `rise`
  stagger class**, while its `.stats` (`--i="1"`) and Cancel wrapper
  (`--i="2"`) do. The story text does not specify stagger indices, so the
  prototype's own choice was followed rather than treated as a
  disagreement to flag: the item row enters unstaggered (matching Staged's
  own item, which *does* stagger, is a difference between the two views
  the prototype itself makes deliberately -- Sending's item is a
  continuation of the same identity Staged already announced, so it does
  not need re-announcing with a delay). Recorded here for visibility, not
  as an unresolved conflict.
- **Cross-story key collision** (`copy.label.wireBytes`/`copy.label.throughput`):
  covered in full above under "Cross-story collision, resolved without
  touching the other story's files." The AC's literal key names could not
  be honoured without either breaking `OutcomePanel.test.tsx` (Story 9.6's
  scope) or editing a file this session was told not to touch; the
  *displayed* strings match the AC exactly via two new keys instead.

## Native verification

Pending -- orchestrator, per this story's own instructions ("Native checks
are 'pending — orchestrator'"). This session drove the rendered-Chromium
suite (`npm run test:browser`) and confirmed the ring's three modes, the
column geometry, the centred percentage and the card's containment at
1024x768 and 640x480 there, but did not drive the built macOS binary by
hand. Per AGENTS.md's rule for anything WebKit-sensitive, the built binary
should still be checked before this ships: a real transfer showing the
ring animate from 0% to completion (both known and unknown totals if
feasible to force), in both colour schemes and with Reduce Motion on.

## Nothing else left open

Every acceptance criterion is implemented and mutation-verified above. The
one cross-story key collision found while running the full suite is
resolved without touching Story 9.6's files, and recorded as a deliberate,
reported deviation from the AC's literal key names (the displayed strings
match exactly). Native verification is the orchestrator's, as scoped.

## Review follow-up (branch `fix-9-5-sending-details`, from `origin/epic-9-motion-and-clarity` @ `9eb0702`)

After Story 9.5 merged, the orchestrator's rendered check at 1024x768
against the owner-approved prototype (`mockups/epic-9-proposal.html`,
`t-sending`, `.ring .pct`, `.stats`, `.stat`) found four visual defects
neither the jsdom suite nor the existing rendered describe block caught.
All four are pure layout/visual bugs the text-only suites cannot see, so
this follow-up adds rendered-Chromium coverage for each, following the
defect-fix carve-out: rendered test extended/added, fix applied, mutation
confirming the test actually catches the regression.

### Defect 1: the percentage and its "%" sign rendered on two separate lines, one above the ring's centre and one near its bottom edge

**Cause, two independent bugs in the same rule.**

1. `.fd-ring__pct` was `display: grid; place-items: center` with two direct
   children (the number `<span>` and the `<small>` "%" sign) and no
   declared `grid-template-columns`. An implicit grid with no explicit
   column count packs multiple children into separate rows by default
   (`grid-auto-flow: row`, one implicit column), so the number centred on
   its own row while "%" centred on a second row down.
2. `.fd-ring__pct` was also `position: absolute; inset: 0` -- stretched to
   fill the entire 216px frame -- with the centring applied *inside* that
   already-full-size box. Fixing bug 1 alone (switching to `display: flex;
   align-items: baseline`) put both children on one line, but the line
   still rendered flush with the box's own cross-start edge rather than
   centred in the 216px frame: measured at ~91px above the frame's true
   centre, not a rounding difference.

**Why the existing suite missed it.** The "centres the determinate
percentage inside the ring" test (added with Story 9.5 itself) measured
`.fd-ring__pct`'s own `getBoundingClientRect()` against the frame's --
but that element was `position: absolute; inset: 0`, so its bounding box
is *always* exactly the frame's own box by construction, regardless of how
its children are laid out inside it. The test could not have caught either
bug; it was proving the container was the container.

**Fix.** Matches the owner-approved prototype's own structure exactly:
`.fd-ring-frame` (the parent) becomes the `display: grid; place-items:
center` container; `.fd-ring` (the `<svg>`) becomes `position: absolute;
inset: 0` so it never participates in the frame's grid layout (matching
the prototype's own `.ring svg`); `.fd-ring__pct` drops its own
`position`/`inset` and becomes a plain, content-sized, normal-flow grid
item, with `display: flex; align-items: baseline` internally so the number
and "%" share one text baseline. A content-sized item centred by its
parent's `place-items: center` is centred on both axes by construction --
there is no cross-axis guess left to get wrong.

**New/extended test** (`browser/accessibility.test.tsx`, "keeps the
percentage and its '%' sign on one baseline, centred together in the ring",
replacing the reasoning gap in the pre-existing "centres the determinate
percentage..." test, which is kept as-is alongside it): measures the
number `<span class="fd-ring__pct-value">` and the `<small>` directly,
not the stretched container -- same baseline (bottom edges within 5px,
using bottom rather than centre-Y because a 30px number and a 12.5px "%"
sharing a true baseline still have several pixels of centre-to-centre
offset purely from the font-size difference), horizontally adjacent (gap
between -1px and 6px), and their **combined** bounding box centred in the
ring frame (within 4px, both axes).

**Before/after measurements (1024x768 / 640x480, `renderTransferringInAppShell()`'s default known-positive state):**

| Measurement | Before (broken) | After (fixed) |
|---|---|---|
| Number/`%` bottom-edge difference (same-baseline check) | 97.9px (both viewports -- two separate rows) | 3.1px (both viewports) |
| Combined "43%" box vs. frame centre, horizontal | not measured (container-only test passed trivially) | 0.01px off (both viewports) |
| Combined "43%" box vs. frame centre, vertical | 90.75px off (identical at both viewports) | 0.0px off (both viewports) |

### Defect 2: the figures were still Story 7.5's bordered metric boxes, and the value repeated its own caption

**Cause.** `.fd-metric` still carried Story 7.5's `padding`/`border: 1px
solid var(--color-separator)`/`border-radius`/`background` -- never
revisited when Story 9.5 moved the percentage into the ring and left these
two figures as the card's only remaining Story 7.5 leftover. Separately,
the "Sent" figure's value read `${formatBytes(bytesSent)} ${copy.label.sent}`
(e.g. "14.0 KB sent") directly over its own "Sent" caption -- the value
repeating the caption's word rather than adding a second fact.

**Fix.** `.fd-metrics` becomes a plain flex row (`display: flex; flex-wrap:
wrap; gap: var(--spacing-6)`, ~22px per the prototype's own `.stats`, the
nearest design token) and `.fd-metric` drops every box property, becoming
a flex column stacking the bold value over the muted caption with no
border, fill, or padding -- matching the prototype's `.stats`/`.stat`
exactly. `TransferMetrics` now reads sent-of-total for a known total
(`copy.label.of`, already registered) -- "14.0 KB of 32.6 KB" -- which is
new information the caption alone does not carry; an unknown total or an
empty payload has no meaningful "of X" to add, so those two read the bare
wire figure instead. Speed is unaffected (`formatRate`, captioned "Speed").
The 639px reflow breakpoint, which used to reset `.fd-metrics`'s
`grid-template-columns`, now switches its `flex-direction` to `column`
instead (there is no longer a grid template to reset).

**New/extended tests:** `browser/accessibility.test.tsx`, "gives each
figure a plain, unboxed presentation whose value does not repeat its own
caption" -- computed `borderWidth` is `0px` (not `borderStyle`, which
Tailwind's preflight resets to `solid` globally regardless of whether a
border is actually declared -- `borderWidth` is the property that
distinguishes a real border from that reset), computed `backgroundColor`
is transparent, and the figure's own text does not end with its caption's
text. `styles.test.ts` gained matching CSS-text assertions (no
`border`/`background`/`padding` on `.fd-metric`, `display: flex` not
`grid-template-columns` on `.fd-metrics`, the 639px breakpoint folding to
`flex-direction: column`). `TransferringView.test.tsx`'s existing mode
tests updated for the new value text ("5.8 MB of 8.4 MB" for known
positive, "48.2 MB"/"0 bytes" bare for unknown/known-empty).

**Before/after measurements:** known-positive figure text "5.8 MB
sentSent" (repeated word, concatenated with its own caption in the DOM) ->
"5.8 MB of 8.4 MBSent" (new information, no repeat); `.fd-metric` computed
`borderWidth` 1px -> 0px; computed `backgroundColor` `rgb(255, 255, 255)`
(opaque `--color-surface`) -> `rgba(0, 0, 0, 0)` (transparent).

### Defect 3: the known-empty status read as a bordered, filled box -- an inert control, not plain text

**Cause.** `.fd-empty-status` carried the same Story 7.5-era
`padding`/`border: 1px solid var(--color-control-border)`/`border-radius`/
`background` as the old linear meter's own card-within-a-card styling,
never revisited once Story 9.5 rebuilt the surrounding card.

**Fix.** Drops `padding`/`border`/`border-radius`/`background` entirely,
keeping only `margin: 0`, muted colour, and the existing body-strong
size/weight -- the same visual weight class as `.fd-ring__status` (the
unknown mode's own plain caption), per the follow-up's own instruction.
The known-empty ring panel's own `aria-hidden`/no-`role` guarantee (Story
9.5's original work) is untouched -- this is a purely visual change.

**New test:** `browser/accessibility.test.tsx`, "gives the known-empty
status plain text with no border or fill" -- same `borderWidth`/
`backgroundColor` computed-style check as Defect 2, applied to
`.fd-empty-status` after rendering with an explicit known-empty
`ProgressSnapshot` override. `styles.test.ts` gained a matching CSS-text
assertion.

**Before/after measurements:** computed `borderWidth` 1px -> 0px;
computed `backgroundColor` `rgb(255, 255, 255)` -> `rgba(0, 0, 0, 0)`.

### Defect 4: Cancel rendered as a full-width bar instead of an intrinsic-width, left-aligned button

**Cause.** Cancel is the sole block-level child at the foot of
`.fd-hero__details` (a column flex container), so it inherited
`.fd-button--quiet`'s shared `width: 100%` -- authored for Idle's and the
old Staged Cancel, which really were the only child of their own
containers -- and rendered as a full-width bar. `width: 100%` was also
propped up by the flex container's own default `align-self: stretch`,
which fills the cross axis (the container's width) for any item whose own
`width` resolves to `auto`; overriding `width` alone would not have been
enough.

**Fix.** `.fd-hero__details .fd-button--quiet { width: auto; align-self:
flex-start; }` -- the same two-part fix `.fd-staged-foot
.fd-button--quiet` already applies for the identical shared rule, scoped
here instead of shared with Staged's foot row because Transferring's
Cancel has no disclosure beside it to share a row with.

**New test:** `browser/accessibility.test.tsx`, "gives Cancel an intrinsic
width well under half the details column" -- Cancel's rendered width
compared against the details column's own width, at both viewports.
`styles.test.ts` gained a matching CSS-text assertion for the new rule.

**Before/after measurements (1024x768 / 640x480):** Cancel's rendered
width equalled the details column's own width exactly (438.0px / 550.0px
-- a literal full-width bar) before the fix; well under half of it
afterward (comfortably passing `< detailsWidth / 2`, i.e. `< 219.0px` /
`< 275.0px`).

### Mutation table

Each mutation was applied to the real working tree, confirmed to fail
(naming the defect) via the rendered suite, then reverted -- confirmed
clean afterward by re-running the affected test green. A matching
CSS-text mutation was also run for each defect via `styles.test.ts` where
one exists.

| # | Mutation | Defect | File | Result |
|---|---|---|---|---|
| N1 | Reverted `.fd-ring-frame`/`.fd-ring`/`.fd-ring__pct` to the pre-fix rules (grid-with-no-columns, `.fd-ring__pct` stretched and self-centring) | 1 | `style.css` | KILLED -- rendered: "keeps the percentage and its '%' sign on one baseline, centred together in the ring" at both viewports, `expected 97.9375 to be less than or equal to 5` |
| N2 | Reverted `.fd-metric` to Story 7.5's bordered/filled box | 2 | `style.css` | KILLED -- rendered: "gives each figure a plain, unboxed presentation..." at both viewports, `expected '1px' to be '0px'`; jsdom: `styles.test.ts`'s "gives the metrics row a plain flex layout..." also killed independently (`grid-template-columns` reintroduced) |
| N3 | Reverted `TransferMetrics`'s value text to `${formatBytes(bytesSent)} ${copy.label.sent}` (the old repeated-caption form) unconditionally | 2 | `TransferringView.tsx` | KILLED -- rendered: "gives each figure a plain, unboxed presentation..." at both viewports, `expected true to be false` ("5.8 MB sent" repeats "Sent") |
| N4 | Reverted `.fd-empty-status` to its bordered/filled box | 3 | `style.css` | KILLED -- rendered: "gives the known-empty status plain text with no border or fill" at both viewports, `expected '1px' to be '0px'` |
| N5 | Removed the new `.fd-hero__details .fd-button--quiet` rule entirely | 4 | `style.css` | KILLED -- rendered: "gives Cancel an intrinsic width well under half the details column" at both viewports, `expected 438 to be less than 219` (1024x768) / `expected 550 to be less than 275` (640x480) |

N1-N5 are exactly the five mutations the follow-up's own instructions ask
for (one per defect, plus the JS-level half of Defect 2's two-part fix).
Each was confirmed to fail naming the specific defect before being
reverted; the full suite was green before and after the whole pass.

### Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above and with the working tree back to its intended diff.

- `wails build`: **PASS** -- `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 8.546s.` Same pre-existing linker warning as every prior run, unrelated to this change.
- Bindings drift: **PASS** after `git checkout -- frontend/wailsjs` -- 0 insertions/0 deletions (mode-only churn). No exported `App` command surface touched.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for all nine packages.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`internal/stream` ~98s under the race detector).
- `GOOS=darwin GOARCH=arm64 go build ./...`, `GOOS=linux GOARCH=amd64 go build ./...`, `GOOS=darwin GOARCH=arm64 staticcheck ./...`: **PASS**, all three. This follow-up touched no Go code.
- `cd frontend && npm test` (`npx vitest run`): **PASS** -- 19 files, **755 tests** (5 more than the 750 the pre-follow-up evidence recorded: all 5 are new `it` blocks in `styles.test.ts` -- one split out of the rewritten "renders the percentage..." test plus the four-test "the Sending figures and Cancel read as plain text..." describe block. `TransferringView.test.tsx`'s changes are assertion rewrites inside existing `it`s, not new ones).
- `cd frontend && npm run test:browser`: **PASS** -- 2 files, **57 tests** (8 new for this follow-up: 2 for Defect 1, 2 for Defect 2, 2 for Defect 3, 2 for Defect 4, each `it.each` across 1024x768/640x480).
- Capture churn: none -- `git status --short` showed no change to `frontend/browser/captures/` after either browser-test run.
- No content drift beyond the five files this follow-up touches (`frontend/src/ui/TransferringView.tsx`, `frontend/src/ui/TransferringView.test.tsx`, `frontend/src/style.css`, `frontend/src/ui/styles.test.ts`, `frontend/browser/accessibility.test.tsx`), confirmed via `git status --short` after the full gate.

### One line per defect

1. Percentage and "%" now share one baseline and centre together in the
   ring, fixed by mirroring the prototype's own grid-centres-a-content-
   sized-box structure rather than a stretched self-centring container.
   **Done, mutation-verified (N1).**
2. Figures are unboxed (no border/fill/padding), gapped ~22px as a flex
   row, and the value no longer repeats its own caption (sent-of-total for
   a known total, bare wire figure otherwise). **Done, mutation-verified
   (N2, N3).**
3. Known-empty status is plain, muted text with no border or fill,
   matching the unknown mode's own caption weight; still never a
   percentage-bearing progressbar (untouched). **Done, mutation-verified
   (N4).**
4. Cancel is intrinsic-width and left-aligned under the figures, keeping
   the 44px floor, the pending "Canceling" label, focus retention and
   `aria-disabled` (all pre-existing, unaffected by this purely visual
   fix). **Done, mutation-verified (N5).**
5. Full gate passes (above). **Done.**

Files outside this follow-up's scope (`OutcomePanel.tsx`, `App.tsx`,
`IdleView.tsx`, and Story 9.6's own outcome/receipt CSS and tests) were
not touched.
