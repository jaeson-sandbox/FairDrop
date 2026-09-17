---
title: 'Story 4.1: Replace the Two Browse Controls with One'
type: 'feature'
created: '2026-09-17'
status: 'done'
baseline_commit: '84b0594c63f83b49ac1ece84cd5477de3fb9ea43'
review_loop_iteration: 0
context:
  - '{project-root}/_bmad-output/implementation-artifacts/epic-4-context.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md'
  - '{project-root}/AGENTS.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Idle asks the sender to classify their thing before they have chosen it. Two buttons,
*Select File* and *Select Directory*, sit where one belongs — the owner's complaint is the
two-buttons-ness itself, not the drag-versus-browse split. The split is not a design choice: Wails
exposes `OpenFileDialog` and `OpenDirectoryDialog` separately, and Windows' `IFileOpenDialog` cannot
offer both kinds in one dialog. Two deferred states sit on the same surfaces: a chooser that fails
to open reports a transfer that never existed (D-113), and a successful copy renames the only
control that reaches the capability URL to "Copied" for the rest of the session (D-114).

**Approach:** One control labelled for both kinds, opening a small menu where the platform cannot
offer both in one dialog. The menu is where the asymmetry is absorbed; the label never promises
something a click does not deliver. Give the chooser failure a state that describes it, and give
the copy confirmation back a name that says what the control does.

## Boundaries & Constraints

**Always:** The drop zone keeps the top of the region, its `h1`, and its `idle-instruction` focus
target, and still takes either kind in one gesture. The browse control stays below the firewall
guidance (FR23) and is not the loudest thing on the screen. Every new label enters `EXPERIENCE.md`
by stable key before any code emits it, and any new error code moves the registry, the contract,
the Go table and the TypeScript mirror together under the existing cross-language pin. One
announcement owner per transition, and a dismissed chooser stays silent.

**Ask First:** `DESIGN.md`'s Selection Controls row reads "Equal-weight File and Directory controls;
no false primary choice" — a pair that will not exist. Its replacement wording is a design-contract
change. Also: any fifth error code's exact message, and any change to what the drop zone says.

**Never:** Re-create `IdleView`'s old scar — a control whose label offers both kinds and whose click
opens a file-only dialog. Make folders reachable by drag alone (`EXPERIENCE.md` bans drag-only
input). Introduce a modal: modal depth is limited to OS dialogs, and a menu is permitted only
because it is not one. Reach around Wails to `NSOpenPanel` — that is a later phase this design must
not preclude and does not implement. Let a chooser diagnostic carry the attempted path (AD-9).

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Control activated, pointer or keyboard | Idle | The menu opens, focus lands in it, both kinds are offered | — |
| A kind chosen | menu open | The matching native chooser opens; the menu closes | — |
| Escape, or focus leaving the menu | menu open | The menu closes, focus returns to the control, nothing is announced | — |
| Chooser returns a selection | any kind | Stages exactly as today, `itemKind` from the branch taken | — |
| Chooser dismissed | empty result | Idle, silent, focus back on the control | Not an error |
| Chooser fails to open | dialog error | A state that describes a chooser that did not open, never a stopped transfer | New code; no path in the message (AD-9, D-113) |
| Copy succeeds, then the user returns | Staged | The control still names what it does | D-114 |

</frozen-after-approval>

## Code Map

- `frontend/src/ui/IdleView.tsx:131-137` — the `fd-selection` div holding the two `fd-button
  fd-target` buttons, wired to `onSelectFile` / `onSelectDirectory` (props at `:20-21`). Comments at
  `:55` and `:86-88` carry the ordering rule and the scar; both must survive.
- `frontend/src/App.tsx` — `idleView()` passes both handlers from the controller; the menu's own
  state is view-local, not reducer state.
- `frontend/src/transfer/useTransfer.ts` — `selectFile` / `selectDirectory` on `TransferController`.
  Both already stage with the right `PendingItemKind`; `'unknown'` exists for a control that does not
  yet know, and `selectors.ts` explains why it must not resolve early.
- `frontend/src/ui/copy.ts:101-102` — `label.selectFile` / `label.selectDirectory`. Pinned by
  `copy.test.ts:167-168` (the key list) and `:200-201` (the values), and named in prose at
  `EXPERIENCE.md:39` rather than as a `copy.*` registry row — so the new label needs both.
- `app.go:288-321` — `chooseWith`. The `err != nil` branch returns `ErrTransferFailed` with a comment
  recording exactly why D-113 was left open; that comment is the thing to replace.
- `internal/transfer/errors.go`, `docs/fairdrop-contracts.md`, `frontend/src/transfer/errors.ts`,
  `internal/transfer/errors_test.go` — the five places a code moves through; the pin names all of them.
- `frontend/src/ui/StagedView.tsx:~170` — the Copy button whose label swaps to `copy.copy.confirmation`
  and never swaps back (D-114).
- `frontend/src/ui/styles.test.ts`, `frontend/browser/accessibility.test.tsx` — the 44px floor and the
  rendered suite; a menu is new interactive surface and must clear both.
- Tests naming the current labels: `IdleView.test.tsx` (3), `App.test.tsx` (2), `copy.test.ts` (2),
  `App.focus.test.tsx` (1).

## Tasks & Acceptance

**Execution:**
- [x] `EXPERIENCE.md` — register the control's label and any menu item copy by stable key, and reconcile the Idle prose at `:39` that names two controls.
- [x] `DESIGN.md` — replace the Selection Controls row with one that describes one control (Ask First).
- [x] `frontend/src/ui/copy.ts` — the new keys; retire the two old labels with their pins.
- [x] `frontend/src/ui/IdleView.tsx` — one control plus the menu: `role="menu"`, keyboard operation, Escape, focus return, 44px targets.
- [x] `app.go` + the five registry places — a code that describes a chooser that did not open (D-113).
- [x] `frontend/src/ui/StagedView.tsx` — the copy confirmation stops costing the control its name (D-114).
- [x] `frontend/browser/accessibility.test.tsx` — the menu measured open: targets, reflow, forced colors.
- [x] `evidence-4-1-replace-the-two-browse-controls-with-one.md`, D-113 and D-114 discharged, `epics.md` and `sprint-status.yaml` in step.

**Acceptance Criteria:**
- Given Idle, when a sender looks for how to send something, then exactly one browse control is present where two were, and its label names both kinds.
- Given the menu open, when the sender uses only a keyboard, then every item is reachable, Escape closes it, and focus returns to the control it came from.
- Given a chooser that fails to open, when the failure reaches the window, then it describes a chooser that did not open and names no path.
- Given a link already copied, when the sender returns to the control later, then it still says what it does.

## Evidence

Audit, mutation tables and gate transcripts live in
[evidence-4-1-replace-the-two-browse-controls-with-one.md](evidence-4-1-replace-the-two-browse-controls-with-one.md),
created with the implementation.

## Spec Change Log

**2026-09-17 (approval).** Owner decisions taken before dispatch, so the Ask First list does not
block the implementer.

*The Windows split.* The single control opens a small menu offering both kinds. The menu is where
the platform asymmetry is absorbed, so the label never promises what a click does not deliver.
Rejected: a file-only chooser with folders by drag (re-creates IdleView's scar, and makes folders
drag-only, which EXPERIENCE.md bans); two dialogs in sequence (asks the user to reject one to reach
the other); restyling two buttons (does not deliver the story).

*The label.* `Choose a file or folder` -- it names both kinds and agrees with the drop zone's
existing `copy.idle.instruction` rather than contradicting it.

*D-113's message.* `FairDrop couldn't open the chooser. Try again, or drag the item onto the
window.` It names what failed and offers the one recovery already present on the same screen, which
matters most precisely when the dialog subsystem is the thing that is unwell. A fifth code, because
`setup_failed` is a claim about an item the user chose and none was.

*DESIGN.md's Selection Controls row.* Replaced with `One selection control, quieter than the drop
zone.` The original balanced two peers; with one control that intent is moot, and what remains worth
stating is the hierarchy the owner set on 2026-09-13. It deliberately does not name the menu, so a
later NSOpenPanel phase can drop the menu on macOS without amending the design contract.

*Not changed:* the drop zone's own copy. `Drop one file or folder.` already agrees with the new
label, so nothing needs reconciling beyond the Idle prose at EXPERIENCE.md:39 that still names two
controls.

## Design Notes

**The menu is the seam, and it is the story's real cost.** This is FairDrop's first floating
surface. `EXPERIENCE.md` limits modal depth to OS dialogs; a menu is permitted because it is not
modal, not because the bar is low. It has to be operable by keyboard alone, it has to return focus,
and it has to clear the same accessibility floor as everything else.

**macOS gets no special case here.** `NSOpenPanel` can take both kinds at once, and a later phase may
route it that way — one step instead of two, with no second label. This design must leave that door
open, which it does: the menu is a view concern, and both kinds already reach the same
`StageTransfer`.

**`'unknown'` is already paid for.** A control that does not know the kind costs nothing downstream —
native drop has used it since Epic 1.

## Verification

**Commands:**
- `gofmt -l .`; `go vet ./...`; `go tool staticcheck ./...`; `go test -count=1 ./...` — clean
- `cd frontend && npx tsc --noEmit -p tsconfig.json`; `npx vitest run`; `npm run test:browser`
- The native run read with `gh run view --json conclusion,jobs`
- Mutations: drop the menu's Escape handler; skip the focus return; let the chooser failure report `transfer_failed` again — each must fail a named test
