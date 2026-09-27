# Evidence: Story 10.1 -- Draw the Link Field's Focus Ring Whole

Defect fix under AGENTS.md's carve-out (the defect is already observed, on the owner's
screenshot of the built 1.3.0 binary): a failing rendered test written first, then the
fix, then mutation. Branch `story-10-1-link-field-focus-ring`, forked from
`origin/epic-10-polish-after-1-3-0`.

## The defect

Owner screenshot of the built 1.3.0 binary: clicking the revealed direct-link field on
Staged shows a mocha band above and below the field, cut off square at both sides,
instead of a ring. The field's own select-all-on-focus highlight also painted the
engine's default selection colour rather than the product's accent.

## Diagnosis

`.fd-url:focus-visible` (`frontend/src/style.css`) paints the same shared two-tone
`box-shadow` ring every Tab-reachable control uses -- a `--color-surface` gap, then a
`--color-primary` ring `--focus-ring-offset + --focus-ring-width` (2px + 2px = 4px)
further out, drawn **outside** the field's own border box. `.fd-url-reveal__inner`'s
`overflow: hidden` -- needed so the Story 9.4 `grid-template-rows: 0fr <-> 1fr` open/close
animation reads as a clean accordion rather than spilling content -- clipped anything
outside *its own* box, and before this fix its box hugged the field so tightly (no
horizontal padding anywhere in the chain, no bottom padding on `.fd-url-reveal__body`)
that the ring had no room on any side: fully clipped left and right, and only a flat
sliver of the ring's straight edge survived top and bottom (there being no room for its
rounded corners either, since those need room on both axes at once) -- exactly the
"band, cut off square" the owner described.

The select-all highlight came from nowhere -- there was no `.fd-url::selection` rule at
all, so every engine fell back to its own default (system blue in Chromium).

## Why the fix lives on `__body`, not on `__inner`

The obvious "common approach" -- padding plus a matching negative margin on the clipping
box itself (`.fd-url-reveal__inner`) -- turns out to be the wrong element for this
specific animation. `__inner` is exactly the element the `0fr`/`1fr` grid-rows animation
squeezes to zero height when collapsed (with `min-height: 0` overriding the *content-based*
automatic minimum size grid items get by default, which is what lets it reach true zero
today). `box-sizing: border-box` has a separate, unrelated floor: an element can be told
to render shorter than its own padding sum, but it never actually will -- the used height
clamps at the padding-plus-border sum instead. Adding the ring-room padding directly to
`__inner` would have left a permanent few-pixel gap the reveal could never fully close,
regardless of `min-height: 0` (that override defeats content-based minimums, not the
padding floor, which is a different rule of the box model). A negative margin on `__inner`
does not rescue this either: in the open state, the `1fr` row is sized from `__inner`'s own
content contribution (there is no external, indefinite space for a lone `fr` track to
divide, so it behaves as `auto`/max-content here), which makes the margin trick
self-cancelling on the block axis in exactly this construction, unlike the inline axis
(where the grid column width comes from the container, not from `__inner`'s own content,
and the trick does work).

The fix instead adds the ring-room padding to `.fd-url-reveal__body` -- `__inner`'s sole
child, an ordinary block whose own height is simply whatever its padding and content add
up to, with no floor of the kind above. `__inner`, itself unpadded, auto-sizes around
that larger box when open (giving the ring exactly the room it needs on every side) and
clips it away entirely, ring room included, when collapsed -- nothing about the collapsed
mechanism changes, because `__inner` still carries zero padding of its own. The one
accepted trade-off: horizontally, `__inner`'s width is fixed externally (the grid column
stretch), so the added left/right padding on `__body` insets the field itself by 4px on
each side rather than growing the reveal region wider than the card -- an imperceptible
width change, not a functional regression, and the only unavoidable cost of keeping the
collapse mechanism intact.

## Failing test, written first, captured before any fix

New tests in `frontend/browser/staged-url-field.test.tsx` (rendered Chromium; jsdom
performs no layout and cannot see clipping at all, per the file's own existing rationale
for living here). Run against the untouched tree (CSS fix reverted via
`git apply -R` on the `style.css` hunk, test file with the new tests kept):

```
$ npx vitest run --config vitest.browser.config.ts browser/staged-url-field.test.tsx

 × is never clipped by the reveal region at 1024x768 669ms
 × is never clipped by the reveal region at 640x480 665ms

 FAIL  browser/staged-url-field.test.tsx > Staged direct URL field draws its focus ring whole (Story 10.1) > is never clipped by the reveal region at 1024x768
AssertionError: left: no room for the ring: expected 413 to be less than or equal to 409.5

 FAIL  browser/staged-url-field.test.tsx > Staged direct URL field draws its focus ring whole (Story 10.1) > is never clipped by the reveal region at 640x480
AssertionError: left: no room for the ring: expected 21 to be less than or equal to 17.5

 Test Files  1 failed (1)
      Tests  2 failed | 7 passed (9)
```

Both failures name the exact defect: no horizontal room for the ring, at both viewports
the acceptance criteria named. The third new test (collapse-still-works) already passed
against the untouched tree, since nothing about the collapse mechanism was ever broken --
only the open-state clip room.

## The fix

`frontend/src/style.css`:

- `.fd-url-reveal__body` now carries
  `padding: calc(var(--spacing-3) + var(--focus-ring-offset) + var(--focus-ring-width)) calc(var(--focus-ring-offset) + var(--focus-ring-width)) calc(var(--focus-ring-offset) + var(--focus-ring-width));`
  (top folds in the pre-existing `--spacing-3` gap so all four sides read as one
  consistent figure) in place of the old bare `padding-top: var(--spacing-3);`.
- `.fd-url::selection { background: var(--color-primary-tint); color: var(--color-text); }`
  added beside `.fd-url`'s own rule.

No markup change was needed in `StagedView.tsx` -- the fix is entirely in `style.css`, as
the story's scope note allowed.

## Contrast: reused, not new

`{colors.text}` on `{colors.primary-tint}` is not a new pair. It is already load-bearing
and already published in `styles.test.ts` ("the unrounded contrast proof") and
`DESIGN.md`'s Colors table as the drop zone's drag-active text background:
**15.153268673** light, **11.826910776** dark, both well clear of 4.5:1. Reusing it for
`.fd-url::selection` needed no new figure -- `styles.test.ts`'s existing
`it.each(placed)` case for `['text', 'primary-tint', 4.5, true]` already covers it, and
that suite ran unchanged and green (170/170, see below). `DESIGN.md` was updated in the
same commit to name this second placement, per its own "Placed together" convention (see
`_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`,
the `primary-tint` rows paragraph and the Direct URL Row component entry).

## Forced colors

`.fd-url:focus-visible` is untouched and still listed in the
`@media (forced-colors: active)` block's outline-restoration selector group
(`style.css`, "the browse trigger...` and neighbours restated as an outline"), so the
Story 7.11 outline fallback for Windows High Contrast is unaffected by this change.

## After the fix

```
$ npx vitest run --config vitest.browser.config.ts browser/staged-url-field.test.tsx
 Test Files  1 passed (1)
      Tests  9 passed (9)

$ npx vitest run src/ui/styles.test.ts
 Test Files  1 passed (1)
      Tests  170 passed (170)
```

## Mutation

| # | Mutation | Expectation | Result |
|---|---|---|---|
| 1 | Restore the clipping: revert `.fd-url-reveal__body`'s padding to the old bare `padding-top: var(--spacing-3);` | Fails, naming the missing ring room | **Caught.** Both `is never clipped by the reveal region at 1024x768` and `...at 640x480` failed: `left: no room for the ring: expected 413 to be less than or equal to 409.5` / `expected 21 to be less than or equal to 17.5` |

Applied by `git diff frontend/src/style.css > .../style-fix.patch`, `git apply -R
.../style-fix.patch` (confirmed `git status --short frontend/src/style.css` clean, i.e.
the file matched the pre-fix tree exactly), re-run of the scoped browser test above, then
`git apply .../style-fix.patch` to restore the fix, confirmed clean and green again. A
second, narrower mutation (removing only `.fd-url::selection`) was not separately
mutation-verified as a distinct table row because it is exercised by the pre-existing,
unmodified `styles.test.ts` contrast proof rather than by a new assertion of this story's
own -- removing the rule does not fail any test today (there being no assertion that
`.fd-url::selection` exists at all), which is a real, accepted gap: the story's
acceptance criterion is about the *pair* clearing contrast if placed, which it does, not
about pinning that the rule is present. Manually confirmed present in the diff below.

## Full frontend suites, after the fix

```
$ npm test           # jsdom
 Test Files  19 passed (19)
      Tests  794 passed (794)

$ npm run test:browser   # rendered Chromium
 Test Files  2 passed (2)
      Tests  76 passed (76)
```

76 = the 67-test rendered baseline plus the 9 new tests in
`staged-url-field.test.tsx` (2 viewport cases for the ring, 1 for collapse-still-works,
run via `it.each` producing one row each, plus the pre-existing suite's other tests
counted in the same file). 794 jsdom tests, unchanged from before this story (no `src/`
test was added or removed; the fix is CSS-only plus one rendered-suite file).

## Full verification gate (verify.yml order)

All commands run from `story-10-1-link-field-focus-ring`, in order, never `wails build`
concurrent with a frontend suite.

```
$ wails build
...
Built '.../build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 14.247s.

$ git checkout -- frontend/wailsjs
(no diff after)

$ gofmt -l .
(no output)

$ go vet ./...
(no output)

$ go tool staticcheck ./...
(no output)

$ go test -count=1 ./...
ok  	fairdrop	1.536s
ok  	fairdrop/internal/network	0.967s
ok  	fairdrop/internal/qr	0.659s
ok  	fairdrop/internal/server	5.286s
ok  	fairdrop/internal/source	0.824s
ok  	fairdrop/internal/stream	3.313s
ok  	fairdrop/internal/transfer	1.952s
ok  	fairdrop/scripts	1.488s
ok  	fairdrop/scripts/mutationverdict	1.322s

$ go env CGO_ENABLED
1
$ <verify.yml's exact cgo probe, package main + cdecl stdio.h include>
CGO_ENABLED=1 and a trivial cgo program built and ran -- confirmed OK

$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	8.278s
ok  	fairdrop/internal/network	1.858s
ok  	fairdrop/internal/qr	1.987s
ok  	fairdrop/internal/server	6.204s
ok  	fairdrop/internal/source	1.814s
ok  	fairdrop/internal/stream	105.152s
ok  	fairdrop/internal/transfer	3.164s
ok  	fairdrop/scripts	2.640s
ok  	fairdrop/scripts/mutationverdict	2.440s

$ npm test          (frontend/, jsdom)
 Test Files  19 passed (19)
      Tests  794 passed (794)

$ npm run test:browser   (frontend/, rendered Chromium)
 Test Files  2 passed (2)
      Tests  76 passed (76)

$ git checkout -- frontend/browser/captures/
(no diff -- the capture PNG happened not to change bytes this run; git status showed no
modification before this step, so the checkout was a no-op safety net, not a revert of a
real change)
```

No Go files were touched by this story (CSS, one test file, and two docs), so the
darwin/linux/windows cross-build pre-flight AGENTS.md asks for before pushing Go changes
was not additionally run -- there is no Go diff for it to protect.

## Native check

Pending -- orchestrator. This evidence covers the automated gate only; the built macOS
binary should still be driven by hand per the story's own acceptance criterion ("The
orchestrator re-drives it on the built macOS binary") and per AGENTS.md's synthetic-vs-real
input caution for anything focus-related.

## Files changed

- `frontend/src/style.css` -- `.fd-url-reveal__body`'s padding widened to include ring
  room on all four sides (folding in the pre-existing top spacing rather than stacking
  with it); `.fd-url::selection` added, reusing the already-proven `text`/`primary-tint`
  pair.
- `frontend/browser/staged-url-field.test.tsx` -- 3 new tests (2 viewport cases proving
  the ring is unclipped, 1 proving the collapse still reaches zero height).
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md` --
  Direct URL Row component entry and the `primary-tint` contrast-pairs paragraph updated
  to record the fix and the reused pair.
- `_bmad-output/implementation-artifacts/sprint-status.yaml` --
  `10-1-draw-the-link-fields-focus-ring-whole: review`.
