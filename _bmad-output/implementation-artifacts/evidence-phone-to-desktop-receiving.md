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

# Frontend (Story B)

Spec: `spec-phone-to-desktop-receiving.md`, Story B. Baseline: Story A at `6bb7f12` (detached HEAD, nothing committed). Backend Go code was not touched; no backend defect or contract gap was found. Produced on macOS (darwin/arm64, Node 24.19, Chromium via Playwright). What this does **not** prove is in "Unproven here (Story B)".

## I/O and edge-case matrix to executed tests (frontend)

Test names are exact. `V` = `src/transfer/validation.receive.test.ts`, `S` = `state.receive.test.ts`, `C` = `useTransfer.receive.test.tsx`, `Sel` = `selectors.receive.test.ts`, `Idle` = `src/ui/IdleView.receive.test.tsx`, `Wait` = `ReceiveWaitingView.test.tsx`, `Recv` = `ReceivingView.test.tsx`, `Out` = `ReceiveOutcomeView.test.tsx`, `Ann` = `announce.receive.test.ts`, `Copy` = `copy.receive.test.ts`, `App` = `src/App.receive.test.tsx`, `Br` = `frontend/browser/receive.test.tsx` (rendered Chromium).

| Requirement | Executed test(s) |
| --- | --- |
| Receive action in Idle beside the send entry points; send entry points unchanged | Idle `the Receive entry in Idle` (named Receive, hint, 44px secondary, outside the drop zone and not a drop target, absent without a handler, rest of Idle unchanged); App `the Receive entry`; existing `IdleView.test.tsx` and `App.test.tsx` pass unmodified apart from two added controller members |
| Chooser dismissed is a no-op; focus returns to Receive | C `does nothing when the chooser is dismissed`, `treats a whitespace-only result like a dismissal`, `can be activated again after a dismissal`; Idle `focus after the chooser` (focus on activation, returned on dismissal, marker only for keyboard, cleared on blur, untouched for `proceeded`/`ignored`, detached control); App `returns focus to Receive after a dismissed chooser` |
| Single-flight guard against double activation | C `single flight` (chooser open, session starting, outside Idle, send chooser vs receive chooser); `refuses Receive while the collection draft is open` |
| Cancel or unmount invalidates a pending chooser completion | C `invalidated chooser completions` (late folder after unmount, after Cancel, late failure, start finishing after unmount is quiesced) |
| Refusals use receive-specific safe copy in the existing error pattern | C `refusals` (chooser failure, six StartReceive codes, unrecognised rejection, malformed acknowledgement, failed quiescing cancel, cancelled start, late success); S `marks a failed receive start...`; Copy `receive failure wording` (exact heading and message for 10 codes, fallback, no send/item/prepare, banned vocabulary, frozen); Idle `a failed Receive card`; App `a failed Receive start` |
| Waiting view: QR, non-draggable, link on demand, Copy Link, basename only, instruction, Cancel | Wait `the receive waiting view`, `the upload link`, `cancelling`; Br `the receive QR image cannot start a drag` |
| Polite notice for `receive_too_large`, session keeps waiting | V `the transfer-notice event`; S `waiting for a phone`; Wait `a refused upload`; Ann `a refused upload`; App `speaks a refused upload once...`, `speaks a second refusal again`, `does not speak a refusal over a pending cancellation` |
| Receiving view: bytes vs declared total with percentage, files saved, Cancel | Recv (all); S `receiving`; Sel `receive progress` |
| Outcome: N files saved / Upload incomplete / Upload cancelled / none saved, singular and plural | Out `a complete receive`, `an incomplete receive is never shown as a success`, `a cancelled receive`; Copy `the composed count copy`, `the composed outcome sentences`; App `says each outcome truthfully through the whole shell` |
| Marking warning when `markingWarning` | Out `the marking warning` |
| Show in Folder only when `subfolderExists` | Out `Show in Folder` (six outcome/flag combinations, flag not count, failure note); C `Show in Folder` (guards, single launch, fixed failure note, recovery); App `the held outcome at the App boundary` |
| Done calls CancelTransfer to return to Idle; outcome is held | C `holds the outcome`, `leaves the outcome through the existing Cancel command...`, `does not issue a second Cancel...`, `keeps an outcome visible if leaving fails`; S `the held outcome`; App `Done leaves through the existing Cancel command, never a new one` |
| Strict validation of receive metadata and event payloads | V (80 cases: `parseReceiveMetadata`, `parseReceiveStatus`, receive lifecycle events, notice event) |
| Session-kind awareness; send flows untouched and never offered for a receive outcome | S `send sessions never take receive events`; Sel `never offers a receive outcome as a send outcome card`; C `send flows stay out of a receive session` (no Send Again/Try Again for any of the three outcomes, remembered send path forgotten, drop and draft refused, send works afterwards); Out `never offers a send affordance`; App `offers no send affordance beside any receive outcome`; the existing send suites pass unmodified (see "Existing tests touched") |
| Announcements: one owner per transition; focus moves | Ann (all rows); App `receive transitions: one owner each`; announce `focusTargets` list extended in `announce.test.ts` |
| No button-label ellipsis | `noButtonEllipsis.test.tsx` `no receive button or menu item ends in an ellipsis` (Idle, pending, waiting, receiving, three outcomes with and without a folder, failure card with both retry labels); Copy `puts no ellipsis on a control label` |
| Quartz tokens, 44px targets, forced-colors outline, reduced motion | `styles.receive.test.ts`; Br `the receive views at the desktop window size` (1024x768 and 640x480, every `.fd-target` at 44px), `at 320 CSS pixels`, `under 200% text and the WCAG text-spacing overrides`, `forced colors on the receive views`, `the held outcome does not jump when it appears`; existing `styles.test.ts` (no `@keyframes`, no fractional text opacity, token contrast) passes unmodified |
| EXPERIENCE copy registry and routing table | `copy.test.ts` `the spine table and the registry that quotes it` now checks the 20 new `copy.receive.*` rows against `copy.ts` |

## Scoped frontend mutation inventory

One canonical script: `scripts/verify-receive-frontend-mutations.py`. This table is generated from its `CASES` tuple (`gen` step: import the script, print each case), not typed by hand. Each case, in an isolated copy of `frontend/`: proves a passing baseline for its test file and `-t` filter (and that the filter selected at least one test), proves every source anchor matches exactly once, applies the mutation, and requires a FAIL line in the named test file whose test path contains the named assertion. A compile or import failure is refused as a kill. Complete stdout/stderr of every run is in `evidence-phone-to-desktop-receiving-frontend-logs/<case>-baseline.log` and `<case>-mutation.log`; `_run-summary.log` is the script's own output.

Result: **60 cases, 60 baselines passed, 60 mutants failed on their named assertion** (`60/60 receive frontend mutations killed`, exit 0). 54 cases run under jsdom, 6 in rendered Chromium (`browser/` files run under `vitest.browser.config.ts`).

| Case | Guarantee broken | Source mutated | Test file | Assertion that must fail |
| --- | --- | --- | --- | --- |
| `dismissal-noop` | a whitespace-only chooser result is a dismissal too | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `treats a whitespace-only result like a dismissal` |
| `dismissal-startreceive` | a dismissed chooser never reaches StartReceive | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `does nothing when the chooser is dismissed` |
| `dismissal-focus-return` | a dismissed chooser returns focus to Receive | `src/ui/IdleView.tsx` | `src/ui/IdleView.receive.test.tsx` | `does not touch focus or the marker when the activation ends` |
| `focus-ring-marker-pointer` | a pointer activation never marks the scripted focus return | `src/ui/IdleView.tsx` | `src/ui/IdleView.receive.test.tsx` | `returns focus after a pointer activation without marking it` |
| `double-activation` | a second Receive while the chooser is open is refused | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `refuses a second activation while the chooser is open` |
| `receive-outside-idle` | Receive is admitted from Idle only | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `refuses Receive outside Idle` |
| `cancel-invalidates-chooser` | Cancel invalidates a chooser result that arrives late | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `ignores a folder chosen after Cancel was activated` |
| `unmount-invalidates-chooser` | unmount invalidates a chooser result that arrives late | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `ignores a folder chosen after unmount` |
| `metadata-send-url` | a send (download) url is refused as receive metadata | `src/transfer/validation.ts` | `src/transfer/validation.receive.test.ts` | `refuses a send (download) url` |
| `metadata-extra-field` | extra receive metadata fields are refused | `src/transfer/validation.ts` | `src/transfer/validation.receive.test.ts` | `refuses an extra field` |
| `metadata-control-chars` | a destination with control characters is refused | `src/transfer/validation.ts` | `src/transfer/validation.receive.test.ts` | `refuses a destination with a control character` |
| `metadata-session-id` | a malformed receive session id is refused | `src/transfer/validation.ts` | `src/transfer/validation.receive.test.ts` | `refuses a non-hex session id` |
| `status-extra-field` | a receive payload naming anything beyond the pinned keys is refused | `src/transfer/validation.ts` | `src/transfer/validation.receive.test.ts` | `refuses an extra field naming a path` |
| `status-complete-result` | an error event can never say complete | `src/transfer/validation.ts` | `src/transfer/validation.receive.test.ts` | `refuses error saying complete` |
| `cancel-code-contradiction` | a cancelled result beside a non-cancelled code is refused | `src/transfer/validation.ts` | `src/transfer/validation.receive.test.ts` | `a contradiction is never guessed at` |
| `controller-malformed-ack` | a malformed StartReceive acknowledgement quiesces and says nothing started | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `refuses a malformed acknowledgement` |
| `receive-draft-open` | Receive is refused while the collection draft is open | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `refuses Receive while the collection draft is open, without opening a chooser` |
| `show-hidden-without-subfolder` | Show in Folder is not rendered without a subfolder | `src/ui/ReceiveOutcomeView.tsx` | `src/ui/ReceiveOutcomeView.test.tsx` | `is not rendered for a complete outcome with no subfolder` |
| `show-flag-not-count` | Show in Folder follows the backend flag, not the saved count | `src/ui/ReceiveOutcomeView.tsx` | `src/ui/ReceiveOutcomeView.test.tsx` | `follows the backend flag, not the count` |
| `show-controller-guard` | the controller never calls ShowReceivedFolder without a subfolder | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `never calls the command when the subfolder does not exist` |
| `show-single-launch` | a double activation of Show in Folder launches once | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `launches once for a double activation` |
| `show-failure-note` | a failed Show in Folder is reported as a fixed note | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `records a failed Show in Folder beside the outcome` |
| `saved-count-plural` | 1 file, not 1 files | `src/ui/copy.ts` | `src/ui/copy.receive.test.ts` | `the composed count copy > says 1 as` |
| `saved-count-zero` | no files saved is said as no files saved | `src/ui/copy.ts` | `src/ui/copy.receive.test.ts` | `the composed count copy > says 0 as` |
| `saved-count-receiving-view` | the receiving view shows the backend's saved count | `src/ui/ReceivingView.tsx` | `src/ui/ReceivingView.test.tsx` | `the receiving view > says 2 saved` |
| `saved-count-outcome-heading` | the outcome heading carries the exact count | `src/ui/ReceiveOutcomeView.tsx` | `src/ui/ReceiveOutcomeView.test.tsx` | `heads it "Upload incomplete" with the exact count` |
| `saved-count-monotonic` | the saved count never goes down while receiving | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `refuses progress that goes backwards, changes the declared total, or lowers the saved count` |
| `incomplete-as-complete-reducer` | an incomplete error event becomes an incomplete outcome | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `shows an incomplete upload as incomplete with the exact count` |
| `incomplete-as-complete-demotion` | a success that cannot be reconciled is demoted to incomplete | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `demotes a success whose snapshot does not follow the accepted progress to incomplete` |
| `incomplete-as-complete-view` | the incomplete heading is never the success heading | `src/ui/ReceiveOutcomeView.tsx` | `src/ui/ReceiveOutcomeView.test.tsx` | `heads it "Upload incomplete" with the exact count` |
| `incomplete-styling` | an incomplete outcome does not take the success styling | `src/ui/ReceiveOutcomeView.tsx` | `src/ui/ReceiveOutcomeView.test.tsx` | `heads it "Upload incomplete" with the exact count` |
| `cancelled-heading` | a cancelled upload says cancelled, not incomplete | `src/ui/ReceiveOutcomeView.tsx` | `src/ui/ReceiveOutcomeView.test.tsx` | `heads it "Upload cancelled" with the exact count` |
| `done-calls-cancel` | Done leaves through the existing CancelTransfer | `src/App.tsx` | `src/App.receive.test.tsx` | `Done leaves through the existing Cancel command, never a new one` |
| `done-button-wiring` | the Done button calls onDone | `src/ui/ReceiveOutcomeView.tsx` | `src/ui/ReceiveOutcomeView.test.tsx` | `Done calls onDone once and nothing else` |
| `controller-cancel-outcome` | the controller cancels a held receive outcome | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `leaves the outcome through the existing Cancel command, and only the reset returns to Idle` |
| `outcome-held` | a receive outcome is held, not reset by anything but the user's Done | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `is held: nothing but a reset moves it` |
| `notice-keeps-waiting` | a space refusal keeps the session waiting | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `counts each refused upload as a new state and keeps waiting` |
| `notice-count` | each refusal is a new state, so it is announced again | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `counts each refused upload as a new state and keeps waiting` |
| `notice-announced` | a refusal is spoken by the announcer | `src/ui/announce.ts` | `src/ui/announce.receive.test.ts` | `a refused upload > is spoken by the announcer, not focused` |
| `notice-not-over-cancel` | a refusal is not spoken over Canceling | `src/ui/announce.ts` | `src/ui/announce.receive.test.ts` | `is not spoken over a pending cancellation` |
| `notice-visible` | a refusal leaves a visible note beside the code | `src/ui/ReceiveWaitingView.tsx` | `src/ui/ReceiveWaitingView.test.tsx` | `keeps waiting and says what happened` |
| `receive-not-retained` | a receive outcome is never retained as a send outcome card | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `leaves to plain Idle on reset, retaining nothing a send outcome card could offer` |
| `receive-forgets-send-path` | starting a receive forgets any remembered send path | `src/transfer/useTransfer.ts` | `src/transfer/useTransfer.receive.test.tsx` | `forgets a remembered send path when a receive session starts` |
| `send-rejects-receive-events` | a send session never applies a receive payload | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `ignores receive progress, complete and error on a transferring send session` |
| `receive-rejects-send-events` | a receive session never applies a send event | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `refuses progress with no receive payload and progress with an unknown total` |
| `send-ack-over-receive` | a send acknowledgement cannot commit over a pending receive | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `refuses to commit a send acknowledgement over a pending receive` |
| `receive-error-context` | a failed receive start uses receive wording and receive actions | `src/transfer/state.ts` | `src/transfer/state.receive.test.ts` | `marks a failed receive start so the failure card uses receive wording` |
| `receive-card-wording` | a receive failure card reads the receive copy | `src/ui/OutcomePanel.tsx` | `src/ui/IdleView.receive.test.tsx` | `uses receive wording for the folder it could not use` |
| `receive-retry-action` | a receive failure card offers a retry that re-opens the folder chooser | `src/App.tsx` | `src/App.receive.test.tsx` | `says Try Again for a failure that is worth another attempt, and reopens the Receive chooser` |
| `receive-choose-label` | a receive failure card for an unusable folder says Choose Another Folder | `src/App.tsx` | `src/App.receive.test.tsx` | `shows receive wording and a folder-chooser action, never the send ones` |
| `route-leave-outcome` | leaving the outcome focuses the Idle instruction | `src/ui/announce.ts` | `src/ui/announce.receive.test.ts` | `focuses the Idle instruction when the outcome is left` |
| `route-outcome-focus` | a receive outcome moves focus once to the outcome | `src/ui/announce.ts` | `src/ui/announce.receive.test.ts` | `focuses the outcome when a receive ends, whichever way it ended` |
| `qr-draggable` | the receive QR image cannot start a drag | `src/ui/ReceiveWaitingView.tsx` | `src/ui/ReceiveWaitingView.test.tsx` | `marks the QR image non-draggable` |
| `marking-warning` | the marking warning appears when the backend reports it | `src/ui/ReceiveOutcomeView.tsx` | `src/ui/ReceiveOutcomeView.test.tsx` | `appears when the backend says marking failed` |
| `browser-forced-colors-outline` | the Receive control keeps a real outline after a scripted focus return under forced colors | `src/style.css` | `browser/receive.test.tsx` | `draws a real outline, not a box-shadow, on the Receive control after a scripted focus return` |
| `browser-reflow-320` | the receive views reflow to one column at 320 CSS pixels | `src/style.css` | `browser/receive.test.tsx` | `scrolls only vertically with nothing clipped` |
| `browser-clipped-text` | the receive views grow under 200% text and text spacing without clipping | `src/style.css` | `browser/receive.test.tsx` | `grows without horizontal scroll or a clipped container` |
| `browser-qr-draggable` | the rendered receive QR image is not draggable | `src/ui/ReceiveWaitingView.tsx` | `browser/receive.test.tsx` | `marks the receive QR non-draggable both ways` |
| `browser-outcome-centred` | the held outcome card is centred, not pinned to the top | `src/style.css` | `browser/receive.test.tsx` | `centres the outcome card in the window rather than pinning it to the top` |
| `browser-entry-wrap` | the Receive entry row wraps instead of widening the page at 320 CSS pixels | `src/style.css` | `browser/receive.test.tsx` | `scrolls only vertically with nothing clipped` |

Mutants worth naming explicitly from the brief: dismissal no-op (`dismissal-noop`, `dismissal-startreceive`), double-activation guard (`double-activation`, `receive-outside-idle`), malformed receive metadata rejected (`metadata-*`, `status-*`, `controller-malformed-ack`), Show in Folder hidden without subfolder (`show-hidden-without-subfolder`, `show-flag-not-count`, `show-controller-guard`), saved-count copy (`saved-count-*`), incomplete never shown as complete (`incomplete-as-complete-*`, `incomplete-styling`, `cancelled-heading`), Done calls cancel (`done-calls-cancel`, `done-button-wiring`, `controller-cancel-outcome`), notice keeps the waiting view (`notice-keeps-waiting`, `notice-count`, `notice-announced`), send flows not offered on a receive outcome (`receive-not-retained`, `receive-forgets-send-path`, `send-rejects-receive-events`, `send-ack-over-receive`).

Two honest findings from building the inventory:

- A first draft of `unmount-invalidates-chooser` removed one guard and survived, because unmount is guarded in three independent places (`browseMayCommit`, the cleanup that nulls the operation, and the mounted check in the session start). The case now removes all three; it is a defence-in-depth claim and a single-guard mutant cannot be killed by design.
- A first draft of `browser-forced-colors-outline` survived because Chromium matches `:focus-visible` after a scripted focus (AGENTS.md, WebKit fact 2, the same difference in reverse), so the live control kept its outline from the other rule. The test now also asserts on an unfocused probe carrying only `data-focus-return`, which only the marker rule can outline. A second draft of the script's FAIL-line regex missed the browser project's `|chromium|` prefix and reported a false survivor; it now accepts it.

## Verification run (Story B)

All commands sequential, never in parallel, from the worktree, `PATH` prepended with `/opt/homebrew/bin` and `~/go/bin`. `wails build` was **not** run: no exported `App` command changed, and the committed `frontend/wailsjs/` already carries `SelectReceiveFolder`, `StartReceive`, `ShowReceivedFolder` and `ReceiveMetadata` from Story A. Transcripts are in `evidence-phone-to-desktop-receiving-frontend-logs/gate/`.

| Step | Result |
| --- | --- |
| `npm ci` (frontend) | pass |
| `npm test` before Story B (baseline) | 22 files, 863 tests passed |
| `npx tsc --noEmit` / `npm run build` | pass |
| `npm test` | 34 files, 1253 tests passed |
| `npm run test:browser` | 4 files, 123 tests passed (was 86 before Story B; 37 new in `browser/receive.test.tsx`); `receiver-live.mjs` passed; `upload-live.mjs` passed |
| `git checkout -- frontend/browser/captures/` | run; the capture PNG was not left modified |
| `go test -count=1 ./...` | all 10 packages `ok` (Go untouched) |
| `gofmt -l .` | empty |
| `python3 scripts/verify-receive-frontend-mutations.py` | exit 0, 60 of 60 |

## Existing tests touched

No existing assertion was weakened or removed. Extensions only: `App.test.tsx` and `App.focus.test.tsx` gained the two new controller members (`receive`, `showReceivedFolder`) in their typed command objects (D-072 turns a missing member into a compile error); `useTransfer.test.tsx` now expects six lifecycle subscriptions instead of five (`transfer-notice`), with the StrictMode counts doubled to match; `announce.test.ts` lists the three new focus targets; `copy.test.ts` lists the new copy key paths and, through the EXPERIENCE table, the 20 new rows; `noButtonEllipsis.test.tsx` gained a receive describe block. Every other pre-existing test file ran unmodified and green.

## Decisions that go beyond the spec text (Story B)

1. **Receive has its own state phases** (`receive-waiting`, `receive-receiving`, `receive-outcome`; a pending receive reuses `pending` with `itemKind: 'receive'`) rather than a flag on the send ones, so no send affordance can attach to a receive session by construction. A receive outcome is never retained in Idle.
2. **A receive failure to start carries `commandContext: 'receive'`** on the Idle state so the failure card reads `receiveErrorCopy` and offers the folder chooser (Try Again / Choose Another Folder), never the send browse menu. No new error code; the backend's existing codes are re-worded on the frontend only, as the contract anticipated.
3. **A success whose terminal snapshot cannot be reconciled with the progress already accepted is shown as incomplete**, never complete. A receive terminal event with a malformed or missing receive payload is refused (the user can still Cancel). A `cancelled` result must arrive with the `cancelled` code; any contradiction is refused, not guessed at.
4. **Strict wire validation**: the receive payload is pinned to exactly its four keys, `destination` must be a bounded display name without control characters, and the URL must be the `/upload/` route. A volume root's separators are allowed because it has no basename.
5. **"Upload" is allowed in receive copy** (EXPERIENCE vocabulary rule amended); refusal copy says "OneDrive-style placeholder folders" instead of "cloud". The spec's spelling "cancelled" is kept in the two outcome headings; older transfer copy keeps "canceled".
6. **Leaving the outcome moves focus to the Idle instruction** (new routing row `receive-leave`): nothing stays mounted to hold focus, unlike a send outcome.
7. **`ProgressRing`, `OutcomeIcon` and `rise` were exported** from existing view modules and `OutcomePanel` gained optional `receive` and `retryLabel` props; send views render identically when they are absent.

## Unproven here (Story B)

- **Native WKWebView and WebView2 interaction.** Nothing here ran the built Wails binary. Specifically unobserved: the folder chooser round trip and focus return on macOS (WebKit does not focus a clicked button; the code focuses explicitly and uses the `data-focus-return` marker, which Chromium cannot show failing), Tab reaching the Receive button, Show in Folder opening Finder or Explorer, and the QR drag guard (pinned as mechanism only, as for Staged).
- **Screen readers.** The one-owner routing is proven against the table in jsdom; VoiceOver and Narrator were not run.
- **A real phone upload end to end through the UI.** The frontend was driven by fabricated events matching the Go-pinned JSON; the Go side was proven separately by Story A.
- **Windows.** Frontend suites are platform-neutral and ran on macOS only.
- Event ordering race: a lifecycle event that reaches the frontend before the `StartReceive` command resolves is dropped (as it is for a send); not changed here.
