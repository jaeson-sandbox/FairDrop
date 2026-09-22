# Evidence: Story 7.5: Rebuild Progress and the Outcome Panel

## What changed

### `frontend/src/ui/OutcomePanel.tsx`

Replaced the two non-visible `data-receipt-name`/`data-receipt-bytes-sent`
attributes Story 7.4 left as a wiring proof with the real Completion Receipt
UI, and rebuilt the Done/Error glyph:

- **`CompletionReceipt`** (new internal component): a `<dl>` of two
  `<div class="fd-receipt__cell">` groups -- item name (in a `<bdi dir="auto">`,
  captioned "File"/"Folder" from `copy.label.file`/`copy.label.folder`) and
  wire bytes sent (`formatBytes(receipt.bytesSent)`, captioned
  `copy.label.wireBytes`, the same approved word Transfer Metrics already
  uses). **Two cells, not three.** No new copy string was invented: both
  captions reuse words already in the approved registry.
- **`OutcomeIcon`** (new internal component): a 74px tinted disc. Done
  renders an inline `<svg class="fd-outcome__check">` with a single
  `<path pathLength={32}>` -- `pathLength` normalizes the path's own length
  to exactly 32 units regardless of its coordinates, so `stroke-dasharray: 32`
  / `stroke-dashoffset: 32` in `style.css` always mean "the whole stroke" /
  "none of it drawn," independent of the `d` attribute chosen. Error keeps the
  literal `!` glyph. A `useEffect` adds the `fd-outcome__check--drawn`
  modifier once, unconditionally, after mount -- not gated on
  `prefers-reduced-motion` in any way, because gating it there is exactly the
  bug the acceptance criterion's named mutation describes.
- **The single `onDismiss` control** now carries `fd-button--primary` when
  the outcome is live (nothing else on screen is the next action) and
  `fd-button--quiet` when it is retained (Idle's own browse control is the
  next action; this one only needs to be quiet Dismiss). The label stays
  `copy.outcome.dismiss` in both cases -- see "One control, not two" below
  for why.

### `frontend/src/ui/TransferringView.tsx`

`ProgressPresentation`'s three branches each gained a `.fd-progress-head`
wrapper that puts the percentage/status readout and `TransferMetrics` (wire
bytes, then throughput) above the track, and the known-positive percentage
span now carries `className="fd-progress-percent"` (the `{typography.numeric}`
treatment). `TransferMetrics` moved from a sibling of `ProgressPresentation`
into each mode's own head, so a `known-empty` transfer also shows its wire-byte
cell above the (decorative) track rather than after it. No selector, no prop,
and no accessible name changed; `TransferringView.test.tsx` needed no edits
(all 589 unit tests pass unmodified against this file).

### `frontend/src/style.css`

- Split the shared `.fd-pending-card, .fd-transfer-view` rule so the Progress
  Card (`.fd-transfer-view`) alone moved to `{rounded.xxl}` / `{elevation.sh-3}`;
  `.fd-pending-card` (Story 7.x's `StagePendingCard`, out of this story's
  scope) keeps its original `{rounded.lg}`, no shadow.
- `.fd-meter` height `10px` -> `8px` (functional boundary and `{rounded.full}`
  were already correct).
- `.fd-meter__fill` was already `background: var(--color-primary)` -- solid,
  no gradient. Confirmed by mutation (M-gradient below), not changed.
- New `.fd-progress-head`, `.fd-progress-percent` rules for the restructured
  head.
- Rebuilt `.fd-outcome`: centred, `{rounded.xxl}` / `{elevation.sh-3}` (was
  `{rounded.lg}`, a `currentColor` border, no shadow). `.fd-outcome__body`
  now `color: var(--color-muted)` (was `--color-text`) -- DESIGN.md's Outcome
  Panel row calls it "muted body."
- New `.fd-outcome__icon`/`--done`/`--error` (74px tinted discs) and
  `.fd-outcome__check`/`.fd-outcome__check-path`/`--drawn` (the stroke-drawn
  check: a plain `transition`, no keyframe rule anywhere in the sheet).
- New `.fd-receipt`/`__cell`/`__caption`/`__value` (two cells on
  `{colors.fill}`, divided by a `{colors.separator}` border between cells --
  DESIGN.md's Completion Receipt row).

### Tests

- `frontend/src/ui/OutcomePanel.test.tsx`: replaced the Story 7.4
  `data-receipt-*` assertions with assertions on the rendered receipt text;
  updated the icon test to check for the SVG check rather than a `✓` text
  node; added tests for the error `!` glyph, the unconditional post-mount
  draw, the primary-vs-quiet button weight, an explicit "never `role=alert`"
  sweep across all four outcome shapes (live/retained x Done/Error), and a
  dedicated "never a third cell or a duration figure" guard.
- `frontend/src/ui/styles.test.ts`: added `describe` blocks for the progress
  card/meter, the outcome panel, and the completion receipt (Story 7.5),
  following the existing per-story pattern (`the browse menu surface
  (Story 7.2)`, etc.). Moved the pre-existing `progress presentation` block
  next to the new progress-card block rather than leaving a duplicate
  `describe` of the same name in two places.

## Resolving the epics.md/DESIGN.md conflict on the fill

`epics.md`'s Story 7.5 AC1 says the determinate meter gets "a gradient fill."
`DESIGN.md`'s Progress Meter row and its Elevation & Depth section are
explicit and repeated: *"fill is **solid** `{colors.primary}`, not a
gradient -- the single-gradient rule is absolute and the button already
spends it"* and *"Exactly one gradient exists in the product: the primary
button's... The progress fill is therefore solid, not a gradient."*
`styles.test.ts`'s existing sheet-wide count (`gradients.toHaveLength(1)`,
already pinned by Story 7.2) makes a second gradient a hard failure
regardless of wording. This is the same class of defect commit `f2fe947`
("fix two spec defects Story 7.2 surfaced") already fixed once for this
epic. I implemented the fill as solid, per DESIGN.md, the task brief's
explicit instruction, and the existing test; `epics.md` AC1's "gradient
fill" wording is a spec defect that should be corrected the same way, and I
am flagging it rather than silently working around it.

## One control, not two: "primary next action, and Dismiss"

`epics.md` AC3 asks for "a primary next action, and Dismiss where the caller
supplies it." `OutcomePanel` has exactly one caller-supplied handler
(`onDismiss`), and `OutcomePanel.test.tsx` already pinned, before this story,
that only one control ever renders and that it is absent entirely when no
handler is supplied (`offers no control when the caller supplies no
handler`). I did not add a second control. Instead:

- A **live** terminal outcome (`App.tsx`'s own top-level `outcome`, not yet
  retained) has nothing else on screen to be a next action -- `phaseBody`
  renders nothing for a terminal phase, "the outcome panel above is its whole
  view" (comment already in `App.tsx`). Its one control therefore takes
  `fd-button--primary` weight and is, itself, the primary next action --
  clicking it is what forces `useTransfer.ts`'s `cancel()` to resolve the
  backend's terminal lease and return to Idle if the three-second reset never
  arrives (D-059, already implemented in a prior story).
- A **retained** outcome sits above Idle's own `copy.label.chooseFileOrFolder`
  browse control, which already *is* the next action once retained (routing
  table: "Starting Stage dismisses a retained outcome"). Its one control
  therefore only needs to be the quiet Dismiss beside it, so it takes
  `fd-button--quiet`.

No new copy string exists for a distinct "primary" label -- `EXPERIENCE.md`'s
copy table has exactly one outcome-control row, `copy.outcome.dismiss` =
"Dismiss" -- so both weights keep that same approved text and differ only in
visual weight (`fd-button--primary` vs `fd-button--quiet`), which is what the
new "gives the live control primary weight" / "adds Dismiss ... quiet"
tests in `OutcomePanel.test.tsx` pin.

## Files touched

- `frontend/src/ui/OutcomePanel.tsx`
- `frontend/src/ui/OutcomePanel.test.tsx`
- `frontend/src/ui/TransferringView.tsx`
- `frontend/src/style.css`
- `frontend/src/ui/styles.test.ts`

`frontend/src/ui/TransferringView.test.tsx` was read in full and needed no
edits -- every existing assertion (text content, roles, `aria-*`, className
substrings) still holds against the restructured markup, which is itself
evidence the restructure changed layout, not contract.

## Mutation table

Every mutation below was applied to the real working tree with a Python/`sed`
edit, run against the real suite (`npx vitest run --run <file>`, or the full
`npm test -- --run` where noted), confirmed to fail and name the problem,
then reverted from a pre-mutation-pass backup copy of the file
(`cp /tmp/<file>.bak <file>`), with a `diff` against that backup confirming a
byte-identical revert before moving to the next mutation. `git status --short`
after the whole pass showed only the five intended files modified.

| # | Mutation | AC / claim | Result |
|---|---|---|---|
| M-gradient | `.fd-meter__fill`'s `background: var(--color-primary)` -> `linear-gradient(var(--color-primary-hi), var(--color-primary))` | AC1 (solid fill, single-gradient rule) | KILLED -- 2 failures: the sheet-wide `pins the three elevation tokens and the single-gradient rule` (`expected [ ... ] to have a length of 1` -> 2), and the new `fills the track solid` (`expected '...' to contain 'background: var(--color-primary);'`) |
| M-reduced-check | Added `.fd-outcome__check-path { stroke-dashoffset: 32; }` inside the existing `@media (prefers-reduced-motion: reduce)` block | AC4 (the literal mutation the acceptance criterion names) | KILLED -- `draws the check via a transition, never a keyframe animation, and leaves it fully drawn under reduced motion`: `expected '...' not to contain 'stroke-dashoffset: 32'` |
| M-gate-draw | Gated the `useEffect`'s `setDrawn(true)` behind `if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return` | AC4 (the check must never be left undrawn, not even via JS) | KILLED -- `draws the check stroke once after mount, unconditionally` fails (the mutation also threw, since jsdom has no `matchMedia`, but the named test is in the failure list either way) |
| M-duration | Added a third `.fd-receipt__cell` rendering a literal "Duration"/"0:00" | AC3 (two cells, not three; no elapsed-time figure -- named as the worst possible outcome of this story) | KILLED -- 2 failures: `renders exactly two cells...` (`expected ...(3) to have a length of 2`) and `never renders a third cell or any elapsed-time figure` (same) |
| M-alert | Added `role="alert"` to the `<section>` | AC5 (no `role="alert"` in any form) | KILLED -- 4 failures, one per outcome shape (`never carries role="alert", for a live Done` / `a retained Done` / `a live Error` / `a retained Error`) |
| M-button-weight | Removed the `outcome.retained` conditional, hard-coding `fd-button--quiet` on both branches | AC3 ("a primary next action") | KILLED -- `gives the live control primary weight, not the quiet retained styling`: `expected '...' to contain 'fd-button--primary'` |
| M-meter-height | `.fd-meter`'s `height: 8px` -> `10px` | AC1 ("8px `{rounded.full}` track") | KILLED -- `is an 8px rounded.full track with a functional boundary`: `expected '...' to contain 'height: 8px;'` |
| M-card-elevation | `.fd-transfer-view`'s `border-radius: var(--radius-xxl); ... box-shadow: var(--shadow-sh-3);` -> `border-radius: var(--radius-lg);` (shadow removed) | AC1 (Progress Card, `{rounded.xxl}` at `{elevation.sh-3}`) | KILLED -- `is a rounded.xxl surface at sh-3...`: `expected '...' to contain 'border-radius: var(--radius-xxl);'` |

Not re-mutated (already mutation-proven by Story 7.4, unchanged by this
story): `receipt.bytesSent` is never read from `metadata.size` --
`OutcomePanel.tsx` has no `metadata` in scope at all, only
`OutcomePresentation`'s `receipt: CompletionReceipt`, so there is no field to
substitute; `state.test.ts`'s M-size (Story 7.4 evidence) already proves the
value is correct at its source. The pre-existing forced-colors, focus-ring,
and 320px-reflow guarantees were not touched by this story's diff and are
covered by their own already-passing, unmodified tests (confirmed by the full
run below, not re-mutated here to keep this pass scoped to what actually
changed).

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation pass
above and with the working tree back to its intended diff (`git status
--short` showed only the five files above; a `diff` against pre-mutation
backups confirmed each was byte-identical to its intended state before this
final run).

- `wails build`: **PASS**. `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 8.721s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change.
- Bindings mode drift: `App.d.ts`, `App.js`, `models.ts` came back `755` after `wails build`, as documented; `chmod 644` applied to all three. `git -c core.fileMode=false diff --quiet -- frontend/wailsjs` exit 0 (no drift).
- `scripts/verify-build-asset-drift.sh`: **PASS**, no output.
- `frontend/dist/.gitkeep`: present.
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `go env CGO_ENABLED`: `1`, confirmed before trusting the race run.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`internal/stream` ~105s under the race detector, the rest a few seconds each).
- `GOOS=windows GOARCH=amd64 go build ./...`: **PASS**.
- `GOOS=darwin GOARCH=arm64 go build ./...`: **PASS** (pre-flight).
- `GOOS=linux GOARCH=amd64 go build ./...`: **PASS** (pre-flight).
- `cd frontend && npm test -- --run` (not concurrent with `wails build`): **PASS** -- 17 files, **589 tests** (568 baseline on `epic-7-quartz` before this branch + 21 net new, measured directly by running the suite before and after this story's `OutcomePanel.test.tsx`/`styles.test.ts` changes -- new receipt, icon, draw-timing, button-weight and `role="alert"`-sweep cases in `OutcomePanel.test.tsx`, new progress-card/meter/outcome/receipt cases in `styles.test.ts`, minus the one duplicate `progress presentation` block removed).
- `cd frontend && npm run test:browser`: **PASS** -- 1 file, 17 tests, unchanged.
- `git diff --check`: **PASS**, no output (no whitespace errors).
- Line-ending check (`git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'`): **PASS**, no matches (grep exit 1).

## Nothing left open

All six Story 7.5 acceptance criteria are implemented and mutation-verified
above:

1. Determinate transfer: percentage in `{typography.numeric}` with tabular
   numerals (`.fd-progress-percent`), wire bytes first and throughput second
   beside it (`TransferMetrics` inside `.fd-progress-head`), above an 8px
   `{rounded.full}` track with a functional boundary (M-meter-height) and a
   **solid** fill (M-gradient) -- see "Resolving the epics.md/DESIGN.md
   conflict on the fill" above for why solid, not gradient, is correct.
2. Unknown-total and known-empty presentations preserved exactly: the static
   unknown pattern and decorative known-empty track are unchanged code paths,
   still covered by the untouched, still-passing `TransferringView.test.tsx`.
3. Done outcome: success disc with stroke-drawn check, heading, body, the
   two-cell receipt (M-duration), a primary next action, and Dismiss where
   the caller supplies it (M-button-weight). D-059's live control still
   carries through unchanged.
4. `prefers-reduced-motion: reduce` leaves the check fully drawn
   (M-reduced-check, M-gate-draw -- the exact mutation the criterion names).
5. Error outcome: same composition, `!` glyph, heading/message read from the
   fixed registry by code (unchanged, pre-existing structural guarantee, still
   covered), no raw diagnostics, no `role="alert"` in any form (M-alert, all
   four outcome shapes).
6. Focus routing: `data-focus-target`, `tabIndex={-1}`, and the no-ring rule
   are unchanged by this story's diff (`OutcomePanel.tsx`'s `<section>` props
   were not touched) and remain covered by their own pre-existing, still-
   passing tests.
7. The full gate passes -- see transcripts above.
