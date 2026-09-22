# Evidence: Escape clears focus, and the disclosure body clears the focus ring

Two owner-observed defects, fixed on `epic-7-quartz` under the defect-fix
carve-out in `AGENTS.md`'s Git workflow section (already observed, failing
test written first, mutation-verified, no story).

## Change 1: Escape clears focus rather than creating it

**Owner's words:** "I feel like escape should remove ANY highlighting of the
tabs, not add it in. I feel like that makes more sense."

### Root cause

Story 7.10 added `[data-focus-return]` so `closeAndReturnFocus` would paint a
visible ring on the browse trigger after a scripted focus move, because
WebKit does not match `:focus-visible` for an element focused by script
(AGENTS.md, "macOS WebKit focus behaviour," item 2). `BrowseControl`'s
`closeAndReturnFocus` was the single function behind *both* of its own
scripted focus moves: Escape, and choosing an item by keyboard. That made the
ring correct for the second case (the sender is still navigating) and wrong
for the first (Escape means "get me out of this," not "put me somewhere").

The disclosure summary (`<details>`/`<summary>`) and `.fd-url` had no Escape
handling at all -- nothing to dismiss, so nothing cleared focus there either.

### Fix

- `frontend/src/ui/IdleView.tsx`: split `closeAndReturnFocus` (still used
  only by `choose`, unchanged -- keeps the marker and the ring) from a new
  `closeAndBlur` (used by both of `BrowseControl`'s own Escape branches --
  `handleMenuKeyDown` and the defensive fallback in `handleTriggerKeyDown`).
  `closeAndBlur` blurs `document.activeElement` explicitly, then closes the
  menu; it never touches `[data-focus-return]`.
- `frontend/src/ui/Disclosure.tsx`: added `handleSummaryKeyDown`, wired to
  the `<summary>`'s `onKeyDown`. Escape blurs the summary. No native
  `<details>` Escape behaviour existed to preserve -- there is nothing open
  to dismiss on this element -- so this is purely the blur.
- `frontend/src/ui/StagedView.tsx`: same one-line Escape-blurs-the-field
  handler added to `.fd-url`'s `onKeyDown`, for consistency with the other
  keyboard-operable controls the scope names, even though no owner report
  named this field specifically.

Scope respected: only the four keyboard-operable controls named in the task
(browse trigger, `.fd-url`, `.fd-disclosure__summary`, browse menu items) got
an Escape-blur path. Routed landing targets (`[data-focus-target]` -- state
headings, the cancel summary, the outcome panel) were never given an Escape
handler at all, so they cannot be blurred by this change; a dedicated test
below confirms a focused landing target survives Escape untouched.

### The trade-off, recorded in both places (code comment and here)

With nothing focused after Escape, the next Tab restarts from the top of the
document instead of continuing from wherever the sender was. In Idle there
are three tab stops, so the cost is small. No WCAG 2.4.7 obligation is left
unmet: that requirement is about the visibility of *a* focused component's
indicator, and after Escape there is not a focused component for the
requirement to apply to. See the doc comment on `closeAndBlur` in
`IdleView.tsx` for the same reasoning kept where the next reader will find it
without this file.

### Keeping the marker alive on the keyboard-choose path while removing it from Escape

`closeAndReturnFocus` (still called only by `choose`, itself called only by
each menu item's `onClick`) is untouched: it still calls `triggerRef.current
?.focus()` and `setTriggerFocusReturned(true)`, so `[data-focus-return]` is
still set -- and still styled -- exactly as before for "File" or "Folder"
chosen by keyboard. `closeAndBlur` is a new, separate function used only by
the two Escape call sites; it shares no code path with `closeAndReturnFocus`
and never sets the flag. The regression test that used to prove the marker's
lifecycle via Escape ("clears the trigger scripted-return marker on blur")
was rewritten to prove it via the keyboard-choose path instead, since that is
now the only path that ever sets it.

### Did scoping the blur conflict with anything the existing handlers defend?

No. `handleMenuBlur` (focus leaving the menu on its own -- Tab, or a pointer
landing elsewhere) is unrelated: it fires from a *blur event bubbling up to
the menu container*, not from a call this change makes, and Escape's
`event.preventDefault()` plus early return already bypass it exactly as
before. The `Tab` branch in `handleMenuKeyDown` and the click-outside path
are both untouched. The one interaction checked closely: calling
`document.activeElement.blur()` on the menu container itself (the
pointer-open case, where the *container* -- not an item -- holds focus) does
synchronously fire the container's own `onBlur={handleMenuBlur}` before
`closeAndBlur`'s own `setOpen(false)` runs. `handleMenuBlur` reads
`event.relatedTarget` (`null` from an explicit `.blur()` call), which is
neither inside the menu nor the trigger, so it calls `setOpen(false)` itself
-- redundant with, but not in conflict with, the call `closeAndBlur` makes
right after. No new state is left inconsistent by the double call (React
bails out a no-op `setOpen(false)` on the second one).

## Change 2: the disclosure body's top padding now clears the focus ring

**Owner's words:** "these two tabs at the bottom when they have the
highlighting it sort of covers the text, so maybe we need to offset them
down a bit more as well."

### Root cause

`.fd-disclosure__body` was `padding: 0 var(--spacing-4) var(--spacing-4)` --
**no top padding at all** -- so the open body's first line sat flush against
the summary's bottom edge, exactly where `.fd-disclosure__summary:focus-visible`'s
two-tone ring (`--focus-ring-offset` 2px + `--focus-ring-width` 2px = 4px
total) extends beyond the summary's own box.

### Fix

`frontend/src/style.css`: `.fd-disclosure__body` is now `padding:
var(--spacing-3) var(--spacing-4) var(--spacing-4)`.

**Token chosen: `--spacing-3` (12px).** Reasoning:

- It clears the ring's 4px total extent by 8px of real headroom, not the bare
  minimum -- the owner's screenshot showed the body copy sitting too tight
  under the summary even ignoring the ring, so the fix should read as
  "properly spaced," not "just barely not overlapping."
- `--spacing-2` (8px) was considered and rejected: it only doubles the ring's
  extent, and the owner's report reads as wanting more breathing room in the
  open state generally, not the smallest value that technically clears.
- `--spacing-3` (12px) also matches `.fd-disclosure__summary`'s own internal
  `gap: var(--spacing-3)` (the row gap between the heading and the chevron),
  so the open disclosure reads as one consistent vertical rhythm rather than
  two unrelated spacing values living side by side in the same component.

### Checked elsewhere for the same class of overlap

Every other ringed element in the stylesheet was checked for a sibling
sitting inside its own ring's reach (4px, from the same
`--focus-ring-offset` + `--focus-ring-width` pair used everywhere per the
"one focus indicator for the whole app" comment on `:root`):

- `.fd-browse-menu` (the browse trigger's own ring, and the browse menu
  items' ring) is separated from the trigger by `margin-block-start:
  var(--spacing-2)` (8px) -- clears the 4px extent.
- Between browse menu items, `gap: var(--spacing-1)` (4px) exactly equals the
  ring's total extent. This is two same-looking controls meeting each
  other's ring, not a ring landing on top of content -- the defect class the
  owner reported and this task scopes to -- so it was left alone; changing it
  was not asked for and risks disturbing DESIGN.md's specified rhythm for
  that menu.
- `.fd-url` (StagedView) sits inside `.fd-direct-row`, a flex row where it is
  followed by the Copy button, not by text content; no overlap.
- No other component places body copy directly beneath a ringed
  `:focus-visible` element with zero top spacing; `.fd-disclosure__body` was
  the only offender.

## Failing-test output, captured before either fix

Both changes' tests were written first, confirmed to fail against the
pre-fix tree, and only then was the production code changed.

```
 ❯ src/ui/styles.test.ts (142 tests | 1 failed) 42ms
     × gives the open disclosure's body enough top padding to clear the focus ring, expressed as a token, not a magic number 6ms
 ❯ src/ui/IdleView.test.tsx (67 tests | 3 failed) 702ms
     × blurs the firewall summary on Escape 16ms
     × closes on Escape, clears focus rather than returning it, and announces nothing 15ms
     × closes on Escape while focus is still on the control, and clears focus rather than leaving the trigger ringed 22ms

⎯⎯⎯⎯⎯⎯⎯ Failed Tests 4 ⎯⎯⎯⎯⎯⎯⎯

 FAIL  src/ui/IdleView.test.tsx > Escape clears focus on a focused disclosure summary, with no menu open > blurs the firewall summary on Escape
AssertionError: expected <summary …(1)>…(2)</summary> not to be <summary …(1)>…(2)</summary> // Object.is equality
 ❯ src/ui/IdleView.test.tsx:261:44
    259|         fireEvent.keyDown(summary, {key: 'Escape'})
    260|
    261|         expect(document.activeElement).not.toBe(summary)
       |                                            ^

 FAIL  src/ui/IdleView.test.tsx > the browse menu > closes on Escape, clears focus rather than returning it, and announces nothing
AssertionError: expected <button type="button" …(5)>…(1)</button> not to be <button type="button" …(5)>…(1)</button> // Object.is equality
 ❯ src/ui/IdleView.test.tsx:400:44
    398|
    399|         expect(screen.queryByRole('menu')).toBeNull()
    400|         expect(document.activeElement).not.toBe(trigger)
       |                                            ^

 FAIL  src/ui/IdleView.test.tsx > the browse menu follows the menu-button pattern > closes on Escape while focus is still on the control, and clears focus rather than leaving the trigger ringed
AssertionError: expected <button type="button" …(4)>…(1)</button> not to be <button type="button" …(4)>…(1)</button> // Object.is equality
 ❯ src/ui/IdleView.test.tsx:907:44
    905|         // through `handleMenuKeyDown`) -- it must clear focus exactly…
    906|         // the primary path does, not leave the trigger focused and ri…
    907|         expect(document.activeElement).not.toBe(control())
       |                                            ^

 FAIL  src/ui/styles.test.ts > Story 7.3: rebuilding Idle > gives the open disclosure's body enough top padding to clear the focus ring, expressed as a token, not a magic number
AssertionError: a spacing token for the body's top padding, not a bare pixel value: expected null to be truthy
 ❯ src/ui/styles.test.ts:709:104

 Test Files  2 failed | 15 passed (17)
      Tests  4 failed | 661 passed (665)
```

(An early version of the stylesheet test read `--focus-ring-offset`/
`--focus-ring-width` from the wrong parsed block -- `@theme` instead of
`:root`, where those two tokens actually live -- and failed with `expected
NaN to be greater than 0` instead of naming the real defect. Fixed to read
from the whole stylesheet before this capture; the run above is the correct
failing-test evidence.)

After both fixes landed, the same run is fully green:

```
 Test Files  17 passed (17)
      Tests  665 passed (665)
```

## Mutation table

| # | Mutation | Result |
|---|----------|--------|
| 1 | `closeAndBlur` no longer calls `.blur()` (only `setOpen(false)`) | **Fails**, naming the ring left behind: `closes on Escape while focus is still on the control, and clears focus rather than leaving the trigger ringed` -- `expected <button> not to be <button>`. (The menu-open-item-focused path is unaffected by this specific mutation because the focused item unmounts on `setOpen(false)` regardless, which the browser itself resolves to `<body>`; the defensive-fallback trigger-focused path is the one that has no unmount to fall back on, and it is what catches the mutation.) |
| 2 | Blur a routed landing target (added an Escape handler to the `[data-focus-target="idle-instruction"]` heading, simulating the scope violation) | **Fails**, naming the landing target: `never blurs a routed landing target, which this path never touches` -- `expected <h1> to be <h1>` (heading no longer equals itself as `activeElement` after the simulated blur). |
| 3 | Remove the body's top padding (`padding: 0 var(--spacing-4) var(--spacing-4)`) | **Fails**, naming the missing token: `gives the open disclosure's body enough top padding to clear the focus ring, expressed as a token, not a magic number` -- `a spacing token for the body's top padding, not a bare pixel value: expected null to be truthy`. |
| 4 | Drop `setTriggerFocusReturned(true)` from `closeAndReturnFocus` (the keyboard-choose path) | **Fails** in two places: `returns focus to the control once a kind is chosen` and `clears the trigger scripted-return marker on blur` -- both `expected null to be ''`. |

Each mutation was applied individually, run, confirmed to fail naming the
right defect, then reverted before the next.

## Gate transcript (verify.yml order, run on this macOS machine)

```
$ wails build
...
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 9.949s.

$ gofmt -l .
(no output -- clean)

$ go vet ./...
(no output -- clean)

$ go tool staticcheck ./...
(no output -- clean)

$ go test -count=1 ./...
ok  	fairdrop	1.909s
ok  	fairdrop/internal/network	0.238s
ok  	fairdrop/internal/qr	0.427s
ok  	fairdrop/internal/server	5.088s
ok  	fairdrop/internal/source	0.586s
ok  	fairdrop/internal/stream	3.954s
ok  	fairdrop/internal/transfer	1.761s
ok  	fairdrop/scripts	1.302s
ok  	fairdrop/scripts/mutationverdict	1.630s

$ go env CGO_ENABLED
1

$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	8.247s
ok  	fairdrop/internal/network	2.246s
ok  	fairdrop/internal/qr	2.356s
ok  	fairdrop/internal/server	5.832s
ok  	fairdrop/internal/source	1.390s
ok  	fairdrop/internal/stream	102.429s
ok  	fairdrop/internal/transfer	2.773s
ok  	fairdrop/scripts	2.793s
ok  	fairdrop/scripts/mutationverdict	2.605s

$ npm test -- --run   (frontend/, jsdom)
 Test Files  17 passed (17)
      Tests  665 passed (665)

$ npm run test:browser   (frontend/, rendered Chromium)
 Test Files  2 passed (2)
      Tests  23 passed (23)

$ GOOS=windows GOARCH=amd64 go build ./...
(no output -- clean)
```

`wails build` and the frontend suites were run one after another, never
concurrently, per the pitfall in `AGENTS.md`. `wails build` flipped
`frontend/wailsjs/go/main/App.d.ts`, `App.js`, and `models.ts` to mode 0755;
`chmod 644` restored them (`git diff` on those three files showed only a mode
change, no content diff, both before and after the restoration).
`frontend/browser/captures/qr-panel-forced-colors.capture.png` was
regenerated by `npm run test:browser` as expected and reverted with `git
checkout --` per the same pitfall -- it changed because the PNG encoder is
not byte-deterministic, not because anything visual changed.

No debug instrumentation was added at any point in this pass, so there was
nothing to remove before the build.

## Files touched

- `frontend/src/ui/IdleView.tsx` -- `closeAndBlur` added, wired to both of
  `BrowseControl`'s own Escape branches; `closeAndReturnFocus` narrowed to
  the keyboard-choose path only.
- `frontend/src/ui/Disclosure.tsx` -- Escape-blurs-the-summary handler added.
- `frontend/src/ui/StagedView.tsx` -- Escape-blurs-the-field handler added to
  `.fd-url`, for scope consistency.
- `frontend/src/style.css` -- `.fd-disclosure__body` top padding.
- `frontend/src/ui/IdleView.test.tsx` -- Escape tests rewritten for the new
  behaviour; two new tests for the disclosure-summary Escape path and the
  landing-target scoping guarantee.
- `frontend/src/ui/styles.test.ts` -- new token-derived test for the body's
  top padding against the ring's total extent.
