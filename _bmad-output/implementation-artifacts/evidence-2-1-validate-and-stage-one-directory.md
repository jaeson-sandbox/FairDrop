# Evidence: Story 2.1: Validate and Stage One Directory

Moved out of the spec on 2026-09-08 so the spec stays at its intended size. Nothing here was
edited in the move; section order is preserved.

## Prior Loop Evidence (must be reconfirmed)

The following evidence belongs to the discarded review-loop-1 implementation. Re-derived code must reproduce or supersede it before review; it is not current acceptance evidence.

- Independent green gates: `gofmt -l .`; `go test -count=1 ./...`; `go vet ./...`; `go test -count=1 -race ./...`; frontend Vitest (17 files, 490 tests); frontend production build; and `wails build` producing `build/bin/fairdrop.exe`.
- Linux/amd64 and Darwin/amd64 source test binaries cross-compiled. Unix execution remains a Story 2.2/native-runner obligation; no Unix result is claimed here.
- Focused Windows source run executed and passed native junction-root/nested-junction refusal, parent-handle lookup after rename, long and extended paths, real file/directory defaults, and the complete loopback administrative-share fixture. Native symlink cases skipped only because the process lacks symlink privilege; deterministic link/reparse tests and native junction tests covered the same refusal matrix.
- Contract greps found one production `SourcePort`, no retired production interface declarations, and no `WalkDir` or `ReadDir(-1)` in the source adapter.

Every mutation below failed through the named behavioral assertion, not compilation, and was restored before the final green run:

| Deliberate break | Named failing test |
| --- | --- |
| Remove Windows `FILE_OPEN_REPARSE_POINT` | `TestInspectRejectsNativeJunctionRootWithTrailingSeparator` |
| Drop `RootDirectory: parent` from `NtCreateFile` | `TestNativeChildLookupStaysRelativeToOpenedParentAfterRename` |
| Remove inspected/opened identity comparison | `TestInspectRefusesChangedOpenedIdentityBeforeEnumeration` |
| Ignore opened reparse metadata | `TestInspectRefusesPostOpenReparseBeforeEnumeration` |
| Change `ReadDir(1)` batch to two | `TestInspectUsesParentRelativeOneEntryTraversalAndBoundedHandles` |
| Disable active-ancestor cycle refusal | `TestInspectRefusesActiveAncestorCycle` |
| Remove immediate post-inspection cancellation check | `TestInspectCancellationWinsAfterActiveOperation` |
| Admit one overflowing `int64` sum | `TestInspectRejectsNegativeAndOverflowingLogicalSizes/overflow` |
| Admit `9007199254740992` through the coordinator | `TestStageRejectsUnrepresentableMetadataBeforeResourceAcquisition/{oversize_file,oversize_directory}` |
| Change the POSIX root label literal | `TestFilesystemRootLabelsAreSeparatorFree` |
| Restore the early-return ancestor cleanup leak | `TestInspectAncestorCloseFailureStillClosesEveryOwnedHandle` and `TestInspectCancellationDuringAncestorCloseStillClosesEveryOwnedHandle` |

## Current Loop 2 Evidence

- Green gates were independently rerun after the hardening patch against this exact worktree: `gofmt -l .`; `go test -count=1 ./...`; `go vet ./...`; `go test -count=1 -race ./...`; frontend Vitest (17 files, 490 tests); frontend production build; and `wails build`, producing `build/bin/fairdrop.exe`.
- Linux/amd64 and Darwin/amd64 source test binaries cross-compiled successfully. Native Unix execution remains a Story 3.2 runner gap.
- Focused Windows source tests executed and passed direct-API native junction root/ancestor/nested refusal, long and extended paths, parent-relative directory opening after parent rename, metadata-only file staging through a traverse-only ancestor, DOS-device superscript aliases, and a complete loopback administrative-share directory fixture. The optional environment-configured UNC fixtures skipped because their variables were unset; the ordinary loopback UNC capability check and FairDrop inspection ran separately and both passed.
- Contract/code-shape audit found one production `SourcePort`, no retired production interface declarations, no `WalkDir`, no `ReadDir(-1)`, no source shadow interface, and no production reconstructed child paths.
- Formal review regressions passed in the focused Windows source/coordinator run. Native selected, dangling, and ancestor symlink cases skipped only for Windows error 1314 (missing symlink privilege); direct-API junction, long/extended path, zero-byte file, fake socket/irregular mode, nil/pre-cancelled context, Parse cancellation, coded-error, unknown-kind, and enumeration-close cases ran and passed. The 50,000-entry retained-memory/handle ceiling passed 10 consecutive runs, and Linux/amd64 plus Darwin/amd64 source test binaries cross-compiled with the new POSIX search/enumeration substitution and Linux O_PATH/FIFO tests. Native POSIX execution remains the documented Story 3.2 runner gap.

### Formal Review Triage

The three context-free review layers produced 24 raw findings. Findings with the same claim/action were deduplicated independently, then routed as follows:

- **Patched:** native root-label loss after `..`; non-zero metadata returned with a deferred cleanup error; cancellation between Parse and the first open; coded filesystem errors being reclassified; unknown staged item kinds; the stable `source_changed` description; overly broad native-fixture skips; incomplete parser assertions; missing retained-heap evidence; missing enumeration-close coverage; and baseline regressions for zero-byte files, native links, special modes, and already-cancelled/nil contexts. POSIX/Linux-specific tests were added for the platform guarantees even though this Windows host can only cross-compile them.
- **Deferred:** execution of the new production POSIX tests on native Linux and macOS. This is an infrastructure/runner limitation, not a code exemption, and remains owned by Story 3.2.
- **Rejected:** treating direct `icacls.exe` execution as a shell command (there is no command shell or interpolated path); requiring non-native mixed-slash spellings for the Windows extended namespace; and requiring a case-canonical filename when the caller-path preservation contract permits the inspected component spelling. These do not represent supported-product failures.

No finding required changing the frozen intent or re-deriving the architecture. The accepted patches were mutation-checked and the complete verification suite was rerun afterward.

### Matrix Test Audit

| Frozen matrix row | Covering behavioral tests that ran and passed |
| --- | --- |
| Safe tree | `TestInspectProductionDefaultSafeTreesAndFiles`, `TestInspectProductionDefaultEmptyAndTrailingDirectory`, `TestStagedDirectoryReportsIsDir`, and the pre-existing exact URL/QR/session/warning coordinator assertions |
| Unsafe entry | `TestInspectRejectsLinksSpecialsAndStopsBeforeLaterEntries`, `TestInspectRejectsNativeJunctionAncestor`, `TestInspectRejectsNativeJunctionRootWithTrailingSeparator`, and `TestInspectRejectsNativeNestedJunction` |
| Metadata race | `TestInspectRefusesChangedOpenedIdentityBeforeEnumeration`, `TestInspectNestedLookupStaysRelativeToOpenedParentAfterRename`, and `TestNativeChildLookupStaysRelativeToOpenedParentAfterRename` |
| Cancellation | `TestInspectCancellationWinsAfterEveryActiveOperation`, `TestInspectCancellationWinsDuringLexicalSearchOperations`, and `TestInspectCancellationDuringAncestorCloseStillClosesEveryOwnedHandle` |
| Invalid size | `TestInspectRejectsNegativeAndOverflowingLogicalSizes` and `TestStageRejectsUnrepresentableMetadataBeforeResourceAcquisition` |
| Native paths | `TestInspectProductionDefaultDotDotPreservesCallerPath`, `TestInspectPreservesLongWindowsPath`, `TestInspectPreservesExtendedLengthWindowsPath`, and `TestInspectLoopbackAdministrativeShareDirectory` |
| Trailing separator | `TestInspectProductionDefaultEmptyAndTrailingDirectory` and `TestInspectRejectsNativeJunctionRootWithTrailingSeparator` |

All covering tests above executed rather than merely existing. The two environment-variable UNC tests were skipped because no external fixtures were configured; they are supplementary and do not supply the matrix coverage claimed here.

Every loop-2 mutation below failed through the named behavioral assertion and was restored before the final green run:

| Deliberate break | Named failing test |
| --- | --- |
| Remove Windows `FILE_OPEN_REPARSE_POINT` | `TestInspectRejectsNativeJunctionRootWithTrailingSeparator` |
| Drop `RootDirectory` from `NtCreateFile` | `TestNativeChildLookupStaysRelativeToOpenedParentAfterRename` |
| Remove inspected/opened identity comparison | `TestInspectRefusesChangedOpenedIdentityBeforeEnumeration` |
| Skip opened reparse metadata refusal | `TestInspectRefusesPostOpenReparseBeforeEnumeration` |
| Change `ReadDir(1)` batch to two | `TestInspectUsesParentRelativeOneEntryTraversalAndBoundedHandles` |
| Disable active-ancestor cycle refusal | `TestInspectRejectsActiveAncestorCycle` |
| Remove immediate post-`ReadDir` cancellation check | `TestInspectCancellationWinsAfterEveryActiveOperation/read:root` |
| Admit one overflowing `int64` sum | `TestInspectRejectsNegativeAndOverflowingLogicalSizes/overflow` |
| Admit `9007199254740992` through the coordinator | `TestStageRejectsUnrepresentableMetadataBeforeResourceAcquisition/oversize_file` |
| Include punctuation in the Windows drive-root label | `TestFilesystemRootLabelsAreSeparatorFree` |
| Add file-read/list rights to metadata opens | `TestWindowsHandleRightsSeparateMetadataSearchAndEnumeration` |
| Return early from multi-handle cleanup | `TestInspectAncestorCloseFailureStillClosesEveryOwnedHandle` |
| Restore a synthetic anchor name after component + `..` | `TestInspectUsesNativeRootLabelsAfterDotDotReturnsToAnchor/{drive_component_dot-dot,UNC_component_dot-dot}` |
| Keep returned metadata after deferred cleanup failure | `TestInspectDeferredCleanupErrorClearsReturnedMetadata` |
| Omit the post-Parse cancellation check | `TestInspectCancellationFromParsePreventsAnchorOpen` |
| Reclassify coded adapter errors, including `transfer_failed` | both coded-error variants across all seven operations in `TestInspectPreservesCodedErrorsAcrossFilesystemClassifiers` |
| Admit an unknown staged item kind | `TestStageRejectsUnknownItemKindBeforeResourceAcquisition` |

## Independent Review Round (2026-09-03)

The recorded loop-2 evidence was re-verified rather than accepted. Gates were rerun
from a clean tree on this host: `gofmt -l .` clean, `go vet ./...` 0, `go test -count=1 ./...`
0 across 7 packages, `go test -count=1 -race ./...` 0, frontend Vitest 17 files / 490 tests,
frontend production build, and `wails build` exit 0.

Sixteen fresh mutations were then run against guards the recorded table did not cover.
Three load-bearing guarantees turned out to be pinned by **no** test, each because a second
guard masked it:

| Surviving mutation | Why it survived | Now caught by |
| --- | --- | --- |
| Delete the special-file clause in `rejectUnsupportedInfo` **and** the not-a-directory fallback together | Every unsafe-entry case added its node as a *child*; the frozen matrix refuses a **selected** special file too | `TestInspectRejectsSelectedSpecialFile` |
| Delete the trailing-separator refusal in the regular-file branch | `hadTrailingSep` was pinned where it is *parsed*, never where it is *consumed* | `TestInspectRejectsTrailingSeparatorOnRegularFile` |
| Disable the `requireDirectory` recheck in `verifyOpened` | The identity comparison caught every existing case; only an identity-preserving swap isolates this guard | `TestInspectRefusesNestedDirectoryOpenedAsFileWithUnchangedIdentity` |

Two further survivors were examined and are **not** defects, recorded so a later loop does not re-chase them:

- Removing `entrySize < 0` from the traversal sum is an equivalent mutant: `math.MaxInt64 - (-1)`
  overflows to `math.MinInt64`, so the sibling overflow clause still rejects the entry. The
  `/negative` subtest does pin the behaviour.
- `if !currentInfo.IsDir()` after the component walk still survives alone. It is reachable only
  when a lexical ancestor is swapped for a special file between descent and a `..` pop, because
  that pop re-`Stat`s without re-running `rejectUnsupportedInfo`. Pinning it needs a fake that
  changes its reported mode between two stats of the same non-opened handle, which the harness
  cannot yet express. Deferred to `2-2-stream-a-safe-directory-zip`, which already owns
  claim-time revalidation.

The three added tests were themselves mutation-checked: each fails through its named assertion,
not through a compile error, and the full gate is green with them in place.

