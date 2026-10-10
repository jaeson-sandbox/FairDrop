# Multiple selected items — implementation evidence

Baseline: `c56a39eee5494b8d8706e1b801e5fc077245223b`. Implementation branch: `codex/multiple-selected-items`. This file records executed proof for the collection extension; the canonical SPEC remains in `_bmad-output/specs/spec-multiple-selected-items/`.

## Backend matrix coverage

| Matrix input/state | Executed assertion |
|---|---|
| One file/folder | Existing `go test -count=1 ./...` tests for single-file and directory flows passed; `Stage` remains a one-path wrapper. |
| 2–16 mixed roots and matching basenames | `TestCollectionStreamsAllMembersWithNumberedRoots` independently reads the ZIP and checks exact bytes, empty folder, root order, name, and unknown wire length. `TestCollectionLandingShowsCountAndLogicalSizeWithoutPaths` checks GET metadata and no payload preparation. |
| Zero/17 paths, duplicate or overlap | `TestCollectionCountRefusalPrecedesFilesystemAndNetwork`, `TestCollectionRejectsDuplicateAndAncestorSelections`, `TestSelectionOverlapRespectsRootAndComponentBoundaries`. |
| Invalid member, aggregate overflow, cancellation | `TestCollectionCheckedSumAndCallerSliceOwnership`, `TestCollectionLaterFilePrepareFailureClosesEarlierDescriptor`, `TestCollectionCancellationBetweenMembersClosesEarlierDescriptor`; each checks no premature network setup or retained earlier descriptor as applicable. |
| Multiple pins and deep tree | `TestCollectionPinsShareRetainedDirectoryBudget` opens simultaneous native pins and reaches the 64-retained-handle boundary with one unchanged tree. |
| Later source replacement | `TestCollectionLaterDirectoryReplacementAbortsWithoutCentralDirectory` replaces the last directory after Prepare, then checks failure and unreadable partial ZIP. `TestCollectionPreparedFileLengthIsEnforced` checks top-level file truncation fails and growth is capped to prepared length. |
| Partial Prepare, Close-before-write, failed ZIP | `TestCollectionLaterFilePrepareFailureClosesEarlierDescriptor`, `TestCollectionCloseBeforeWriteReleasesEveryMember`, and the replacement test above. |
| Public ownership boundaries | `TestCollectionCopiesServerBoundaryAndPublishesAggregateOnly`, `TestCollectionPayloadAdapterCannotMutateServerMetadata`, `TestCollectionPreparationSnapshotsCallerMembersBeforeExternalWork`, and `TestSelectFilesReturnsEveryChosenPathAndStagesNothing`. The chooser dismissal test asserts literal JSON `[]`. |
| Archive root-name limits | `TestCollectionRootNameFallbackAndPostPrefixLimit` and `TestCollectionNumberedRootFitsMultibyteFilesystemLimit` check fallback, trailing trim, ordinal, 200-rune and 255-byte limits. |

Frontend collection list, focus, stale chooser, Send Again, and rendered browser coverage are owned by the parallel frontend implementation. The final integration gate and native CI verdict belong to the main session; no native interaction result is claimed here.

## Verification transcript

- `/Users/jaesonmartin/go/bin/wails build` completed on macOS arm64 and regenerated `frontend/wailsjs/` after adding bound methods.
- `go test -count=1 ./...`, `go vet ./...`, and `go tool staticcheck ./...` passed at the backend checkpoint. Focused collection tests were rerun after subsequent ownership/name fixes and passed.
- The canonical full gate, race run, browser suite, line-ending check, and native CI are pending integration by the main session. Wails was not run concurrently with frontend tests.

## Scoped mutation proof

`mutate-multiple-selected-items.py` is the sole Go case inventory and runner. It copies the current source to a platform-native temporary directory with Python, mutates one exact anchor per case, runs the same named focused test on baseline and mutant using `go test -json -count=1`, and applies `scripts/mutationverdict` to require a passing named baseline and a mutation-specific named assertion failure. Build/setup failures, panic, skip, timeout, and unrelated test failures are rejected. Unique full JSON transcripts are in `evidence-multiple-selected-items-mutation-logs/` with `summary.txt`.

| Case | Guarantee named by failing assertion |
|---|---|
| `count` | 17 paths never reach external work. |
| `overlap`, `root_overlap` | Duplicate, ancestor, and filesystem-root overlap are refused without false sibling matches. |
| `overflow` | Checked aggregate sum refuses `path_unsupported` before network. |
| `slice_copy`, `member_copy`, `handler_copy` | Caller paths, payload members, and server metadata are independently owned. |
| `numbered_names`, `name_fallback`, `name_trim`, `name_bytes` | Distinct ordinal names, existing fallback, safe truncation, and UTF-8 byte bound. |
| `shared_budget` | Additional prepared pins count against the source's native retained-handle limit. |
| `decorator_single_inspect` | The budgeted selection decorator resolves and traverses each root once. |
| `coordinator_pin_reservation`, `stream_pin_reservation` | Both call sites reserve every other selected directory's future pin against the shared budget. |
| `directory_member_close` | Collection cleanup releases prepared directory pins as well as file descriptors. |
| `short_file` | A truncated prepared top-level file cannot finalize a successful collection. |
| `halt_finalization` | A replaced later root cannot yield a readable partial archive. |
| `cleanup`, `cancel_cleanup` | Earlier acquired descriptors close after later failure or cancellation. |

All 20 cases finished with `baseline=pass` and `mutant=named assertion` in the final inventory run on the integrated tree (2026-10-10, `evidence-multiple-selected-items-integration-logs/final-collection-go-mutations.log`). No omitted inventory case is counted as proof.

## Review corrections

Independent integration review found and fixed root-prefix overlap, collection-member mutation during external preparation, payload-adapter mutation of server metadata, missing fallback/post-truncation trim, and multibyte filename overflow. Each has an executed assertion and a scoped mutation above. The original one-item flow and directory pin semantics remain in their existing tests.

## Final three-layer review triage

All three independent reviewers received the complete implementation diff. Blind, edge-case and verification-gap results were collected before triage. Duplicate findings were combined only where the claim and required correction matched. These are bounded patches to the existing contract; no new product intent or weakened guarantee is required.

| Finding | Severity / route | Required correction |
|---|---|---|
| Failed Stage → draft Cancel retains remembered paths | medium / patch | Forget the selection and disable retry on cancellation; prove the transition. |
| Draft names expose directional/control characters | medium / patch | Sanitize display names and accessible labels while preserving private original paths. Found by blind and edge reviews. |
| Flex rows hide ordered-list markers | low / patch | Show explicit ordinal labels, including duplicate-basename rows. |
| Restored draft error may not be announced on mount | medium / patch | Associate the focused heading with the error rather than relying only on initial live-region content. |
| Malformed native drop refusal is invisible in an open draft | medium / patch | Route the error to the visible draft without discarding its rows. |
| Budgeted decorator redundantly traverses directories | medium / patch | Resolve canonical paths independently of inspection; retain admission/cancellation semantics. |
| Call-site shared-budget reservations lack regression proof | high / patch | Test real multi-directory admission/preparation boundaries and mutate both reservation callers. Blind and verification reviewers independently found this gap. |
| Collection cleanup does not observe directory-pin release | high / patch | Track mixed resources across cleanup paths and close errors; mutate omission of directory cleanup. Blind and verification reviewers independently found this gap. |
| Native overlap/canonical alias cases lack coverage | medium / patch | Add platform-specific path and resolved-alias assertions; distinguish local proof from native Windows CI. |
| Real collection HTTP/ZIP boundary lacks integration coverage | medium / patch | Exercise authorized POST, attachment/framing, readable bytes, completion and failure abort using production payloads. |

Pre-patch integrated checkpoint: the isolated macOS checkout passed Wails build, regenerated-binding equality, build-asset drift, formatting, vet, staticcheck, full Go tests, cgo/race tests, 857 frontend tests, 86 browser tests and the real receiver browser fixture, LF checks, Darwin/Linux cross-builds and Darwin staticcheck. Full output is retained at `/private/tmp/fairdrop-collections-build.log` and `/private/tmp/fairdrop-collections-gate.log`; these are checkpoint results, not final patch verification.

Native collection UI interaction remains unverified: computer-use reported that the Mac was locked and could not automatically unlock it. The optional manual observation was not substituted with jsdom or Chromium results. The personal-project policy allows automated verification to proceed.

Documentation audit also corrected the stale resolver-recovery paragraph in `docs/release-policy.md` to match the existing timeout behavior already described in the active contracts; historical release notes remain unchanged.

The first integrated run of `scripts/verify-native-mutations.sh` stopped with exit 255 at `pin reservation mutation did not match`. Collection accounting had changed the old mutation anchor. This is a harness compatibility failure, not a killed mutation or product pass. Complete failing output is retained at `/private/tmp/fairdrop-collections-native-mutations.log`; the canonical script is being corrected and its full inventory must pass before integration.

## Final integration verification (2026-10-10)

All ten three-layer review findings above are patched with regression tests: `TestBudgetedSelectionResolvesOnceAndTraversesOnce`, `TestCollectionDirectoryPinsCloseExactlyOnceOnEveryExit`, `TestWindowsSelectionOverlapUsesCaseFoldAndVolumeComponents`, and the native `TestNativeCollectionSharedDirectoryBoundaryAndHTTP`, `TestNativeCollectionRejectsCanonicalAncestorAliasBeforeNetwork` and `TestNativeCollectionReplacedLaterRootAbortsAuthorizedHTTP` (real authorized POST, attachment framing, readable bytes and failed-member abort). The five frontend findings are recorded in the frontend evidence.

The native mutation harness anchors were corrected for collection accounting (`withSelectionRetained(..., 1+otherPins, ...)`, the `acceptResolved` result gate) and a Windows-only case-fold mutation was added. Final results on the integrated working tree, macOS arm64:

| Check | Result | Log |
|---|---|---|
| `wails build`, then bindings re-diffed | Passed; regenerated bindings identical in content (Wails rewrote file modes to 0755, restored to 0644) | `final-wails-build.log` |
| Canonical ordered gate: dist `.gitkeep`, build-asset drift, gofmt, vet, staticcheck, Go tests, cgo, race, frontend, browser, line endings, Darwin/Linux/Windows cross-build, Darwin staticcheck | Passed: 9 Go packages ok (plain and `-race`), 863 frontend tests, 86 browser tests plus live receiver fixture | `final-integrated-gate.log` |
| `scripts/verify-native-mutations.sh` | Exit 0; 51 baselines passed; 68/68 darwin mutations killed by named assertions; source restored byte-identically. The 69th case is Windows-only and runs in native CI | `final-native-mutations.log` |
| Collection Go inventory | 20/20 killed | `final-collection-go-mutations.log` |
| Collection frontend inventory | 10/10 killed | `final-collection-frontend-mutations.log` |

The untracked Finder file `build/.DS_Store` was moved aside for the drift check only and restored; it is not part of this change. The superseded harness-anchor and cancellation-wiring failure logs remain as the record of what the corrections fixed. Native WKWebView collection interaction is still unobserved by a human; native Windows and macOS CI on the pull request is the platform proof.
