# Evidence: stray UA focus ring and undersized browse chevron (defect fixes)

Defect-fix carve-out (AGENTS.md "Git workflow"): two owner-observed defects, a failing
test written first for each, both fixes mutation-verified. Branch `epic-7-quartz`, no
story.

## Defect 1: a stray blue system focus ring (regression, mine)

### The defect

Owner-observed on the built binary: pressing Escape closes the browse menu correctly,
but whatever receives focus afterwards shows a blue macOS system focus ring in addition
to the product's mocha two-tone ring. The owner's screenshot showed it around the
"Local network access" disclosure summary.

### Diagnosis

Story 7.11 replaced every focus ring's `outline` with a stacked `box-shadow`:

```css
.fd-button:focus-visible,
.fd-url:focus-visible,
.fd-button[data-focus-return] {
    box-shadow:
        0 0 0 var(--focus-ring-offset) var(--color-surface),
        0 0 0 calc(var(--focus-ring-offset) + var(--focus-ring-width)) var(--color-primary);
}
```

`outline` and `box-shadow` are not the same mechanism: setting `outline` **replaces**
the user agent's own default focus outline, but `box-shadow` is an independent paint
layer that does not suppress it. WebKit kept drawing its own default outline underneath
the shadow ring, so any focused control painted two rings at once. This was true on
every ringed surface, not only the disclosure the owner saw it on:

- `.fd-button:focus-visible` / `.fd-url:focus-visible` / `.fd-button[data-focus-return]`
  (the browse trigger, `.fd-url`, and every other plain `.fd-button`)
- `.fd-disclosure__summary:focus-visible` (the disclosure the owner reported)
- `.fd-browse-menu .fd-button:focus` (the browse menu items)

The routed landing targets (`[data-focus-target]:focus-visible`) were already correct --
that rule already carried `outline: none;` from Story 7.10's stuck-"selected" fix -- and
stay unaffected by this change.

The forced-colors block (`style.css` around line 1699, now ~1730) deliberately restores
a real `outline: var(--focus-ring-width) solid Highlight;` for the same three ring
selectors, because Windows High Contrast strips decorative `box-shadow`. That block is
declared after the normal-mode rules and inside its own `@media (forced-colors: active)`
block, so it continues to win in that mode; the fix below only ever adds `outline: none;`
outside forced colors.

### Failing test, written first, captured before any fix

Three new tests were added -- to `the focus indicator` and `the browse menu surface`
describe blocks in `frontend/src/ui/styles.test.ts`, and one to a new describe block in
`frontend/src/ui/IdleView.test.tsx` for the chevron defect below -- run against the
untouched tree:

```
 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend

 ❯ src/ui/styles.test.ts (141 tests | 3 failed) 24ms
     × suppresses the UA default outline on every box-shadow ring, so WebKit cannot paint its own blue ring underneath the product ring (regression fix) 4ms
     × gives the browse trigger chevron the same 12x12 border-chevron mechanism as the disclosure, not a text glyph (defect fix) 1ms
     × suppresses the UA default outline on the focused menu item too, for the same reason as the shared ring rule (regression fix) 1ms
 ❯ src/ui/IdleView.test.tsx (65 tests | 1 failed) 335ms
     × renders the chevron as the shared border-chevron element, not a text glyph 7ms

⎯⎯⎯⎯⎯⎯⎯ Failed Tests 4 ⎯⎯⎯⎯⎯⎯⎯

 FAIL  src/ui/IdleView.test.tsx > the browse trigger chevron matches the disclosure chevron family (defect fix) > renders the chevron as the shared border-chevron element, not a text glyph
AssertionError: expected '⌄' to be '' // Object.is equality

 FAIL  src/ui/styles.test.ts > the focus indicator > suppresses the UA default outline on every box-shadow ring, so WebKit cannot paint its own blue ring underneath the product ring (regression fix)
AssertionError: .fd-button:focus-visible,: expected '.fd-button:focus-visible,\n.fd-url:fo…' to contain 'outline: none;'

 FAIL  src/ui/styles.test.ts > Story 7.3: rebuilding Idle > gives the browse trigger chevron the same 12x12 border-chevron mechanism as the disclosure, not a text glyph (defect fix)
AssertionError: expected '.fd-browse-trigger__chevron {\n    ma…' to contain 'width: 12px;'

 FAIL  src/ui/styles.test.ts > the browse menu surface (Story 7.2) > suppresses the UA default outline on the focused menu item too, for the same reason as the shared ring rule (regression fix)
AssertionError: expected '.fd-browse-menu .fd-button:focus {\n …' to contain 'outline: none;'

 Test Files  2 failed (2)
      Tests  4 failed | 202 passed (206)
```

### The fix

`outline: none;` added alongside the `box-shadow` in all three normal-mode ring rules in
`frontend/src/style.css`: `.fd-button:focus-visible, .fd-url:focus-visible,
.fd-button[data-focus-return]`, `.fd-disclosure__summary:focus-visible`, and
`.fd-browse-menu .fd-button:focus`. The forced-colors block is untouched -- it sets a
real `outline` value, which only applies inside its own `@media` block and still wins
there because it is declared later.

Three pre-existing tests (`the focus indicator > draws a two-tone ring...`, `...applies
the identical two-tone ring to the disclosure summary...`, and `the browse menu surface
> distinguishes the focused item...`) asserted `not.toMatch(/outline:/)` on these same
rule blocks -- true of the buggy tree, since no outline appeared there at all. Those
assertions were narrowed to `not.toMatch(/outline:(?!\s*none\b)/)` so they still refuse a
*real* outline value (which would re-fight the box-shadow the way Story 7.11 retired)
while accepting the `outline: none;` this fix adds.

### After the fix

```
 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend

 Test Files  2 passed (2)
      Tests  206 passed (206)
```

## Defect 2: the browse control's chevron is too small

### The defect

Owner-observed: the chevron at the right edge of "Choose a file or folder" looks tiny
and thin next to the disclosure chevrons.

### Diagnosis

Two different mechanisms were in play:

- `.fd-disclosure__chevron` (the disclosure marker): a 12x12 box with `border-right` /
  `border-bottom` of `2px solid var(--color-muted)` rotated 45deg, animating its
  rotation on open.
- `.fd-browse-trigger__chevron` (the trigger): a **text glyph**,
  `<span className="fd-browse-trigger__chevron" aria-hidden="true">⌄</span>` (U+2304),
  styled only with `margin-inline-start: auto`. A text glyph at the control's font size
  renders small and hairline-thin -- exactly the "tiny and thin" the owner saw.

### Failing test, written first, captured before any fix

See the combined capture above -- `IdleView.test.tsx`'s `renders the chevron as the
shared border-chevron element, not a text glyph` and `styles.test.ts`'s `gives the
browse trigger chevron the same 12x12 border-chevron mechanism as the disclosure, not a
text glyph (defect fix)` both failed on the untouched tree, the former because the
rendered element still carried the `⌄` text node, the latter because
`.fd-browse-trigger__chevron` carried only `margin-inline-start: auto`.

### The fix

`.fd-browse-trigger__chevron` in `frontend/src/style.css` now uses the identical
border-chevron mechanism as `.fd-disclosure__chevron` -- 12x12 box, 2px
`border-right`/`border-bottom` -- but coloured `var(--color-primary-ink)` rather than
`var(--color-muted)`, since the trigger is a filled primary control (mocha fill), not a
surface row the disclosure summary sits on. `frontend/src/ui/IdleView.tsx`'s trigger
markup drops the `⌄` text glyph, leaving `<span className="fd-browse-trigger__chevron"
aria-hidden="true"/>` -- an empty decorative box, same as before for accessibility
(`aria-hidden` kept; the accessible name still comes from the button's label).

**Rotation: no rotation, chosen deliberately.** The disclosure rotates because a
`<details>` expands in place -- the same element's summary keeps its position and the
chevron communicates the now-open content right beneath it. The browse trigger's menu
pops over the surface instead; the trigger itself does not change state the way an
expanded `<details>` does, and the platform's own popup-button affordance (a menu
button) does not rotate its indicator when the menu opens. Rotating it here would imply
an expand/collapse relationship the control does not have, so the chevron stays static,
always pointing down. (`aria-expanded` on the trigger already communicates open/closed
state to assistive technology; the glyph is decoration only.)

### After the fix

```
 RUN  v4.1.11 /Users/jaesonmartin/Projects/FairDrop/frontend

 Test Files  2 passed (2)
      Tests  206 passed (206)
```

## Full frontend suites, after both fixes

```
$ npm test           # jsdom
 Test Files  17 passed (17)
      Tests  662 passed (662)

$ npm run test:browser   # rendered Chromium
 Test Files  2 passed (2)
      Tests  23 passed (23)
```

662 = the 658-test baseline plus the 4 new failing-first tests above (3 in
`styles.test.ts`, 1 in `IdleView.test.tsx`). The rendered-Chromium suite is unchanged at
23, since neither defect needed a rendered-engine assertion -- both are provable from the
stylesheet and the component markup, per the task brief.

## Mutation table

| # | Mutation | Expectation | Result |
|---|---|---|---|
| 1 | Remove the `outline: none;` suppression from all three normal-mode ring rules (`.fd-button:focus-visible,...`, `.fd-disclosure__summary:focus-visible`, `.fd-browse-menu .fd-button:focus`) | Fails, naming the missing suppression (double ring) | **Caught.** `styles.test.ts` failed 2 assertions: `suppresses the UA default outline on every box-shadow ring...` and `...on the focused menu item too...`, both `expected ... to contain 'outline: none;'` |
| 2 | Return the browse trigger to a text glyph (`<span ...>⌄</span>`) | Fails, naming the reintroduced glyph | **Caught.** `IdleView.test.tsx`'s `renders the chevron as the shared border-chevron element, not a text glyph` failed: `expected '⌄' to be ''` |
| 3 | Break the forced-colors `outline` restoration (`outline: var(--focus-ring-width) solid Highlight;` → `outline: none;`) | Fails, naming the lost Windows High Contrast fallback | **Caught.** Pre-existing test `forced colors > restates every box-shadow focus ring as an outline, because Windows High Contrast Mode strips decorative box-shadow (Story 7.11)` failed: `expected ... to contain 'outline: var(--focus-ring-width) solid Highlight;'` |

Each mutation was applied to `frontend/src/style.css` (and, for #2,
`frontend/src/ui/IdleView.tsx`), run individually against `npx vitest run
src/ui/styles.test.ts` / `src/ui/IdleView.test.tsx`, confirmed to fail naming the
regression, then reverted with `git diff --stat` confirmed clean before the next
mutation.

## Scope check: no double ring beyond the disclosure

The owner saw the defect on the disclosure summary, but the cause -- `box-shadow` not
suppressing the UA outline -- applied identically to every rule built the same way:
the browse trigger and `.fd-url` (`.fd-button:focus-visible` group), the
`[data-focus-return]` scripted-return marker (same selector group), and the browse
menu items (`.fd-browse-menu .fd-button:focus`). All three normal-mode ring rules
needed the same `outline: none;` addition; none was already correct. The routed landing
targets (`[data-focus-target]:focus-visible`) were already correct from Story 7.10 and
needed no change.

## Full verification gate (verify.yml order)

All commands run from a clean `epic-7-quartz` checkout, in order, never `wails build`
concurrent with a frontend suite.

```
$ wails build
...
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 7.261s.
```

`frontend/wailsjs/go/main/App.d.ts`, `App.js`, and `frontend/wailsjs/go/models.ts` were
flipped to mode 755 by the build with no content change; `chmod 644` restored them and
`git status --short` showed no diff on those three files afterward.

```
$ gofmt -l .
(no output)

$ go vet ./...
(no output)

$ go tool staticcheck ./...
(no output)

$ go test -count=1 ./...
ok  	fairdrop	1.893s
ok  	fairdrop/internal/network	0.414s
ok  	fairdrop/internal/qr	0.587s
ok  	fairdrop/internal/server	4.379s
ok  	fairdrop/internal/source	0.961s
ok  	fairdrop/internal/stream	3.874s
ok  	fairdrop/internal/transfer	1.728s
ok  	fairdrop/scripts	1.587s
ok  	fairdrop/scripts/mutationverdict	1.261s

$ go env CGO_ENABLED
1
$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	9.116s
ok  	fairdrop/internal/network	1.244s
ok  	fairdrop/internal/qr	2.478s
ok  	fairdrop/internal/server	5.781s
ok  	fairdrop/internal/source	2.308s
ok  	fairdrop/internal/stream	100.323s
ok  	fairdrop/internal/transfer	1.779s
ok  	fairdrop/scripts	2.794s
ok  	fairdrop/scripts/mutationverdict	2.606s

$ npm test          (frontend/, jsdom)
 Test Files  17 passed (17)
      Tests  662 passed (662)

$ npm run test:browser   (frontend/, rendered Chromium)
 Test Files  2 passed (2)
      Tests  23 passed (23)

$ GOOS=windows GOARCH=amd64 go build ./...
(no output)
```

`frontend/browser/captures/qr-panel-forced-colors.capture.png` was rewritten by
`test:browser` (non-deterministic PNG encoding, per AGENTS.md) and reverted with `git
checkout -- frontend/browser/captures/` before committing.

No debug instrumentation was present before `wails build` ran -- confirmed by grep for
`keyprobe` across `frontend/src`, `main.go`, and `app.go` before the build, per the
"remove instrumentation before building" pitfall in the task brief.

## Files changed

- `frontend/src/style.css` -- `outline: none;` on the three normal-mode ring rules;
  `.fd-browse-trigger__chevron` redrawn as the shared border-chevron mechanism.
- `frontend/src/ui/IdleView.tsx` -- browse trigger chevron span drops its text glyph.
- `frontend/src/ui/styles.test.ts` -- 3 new tests (2 ring-outline regression tests, 1
  chevron-mechanism test); 3 pre-existing `not.toMatch(/outline:/)` assertions narrowed
  to permit `outline: none;` while still refusing a real outline value.
- `frontend/src/ui/IdleView.test.tsx` -- 1 new test asserting the trigger's chevron is
  the shared border-chevron element rather than a text glyph.
