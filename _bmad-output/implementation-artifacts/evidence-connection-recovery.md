# Connection recovery implementation evidence

2026-10-12, branch `codex/connection-recovery`. Implementation is review-ready; this record distinguishes executed proof from remaining release gates. Manual native interaction was not observed.

## Executed I/O matrix

| Contract edge | Executed proof |
|---|---|
| Sorted, deduplicated, eligible and read-only options; stale explicit ID | `TestConnectionOptionsAreSortedDeduplicatedAndReadOnly`, `TestRefreshConnectionPreflightRefusesStaleChoiceWithoutRetirement`, panel stale-list and vanished-choice tests |
| Fresh identity and retired old send code | `TestRefreshConnectionRetiresOldSendBeforePublishingFreshIdentity`; `TestRecoveredHTTPRejectsOldTokenAndSendsExactFile` uses real HTTP and checks retired token rejection plus exact new payload bytes |
| Empty/stale IDs, claim and duplicate refresh | `TestRefreshConnectionRefusesEmptyAndStaleSessionIDs`, `TestRefreshReservationExcludesClaimAndSecondRefresh`, `TestRefreshConnectionCannotCancelWinningClaim` |
| Bounded teardown, Cancel/Shutdown and final failure | `TestRefreshConnectionCleanupFailureBlocksReplacement`, `TestCancelDuringRecoveryRetirementPreventsReplacement`, `TestShutdownDuringRecoveryRebuildPreventsCommit`, `TestRefreshFailureAfterRetirementRemovesOldCode` |
| Receive destination ownership | `TestRefreshReceiveRetainsOneDestinationAndClosesOnExit`, `TestRefreshReceiveReplacementFailureClosesRetainedDestinationOnce` |
| UI stale responses, early events, malformed acknowledgments | `frontend/src/transfer/recovery.test.tsx` (15 cases, including table-driven rejections), `frontend/src/ui/ConnectionRecoveryPanel.test.tsx` (3 cases); rendered Chromium recovery control test at 320 CSS pixels |
| Original retired send URL and real receive sink | `TestRecoveredHTTPRejectsOldTokenAndSendsExactFile` executes the original URL and retired token on the new listener; `TestRecoveredReceiveHTTPWritesOnlyAfterFreshClaimToApprovedFolder` proves GET creates no output, old token POST cannot write, fresh POST writes exact bytes into the approved folder |
| Explicit address, collection, identity handoff | `TestAppForwardsExplicitConnectionChoiceUnchanged`, `TestRefreshConnectionForwardsDistinctExplicitEndpointIntoURLAndBeacon`, `TestRefreshCollectionReinspectsInArchiveOrder`, `TestRefreshCollectionInspectionFailureRetiresOldCode`, `TestCancelDuringRecoveryIdentityGenerationPreventsReplacement` |
| Retired Wails error and receive frontend focus | `TestRecoveryFailureWailsFormatterMarksRetirementWithoutLeakingCause`, receive hook recovery tests, `App.focus.test.tsx` recovered heading/description test, preflight route and refusal-count test |

## Commands run

- `wails build`: passed, regenerated bindings. Run after exported command changes; no generated file was hand edited.
- `gofmt -l` on authored Go paths: no output.
- `go vet ./...`, `go tool staticcheck ./...`, `go test -count=1 ./...`: passed.
- `go env CGO_ENABLED`: `1`.
- First `go test -count=1 -race ./...`: source/stream and all packages except transfer passed; transfer had a test-only observer-publication timing failure in `TestRefreshConnectionCleanupFailureBlocksReplacement`. The assertion now waits for the asynchronous event; scoped transfer race and a complete `go test -count=1 -race ./...` rerun passed across every package (including the 108-second stream suite).
- `npm test -- --run`: 37 files, 1,283 tests passed. Focused recovery suite: 11 passed after final controller fix.
- `npm run test:browser`: 4 files, 127 tests, receiver and upload live browser scripts passed after updating three browser mocks for the additive Wails command. Focused rendered recovery check, `npx vitest run --config vitest.browser.config.ts browser/accessibility.test.tsx -t 'expanded recovery controls'`: passed (320-pixel reflow, 44-pixel target, operable refresh).
- Review corrections: `wails build` passed again after the generated-model browser fixture fix; `npm test -- --run` passed 37 files, 1,289 tests after the full review patch; focused `go test -count=1 ./internal/transfer .` passed. Parent is running the isolated final canonical gate (Wails, assets, vet/staticcheck/Go/cgo already passed as of this evidence edit; race and remaining steps pending).

## Mutation inventory and remaining gates

The canonical existing inventory is `scripts/verify-native-mutations.sh`. The complete run in isolated `/tmp/fairdrop-recovery-scoped.CLOmGL` passed **51 structured baselines and 68 named mutation kills**. Full transcript: `/tmp/fairdrop-recovery-native-mutations-full.log`. An earlier partial run was stopped at the orchestrator's request before review; it is superseded by this complete pass. The source checkout was never mutated.

New canonical Go inventory: `scripts/verify-connection-recovery-mutations.py` has nine cases, all killed by named failing assertions, with a passing named baseline for each. It requires an isolated source copy and `FAIRDROP_MUTATION_ISOLATED=1`; the executed command in `/tmp/fairdrop-recovery-scoped-final.KmGbEC` was `FAIRDROP_MUTATION_ISOLATED=1 python3 scripts/verify-connection-recovery-mutations.py --log-dir /tmp/fairdrop-recovery-scoped-final-logs`. Cases: `stale_choice`, `fresh_identity`, `reinspect_collection`, `destination_owner`, `explicit_endpoint`, `cleanup_quiescence`, `claim_fence`, `identity_cancel`, `retired_marker`. Each case has complete baseline and mutation JSON transcripts under that log directory; `scripts/mutationverdict` rejected compilation/setup failures as kills.

New canonical frontend inventory: `python3 scripts/verify-connection-recovery-frontend-mutations.py --log-dir /tmp/fairdrop-recovery-frontend-mutation-logs` passed four named baseline/kill pairs in isolated frontend copies: `old_code_hidden`, `stale_list`, `unmount_cancel`, `retired_rejection`. Full Vitest logs are in the given directory; the runner rejects import/compile failures and requires a named assertion failure.

Remaining release evidence: parent final isolated gate completion, native Windows/macOS and Linux CI. No manual nearby-device or native screen-reader pass is claimed.

## Independent review round 1

Three context-free layers completed against `/private/tmp/fairdrop-connection-recovery-review.diff`: blind hunter, edge-case hunter, verification-gap reviewer. Findings were deduplicated by claim/action. All accepted findings below are bounded patches to existing intent; no scope expansion or invariant change is needed.

| ID | Severity | Route | Required correction/proof |
| --- | --- | --- | --- |
| R1 | high | patch | Cancel pending recovery on controller unmount; do not leave an undisplayed replacement listener. |
| R2 | medium | patch | Apply ordinary selection-memory cleanup when cancelling recovery. |
| R3 | medium | patch | Make recovery Cancel single-flight, including same-tick repeated calls. |
| R4 | medium | patch | Focus/announce command errors on pre-retirement refusal in send and receive. |
| R5 | medium | patch | Preserve receive refusal count when preflight leaves the original session intact. |
| R6 | medium | patch | Recompute unavailable-choice state on every fresh list, including a returning endpoint. |
| R7 | low | patch | Keep advertised address visible during enumeration failure using existing waiting metadata. |
| R8 | low | patch | Make waiting firewall/HTTP guidance consistent with explicit refresh. |
| R9 | high | patch | Execute the original retired URL as well as an old token on the new listener. |
| R10 | high | patch | Real receive HTTP/sink proof: no preclaim output, fresh upload in approved folder, old capability cannot write. |
| R11 | high | patch | Deterministic Cancel during identity-generation handoff. |
| R12 | high | patch | Canonical named recovery mutations and complete native inventory. |
| R13 | high | patch | Verify actual Wails formatter serializes fixed errors plus retired marker. |
| R14 | high | patch | Verify explicit nondefault endpoint forwarded through App/coordinator to URL and beacon. |
| R15 | high | patch | Execute collection reinspection failures and successful archive-order preservation. |
| R16 | high | patch | Exercise receive frontend refresh, pending code removal, replacement focus and announcement. |
| P1 | high | patch | Clean-copy Wails build found typed browser fixture missing generated convertValues; fix fixture and repeat gate. |

The initial parent clean-copy Wails build failure is preserved in the parent tool transcript; `/private/tmp/fairdrop-recovery-gate-logs/wails.log` was overwritten by the passing rerun. The failure was a typed browser fixture missing generated `convertValues`; it was fixed with the generated constructor, and Wails build now passes. The independent review findings above are patched; final acceptance awaits the parent's remaining release gates.

Independent follow-up: the blind reviewer re-read the repaired controller, cancellation, error focus, refusal state, panel availability/address, help copy and added integration/handoff tests. It reported no remaining concrete functional defects in those fixes. The complete mutation inventories subsequently passed, as recorded above.

## Final local gate

The reviewed source snapshot in `/private/tmp/fairdrop-recovery-gate-xaajfaer` passed the ordered local gate: Wails production build, byte-for-byte generated bindings and build assets plus `.gitkeep`, gofmt, vet, pinned staticcheck, full Go tests, cgo compile/run probe, full race tests, **1,289 frontend tests**, **128 Chromium browser tests** plus receiver/upload live browser scripts, LF checks, Darwin arm64 and Linux amd64 builds, and Darwin staticcheck. Full per-step logs: `/private/tmp/fairdrop-recovery-gate-logs/`. Native unusable-lock launch proof is deferred to CI because its canonical script explicitly forbids running on a developer machine.

Two verification-copy issues were corrected without changing application behavior: a newly added Go test needed gofmt, and the frontend copy test initially arrived before its matching UX table update. The latter full failure is retained in `frontend-incomplete-snapshot.log`; the synchronized frontend/browser rerun passed. Author-written whitespace checks pass; Wails-generated tab-only blank lines match generator output and were not hand-edited. The user's unrelated `build/.DS_Store` remains untouched and outside the verification snapshot and commit.

All review patches are complete. Native Windows/macOS and Linux CI remains the integration gate; no release/version change is included. Manual native interaction, nearby-device connectivity, and screen-reader behavior remain unobserved.
