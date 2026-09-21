# Evidence: Story 7.9 — Make Resizing Seamless

## Summary

Replaced the JavaScript sizing of the Staged direct-URL field (a
`useLayoutEffect` measuring `scrollHeight`, kept live across a window resize
by a `ResizeObserver` whose write was deferred a frame through
`requestAnimationFrame`) with a pure-CSS grid + hidden-mirror technique. The
field's height is now determined entirely by layout: a `display: grid`
wrapper (`.fd-url-wrap`) places the real `<textarea>` and a hidden mirror
element (`.fd-url-mirror`) in the same grid cell (`grid-area: 1 / 1`); the
mirror is a plain block carrying the same URL text, so its wrapped height
sets the cell's height, and the textarea stretches to fill it. A width change
re-wraps the mirror and resizes the cell inside the same layout pass
everything else on the page re-wraps in — no observer, no
`requestAnimationFrame`, no width-change bookkeeping, no JavaScript at all.

`StagedView.tsx` is **74 lines shorter** net (54 insertions, 128 deletions).

Added a width-sweep test in `frontend/browser/staged-url-field.test.tsx`
(1200px down to 320px in steps of 40) that measures, at every step: the field
never clips, the page never opens a horizontal scrollbar, no tracked content
pair overlaps, and the layout passes through exactly the two arrangements the
759px reflow breakpoint defines — replacing "looks smooth" with a
measurement.

## One deviation from the story's literal implementation sketch, found by mutation and fixed

The story text proposed passing the URL to the wrapper as a `data-*`
attribute and mirroring it with a `::after` pseudo-element reading
`content: attr(...)`. That was the first thing implemented, and it broke
three pre-existing tests in `frontend/browser/accessibility.test.tsx` (the
WCAG 1.4.12 text-spacing override suite, D-068) the first time the full gate
ran:

```
FAIL browser/accessibility.test.tsx > WCAG 1.4.12 text-spacing overrides (D-068) > keeps Staged unclipped under all four overrides
AssertionError: textarea.fd-url.fd-target "..." clips its content at 200% text:
scrollHeight (96px) exceeds its own clientHeight (94px)
```

(and two more of the same shape, plus a fourth failure in the 200%-text-alone
case). The cause: `accessibility.test.tsx` simulates a *reader's* text-spacing
override exactly the way the WCAG 1.4.12 bookmarklet does, with a bare
`* { line-height: 1.5 !important; letter-spacing: ...; word-spacing: ...; }`
rule. A bare universal selector does not match generated pseudo-element
content, so the `::after` mirror kept the sheet's own `--leading-code` while
the real textarea (a genuine element, matched by `*`) jumped to `1.5` —
desyncing the two and clipping the field the moment such an override was in
effect, in both engines equally (this is a selector-matching fact, not a
WebKit/Blink difference).

**Fix:** the mirror is a real sibling `<div className="fd-url-mirror" aria-hidden="true">`
carrying the URL as ordinary text content, not a `::after` reading a `data-*`
attribute. Being a genuine DOM element, it is matched by the same bare `*`
rule the textarea is, so the two can never desync under a reader's override.
Everything else about the technique (grid wrapper, shared cell, shared
design-token box model, `visibility: hidden`) is unchanged from the story's
intent. This is reported here per the task's own instruction: "if the CSS
technique cannot reproduce the JavaScript version's behaviour in both
engines, say so plainly." It does reproduce it, in both engines, with this
one substitution — the literal `attr()`/`::after` sketch does not, in either
engine, under a real accessibility use case this repo already tests for.

## A second, smaller consequence, fixed the same way as the first

The real mirror element still needs to carry the capability token
(`.fd-url-mirror`'s text content) somewhere in the DOM for CSS to size against
it, which means the token now appears twice in serialized HTML instead of
once. `src/ui/StagedView.test.tsx`'s `exposes the capability token ... exactly
once` test pinned the old count. It was updated (not deleted) to assert
`occurrences === 2` and that the second occurrence is inert: inside
`.fd-url-mirror`, which carries `aria-hidden="true"` and (per style.css)
`visibility: hidden` — never a link, never prose, never a second reading. This
is the one test outside `frontend/browser/staged-url-field.test.tsx` that
needed a substantive edit; the reasoning is inline as a comment at the
assertion site.

`frontend/browser/staged-url-field.test.tsx` — the file the story calls out
by name — was **only appended to**: `git diff --stat` shows 112 insertions, 0
deletions for it. Every original case is byte-for-byte unchanged and passes.

## Mutation table

All commands run from `/Users/jaesonmartin/Projects/FairDrop/frontend`. Every
mutation was reverted and the full pair of suites reconfirmed green
(632 jsdom / 23 rendered) before moving to the next.

| # | Acceptance criterion | Mutation | Command | Result |
|---|---|---|---|---|
| 1 | Height determined entirely by CSS layout | `.fd-url { block-size: auto; align-self: start; }` — the field falls back to its native `rows`-based intrinsic height, ignoring the grid cell (equivalent to "reintroduce fixed sizing with no CSS sizing driving it") | `npm run test:browser` | **10/23 fail**, all naming the exact clip, e.g. `.fd-url clips its value: scrollHeight (76px) exceeds its own clientHeight (42px) for a 68-character URL`. Reproduces the original observed defect exactly. |
| 2 | Mirror hidden from AT via `visibility: hidden`, not `opacity`/position | `.fd-url-mirror { opacity: 0; }` (removed `visibility: hidden`) | `npm test -- --run styles.test` | **2/127 fail** in that file: the new `hides the URL field sizing mirror from assistive technology...` test (names `expect(mirror).toContain('visibility: hidden;')` failing) and the pre-existing `dims nothing with opacity...` sheet-wide guard (`expected ['0'] to deeply equal []`). |
| 2b | Mirror inert to AT | Removed `aria-hidden="true"` from the mirror `<div>` in `StagedView.tsx` | `npm test -- --run StagedView.test` | **1/40 fails**, naming it: `exposes the capability token as readable content exactly once...` — `expected null to be 'true'`. |
| 3 | Mirror and field share font/padding/border/line-height tokens | `.fd-url-mirror`'s `padding` changed to `var(--spacing-1)` and `line-height` changed to the literal `1` (both diverging from `.fd-url`'s tokens) | `npm run test:browser` then `npm test -- --run styles.test` | Rendered: **10/23 fail**, naming the clip (e.g. `scrollHeight (94px) exceeds ... clientHeight (44px)`). jsdom: **2/127 fail** in the new `it.each` token-sharing test, naming exactly `shares line-height: var(--leading-code); between the URL field and its sizing mirror`. |
| 4 | Existing `staged-url-field.test.tsx` cases stay in place | (verification only, no mutation) | `git diff --stat -- frontend/browser/staged-url-field.test.tsx` | `112 insertions(+), 0 deletions(-)` — every original case byte-identical. |
| 5 | Width sweep catches a pinned height | Same as mutation #1 (`.fd-url { block-size: auto; align-self: start; }`) | `npm run test:browser -- staged-url-field -t "never clips"` | **1/1 fails**, naming the first width: `Error: at 1200px, .fd-url clips its value: scrollHeight (94px) exceeds its own clientHeight (42px) for a 96-character URL`. (The sweep starts at 1200px and descends, so with the field pinned to its intrinsic height throughout, the very first width already clips — the sweep's per-iteration `try/catch` wraps the shared clip assertion with `at ${width}px, ...` specifically so this case names the width rather than only the symptom.) |
| 6 | No `ResizeObserver`/`requestAnimationFrame`/height measurement in `StagedView.tsx` | (verification only) | `grep -n "ResizeObserver\|requestAnimationFrame\|useLayoutEffect\|scrollHeight" frontend/src/ui/StagedView.tsx` | No matches — confirmed below. |

```
$ grep -n "ResizeObserver\|requestAnimationFrame\|useLayoutEffect\|scrollHeight" frontend/src/ui/StagedView.tsx
$
```

## Breakpoint-arrangement sweep, in detail

`frontend/browser/staged-url-field.test.tsx`'s new
`Staged view resizes seamlessly across a continuous width sweep (Story 7.9)`
describe block:

- Renders `StagedView` once with the deliberately longer IPv6-host capability
  URL (the case most likely to clip if the mirror and field ever drift).
- Steps the viewport from 1200px to 320px in steps of 40 (23 widths total,
  covering the 759px reflow breakpoint and the 320px reflow floor from both
  directions).
- At every step, asserts:
  - `.fd-url`'s `scrollHeight <= clientHeight` (no clipping), with the
    current width folded into the failure message.
  - `document.documentElement.scrollWidth <= clientWidth` (no page-level
    horizontal scrollbar).
  - `.fd-hero__details` and `.fd-qr-panel` bounding rects never intersect.
  - `.fd-url-wrap` and the copy `<button>` bounding rects never intersect.
- After the sweep, asserts the set of distinct `(fd-hero track count, 
  fd-direct-row track count)` arrangements visited has exactly **2** members
  (style.css's "Reflow" section defines exactly one breakpoint, at 759px, for
  both `.fd-hero` and `.fd-direct-row`) and that the sequence of arrangements
  transitions **exactly once** across the whole descending sweep — no width
  produces a transient third arrangement, and the layout never reverts to the
  wider arrangement once it has narrowed.

## Full gate, run locally in `verify.yml` order

All commands run from `/Users/jaesonmartin/Projects/FairDrop` (Go) or
`/Users/jaesonmartin/Projects/FairDrop/frontend` (npm), one after another,
never `wails build` concurrent with a frontend suite.

1. `wails build` — succeeded: `Built '.../fairdrop.app/Contents/MacOS/fairdrop' in 7.601s.`
   Regenerated `frontend/wailsjs/go/main/App.d.ts`, `App.js`, and
   `frontend/wailsjs/go/models.ts` at mode 755; `chmod 644` applied to all
   three. `git diff --stat` showed no content diff for them afterward (this
   story never touches the exported `App` command surface).
2. `gofmt -l .` — no output (nothing to reformat).
3. `go vet ./...` — no output (clean).
4. `go tool staticcheck ./...` — no output (clean).
5. `go test -count=1 ./...` — all packages `ok` (`fairdrop`,
   `internal/network`, `internal/qr`, `internal/server`, `internal/source`,
   `internal/stream`, `internal/transfer`, `scripts`,
   `scripts/mutationverdict`).
6. `CGO_ENABLED=1 go test -count=1 -race ./...` — confirmed `go env
   CGO_ENABLED` prints `1` first; all packages `ok` under the race detector.
7. `npm test -- --run` (jsdom suite) — **632/632 pass**, 17 files (622
   previously + 10 new: the updated `StagedView.test.tsx` assertion counts as
   an edit not an addition, and `styles.test.ts` gained 1 + 9 (`it.each`)
   assertions for the mirror's AT-hiding and shared box-model tokens).
8. `npm run test:browser` — **23/23 pass**, 2 files (`accessibility.test.tsx`
   unchanged at 17; `staged-url-field.test.tsx` at 6 — the 5 pre-existing
   cases, byte-identical, plus the 1 new width-sweep case).
9. `GOOS=windows GOARCH=amd64 go build ./...` — succeeded, no output.

Pre-flight (not part of the CI matrix, but named in AGENTS.md "Running and
verifying"):
- `GOOS=darwin GOARCH=arm64 go build ./...` — succeeded.
- `GOOS=linux GOARCH=amd64 go build ./...` — succeeded.

`frontend/browser/captures/qr-panel-forced-colors.capture.png` is regenerated
by every `test:browser` run (a live screenshot, not a fixture); it was
reverted with `git checkout --` after every run in this evidence, since this
story touches nothing that capture proves.

## Line-count claim

```
$ git diff --stat -- frontend/src/ui/StagedView.tsx
 frontend/src/ui/StagedView.tsx | 182 +++++++++--------------------------
 1 file changed, 54 insertions(+), 128 deletions(-)
```

Net **-74 lines** in `StagedView.tsx`: the entire sizing `useLayoutEffect`
(the `useRef`s, the `resize()` closure, the border-compensation comment, the
`ResizeObserver` construction, its width-change guard, and its loop-detector
comment) is gone, replaced by a two-element wrapper in the JSX and a doc
comment explaining the CSS technique and the WCAG deviation above.

## Files changed

- `frontend/src/ui/StagedView.tsx` — the fix: JS sizing effect deleted,
  textarea wrapped in `.fd-url-wrap` beside a hidden `.fd-url-mirror` sibling.
- `frontend/src/style.css` — `.fd-url-wrap` (grid), `.fd-url-mirror` (hidden
  mirror, shares design tokens with `.fd-url`), and `.fd-url` updated for
  `grid-area`/`block-size: 100%`/`overflow: hidden`.
- `frontend/browser/staged-url-field.test.tsx` — appended only: the new
  width-sweep describe block. Every prior case unchanged.
- `frontend/src/ui/StagedView.test.tsx` — one assertion updated (token
  occurrence count 1 → 2, with the reasoning inline) to match the real DOM
  shape the CSS mirror technique requires.
- `frontend/src/ui/styles.test.ts` — new assertions: the mirror's
  `visibility: hidden` guarantee, and the shared-token proof between `.fd-url`
  and `.fd-url-mirror`.
