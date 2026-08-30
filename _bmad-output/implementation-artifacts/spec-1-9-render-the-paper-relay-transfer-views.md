---
title: 'Story 1.9 — Render the Paper Relay Transfer Views'
type: 'feature'
created: '2026-08-30'
status: 'implemented'
review_loop_iteration: 0
baseline_commit: '19ad4ee9b8fdbacfb16364192fcaf4d047105ae0'
context:
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/DESIGN.md'
  - '{project-root}/_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/EXPERIENCE.md'
  - '{project-root}/_bmad-output/implementation-artifacts/epic-1-context.md'
---

<frozen-after-approval reason="human-owned intent — do not modify unless human renegotiates">

## Intent

**Problem:** Story 1.8 landed a correct, defended transfer state machine that nothing renders. `App.tsx` is a dark placeholder with one unstyled drop rectangle: no visual system, no view per phase, no QR, no link, no progress, no outcome, and no way to browse for an item at all — the two bound dialog commands have never been called.

**Approach:** Author the Terracotta Linen token layer and the five phase views that consume the Story 1.8 controller and selectors. Every literal string comes from one copy registry keyed by `EXPERIENCE.md` stable keys; every visual decision comes from `DESIGN.md` tokens. The controller gains the two browse commands so a dialog result reaches Stage exactly as a native drop does.

## Boundaries & Constraints

**Always:** Render from `state`/selectors only — no phase inferred from paths, promises, animation, or elapsed time. Take every literal string from the copy registry by stable key, including error headings and the fixed `PublicError.message` verbatim. Take every color, type ramp, radius, and spacing value from the `DESIGN.md` token set through Tailwind v4 CSS variables. Prepend `data:image/png;base64,` only at render. Keep the inherited `--wails-drop-target: drop` gate and `OnFileDrop(cb, true)`. Give every activation target ≥44px in both dimensions. Follow the OS color scheme with no theme control and no opposite-theme flash. Reflow to one column at 320 CSS px with vertical scrolling as the only overflow.

**Ask First:** Any Go, binding, or `main.go` change; any new runtime dependency; any string not in the registry or any wording change to one; any persistence, frontend lifecycle timer, or clipboard clearing; any change to Story 1.8's reducer transitions, correlation, or reset rules; adding a retained outcome kind.

**Never:** Never add a DOM drop handler or class-only drop gate, spell or expose the capability token, render `cancelled` as an Error, render a sender-side activation link, show transfer history, claim receiver identity, storage, encryption, or a claim, use banned vocabulary (secure, private, pair, sync, AirDrop naming, universal compatibility), animate celebration/sweep/shimmer/blink, let an animation callback own lifecycle state, edit generated bindings, or add Tailwind v3/PostCSS config.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
|----------|--------------|---------------------------|----------------|
| Idle at rest | `phase: 'idle'`, no retained outcome | Firewall preflight, then drop target with `copy.idle.instruction`, then equal File/Directory controls, in document order | No history, no QR, no session control |
| Browse | Dialog resolves non-empty / empty / rejects | Non-empty stages immediately; empty is silent; rejection shows the fixed Error Panel | Empty result produces no message and no state change |
| Invalid drop | Zero, multiple, or non-string paths | Fixed `invalid_selection` Error Panel; nothing staged | Never stages the first item |
| Stage pending | `phase: 'pending'` | Pending Card naming the kind via `copy.stage.pending.file`/`.folder`, plus `copy.cancel.preparation` | No QR, no session control, no STAGED claim |
| Staged | `phase: 'staged'` | Heading, bidi-isolated name with logical size, folder note, QR primary, readonly URL row, disclosures, Cancel | `beacon_warning` renders the non-terminal Warning Banner |
| Copy link | Clipboard write resolves / rejects | Label becomes `copy.copy.confirmation`; failure leaves the action label unchanged | No toast, no focus move, no clipboard clearing |
| Transferring | Three progress modes | Determinate meter; static non-directional pattern with `copy.progress.unknown`; `copy.progress.known_empty` with no percentage bar | Metrics show wire `bytesSent`; known-empty omits speed and percentage |
| Terminal and retained | `done` / `error` / retained Idle | Done copy, or the fixed heading and exact registry message; retained node stays with `copy.outcome.dismiss` | No frontend timer removes it; `cancelled` is never an Error |
| Reflow | ≥760px, 640–759px, 320px effective | Details may sit beside the QR; then QR above URL and actions; then one column | No horizontal page scroll, no clipped action |

</frozen-after-approval>

## Code Map

- `_bmad-output/planning-artifacts/ux-designs/ux-FairDrop-2026-08-23/DESIGN.md` — binding token set (frontmatter `colors`/`typography`/`rounded`/`spacing`/`components`), contrast table, component visual contracts. Read-only.
- `.../EXPERIENCE.md:65` — the copy registry; `:109` the error heading/message table; `:128` component behavioural rules; `:161` state patterns. Read-only, and the source of every string.
- `.../mockups/{key-idle-preparing,key-staged-folder,key-transferring,key-outcomes}.html` — production reference for all five compositions; each names the spine rows it implements. Copy structure and CSS values from these; they are not runnable app code.
- `frontend/src/transfer/state.ts:60` — `TransferState` union; `:83` initial state. The render input. Do not modify.
- `frontend/src/transfer/selectors.ts:126` — `ProgressSelection` union already supplies the three modes; `:192` `selectMetadata`; `:196` `selectRetainedOutcome`; `:200` `selectVisibleError`. Extend only additively.
- `frontend/src/transfer/useTransfer.ts:20` — `TransferController`; `:86` `stage`; `:131` `cancel`. Add the two browse commands here; the controller stays the single command owner.
- `frontend/src/transfer/errors.ts:34` — `fixedErrorMessages` already holds the twelve exact messages. Reuse; never restate them.
- `frontend/wailsjs/go/main/App.d.ts` — `SelectFile()`/`SelectDirectory()` are bound and have never been called. Generated; do not edit.
- `frontend/src/App.tsx:1` — current placeholder; `App.test.tsx:63` — the Phase 1 drop-gate proofs that must survive.
- `frontend/src/style.css:1` — `@import "tailwindcss"` and the Nunito face. Tailwind v4 via the Vite plugin; no config file exists and none is wanted.
- `frontend/package.json:19` — `framer-motion` is already a locked dependency; using it needs no new install.

## Tasks & Acceptance

**Execution:**
- [x] `frontend/src/style.css` — declare the Terracotta Linen tokens in a Tailwind v4 `@theme` block with a `prefers-color-scheme` dark pair and `color-scheme: light dark`, so first paint cannot flash the opposite theme.
- [x] `frontend/src/ui/copy.ts` — one frozen registry mapping every `EXPERIENCE.md` stable key to its exact string, plus the fixed error headings; re-export the error messages from `errors.ts` rather than duplicating them.
- [x] `frontend/src/transfer/useTransfer.ts` — add `selectFile`/`selectDirectory`, sharing the Stage generation guard: a non-empty result stages, an empty result is silent, a rejection becomes a command error.
- [x] `frontend/src/transfer/selectors.ts` — add only what the views need to stay logic-free (pending item kind, staged warnings, retained-outcome presentation).
- [x] `frontend/src/ui/IdleView.tsx` — firewall preflight, drop target, browse controls, retained outcome with `copy.outcome.dismiss`.
- [x] `frontend/src/ui/StagePendingCard.tsx` — kind-specific preparing copy and the preparation Cancel.
- [x] `frontend/src/ui/StagedView.tsx` — item summary, QR panel, direct URL row with copy action and feedback, disclosures, warning banner, Cancel.
- [x] `frontend/src/ui/TransferringView.tsx` — the three progress presentations and the wire-byte metrics.
- [x] `frontend/src/ui/OutcomePanel.tsx` — Done and Error panels, shared by the terminal phases and the retained Idle node.
- [x] `frontend/src/App.tsx` — compose one view per phase behind the existing controller and drop gate; keep the inherited CSS gate untouched.
- [x] `frontend/src/ui/*.test.tsx` — cover every matrix row, all three progress modes, the copy registry values as literals, QR rendering, and the reflow breakpoints.

**Acceptance Criteria:**
- Given any rendered view, when its text is read, then every string equals a registry value character for character, and no banned term or raw adapter text appears.
- Given a staged session, when the DOM is serialized, then the capability token appears only inside the `data:` QR source and the readonly URL value, and never as prose or an anchor target.
- Given each phase in turn, when the view mounts, then exactly one phase view is present and it renders only from state the reducer owns.
- Given a token value used by a view, when it is traced, then it resolves to a `DESIGN.md` variable rather than a literal color, size, or radius.
- Given 320 CSS pixels of effective width, when the layout reflows, then no page-level horizontal scrollbar exists and every action stays reachable at 44px.

## Spec Change Log

Implementation decisions taken inside the approved boundaries, recorded so a
reviewer can find them without a diff hunt. None of them changes the frozen
intent; each is a place where the spec left one degree of freedom.

- **A chooser rejection reaches Idle through the existing transitions.** The
  matrix requires the fixed Error Panel, `state.ts` is marked do-not-modify, and
  the reducer's only route to an Idle `commandError` is `stage-requested` →
  `stage-failed`. `browse` therefore dispatches both halves back to back in the
  catch. React applies them in one batch, so Pending is reduced but never
  rendered; `useTransfer.test.tsx` records every rendered phase and asserts
  `'pending'` is not among them. No reducer transition, correlation rule, or
  reset rule was touched.
- **The browse commands hold their own operation slot.** Sharing
  `stageOperationRef` would have made the unmount path owe a `CancelTransfer`
  for a session that was never staged. `browseOperationRef` blocks a second
  chooser and a native drop for the same window, and a test pins that no
  `CancelTransfer` is issued for an abandoned chooser.
- **A `'unknown'` pending kind shows no kind tab and the file wording.** A
  native drop supplies a path only, and the registry has no kind-neutral
  preparing string. Epic 1's source adapter accepts regular files, so a dropped
  folder fails validation a moment later with its own fixed copy. Epic 2 should
  revisit this line when directories become stageable by drop.
- **Two files beyond the Execution list.** `frontend/src/ui/format.ts` holds the
  byte and rate presentation shared by the item summary and the metrics, built
  from `copy.unit`; `frontend/src/ui/styles.test.ts` proves the token layer, the
  44px floor and the reflow breakpoints against the stylesheet, because jsdom
  performs no layout and evaluates no media query.
- **`copy.label` and `copy.unit`.** The Voice-and-Tone table does not tabulate
  control names, the firewall block's own headings, metric captions, or byte
  units, but the views need them. They live in the same frozen registry, in
  their own group, each carrying its source in a comment.
- **Component styling is authored CSS over the `@theme` tokens.** Tailwind v4
  declares the token layer; the Paper Relay components (dashed drop zone, packet
  tab, static unknown-progress pattern, paper offset) read those variables from
  `.fd-*` rules rather than utility strings. Nothing outside the two token
  blocks contains a color, and a test enforces that.
- **`@font-face` removed from `style.css`.** DESIGN.md allows system-safe stacks
  only, and no view named the bundled face any more. The asset files are left in
  place untouched.
- **1.10's surfaces are registered but not rendered.** `copy.help.*`, the
  firewall recovery strings, `copy.cancel.won`, `copy.name.show_full` and
  `copy.external.promise` are in the registry as the task requires, and no view
  renders them. The Status Announcer is pre-mounted, atomic and empty; 1.10
  gives it its content and its focus routes.

## Design Notes

Story 1.10 owns the accessibility contract, not this story: announcement ownership and focus routing, `role="progressbar"` ARIA semantics, the assistive speech throttle, `prefers-reduced-motion` behaviour, forced colors, unrounded contrast proof, `copy.name.show_full`, `aria-disabled` cancel-pending semantics, and the `copy.help.*` and firewall-recovery surfaces. Render the semantic elements and their copy here; do not invent the routing 1.10 will specify.

`copy.cancel.won` is deliberately absent. Story 1.8 rejected adding a third retained outcome, so `transfer-reset` from Staged or Transferring returns plain Idle. Story 1.10 owns that summary and its focus route.

The three progress modes already exist as `ProgressSelection.mode`. Render from that discriminator; never divide, and never recompute a percentage in a component.

`beacon_warning` reaches the view through `metadata.warnings`, already reduced to fixed copy by `parseWarning`. It is a warning, not an error: the QR and link stay usable and no phase changes.

Clipboard copy uses `navigator.clipboard.writeText`; a rejection must leave the action label unchanged rather than claim a copy that did not happen. Wails' generated runtime exposes no clipboard helper, and adding one would be a Go change.

## Verification

**Commands:**
- `cd frontend && npm.cmd test` — expected: every existing test still passes alongside the new view tests.
- `cd frontend && npm.cmd run build` — expected: `tsc` and Vite production build clean.
- `wails build` — expected: bindings regenerate unchanged and the binary builds.
- `go test -count=1 ./... && go vet ./...` — expected: unchanged; this story touches no Go.
- `git diff --check` — expected: clean; no Tailwind v3 or PostCSS file appears.

**Manual checks (if no CLI):**
- Launch `build/bin/fairdrop.exe`, stage a real file, and confirm the QR renders square and scannable, the URL row is readonly, and light/dark follow the OS with no flash on first paint.
