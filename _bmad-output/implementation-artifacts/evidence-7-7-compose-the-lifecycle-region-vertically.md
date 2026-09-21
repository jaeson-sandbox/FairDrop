# Evidence: Story 7.7: Compose the Lifecycle Region Vertically

## What changed

### `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`

Amended **first**, per the story's ordering rule ("the spine is amended
first, not retrofitted afterwards"):

- New **"Vertical composition (Story 7.7)"** subsection under **Layout &
  Spacing**, immediately after the existing reflow-contract paragraph. States:
  the lifecycle region fills the available window height rather than hugging
  its content; Idle spends the extra height by growing the drop zone
  (bounded by a maximum) while its other controls keep natural spacing;
  Pending, Transferring and a terminal Done/Error rendered as the phase view
  centre vertically; **Staged is the stated exception** and stays top-aligned
  because it is content-rich and centring would move the QR off-screen first
  as the window shortens; a retained outcome above Idle keeps its natural
  height and never grows or centres; and at the 640×480 minimum the drop
  zone's growth is the first thing to yield, via a *bounded flexible* height
  (flex-grow with a max, never fixed), so the window scrolls rather than
  clipping.
- **Components table, Disclosure row**: removed "with a leading tinted icon"
  and added a sentence recording *why* -- the icon resolved to a featureless
  coloured dot at its rendered size, which the acceptance criteria call an
  unacceptable outcome, and there wasn't room for a legible shape beside a
  one-line summary, so the row ships with no icon, matching the platform's own
  disclosure rows.
- **Elevation & Depth, gradient rule**: added a paragraph recording the
  `{colors.primary-hi-dark}` narrowing (`#6FB0FF` → `#5EA6FF`, half the
  original delta above `{colors.primary-dark}`) and stating explicitly that
  this token is not one the unrounded contrast proof publishes a figure for,
  so no published ratio moved and no re-derivation was needed.
- **Frontmatter**: `primary-hi-dark` updated from `'#6FB0FF'` to `'#5EA6FF'`
  to match the stylesheet -- the frontmatter is a value the stylesheet has to
  agree with, not merely prose describing it, so it had to move too or the
  same document/code drift this epic has already found three times would
  have reappeared as a fourth instance.

### `frontend/src/style.css`

- **`.fd-region`**: gained `flex: 1 1 auto`, the growth rule. It is the
  common ancestor for every phase's own view (Idle, Pending, Staged,
  Transferring all render it), so this one declaration is what lets the
  region fill `.fd-app`'s height instead of sizing to its content.
- **New rule** `.fd-region[data-phase-view='pending'], .fd-region[data-phase-view='transferring'], .fd-app > .fd-outcome[data-phase-view='outcome'] { flex: 1 1 auto; justify-content: center; }`
  -- the centring rule for the three short states. The terminal outcome panel
  is reached through the attribute selector, not the bare `.fd-outcome`
  class, because `.fd-outcome` is also used for a retained panel above Idle
  and for a command-failure panel rendered inside `.fd-idle` -- neither may
  grow or centre.
- **New rule** `.fd-app > .fd-outcome[data-phase-view='outcome'] { width: 100%; max-width: 720px; margin-inline: auto; }`
  -- gives the terminal outcome panel the same centred-column width
  `.fd-region` already has, since it is not itself wrapped in `.fd-region`.
- **`.fd-idle`**: gained `flex: 1 1 auto`, so it consumes the full height
  `.fd-region` grows to -- the prerequisite for the drop zone below having
  anything to grow into.
- **`.fd-drop-zone`**: gained `display: flex; flex: 1 1 auto; flex-direction: column; min-block-size: 200px; max-block-size: 420px;`.
  `min-block-size` is the floor the zone already reported; `max-block-size`
  is the new bound the acceptance criteria require, so a maximised window
  does not produce an absurd target. Both are declared as a range, never a
  plain `block-size`/`height`, which is the mechanism the acceptance
  criteria's fixed-height mutation targets.
- **`.fd-drop-zone__inner`**: gained `flex: 1 1 auto`, so the actual dashed
  box -- not just its outer padding shell -- is what visibly grows.
- **Removed** `.fd-disclosure__icon` and its `::before` rule (the 26px tinted
  circle with a 10px dot). Nothing else referenced these selectors.
- **`--color-primary-hi` (dark override)**: `#6FB0FF` → `#5EA6FF`, with a
  comment recording the reasoning and that it disturbs no published contrast
  pair.

### `frontend/src/ui/Disclosure.tsx`

Removed the `<span className="fd-disclosure__icon" aria-hidden="true"/>`
from `<summary>`, and updated the component's doc comment to record the
removal and point at DESIGN.md's Components table. No prop, no test hook, no
accessible-name-bearing element was touched -- the icon was `aria-hidden`
before removal, so nothing assistive technology reads changes.

### `frontend/src/ui/styles.test.ts`

- Updated the pinned dark-palette literal for `primary-hi` from `'#6FB0FF'`
  to `'#5EA6FF'` in `declares the authored dark half as exact values rather
  than an inversion`, with a comment explaining why this is the one token
  that could move without re-deriving a contrast figure (it is absent from
  the `placed` array the unrounded-contrast-proof `describe` block checks).
  **This is the one existing assertion this story modified**, and it is a
  literal-pin update the acceptance criteria explicitly anticipate ("the
  primary control on the dark canvas... prefer narrowing the gradient's
  top-stop delta"), not a loosening: the test still pins an exact hex value,
  just a different one, and still fails on any other value including the old
  one (proven below, mutation M-primary-hi).
- New `describe('Story 7.7: compose the lifecycle region vertically')` block,
  seven tests, detailed in the mutation table below.

No other existing test file was touched. `IdleView.test.tsx`,
`StagedView.test.tsx`, `TransferringView.test.tsx`, `OutcomePanel.test.tsx`,
`StagePendingCard.test.tsx`, `App.focus.test.tsx` and every other suite pass
unmodified (confirmed by the full `npm test -- --run` run below).

## Why `.fd-region` alone, and not per-phase-view CSS

Every phase view already renders exactly one `<div className="fd-region"
data-phase-view="...">` (`IdleView.tsx`, `StagePendingCard.tsx`,
`StagedView.tsx`, `TransferringView.tsx`) except the terminal Done/Error
phase view, which is `OutcomePanel.tsx`'s `<section className="fd-outcome"
data-phase-view="outcome">` rendered directly by `App.tsx` (not wrapped in
`.fd-region`, because the reset that retains an outcome above Idle has to
keep that exact DOM node -- see the comment above `phaseBody` in `App.tsx`).
Growing `.fd-region` and (separately, with an equivalent rule) the terminal
outcome panel therefore reaches every phase's own view with two rules
instead of one per state, and the `data-phase-view` attribute -- already
present for test/diagnostic purposes -- is what lets the centring rule target
exactly Pending, Transferring and the terminal outcome, and nothing else.

## Files touched

- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`
- `frontend/src/style.css`
- `frontend/src/ui/Disclosure.tsx`
- `frontend/src/ui/styles.test.ts`

`IdleView.tsx`, `App.tsx`, `OutcomePanel.tsx`, `StagedView.tsx`,
`TransferringView.tsx` and `StagePendingCard.tsx` were all read in full and
needed no edits: the existing `.fd-region`/`.fd-idle`/`.fd-outcome` class
names and `data-phase-view` attributes already carried everything the new
CSS needed to target. `BrowseControl`'s keyboard handlers in `IdleView.tsx`
were not touched, as instructed.

## Mutation table

Every mutation below was applied to the real working tree with a Python
edit, run against the real suite (`npx vitest run --run -t "<test name
fragment>"`), confirmed to fail and name the problem, then reverted from a
pre-mutation-pass backup copy of `style.css` (`cp ~/.style_css_backup_7_7
frontend/src/style.css`), with a `diff` against that backup confirming a
byte-identical revert before moving to the next mutation. `git status
--short` after the whole pass showed only the four intended files modified,
matched by a full `npm test -- --run` returning to 613/613 green.

| # | Mutation | AC / claim | Result |
|---|---|---|---|
| M-region-hug | Removed `flex: 1 1 auto;` from `.fd-region` | "the lifecycle region fills the available height rather than hugging its content"; the story's own named mutation, "remove the growth and restore the hugging column" | KILLED -- `grows the region to fill the available height rather than hugging its content`: `expected '.fd-region {...' to match /flex:\s*1 1 auto;/` |
| M-no-center | Removed `justify-content: center;` from the pending/transferring/outcome rule | "the region is centred vertically" for Pending/Transferring/terminal outcome; named mutation "top-align any of them" | KILLED -- `centres Pending, Transferring and the terminal outcome-as-phase-view, never Idle or Staged`: `expected '...' to contain 'justify-content: center;'` |
| M-fixed-height | `.fd-drop-zone`'s `min-block-size: 200px; max-block-size: 420px;` → `block-size: 420px;` | "bounded by a maximum", "no fixed height traps content"; named mutation "give the drop zone a fixed height instead of a bounded flexible one" | KILLED -- `grows the drop zone into Idle's slack with a bounded flexible height, never a fixed one`: `expected '...' to match /min-block-size:\s*200px;/` |
| M-fixed-height-alongside | Same target, but *added* a literal `block-size: 420px;` alongside the existing bounded pair rather than replacing them, to prove the guard catches a fixed height that merely coexists with the bound | Same AC as above, stronger form | KILLED -- `expected '...' not to match /(?<!min-\|max-)\bblock-size:/` |
| M-icon-back | Reintroduced `.fd-disclosure__icon { ... }` (the 26px tinted circle) into the stylesheet | "either render them large enough... or remove them... a dot is not an acceptable outcome"; this repo took removal | KILLED -- `removes the disclosure summary icon rather than shipping a featureless dot`: `expected '...' not to contain '.fd-disclosure__icon'` |
| M-primary-hi | `--color-primary-hi` (dark) `#5EA6FF` → `#6FB0FF` (reverted the narrowing) | "its fill reads less hot... [than] today"; the accent-narrowing claim | KILLED (two independent tests) -- `narrows the dark primary-hi delta without disturbing any published contrast figure`: `expected '...' to contain '--color-primary-hi: #5EA6FF;'`; also kills the pre-existing `declares the authored dark half as exact values rather than an inversion` pin |
| M-retained-grows | Added `flex: 1 1 auto;` to the base (unqualified) `.fd-outcome { ... }` rule, simulating the growth rule leaking onto a retained or command-failure panel instead of staying scoped to the terminal-phase-view selector | "a retained outcome above Idle... [is] not pushed off-screen or [causes] the drop zone collapsing to nothing"; "the two compose" | KILLED -- `excludes a retained outcome from growth or centering, so it keeps its natural height`: `expected '.fd-outcome {...' not to match /flex:\s*1/` |

Not re-mutated (unchanged by this story, already proven by their own
still-passing, unmodified tests): the 320px/640px/760px reflow breakpoints,
the functional-boundary/decorative-separator split, the single-gradient and
three-shadow-layer ceiling, the forced-colors and reduced-motion overrides,
and every focus-routing rule. All of these are exercised by the full test
run below and none of their assertions needed touching, which is itself
evidence this story stayed a layout change.

## Manual observation

Per `docs/release-policy.md`, automated verification is mandatory and manual
observation is optional for this personal project. This story's own scope
description says it is proven at layout mechanism, not screenshot, because
"you cannot see" the sizes involved from a headless run -- consistent with
that, no manual drive of the built binary at the default or minimum window
size was performed for this story; every claim above is mechanism-level
(pinned CSS rules), not a rendered observation. Recorded honestly as **not
observed**, not fabricated.

## Gate transcripts (macOS arm64, native)

Run in the order `.github/workflows/verify.yml` uses, after the mutation
pass above and with the working tree back to its intended diff (`git status
--short` showed only the four files above; a `diff` against the
pre-mutation backup confirmed `style.css` was byte-identical to its intended
state before this final run).

- `wails build`: **PASS**. `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 11.914s.` One pre-existing linker warning (`object file ... built for newer 'macOS' version (13.0) than being linked (11.0)`), unrelated to this change.
- Bindings mode drift: `App.d.ts`, `App.js`, `models.ts` came back `755` after `wails build`, as documented; `chmod 644` applied to all three; `git status --short` showed no diff for any of the three afterward (content was already identical, only the mode bit had changed).
- `scripts/verify-build-asset-drift.sh`: **PASS**, no output, exit 0.
- `frontend/dist/.gitkeep`: present.
- `gofmt -l .`: **PASS**, no output (no Go files touched by this story).
- `go vet ./...`: **PASS**, no output.
- `go tool staticcheck ./...`: **PASS**, no output.
- `go test -count=1 ./...`: **PASS** -- `ok` for `fairdrop`, `internal/network`, `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`, `scripts`, `scripts/mutationverdict`.
- `go env CGO_ENABLED`: `1`, confirmed before trusting the race run.
- `CGO_ENABLED=1 go test -count=1 -race ./...`: **PASS** -- `ok` for all nine packages (`internal/stream` ~107s under the race detector, the rest a few seconds each).
- `GOOS=windows GOARCH=amd64 go build ./...`: **PASS**.
- `GOOS=darwin GOARCH=arm64 go build ./...`: **PASS** (pre-flight).
- `GOOS=linux GOARCH=amd64 go build ./...`: **PASS** (pre-flight).
- `cd frontend && npm test -- --run` (not concurrent with `wails build`): **PASS** -- 17 files, **613 tests** (606 baseline on `epic-7-quartz` before this story + 7 net new, all in the new `styles.test.ts` "Story 7.7" `describe` block; one existing test's pinned literal was updated, none added or removed elsewhere).
- `cd frontend && npm run test:browser`: **PASS** -- 1 file, 17 tests, unchanged.
- `git diff --check`: **PASS**, no output (no whitespace errors).
- Line-ending check (`git ls-files --eol | grep -E '(i|w)/(crlf|mixed)'`): **PASS**, no matches (grep exit 1).

## Nothing left open

All nine Story 7.7 acceptance criteria are implemented and mutation-verified
above:

1. DESIGN.md's Layout & Spacing gains the vertical-composition rule, amended
   before the stylesheet implemented it -- **done**, see "What changed" above.
2. The lifecycle region fills the available height rather than hugging its
   content -- **done**, M-region-hug.
3. Idle's drop zone absorbs the slack, bounded by a maximum, while its other
   controls keep natural spacing -- **done**, M-fixed-height and
   M-fixed-height-alongside; the `.fd-idle`/`.fd-drop-zone`/`.fd-drop-zone__inner`
   flex chain is what makes the growth reach the actual visible box.
4. Pending, Transferring and the terminal Done/Error phase view centre
   vertically -- **done**, M-no-center.
5. Staged stays top-aligned, and DESIGN.md states why -- **done**: Staged's
   `.fd-region[data-phase-view='staged']` is not named by the centring
   selector at all (the flex default is top alignment), and the
   `states the vertical-composition rule in DESIGN.md` test additionally
   pins that DESIGN.md's own text names the exception rather than leaving it
   as CSS silence.
6. At the 640×480 minimum / 320px content width / 200% zoom / text-spacing
   overrides, the drop zone's growth yields first via a bounded flexible
   height, and every existing reflow assertion still passes unchanged --
   **done**: the entire pre-existing `reflow to 320 CSS pixels` `describe`
   block passed unmodified in the full run above, and M-fixed-height proves
   the fixed-height failure mode is caught.
7. A retained outcome above Idle composes without being pushed off-screen or
   collapsing the drop zone -- **done**, M-retained-grows; the retained panel
   keeps `flex: 0 1 auto` (the default), and the drop zone's
   `min-block-size: 200px` floor is unconditional -- a retained panel above
   it can shrink the zone's flex-grow allotment but never below that floor.
8. Disclosure summary glyphs are legible or removed, never a dot -- **done**,
   removal chosen; M-icon-back.
9. The primary control's dark-canvas fill reads less hot, via a narrowed
   gradient top-stop delta, with every published contrast figure
   undisturbed -- **done**, M-primary-hi; `{colors.primary}` itself is
   unchanged, so no figure in the unrounded contrast proof moved, confirmed
   by the full `npm test -- --run` pass (all pre-existing `the unrounded
   contrast proof` tests still pass unmodified).
10. The full gate passes on macOS -- **done**, transcripts above. Windows was
    not run natively in this environment; the `GOOS=windows GOARCH=amd64 go
    build ./...` pre-flight and the unmodified CI workflow (which runs the
    full gate on `windows-latest` for every push to `epic-*`) are the
    coverage available here, consistent with every prior story's evidence in
    this epic.
