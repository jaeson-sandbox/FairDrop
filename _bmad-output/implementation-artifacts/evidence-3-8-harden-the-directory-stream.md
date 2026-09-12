# Story 3.8 evidence

## Final local gate and handoff

Parent ran the complete ordered gate without interruption after the accepted
review-fix batch. `reviewed-gate-1-transcript.log` and `reviewed-gate-1/` under
`C:/Users/jaeso/AppData/Local/Temp/fairdrop-3-8-checkpoint-2/` retain the results:
Wails build, binding drift/.gitkeep, gofmt, vet, pinned staticcheck, uncached full
Go tests, CGO_ENABLED=1 with a compiled probe, uncached full race tests (stream
195.238s), all 498 frontend tests, LF and diff checks, Darwin arm64 build/vet/bare
staticcheck and Linux amd64 build/vet. All passed. Foreign checks are preflight,
not native proof. Only handoff/status documentation was updated afterward; no
production or test code changed after this run.

All accepted round-1 patches are resolved with durable assertion-validated
mutation evidence below. No additional functional finding was deferred or waived.
Manual device/UI observations remain optional and unperformed in this checkpoint.
No release, main merge or next-story implementation is implied by this milestone.

## Current status

**Latest:** implementation and independent review are complete. The full ordered
local gate passed on the reviewed code; spec is `done`, sprint is `review` for
owner acceptance. Native CI confirmation follows the checkpoint push. The
checkpoint history below preserves earlier states and must not override this one.

Checkpoint 1 is verified and pushed. Checkpoint 2 implementation, accepted review
fixes and the final local gate are complete. Preserved baseline:
`6a366121fa9fbcd218935aa2119970e3b4e6913a`.

The owner approved retaining all assigned findings with smaller, separately
verified checkpoints following party mode ("yeah we can"). First checkpoint:
source/root/reader safety and archive names/modes/drain, with matching contracts.
Full local verification and mutations precede milestone commit/push; native CI
conclusions are read explicitly. Second checkpoint: network/coordinator cleanup,
retry fencing, and server timeout branches. Formal adversarial review and final
matrix audit follow implementation, not this planning discussion.

## Continuity and authorization

- Pre-existing working-tree edits were this session's Story 3.7 acceptance and
  Story 3.8 planning/party documents, not unexplained user changes. Preserve them.
- Story 3.7 is done after owner acceptance and all three jobs in Verify
  34679085296 succeeding. No production code differs from the recorded baseline
  at the start of Story 3.8 implementation.
- Frozen intent/matrix remains unchanged by party mode. Non-frozen execution and
  design notes carry the approved sequencing and narrower claims: identity from
  Prepare, not original Stage; unsnapshotted contents; no receiver-wide collision
  guarantee. No public UI/copy/dependency expansion is authorized.
- Party discussion used configured session-mode personas, not independent code
  reviewers; its notes are `_bmad-output/party-mode/story-3-8-2026-09-12.md`.
- Keep every scoped D-ID open until its corresponding verified closure exists.
  D-096/099/101/102 remain unfinished during the first checkpoint. Never mark the
  story complete merely because the folder-only checkpoint is green.

## Verification ledger

Checkpoint 1 implementation, full ordered local verification and native CI are
complete. First two tasks and six scoped IDs are discharged below. No cleanup
implementation or independent whole-story review has run.

### Implementation and reasoning

- Added consumer-owned PreparedDirectory (Walk/Close) to SourcePort. The raw
  source implements the search-only pin; the existing selection decorator embeds
  SourcePort and promotes the added method without another path resolver.
  Payload preparation retains that capability and ZIP writing consumes it.
- The pin is acquired at Prepare, not Inspect/Stage. Fresh no-follow validation
  compares identity before walking the same validated handle. Contents remain
  unsnapshotted; no promise about original Stage identity or collision-free
  extraction. ZIP root timestamp remains staged metadata.
- Inspection reserves the future pin. Without that reservation, an unchanged
  tree could pass preflight and exceed the handle budget only after headers.
- Borrowed Read and release share one synchronization helper spanning real
  native reads, not merely a synchronized boolean. The reader test blocks actual
  Read and checks lock ownership; a separate production emitFile test observes
  revocation at the owned-close boundary. Prepared Close similarly joins Walk.
  These do not make blocked OS I/O interruptible; scheduler-dependent overlap
  observation alone is not offered as deterministic proof.
- Portable names share one host-independent predicate. Parent audit caught
  `NUL .txt`: Windows device stems need trailing-space trimming before extension
  classification. Ordinary Unicode/spaces and apostrophes/semicolons remain;
  nested names are refused rather than sanitized. Download header sanitization
  remains a separate boundary. ZIP modes are defaults, not source preservation.
- Source arithmetic/batch errors are phase-specific without remapping other
  coded errors. The archive drain now shares the 101-empty-read behavior of file
  and entry copies; a finite test reader independently ends at 102, so deleting
  the guard fails an assertion instead of hanging.
- Binding SPEC, contracts, architecture/spine/memlog and AGENTS were synchronized
  without regenerating managed context or dropping Story 3.7 protections.

### Matrix coverage (checkpoint 1 only)

| Frozen row | Passing native coverage at c87940bf26454eaa03e647387e30cd418e15744b |
| --- | --- |
| Healthy folder / modes | Existing empty/nested/Unicode/ZIP64 suites; TestArchiveEmitsExplicitPortableModes |
| Depth | TestDirectoryHandleBudgetIncludesAncestorsAndPreparedPin; TestLexicalHandleBudgetRefusesBeforeSearchOpen |
| Root replacement | TestPreparedDirectoryOwnsOnlyLazyPinAndRejectsReplacement; TestPreparedDirectoryNativeReplacementNeverReadsNewBytes; TestPreparedArchiveNativeRootReplacementIsRefused |
| Borrow lifetime | TestBorrowedReaderRevocationJoinsAnInFlightRead; TestWalkRevokesBorrowBeforeOwnedClose; TestPreparedCloseJoinsActiveWalk; TestPreparedDirectoryClosesWithoutWalkingAndOnPreparationFailure |
| Unsafe names | TestPortableArchiveSegmentsRejectAmbiguousReceiverNames; TestSourceRejectsPortableNamesDuringInspectionAndWalk; TestArchivePortableNamesAreRejectedAtBothBoundaries; TestPrepareValidatesSanitizedArchiveRootAndReleasesPin |
| Empty reads | TestEmptyReadGuardsFailOnRead101AndResetOnProgress (drain/entry/file, stalled/progress) |
| Arithmetic / batch | TestSourceArithmeticAndBatchFaultsArePhaseCorrect |
| Wedged cleanup / nested budgets / isolated server waits | Pending checkpoint 2; not covered by this milestone |

### Mutation proof

`scripts/verify-native-mutations.sh` now adds 16 named baselines and 18 injected
breaks for this checkpoint. Its default remains the complete native suite;
restoration includes prepared.go and the shared name predicate. Each verdict
requires a passing unskipped baseline plus the matching assertion in a failing
test/subtest. Existing POSIX volume-prefix mutation now targets the shared source
predicate; it no longer silently misses the deleted volumeQualified helper.

Local scoped run: **18/18 killed**, 16/16 baselines passed. Exact mutation labels,
target tests and assertion substrings are executable in the script. They cover
cap/reservation/lexical admission, prepared identity and production ZIP wiring,
pin closure/Walk ownership, borrowed lock/visitor-return wiring, source codes,
device stem/source/ZIP/root boundaries, both modes, and drain stall/reset.

Full logs (including failed attempts) are retained at
`C:/Users/jaeso/AppData/Local/Temp/fairdrop-3-8-checkpoint-1/`:

- `mutations.log`: full local script correctly refused a skipped existing
  symlink baseline because this Windows account lacks symlink privilege. This
  is not a passing full mutation run. Native CI still requires that capability.
- `directory-mutations.log`: first scoped run stopped on an exact assertion
  mismatch (`want` versus the stream helper's `want code`). The underlying
  regression failed the intended test, but the verifier correctly refused the
  mismatched evidence. No killed claim was recorded for that attempt.
- `directory-mutations-2.log`: corrected scoped runner, 16 passing baselines and
  18 valid kills. The temporary runner extracts the same baseline/mutation blocks
  from the canonical script; no native default was weakened to accommodate this
  workstation.
- `full-gate.log` and per-gate logs: complete ordered gate passed, including Wails
  build, binding drift/.gitkeep, gofmt, vet, pinned staticcheck, uncached normal
  Go tests, CGO_ENABLED=1 and a compiled cgo probe, uncached race tests (every
  package `ok`), 498 frontend tests in 17 files, LF and diff checks. Darwin arm64
  build/vet/bare-staticcheck and Linux amd64 build/vet passed as preflight only.

### Checkpoint 1 native verdict

Implementation commit: `c87940bf26454eaa03e647387e30cd418e15744b`, pushed to
`origin/epic-3-run-reliably-on-supported-desktops`.
[Verify 34682893871](https://github.com/jaeson-sandbox/FairDrop/actions/runs/34682893871)
completed **success**. Main read `gh run view --json status,conclusion,headSha,jobs`
and confirmed that exact SHA and all three explicit job conclusions, rather than
trusting the watch command's exit status.

| Native job | Conclusion | Valid killed mutations |
| --- | --- | --- |
| Windows desktop | success | 34 |
| macOS desktop | success | 45 |
| Linux adapters (not desktop release proof) | success | 37 |

All 18 new mutations ran on every runner with passing unskipped baselines and
the required assertion-associated failures. The full native logs are retained in
`native-ci.log` beside the local gate logs. Windows UNC/symlink capability and
macOS built-process lock smoke remained mandatory and passed. Windows/macOS
frontend suites each passed 498 tests; Go normal/race, lint and native builds
passed. No new manual device or UI observation is claimed.

| Closed finding | Verified resolution |
| --- | --- |
| D-077 | 64 retained handles plus three transient; lexical admission and future-pin reservation tested/mutated |
| D-079 | Shared portable source/ZIP predicate and sanitized-root validation; spaced device stems, source/ZIP/root boundary mutations |
| D-080 | Native-read/revocation shared synchronization and actual visitor-return close wiring; race and both mutations passed |
| D-081 | Literal 0755/0644 ZIP mode assertions and both mode mutations |
| D-082 | Search-only prepared identity pin, native replacement refusal, production ZIP capability-wiring mutation, Close ownership |
| D-105 | Inspection arithmetic/batch setup_failed versus streaming transfer_failed; phase mutation caught |
| Epic 2 archive-drain action | Failure on empty read 101, reset after progress; guard and reset mutations caught without hanging |

Before the documentation milestone, the complete ordered local gate was repeated
successfully with no production changes; logs are in the `handoff/` subdirectory.
The first race stream run took 204.535 seconds. Foreign Darwin/Linux checks remain
preflight only; the native results above are the platform proof.

### Historical resume plan for checkpoint 2

- Spec and sprint stay **in-progress**. D-096/099/101/102 remain live Story 3.8
  work; do not mark the story done or skip independent review.
- StopBeacon must detach under lock, wait outside it and retain outstanding-stop
  ownership; retries must not create additional stuck workers or let a late stop
  touch a newer responder/session.
- Coordinate server 10-second, coordinator 15-second and outer lease budgets;
  propagate an inner timeout as failure, not an ordinary cleanup diagnostic.
  Prove accept-loop, handler and connection timeout branches independently.
- Distinguish an adapter call still running from a call that returned a coded
  unquiescent failure; never infer the underlying resource stopped from either
  a caller timeout or merely a completed function. Preserve Story 3.4's honest
  quiescence semantics and Story 3.7's finalization/selection guards.
- After cleanup implementation: repeat ordered gates and mutation proof, audit
  every frozen matrix row, then run BMAD step 04's independent parallel reviews.
  Party-mode discussion was not that review.

## Checkpoint 2 implementation and local proof

- `network.Manager` now detaches a responder under its mutex and selection gate,
  then calls `Shutdown` after releasing both. Concurrent stops join the retained
  result; address selection remains responsive and a new responder refuses while
  cleanup is outstanding. A partial handle returned by failed/cancelled Start uses
  the same path, closing the lock-poisoning gap identified during parent audit.
- The coordinator retains at most one in-flight cleanup call for each adapter.
  Stage checks those calls inside the state/lease admission critical section, so
  the forced entropy interleaving cannot admit a session after cleanup becomes
  outstanding. A late result is fenced from a newer session and retries do not
  accumulate goroutines.
- Server teardown publishes persistent accept-loop, handler and connection
  completion channels before exposing the unresolved run. Start refuses until
  all three actually close, then reuse recovers. Repeated Stop does not create
  new waiter goroutines. The old expectation that Start could succeed while the
  prior run remained unquiesced was corrected because it contradicted the frozen
  no-new-session/no-accumulation matrix row.
- `transfer.MarkUnquiescent` distinguishes a server bound from an ordinary
  cleanup diagnostic. The server's inner bound remains 10 seconds; the
  coordinator is 15 seconds and propagates the specific marked result. The root
  test relates both exported values. Timer expiry rechecks every completion lane
  before naming it, and exact isolated assertions cover accept loop, handler and
  connection. The repeated-diagnostic test now injects a real diagnostic rather
  than passing on nil/nil.

Scoped ordinary Go tests passed for `internal/network`, `internal/server`,
`internal/transfer`, and the root package. Scoped race tests passed for all three
changed internal packages. Thirteen checkpoint-2 mutations were killed with
named assertion evidence: responder overlap; both locks across normal Shutdown;
both locks across failed-start cleanup; Stage admission; cleanup coalescing;
nested bound relationship; inner-result propagation; unquiescent marker; server
retry fence; and accept-loop/connection name removal. The canonical native
mutation script contains these cases and backs up every newly mutated file.

Full-gate attempt history is retained under
`C:/Users/jaeso/AppData/Local/Temp/fairdrop-3-8-checkpoint-2/`. The first race
attempt found the legacy concurrent Stop expectation after both joiners correctly
received one cleanup diagnostic. The prepared script overwrote that original
`go-race.log`; only the tool transcript retains the complete failure, so no file
preservation is claimed. A later run passed Wails build, binding drift, vet,
staticcheck, ordinary Go tests, the cgo probe, the complete race suite (including
`internal/stream` in 221.122s), and 498 frontend tests. Its detached wrapper ended
before line-ending/diff and foreign-preflight steps; those are recorded separately
after the final edits. This is local implementation evidence, not native CI or
formal whole-story review.

After the final edits, `gofmt -l`, `go vet ./...`, pinned `go tool staticcheck
./...`, and uncached `go test -count=1 -timeout 240s ./...` passed. Scoped race
tests for network/server/transfer passed again. The LF check, `git diff --check`,
Darwin arm64 build/vet/bare-staticcheck, and Linux amd64 build/vet also passed.
Foreign builds remain preflight only. The final whole-tree race run is the passed
221.122-second run above; parent review may require one final complete sequential
gate on the reviewed tree before commit/push.

### Parent matrix audit before independent review

The parent reran the remaining matrix coverage with verbose, uncached output in
`C:/Users/jaeso/AppData/Local/Temp/fairdrop-3-8-checkpoint-2/parent-matrix-audit.log`.
All named tests ran and passed, with no skips:

| Matrix row | Executed coverage |
| --- | --- |
| Wedged cleanup | `TestWedgedStopDetachesAndCoalescesWithoutPoisoningSelection`, `TestFailedStartCleanupDoesNotHoldManagerLocksOrAdmitAResponder`, `TestStageRechecksOutstandingCleanupAtAdmission`, `TestAClaimWhoseStopBeaconTimedOutLeavesItForTeardown`, `TestStopReleasesItsMutexBeforeWaitingAndFencesAConcurrentStart`, `TestStopReturnsACodedFailureWhenAHandlerNeverReturns` |
| Nested timeout | `TestCoordinatorCleanupOutlastsServerTeardown`, `TestCoordinatorPropagatesAnInnerUnquiescentServerFailure`, `TestARepeatedStopReplaysTheFirstCallsCleanupDiagnostic` |
| Isolated server wait | All three subtests of `TestEachServerTeardownWaitIsNamedInIsolation`: accept loop, handler, connection |

Rows 1–7 retain checkpoint-1 native evidence above. The resumed documentation
checkpoint `72e1fe329fadc8dc3d9a084a08414a9c4d67ee96` also has explicit success
conclusions for Windows, macOS and Linux adapters in Verify 34683482686.
The owner requested Sol/Luna subagents to reduce cost: implementation was moved
to a fresh Sol agent, and independent review will use Sol rather than inherit the
main model. This preference is preserved outside the managed AGENTS.md block.

### Independent whole-story review, round 1

Three fresh context-free Sol reviewers ran together on the full diff from the
preserved baseline, not merely checkpoint 2: blind hunter, edge-case hunter and
verification-gap reviewer. The owner explicitly requested Sol/Luna, overriding
the workflow's default same-model review preference. Main waited for all three
results before triage. No intent or design change was established; the accepted
items are bounded verification/documentation patches under the existing matrix.

| Accepted finding | Severity / route | Required resolution |
| --- | --- | --- |
| Shared non-nil beacon result unpinned (blind + verification) | medium / patch | Force a joined waiter and check wrapped cause; do not require two warnings from a scheduler-dependent race where the second caller may arrive after cleanup |
| Failed-start cleanup Stop joiner untested | medium / patch | Join partial-handle cleanup, retain its result, prove one Shutdown and recovery |
| Overlap refusal does not pin factory call count | medium / patch | Assert no second factory invocation before refusal, for both cleanup paths |
| Server cleanup slot production retry/recovery untested | medium / patch | Drive a real coordinator timeout transition through a blocked server fake, retries, late completion and recovery; retain separate entropy-admission test |
| Directory archive Close error delegation untested | medium / patch | Sentinel error, exactly-once delegation and repeated-close semantics; mutate away propagation |
| Persistent server waiter wiring bypassed by isolated-name fixture | medium / patch | Exercise actual waiter initialization with outstanding work and release, not preclosed injected completion lanes |
| Evidence current state and mutation detail incomplete | low / patch | Current summary, historical labels, per-mutation test/assertion table and retained proof paths |
| Final reviewed tree lacks uninterrupted whole gate | medium / patch | Parent runs full ordered gate after fixes; native CI remains required after push |

The prior lost failure cannot be reconstructed from the durable filesystem logs;
its loss remains disclosed rather than converted into a fictional preserved pass.
Rerun evidence is now durable: `cleanup-mutations.log`, eight uniquely named
`cleanup-baseline-*.log` files, thirteen `cleanup-mutation-*.log` files and
`post-edit-scoped-race.log` in the checkpoint-2 log directory. Main read the
aggregate: eight baselines passed, 13/13 assertion-associated kills, and all three
changed internal packages passed race detection. Mutation restoration preserved
the implementer's before/after working-diff hash.

Triage retained the approved exported timeout comparison: removing those values
would undo the spec's explicit cross-package budget pin. Exact isolated timeout
names plus production configuration pins remain useful independent checks; no
test should sleep out real 10/15-second bounds. No timestamp snapshot, portable
case/normalization collision guarantee, or malformed-Reader contract extension is
introduced. The late-result report suggestion did not establish a new regression:
the pre-existing bound already reports unproven cleanup, and late ordinary errors
were not previously delivered. These observations do not expand this story.

### Checkpoint-2 mutation ledger and round-1 fixes

Every row has a passing named baseline and an assertion-associated injected
failure. Logs are under `C:/Users/jaeso/AppData/Local/Temp/fairdrop-3-8-checkpoint-2/`.

| Mutation | Baseline test | Named assertion | Durable log |
| --- | --- | --- | --- |
| Admit responder during cleanup | `TestWedgedStopDetachesAndCoalescesWithoutPoisoningSelection` | `want beacon_warning` | `cleanup-mutation-overlap.log` |
| Retain manager mutex across Shutdown | same | `retained the manager mutex` | `cleanup-mutation-stop-mutex.log` |
| Retain selection gate across Shutdown | same | `retained the selection gate` | `cleanup-mutation-stop-gate.log` |
| Retain mutex across failed-start Shutdown | `TestFailedStartCleanupDoesNotHoldManagerLocksOrAdmitAResponder` | `retained the manager mutex` | `cleanup-mutation-failed-mutex.log` |
| Retain gate across failed-start Shutdown | same | `retained the selection gate` | `cleanup-mutation-failed-gate.log` |
| Admit Stage across cleanup race | `TestStageRechecksOutstandingCleanupAtAdmission` | `Stage across cleanup-admission race` | `cleanup-mutation-admission.log` |
| Duplicate outstanding adapter cleanup | `TestAClaimWhoseStopBeaconTimedOutLeavesItForTeardown` | `launched another StopBeacon` | `cleanup-mutation-coalesce.log` |
| Equalize nested cleanup bounds | `TestCoordinatorCleanupOutlastsServerTeardown` | `want 15s` | `cleanup-mutation-bounds.log` |
| Absorb inner unquiescent result | `TestCoordinatorPropagatesAnInnerUnquiescentServerFailure` | `inner unquiescent marker` | `cleanup-mutation-propagate.log` |
| Erase server unquiescent marker | `TestEachServerTeardownWaitIsNamedInIsolation` | `structurally unquiescent` | `cleanup-mutation-marker.log` |
| Admit unresolved server retry | `TestStopReleasesItsMutexBeforeWaitingAndFencesAConcurrentStart` | `want server_start_failed` | `cleanup-mutation-server-fence.log` |
| Omit accept-loop timeout name | `TestEachServerTeardownWaitIsNamedInIsolation` | `exact isolated diagnostic` | `cleanup-mutation-accept-name.log` |
| Omit connection timeout name | same | `exact isolated diagnostic` | `cleanup-mutation-connection-name.log` |
| Drop normal beacon join result | `TestStopBeaconJoinerReceivesTheOwnersCleanupDiagnostic` | `same non-nil wrapped error` | `review-fix-mutation-normal-join-result.log` |
| Drop failed-start join result | `TestStopBeaconJoinsFailedStartCleanupAndRecovery` | `same non-nil wrapped error` | `review-fix-mutation-failed-start-join-result.log` |
| Invoke factory before normal refusal | `TestStopBeaconJoinerReceivesTheOwnersCleanupDiagnostic` | `original one only` | `review-fix-mutation-normal-overlap-factory.log` |
| Invoke factory before failed-start refusal | `TestStopBeaconJoinsFailedStartCleanupAndRecovery` | `want one` | `review-fix-mutation-failed-start-overlap-factory.log` |
| Detach production server cleanup slot | `TestTimedOutServerStopFencesNewSessionsUntilTheProductionCallCompletes` | `want busy` | `review-fix-mutation-server-slot.log` |
| Duplicate production server cleanup | same | `one coalesced call` | `review-fix-mutation-server-coalescing.log` |
| Drop prepared Close result | `TestArchiveCloseDelegatesOnceAndPreservesThePreparedCause` | `prepared close cause` | `review-fix-mutation-archive-close-error.log` |
| Detach real handler waiter | `TestInitQuiescenceTracksRealHandlerAndConnectionWaitState` | `handler quiescence channel closed` | `review-fix-mutation-handler-waiter.log` |
| Detach real connection waiter | same | `connection quiescence channel closed` | `review-fix-mutation-connection-waiter.log` |

Round-1 patch proof is complete: five new baselines and 9/9 new mutations passed
in `review-fix-mutations-11.log`; its working-diff hashes before and after
restoration are both `3ab0994cac9d385f4e981362a430f88da4e75f39`.
Named rows pass in `review-fixes-final-scoped-normal-2.log` and with
`CGO_ENABLED=1` in `review-fixes-final-scoped-race-2.log`. Attempts 1–10 remain as
historical proof-run failures while assertion routing, fake-timer observation and
Windows restoration retry were made deterministic; none is claimed as a product
failure or passing aggregate. The original overwritten concurrent-Stop failure
remains lost as disclosed above. No full ZIP64 gate ran during this patch batch.
