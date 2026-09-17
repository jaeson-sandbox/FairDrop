# Epic 4 Context: Refine the Selection Experience

<!-- Compiled from planning artifacts. Edit freely. Regenerate with compile-epic-context if planning docs change. -->

## Goal

Epic 4 refines the Idle screen's selection experience after Epic 3 shipped FairDrop's native release. Story 4.2 already closed ten verified action items from the Epic 3 retrospective, restoring guarantees a broken build should fail on; it is done and released in v0.3.0. The remaining work, Story 4.1, replaces the Idle screen's two browse buttons (one for a file, one for a folder) with a single control labelled for both, answering an owner complaint about the two-buttons-ness itself rather than the drag-versus-browse split. The design must absorb a real platform asymmetry -- Windows' native file-open dialog cannot offer both kinds in one call the way macOS's can -- without showing that seam to the user, and it must close two deferred-work items surfaced by adjacent stories: a native chooser that fails to open misreporting a transfer that never existed (D-113), and a Copy button that permanently loses its accessible name after first use (D-114).

## Stories

Status: Story 4.2 is done and released in v0.3.0. Story 4.1 is the epic's only remaining work, currently backlog.

- Story 4.2: Close What the Epic 3 Retrospective Found (done)
- Story 4.1: Replace the Two Browse Controls with One (backlog, next work)

## Requirements & Constraints

- Exactly one browse control replaces the two, labelled for both file and folder kinds, in the position the two buttons occupied.
- The drop zone keeps leading the region, keeps the `idle-instruction` focus target the routing table names, and keeps accepting either kind in one gesture.
- The control is operable by pointer or keyboard, reaches both a file and a folder destination, has its choice announced by exactly one owner, and returns focus to where it started.
- Where the native chooser underneath cannot offer both kinds in one dialog, the control absorbs that difference; no path may open a chooser that contradicts what its label offered.
- A later phase may route macOS through `NSOpenPanel` directly for a true one-step chooser; this story's design must not preclude that but does not implement it.
- Any label this story introduces enters `EXPERIENCE.md` by stable key before any code emits it, and moves the registry, the contract, the Go table, and the TypeScript mirror together, per the discipline Story 3.5 established.
- D-113 and D-114 are both in this story's scope; the story is not done while either is open.

## Technical Decisions

- Wails exposes `OpenFileDialog` and `OpenDirectoryDialog` as separate calls. Underneath, macOS's `NSOpenPanel` can accept `canChooseFiles` + `canChooseDirectories` together, but Windows' `IFileOpenDialog` cannot -- `FOS_PICKFOLDERS` is a mode switch, not an addition. Any one-control design must answer what Windows does.
- `PendingItemKind` already carries `'unknown'`, used today by native drop, with `copy.stage.pending.item` already written for it and existing code explaining why it must not be resolved early -- a control that does not yet know the kind costs nothing downstream.
- `IdleView.tsx` carries a prior defect worth not repeating: the drop zone once opened the file chooser on click, contradicting the "file or folder" instruction above it, and a live run hit it. Whatever replaces the two buttons must not recreate that behavior.
- `EXPERIENCE.md` limits modal depth to OS dialogs. A dropdown menu is permitted (it is not a modal) but would be the product's first floating surface, carrying `role="menu"` semantics, keyboard operation, Escape and focus return, the 44px target floor, and the one-announcement-owner rule -- real cost against the grain of the existing pattern.
- **Owner decision, hierarchy (2026-09-13):** the drop zone keeps the top of the page and its `h1`, visually calmed but not demoted. FR23 stays as written -- firewall guidance ahead of the selection controls -- so the browse control stays below it and must not be the loudest thing on the screen. Rejected: promoting browse above the firewall guidance (renegotiates a frozen requirement), and quieting both (leaves Idle with no confident affordance on the one screen where the user has not acted yet).
- AD-9 forbids a capability token, a selected filesystem path, or a session-selected name from reaching a diagnostic, a log, or any surface outside the process; a chooser-failure path must not leak the attempted path.
- **D-113:** `chooseWith`'s dialog-open failure currently reports `transfer_failed` ("the transfer stopped before FairDrop finished sending") for a transfer that never existed. Neither `setup_failed` (implies an item was chosen) nor `not_ready` (a second-instance state) fits, and no code was written for this case. Fixing it may require a new code, introduced through the same registry/contract/Go/TypeScript pin as any other new copy.
- **D-114:** a successful Copy replaces the "Copy download link" button's whole visible and accessible label with `copy.copy.confirmation` ("Copied") for the rest of the session, with no timer to revert it (`EXPERIENCE.md` bans frontend lifecycle timers). The one control that reaches the capability URL then loses the name that says what it does (WCAG 4.1.2). Every candidate fix is itself a design decision: swapping the label back needs a trigger `EXPERIENCE.md` does not sanction, keeping the accessible name while showing "Copied" breaks label-in-name, and moving the confirmation beside the button changes a control `DESIGN.md` lays out.

## UX & Interaction Patterns

- Banned outright: DOM file drop, drag-only input, hover-only actions, modal warnings, frontend lifecycle/reset timers, automatic clipboard clearing, celebratory animation, tray/always-on-top behavior, and navigation chrome. Modal depth is limited to OS dialogs.
- The accessibility floor is a product target with automated proof: unrounded contrast figures, 320 CSS px one-dimensional reflow, 200% text plus WCAG 1.4.12 text-spacing overrides (1.5x line height, 2x paragraph spacing, 0.12em letter spacing, 0.16em word spacing), a 44px activation floor on every interactive target, and exactly one announcement owner per transition.
- `copy.idle.instruction` ("Drop one file or folder.") is the fixed drop-zone copy today; it may need reconciling with a single labelled control, again by stable key.
- A non-empty chooser result stages immediately; an empty (cancelled) result stays quiet with no announcement, and native dialog cancel returns focus to the invoking control with no new speech -- this existing contract must survive the collapse to one control.
- `DESIGN.md` currently specifies the two Selection Controls as visually equal-weight, "no false primary choice"; collapsing them into one control changes that pairing and must still clear the unrounded-contrast and 44px-floor tests the accessibility suite already enforces.

## Cross-Story Dependencies

- D-113 (from Story 3.11) and D-114 (from Story 3.12) were both explicitly routed to this story by their origin stories because closing either is a design decision only a rebuild of the selection controls can make.
- Any label or copy this story introduces follows Story 3.5's cross-language pin: `EXPERIENCE.md`, `docs/fairdrop-contracts.md`, the Go error table, and the TypeScript mirror move together.
- Story 4.2 (done, released in v0.3.0) closed ten unrelated Epic 3 retrospective action items and shares no mechanism with 4.1's selection-control rebuild.
- Routing macOS through `NSOpenPanel` directly, bypassing Wails' dialog API for a true one-step chooser, is explicitly a later phase outside this story.
