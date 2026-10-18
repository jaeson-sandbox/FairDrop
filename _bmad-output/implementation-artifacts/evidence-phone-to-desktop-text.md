# Phone-to-desktop text implementation evidence

Baseline: `65f387336d95ad940cd8a644e597c44e1b8366a5`. Main owns independent review, the ordered full gate, commit, push and integration. This file records focused implementation proof only. Full mutation transcripts are retained outside the worktree at `/private/tmp/fairdrop-receive-text-O12NE9/evidence-phone-to-desktop-text-logs/` (`go/`, `frontend/`, and `existing-text-frontend/`).

## Matrix coverage

| Contract row | Executed proof |
|---|---|
| Entry, empty GET, no claim or I/O | `TestReceiveTextStartsEmptyWithoutSourceSinkOrPayloadMetadata`, `TestReceiveTextFormGETIsEmptyAndDoesNotClaim`, frontend `starts without file chooser, source or clipboard commands`; native form-to-App test. |
| Exact route, methods, token and spent claim | `TestReceiveTextMethodPathAndTokenRefusalsLeaveClaimAvailable`, `TestReceiveTextRealFormEncodingCommitsExactNormalizedValueOnce` (second POST 423). |
| Bad form/media/UTF-8/blank/control and reusable code | `TestReceiveTextRejectsMalformedBodiesWithoutSpendingCode`; fixed 400/413 without claim. |
| Encoded/raw/normalized limits and chunked body | `TestReceiveTextEncodedReaderStopsAtLiteralCapPlusOne` pins the literal 394,240-byte encoded cap and 394,241-byte maximum read with a counting reader; `TestReceiveTextNormalizedBoundaryAndChunkedEncodedExpansion` accepts exactly 65,536 normalized bytes after percent expansion and rejects 65,537. The malformed-body test also checks chunked encoded oversize. |
| Interrupted input, one processing reader, stale body | `TestReceiveTextOneBodyReaderAndInterruptedBodyDoNotClaim`, `TestReceiveTextRetiredWhileBodyIsBlockedCannotCommit`, `TestReceiveTextCancelledDuringAuthorizationCannotCommit`. Deterministic gates hold a reader/authorization before retirement. |
| Real HTTP form-to-desktop value, Unicode, CRLF, literal markup | `TestNativeReceiveTextFormReachesHeldDesktopValue` uses the real App, coordinator and server with `url.Values`; `TestReceiveTextRealFormEncodingCommitsExactNormalizedValueOnce` checks body-only parsing despite a URL query. |
| Commit, early terminal, held result, failure and acknowledgement | `TestReceiveTextCommitReadHeldOutcomeAndDoneClear` reads synchronously inside the `TransferComplete` observer, before final state store; `TestReceiveTextServerCompleteWithoutCommitBecomesFailure`; `TestReceiveTextFailureAndCancellationNeverExposeValue`; `TestReceiveTextCommittedReceiptSurvivesPhoneAcknowledgementWriteFailure`. |
| Native copy, no automatic copy, wrong/stale IDs, retry | `TestNativeReceiveTextFormReachesHeldDesktopValue`, `app_receive_text_test.go`, controller `announces copy success only after native success and allows a failure retry`. Generic `CopyToClipboard` link cap remains unchanged. |
| Done, Cancel, replacement, recovery and late promises | Coordinator recovery/failure tests, controller `fences old copy across Done and a new receipt while allowing the new copy`, `ignores an obsolete fetch while a newer read of the same receipt is pending`, and `ignores a late value after Done and unmount`. |
| Payload-free metadata/events and literal readonly display | Coordinator JSON checks; frontend `ReceiveTextViews.test.tsx` checks inert markup, no anchor, no partial copy, and recovery heading description. |
| 320px, focus, forced colors | Chromium rendered `frontend/browser/receive-text.test.tsx` checks 320px overflow and 44px targets, readonly-field focus, and a real forced-colors outline. This does not assert WKWebView-specific behavior. |

## Focused verification

- `/Users/jaesonmartin/go/bin/wails build` passed, regenerating bound commands before frontend checks. The generated file modes were normalized to `100644` afterward.
- `npm test` passed: 41 files, 1,330 tests.
- `go test -count=1 -race -run 'TestReceiveText|TestNativeReceiveText|TestAppReceiveText' ./internal/server ./internal/transfer .` passed for all three packages.
- `vitest run --config vitest.browser.config.ts browser/receive-text.test.tsx` passed: 2 rendered tests.
- The earlier focused non-race Go tests and 8-test controller suite passed. Main will run the complete ordered gate in an isolated snapshot.

## Named scoped mutations

The canonical Go inventory is `scripts/verify-receive-text-mutations.py`, invoked by `scripts/verify-native-mutations.sh` without changing workflow pins. It ran in an isolated source copy. Every one of its eight baselines passed and every mutant failed its named assertion, not compilation, timeout or panic. Each case has a unique `*-baseline.json` and `*-mutation.json` under the retained `go/` log directory.

| Case | Defense and named test |
|---|---|
| `bounded-reader` | Literal cap+1 read ceiling; `TestReceiveTextEncodedReaderStopsAtLiteralCapPlusOne`. |
| `encoded-cap` | Chunked encoded oversize is 413; `TestReceiveTextRejectsMalformedBodiesWithoutSpendingCode`. |
| `one-reader` | Competing POST gets 423 while a body reader is held; `TestReceiveTextOneBodyReaderAndInterruptedBodyDoNotClaim`. |
| `post-authorization-cancel` | Direct request-context check before commit; `TestReceiveTextCancelledDuringAuthorizationCannotCommit`. |
| `stale-body` | Retired run refuses an already-entered old body; `TestReceiveTextRetiredWhileBodyIsBlockedCannotCommit`. |
| `no-payload-echo` | Phone acknowledgement never includes the accepted value; `TestReceiveTextRealFormEncodingCommitsExactNormalizedValueOnce`. |
| `no-empty-success` | No completion without committed RAM value; `TestReceiveTextServerCompleteWithoutCommitBecomesFailure`. |
| `held-receipt` | Accepted inbound text does not arm outbound reset; `TestReceiveTextCommitReadHeldOutcomeAndDoneClear`. |

The canonical inbound frontend inventory is `scripts/verify-receive-text-frontend-mutations.py`. Its two baselines passed and its two mutants failed named behavioral assertions: `copy-owned-by-session` (an old pending copy cannot block the next receipt) and `obsolete-fetch` (an older read cannot populate a newer in-flight read). All four logs are under `frontend/`. The existing outbound text inventory `scripts/verify-text-frontend-mutations.py` was rerun after updating its live-unmount source anchor for the combined text modes: all 13 baselines and all 13 mutations passed, with 26 logs under `existing-text-frontend/`.

The scoped proof does not replace native Windows/macOS CI or a manual phone/WKWebView observation. No manual observation is claimed here.

## Independent review and triage (2026-10-18)

Three context-free reviewers ran together over the full tracked-plus-untracked diff from the exact baseline, excluding unrelated `build/.DS_Store`. Review input: `/private/tmp/fairdrop-receive-text-review.diff`. The blind, edge-case, and verification-gap layers returned 12, 3, and 4 findings respectively. Two edge findings duplicated the blind layer's late-link announcement and failed-Done findings; those share fixes below. All 17 remaining findings are patches: the existing contract already establishes caller cancellation, session ownership, recoverable held outcomes, Quartz interaction conventions, and executable browser/mutation proof. No intent decision or spec contradiction is needed. No finding is deferred.

| ID | Severity | Route | Finding and required patch |
|---|---|---|---|
| R01 | high | patch | Cancelled authorization can strand CLAIMING; settle admission and prove the real coordinator path with a deterministic barrier. |
| R02 | high | patch | AfterFunc propagation can lag caller cancellation; derive commit context directly from the request and prove cancellation at commit. |
| R03 | high | patch | Failed Done invalidates outstanding fetch/copy forever; restore a usable held outcome with retryable cancellation feedback. |
| R04 | medium | patch | Late link-copy completion announces received text was copied; distinguish copy purpose and originating phase. |
| R05 | medium | patch | Initial/recovery failures lose receive-text mode; preserve text-specific retry without file/folder selection. |
| R06 | medium | patch | Programmatically focused outcome has no accessible name; connect section and heading. |
| R07 | medium | patch | Successful fetch retry removes focused button; route focus to a stable named destination. |
| R08 | medium | patch | Repeated fetch failure has no new feedback/pending state; represent attempts, guard duplicate activation, and announce each completion. |
| R09 | medium | patch | Waiting beacon warning is not in the focus description; include stable warning IDs alongside recovered-code description. |
| R10 | medium | patch | Waiting controls omit the established show/hide link and one-use caveat; reuse Quartz semantics. |
| R11 | medium | patch | Native disabled controls lose focus during asynchronous actions; use guarded aria-disabled controls with rendered proof. |
| R12 | medium | patch | Phone form is not exercised by a browser; render production HTML and test actual encoding, layout, selection and focus. |
| R13 | medium | patch | Inbound frontend mutation inventory has no maintained CI caller; wire an appropriate frontend verification entry point. |
| R14 | high | patch | Getter identity guard lacks proof against another successful receipt; exercise old/wrong IDs and clipboard refusal with real successive sessions, then mutate the guard. |
| R15 | medium | patch | App entry is untested; click the actual Idle action through App/controller and kill a missing-handler mutation. |
| R16 | high | patch | Live inbound unmount is untested; cover waiting/receiving/held state directly and mutate cleanup branches. |
| R17 | medium | patch | Recovery preflight refusal focuses the waiting heading instead of the command failure; route error focus and prove it. |

The original implementation agent is no longer available in the live agent tree. Under the owner's Sol implementation requirement, a replacement `gpt-6-sol` agent received the complete patch group and existing spec/evidence. Root retains review, full verification, and integration ownership.

Pre-patch isolated full-gate snapshot: `/private/tmp/fairdrop-receive-text-gate-3yeu5g9q`; logs: `/private/tmp/fairdrop-receive-text-gate-logs-3yeu5g9q`. Build, binding/build-asset drift, formatting, vet, staticcheck, full Go suite, native matrix, cgo, full race suite, frontend, rendered browser suite, line endings, and Darwin/Linux preflights passed. Mutation inventory is still running. This snapshot does not prove subsequent review patches.

The pre-patch isolated gate completed successfully, including the entire canonical native mutation inventory, existing outbound-text Go/frontend inventories, and the inbound-text frontend inventory. Complete logs remain at the path above. The reviewed patch set still requires its own full gate.

## Review patch proof (2026-10-18)

The deduplicated R01–R17 patch is in the branch. Caller cancellation now parents the claim context and a cancelled CLAIMING admission returns the live coordinator to STAGED; `TestReceiveTextCancelledDuringAuthorizationCannotCommit` and `TestReceiveTextCallerCancellationAtClaimCommitLeavesSessionReusable` drive those boundaries with real contexts. A failed Done retains a usable text receipt, reports one fixed cleanup error, and permits fetch/copy and Done retry. Tests cover both operations pending, late completion, and refusal of a fetch retry during unresolved Done. The App separates link-copy and received-text-copy announcements, keeps receive-text error context for initial and retired recovery failures, and tests the real Idle entry through App and the controller. Direct unmount tests cover waiting, receiving and held outcome; the latter also suppresses a late getter.

The held outcome is named by its heading. Fetch retry focuses its button on activation, holds that focus while loading, and moves focus to the named outcome when the button disappears on success. A repeated failure passes through a fresh pending state for another command-error focus transition. The waiting heading describes beacon warnings and recovered-code status, the link uses Show/Hide Link with a one-use caveat, and pending Copy text/Done/Cancel controls retain focus with guarded `aria-disabled` activation. `ReceiveTextViews.test.tsx` checks command-error uniqueness after failed Done; Chromium `browser/receive-text.test.tsx` checks retry, Copy text and Done focus across pending renders.

The production phone form is served by `TestBrowserLiveFixture` and submitted by `frontend/browser/receiver-live.mjs` with JavaScript disabled. That browser run passed at 320 CSS pixels and checked the live form's method, action, media type, controls, focus, native selection, dark and forced-colors presentation, encoded CRLF/Unicode request, receipt page and server-owned accepted value. The runner now propagates a failing fixture result. A deliberately wrong expected server value in an isolated copy made it exit 1 with `browser form submitted claims=1`; the unique full log is `/tmp/fairdrop-receive-form-negative-krukigou/mutation.log`. The native App test now completes two successful receipts and checks old/wrong IDs refuse read and copy without a clipboard write while the second remains usable.

Focused final checks passed: `go test -count=1 -run 'TestVerifyWorkflow|TestReceiveText|TestNativeReceiveText' ./internal/server ./internal/transfer .`; `npm run build`; `npm test -- --run src/App.receive.test.tsx src/App.receiveText.integration.test.tsx src/transfer/receiveText.test.tsx src/ui/ReceiveTextViews.test.tsx src/ui/announce.test.ts` (96 tests at the review patch point); `npx vitest run --config vitest.browser.config.ts browser/receive-text.test.tsx` (4 rendered tests); and `node browser/receiver-live.mjs`. The maintained native macOS/Windows workflow now invokes `scripts/verify-receive-text-frontend-mutations.py` after the frontend suite and uploads per-OS logs even on failure. `verify_workflow_test.go` pins invocation, retention and order; the Linux adapter job still runs only its native mutation inventory.

The final canonical scoped Go inventory passed all 11 baselines and killed all 11 mutants by named assertions in `/tmp/fairdrop-receive-text-gofix-final-91bupcmz/logs/`. Added review mutants cover a request-parented claim context, CLAIMING recovery after caller cancellation, and the current receipt's ID guard. The final canonical inbound frontend inventory passed all 9 baselines and killed all 9 mutants by named assertions in `/tmp/fairdrop-receive-text-frontfix-final-zgPLhy/`; added cases cover failed Done during pending copy/fetch, App entry, late link copy, direct live unmount, the pending-Done fetch fence and retry click focus. An earlier isolated Go inventory copy lacked `frontend/dist/.gitkeep` and failed its root-package baseline at build time; its distinct `/tmp/fairdrop-receive-text-gofix-vt0advlx/logs/` was retained, then a corrected copy passed all cases in `/tmp/fairdrop-receive-text-gofix-vt0advlx/logs-retry/` before the final 11-case run. These focused proofs leave native Windows/macOS CI and any unperformed manual device checks to be reported separately.

The updated full-gate snapshot's first frontend pass found the newly approved `receiveText.once` string absent from the literal copy-key inventory and Quartz copy table; complete failure output is retained in `/private/tmp/fairdrop-receive-text-final-logs-359w4zu6/frontend.log`. Both registries now include the exact string. `npm test -- --run src/ui/copy.test.ts` passes all 25 tests. Main is restarting the complete ordered gate on a frozen snapshot after this correction.

## Orchestrator follow-up review

The edge-case reviewer re-read the backend patches and found no remaining correctness defect in cancellation propagation, CLAIMING rollback, or stale receipt IDs. Its remaining request-context mutation gap was closed by `request-parented-claim-context` in the final 11-case inventory.

The blind reviewer re-read the frontend patches and identified three residuals: fetch retry admission while Done is unresolved, competing error focus targets after failed Done, and missing explicit pointer focus on Retry. Sol patched all three and added behavioral assertions. A final narrow read confirmed all three resolved with no defect found directly in those fixes. These were read-only reviews, not native UI observations.

Root found that the live browser fixture's cleanup catch swallowed a failing Go assertion; Sol corrected exit propagation and the deliberately wrong expected-value run proved it. The complete intermediate gate passed build, static checks, Go and race suites but failed the frontend copy registry on the new `receiveText.once` key. The preserved failure is `/private/tmp/fairdrop-receive-text-final-logs-359w4zu6/frontend.log`. The approved key inventory and Quartz copy table were corrected, with the 25-test copy suite passing.

After all production/test/copy-table changes stabilized, root created a fresh frozen snapshot at `/private/tmp/fairdrop-receive-text-verified-97resnqi` and restarted the full ordered gate from Wails. Its logs are `/private/tmp/fairdrop-receive-text-verified-logs-97resnqi`. This fresh run, rather than the earlier mutable snapshot, is the final local gate candidate.

Evidence limitation: the initial implementation agent overwrote some first-attempt scoped mutation log files before the instruction to use unique run directories. Their tool output remains in the conversation, but those original complete files are not available. Final named inventories and every subsequent review/final-gate failure listed above have separate retained logs; no claim is made that the earlier overwritten files were preserved.

The frozen gate's canonical receive notice/claim baseline then exposed a pre-existing test race: `TestCallerAbortAtNoticeCompletionStillSettlesSpentLink` waited for the published `TransferError`, while `acceptTerminal` stores `stateError` and frees its lease only after publication. Its full failure is `/var/folders/kh/sg9n272s6lqgyt2yc7jgdvj40000gn/T/tmp.skac1ZTrKQ/baseline-TestCallerAbortAtNoticeCompletionStillSettlesSpentLink.log` within `/private/tmp/fairdrop-receive-text-verified-logs-97resnqi/mutations.log`. The test invokes `abandonNoticeClaim` directly and never enters CLAIMING before that assertion, so the new rollback could not cause this failure. `coordinator_receive_test.go` now waits for terminal settlement before checking ERROR, and two adjacent notice tests with the same event-before-state assertion use their drainer join. No production code or expected spent-link outcome changed. Focused `-count=100` and `-race -count=20` runs of the three affected notice tests passed. The exact canonical notice/claim inventory passed all eight baselines and killed all seven named mutants in an isolated copy, retaining unique logs at `/var/folders/kh/sg9n272s6lqgyt2yc7jgdvj40000gn/T/tmp.oCJwhrYRRt/`.

## Final local gate

The final production source passed Wails build; bindings/build-asset drift and `.gitkeep`; gofmt, vet and pinned staticcheck; the complete regular Go suite and native matrix; an actual cgo probe; the full race suite; all 1,346 frontend tests; all 132 rendered browser tests plus real file download, receive-text form submission and multipart upload checks; line-ending validation; and Darwin/Linux build plus Darwin staticcheck preflights. Logs are `/private/tmp/fairdrop-receive-text-verified-logs-97resnqi`.

After the test-only notice settlement correction described above, the entire canonical native mutation inventory passed, followed by the existing outbound-text Go/frontend inventories and final nine-case inbound frontend inventory. Resumed phase logs are separately retained at `/private/tmp/fairdrop-receive-text-verified-resume-logs-97resnqi`; the failed initial phase remains intact. All production source matches the verified snapshot; final differences are this evidence/spec completion and the separately verified notice test correction. No native phone or WKWebView observation is claimed.

The implementation spec is done; there is no story_key, so sprint synchronization is inapplicable. Push/PR/integration proceed under the owner's standing authorization and repository commit-and-push instructions, which supersede the generic skill's no-auto-push default. Exact-head native CI is required before merge and will be recorded in the PR. Version and published release remain 1.7.0.
