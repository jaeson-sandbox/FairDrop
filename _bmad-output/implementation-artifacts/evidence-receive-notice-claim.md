# Receive notice/claim implementation evidence

Baseline revision: `040bb30d77c99f965d6c50c5f0c12ecc54173692` (implementation baseline). The full red transcript remains at `/tmp/fairdrop-notice-claim-red.log`. Before the fix, `go test -count=1 ./internal/transfer -run '^TestClaimWaitsForRefusalNoticePublication$'` failed:

```text
--- FAIL: TestClaimWaitsForRefusalNoticePublication (2.00s)
    coordinator_receive_test.go:723: no bound armed after the first 2 became pending (now 2 armed)
FAIL
FAIL fairdrop/internal/transfer 2.231s
```

## Behavioral proof

| Scenario | Proof and limit |
| --- | --- |
| Healthy notice/claim overlap and event order | `TestClaimWaitsForRefusalNoticePublication` and composed real HTTP/coordinator/sink `TestNativeRefusalNoticeAndReservedUploadUseTheSameLink`: 413 keeps the link, valid POST reserves it while notice publication is blocked, then saves exact bytes and publishes notice before started. The composed test injects an inert publication-bound timer to keep that overlap deterministic. |
| One HTTP reservation | `TestCompetingClaimsWaitingForOneNoticeStartOnce`, `TestRefusalNoticeDoesNotSpendAWaitingHTTPUploadReservation`, and the composed test's 423 probe. The server test covers reservation with a stub authorizer; the composed test covers production authorization. |
| Caller abort and session ownership | `TestClaimWaitingForNoticeHonorsCallerCancellation`, `TestPostNoticeRevalidationAbandonsACancelledCallerClaim`, `TestCallerAbortAtNoticeCompletionStillSettlesSpentLink`, and `TestClaimWaitingForNoticeCannotClaimReplacementAfterCancel`. The postwait entrypoint test uses a real derived Context: `Done` closes when `Err` first becomes nonnil. Existing `TestRecoveryReservationIsClaimFenceBeforeAdapterStop` and `TestRefreshConnectionCannotCancelWinningClaim` force both recovery/claim owners at the shared lease boundary. A direct notice-waiter/recovery schedule is not separately forced. |
| One deadline across real successive notices | `TestSuccessiveNoticePublicationsShareOneClaimDeadline` enqueues both notices through the same drainer and pauses the claim before its first wait; it proves the second publication does not arm a fresh deadline. |
| Bounded observer and spent-link retirement | `TestBoundedNoticePublicationReleasesClaimWithoutPromisingLateDelivery` fires the publication bound and checks its diagnostic and safe claim while the observer is still blocked. `TestClaimWaitingForNoticeHasOneBound` fires the claim bound, then releases publication and checks terminal error, no started event, and no file write. The latter is a coordinator test; no separate composed HTTP timeout/404 test was added. |

`internal/server/upload.go` passes the session context `r.ctx` to `AuthorizeClaim`, not `request.Context()`. Caller-only cancellation above proves the public authorization API; it is not evidence that a phone disconnect cancels this wait. The server independently watches request disconnection after claim: `TestADroppedConnectionKeepsCompletedFilesAndRemovesThePartial` and `TestADroppedConnectionBeforeAnyFileSavedLeavesNoSubfolder` use real raw connections and prove incomplete upload outcomes. They do not prove pre-claim cancellation. The existing five-second `Observer.Publish` bound may release the lease before a stuck observer returns, so event delivery after that diagnostic is unconfirmed; only healthy synchronous publication proves notice-before-started.

## Scoped mutation inventory

The canonical script `scripts/verify-receive-notice-claim-mutations.sh` is called by `scripts/verify-native-mutations.sh`. It was run in an isolated clone copied from this working tree, so mutations could not disturb the parent's ordered gate:

```text
bash scripts/verify-receive-notice-claim-mutations.sh
BASELINE PASSED: TestSuccessiveNoticePublicationsShareOneClaimDeadline
BASELINE PASSED: TestPostNoticeRevalidationAbandonsACancelledCallerClaim
KILLED: pre-fix-admission by TestClaimWaitsForRefusalNoticePublication
KILLED: reset-deadline-per-notice by TestSuccessiveNoticePublicationsShareOneClaimDeadline
KILLED: omit-postwait-abandonment by TestPostNoticeRevalidationAbandonsACancelledCallerClaim
```

The complete run passed eight baselines and killed seven mutations, including shrunken claim bound, mis-coded caller cancellation, omitted timeout terminal outcome, and omitted caller-abort outcome. Named failing assertions from the retained JSON transcripts:

```text
pre-fix-admission: no bound armed after the first 2 became pending (now 2 armed)
reset-deadline-per-notice: claim reset deadline after second notice: 2 claim timers, want one
omit-postwait-abandonment: post-notice cancellation retires the spent waiting link never happened
```

Complete transcripts: `/var/folders/kh/sg9n272s6lqgyt2yc7jgdvj40000gn/T/tmp.REvzQVqydO`; run summary: `/tmp/fairdrop-notice-mutations.log`. `mutationverdict` rejects build, setup, timeout, and unrelated failures as mutation proof.

## Verification handoff

The orchestrator reported a passing pre-review ordered macOS gate and complete native mutations on a prior snapshot at `/private/tmp/fairdrop-notice-gate-logs-9smuhafd`. Changed-package race checks and final snapshot gate after these test/evidence patches are owned by the orchestrator.

## Independent review triage

Three context-free review layers read the complete baseline-to-working-tree diff: blind, edge-case and verification-gap. Edge returned no findings. Blind returned twelve findings; verification identified the same entrypoint cancellation gap as blind item 6. No new product intent or spec loopback was needed.

| Finding | Severity | Route / action |
|---|---|---|
| Unqualified notice-before-started wording ignores existing observer bound | medium | patch: qualify healthy delivery, test observer bound and diagnostic |
| Composed overlap uses inert bounds | low | patch: keep deterministic overlap isolation; add unit-level publication-bound diagnostic proof |
| Successive-notice test manually replaces signal | medium | patch: add actual publication handoff and total-deadline mutation proof |
| Replacement test completes old caller before installing replacement | low | patch: narrow evidence to the schedules actually exercised and cite existing ownership proof |
| No active notice-waiter/recovery race | medium | reject as additional test scope: waiting owns no lease and existing deterministic claim/recovery tests pin the same ownership transition; direct combined schedule remains unverified |
| Completion/cancellation tested only through settlement helper (two reviewers) | medium | patch: entrypoint regression and named mutation |
| Caller cancellation was described as phone disconnect, but HTTP uses session context | medium | patch: correct docs/evidence and inspect actual disconnect path; preserve existing HTTP context contract |
| Fake-server bound test lacks composed terminal/refusal proof | medium | reject as additional test scope: unit deadline/terminal plus unchanged server authorization-error response tests cover separate contracts; combined timeout/404 schedule remains unverified |
| Missing barrier cleanup in server regression | low | patch: release on every test exit |
| Raw HTTP refusal read lacks deadline | low | patch: bounded socket deadline |
| Central wait/revalidation guarantees lack mutation kills | medium | patch: add postwait cancellation and repeated-deadline cases; retain existing identity and lease assertions |
| Full evidence only in temporary paths | low | patch: retain concise command/result excerpts and baseline in committed evidence; full logs remain referenced |

The verification reviewer inspected the added public-entrypoint cancellation regression and its exact missing-cleanup mutation and confirmed the gap was addressed. Full local gate on the pre-review snapshot passed; final patched snapshot verification follows.

## Final local verification

Final patched snapshot `/private/tmp/fairdrop-notice-final-64_zwigy` passed the ordered Wails build, binding/build-asset drift, gofmt, vet, pinned staticcheck, full Go suite, native matrix, cgo probe, full race suite, frontend tests, browser tests, LF checks, Darwin/Linux preflights and full canonical native mutations. Complete logs: `/private/tmp/fairdrop-notice-final-logs-64_zwigy`. Rendered browser suite: 128 tests across four files plus the live receiver page/HTTP check. The final scoped runner passed eight baselines and seven named kills, with complete JSON logs in `/var/folders/kh/sg9n272s6lqgyt2yc7jgdvj40000gn/T/tmp.UpXkTi3Omg`.

The CI-only macOS unusable-lock smoke and Windows resource/native proofs remain for exact-head GitHub Actions. Native GUI/nearby-device observations were not performed. No release version was changed.
