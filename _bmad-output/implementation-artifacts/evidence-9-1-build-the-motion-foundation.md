# Evidence: Story 9.1: Build the Motion Foundation

## Summary

Story 9.1 gives Epic 9 its motion primitives: a fade-plus-rise entrance for every
`data-phase-view`, a capped per-child stagger helper, a fade-plus-scale entrance for the browse
menu, and a smooth cross-engine expand/collapse for `Disclosure`. DESIGN.md's Motion section is
rewritten to state all of this before the CSS, and its Disclosure row records the implementation
choice. No view is re-laid-out and no copy changes; scope is exactly `frontend/src/style.css`'s
motion rules, `frontend/src/ui/Disclosure.tsx`, the browse menu's entrance, DESIGN.md's Motion
section and Disclosure row, and the touched test files (`styles.test.ts`, `IdleView.test.tsx`,
`accessibility.test.tsx`). No Go file changed.

## What changed and why

- **`frontend/src/style.css`**
  - `[data-phase-view]` (matches all five views -- Idle, Pending, Staged, Transferring, the
    outcome panel when it is the phase view): a resting `opacity: 1; translate: none;` plus a
    `transition` on both, with the entrance state (`opacity: 0; translate: 0 10px;`) declared only
    inside `@starting-style`. No JavaScript, no `@keyframes`, no `animation`. An engine without
    `@starting-style` support never sees the starting state at all, so it paints the resting rule
    from first layout -- progressive enhancement by construction, nothing to detect.
  - `.fd-rise` + its own `@starting-style`: the optional per-child stagger helper. A child opts in
    with the class and sets `--fd-stagger` to its position; `transition-delay:
    calc(min(var(--fd-stagger, 0), 5) * 55ms)` caps the queue at five steps (~275ms). Not wired to
    any element yet -- Stories 9.3-9.6 consume it.
  - `.fd-browse-menu`: fade + scale-from-0.96 entrance via `@starting-style`, `transform-origin:
    top left` (the menu's actual anchor corner -- `inset-inline-start: 0`, not the owner-approved
    prototype's centred-trigger layout, which is why the origin differs from the prototype's `top
    center`).
  - `prefers-reduced-motion: reduce`: added `translate: none !important; scale: none !important;`
    to the existing universal rule. The pre-existing duration/delay collapse alone was not enough
    for "no element translates or scales on entrance" -- a 1ms transition from `translate: 0 10px`
    to `none` still, for that 1ms, translates. Verified this does not touch
    `.fd-button:active`'s `transform: scale(0.975)` press feedback (a different CSS property from
    the standalone `scale` reset).
  - `.fd-disclosure__region` / `.fd-disclosure__region-inner`: the `grid-template-rows: 0fr <->
    1fr` expand/collapse, with a `visibility` transition (delayed on close, instant on open) that
    keeps collapsed content out of the tab order and the accessibility tree without an unmount.
  - `.fd-disclosure__summary`/`__heading`/`__chevron`: restyled for the new `<button>`/`<h2>`
    structure (see Disclosure choice below); chevron rotation now keys off `[data-open]` instead
    of native `[open]`.
- **`frontend/src/ui/Disclosure.tsx`**: rewritten from native `<details>`/`<summary>` to a
  controlled `<button aria-expanded aria-controls>` wrapped in an `<h2>`, plus a region div. See
  "The Disclosure choice" below.
- **DESIGN.md**: Motion section rewritten before any CSS was written (entrance rule, stagger,
  browse menu, progressive enhancement, the `@keyframes` ban and its reason, the check-mark
  sentence removed rather than contradicted, the `:active` scale note reconciled with the new
  entrance scale). Disclosure row in the Components table appended with the Story 9.1 choice and
  reasoning.
- **Test files**: `styles.test.ts` extended with a new "Story 9.1: the motion foundation" describe
  block plus edits to the pre-existing Disclosure-shaped assertions (tag names, selectors) that the
  markup change requires; `IdleView.test.tsx` extended/rewritten for the same reason (every place
  that asserted `DETAILS`/`SUMMARY` or `> summary`); `accessibility.test.tsx` gained a new describe
  block proving a phase view's rendered resting state, plus a fix described below.

## The Disclosure implementation choice, and why

DESIGN.md's acceptance criteria allowed keeping native `<details>` if a smooth open was achievable
in both Chromium and WebKit. It was not chosen, for two reasons found while implementing:

1. **Cross-engine animatability.** Animating a native `<details>` open/close smoothly needs either
   JavaScript measuring `scrollHeight` (a mechanism this story's ACs steer away from -- it fights
   the `@starting-style`/CSS-transition approach used everywhere else) or the newer
   `::details-content` pseudo-element, which is not available across the WKWebView range
   `build/darwin/Info.plist`'s `LSMinimumSystemVersion` 10.13 implies. `grid-template-rows`
   interpolating between `0fr` and `1fr` on a plain `<div>` has materially broader support and is
   the exact technique the owner-approved prototype
   (`mockups/epic-9-proposal.html`, `.row-body`) already uses for every expand/collapse in the
   product -- including the ones inside Staged's own help box that Story 9.4 will build.
2. **Consistency.** Using one technique for both the browse menu (`@starting-style`) and the
   disclosure (`grid-template-rows`) keeps the sheet's motion vocabulary small, matching the
   prototype's own approach rather than inventing a details-content path that only Idle's two
   disclosures would use.

Trade-offs recorded honestly:

- `<button>`'s content model does not permit a heading child (phrasing content only), unlike
  `<summary>`, which explicitly permits one heading as its first child. The `<h2>` now *wraps* the
  button (the WAI-ARIA APG accordion pattern) rather than sitting inside it.
  `screen.getByRole('heading', {name: ...})` still finds it -- a heading's accessible name is
  computed from its full text content, descending into the button the same way it descended into
  `<summary>`.
- Collapsed content stays mounted (a transitioned `visibility: hidden`, not `inert` -- unsupported
  on the compatibility range in question -- and not an unmount) so `grid-template-rows` has
  something real to animate. This is a deliberate departure from native `<details>`'s
  `display: none`, which removed the content from layout entirely.
- Keyboard operability, Enter/Space activation, Escape blurring the trigger, and every existing
  recovery/firewall string are unchanged in substance; the markup that carries them changed, and
  the tests that pinned the old markup were rewritten with comments naming why, not loosened.

## Mutation table

Every mutation below was applied by hand to the working tree, the named test(s) run, the failure
(with the specific message) recorded, then the file restored and re-diffed against a saved copy to
confirm an exact restore before moving to the next mutation.

| # | Mutation | Test(s) run | Result |
|---|---|---|---|
| 1 | Deleted the `@starting-style` block for `[data-phase-view]` | `styles.test.ts` > "Story 9.1: the motion foundation > enters every phase view..." | **Failed**, naming it: `the @starting-style block for [data-phase-view]: expected null to be truthy` |
| 2 | Replaced the `[data-phase-view]` entrance with an equivalent `@keyframes fd-phase-view-enter` + `animation:` | same test | **Failed**: the `transition:` assertion no longer matched the rewritten rule (and the sheet-wide `@keyframes`/`animation:` ban would separately catch this) |
| 3 | Deleted `translate: none !important;` / `scale: none !important;` from the reduced-motion block | `styles.test.ts` > "removes translate and scale from every entrance outright..." | **Failed**, naming both missing lines |
| 4 | Flipped `[data-phase-view]`'s resting `opacity: 1;` to `opacity: 0;` | `styles.test.ts` > "enters every phase view..." **and** `accessibility.test.tsx` (real Chromium) > "leaves the Idle/Staged phase view fully opaque..." | **Both failed.** Unit: `expected '...' to contain 'opacity: 1;'`. Browser (real layout, post-settle): `expected '0' to be '1'` on both Idle and Staged |
| 5 | Swapped `.fd-disclosure__region`'s `grid-template-rows: 0fr;` for `max-height: 0;` | `styles.test.ts` > "expands and collapses the disclosure region with grid-template-rows..." | **Failed**, naming the missing `grid-template-rows: 0fr;` |
| 6 | Deleted the `@starting-style` block for `.fd-browse-menu` | `styles.test.ts` > "scales and fades the browse menu in from ~0.96..." | **Failed**, naming it |
| 7 | Removed the `min(..., 5)` cap from `.fd-rise`'s `transition-delay` calc | `styles.test.ts` > "provides a capped, reduced-motion-neutral per-child stagger helper..." | **Failed**, naming the uncapped calc it found instead |
| 8 | `Disclosure.tsx`: made `handleSummaryKeyDown` a no-op (Escape no longer blurs) | `IdleView.test.tsx` > "blurs the firewall summary on Escape" | **Failed**: `expected <button> not to be <button>` (still focused after Escape) |
| 9 | `Disclosure.tsx`: made the trigger's `onClick` a no-op (never toggles) | `IdleView.test.tsx` > "flips aria-expanded and the region's data-open together, and back again" | **Failed**: `expected 'false' to be 'true'` |

All nine restores were diffed byte-for-byte against the pre-mutation copy (`diff ... && echo
IDENTICAL`) before continuing.

## A flaky browser test found and fixed during this story (not a mutation)

Adding entrance motion to every phase view and the browse menu introduced a real race in
`accessibility.test.tsx`'s pre-existing rendered-geometry checks (320px reflow, 200% text, the
44px activation floor): a `getBoundingClientRect()`/`getComputedStyle()` call taken immediately
after `render()` can read a mid-transition size. This was not hypothetical -- it reproduced
directly: the browse trigger measured `43.999984...px` tall against the required `44`, a sub-pixel
remnant of measuring while an ancestor's `translate` was still interpolating, and it reproduced
about 2 times in 5 runs before the fix, 0 times in 10 runs after. Confirmed absent on the
pre-Story-9.1 baseline (`git stash`, 8/8 clean runs) before treating it as new.

Fix: `renderStaged`, `renderTransferring` and `renderIdleMenuOpen` in `accessibility.test.tsx` are
now `async` and each awaits a shared `waitForEntranceToSettle(container)` helper (two
`requestAnimationFrame` ticks, then `Promise.all` over `element.getAnimations({subtree: true}).map(a
=> a.finished)`) before returning. Waiting from the render root's `container` -- not from one
descendant -- is what catches both the phase view's own entrance and a nested one (the browse
menu) in a single call; an earlier version of this fix waited on the menu alone and missed the
ancestor region's still-running translate, which is exactly how the flake first reached a green
run. Every call site was updated to `await` the now-`Promise`-returning helpers (including the one
test that does not measure geometry at all, `QR drag source is disabled`, purely because the
render helper's return type changed).

## Gate transcript summary

All commands run from the repo root, in the order AGENTS.md specifies, on macOS (darwin/arm64).

1. `wails build` -- built `build/bin/fairdrop.app` in ~7-13s across two runs; `git status` showed
   no content drift in `frontend/wailsjs/` (only incidental file-mode bits from the regenerate,
   reverted with `git checkout --`).
2. `gofmt -l .` -- no output (clean).
3. `go vet ./...` -- no output (clean).
4. `go tool staticcheck ./...` -- no output (clean).
5. `go test -count=1 ./...` -- `ok` across all 9 packages (`fairdrop`, `internal/network`,
   `internal/qr`, `internal/server`, `internal/source`, `internal/stream`, `internal/transfer`,
   `scripts`, `scripts/mutationverdict`).
6. `CGO_ENABLED=1 go test -count=1 -race ./...` -- confirmed `go env CGO_ENABLED` prints `1` first;
   `ok` across all 9 packages (the longest, `internal/stream`, ~100s under the race detector).
7. `cd frontend && npm test` -- **17 files / 679 tests passed**, no failures, run three times
   across the session (before, during, and after mutation testing) with identical results.
8. `cd frontend && npm run test:browser` -- **2 files / 26 tests passed**, real Chromium via
   Playwright 1.63.0. Run 6 consecutive times after the flake fix above (plus 4 more targeted at
   just the previously-flaky test) with 10/10 green; `git checkout --
   frontend/browser/captures/` afterward each time per the AGENTS.md capture-churn pitfall.
9. `GOOS=darwin GOARCH=arm64 go build ./...` and `GOOS=linux GOARCH=amd64 go build ./...` --
   both clean (pre-flight only; no Go source changed this story).
10. `GOOS=darwin GOARCH=arm64 ~/go/bin/staticcheck ./...` (bare binary) -- no output (clean).
11. Line-ending check: every touched file (`style.css`, `Disclosure.tsx`, `IdleView.test.tsx`,
    `styles.test.ts`, `accessibility.test.tsx`, `DESIGN.md`) confirmed LF-only by hand
    (`grep -qU $'\r'`).

No Go source file changed in this story, so steps 5, 6, 9 and 10 are pre-flight confirmation that
nothing broke, not evidence of new behaviour.

## Uncertain, deviating, or pending

- **"Drive the built macOS binary by hand" (the final AC bullet) is pending -- orchestrator to
  drive the built binary.** Not done here, and not claimed as done. `wails build` succeeded and
  produced `build/bin/fairdrop.app`; the app was not launched or interacted with by hand in this
  session.
- **`transform-origin: top left` deviates from the prototype's `top center`.** Recorded in-line in
  style.css and above: the prototype's browse menu is centred under a centred trigger
  (`left: 50%; translate: -50% 0`), while this product's menu is left-anchored
  (`inset-inline-start: 0`) under a full-width trigger. The origin was chosen to match this
  product's actual anchor point, not the prototype's literal value, since the AC asks for "origin
  at its trigger" rather than a specific keyword.
- **The `opacity: 0`/`opacity: 1` binary values in `@starting-style` widened one pre-existing
  test** ("dims nothing with opacity, because a composited pair publishes no figure") to allow `0`
  alongside `1`, narrowly, with a comment distinguishing binary presence/absence from the
  fractional-dimming defect that test was written to catch. Any value strictly between 0 and 1
  remains forbidden exactly as before.
- **The stagger helper (`.fd-rise`/`--fd-stagger`) is unused by any component in this story** --
  by design, per the story's scope ("9.3-9.6 consume what this story builds"). It is proved to
  exist, be capped, and be reduced-motion-neutral by `styles.test.ts` alone; there is no rendered
  proof of a staggered sequence yet because nothing sets `--fd-stagger` yet.
- Windows and Linux were not built or tested in this session (macOS-only local environment, per
  AGENTS.md's tool-availability notes); the darwin/arm64 and linux/amd64 items above are Go
  cross-compile pre-flights only, not platform-native runs.
