# Evidence: Story 7.8: Return the Action Colour to the Logo, and Put the Action First

## Summary

Two changes, both owner-directed on 2026-09-20 after driving the built Quartz
binary:

1. **The accent returns to the logo's mocha.** Quartz's first cut took the
   action colour to system blue on the reasoning that blue reads native on
   both platforms. Five tokens move to the published copper-mocha values
   sampled from `build/appicon.png`: `--color-primary`, `--color-primary-hi`,
   `--color-primary-hover`, `--color-primary-ink`, and `--color-focus` (light
   and dark halves).
2. **Idle's rows are reordered.** New document order: drop zone,
   command-failure panel (if any), **browse control**, firewall preflight
   disclosure, recovery disclosure. `BrowseControl`'s internal behaviour
   (Escape, Tab, arrow, blur handling) is untouched — only the two JSX nodes
   were moved in `IdleView.tsx`.

## Review follow-up: `--color-primary-tint` was also blue

The first pass of this story left `--color-primary-tint` unchanged
(`#E6EFFB` / `#23303F`) because it is not one of the five tokens the story's
table names, and the acceptance criteria bind to exactly those five. That
reading of the contract was defensible, but it was **incomplete against the
owner's actual intent** — "replace the blue accents with the logo's mocha" —
which the review caught: `primary-tint` sits directly against mocha in two
places (`.fd-drop-zone.wails-drop-target-active .fd-drop-zone__inner`'s
drag-active fill, inside a now-solid-mocha border; `.fd-browse-menu
.fd-button:focus-visible`'s tint halo, around a now-mocha-filled item), so a
leftover blue wash there read as a bug rather than a design choice. This
section records the fix; the token table and contrast tables below reflect
the final state, not the first-pass one.

**New values:** `--color-primary-tint` light `#F5EEEB` (mocha at 10% on
`--color-surface`), dark `#372E2B` (mocha at 12% on `--color-surface-dark`).
The drag-active fill **carries text** — the drop zone's heading and meta line
both render on it while a drag is over it — so both `text` and `muted` on
`primary-tint` were checked, not only assumed safe because the hue matched.
The dark fraction is deliberately 12%, not 16%: 16% (`#3E332E`, mocha mixed at
16% over `--color-surface-dark`) put `muted` on it at **4.485388049**, under
the 4.5:1 floor — confirmed by mutation below. `primary` on `primary-tint`
(the drag-active border/fill boundary pair) was also added to the proof.

## Focus moves off the action colour

`--color-focus` was equal to `--color-primary` before this story (both
`#0A6CD8` in light mode). With a mocha accent, a mocha focus ring on a button
already filled with mocha is not an indicator, so focus becomes a dedicated
violet: `#6B4E9E` light / `#B79BE0` dark — distinct from `primary` in both
modes, and clearing its own floors (≥3:1 against every surface it sits on; see
the contrast tables below).

## Token values (final)

| Token | Light | Dark |
|---|---|---|
| `primary` | `#9C5636` | `#E39B70` |
| `primary-hi` (gradient top stop only) | `#B06A45` | `#EBAA82` |
| `primary-hover` | `#7F4428` (darker than primary) | `#F0B694` (lighter than primary) |
| `primary-ink` | `#FFFFFF` | `#2B1206` |
| `focus` | `#6B4E9E` | `#B79BE0` |
| `primary-tint` (review follow-up, not in the original five) | `#F5EEEB` | `#372E2B` |

The five story-table values match the story's published table exactly. No
value was adjusted during implementation — every floor cleared on the first
check (see "What was re-derived" below). `primary-tint` was added by the
review follow-up above; its dark fraction (12%) was chosen specifically
because a higher one (16%) failed a floor — see the mutation table.

## What was re-derived (never hand-computed)

Every figure below was recomputed with the exact formula
`styles.test.ts` uses (`frontend/src/ui/styles.test.ts`, "the unrounded
contrast proof") against the tokens actually declared in `style.css`, then
copied verbatim into DESIGN.md and into this evidence file. Nothing here was
hand-adjusted; the `it('publishes every figure it proves, unrounded, in
DESIGN.md')` test enforces this by re-deriving the same figures from the
stylesheet and asserting DESIGN.md contains each one.

Text pairs (≥4.5:1), changed rows only — everything not listed here (text/
muted/error/status-on-surface) is untouched because none of those tokens
moved:

| Text pair | Light ratio | Dark ratio |
|---|---:|---:|
| `primary-ink` on `primary` | 5.529272927 | 7.709467487 |
| `primary-ink` on `primary-hover` | 7.608987041 | 9.920419953 |
| `text` on `primary-tint` (review follow-up) | 15.153268673 | 11.826910776 |
| `muted` on `primary-tint` (review follow-up) | 5.048617711 | 4.851652072 |

Load-bearing non-text pairs (≥3:1, unrounded), changed rows only:

| Pair | Light ratio | Dark ratio |
|---|---:|---:|
| `primary` on `track` | 4.560313844 | 4.954296293 |
| `primary` on `surface` | 5.529272927 | 7.193529288 |
| `primary` on `elevated` | 5.529272927 | 6.510527964 |
| `focus` on `elevated` | 6.544828832 | 6.219794189 |
| `primary` on `primary-tint` (review follow-up) | 4.821510572 | 5.785865409 |

Weakest-adjacent claim for focus (published as a floor, not a full row per
surface): the weakest of `focus` against `canvas`/`surface`/`elevated` is
`focus` on `canvas` at **5.853767247** (light) — `canvas` is still the
weakest adjacent surface with the new violet token, same as when focus
equalled primary. (Full light-mode figures: canvas 5.853767247, surface
6.544828832, elevated 6.544828832.)

Weakest-of-status-on-canvas claim (unaffected — status tokens did not move):
success at **4.799052371** light, error at **7.116437439** dark — unchanged
from the pre-Story-7.8 published figures, confirmed by recomputation, no edit
needed to that sentence in DESIGN.md.

Status-on-surface floor (unaffected): still exceeds **5.36:1** light and
**6.47:1** dark — confirmed by recomputation, no edit needed.

`qr-ink`/`qr-surface` fixed substrate (unaffected): **17.377657264** in both
modes — confirmed unchanged.

## Mutation table

Every load-bearing claim below was broken on purpose, confirmed to fail and
name the break, then reverted. Working copies of the pre-mutation files were
kept at `/tmp/mutations/*.orig` during the session for exact reverts; `git
status`/`git diff --stat` confirm only the intended five files differ from
`origin/epic-7-quartz` afterward.

| # | Claim (acceptance criterion) | Mutation | Result |
|---|---|---|---|
| 1 | Focus is distinct from primary | Set `--color-focus: #9C5636` (== `--color-primary`) in the light `@theme` block | **Failed, naming the collapse.** 4 tests failed: `the Quartz token layer > declares every Quartz light value...` (token-string mismatch), `the focus indicator is distinct from the action colour (Story 7.8) > never lets focus collapse onto primary in either mode` — `AssertionError: focus must not collapse onto primary in light mode: expected '#9C5636' not to be '#9C5636'` (the assertion this AC's mutation directly targets), plus the "declares the published Story 7.8 mocha and violet values exactly" and "keeps the weakest-adjacent claim..." tests, which depend on the correct token. Reverted; `npm test -- --run` green (617). |
| 2 | Every published contrast figure is re-derived, never hand-edited | Changed `primary` on `track`'s published DESIGN.md figure from `4.560313844` to `4.560313845` (chosen because this value, unlike `primary-ink`/`primary`, is not numerically coincident with any other published row) | **Failed, naming the pair.** `the unrounded contrast proof > publishes every figure it proves, unrounded, in DESIGN.md` — `AssertionError: primary/track = 4.560313844: expected [DESIGN.md] to contain '4.560313844'`. Reverted; suite green. |
| 3 | Hover direction: light darkens on hover | Set light `--color-primary-hover: #B06A45` (== `primary-hi`, lighter than `primary` — wrong direction) | **Failed, naming the pair.** `primary-ink on primary-hover clears its AA ratio in both authored modes` — `expected 4.205408882453538 to be greater than 4.5`; `gives the hover fill its own direction-correct token...` — `primary-ink on primary-hover (light): expected 4.205408882453538 to be greater than 4.5`; plus the `publishes every figure...` test naming `primary-ink/primary-hover = 4.205408882`. Reverted; suite green. |
| 3b | Hover direction: dark lightens on hover | Set dark `--color-primary-hover: #B06A45` (darker than dark `primary` — wrong direction) | **Failed, naming the pair.** Same three tests, this time naming `(dark)`: `expected 4.189690959296328 to be greater than 4.5`. Reverted; suite green. |
| 4 | Idle document order: browse control ahead of both disclosures | Moved the `<BrowseControl>` JSX node in `IdleView.tsx` back to after the firewall `<Disclosure>` (its pre-Story-7.8 position) | **Failed, naming the wrong order.** `IdleView.test.tsx`: `Idle at rest > leads with the drop target, puts the browse control ahead of both disclosures...` — `expected [ Array(4) ] to deeply equal [ Array(4) ]` (order mismatch: `fd-drop-zone, fd-preflight, fd-selection, fd-help` instead of `fd-drop-zone, fd-selection, fd-preflight, fd-help`); and `the firewall preflight is a collapsed disclosure > follows the browse control (Story 7.8)...` — `expected [ 'fd-preflight', 'fd-selection' ] to deeply equal [ 'fd-selection', 'fd-preflight' ]`. Reverted; `IdleView.test.tsx` green (49/49). |
| 5 (review follow-up) | `primary-tint` is mocha, not the leftover blue wash | Reverted light `--color-primary-tint` to `#E6EFFB` | **Failed, naming the pair.** `the Quartz token layer > declares every Quartz light value...` — token-string mismatch; `primary-tint carries text (Story 7.8 review follow-up) > is mocha, not the leftover blue wash...`; `the unrounded contrast proof > publishes every figure it proves, unrounded, in DESIGN.md` — `AssertionError: text/primary-tint = 14.981366511: expected [DESIGN.md] to contain '14.981366511'` (the recomputed figure for the reverted blue no longer matches the published mocha figure). Reverted; suite green. |
| 5b (review follow-up) | `primary-tint`'s dark fraction keeps `muted` above 4.5:1 | Set dark `--color-primary-tint: #3E332E` (mocha mixed at the rejected 16% fraction, computed by hand to confirm it reproduces the review's finding before mutating) | **Failed, naming the pair.** `primary-tint carries text... > keeps muted readable on the drag-active fill...` — `AssertionError: muted on primary-tint (dark): expected 4.485388049041351 to be greater than 4.5`; `the unrounded contrast proof > muted on primary-tint clears its AA ratio in both authored modes` — same figure; `publishes every figure...` — naming `text/primary-tint = 10.934066060` (the recomputed, now-wrong figure). Reverted; suite green. |

Mutation 4 also stands as the proof for the focus-order acceptance criterion
("leave a disclosure ahead of the control in the DOM -> must fail"): Idle has
no separate script-driven tab order, so DOM order *is* Tab order here, and the
same mutation that breaks the document-order test breaks the tab-order claim
identically.

Mutations 5 and 5b are the review follow-up's own required mutations ("set
`primary-tint` back to a blue value, or to a mocha fraction that puts `muted`
under 4.5 -> must fail naming the pair"). 5b's rejected value was independently
computed (mocha mixed at 16% over `--color-surface-dark`, by the same
channel/luminance/contrast formula `styles.test.ts` uses) before being applied
as a mutation, to confirm it reproduces the review's stated finding (4.49:1)
rather than merely asserting the review was right.

## Existing tests modified, with justification

- `frontend/src/ui/styles.test.ts`
  - `declares every Quartz light value...` / `declares the authored dark half...`:
    literal hex values updated from the old blue palette to the Story 7.8
    mocha/violet palette (this *is* the pin the acceptance criteria require —
    the assertion shape is unchanged, only the literals move, matching
    DESIGN.md's frontmatter).
  - `narrows the dark primary-hi delta without disturbing any published
    contrast figure` (Story 7.7's test): the literal `#5EA6FF` it pinned was
    itself superseded by Story 7.8's mocha value `#EBAA82`. Updated the
    expected literal and added an explanatory comment noting the supersession;
    the property under test (primary-hi never silently drifts out of sync
    with DESIGN.md) is unchanged and still enforced against the new value.
  - Added two new tests under a new `describe('the focus indicator is
    distinct from the action colour (Story 7.8)')` block: one asserting
    `focus !== primary` in both modes (the AC's own mutation target, listed
    above as #1), and one pinning the five published Story 7.8 token values
    directly by literal string.
  - Added `text`/`primary-tint` and `muted`/`primary-tint` (published) and
    `primary`/`primary-tint` (published) rows to the `placed` contrast-proof
    array (review follow-up) — these ride the existing `it.each` machinery
    rather than needing bespoke assertions.
  - Added a new `describe('primary-tint carries text (Story 7.8 review
    follow-up)')` block (review follow-up): one test pinning the two
    `primary-tint` literals directly and rejecting the old blue values, one
    test independently recomputing `muted` on the dark token via the same
    luminance/contrast formula as `styles.test.ts`'s own proof, so it would
    catch any future fraction that repeats the rejected-16% mistake, not only
    the one hex this repo rejected.
- `frontend/src/ui/IdleView.test.tsx`
  - `Idle at rest > leads with the drop target, keeps the preflight ahead of
    the browse controls...`: renamed and its expected order changed from
    `['fd-drop-zone', 'fd-preflight', 'fd-selection', 'fd-help']` to
    `['fd-drop-zone', 'fd-selection', 'fd-preflight', 'fd-help']` — this is
    the direct assertion of the acceptance criterion's new document order.
  - `the firewall preflight is a collapsed disclosure > precedes the browse
    control, same as the always-open preflight did`: renamed and its expected
    order reversed to `['fd-selection', 'fd-preflight']` — same reasoning.
  - `renders the outcome panel between the drop zone and the preflight
    disclosure` (command-failure test) was **left unchanged**: its query only
    selects `.fd-drop-zone, .fd-outcome, .fd-preflight`, and the browse
    control (`.fd-selection`) is not one of those three classes, so its
    relative order (`fd-drop-zone`, `fd-outcome`, `fd-preflight`) is
    unaffected by moving the browse control — verified, not assumed, by
    running the full IdleView suite green with no further edits needed there.

## FR23 — recorded in DESIGN.md

`_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`'s
"FR23 and the disclosures" section was rewritten to describe the second
weakening: the firewall preflight no longer precedes the selection control at
all. The trade is recorded honestly (a first-time sender can reach the picker
without passing the firewall guidance) alongside what still holds (guidance
present, one control away, named by its summary, reachable before the OS
prompt). The "Vertical composition (Story 7.7)" section's prose describing
Idle's controls ("the disclosures, the browse control") was also corrected to
the new order, and the stale "one blue reserved for the next action" / "focus
token equals primary" sentences in the Brand & Style and Colors sections were
updated to describe the mocha/violet palette.

## `main.go` / `main_test.go` — verified, not assumed

Per the story's explicit warning (the same assumption was wrong in Story
7.1): `--color-canvas` is untouched by this story (`#F2F2F4` light /
`#161618` dark in both `style.css` and `main_test.go`'s pinned literals).
Confirmed by `grep` before touching anything and by the full `go test` and
`-race` runs passing unmodified — no edit was made to `main.go` or
`main_test.go`.

## Forced colors

No new `@theme` token was introduced (all six changed tokens, including the
review follow-up's `primary-tint`, already existed and already had
system-colour overrides: `primary`/`primary-hi`/`primary-hover` →
`Highlight`, `primary-ink` → `HighlightText`, `focus` → `Highlight`,
`primary-tint` → `Canvas`). `styles.test.ts`'s completeness test (`supersedes
Quartz with system colours on every authored token`) passed unmodified,
confirming no token was left uncovered. The gradient and shadow exemptions
are unaffected — still exactly one gradient, three shadow tokens.

## Full gate (run natively on macOS, in verify.yml order)

Run twice: once for the initial story implementation (five action tokens,
Idle reorder), once more after the review follow-up (`primary-tint`). Both
runs green; figures below are from the final (post-follow-up) run. All
commands were run from the repository root unless noted, with Go and Wails
on `PATH` (`~/go/bin`, `/opt/homebrew/bin`).

1. `wails build` — **passed.** Regenerated `frontend/wailsjs/go/{main/App.d.ts,main/App.js,models.ts}`
   at mode 755 each time; `chmod 644` applied to all three after every build.
   `git diff --stat frontend/wailsjs` / `git status --short frontend/wailsjs`
   — no diff after each chmod (content identical to the committed copies).
2. `gofmt -l .` — no output (clean).
3. `go vet ./...` — no output (clean).
4. `go tool staticcheck ./...` — no output (clean).
5. `go test -count=1 ./...` — all packages `ok`
   (`fairdrop`, `fairdrop/internal/network`, `fairdrop/internal/qr`,
   `fairdrop/internal/server`, `fairdrop/internal/source`,
   `fairdrop/internal/stream`, `fairdrop/internal/transfer`,
   `fairdrop/scripts`, `fairdrop/scripts/mutationverdict`).
6. `CGO_ENABLED=1 go test -count=1 -race ./...` (confirmed `go env
   CGO_ENABLED` prints `1` first) — all packages `ok`, including
   `fairdrop/internal/stream` at ~102-103s under the race detector.
7. `cd frontend && npm test -- --run` — **622 passed** (615 pre-existing +
   2 from the initial pass + 5 from the review follow-up), 17 files, no
   failures.
8. `cd frontend && npm run test:browser` — **17 passed**, 1 file, no
   failures.
9. `GOOS=windows GOARCH=amd64 go build ./...` — passed.
10. Pre-flight (not part of verify.yml, but required before pushing):
    `GOOS=darwin GOARCH=arm64 go build ./...`,
    `GOOS=linux GOARCH=amd64 go build ./...`, and
    `GOOS=darwin GOARCH=arm64 staticcheck ./...` — all passed.

`wails build` was never run concurrently with either frontend suite; the Go
gate, `wails build`, and the frontend suites were run strictly one after
another, per AGENTS.md.

## Files changed

- `frontend/src/style.css` — six action tokens (light + dark: the five
  story-table tokens plus `primary-tint` from the review follow-up), new
  comments explaining the Story 7.8 mocha/violet change, superseding the old
  blue-specific narrowing comment, and recording why the dark `primary-tint`
  fraction is 12% rather than 16%.
- `frontend/src/ui/IdleView.tsx` — reordered `<BrowseControl>` ahead of the
  firewall `<Disclosure>`; updated comments claiming the old order (the
  module doc comment, the drop-zone comment, and the disclosure comment).
  `BrowseControl`'s own function body (Escape/Tab/arrow/blur handling) is
  byte-for-byte unchanged.
- `frontend/src/ui/IdleView.test.tsx` — two tests' expected order updated to
  match the new document order (see "Existing tests modified" above).
- `frontend/src/ui/styles.test.ts` — light/dark palette literals updated
  (including `primary-tint` from the review follow-up); the Story 7.7
  `primary-hi` pin updated to its Story-7.8-superseded value; `text`/
  `muted`/`primary` × `primary-tint` rows added to the `placed` contrast
  array; new `describe` blocks for the focus/primary distinctness mutation
  and literal token pin, and for the `primary-tint` mocha/blue and
  dark-fraction mutations.
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md` —
  frontmatter `colors` block (six light + six dark values, including
  `primary-tint`), the two derived contrast tables (changed rows only, plus
  the new `primary-tint` rows and their explanatory paragraph), the focus
  weakest-adjacent prose, the "FR23 and the disclosures" section (second
  weakening recorded), the Story 7.7 vertical-composition prose (order
  corrected), and the Brand & Style intro (blue → accent, with a pointer to
  the Colors section).

## Things worth flagging

- **Resolved by review follow-up:** the initial pass deliberately left
  `--color-primary-tint` at its pre-Story-7.8 blue-ish value (`#E6EFFB` /
  `#23303F`) because it is not one of the five tokens the story's table
  names, and the acceptance criteria bind to exactly those five. That reading
  of the written contract was reasonable on its own terms, but it was
  **incomplete against the owner's stated intent** — "replace the blue
  accents with the logo's mocha" is broader than the five-token list, and
  `primary-tint` sits directly against mocha in two places. The coordinator
  caught this at review; it is fixed in this evidence file's "Review
  follow-up" section above, with its own mutation coverage and re-derived
  contrast figures. Recorded here as the general lesson: when a story names
  specific tokens but the owner's stated intent is broader, flag the gap
  immediately (as the first pass did) rather than treating the narrow
  reading as final.
- No published figure failed a floor at any point in the initial pass —
  every value in the story's token table cleared both WCAG floors on the
  first computation. The review follow-up's dark `primary-tint` did fail a
  floor once, at the first fraction tried (16%, `muted` at 4.485:1) — the
  coordinator's own check, confirmed independently above rather than taken
  on faith, and resolved by narrowing to 12% before any value shipped.
