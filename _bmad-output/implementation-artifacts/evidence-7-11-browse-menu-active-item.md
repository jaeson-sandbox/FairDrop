# Evidence: Story 7.11 — Give the Browse Menu One Active Item

## Summary

The browse menu (`frontend/src/ui/IdleView.tsx`'s `BrowseControl`) had two
competing ideas of "the item about to be chosen":

1. The open effect (`if (open) firstItemRef.current?.focus()`) fired on
   **every** open, including one caused by a pointer click, so the menu
   always pre-selected `File` even when the sender had just clicked the
   control with a mouse.
2. `:focus` painted the full treatment (mocha fill, tint halo, violet ring —
   Story 7.10) while `.fd-browse-menu .fd-button:hover` painted only a faint
   `background: var(--color-fill)`, so hovering `Folder` while `File` still
   held focus showed the strongly-marked item as the one the sender was
   *not* pointing at.

The fix is a single active item, owned by focus, driven by whichever input
last acted:

- **Pointer open** (`handleTriggerClick`, keyed off `event.detail !== 0` —
  the OS click count a real pointer click carries, versus `0` for a click
  synthesized from Enter/Space) pre-selects nothing; focus stays on the
  trigger.
- **Keyboard open** (ArrowDown/ArrowUp on the trigger, or a keyboard-style
  activation with `event.detail === 0`) still focuses the first item, exactly
  as the pre-existing I/O-matrix rule requires.
- **Hovering an item moves focus to it** (`onMouseEnter={(event) =>
  event.currentTarget.focus()}` on each menu item), so hover and keyboard
  navigation share the one `:focus` rule in `style.css`. The separate,
  weaker `.fd-browse-menu .fd-button:hover` rule is removed.
- **The dead-key gap is closed**: with the menu already open by pointer
  (focus still on the trigger), ArrowDown/ArrowUp on the trigger used to call
  `setOpen(true)` on an already-open menu — no state change, so the open
  effect never re-ran and the key did nothing. `handleTriggerKeyDown` now
  checks `open` first and, when already open, focuses the first item
  directly.

`DESIGN.md`'s **Browse Menu** row was amended before the CSS, per the spine
requirement, to state the single-active-item rule, that hover and keyboard
focus share one appearance, and that a pointer-open pre-selects nothing.

## Protected code: what changed and what did not

This story was explicitly authorized to edit `BrowseControl`'s event
handling, which every prior Epic 7 story was told to leave alone.

**Changed:**
- The open effect (`useEffect`) now reads a new `openedByRef` before
  deciding whether to focus the first item.
- The trigger's `onClick` became `handleTriggerClick`, which sets
  `openedByRef` from `event.detail` before toggling `open`.
- `handleTriggerKeyDown`'s ArrowDown/ArrowUp branch now branches on whether
  the menu is already open.
- Each menu item gained an `onMouseEnter` handler.

**Unchanged, and re-verified against every existing test:**
- `closeAndReturnFocus` (Escape / item-chosen focus return, and the
  `data-focus-return` marker Story 7.10 added) — untouched.
- `handleMenuKeyDown`'s Escape, Tab, Shift+Tab, Home/End and Arrow-navigation
  branches — untouched.
- `handleMenuBlur` (the "does not fight the sender's own focus move" guard,
  and the "closing when `relatedTarget` is the trigger would let a second
  press reopen rather than dismiss" guard) — untouched.
- The trigger's own Escape branch in `handleTriggerKeyDown` — untouched.

Every comment on the handlers above is preserved verbatim. Two comments
whose premises the change makes stale were rewritten rather than deleted:
the open-effect's comment (now explains the keyboard-only pre-select rule
and points at the new `onMouseEnter`), and `handleTriggerKeyDown`'s doc
comment (now explains the dead-key-gap fix). The CSS comment above
`.fd-browse-menu .fd-button:focus` in `style.css`, which claimed menu items
are "only ever focused programmatically... by the open effect... and the
arrow-key handler," is updated to add the third path (`onMouseEnter`) this
story introduces; the WebKit `:focus-visible` reasoning it documents is
untouched and still correct.

## Files changed

- `frontend/src/ui/IdleView.tsx` — `openedByRef`, the conditional open
  effect, `handleTriggerClick`, the pointer-opened arrow-key branch in
  `handleTriggerKeyDown`, and `onMouseEnter` on each menu item.
- `frontend/src/style.css` — removed `.fd-browse-menu .fd-button:hover`;
  added/updated comments explaining the removal and the third focus path.
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`
  — amended the **Browse Menu** row (spine-first, before the CSS edit).
- `frontend/src/ui/IdleView.test.tsx` — new `describe('the browse menu has
  one active item (Story 7.11)')` block, six new tests, nothing else
  touched.
- `frontend/src/ui/styles.test.ts` — two new tests in the existing "browse
  menu surface" describe block, nothing else touched.

## Existing `BrowseControl` tests: pass unchanged

`git diff --stat frontend/src/ui/IdleView.test.tsx frontend/src/ui/styles.test.ts`
shows pure insertions (111 and 24 lines respectively, 0 deletions, 0
modifications to existing lines) — confirmed by reading the diff, not only
the stat. Every pre-existing test in both files was run, unedited, and
passed:

```
$ npm test -- --run
 Test Files  17 passed (17)
      Tests  645 passed (645)
```

637 pre-existing + 8 new (6 in `IdleView.test.tsx`, 2 in `styles.test.ts`) =
645. In particular, every test this story's brief named by name as
must-stay-green passed with zero edits: `'closes on Escape, returns focus to
the control, and announces nothing'`, `'closes on Tab, which in this view
wraps focus back onto the control'`, `'closes on Shift+Tab as well...'`,
`'closes quietly when focus leaves the menu on its own, without recapturing
it'`, `'returns focus to the control once a kind is chosen'`, the two
"dismisses its own menu" tests (toggle-not-reopen), and `'never rings a
routed landing target...'` in `styles.test.ts` (unaffected — that test lives
outside the browse-menu describe block and was not touched).

One existing test's *behaviour* is worth calling out explicitly because it
looks at first glance like it should have needed changing: `'opens on
activation, offers both kinds, and lands focus in it'` uses `fireEvent.click`
with no `detail` override and asserts the first item is focused after. jsdom's
`fireEvent.click` defaults `event.detail` to `0` — the same value a real
keyboard-triggered click (Enter/Space) carries — so this test (and every
other `fireEvent.click(...)` call throughout the pre-existing suite, none of
which pass `detail`) is read by `handleTriggerClick` as a keyboard-style
open, and keeps focusing the first item exactly as before. This is not a
coincidence I discovered after the fact — it is the reason `event.detail` was
chosen as the discriminator: it is the one signal a real browser sets
differently for a pointer click versus a keyboard activation, and it happens
to make every pre-existing `fireEvent.click`-based test continue to mean
what it always meant.

## Pointer-opened Escape and arrow-key cases

- **Escape while pointer-opened (focus on trigger):** unchanged code path.
  `handleTriggerKeyDown`'s Escape branch already handled this — `if (!open)
  return; ... setOpen(false)` — because Escape's own defect (a pointer press
  leaving focus on the trigger while the menu is open) was fixed by Story
  7.10's era of work and predates this story. This story did not touch that
  branch. Covered by the pre-existing `'closes on Escape while focus is
  still on the control'` test, unmodified and green.
- **ArrowDown/ArrowUp while pointer-opened (focus on trigger):** this is the
  dead-key gap the story brief calls out. `handleTriggerKeyDown` now checks
  `if (open)` before the previous unconditional `setOpen(true)`; when already
  open it calls `firstItemRef.current?.focus()` directly, handing off to the
  menu's own `onKeyDown` for every keystroke after that one. New test:
  `'closes the dead-key gap: ArrowDown/ArrowUp on a pointer-opened trigger
  moves focus into the menu'`.

## Design conflict check

No conflict found. The one place this story's design brushed against a
handler `BrowseControl`'s comments say was defending a reproduced defect —
`handleMenuBlur`'s "closing when `relatedTarget` is the trigger would let a
second press reopen rather than dismiss" guard — was not touched, and hover
focusing an item does not route through `handleMenuBlur` in a way that could
retrigger it: moving focus from one menu item to another (or from the
trigger to a menu item) always has a `relatedTarget` contained by
`menuRef.current`, which is the existing "still inside the menu, do nothing"
early return. This was proved by mutation (case 3 below) and confirmed by
the passing pre-existing "closes quietly when focus leaves the menu on its
own" test, which exercises `handleMenuBlur`'s other guard and remained
green throughout.

## Mutation table

Every mutation below was applied by hand, confirmed to make the named
test(s) fail (with the failure naming the mutated behaviour, not merely
"a test failed"), then reverted before the next.

| # | Mutation | Command | Result |
|---|---|---|---|
| 1 | Pre-select unconditionally on open (reverted the open effect's guard: `if (open) firstItemRef.current?.focus()`, dropping the `openedByRef.current === 'keyboard'` check) | `npm test -- --run` | **3 tests fail**, each naming the pointer-open guarantee: `'pre-selects nothing when opened by a real pointer click, leaving focus on the trigger'`, and both dead-key-gap-test iterations (`'closes the dead-key gap...'`, which asserts focus is on the trigger *before* the key is pressed). Confirmed, then reverted. |
| 2 | Invert the keyboard guard (`openedByRef.current === 'pointer'` instead of `'keyboard'`), so a keyboard open pre-selects nothing | `npm test -- --run` | **8 tests fail**, including the pre-existing `'opens on activation, offers both kinds, and lands focus in it'`, the new keyboard-open and ArrowDown/ArrowUp-open tests, and the Home/End navigation test (which depends on the menu opening with focus already inside it). Confirmed the keyboard path is load-bearing for pre-existing tests too, then reverted. |
| 3 | Remove `onMouseEnter` from both menu items | `npm test -- --run` | **2 tests fail**: `'moves focus to an item on hover, and only that item'` and `'hands off from keyboard focus to hover focus cleanly -- never two items marked at once'`. No other test broke, confirming hover-focus is additive and does not interact with `handleMenuBlur`'s guards. Confirmed, then reverted. |
| 4 | Reintroduce `.fd-browse-menu .fd-button:hover { background: var(--color-fill); }` in `style.css` | `npx vitest run --run src/ui/styles.test.ts` | **2 tests fail**: the new `'carries no separate :hover appearance for menu items (Story 7.11)'` and `'gives the focused menu item the only marked appearance...'`. Confirmed, then reverted. |
| 5 | Leave the dead-key gap open (drop the `if (open) { firstItemRef.current?.focus(); return }` branch, restoring the old unconditional `setOpen(true)`) | `npm test -- --run` | **1 test fails**: `'closes the dead-key gap: ArrowDown/ArrowUp on a pointer-opened trigger moves focus into the menu'`. Confirmed, then reverted. |

All five mutations were applied one at a time to a clean tree (each
confirmed via `git diff --stat` before and after), and the tree was restored
to the pre-mutation state (verified with `git diff --stat` returning to the
same five-file, pure-implementation diff) before moving to the next.

## Full verification gate (macOS, `verify.yml` order)

All commands below were run from the repository root except where noted,
against the tree with all five mutations reverted.

```
$ wails build
...
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 8.304s.
```
(`frontend/wailsjs/go/main/App.d.ts`, `App.js` and `models.ts` were flipped
to mode 755 by this build, as AGENTS.md warns; `chmod 644` restored them —
confirmed no other content diff in those three files, only the mode bits.)

```
$ gofmt -l .
(no output)

$ go vet ./...
(no output)

$ go tool staticcheck ./...
(no output)

$ go test -count=1 ./...
ok  	fairdrop	1.738s
ok  	fairdrop/internal/network	0.779s
ok  	fairdrop/internal/qr	0.248s
ok  	fairdrop/internal/server	5.287s
ok  	fairdrop/internal/source	0.966s
ok  	fairdrop/internal/stream	3.220s
ok  	fairdrop/internal/transfer	1.897s
ok  	fairdrop/scripts	1.282s
ok  	fairdrop/scripts/mutationverdict	1.436s

$ go env CGO_ENABLED
1

$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	8.077s
ok  	fairdrop/internal/network	1.656s
ok  	fairdrop/internal/qr	1.949s
ok  	fairdrop/internal/server	6.355s
ok  	fairdrop/internal/source	2.147s
ok  	fairdrop/internal/stream	101.924s
ok  	fairdrop/internal/transfer	2.908s
ok  	fairdrop/scripts	2.397s
ok  	fairdrop/scripts/mutationverdict	2.754s
```

Frontend suites, run sequentially and never concurrently with `wails build`:

```
$ cd frontend && npm test -- --run
 Test Files  17 passed (17)
      Tests  645 passed (645)

$ npm run test:browser
 Test Files  2 passed (2)
      Tests  23 passed (23)
```

`frontend/browser/captures/qr-panel-forced-colors.capture.png` was rewritten
(1-byte PNG-encoding churn, per the known AGENTS.md pitfall) and restored
with `git checkout -- frontend/browser/captures/`; `git status` showed no
diff for it afterward.

Cross-platform build pre-flight (AGENTS.md):

```
$ GOOS=windows GOARCH=amd64 go build ./...
(no output — success)

$ GOOS=darwin GOARCH=arm64 go build ./...
(no output — success)

$ GOOS=linux GOARCH=amd64 go build ./...
(no output — success)
```

## Final tree state

```
$ git status --short
 M _bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md
 M frontend/src/style.css
 M frontend/src/ui/IdleView.test.tsx
 M frontend/src/ui/IdleView.tsx
 M frontend/src/ui/styles.test.ts
```

No generated-binding drift, no capture-PNG churn, no unrelated files
touched.

## Follow-up: two owner findings on the built macOS binary

After the work above shipped, the owner drove the rebuilt app and reported
two further defects, both traced to the same component and, for the first
one, the same story:

### Regression 1 (serious): a pointer-open killed Escape, the arrows, and click-outside

**Symptom.** After opening the menu with a mouse click: Escape did nothing,
ArrowDown/ArrowUp did nothing (the "dead-key gap" this story believed it had
already closed), and clicking outside the menu did not dismiss it -- only a
second click on the trigger closed it.

**Root cause, confirmed on the built binary.** WebKit does not focus a
`<button>` when it is clicked -- it mirrors the native macOS platform, where
a pointer click on a button moves no keyboard focus at all (Chromium
differs: it focuses on mousedown). The pointer-open fix earlier in this
document removed the old unconditional "focus the first item on every
open," which fixed the visual defect (`File` no longer pre-selected on a
pointer click), but put nothing in its place for the pointer branch. A
pointer-open therefore left focus on neither the trigger nor any item -- on
`document.body` -- which broke two handlers at once:
- `handleTriggerKeyDown` never fired, because the trigger never had focus.
  That killed Escape and the arrow keys.
- `handleMenuBlur` never fired, because focus was never inside the menu to
  begin with, so there was nothing for a later blur to report. That killed
  click-outside dismissal.

**Why the original suite did not catch it.** jsdom's `fireEvent.click` moves
no focus by itself (confirmed empirically: a bare click leaves
`document.activeElement` wherever it already was), which is exactly why the
original defect was invisible in jsdom too. But the tests written for the
first pass of this story papered over that gap by hand: `control().focus()`
was called explicitly before firing a pointer-styled click, to give the
test something to assert focus *stayed* on. That manual call encoded an
assumption -- "a pointer press leaves focus on the trigger" -- that turned
out to be false on the one real engine that matters here. This is the same
class of defect as Story 7.10's `:focus-visible`/WebKit gap and the
original `tabFocusesLinks` bug: a green test pinning behaviour that is dead
on a platform the suite never runs, because the test itself manufactured
the premise the bug depended on being false.

**This is the third distinct macOS-only focus behaviour this epic has hit**
(recorded here so the next person does not have to rediscover it):
1. Story 7.10: WebKit never matches `:focus-visible` for a script-focused
   element.
2. `main.go`'s `TabFocusesLinks`: WebKit's macOS default leaves Tab unable
   to reach a `<button>` at all.
3. Story 7.11: WebKit does not focus a `<button>` on click, unlike Chromium.

**The fix.** The menu container itself (`.fd-browse-menu`) now takes real
focus on a pointer-open, via a new `tabIndex={-1}` (the same roving-tabindex
technique already used for the menu items) and a `menuRef.current?.focus()`
call in the open effect's pointer branch, in `frontend/src/ui/IdleView.tsx`.
The container already carries `onKeyDown={handleMenuKeyDown}` and
`onBlur={handleMenuBlur}`, so Escape, the arrows, and click-outside are all
live the instant the menu opens, and nothing is visually marked because
`.fd-browse-menu .fd-button:focus` in `style.css` only ever matches an item,
never the container `<div>`. `handleTriggerKeyDown`'s own `if (open)`
branches (Escape and the arrow keys) are kept as a defensive fallback --
documented in their own comment as no longer the primary path -- rather
than deleted, since a direct-dispatch test or a future focus path could
still legitimately land real focus back on the trigger while the menu is
open.

**Proof the regression was real, and that it is fixed.** Per the
coordinator's explicit instruction, none of the new tests presuppose that a
click focused anything. `frontend/src/ui/IdleView.test.tsx`'s
`describe('a pointer-open keeps Escape, the arrows and click-outside alive
(owner regression, confirmed on the macOS binary)')` dispatches every key on
`document.activeElement ?? document.body` -- wherever a real keypress would
actually land -- never on `control()` directly, which is what let the first
version of this story's tests pass over a dead interaction. These four
tests were run against the tree exactly as committed for the first pass of
this story (before any of the fixes in this follow-up section), confirmed
to fail, and the failure log was retained:

```
$ npm test -- --run src/ui/IdleView.test.tsx
 ❯ src/ui/IdleView.test.tsx (64 tests | 5 failed)
     × Escape still closes a pointer-opened menu
     × ArrowDown still moves focus into a pointer-opened menu
     × ArrowUp still moves focus into a pointer-opened menu
     × a focus move to an element outside the control closes a pointer-opened menu
     × clears a hover-marked item when the pointer leaves the whole menu
 Tests  5 failed | 59 passed (64)
```

(The fifth failure, `'clears a hover-marked item...'`, is Regression 2
below -- both were written and proved-failing in the same pass, since they
share a root cause and a fix target per the coordinator's instruction not to
treat them as separate.) After the fix, the same run: `64 passed (64)`.

### Regression 2: hovering off an item did not unhighlight it

**Symptom, in the owner's words:** "when I move my mouse OFF of any option
they don't both unhighlight, but when I click on the expander again they
both unhighlight."

**Root cause.** Hovering an item moves *focus* to it (the mechanism this
story's first pass introduced so hover and keyboard share one appearance).
Moving the pointer away does not itself remove focus, so the item stayed
marked after the mouse left -- a native menu clears the highlight when the
pointer leaves it.

**The fix must not simply blur to nowhere.** A bare `blur()` to
`document.body` would reintroduce Regression 1: focus on `body` means the
trigger's key handler and the menu's blur handler are both dead again. The
fix instead moves focus to the same neutral holder Regression 1's fix
already introduced -- the menu container -- so every handler stays live
with nothing marked. One answer, in both places, for "where does focus live
when no item is active."

**Keyboard selections must survive an incidental mouse pass.** A new ref,
`activeSourceRef` (`'pointer' | 'keyboard' | null`), records which input
last placed the current mark: `'keyboard'` from the open effect's keyboard
branch and every move in `handleMenuKeyDown`'s navigation block (Home, End,
ArrowUp/Down), `'pointer'` from each item's `onMouseEnter`. A new
`handleMenuMouseLeave`, attached to the menu container's `onMouseLeave`
(which only fires when the pointer leaves the container itself, never when
it moves between sibling items -- `mouseleave` does not bubble), clears the
mark and returns focus to the container **only when `activeSourceRef.current
=== 'pointer'`**. A keyboard-marked item is left untouched, including when
the mouse had previously marked a different item before the keyboard took
back over.

**Mutation and proof.** The `'clears a hover-marked item...'` test above was
proved failing against the pre-fix tree in the same run as Regression 1's
four tests (see the log above). Two more tests pin the "leave keyboard
alone" half: `'leaves a keyboard-marked item alone when the pointer merely
passes over the menu and leaves'` and `'...even after the mouse had
previously marked a different one'`, both passing after the fix.

## Follow-up mutation table

Applied by hand to the fully-fixed tree, one at a time, each confirmed to
fail naming the mutated behaviour, then reverted.

| # | Mutation | Command | Result |
|---|---|---|---|
| 1 | Pointer-open branch of the open effect no longer focuses the menu container (`menuRef.current?.focus()` removed) | `npm test -- --run src/ui/IdleView.test.tsx` | **4 tests fail**: `'Escape still closes a pointer-opened menu'`, `'ArrowDown still moves focus into a pointer-opened menu'`, `'ArrowUp still moves focus into a pointer-opened menu'`, `'a focus move to an element outside the control closes a pointer-opened menu'`. Confirmed, then reverted. |
| 2 | `handleMenuMouseLeave` drops its `activeSourceRef.current !== 'pointer'` guard, clearing unconditionally | `npm test -- --run src/ui/IdleView.test.tsx` | **2 tests fail**: `'leaves a keyboard-marked item alone when the pointer merely passes over the menu and leaves'` and `'...even after the mouse had previously marked a different one'`. Confirmed a keyboard selection would have been stolen by an incidental mouse pass, then reverted. |
| 3 | `onMouseLeave={handleMenuMouseLeave}` removed from the menu container entirely | `npm test -- --run src/ui/IdleView.test.tsx` | **1 test fails**: `'clears a hover-marked item when the pointer leaves the whole menu'`. Confirmed, then reverted. |
| 4 | The shared ring rule's `box-shadow` collapsed to a single `var(--color-primary)` layer with no surface gap (the reframed Story 7.8 guard's own named mutation) | `npx vitest run --run src/ui/styles.test.ts` | **2 tests fail**: `'draws a two-tone ring -- a surface gap, then a primary ring -- for the two Tab-reachable controls (Story 7.11)'` and `'keeps a surface-coloured gap strictly inside the primary ring on every ringed control'` -- the reframed guard biting exactly as the owner asked it to be proved. Confirmed, then reverted. |

All four were applied and reverted individually; `git diff --stat` returned
to the same five-file diff after each revert.

## The retired violet ring: what changed and why

Story 7.8 introduced a dedicated violet `--color-focus` token (`#6B4E9E`
light / `#B79BE0` dark), distinct from `--color-primary`, because the ring
was an `outline` painted directly against whatever fill sat under it, and a
same-hue outline on a same-hue fill disappears. The owner found the
resulting violet "poorly polished" -- a second hue with no relationship to
the rest of the product -- and asked for the ring to be the product's one
accent colour, using a two-tone technique (a surface-coloured gap, then the
ring) to keep it visible instead of a different hue.

**`--color-focus` is removed entirely, not aliased to `--color-primary`.**
The alternative -- keeping the token as a redundant alias -- was considered
and rejected: a token that always equals another token adds indirection
without adding information, and risks silently drifting apart from it in a
later edit with no test able to tell the two apart by name. Every rule that
used to read `--color-focus` now reads `--color-primary` directly, so there
is exactly one accent colour in the product, full stop.

**Implementation.** `--focus-ring-width` (2px, was 3px) and
`--focus-ring-offset` (2px, unchanged) are redefined as, respectively, the
ring's own thickness and the surface-coloured gap's width, both still
declared once on `:root`. Every ring site stacks two `box-shadow` layers
instead of an `outline`:
```css
box-shadow:
    0 0 0 var(--focus-ring-offset) var(--color-surface),
    0 0 0 calc(var(--focus-ring-offset) + var(--focus-ring-width)) var(--color-primary);
```
applied identically to the trigger/`.fd-url`/`[data-focus-return]` shared
rule and to `.fd-disclosure__summary:focus-visible` -- the second and third
rings the owner named explicitly ("the trigger, `.fd-url`, and the menu
items... one focus appearance in the product, not two"). The browse menu
item's rule stacks a third layer (the existing 4px tint halo) ahead of the
same gap/ring pair, offset by `calc(4px + ...)`.

**Contrast figures, re-derived independently rather than copied from the
owner's message**, using the same relative-luminance formula
`styles.test.ts` already implements (verified against a standalone Python
reimplementation of the same WCAG formula, not read back from the
stylesheet under test):

| Pair | Light | Dark |
|---|---:|---:|
| `primary` on `surface` (already published, unchanged) | 5.529272927 | 7.193529288 |
| `primary` on `elevated` (already published, unchanged) | 5.529272927 | 6.510527964 |
| `primary` on `canvas` (new) | 4.945442820 | 7.907185645 |
| `primary` on `fill` (new) | 4.898523535 | 5.667386798 |

All four clear the 3:1 non-text floor in both modes, confirming the owner's
independently-stated figures. `fill` is the weakest pairing in both modes.
These are published in `DESIGN.md`'s Colors section and load-bearing pair
table, and asserted unrounded by `styles.test.ts`'s existing
`'publishes every figure it proves, unrounded, in DESIGN.md'` test (which
needed no changes itself -- it already iterates whatever `placed` declares
as published).

**The Story 7.8 guard is reframed, not deleted, per the coordinator's
explicit instruction.** The old test asserted `--color-focus !== --color-primary`
in both modes -- a hue-difference mechanism, because that was how
visibility was protected at the time. Story 7.11 makes the ring literally
`--color-primary` on purpose, so that assertion's premise is now false by
design; the *thing it protected*, visibility, still needs protecting. The
replacement, `'keeps a surface-coloured gap strictly inside the primary
ring on every ringed control'`, asserts the structural guarantee instead: a
`--color-surface`-coloured shadow layer at `var(--focus-ring-offset)` must
exist strictly inside a `--color-primary`-coloured layer at
`calc(var(--focus-ring-offset) + var(--focus-ring-width))`. Mutation table
row 4 above is this guard's own proof, per the coordinator's instruction to
mutate the gap away and confirm the reframed guard bites.

**Forced colors.** `--color-focus: Highlight;` is removed from the
`forced-colors: active` override; it needed no replacement, because
`--color-primary` already resolves to `Highlight` and `--color-surface`
already resolves to `Canvas` there, so the ring's two colours are already
correct without a third variable to keep in step. One further addition,
not requested but judged necessary: Windows High Contrast Mode strips
decorative `box-shadow` the same way this file already strips the
elevation shadows (`--shadow-sh-*: none`) in that block, so a `box-shadow`-
only ring would have risked becoming invisible specifically in forced
colors, despite being visible everywhere else. A forced-colors-scoped
override restates every ring site as `outline: var(--focus-ring-width)
solid Highlight;` with `box-shadow: none;`, pinned by a new test
(`'restates every box-shadow focus ring as an outline...'`). This is the
one piece of this follow-up not directly requested; flagged here rather
than silently added.

## Updated verification gate (macOS, after both follow-up fixes)

```
$ wails build
...
Built '/Users/jaesonmartin/Projects/FairDrop/build/bin/fairdrop.app/Contents/MacOS/fairdrop' in 7.359s.
```
(bindings flipped to mode 755 again; `chmod 644` restored them, no content
diff.)

```
$ gofmt -l .
(no output)

$ go vet ./...
(no output)

$ go tool staticcheck ./...
(no output)

$ go test -count=1 ./...
ok  	fairdrop	2.041s
ok  	fairdrop/internal/network	0.592s
ok  	fairdrop/internal/qr	1.281s
ok  	fairdrop/internal/server	4.560s
ok  	fairdrop/internal/source	0.773s
ok  	fairdrop/internal/stream	3.063s
ok  	fairdrop/internal/transfer	1.875s
ok  	fairdrop/scripts	1.105s
ok  	fairdrop/scripts/mutationverdict	1.410s

$ CGO_ENABLED=1 go test -count=1 -race ./...
ok  	fairdrop	8.463s
ok  	fairdrop/internal/network	1.251s
ok  	fairdrop/internal/qr	1.865s
ok  	fairdrop/internal/server	6.149s
ok  	fairdrop/internal/source	2.321s
ok  	fairdrop/internal/stream	99.206s
ok  	fairdrop/internal/transfer	2.706s
ok  	fairdrop/scripts	2.710s
ok  	fairdrop/scripts/mutationverdict	2.536s
```

```
$ cd frontend && npm test -- --run
 Test Files  17 passed (17)
      Tests  654 passed (654)

$ npm run test:browser
 Test Files  2 passed (2)
      Tests  23 passed (23)
```

(654 = 637 pre-Story-7.11 baseline + 17 Story-7.11 tests, net of this
follow-up's edits to the story's own test blocks: 4 Regression-1 tests + 4
Regression-2 tests + 6 surviving/updated tests from the first pass in
`IdleView.test.tsx`, and 3 new + 2 rewritten focus-ring tests net across
`styles.test.ts`'s several affected describe blocks. Every test outside
Story 7.11's own blocks -- confirmed by diffing this file's history/tail
sections against the previous commit and the pre-Story-7.11 baseline
byte-for-byte -- is untouched.)

`frontend/browser/captures/qr-panel-forced-colors.capture.png` churned and
was restored with `git checkout -- frontend/browser/captures/` after each
browser-suite run.

```
$ GOOS=windows GOARCH=amd64 go build ./...
(no output -- success)

$ GOOS=darwin GOARCH=arm64 go build ./...
(no output -- success)

$ GOOS=linux GOARCH=amd64 go build ./...
(no output -- success)
```

## Files touched in this follow-up (beyond the first pass)

- `frontend/src/ui/IdleView.tsx` -- `activeSourceRef`, the pointer-open
  branch's container focus, `handleMenuMouseLeave`, `tabIndex={-1}` and
  `onMouseLeave` on the menu container, `onMouseEnter` on each item now also
  sets `activeSourceRef`, and updated/added comments on
  `handleTriggerKeyDown` and the open effect explaining the corrected
  premise.
- `frontend/src/style.css` -- `--color-focus` removed from `@theme`, the
  dark override, and the forced-colors override; `--focus-ring-width`/
  `--focus-ring-offset` redefined as gap/ring tokens; every ring site
  converted from `outline` to stacked `box-shadow`; a new forced-colors
  outline restatement.
- `frontend/src/ui/IdleView.test.tsx` -- the flawed
  `control().focus()`-staged tests from the first pass replaced; new
  `describe` blocks for both regressions.
- `frontend/src/ui/styles.test.ts` -- the Story 7.8 guard reframed; the
  contrast-proof `placed` table's `focus`-keyed rows replaced with
  `primary`-vs-`canvas`/`fill`; new two-tone-ring mechanism tests; forced
  colors test updated.
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/DESIGN.md`
  -- the Colors table's Focus row and the load-bearing contrast table/prose
  rewritten for the two-tone ring.
- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-quartz-2026-09-20/EXPERIENCE.md`
  -- its one `{colors.focus}` reference updated to match, found by grepping
  the repo for the retired token name per AGENTS.md's "when a story deletes
  a contract, grep the whole repo" pitfall.

`_bmad-output/planning-artifacts/epics.md` still describes the Story 7.8
acceptance criteria as originally written (a 3px `{colors.focus}` outline);
left unedited as historical story-definition text, not living spec --
`DESIGN.md` and `EXPERIENCE.md` are the two documents AGENTS.md names as
controlling visual presentation.
