---
title: 'Keep a waiting upload claimable while publishing a refusal notice'
type: 'bugfix'
created: '2026-10-14'
status: 'done'
baseline_commit: '040bb30d77c99f965d6c50c5f0c12ecc54173692'
review_loop_iteration: 0
context:
  - '{project-root}/AGENTS.md'
  - '{project-root}/docs/fairdrop-contracts.md'
  - '{project-root}/docs/fairdrop-architecture.md'
  - '{project-root}/_bmad-output/specs/spec-phone-to-desktop-receiving/receive-contract.md'
  - '{project-root}/_bmad-output/specs/spec-connection-recovery/recovery-contract.md'
---

<frozen-after-approval reason="owner authorizes continuing roadmap work and necessary fixes without repeated approval">

## Intent

**Problem:** Main CI at 040bb30 failed because AuthorizeClaim refuses a valid waiting upload while a refusal notice owns the operation lease. The HTTP handler has already reserved the capability, so an incidental notification permanently consumes an otherwise usable link.

**Approach:** Preserve the existing receive contract: a refusal notice leaves a session claimable. A claim encountering notice publication waits for that specific operation under a bound, then revalidates before committing. Preserve event ordering, claim/recovery winners, cancellation, and all existing resource ownership.

## Boundaries & Constraints

**Always:** Write a deterministic failing regression before the fix. Main orchestrates; Sol implements. Preserve the one-way HTTP reservation and coordinator authorization boundary, exact ID/generation/context checks, started-before-response ordering, bounded waits and no external calls under the state mutex. Claims may wait only for positively identified notice publication, never indiscriminately for setup, claim, recovery or teardown. Keep the entire wait bounded even across successive notices. Revalidate after each wait; Cancel, Shutdown and recovery remain able to win their existing ownership boundary. No invented success or swallowed cleanup failure.

**Ask First:** Any change to one-shot claim semantics, new dependency, persistent state, or additional independently shippable behavior.

**Never:** Fix only the flaky test by sleeping or relaxing assertions; retry every authorization error; reset the server's claimed flag; permit concurrent publication to reorder notice and started events; hold the coordinator mutex while waiting or publishing. Do not implement text sharing, change release versions, commit or push. Preserve unrelated build/.DS_Store.

## I/O & Edge-Case Matrix

| Scenario | Input/state | Expected behavior | Error handling |
| --- | --- | --- | --- |
| Notice/claim overlap | Matching STAGED receive; observer blocked publishing notice | Claim waits, then commits once; notice sequence precedes started | Never return a false cancellation merely for notice lease |
| HTTP overlap | Valid upload reserved while notice is in flight | Same capability completes a valid upload after notice finishes | No premature 404 or permanently spent waiting link |
| Cancellation | Caller or session cancelled while claim waits | Prompt refusal, no started event or write | Existing safe cancellation/shutdown semantics |
| Recovery or newer session | Old claim wakes after state/generation changes | Revalidation refuses old session | Never claims replacement |
| Other lease owners | Setup, claim, recovery, teardown | Preserve existing refusal/winner semantics | No new deadlock with Server.Stop waiting for handlers |
| Stuck notice | Observer does not return | Bounded claim wait; no false success | Named coded failure, deterministic timer seam |
| Repeated notices/claims | New notice arrives or competing claims wait | One claim wins; whole wait remains bounded | No duplicate started event or reset of total deadline |

</frozen-after-approval>

## Code Map

- `internal/transfer/receive.go:264`: forwardNotice acquires lease, validates STAGED receive, increments sequence, publishes outside mutex, then releases lease. This is the precisely identified owner a claim may await.
- `internal/transfer/coordinator.go:634`: AuthorizeClaim currently revalidates then nonblockingly acquires lease. A busy lease returns cancelled, even when only notice publication holds it. Preserve all downstream CLAIMING, beacon-stop and TRANSFERRING behavior.
- `internal/transfer/session.go`, `bounded.go`: keep any notice completion signal session-scoped; reuse deterministic bounded-wait mechanisms and caller/session cancellation. Avoid broad lease acquisition changes.
- `internal/server/upload.go:237`: claimed CAS precedes AuthorizeClaim and is irrevocable. Preserve this boundary; proof must exercise it.
- `internal/transfer/coordinator_receive_test.go:698`: the observed failing test waits only for fake observer append, not lease release. Add a controlled observer barrier that deterministically exposes the defect; retain its original claimable-session assertion.
- `internal/transfer/helpers_test.go` and claim/lifecycle/recovery tests: existing harnesses assert external adapter calls occur without mutex held. Preserve their ordering and ownership checks.
- `scripts/verify-native-mutations.sh`, `scripts/mutationverdict`: canonical structured verdicts reject compilation, timeout and unrelated errors as mutation kills.
- Full main CI failure: `/private/tmp/fairdrop-main-040bb30-failure.log`, run 38331350967. Only macOS race test failed; later mutation failures in that log are intentional. A 1000-repeat focused local race run passed, demonstrating why a deterministic barrier is required.

## Tasks & Acceptance

Timeout clarification: record a session-scoped failure when the notice wait bound expires and refuse further claims for that session. Once the blocked publication returns, settle the failure through the existing terminal path, revalidating cancellation/recovery ownership. Do not leave a spent capability displayed as a healthy waiting session after publication resumes. A permanently blocked observer cannot receive another UI event; report that limitation honestly. Never perform synchronous server teardown on the authorizing HTTP goroutine or join the drainer from itself.

**Execution:**
- [x] Add a deterministic notice-publication/claim regression and preserve its failing output before implementation.
- [x] Implement notice-specific bounded waiting and post-wait revalidation in transfer coordinator/session code.
- [x] Cover every matrix row, including real HTTP reservation and deterministic cancellation, stale generation and bound exhaustion.
- [x] Add a canonical scoped mutation inventory; execute named baselines/kills in isolated copies and retain complete logs.
- [x] Update active contracts/architecture for the clarified wait; keep existing receive and recovery guarantees intact.
- [x] Run focused Go/race checks and the complete local verification gate, with Wails before frontend and no concurrent mutation of shared code.

**Acceptance Criteria:**
- Given an oversized-upload refusal being announced, when a valid upload arrives, then it remains pending until the notice finishes and succeeds through the same one-shot link.
- Given cancellation, replacement or an elapsed bound while a claim waits, when publication finishes later, then no abandoned claim starts or writes.
- Given the pre-fix admission path injected back into isolated source, when the deterministic regression runs, then a named assertion fails rather than a compiler or harness error.

## Evidence

Record matrix coverage, red/green and mutation logs, review triage and gate results in [implementation evidence](evidence-receive-notice-claim.md).

## Spec Change Log

- 2026-10-14: Clarified the stuck-notice acceptance path during implementation: a returned coded error alone was invisible to the desktop and left its one-shot link spent. Preserve notice-specific waiting, bounded total deadline, cancellation and existing one-shot reservation; settle a pending failure when publication resumes without introducing teardown waits on the HTTP handler.

## Verification

Run the ordered AGENTS.md gate: Wails, applicable resources, bindings/assets/.gitkeep, gofmt, vet, pinned staticcheck, full Go, cgo probe, full race, frontend and browser, LF and platform preflight. Use isolated copies for mutations and asset verification to exclude the user's DS_Store. Run complete native mutation inventory plus the new canonical cases. Never truncate failing logs. Native Windows/macOS and Linux CI must pass at exact final head before integration; manual nearby-device and native UI checks remain optional and honestly unobserved.

## Suggested Review Order

- Wait only for notice publication, then revalidate the reserved claim.
  [coordinator.go:641](../../internal/transfer/coordinator.go#L641)

- Signal notice completion and settle abandoned reservations under existing ownership rules.
  [receive.go:264](../../internal/transfer/receive.go#L264)

- Prove a real reserved upload succeeds through the same link after notice publication.
  [receive_native_test.go:110](../../receive_native_test.go#L110)

- Prove cancellation at post-notice revalidation retires the spent link.
  [coordinator_receive_test.go:953](../../internal/transfer/coordinator_receive_test.go#L953)

- Run canonical named failures for the new admission guarantees.
  [verify-receive-notice-claim-mutations.sh:1](../../scripts/verify-receive-notice-claim-mutations.sh#L1)

