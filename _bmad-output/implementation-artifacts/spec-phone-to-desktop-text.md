---
title: 'Receive text and links from a phone browser'
type: 'feature'
created: '2026-10-17'
status: 'done'
baseline_commit: '65f387336d95ad940cd8a644e597c44e1b8366a5'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/_bmad-output/specs/spec-phone-to-desktop-text/SPEC.md'
  - '{project-root}/_bmad-output/specs/spec-phone-to-desktop-text/receive-text-contract.md'
  - '{project-root}/docs/release-policy.md'
---

<frozen-after-approval reason="owner selected the proposed feature and authorized spec-first Sol implementation without repeat approvals">

## Intent

Receive a short note or URL from a phone browser and explicitly copy it on the desktop. Add a distinct Receive text entry, script-free phone form, and held selectable desktop outcome using the canonical receive-text contract and its companions.

## Boundaries & Constraints

**Always:** One live session; bounded RAM; existing NormalizeText rules; native explicit clipboard writes; exact-token routing; payload-free events/metadata; stale-response fencing; waiting recovery; Quartz focus and announcements. Canonical companion defines transport limits, commit/acknowledgement semantics and read/copy ownership. Preserve build/.DS_Store. Main owns review, full gate, commits/push and integration; Sol implements and records focused proof.

**Ask First:** Changes to trust model, persistence, dependencies or user-facing goal; an unresolved contract contradiction.

**Never:** Automatic clipboard access, browser storage, file staging, URL navigation, partial-value preview, photo changes, version/release changes, weakened tests, payload diagnostics, or subagent remote operations. Do not widen CopyToClipboard's link cap.

## I/O & Edge-Case Matrix

| Scenario | Input/state | Expected behavior | Refusal |
| --- | --- | --- | --- |
| Entry/GET | Idle; valid unclaimed URL | Empty form; no folder/I/O/claim | Busy excludes other modes |
| Valid text | One bounded UTF-8 form value | Normalize, authorize once, commit once, held exact preview | No implicit copy |
| Bad input | Blank/control/UTF-8/form/duplicate fields | No commit; code reusable | Fixed 400; encoded oversize 413 |
| Stream/race | Partial body, simultaneous POST, Cancel/refresh | One bounded reader; no partial/stale commit | Competing POST 423; stale route 404 |
| Methods/token | HEAD/other/wrong path or token | No disclosure or side effects | Generic 404 |
| Receipt/ack | Commit succeeds then phone response fails | Desktop receipt remains truthful | No delivery/copy claim |
| Read/copy | Correct committed ID versus empty/stale ID | Exact native copy; failure can retry | Fixed safe refusal; no value leak |
| Clearing | Done/Cancel/unmount and late promises | Clear owned value; ignore obsolete results | No resurrected text/Copied |
| Recovery | Waiting network refresh | New ID/token, empty receiver | Old code retired; existing recovery errors |

</frozen-after-approval>

## Code Map

- `internal/transfer/coordinator_text.go`, `text.go`: existing StageText/NormalizeText; retain validation rules. `coordinator.go` owns session. `receive.go`, `outcomes.go`, `lifecycle.go`: receive admission, held outcomes and teardown; publish precedes final state store, so getters must handle early terminal observation.
- `internal/transfer/ports.go`, `types.go`, `recovery.go`: explicit server mode, payload-free metadata and recovery discriminants. Add bounded RAM commit ownership without filesystem SinkPort.
- `internal/server/lifecycle.go`, `handler.go`, `upload.go`, `text.go`: start validation, exclusive routing, preclaim refusal, receipt-versus-ack precedent and secure page headers. Add `receive_text.go` and focused HTTP tests.
- `app.go`: bound command/error boundary and native ClipboardSetText. Generic CopyToClipboard is capped at 2,048 bytes for links; add session-owned StartReceiveText/GetReceivedText/CopyReceivedText instead.
- `frontend/src/transfer/{types,validation,state,useTransfer}.ts`, `App.tsx`: strict DTO/event validation, generation fences, lifecycle/focus routing. `ui/{IdleView,TextViews,ReceiveWaitingView}.tsx`, `copy.ts`, `style.css`: reusable Quartz composition.
- `app_receive_test.go`, `internal/transfer/coordinator_receive_test.go`, `internal/server/text_test.go`, frontend transfer/browser suites: literal wire contracts, deterministic barriers, live form fixtures. `scripts/verify-native-mutations.sh` consumes scoped inventories; preserve workflow pins.

## Tasks & Acceptance

- [x] Implement bounded RAM receive admission/commit/read and held lifecycle in transfer; extend recovery without changing existing modes.
- [x] Implement exact route and script-free bounded form, processing reservation, validation-before-claim, atomic commit and fixed result/error pages in server.
- [x] Add bound session-guarded read/copy commands in App; regenerate Wails bindings through build before frontend checks.
- [x] Extend strict frontend state/controller parsing and UI for entry, waiting, receipt, copy failure/retry, Done and recovery; fence late read/copy results and clear owned references.
- [x] Cover every matrix row, including real HTTP-to-desktop value/encoding, literal markup, no I/O/payload events, commit/Cancel races, clipboard failures and early terminal delivery. Add canonical scoped Go/frontend mutations with unique logs and an evidence matrix.
- [x] Amend README, current contracts/architecture, Quartz UX spines and roadmap for this extension; leave historical release notes and photo flow unchanged.

Given valid submitted text, when the current session commits it, then the desktop can inspect/copy the exact normalized value without a file. Given malformed, incomplete or stale input, when processing ends, then no readable value or success is created. Given Done/cancellation/recovery and late asynchronous work, when it returns, then the new or idle UI contains no obsolete value or success announcement. Given a clipboard failure, when retry succeeds, then only that actual success announces Copied and receipt remains held until Done.

## Evidence

See evidence-phone-to-desktop-text.md beside this spec; keep matrix coverage, mutations, reviews and full logs there.

## Spec Change Log

## Verification

Focused Go, controller and UI tests; Wails build before frontend suite. Main runs the complete ordered AGENTS gate in an isolated snapshot, including native mutations and new scoped mutations; native Windows/macOS/Linux exact-head CI before merge. Every load-bearing guarantee needs a named failing mutation. Manual UI/phone observations are optional and must not be invented. No story_key: sprint synchronization is inapplicable.

## Suggested Review Order

**Entry and session ownership**

- Expose a distinct native entry without filesystem staging.
  [app.go:380](../../app.go#L380)

- Keep accepted text bounded, session-owned, and readable only after successful receipt.
  [coordinator_receive_text.go:47](../../internal/transfer/coordinator_receive_text.go#L47)

**Transport and cancellation**

- Validate the entire form before authorizing and committing one receipt.
  [receive_text.go:30](../../internal/server/receive_text.go#L30)

**Desktop lifetime and interaction**

- Fence late reads and copies across Done, recovery, and replacement.
  [useTransfer.ts:633](../../frontend/src/transfer/useTransfer.ts#L633)

- Present selectable text with explicit native copy and recoverable errors.
  [ReceiveTextViews.tsx:88](../../frontend/src/ui/ReceiveTextViews.tsx#L88)

**Verification and integration**

- Prove browser-to-native receipt and stale-ID clipboard refusal.
  [receive_text_native_test.go:18](../../receive_text_native_test.go#L18)

- Exercise production phone markup and browser encoding.
  [receiver-live.mjs:89](../../frontend/browser/receiver-live.mjs#L89)

- Keep named safety mutations reproducible in CI.
  [verify-receive-text-mutations.py:1](../../scripts/verify-receive-text-mutations.py#L1)
