# Evidence: Story 4.1: Replace the Two Browse Controls with One

Baseline: `84b0594c63f83b49ac1ece84cd5477de3fb9ea43`.

## What the owner decided, before implementation

Recorded 2026-09-17 in the spec's own Spec Change Log, quoted verbatim so the build could not
drift from it:

- **The Windows split** is absorbed by a small menu the single control opens, never by a
  file-only chooser with folders reachable by drag, two dialogs in sequence, or restyled buttons.
- **The label** is `Choose a file or folder` — it agrees with `copy.idle.instruction` instead of
  contradicting it, the way `Select File` alone would have.
- **D-113's message** is `FairDrop couldn't open the chooser. Try again, or drag the item onto the
  window.`, on a fifth code, because `setup_failed` claims an item was chosen and none was.
- **`DESIGN.md`'s Selection Controls row** becomes `One selection control, quieter than the drop
  zone.`, deliberately not naming the menu so a later `NSOpenPanel` phase can drop it on macOS
  without a second design-contract change.

The second clause of D-113's message did not survive review. `drag the item onto the window` is
false: `OnFileDrop` is registered drop-target-gated and `--wails-drop-target: drop` sits only on
`.fd-drop-zone`, so a drop anywhere else on the window is ignored. The owner approved the corrected
wording on the same day — `Try again, or drop the item on the zone above.` — and that is what the
registry carries. The bullet above is left as the owner first wrote it because this section records
the decision, not the outcome; the spec's Spec Change Log carries the same correction.

The exact owner text used a straight apostrophe (`couldn't`) throughout the whole spec document —
checked: zero curly apostrophes anywhere in `spec-4-1-replace-the-two-browse-controls-with-one.md`,
against a registry that is unanimous the other way. Counted from `publicMessages` itself: nineteen
messages, twelve of them carrying an apostrophe, and all twelve curly — not one straight apostrophe
in the set. That reads as an artifact of how the spec was authored, not a deliberate wording choice,
so the shipped message uses the curly apostrophe (`’`) those twelve and `copy.ts`'s own headings
already use. The wording is otherwise
character-for-character what the owner approved.

## D-113: a fifth code, not a re-use

`app.go`'s `chooseWith` wrapped every dialog-open failure as `ErrTransferFailed`, with a comment
explaining why that was deliberately left wrong (Story 3.11's Ask First boundary forbade inventing
unreviewed copy). That comment is now the reasoning for the fix instead: `chooser_failed` replaces
it, because a chooser that never opened is not a transfer that stopped partway — nothing was chosen,
so nothing could have started.

Moved through all eight places a code in this registry touches — the spec's Code Map names the same
eight, `main_test.go`'s own literal `registryEntries` among them, because that literal drives the
cross-file comparison and would otherwise silently stop checking a fifth code:

- `EXPERIENCE.md`'s stable public error table (new row, `chooser_failed`)
- `docs/fairdrop-contracts.md`'s prose table and its `ErrorCode` constant block
- `internal/transfer/errors.go` (`ErrChooserFailed`, `publicMessages`)
- `internal/transfer/errors_test.go` (`TestPublicErrorOfExactRegistryCopy`,
  `TestTheCodeRegistryIsExactlyThisSet`)
- `frontend/src/transfer/errors.ts` (`transferErrorCodes`, `fixedErrorMessages`)
- `frontend/src/transfer/errors.test.ts` (its `transferErrorCodes` set, and the code/message pairs
  `fixedErrorMessages` is pinned against)
- `frontend/src/ui/copy.test.ts` (the `errorHeadings` literal it compares the production record to)
- `main_test.go`'s `registryEntries` literal, which `TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage`
  and `TestTheRegistryLiteralCoversEveryCodeTheDomainDefines` both read

`frontend/src/ui/copy.ts`'s `errorHeadings` (a `Record<TransferErrorCode, string>`) required the new
key to type-check at all, so TypeScript itself is a ninth, involuntary enforcement of the same
pin — an omission there is a compile error, not a silent gap. No path reaches the message (AD-9):
`WrapError` keeps the adapter's own dialog text behind `Unwrap`, unchanged from before.

## D-114: reverting on blur, not on a timer

The three fixes the deferred-work entry had already ruled out stay ruled out: swapping the label
back still needs a sanctioned trigger, showing "Copied" as the visible text while keeping the old
accessible name still breaks label-in-name (WCAG 2.5.3 — a screen-reader user hears one thing and
sees another), and moving the confirmation beside the button still changes `DESIGN.md`'s Direct URL
Row layout.

What had never been evaluated was a **blur handler** — not a timer, not a lifecycle event, nothing
`EXPERIENCE.md`'s ban on frontend lifecycle timers touches. It fires only from the sender's own
focus move: tabbing on, or clicking elsewhere. `StagedView`'s Copy button now calls `setCopied(false)`
`onBlur`, so the control's visible and accessible name are back to `copy.direct_link.action` before
the sender can ever "return to it" (the acceptance criterion's own words) — by the time focus
re-enters the control, the label already answers what it does. A same-focus re-click (the existing
"click on 'Copied' retries the copy" behaviour Story 3.12 built) is unaffected, since no blur
happens between the two clicks.

`EXPERIENCE.md`'s Copy Feedback row and `DESIGN.md`'s matching row both now say so, so a later
implementer does not have to rediscover the reasoning by reading the component.

## The menu: ARIA menu button, not a plain toggle

Two behaviors the I/O matrix names in the same table row turned out to need different handling, and
conflating them would have reintroduced a keyboard trap:

- **Escape** explicitly asks to leave the menu and return — `closeAndReturnFocus()` closes the menu
  and calls `.focus()` on the trigger, matching the row's "focus returns to the control" literally.
- **Focus leaving the menu on its own** (Tab past the last item, a pointer landing elsewhere) closes
  the menu but must **not** call `.focus()` on the trigger: `EXPERIENCE.md`'s Interaction Primitives
  section is explicit that "No focus trap exists outside OS dialogs", and forcibly recapturing focus
  on every Tab-out would be exactly that — a keyboard user could never Tab past the control to
  `RecoveryHelp` below it. `handleMenuBlur` therefore only closes the menu when `relatedTarget` is
  outside the menu subtree; it never moves focus itself. Proved by mutation (M6, below): making the
  two paths identical fails a named test.

Focus returns to the trigger **before** the native chooser opens (inside `choose()`, ahead of
calling `onSelectFile`/`onSelectDirectory`), because the menu item that was just activated is about
to unmount — the browser's own "dialog cancel restores the previously focused element" behaviour,
which nothing here reimplements, needs that element to still exist when the dialog closes.

Menu item copy reuses the already-approved `copy.label.file` / `copy.label.folder` ("File" /
"Folder") rather than inventing new strings — EXPERIENCE.md's Voice and Tone prose already
sanctions "file" and "folder" as vocabulary, so the only genuinely new label is the control's own
(`copy.label.chooseFileOrFolder`).

## Layout

`.fd-selection` held two equal `1fr` grid columns; one control replaced them, so it became
`position: relative; display: flex` with the trigger at `width: 100%`, and dropped out of the
`max-width: 639px` two-column collapse (nothing left to collapse). `.fd-browse-menu` is the app's
first floating surface: absolutely positioned below the trigger, a functional-boundary border (no
box-shadow — `styles.test.ts`'s "spends the one sanctioned paper offset" test reserves that for
`StagedView` alone), and every item still carries `.fd-target` for the 44px floor.

## Mutation table

Sixteen mutations, each applied on its own, run against the suite that should see it, then reverted
before the next — never batched, so no result could be attributed to the wrong change.

| # | Mutation | Result |
|---|---|---|
| M1 | drop the menu's `Escape` branch in `handleMenuKeyDown` | KILLED — `IdleView.test.tsx`: "closes on Escape, returns focus to the control, and announces nothing" |
| M2 | `closeAndReturnFocus` stops calling `.focus()` | KILLED — the same Escape test, and "returns focus to the control once a kind is chosen" |
| M3 | `chooseWith` reports `ErrTransferFailed` again | KILLED — `app_test.go`: `TestDialogFailureIsCodedAndDisclosesNoDialogText` |
| M4 | `StagedView`'s `handleCopyBlur` becomes a no-op | KILLED — `StagedView.test.tsx`: "reverts to the action label once focus leaves the control (D-114)" and "still names the action when the sender returns to the control later" |
| M5 | `handleMenuBlur` calls `closeAndReturnFocus()` instead of leaving focus alone (reintroducing a trap) | KILLED — "closes quietly when focus leaves the menu on its own, without recapturing it" |
| M6 | the trigger stops opening on `ArrowDown`/`ArrowUp` | KILLED — "opens on ArrowDown and on ArrowUp, not only on activation" |
| M7 | the trigger stops handling `Escape` | KILLED — "closes on Escape while focus is still on the control" |
| M8 | `Home` and `End` drop out of the navigation set | KILLED — "moves to the first and last item on Home and End" |
| M9 | `aria-controls` names the menu while it is closed | KILLED — "names no menu in aria-controls while there is no menu" |
| M10 | the menu items take `tabIndex={0}`, one tab stop each | KILLED — "is one tab stop, with the items reachable by arrow rather than by Tab" |
| M11 | `handleMenuBlur` drops the `next === triggerRef.current` guard | KILLED — "closes on a second activation, after the focus move a real press performs" |
| M12 | `.fd-browse-menu` takes a fixed `88px` height and `overflow: hidden` | KILLED — browser: the 320px reflow case and the 200% text case |
| M13 | `min-inline-size` grows to `1200px` | KILLED — browser: four cases, including forced colors |
| M14 | the menu detaches, `margin-block-start: 600px` | **SURVIVED**, then KILLED once the case was rewritten (below) |
| M15 | the menu flips upward — `inset-block-end: 100%` | KILLED — "hangs off its control and fits the 640x480 minimum main.go sets" |
| M16 | the menu takes `min-block-size: 600px`, taller than the window | KILLED — the same case |

### What M14 exposed, in a test written earlier in this same story

The 640x480 case asserted `menu.getBoundingClientRect().bottom <= window.innerHeight`, under the
message "the open menu reaches N px, past the M px window". It could not fail. Opening the menu
moves focus to its first item, and Chromium scrolls a newly focused element into view; because
`getBoundingClientRect()` is viewport-relative, the menu's `bottom` then sits flush against the
viewport edge wherever it was actually laid out. Under M14 the case still passed, reporting
`bottom = 479.95` in a 480px window while `window.scrollY` had reached `723` — the menu was 600px
further down the document than the test believed, and the number it measured was the browser's
scroll, not the layout.

This is a sixth vacuous-test shape, and the first that no mutation of *production* code would have
revealed on its own: the assertion was insensitive to the property it named, not merely
over-satisfied. The rule it yields is narrow and checkable — **after focus moves into an element,
one viewport-relative coordinate compared against the viewport measures scroll-into-view.** Only
differences survive that scroll.

The case is now two scroll-invariant measurements, which are also the two claims the design
actually makes: the gap between the trigger's bottom and the menu's top (the menu hangs off its
control, opening downward with no flip), and the menu's own height against the window's (no
max-height, no scroll of its own, so it has to be readable in one piece at the smallest size
`main.go` allows). M14, M15 and M16 are the three ways that can go wrong, and all three now fail it.

## Verification

Read stage by stage, all from this story's own working tree:

- `gofmt -l .` — clean
- `go vet ./...` — clean
- `go tool staticcheck ./...` — clean
- `go test -count=1 ./...` — 8 packages ok
- `go test -count=1 -race ./...` — 8 packages ok (`CGO_ENABLED=1` confirmed before the run)
- `GOOS=darwin GOARCH=arm64 go build ./...` and `GOOS=linux GOARCH=amd64 go build ./...` — clean
- `GOOS=darwin GOARCH=arm64 staticcheck ./...` (bare binary) — clean
- `cd frontend && npx tsc --noEmit -p tsconfig.json` — clean
- `npx vitest run` — 17 files, 542 tests passing
- `npm run test:browser` — 1 file, 17 tests passing (5 of them the browse menu: 320px reflow, the
  44px floor, forced colors, 200% text, and the 640x480 minimum)
- `wails build` — succeeds; `git status` shows no binding drift (`SelectFile`/`SelectDirectory`'s
  signatures did not change, only their frontend caller)

Native accessibility evidence (Narrator/NVDA/VoiceOver on the new menu) is not part of this
session's verification: `docs/release-policy.md`'s owner-approved policy makes manual
device/screen-reader observation optional for this personal project, and none is fabricated here.

## Nothing left open

D-113 and D-114 are both `discharged` in `deferred-work.md`. `TestEveryOpenDeferredEntryIsCitedByItsOwningStory`'s
vacuity guard previously required at least one *open* entry to exist to prove its own citation logic
still worked; discharging the repository's last two open entries made that guard fail vacuously by
its own design, since `open` was always going to reach zero the day the backlog finally cleared.
Fixed to check that `deferredEntries` parsed *something* (guarding the parser, not the backlog's
size) rather than that something is still open — a genuine, unplanned consequence of finishing this
story's own acceptance criteria, fixed rather than deferred.
