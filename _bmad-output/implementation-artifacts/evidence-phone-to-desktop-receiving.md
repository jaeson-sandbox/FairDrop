# Evidence: phone-to-desktop receiving, Story A (backend)

Spec: `spec-phone-to-desktop-receiving.md` (baseline `a8e10d7`). Product contract: `_bmad-output/specs/spec-phone-to-desktop-receiving/{SPEC,receive-contract}.md`. Story B (frontend) is not covered here.

Everything below was produced on macOS (darwin/arm64, Go 1.27.0, Chromium via Playwright). Windows- and Linux-only code was type-checked and vetted (`GOOS=windows|linux go build/vet`, bare `staticcheck`) but **not executed**; see "Unproven here".

## I/O and edge-case matrix to executed tests

Test names are exact. `sink` = `internal/sink`, `server` = `internal/server`, `transfer` = `internal/transfer`, `root` = package `main`.

| Matrix row | Executed test(s) |
| --- | --- |
| Receive, chooser dismissed: no change, no listener | root `TestDismissingTheReceiveChooserIsANoOp` |
| Chooser opens at Downloads, home, or anywhere; failure is `chooser_failed` | root `TestSelectReceiveFolderOpensTheDirectoryChooserAtDownloads`, `TestSelectReceiveFolderFallsBackToHomeThenToWherever`, `TestSelectReceiveFolderReportsAChooserFailureWithTheChooserCode` |
| Receive with a valid folder from IDLE: waiting session with token and QR, nothing written | transfer `TestStartReceiveCommitsAWaitingSessionAndWritesNothing`, `TestStartReceiveOrdersTheDestinationBeforeEveryNetworkResource`; root `TestNativeReceiveSavesFilesReportsTheExactCountAndShowsTheFolder`, `TestStartReceiveDelegatesWithTheCommandContextAndReturnsMetadataUnchanged`; sink `TestNothingIsCreatedUntilTheFirstFile` |
| Missing, non-directory or link-like folder: typed refusal before networking | transfer `TestStartReceiveRefusesAnUnusableFolderBeforeTouchingTheNetwork`; sink `TestOpenDestinationRefusesWhatItMustAndCreatesNothing`; root `TestNativeReceiveRefusesUnusableFoldersBeforeAnyNetworkResource` |
| Receive outside IDLE: `busy`, no state change | transfer `TestStartReceiveIsAdmittedOnlyFromIdle` (staged send, waiting receive, transferring receive, transferring send, held receive outcome), `TestStageIsRefusedWhileAReceiveSessionIsLive`, `TestStartReceiveWhileClosingIsRefused`, `TestStartReceiveRejectsBadInputsBeforeAnyStateChange`, `TestACoordinatorBuiltWithoutASinkRefusesToReceiveButStillSends`; root `TestNativeReceiveSavesFilesReportsTheExactCountAndShowsTheFolder` |
| GET valid token: script-free page, nothing claimed | server `TestTheUploadPageIsScriptFreeAndLeavesTheSessionWaiting`; browser `frontend/browser/upload-live.mjs` (real Chromium, 320 px) |
| Wrong token, route, method, malformed path: generic 404 | server `TestEveryRouteOrMethodThatIsNotTheExactUploadIsTheGenericNotFound` (19 requests), `TestASendServerDoesNotAnswerTheUploadRoute` |
| POST with no Content-Length (411), not multipart / no file part / oversized part header (400): session keeps waiting, nothing written | server `TestRejectedRequestsWriteNothingAndLeaveTheSessionWaiting` (8 cases, each followed by a successful retry on the same link), `TestAZeroLengthBodyIsAnEmptyUploadNotAMissingLength`, `TestAPartHeaderBeyondItsBoundIsRefusedBeforeTheClaim` |
| POST whose declared size leaves under 3 GiB free: 413, session keeps waiting, desktop notified | server `TestAnUploadTooLargeForTheDiskIsRefusedNotedAndLeavesTheSessionWaiting`, `TestAnUnanswerableSpaceCheckRefusesTheUploadToo`; transfer `TestARefusedUploadNoticeReachesTheDesktopWhileTheSessionKeepsWaiting`, `TestANoticeAfterTheClaimIsDroppedAndAnUnknownOneIsADiagnostic`, `TestASendSessionNeverPublishesANotice`; sink `TestTheSpaceReserveBoundary`, `TestCheckSpaceUsesTheVolumeAnswerAndTheThreeGiBReserve`, `TestCheckSpaceAsksTheRealVolume` |
| Valid POST of 3 files including a duplicate name: subfolder, `x.jpg`, `x (1).jpg`, "3 files saved", desktop complete with count | server `TestOneMultipartPostSavesExactBytesUnderSanitizedAndDedupedNames`, `TestThereIsNoCapOnTheNumberOfFiles` (300 files), `TestPartsOutsideTheFilesFieldAreDiscardedNeverWritten`, `TestTheStandardMultipartEncoderIsAccepted`; transfer `TestACompleteUploadPublishesProgressThenAnOutcomeAndHoldsIt`; root `TestNativeReceiveSavesFilesReportsTheExactCountAndShowsTheFolder`; browser `upload-live.mjs` (3 files from a real form) |
| Subfolder name exists: ` (n)` appended, exclusive mkdir, never reuse | sink `TestAnExistingSubfolderNameGetsANumberAndNothingInItChanges`, `TestASymbolicLinkOnTheSubfolderNameIsNeverFollowed`, `TestMkdirExclusiveRefusesAnyExistingName`, `TestTheSubfolderNameUsesTheClockWallTime` |
| Unsafe, empty or path-bearing phone name: sanitized, `file` fallback, never refused | sink `TestSanitizeNameTable`, `TestSanitizeNameStripsBothSeparatorsOnEveryHost`, `TestSanitizeNameTruncatesOnARuneBoundaryWithinTwoHundredFiftyFiveBytes`, `TestSanitizedNamesAreAlwaysPortableSingleSegments`, `TestDuplicateAndUnsafeNamesStayInsideTheSubfolder`; server `TestOneMultipartPostSavesExactBytesUnderSanitizedAndDedupedNames` (a `..\..\evil\dropper.sh` name) |
| In-upload de-duplication, case-insensitive | sink `TestDuplicateNamesAreNumberedBeforeTheExtensionCaseInsensitively`, `TestADuplicateOfALongNameStillFitsTwoHundredFiftyFiveBytes`, `TestDuplicateAndUnsafeNamesStayInsideTheSubfolder` |
| Existing file never opened for writing, replaced or appended | sink `TestCreateExclusiveNeverOpensAnExistingName`, `TestCreateExclusiveDoesNotFollowALinkOnItsName`, `TestRenameNoReplaceNeverReplacesAnExistingFile`, `TestRenameNoReplaceDoesNotReplaceADirectoryEither`, `TestAFinalNameTakenBeforeTheRenameIsNeverReplaced`; server `TestOneMultipartPostSavesExactBytesUnderSanitizedAndDedupedNames` (a pre-existing file in the destination is unchanged) |
| Files and folders never executable | sink `TestFilesAndFoldersAreNeverExecutable`; server `TestOneMultipartPostSavesExactBytesUnderSanitizedAndDedupedNames` |
| Second POST after claim: 423 while the listener lives, no write | server `TestASecondRequestWhileAnUploadIsInFlightGets423AndWritesNothing` |
| Connection drop mid-file: partial removed, completed kept, "incomplete, N saved" | server `TestADroppedConnectionKeepsCompletedFilesAndRemovesThePartial`; transfer `TestAnIncompleteUploadPublishesAnErrorWithTheExactSavedCount`; sink `TestAReadErrorMidFileRemovesOnlyThePartialFile` |
| Empty subfolder removed when N = 0 | server `TestADroppedConnectionBeforeAnyFileSavedLeavesNoSubfolder`; sink `TestAFirstFileFailureLeavesNoSubfolderOnceTheDestinationCloses`; transfer `TestAnUploadThatSavedNothingReportsIncompleteWithoutASubfolder` |
| Body past the declared size | server `TestABodyPastTheDeclaredLengthIsCutOffAndReportedIncomplete` (over the wire), `TestTheServerItselfAbortsWhenTheBodyExceedsItsDeclaredLength` (handler-level, the only way to deliver surplus bytes) |
| Inactivity | server `TestAnUploadThatStallsIsEndedByTheInactivityBound`, `TestAnUploadLongerThanTheWholeRequestDeadlineStillCompletes`, `TestTheProductionUploadInactivityBoundIsThirtySeconds`; existing `TestATransferLongerThanEveryTimeoutStillCompletes` unchanged and green |
| Write error / disk full | server `TestAWriteErrorMidUploadIsIncompleteWithTheExactCount` |
| Desktop Cancel mid-upload: connection closed within bounds, same keep/remove rule, "cancelled, N saved" | server `TestStopMidFileKeepsCompletedFilesAndRemovesThePartialFile`; transfer `TestCancellingAnUploadInFlightPublishesACancelledOutcomeAndHoldsIt`, `TestACancelRacingACompletionPublishesExactlyOneTerminalOutcome`, `TestCancellingDuringTheClaimPublishesNoOutcome`, `TestCancellingAWaitingReceiveWritesNothingAndResets`; root `TestNativeReceiveCancelMidFileKeepsCompletedFilesAndReportsCancelled` (real coordinator, sink and server) |
| Unquiescent bound reported, never success | transfer `TestADestinationThatNeverClosesIsReportedNotAbsorbed`, `TestADestinationThatNeverOpensTimesOutAndALateOneIsClosed`; the existing Stop/bound suites cover the server and beacon bounds |
| Cancel or failure while the folder is being validated | transfer `TestCancellingWhileTheFolderIsBeingValidatedAbortsTheStart`, `TestAnAbandonedCommandContextAbortsTheStartAfterTheFolderOpens`, `TestAFailedStartAfterTheDestinationOpenedClosesItExactlyOnce` |
| Shutdown | transfer `TestShutdownDuringAnUploadClosesTheDestinationAndPublishesNothing`, `TestShutdownFromAHeldReceiveOutcomeIsQuietAndIdempotent` |
| Marking unsupported: file kept, outcome warning | sink `TestAMarkingFailureKeepsTheFileAndIsReported`; transfer `TestAnIncompleteUploadPublishesAnErrorWithTheExactSavedCount` (`markingWarning` true) |
| Marking applied | sink `TestSavedFilesCarryTheQuarantineAttributeOnMacOS` (executed on macOS); `TestSavedFilesCarryTheMarkOfTheWebOnWindows` (Windows CI only) |
| Show in Folder: argument vector, no shell, refused without a subfolder | root `TestShowReceivedFolderHandsTheSubfolderToTheOSAsOneArgument`, `TestTheFileManagerCommandIsAnArgumentVectorNeverAShellLine`, `TestTheFileManagerCommandRefusesAnUnusablePath`, `TestShowReceivedFolderRefusesWhenThereIsNoSubfolder`, `TestAFailedOSLaunchIsACodedErrorThatNamesNoPath`, `TestNewAppShipsTheNativeFileManagerLauncher`; transfer `TestReceivedFolderRefusesWhenThereIsNoSubfolder`, `TestAHeldReceiveOutcomeServesShowInFolderUntilTheUserLeavesIt` |
| Bounded memory; no OS temp storage | server `TestALargeUploadStreamsWithBoundedMemoryAndNothingInOSTempStorage` (64 MiB upload, peak heap growth 255 KiB measured); sink `TestAFileIsCopiedThroughOneBoundedBufferAndNothingGoesToOSTempStorage`; server `TestTheServerNeverBuffersAMultipartBody` (no `ReadForm`/`ParseMultipartForm`) |
| Disclosure: no path, subfolder, file name or token off the machine | transfer `TestNothingThatLeavesTheCoordinatorNamesTheFolder`; server result-page assertions in `TestOneMultipartPostSavesExactBytesUnderSanitizedAndDedupedNames`; root `TestNativeReceiveSavesFilesReportsTheExactCountAndShowsTheFolder` |
| Wire shapes Story B reads | root `TestTheReceiveWireShapesAreExactlyWhatTheFrontendReads`, `TestPublishEmitsTheNoticeEventUnderItsOwnName`, `TestEventLogLinesCarryCountsAndFixedWordsOnly`, `TestTheAppBindsExactlyTheContractCommands` (three commands added to the allow-list) |
| Receive complete is not demoted by a failed result-page write | server `TestAFailedResultPageWriteDoesNotDemoteACompleteUpload` |
| Production defaults, not just seams | sink `TestTheProductionDefaultsSaveARealFile`, `TestCheckSpaceAsksTheRealVolume`; server `TestTheProductionUploadInactivityBoundIsThirtySeconds`; root `TestNewAppShipsTheNativeFileManagerLauncher` |
| Existing send flows unchanged | the full pre-existing suites pass unmodified apart from one allow-list entry in `TestTheAppBindsExactlyTheContractCommands` and one `sink` field on the transfer test harness |

## Scoped mutation inventory

One canonical script: `_bmad-output/implementation-artifacts/mutate-phone-to-desktop-receiving.py`, derived from `COMMON` plus the host's `BY_PLATFORM` list; it uses `scripts/mutationverdict`, so a build failure, panic, skip or timeout is rejected as proof, and the asserted fragment must appear in the failing test's own transcript. Each case runs the same focused test unmodified (must pass) and with exactly one anchored edit (must fail on the named fragment) in an isolated copy of the tree. Complete transcripts: `evidence-phone-to-desktop-receiving-logs/<id>-{baseline,mutant}.log`; `summary.txt` is the run's verdict.

Result on darwin/arm64: **27 cases, 27 baselines passed, 27 mutants failed on their named assertion.** Cases that returned `build-fail` on a first run (three mutants that left an unused variable or import) were corrected and the whole script re-run from the top; the table is from the second, complete run.

| Case | Guarantee broken | Focused test | Fragment that must appear |
| --- | --- | --- | --- |
| `no_replace_rename` (darwin/linux/windows anchors) | rename never replaces | sink `TestRenameNoReplaceNeverReplacesAnExistingFile` | `want fs.ErrExist` |
| `exclusive_subfolder` | subfolder creation is exclusive | sink `TestAnExistingSubfolderNameGetsANumberAndNothingInItChanges` | `destination holds` |
| `exclusive_file` | temp file creation is exclusive | sink `TestCreateExclusiveNeverOpensAnExistingName` | `createExclusive opened a file that already existed` |
| `reserve_boundary` | exactly 3 GiB remaining passes | sink `TestTheSpaceReserveBoundary` | `exactly the reserve remains` |
| `reserve_constant` | the reserve is 3 GiB | sink `TestCheckSpaceUsesTheVolumeAnswerAndTheThreeGiBReserve` | `want exactly 3 GiB` |
| `overflow_abort` | bytes past the declared length are never saved | server `TestTheServerItselfAbortsWhenTheBodyExceedsItsDeclaredLength` | `want incomplete with exactly 1 file saved` |
| `partial_removal` | the in-progress file is removed on failure | sink `TestAReadErrorMidFileRemovesOnlyThePartialFile` | `want only the completed file` |
| `empty_subfolder` | a session that saved nothing leaves no subfolder | sink `TestAFirstFileFailureLeavesNoSubfolderOnceTheDestinationCloses` | `a session that saved nothing left` |
| `sanitize_separators` | a path-bearing name keeps only its last component (both separators) | sink `TestSanitizeNameStripsBothSeparatorsOnEveryHost` | `want "evil.dll"` |
| `dedupe_case` | in-upload de-duplication is case-insensitive | sink `TestDuplicateNamesAreNumberedBeforeTheExtensionCaseInsensitively` | `want "X (2).JPG"` |
| `bounded_buffer` | one 64 KiB buffer per file | sink `TestAFileIsCopiedThroughOneBoundedBufferAndNothingGoesToOSTempStorage` | `want exactly 64 KiB` |
| `inactivity_refresh` | the inactivity deadline is re-armed on progress | server `TestAnUploadLongerThanTheWholeRequestDeadlineStillCompletes` | `inactivity bound must never have fired` |
| `inactivity_deadline` | a stalled upload is ended by the inactivity bound | server `TestAnUploadThatStallsIsEndedByTheInactivityBound` | `no terminal event within 10s` |
| `multipart_streaming` | the body is streamed, not buffered (behaviour) | server `TestALargeUploadStreamsWithBoundedMemoryAndNothingInOSTempStorage` | `status = 400, want 200` |
| `multipart_streaming_source` | `ReadForm`/`ParseMultipartForm` never appear | server `TestTheServerNeverBuffersAMultipartBody` | `buffers a multipart body` |
| `space_refusal` | an upload that does not fit is refused with 413 | server `TestAnUploadTooLargeForTheDiskIsRefusedNotedAndLeavesTheSessionWaiting` | `want 413` |
| `space_notice` | the desktop is told about a refused upload | same test | `no notice within 10s` |
| `length_required` | a POST without a declared length is 411 | server `TestRejectedRequestsWriteNothingAndLeaveTheSessionWaiting` | `want 411` |
| `claimed_locked` | a claimed session answers 423 | server `TestASecondRequestWhileAnUploadIsInFlightGets423AndWritesNothing` | `during an upload` |
| `complete_not_demoted` | a failed result-page write does not fail a saved upload | server `TestAFailedResultPageWriteDoesNotDemoteACompleteUpload` | `want complete` |
| `receive_admission_idle` | a receive is admitted only from IDLE | transfer `TestStartReceiveIsAdmittedOnlyFromIdle` | `want busy` |
| `cancel_outcome` | a cancelled upload publishes a cancelled outcome, not a bare reset | transfer `TestCancellingAnUploadInFlightPublishesACancelledOutcomeAndHoldsIt` | `want started then one error and no reset` |
| `destination_close` | the destination is closed in the unwind | transfer `TestCancellingAWaitingReceiveWritesNothingAndResets` | `the destination was closed` |
| `outcome_held` | a receive outcome is held, not reset after 3 s | transfer `TestACompleteUploadPublishesProgressThenAnOutcomeAndHoldsIt` | `want it held until the user leaves it` |
| `show_refuses_without_subfolder` | Show in Folder refuses when nothing was saved | transfer `TestReceivedFolderRefusesWhenThereIsNoSubfolder` | `ReceivedFolder before any file was saved` |
| `show_in_folder_no_shell` | the folder goes to the OS as one argument, never a shell line | root `TestTheFileManagerCommandIsAnArgumentVectorNeverAShellLine` | `want "open"` |
| `quarantine_marking` (darwin) / `mark_of_the_web` (windows) | saved files carry the OS download marking | sink `TestSavedFilesCarryTheQuarantineAttributeOnMacOS` / `TestSavedFilesCarryTheMarkOfTheWebOnWindows` | `want the attribute present on a saved file` / `want it present on a saved file` |

A scoped pass proves only its executed cases. This run executed the 27 darwin cases. The linux (3 platform cases) and windows (4 platform cases) anchors exist in the same script and were **not run**; the Windows ones in particular are unproven until the Windows CI job runs the script.

## Verification run

All commands sequential, from the worktree root, `PATH` prepended with `/opt/homebrew/bin` and `~/go/bin`.

| Step | Result |
| --- | --- |
| `wails build` | pass (bindings regenerated: `SelectReceiveFolder`, `ShowReceivedFolder`, `StartReceive`, `ReceiveMetadata`; modes restored to 0644) |
| `gofmt -l .` | empty |
| `go vet ./...` | pass |
| `go tool staticcheck ./...` | pass |
| `go test -count=1 ./...` | all 10 packages `ok` |
| `CGO_ENABLED=1 go test -count=1 -race ./...` | all 10 packages `ok` (`go env CGO_ENABLED` = 1) |
| `npm test` (frontend) | 22 files, 863 tests passed |
| `npm run test:browser` | 86 tests passed, receiver-live passed, upload-live passed |
| `GOOS=darwin GOARCH=arm64 go build ./...`, `GOOS=linux GOARCH=amd64 go build ./...`, `GOOS=windows GOARCH=amd64 go build ./...` | pass |
| `GOOS=windows|linux|darwin go vet` and bare `staticcheck` | pass |
| `python3 _bmad-output/implementation-artifacts/mutate-phone-to-desktop-receiving.py` | exit 0, 27 of 27 |

## Decisions that go beyond the spec text

1. **A receive outcome is held, not reset after three seconds.** The spec calls for Show in Folder and "a way back to Idle" from the outcome; a three second lease is not long enough to reach a button. The coordinator arms no reset for a receive session; Cancel (or Shutdown) leaves it. `docs/fairdrop-contracts.md` and the architecture decision log say so.
2. **Cancel mid-upload publishes a `transfer-error` with `error.code = cancelled` and `receive.result = "cancelled"`**, and holds ERROR, instead of a bare reset, because the spec requires "cancelled, N files saved".
3. **The count comes from the destination, not the server.** The server's terminal event carries no count; the coordinator closes the destination after the server stops and reads the final result. The desktop and the phone's result page read the same number.
4. **A sixth lifecycle event, `transfer-notice`**, carries the "refused for space" note to the desktop. It is new wire surface Story B must subscribe to (the current frontend ignores it).
5. **`ReceiveMetadata` has a `warnings` array** (the beacon warning applies to a receive session too), beyond the four fields the task listed.
6. **No new `ErrorCode`.** Refusals reuse `path_not_found` / `path_unsupported` / `chooser_failed` / `cancelled`; their public copy is send-flavoured. Receive-specific copy needs an owner decision and the cross-language registry updates named in `TestTheCodeRegistryIsExactlyThisSet`.
7. **`frontend/package.json` `test:browser` now also runs `browser/upload-live.mjs`**, a real-Chromium check of the upload form. Test-only; no production frontend file changed.

## Unproven here

- Windows execution of `fs_windows.go` (no-replace `MoveFileEx`, `CREATE_NEW`, reparse-point refusal, `GetDiskFreeSpaceEx`, the `Zone.Identifier` stream) and of the Windows mutation anchors: compile- and vet-checked only. Windows has no `openat`, so its write pin is weaker than POSIX against a hostile local process racing the folder.
- Linux execution of the sink and its `renameat2` path.
- macOS `renameatx_np(RENAME_EXCL)` on FAT/exFAT/network volumes: not verified. The code falls back to the hard-link form only on `ENOTSUP`/`EINVAL`, and otherwise fails closed, so a volume that supports neither yields an incomplete upload rather than a weakened guarantee.
- Phone browsers (Safari iOS, Chrome Android): only Go's clients, hand-built bodies, Go's multipart encoder and desktop Chromium were used.
- Reparse-point refusal on Windows is broad (junctions, OneDrive placeholders); a destination on such a folder is refused. That mirrors the source adapter's rule and is stated, not tested on a placeholder.
- `fsync` durability, and marking on filesystems that cannot hold it, beyond the injected-failure test.
