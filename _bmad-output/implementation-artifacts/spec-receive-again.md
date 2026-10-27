---
title: 'Receive another upload into the same chosen folder'
type: 'feature'
created: '2026-10-26'
status: 'done'
baseline_commit: 'cebc6678fbe08743890e8a23addfc0b2a09be403'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-phone-to-desktop-receiving/receive-contract.md'
  - '{project-root}/_bmad-output/implementation-artifacts/spec-send-again.md'
---

<frozen-after-approval reason="owner selected highest-value proposed intent and explicitly requested GPT-6.1 Sol implementation">

## Intent

After a successful file upload, offer **Receive Again** to accept another upload into the same chosen parent folder without opening the chooser. Retire the completed session, revalidate through ordinary StartReceive, and display a fresh one-time code. Each upload still creates its own exclusive subfolder.

## Boundaries & Constraints

**Always:** One live session; retain only the approved parent path in private controller memory. Preserve normal destination validation, per-upload declared-size plus 3 GiB reserve checks, exclusive writes, Show in Folder and Done. Clear retention on user dismissal/cancellation, unmount, failed upload/new admission or replacement. Distinguish internal lease release from abandonment. Main specifies/reviews/verifies/integrates; GPT-6.1 Sol implements. Preserve build/.DS_Store.

**Ask First:** Persistent folders/history, automatic rearming, repeats after partial/cancelled uploads, or changed trust/storage guarantees.

**Never:** Reuse destination handles, subfolder, listener, token or recovery pathway; expose absolute paths in state/views/events/logs; offer this action for text or incomplete/cancelled outcomes; add dependencies, photo behavior, notifications, version bumps or releases. No remote operations by implementer.

## I/O & Edge-Case Matrix

| Scenario | Input/state | Expected behavior | Refusal |
| --- | --- | --- | --- |
| Success | Complete file outcome + remembered parent | Await terminal release and actual Idle; StartReceive original parent once; fresh code | No chooser or premature busy call |
| Admission | Same-tick/repeated activation | One pending repeat; other outcome actions cannot overlap | Direct invalid-state calls do nothing |
| Retire failure | Cancel rejects | No new start; truthful fixed command error and retryable outcome if still held | Never mislabel as Show in Folder failure |
| Revalidation | Parent missing, replaced by unsafe path, or unavailable | Ordinary bounded destination reopen refuses before new listener | Clear target; receive-specific error, choose folder again |
| New upload | Second session same parent | New exclusive subfolder and normal space check; old files unchanged | Old code cannot accept another upload |
| Abandonment | Done/cancel/unmount/replacement during waits | Forget path; stale continuations cannot start or resurrect a receiver | Pending start uses existing cancellation cleanup |
| Recovery | Waiting refresh followed by success | Preserve approved parent through successful refresh; repeat from new successful identity | Retired events cannot affect next session |
| Failure/other modes | Failed receive, cancelled outcome, text, send | No repeat action or retained receive target | Existing actions remain unchanged |

</frozen-after-approval>

## Code Map

- `frontend/src/transfer/useTransfer.ts`: startReceiveAt currently remembers no folder; add private parent retention, eligibility and guarded repeat. Reuse Send Again's synchronous admission, selection epoch, cancelImpl(true), waitForIdle, StageOperation and late-result cleanup. Clear every abandoned path and preserve only owned internal transitions.
- `frontend/src/transfer/state.ts`, `selectors.ts`: outcome contains basename/result only. Keep raw parent out; distinguish command-error purpose where needed. Existing receive errors always render Show in Folder copy, unsuitable for failed retirement.
- `frontend/src/App.tsx`, `ui/ReceiveOutcomeView.tsx`, `ui/copy.ts`: success-only action, pending guard, truthful error and existing focus/announcement routing. Receive Again primary, Show in Folder secondary, Done quiet; wrap three actions accessibly.
- `app.go` StartReceive and `internal/transfer/receive.go`: existing API reopens/pins destination and creates a fresh identity from Idle. No new bound command is needed. `internal/transfer/recovery.go` transfers a live destination, so do not reuse it for repeats.
- `internal/sink/sink.go`: OpenDestination validates; CheckSpace(declared) runs per upload. Do not add a setup-time free-space rule. Use fixtureDir in native tests.
- `useTransfer.receive.test.tsx`, App integration/focus tests, ReceiveOutcomeView tests and browser suites provide controlled promises and native-layout coverage. Existing file-receive unmount behavior need not broaden; clear retained memory and prevent new work after unmount.
- Current receiving SPEC/contract, `docs/fairdrop-contracts.md`, architecture, README, Quartz DESIGN/EXPERIENCE and roadmap need this narrow retention/action extension; historical release notes stay unchanged.

## Tasks & Acceptance

- [x] Controller: retain original approved parent through successful receive and waiting refresh; expose canReceiveAgain/receiveAgain; serialize admission and fence release/start continuations; clear on every abandoned/failed/replaced path.
- [x] App/view/state/copy: wire success-only action, busy behavior, truthful retirement errors and focus. Preserve Show in Folder and Done, including recoverable failed retirement.
- [x] Tests: cover every matrix row, repeated cycles and fresh clean metadata; real native receive composition proves separate identities/subfolders, old-code refusal, no overwrite and fresh validation/space checks.
- [x] Canonical mutation inventory: prove admission, actual-reset wait, original-parent reuse, successful-state guard, clearing and stale-work fences; run named baselines and assertion kills with unique complete logs. Wire into native desktop CI and workflow pins using existing runner path/UTF-8/ANSI safeguards.
- [x] Update current contracts/UX/README/roadmap and sibling evidence with matrix coverage, review and verification results. Keep publication baseline 1.8.0.

Given a completed upload, when Receive Again runs, then the user gets a new code for the same revalidated parent without a chooser and the next upload cannot overwrite the previous one. Given abandonment, overlap or failed retirement, when callbacks finish, then no unwanted receiver or stale result appears. Given a missing/unsafe folder, when repeated admission occurs, then normal receive refusal applies and no stale target remains available.

## Evidence

See evidence-receive-again.md beside this spec; retain matrix, mutation and review records there.

## Spec Change Log

## Verification

Run focused controller/App/view tests and native composition tests before the full ordered AGENTS gate. Wails precedes standalone frontend checks; never run them concurrently in one tree. Execute canonical inventories, not a copied subset. The existing receive-text runner demonstrates Windows-safe canonical workspace paths, explicit UTF-8 decoding and ANSI-tolerant named-assertion parsing. Main owns full sequential gate, independent three-layer review, verified commit/push and exact-head Windows/macOS/Linux CI before integration. Manual WKWebView/phone interaction is optional and remains unverified unless performed. No story_key; sprint synchronization does not apply.

## Suggested Review Order

**Repeat lifecycle**

- Retire, await confirmed Idle, and revalidate the privately retained parent.
  [useTransfer.ts:1009](../../frontend/src/transfer/useTransfer.ts#L1009)

- Bound missing reset confirmation without authorizing a late restart.
  [useTransfer.ts:1173](../../frontend/src/transfer/useTransfer.ts#L1173)


**Outcome controls**

- Serialize visible outcome actions through the existing App action lock.
  [App.tsx:548](../../frontend/src/App.tsx#L548)

- Offer success-only repetition and preserve focus after opening the saved folder.
  [ReceiveOutcomeView.tsx:42](../../frontend/src/ui/ReceiveOutcomeView.tsx#L42)


**Supporting proof**

- Prove fresh capabilities, exclusive folders, preserved files and destination revalidation.
  [receive_native_test.go:495](../../receive_native_test.go#L495)

- Exercise the same-batch race that previously discarded a replacement parent.
  [useTransfer.receive.test.tsx:956](../../frontend/src/transfer/useTransfer.receive.test.tsx#L956)

- Verify App wiring, competing controls and restored focus using deferred commands.
  [App.receiveAgain.test.tsx:111](../../frontend/src/App.receiveAgain.test.tsx#L111)

- Keep all nineteen named assertion mutations in one canonical inventory.
  [verify-receive-again-mutations.py:18](../../scripts/verify-receive-again-mutations.py#L18)

- Execute and retain mutation proof on native desktop CI.
  [verify.yml:270](../../.github/workflows/verify.yml#L270)
