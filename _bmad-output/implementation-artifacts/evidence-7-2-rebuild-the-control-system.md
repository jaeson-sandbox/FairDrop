# Evidence: Story 7.2: Rebuild the Control System

## What changed

`frontend/src/style.css`, component rules only -- no token added, renamed, or
removed, per scope:

- **`.fd-button` (base):** now `display: inline-flex; align-items: center;
  justify-content: center;` and `block-size: var(--spacing-control-height)`
  (40px), plus a shared `transition` (transform/background-color/border-color,
  120-150ms, the one `--ease-decelerate` curve) and a new
  `.fd-button:active { transform: scale(0.975); }` rule -- DESIGN.md's Motion
  section makes the press scale general to every button, not only the
  primary one, so it is one rule rather than one per variant.
  `prefers-reduced-motion` neutralises the transition through the existing
  universal `*` rule; no new reduced-motion assertion was needed.
- **`.fd-button--primary`:** gained `var(--shadow-sh-1)` ahead of its existing
  1px inset highlight in the same `box-shadow` declaration -- see "A token the
  spine doesn't name," below, for why `sh-1` and not a dedicated `sh-btn`.
- **`.fd-button--secondary` (new):** `background: var(--color-fill-strong)`.
  No border declaration of its own -- the functional boundary stays on the
  shared `.fd-button` base rule, which is the mechanism the acceptance
  criterion's mutation targets. `--color-fill-strong` against `--color-surface`
  measures 1.2236401171551334:1 (computed with the same formula
  `styles.test.ts` uses elsewhere), matching the AC's "1.22:1" figure and
  confirming it names this token rather than `--color-fill` (1.13:1). Applied
  to `StagedView.tsx`'s "Show full name" toggle, the one existing plain
  (unmodified) button whose role -- a persistent secondary action beside the
  primary heading -- matches the variant; no other file in this story's scope
  needed a new class.
- **`.fd-button--copied`:** gained `background: var(--color-success-tint)` --
  DESIGN.md's Copy Feedback row calls for "the success tint and boundary,"
  and only the boundary (border-color) was previously implemented.
- **`.fd-url`:** background moved from `--color-surface` to `--color-fill`
  (DESIGN.md's Direct URL Row: "on `{colors.fill}`"), and `border-radius`
  moved from `--radius-sm` to `--radius-md` (the Shapes section assigns
  `{rounded.md}` to "buttons and fields," not `{rounded.sm}`, which is
  reserved for menu items).
- **`.fd-browse-menu`:** `border-radius` moved from `--radius-md` to
  `--radius-lg`, and it gained `box-shadow: var(--shadow-sh-3)` and
  `padding: 5px` (a literal, matching DESIGN.md's literal "5px padding" --
  not a step on the 4/8 scale). The functional boundary it already carried
  from Story 7.1 is unchanged.
- **`.fd-browse-menu .fd-button` (menu items):** gained
  `border-radius: var(--radius-sm)` and `border-color: transparent; background:
  transparent` at rest (the outer menu's own boundary already bounds the
  surface; a second border per item read as redundant chrome), a `:hover`
  fill, and a `:focus-visible` rule that paints the primary fill, white ink,
  and a `0 0 0 4px var(--color-primary-tint)` tint halo -- on top of, not
  instead of, the shared `.fd-button:focus-visible` ring every Tab-reachable
  control already paints.

`frontend/src/ui/StagedView.tsx`: one className edit (`fd-name-toggle`
button gained `fd-button--secondary`). No JSX structure, handler, or markup
changed anywhere else -- `IdleView.tsx`, `TransferringView.tsx`,
`OutcomePanel.tsx`, and `StagePendingCard.tsx` are untouched.

`frontend/src/ui/styles.test.ts`: seven new tests across three new `describe`
blocks (`the button family (Story 7.2)`, `the browse menu surface (Story
7.2)`, `the copy control takes the success tint (Story 7.2)`), plus one
assertion added to the existing `pins the three elevation tokens and the
single-gradient rule` test -- see "A gap this mutation found," below. No
existing assertion was weakened, loosened, or deleted.

## What was deliberately left alone

- **The browse control's own trigger button** (`IdleView.tsx`) keeps its
  plain, unmodified `.fd-button` class -- DESIGN.md's Components table calls
  it a "full-width primary button," but this story's scope is "the `.fd-button`
  family... and the browse menu surface," not the browse control itself, and
  `IdleView.tsx` is Story 7.3's named file. `IdleView.test.tsx` already
  asserts `expect(control.className).not.toContain('fd-button--primary')`
  with no rationale comment attached; changing that assertion would have been
  outside the "existing suites pass unchanged" contract for a change that
  belongs to a later story anyway. Flagging this for Story 7.3: the trigger
  needs `fd-button--primary` and a trailing chevron glyph to satisfy the
  Components table in full, and that existing negative assertion will need to
  invert when it does.
- **`OutcomePanel.tsx`'s Dismiss button** stays plain `.fd-button` rather than
  `fd-button--quiet`, even though DESIGN.md's Outcome Panel row calls Dismiss
  "quiet." `OutcomePanel.tsx` is named as Story 7.5's scope file; changing it
  here would pre-empt that story's own acceptance criteria and evidence.
  Flagging it so 7.5 doesn't miss it.
- **`.fd-drop-zone`, `.fd-packet`, `.fd-metric`, `.fd-outcome`, `.fd-meter`**
  and every other selector outside the button family, focus ring, URL field,
  copy feedback, and browse menu surface: untouched, per scope.

## A token the spine doesn't name

The acceptance criterion for the primary control asks for `{elevation.sh-btn}`.
DESIGN.md's frontmatter and its Elevation & Depth section declare exactly
three steps -- `sh-1`, `sh-2`, `sh-3` -- and describe them as "a resting
surface," "a raised surface the user is acting on," and "the focal object of
the current lifecycle state." None is named for a button, and no fourth token
exists to add without reopening Story 7.1's already-verified token layer
(out of this story's scope, and `theme.match(/--shadow-sh-1:/g)).toHaveLength(1)`-style
assertions there would need touching for a `sh-btn` token that appears
nowhere else in the design).

I resolved this as `sh-1`: the lightest step, matching "a resting surface"
better than the other two, and the only one that leaves room in the
three-shadow-layer ceiling once combined with the button's existing 1px inset
highlight (`sh-1`'s two layers + the highlight = 3, exactly at the limit;
`sh-2` or `sh-3` would have pushed it over). This is a judgment call, not a
literal reading of the spine, and it is the one place in this story where the
acceptance criterion's wording and DESIGN.md's own token table don't agree.
Flagging it rather than silently picking a value: the spine should either add
a named `sh-btn` step (if a button is meant to read as elevated above resting
surfaces) or the epics.md acceptance criterion should say `sh-1` explicitly.

## Mutation table

Each mutation was applied to the real `frontend/src/style.css` via a small
Python harness (`/tmp/mutate.py`, not committed) that swaps in the mutated
text, runs `./node_modules/.bin/vitest run --run src/ui/styles.test.ts`
against the real suite, and restores the clean file before the next case --
run automatically for all twelve cases in one pass, twice (the second pass
confirms the fix described in "A gap this mutation found" below).

| # | Mutation | AC | Result |
|---|---|---|---|
| M1 | removed `border: 1px solid var(--color-control-border);` from the shared `.fd-button` base rule (secondary keeps its fill) | AC2 | KILLED -- `.fd-button draws its boundary with the functional token, not the decorative one` **and** `gives the secondary control a fill on top of the shared boundary, never instead of it`; 2 tests failed |
| M2 | `.fd-browse-menu`'s `border-radius` changed from `var(--radius-lg)` to `var(--radius-md)` | AC4 | KILLED -- `is a rounded.lg surface at sh-3 with a functional boundary` |
| M3 | `.fd-browse-menu`'s `box-shadow: var(--shadow-sh-3);` deleted | AC4 | KILLED -- same test |
| M4 | `.fd-button--primary`'s `box-shadow` reverted to drop `var(--shadow-sh-1)`, keeping only the inset highlight | AC1 | KILLED -- `gives the primary control its elevation token, a 40px height token and the press scale` |
| M5 | `.fd-button`'s `block-size: var(--spacing-control-height);` deleted | AC1 | KILLED -- same test |
| M6 | `.fd-button:active { transform: scale(0.975); }` emptied | AC1 | KILLED -- same test |
| M7 | `.fd-button--secondary` given `border: none;` alongside its fill (the AC's own named mutation: "remove the boundary and keep the fill") | AC2 | KILLED -- `gives the secondary control a fill on top of the shared boundary, never instead of it` |
| M8 | `.fd-button--copied`'s `background: var(--color-success-tint);` deleted | AC5 | KILLED -- `paints the fill, not only the border and text` |
| M9 | `.fd-browse-menu .fd-button`'s `border-radius` changed from `var(--radius-sm)` to `var(--radius-md)` | AC4 | KILLED -- `gives menu items the sm radius` |
| M10 | `.fd-browse-menu .fd-button:focus-visible` emptied (no fill, no halo) | AC4 | KILLED -- `distinguishes the focused item by primary fill and a tint halo, not the ring alone` |
| M11 | a second real `linear-gradient(...)` added to `.fd-button--primary`'s background | AC1 (spine-wide gradient rule) | KILLED -- `pins the three elevation tokens and the single-gradient rule` |
| M12 | a fourth shadow layer added *inside* the `--shadow-sh-1` token definition | AC1 / spine-wide 3-layer rule | Initially SURVIVED -- see "A gap this mutation found." KILLED after the fix, both counted independently (2 layers from the token declaration, 2 pieces from the `.fd-button--primary` box-shadow declaration -- neither individually over 3) and via the new resolved-composite assertion (3 real layers from the token + 1 inset = 4, over 3) |

Full harness output for both passes retained at
`/tmp/mutation-log-7.2.txt` on this machine (not committed; ephemeral per the
scratchpad convention). `style.css` was diffed against its pre-mutation copy
after the run and confirmed byte-identical to the intended committed state.

## A gap this mutation found

`.fd-button--primary` is the one place in the whole sheet where a
`--shadow-sh-*` token (via `var()`) and an additional literal shadow layer
(the 1px inset highlight) sit in the same `box-shadow` declaration -- every
other user of a `--shadow-sh-*` token (`.fd-packet`, now also
`.fd-browse-menu`) uses the token alone. The existing "no more than three
shadow layers" check (Story 7.1) counts a literal `box-shadow:` declaration's
own comma-separated pieces, and separately counts each `--shadow-sh-*` token
definition's own pieces -- but it never resolves one against the other. A
fourth layer added inside `--shadow-sh-1` therefore read as "2" from the
token side and "2" from the declaration side (`var(--shadow-sh-1)` counts as
one opaque piece, `inset ...` as the other), both comfortably under the
ceiling, while the browser would really composite 3 (from the grown token) +
1 (the inset) = 4 layers on that one control -- exactly the defect this
story's binding constraint forbids ("No element may carry more than three
shadow layers... the check inspects both literal `box-shadow:` declarations
and the `--shadow-sh-*` token definitions they reference through `var()`").

Fixed by adding a third assertion to the same test that resolves
`.fd-button--primary`'s `var(--shadow-sh-1)` reference against the token's
own current definition (string substitution, not a CSS parser -- consistent
with the rest of the file's approach) and counts the real composited layer
total. M12 was re-run afterward and killed. This is the story's own "mutation
is the acceptance bar" principle applied to a test this story added to, not
one it wrote from scratch -- the gap existed in Story 7.1's version of the
check too, but nothing before this story ever combined a shadow token with an
extra literal layer, so nothing had exercised it.

## Existing suites: unchanged, verified

`IdleView.test.tsx`, `StagedView.test.tsx`, `TransferringView.test.tsx`, and
`OutcomePanel.test.tsx` were run in isolation after all CSS and the one
`StagedView.tsx` className edit: **121/121 pass, zero files touched in
`OutcomePanel.tsx`, `IdleView.tsx`, or `TransferringView.tsx`, and the sole
edit to `StagedView.tsx` (adding `fd-button--secondary` to the name-toggle
button's className) required no test change** -- `StagedView.test.tsx` never
asserted that button's className was exactly `fd-button fd-name-toggle
fd-target`, only that it contained `fd-target` (line 495), so the added
modifier class is additive from the test's point of view. No test in any of
the four suites asserts a literal colour or radius that Quartz's component
rules changed, so nothing needed loosening or updating -- the "except where a
test asserts a literal colour or radius" carve-out in the story brief was not
exercised.

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above (working tree returned to its intended committed state first).
Go 1.27.0, Node via `npm test`, Wails CLI 2.15.0, `CGO_ENABLED=1`.

- `wails build`: **PASS**. `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 8.065s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change and present before it (also noted in Story 7.1's evidence).
- `frontend/wailsjs/` permissions: `App.d.ts`, `App.js`, and `models.ts` were regenerated at `644` by this `wails build` (verified with `ls -la`), so no `chmod` was needed before committing; `git status` shows no diff under `frontend/wailsjs/` (the exported `App` command surface did not change).
- `gofmt -l .`: **PASS**, no output.
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `CGO_ENABLED=1 go test -count=1 -race ./...` (`go env CGO_ENABLED` confirmed `1` first): **PASS** -- `ok` for all nine packages (`internal/stream` ~105s under the race detector, the rest a few seconds each).
- `GOOS=windows GOARCH=amd64 go build ./...`: **PASS**.
- `GOOS=darwin GOARCH=arm64 go build ./...`: **PASS** (pre-flight, per the documented AGENTS.md pitfall).
- `GOOS=linux GOARCH=amd64 go build ./...`: **PASS** (pre-flight).
- `cd frontend && npm test -- --run` (not concurrent with `wails build`): **PASS** -- 17 files, **555 tests** (Story 7.1 left 549; this story added 7: 6 new `it` blocks across the three new `describe` blocks, plus 1 new assertion appended inside an existing `it`, which does not add to the count).
- `cd frontend && npm run test:browser`: **PASS** -- 1 file, 17 tests.
- `IdleView.test.tsx`, `StagedView.test.tsx`, `TransferringView.test.tsx`, `OutcomePanel.test.tsx` run in isolation: **PASS** -- 121/121, unchanged from before this story.

`GOOS=darwin GOARCH=arm64 staticcheck ./...` (the bare binary, per the
documented pitfall) was not run separately in this session beyond the native
`go tool staticcheck ./...` above, since this story touched no Go source;
`gofmt -l .` and `go vet ./...` cover the same files and were run natively.

## Nothing left open

All seven Story 7.2 acceptance criteria are implemented and mutation-verified
above:

1. Primary control -- gradient (unchanged from 7.1), 1px inset highlight
   (unchanged), elevation (`sh-1`, this story, with the "sh-btn" naming gap
   documented above), `{rounded.md}` (unchanged), `{typography.control}`
   (unchanged), 40px height token (this story), 44px activation target via
   the existing `.fd-target` rule (unchanged, verified still binding), and
   the 0.975 press scale (this story, general to every button per DESIGN.md's
   Motion section).
2. Secondary control -- fill plus the shared functional boundary; mutation
   M7 kills the exact scenario the AC names.
3. Focus ring -- unchanged from Story 7.1's implementation; both existing
   assertions (`draws one ring...` and `never rings a routed landing
   target...`) re-run unmodified and still pass.
4. Browse menu -- `{rounded.lg}` at `{elevation.sh-3}` with a functional
   boundary (this story); focused item takes the primary fill plus a tint
   halo, not the ring alone (this story, ring confirmed still present
   separately).
5. Copy control -- fixed-width assertion unchanged and still binding; success
   tint added (this story) alongside the pre-existing boundary; revert-on-blur
   behaviour lives in `StagedView.tsx`'s `handleCopyBlur`, untouched by this
   story and still covered by `StagedView.test.tsx`.
6. Every activation target -- the single `.fd-target` rule, unchanged,
   re-verified still binding and still the only mechanism in use.
7. Full gate passes; `IdleView`, `StagedView`, `TransferringView`, and
   `OutcomePanel` suites pass unchanged (verified in isolation above, no
   test in any of the four required a change).

Two things deliberately left for later stories, both flagged above with
reasoning: the browse control trigger's own `fd-button--primary` + chevron
(Story 7.3, `IdleView.tsx`), and `OutcomePanel.tsx`'s Dismiss button moving to
`fd-button--quiet` (Story 7.5, `OutcomePanel.tsx`). The `{elevation.sh-btn}`
naming gap in epics.md against DESIGN.md's three-step token table is flagged
for the spine owner rather than worked around silently.
