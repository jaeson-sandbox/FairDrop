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

The exact owner text used a straight apostrophe (`couldn't`) throughout the whole spec document —
checked: zero curly apostrophes anywhere in `spec-4-1-replace-the-two-browse-controls-with-one.md`,
against 17 in the existing registry's messages alone. That reads as an artifact of how the spec was
authored, not a deliberate wording choice, so the shipped message uses the curly apostrophe (`’`)
the other 17 registry entries and `copy.ts`'s own headings already use. The wording is otherwise
character-for-character what the owner approved.

## D-113: a fifth code, not a re-use

`app.go`'s `chooseWith` wrapped every dialog-open failure as `ErrTransferFailed`, with a comment
explaining why that was deliberately left wrong (Story 3.11's Ask First boundary forbade inventing
unreviewed copy). That comment is now the reasoning for the fix instead: `chooser_failed` replaces
it, because a chooser that never opened is not a transfer that stopped partway — nothing was chosen,
so nothing could have started.

Moved through all six places a code in this registry touches (one more than the spec's Code Map
names, `main_test.go`'s own literal `registryEntries`, which drives the cross-file comparison and
would otherwise silently stop checking a fifth code):

- `EXPERIENCE.md`'s stable public error table (new row, `chooser_failed`)
- `docs/fairdrop-contracts.md`'s prose table and its `ErrorCode` constant block
- `internal/transfer/errors.go` (`ErrChooserFailed`, `publicMessages`)
- `internal/transfer/errors_test.go` (`TestPublicErrorOfExactRegistryCopy`,
  `TestTheCodeRegistryIsExactlyThisSet`)
- `frontend/src/transfer/errors.ts` (`transferErrorCodes`, `fixedErrorMessages`)
- `main_test.go`'s `registryEntries` literal, which `TestTheCrossLanguageErrorRegistryPinsEveryCodeAndMessage`
  and `TestTheRegistryLiteralCoversEveryCodeTheDomainDefines` both read

`frontend/src/ui/copy.ts`'s `errorHeadings` (a `Record<TransferErrorCode, string>`) required the new
key to type-check at all, so TypeScript itself is a seventh, involuntary enforcement of the same
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

| # | Mutation | Result |
|---|---|---|
| M1 | drop the menu's `Escape` branch in `handleMenuKeyDown` | KILLED — `IdleView.test.tsx`: "closes on Escape, returns focus to the control, and announces nothing" |
| M2 | `closeAndReturnFocus` stops calling `.focus()` | KILLED — `IdleView.test.tsx`: the same Escape test, and "returns focus to the control once a kind is chosen" |
| M3 | `chooseWith` reports `ErrTransferFailed` again | KILLED — `app_test.go`: `TestDialogFailureIsCodedAndDisclosesNoDialogText` |
| M4 | `StagedView`'s `handleCopyBlur` becomes a no-op | KILLED — `StagedView.test.tsx`: "reverts to the action label once focus leaves the control (D-114)" and "still names the action when the sender returns to the control later" |
| M5 | `handleMenuBlur` calls `closeAndReturnFocus()` instead of leaving focus alone (reintroducing a trap) | KILLED — `IdleView.test.tsx`: "closes quietly when focus leaves the menu on its own, without recapturing it" |

Each mutation was applied by hand, run against its named test file, confirmed to fail naming the
right assertion, then reverted before the next one — never batched, so no result could be
attributed to the wrong change.

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
- `npx vitest run` — 17 files, 535 tests passing
- `npm run test:browser` — 1 file, 15 tests passing (3 new: the browse menu open, measured for
  320px reflow, the 44px floor, and forced colors)
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
