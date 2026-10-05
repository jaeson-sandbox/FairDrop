---
title: 'Send the successfully delivered item again'
type: feature
created: '2026-10-04'
status: done
review_loop_iteration: 0
baseline_commit: 128ec4a4ee6d5a118265ff9a4b70edcf43738b89
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-receiver-handoff/SPEC.md'
---

<frozen-after-approval reason="owner authorized sequential implementation after specification, 2026-10-04">

## Intent

**Problem:** After sending an item, the sender must choose it again to hand it to another person, although the controller already has an ephemeral selection mechanism for error retry.

**Approach:** Add Send Again to successful outcome cards. Keep the previous selection in controller memory through success and its retained outcome; re-stage normally for a fresh offer. This is CAP-2, sequentially after the receiver landing-page story.

## Boundaries & Constraints

**Always:** Keep one live session and one receiver; revalidate through ordinary Stage, yielding a new token/QR. Retain only the selection path in controller memory, never a payload copy, handle, old URL or persistent history. Preserve Send Another, Done dismissal and error retry. Distinguish user cancellation/dismissal from the internal terminal-lease release needed to stage again. Existing focus, announcements, reduced-motion and native drop behavior apply.

**Ask First:** Persisting history/settings, automatic repeat offers, multiple simultaneous receivers or changing terminal lifecycle semantics.

**Never:** Reuse an old capability, bypass source revalidation, retain paths in reducer/view state or storage, automatically open the chooser, or implement remaining roadmap slices here. No release/version bump.

## I/O & Edge-Case Matrix

| Scenario | Input / State | Expected Output / Behavior | Error Handling |
| --- | --- | --- | --- |
| Live success | Done with remembered selection; Send Again | Release terminal lease, await actual reset, Stage same path once | No premature busy Stage |
| Retained success | Idle with retained Done; Send Again | Stage same path directly with fresh result | No chooser |
| Missing target | Done without remembered selection | No active Send Again; Send Another and Done remain | No invalid path command |
| Changed item | Remembered file deleted/replaced/unsafe | Normal Stage validates and refuses as applicable | Existing typed error/recovery, never replay old payload |
| Double activation | Two calls/clicks while re-staging | One operation/Stage; visible controls busy | No queued second send |
| Dismiss/cancel | User Done/Dismiss, cancel during preparation/transfer, or unmount | Forget remembered target and prevent late action | No later stale Stage |
| Replacement | New valid selection after previous success | Remember only the new selection | Cancelled chooser follows existing quiet behavior |
| Other states | Idle without receipt, pending, staged, transferring, error | Send Again does nothing | Error retry remains its separate permitted action |

</frozen-after-approval>

## Code Map

- `frontend/src/transfer/useTransfer.ts` — rememberedPathRef, current success-clearing render block, stageFromOutcome, retry, cancel and dismissRetained; reuse Stage/lease release without conflating internal release and user abandonment.
- `frontend/src/transfer/useTransfer.test.tsx` — Story 9.2 memory/ref tests and Story 9.6 live terminal lease/reset tests. Replace the explicitly superseded clear-on-success assertion; preserve privacy and other clearing proofs.
- `native_matrix_test.go` and `internal/transfer/coordinator_stage_test.go` — real App/source/server harness and independent identifier generation; reuse these for fresh-session/old-capability proof.
- `frontend/src/App.tsx` — doneCardProps and outcomeActionPending; wire new action through the controller and busy guard.
- `frontend/src/ui/OutcomePanel.tsx` — Done action group, Send Another BrowseControl and quiet Done; add Send Again as primary when available, preserve existing chooser action.
- `frontend/src/ui/copy.ts`, `OutcomePanel.test.tsx`, `App.test.tsx`, `App.focus.test.tsx`, `App.harness.tsx` — shared copy, controller fixtures, rendering, interaction and focus.
- `frontend/browser/accessibility.test.tsx`, `frontend/src/style.css` — actual layout at minimum width, 200% text, keyboard and forced colors; three actions must wrap without clipping or losing targets.
- Quartz `EXPERIENCE.md`, `DESIGN.md`, current `docs/fairdrop-architecture.md`, `README.md` — current terminal actions, memory lifetime and product instructions; historical evidence remains historical.

## Tasks & Acceptance

**Execution:**
- [x] `useTransfer.ts` — expose guarded sendAgain/canSendAgain, retain success target and correctly forget it on user abandonment; keep normal validation and unique session creation.
- [x] `useTransfer.test.tsx` — cover each matrix row, including delayed reset, overlapping calls, stale lifecycle events and unmount; use deterministic controlled promises, not sleeps.
- [x] `App.tsx`, harness and controller mocks — wire Send Again through outcome busy handling without changing retry or native drop.
- [x] `OutcomePanel.tsx`, `copy.ts` and current UX spine — label and arrange three Done actions accessibly; Send Again primary, Send Another secondary, Done quiet.
- [x] Component/App/browser tests — prove actual user activation re-stages once and keyboard/focus/320px/200%/forced-colors behavior remains sound.
- [x] Current docs and sibling evidence — record changed memory lifetime, matrix coverage, meaningful mutations and complete verification output.

**Acceptance Criteria:**
- Given a successful file or folder transfer, when Send Again is activated on live or retained success, then ordinary Stage receives the original path once without opening a chooser and the new returned code is displayed.
- Given a real re-stage of the same item, when its next download completes, then it uses a distinct session/token and the old retired listener cannot deliver again; retain existing backend token-generation proof rather than fabricating a frontend guarantee.
- Given user abandonment or replacement, when old callbacks/events arrive, then they cannot resurrect or send the old selection.
- Given narrow layout and keyboard operation, when the success card offers all three actions, then controls remain visible, distinguishable and operable with no duplicate announcement.

## Evidence

[evidence-send-again.md](evidence-send-again.md)

## Spec Change Log

- 2026-10-04: Initial plan under owner-authorized sequential roadmap; implementation starts after receiver landing-page verification.

## Design Notes

A remembered path is permission to revalidate, not a snapshot. A cancelled chooser preserves the existing outcome; only an admitted new selection replaces its target. Internal lease release may clear global memory only if the intended operation still safely owns its captured target. Avoid relying solely on asynchronous React state to prevent double activation.

Main orchestrator owns commits, remote operations and review dispatch. Implementer runs local verification sequentially and reports; no additional agent dispatch or Git milestone operations.

## Verification

Targeted controller/App/outcome tests and rendered accessibility tests, then canonical sequential gate from AGENTS.md: Wails, drift, format/vet/staticcheck, Go, cgo/race, frontend, browser and LF; Darwin/Linux preflights. Mutation tests must demonstrate success retention, state guard, single-operation admission and abandonment checks fail when broken. Keep complete logs; do not claim a native interaction pass from Chromium.

## Suggested Review Order

- Follow state admission, remembered selection and fresh staging.
  [useTransfer.ts:444](../../frontend/src/transfer/useTransfer.ts#L444)

- Inspect recoverable lease release and user-abandonment handling.
  [useTransfer.ts:300](../../frontend/src/transfer/useTransfer.ts#L300)

- See how one action owns the busy UI until it settles.
  [App.tsx:73](../../frontend/src/App.tsx#L73)

- Review the three accessible success actions.
  [OutcomePanel.tsx:171](../../frontend/src/ui/OutcomePanel.tsx#L171)

- Trace a real controller from click to fresh QR and link.
  [App.sendAgain.test.tsx:54](../../frontend/src/App.sendAgain.test.tsx#L54)

- Check keyboard and forced-colors behavior with actual layout.
  [accessibility.test.tsx:1571](../../frontend/browser/accessibility.test.tsx#L1571)

- Review the executable mutation inventory and evidence.
  [mutation-send-again.py:1](mutation-send-again.py#L1)

- Read matrix coverage, limitations and review disposition.
  [evidence-send-again.md:1](evidence-send-again.md#L1)
