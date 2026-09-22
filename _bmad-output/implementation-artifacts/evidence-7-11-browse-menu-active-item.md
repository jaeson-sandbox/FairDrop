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
