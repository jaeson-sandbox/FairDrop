# Evidence: Defect fix — Idle browse-pill chevron overlap and drop-zone spacing

## Summary

Two defects observed by the orchestrator driving the built macOS binary on
`epic-9-motion-and-clarity`, after Story 9.3 merged:

1. **The centred browse pill's chevron overlapped its label.** In Idle, the
   pill "Choose File or Folder" (`BrowseControl.tsx`) renders a decorative
   trailing chevron (`.fd-browse-trigger__chevron`, a 12x12 box rotated 45deg)
   whose only spacing rule was `margin-inline-start: auto`. That produces a
   gap only when the flex container has spare inline space to distribute to
   the auto margin; `.fd-button--pill` is `display: inline-flex` sized to its
   own content, so there never is any spare space, and the chevron rendered
   touching/overlapping the label's final "r".
2. **The drop zone's contents (glyph, heading, promise line, pill) read as
   spread out**, with large gaps between rows on a tall window.
   `.fd-drop-zone__inner` is `display: grid` with no `align-content`
   declared, which computes to `normal` — behaving as `stretch` for Grid's
   auto-sized row tracks — so the free block space `.fd-drop-zone`'s
   `flex: 1 1 auto` hands it was distributed equally across the four rows
   instead of the rows staying packed together as a group.

This is the AGENTS.md "Git workflow" defect-fix carve-out: both defects were
already observed, failing tests were written before each fix, and both fixes
are mutation-verified below. No story exists for this change; the failing
tests replace the spec. No markup was changed — the fix is CSS in
`frontend/src/style.css` plus tests in `frontend/browser/accessibility.test.tsx`.

## Why the tests live in `frontend/browser/`, not a `jsdom` unit test

jsdom performs no real layout — no flex/grid free-space distribution, no
rotated-box bounding rects, no computed margins from custom properties — so a
`frontend/src/` unit test would pass against both the broken CSS and the fix,
proving nothing (AGENTS.md, "Testing standards, learned the hard way" — a test
that agrees with the bug). Both new `describe` blocks were added to the
existing rendered-Chromium suite, `frontend/browser/accessibility.test.tsx`,
next to the (Story 9.3) disclosure-chevron rendered proof they parallel.

### A wrinkle found while writing the drop-zone test

`renderIdle()` (the suite's existing helper) mounts `IdleView` as the render
root, with no `.fd-app` ancestor. `.fd-region`'s `flex: 1 1 auto` and
`.fd-app`'s `min-height: 100%` need a flex container with a *definite* height
to have any free space to distribute at all; without `.fd-app` in the tree,
every box in the growth chain sizes to its own content and the spacing defect
cannot reproduce under `renderIdle()` regardless of the CSS — confirmed
directly: the drop-zone test passed against the *unfixed* CSS when first run
under `renderIdle()`. A new helper, `renderIdleInAppShell()`, wraps `IdleView`
in a stand-in `<div className="fd-app" style={{height: '100vh'}}>` (the
explicit `100vh` gives the chain a real, viewport-relative height to grow
into, the way `App.tsx`'s actual shell does) — this is what the drop-zone
test now uses. The chevron test does not depend on vertical free space, so it
keeps using the existing `renderIdle()`.

## Failing tests, captured before the fix

```
 ❯ |chromium| browser/accessibility.test.tsx (25 tests | 2 failed | 21 skipped)
     × at the default viewport, the chevron sits clear of the label and inside the button
     × at the 640x480 minimum main.go sets, the chevron sits clear of the label and inside the button

 FAIL … the browse pill keeps its chevron clear of its label (defect fix) > at the default viewport, …
AssertionError: the chevron's left edge sits -2.5px past the label text's own right edge
(text right: 577.1px, chevron left: 574.6px): a rotated 12px box has a ~17px diagonal, so anything
under 6px reads as touching or overlapping the label: expected -2.48529052734375 to be greater than
or equal to 6

 FAIL … at the 640x480 minimum main.go sets, …
AssertionError: the chevron's left edge sits -2.5px past the label text's own right edge
(text right: 385.1px, chevron left: 382.6px): expected -2.48529052734375 to be greater than or equal to 6

 Test Files  1 failed (1)
      Tests  2 failed | 2 passed | 21 skipped (25)
```

```
 ❯ |chromium| browser/accessibility.test.tsx (25 tests | 2 failed | 23 skipped)
     × at the default viewport, keeps the heading, promise line and pill close together
     × at the 640x480 minimum main.go sets, keeps the heading, promise line and pill close together

 FAIL … the drop zone packs its contents together instead of spreading them out (defect fix) > at the default viewport, …
AssertionError: the heading's bottom edge sits 48.0px above the promise line's top edge: expected 48.015625 to be less than or equal to 12

 FAIL … at the 640x480 minimum main.go sets, …
AssertionError: the heading's bottom edge sits 18.6px above the promise line's top edge: expected 18.640625 to be less than or equal to 12

 Test Files  1 failed (1)
      Tests  2 failed | 23 skipped (25)
```

Both failures negative/oversized exactly the measured claim in the observed
defects: an overlapping (negative) gap for the chevron, and a heading-to-
promise gap of 18.6-48.0px (well past the 12px bound) for the drop zone —
confirming the tests fail on the unfixed code, not merely on an unrelated
assertion.

## The fixes (CSS only, `frontend/src/style.css`)

### 1. Chevron gap

```css
.fd-browse-trigger__chevron {
    ...
    margin-inline-start: var(--spacing-3); /* was: auto */
    ...
}
```

`var(--spacing-2)` (8px) was tried first; the rendered gap (label-text-right
to chevron-left, measured via a `Range` over the label's own text node) came
out at 5.5px against the ≥6px bound — a consistent ~2.5px offset between the
declared margin and the measured gap (likely a sub-pixel glyph-advance
artifact of the Range measurement, not a margin-collapse issue, since the
button is `display: inline-flex` and margins do not collapse between flex
children). `var(--spacing-3)` (12px) clears the bound with margin at both
tested viewports.

### 2. Drop-zone spacing

```css
.fd-drop-zone__inner {
    display: grid;
    flex: 1 1 auto;
    min-block-size: 200px;
    place-items: center;
    align-content: center;   /* new */
    ...
}

.fd-drop-symbol {
    ...
    margin: 0 auto var(--spacing-4);  /* was: var(--spacing-3), i.e. 12px -> 16px */
    ...
}

.fd-drop-zone__inner .fd-state-heading {
    margin-bottom: 6px;
}

.fd-drop-zone__inner .fd-meta {
    margin-bottom: var(--spacing-5);  /* 20px */
}
```

`align-content: center` packs the four rows (their sizes now driven by
content plus the margins above, not by stretched auto tracks) and centres
that packed block within whatever extra height `.fd-drop-zone`'s
`flex: 1 1 auto` hands the container. The heading and promise-line margins
are scoped to `.fd-drop-zone__inner` rather than added to the shared
`.fd-state-heading`/`.fd-meta` rules, since those classes are reused by
Staged, Transferring, the pending card and the outcome panel — none of which
this fix touches.

No markup changes: both fixes are CSS-only, confirmed by `git diff --stat`
touching only `frontend/src/style.css` and
`frontend/browser/accessibility.test.tsx`.

## Passing tests, after the fix

```
 ❯ |chromium| browser/accessibility.test.tsx
 Test Files  1 passed (1)
      Tests  4 passed | 21 skipped (25)
```

(All four new cases — chevron at default and 640x480, drop-zone spacing at
default and 640x480 — pass together.)

## Mutation table

| # | Mutation | Command | Result |
|---|---|---|---|
| 1 | Revert `.fd-browse-trigger__chevron`'s `margin-inline-start` from `var(--spacing-3)` back to `auto` | `npx vitest run --config vitest.browser.config.ts browser/accessibility.test.tsx -t "browse pill keeps its chevron\|keeps the heading, promise line and pill close together"` | **2/4 fail**, both chevron cases, naming the collapsed gap: `the chevron's left edge sits -2.5px past the label text's own right edge ...: expected -2.48529052734375 to be greater than or equal to 6`. The drop-zone cases stay green — the mutation is isolated to the chevron fix, as intended. |
| 2 | Remove `align-content: center` from `.fd-drop-zone__inner` (glyph/heading/promise margins left at their fixed values) | same command | **2/4 fail**, both drop-zone cases, naming the regrown gap: `the heading's bottom edge sits 46.5px above the promise line's top edge: expected 46.515625 to be less than or equal to 12` (and `17.1px` at 640x480). The chevron cases stay green — isolated to the spacing fix, as intended. |

Both mutations were reverted (`style.css` diffed back to the fixed version
and confirmed byte-identical) and the suite reconfirmed green before
proceeding to the full gate.

## Full gate, run locally in `verify.yml` order

All commands run from `/Users/jaesonmartin/Projects/FairDrop/.claude/worktrees/agent-a8ca5fe868db26745`
(Go) or its `frontend/` subdirectory (npm), one after another, never
`wails build` concurrent with a frontend suite.

1. `wails build` — succeeded; `Built '.../fairdrop.app/Contents/MacOS/fairdrop'`
   in 13.0s. Regenerated `frontend/wailsjs/go/main/App.d.ts`, `App.js`,
   `models.ts` — mode-only churn (`git diff --stat` showed `0 insertions(+),
   0 deletions(-)` for all three), reverted with `git checkout --
   frontend/wailsjs`.
2. `gofmt -l .` — no output (nothing to reformat).
3. `go vet ./...` — no output (clean).
4. `go tool staticcheck ./...` — no output (clean).
5. `go test -count=1 ./...` — all packages `ok`: `fairdrop`,
   `internal/network`, `internal/qr`, `internal/server`, `internal/source`,
   `internal/stream`, `internal/transfer`, `scripts`,
   `scripts/mutationverdict`.
6. `cd frontend && npm test` — **734/734 pass**, 19 files.
7. `npm run test:browser` — **31/31 pass**, 2 files (accessibility.test.tsx
   now 25, up from 21; the other browser suite file unchanged).
   `frontend/browser/captures/qr-panel-forced-colors.capture.png` was
   regenerated (a live screenshot, not a fixture, per AGENTS.md) and reverted
   with `git checkout -- frontend/browser/captures/` since this fix touches
   nothing that capture proves.

Pre-flight (not part of the CI matrix, but named in AGENTS.md "Running and
verifying"): `GOOS=linux GOARCH=amd64 go build ./...` — succeeded. (No Go
source was touched by this change; the native `wails build` above already
covers `darwin/arm64`, and no Windows-only code path is involved, so a
Windows cross-build was not run — this is a CSS/test-only change.)

## Files changed

- `frontend/src/style.css` — the two fixes: `.fd-browse-trigger__chevron`'s
  fixed `margin-inline-start`, and `.fd-drop-zone__inner`'s
  `align-content: center` paired with fixed margins on `.fd-drop-symbol` and
  two rules scoped to `.fd-drop-zone__inner .fd-state-heading` /
  `.fd-drop-zone__inner .fd-meta`.
- `frontend/browser/accessibility.test.tsx` — new `renderIdleInAppShell()`
  helper, and two new `describe` blocks: "the browse pill keeps its chevron
  clear of its label (defect fix)" and "the drop zone packs its contents
  together instead of spreading them out (defect fix)", each with two
  viewport cases (default, 640x480).

## Anything unsure

- The ~2.5px offset between the declared chevron margin and the measured
  Range-to-chevron gap was not root-caused beyond "a sub-pixel glyph-advance
  artifact of measuring a `Range` over the text node" — it was worked around
  empirically (bumping the token until the measured gap cleared 6px with
  margin at both tested viewports) rather than traced to a specific font-
  metrics mechanism. It does not affect correctness of the fix or the test,
  but a future reader tightening the margin back down should re-measure
  rather than assume the token-to-pixel mapping is exact.
- `StagedView.tsx` and its `style.css` rules were left untouched throughout,
  per the parallel-agent carve-out in this task's brief; `.fd-state-heading`
  and `.fd-meta` margin scoping was written specifically to avoid any
  incidental effect on Staged, Transferring, the pending card or the outcome
  panel, but those views' own rendered tests were not independently
  re-inspected beyond the full `npm test` / `npm run test:browser` runs
  above passing unchanged.
