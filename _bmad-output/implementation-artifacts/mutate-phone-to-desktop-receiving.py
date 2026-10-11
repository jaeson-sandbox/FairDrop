"""Canonical, scoped mutation inventory for phone-to-desktop receiving (Story A).

Run from the repository root with
`python3 _bmad-output/implementation-artifacts/mutate-phone-to-desktop-receiving.py [case-id ...]`.
With no ids every case for this platform runs; ids narrow a run for iteration and
prove only the cases they name.

Every case derives from CASES below -- there is no second, hand-copied list. Each
case runs one focused top-level test twice in an isolated copy of the tree: once
unmodified (it must pass) and once with exactly one anchored source edit applied
(it must fail, and the failing test's own transcript must contain the named
assertion fragment). `scripts/mutationverdict` rejects build failures, panics,
skips, timeouts and an assertion that appears only in some other test's output.
Full transcripts are kept beside the evidence file.

Platform-specific guarantees (no-replace rename, exclusive creation, download
marking) have one anchor per platform, chosen by the host this runs on. A scoped
run proves only the cases for its own platform; the macOS and Windows CI jobs are
where the other platforms' anchors are exercised.
"""

from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[2]
LOGS = ROOT / "_bmad-output/implementation-artifacts/evidence-phone-to-desktop-receiving-logs"

PLATFORM = {"darwin": "darwin", "linux": "linux", "win32": "windows"}.get(sys.platform, sys.platform)

# (id, file, exact original, exact mutant, focused package, focused test, assertion fragment)
COMMON = [
    # --- storage safety (internal/sink) ---
    ("reserve_boundary", "internal/sink/sink.go", "return available-uint64(declared) >= uint64(ReserveBytes)", "return available-uint64(declared) > uint64(ReserveBytes)", "./internal/sink", "TestTheSpaceReserveBoundary", "exactly the reserve remains"),
    ("reserve_constant", "internal/sink/sink.go", "ReserveBytes int64 = 3 << 30", "ReserveBytes int64 = 2 << 30", "./internal/sink", "TestCheckSpaceUsesTheVolumeAnswerAndTheThreeGiBReserve", "want exactly 3 GiB"),
    ("partial_removal", "internal/sink/sink.go", "written, markFailed, err := d.writeAndClose(ctx, sub, file, tempName, content)\n\tif err != nil {\n\t\td.discardTemp(sub, tempName)\n", "written, markFailed, err := d.writeAndClose(ctx, sub, file, tempName, content)\n\tif err != nil {\n", "./internal/sink", "TestAReadErrorMidFileRemovesOnlyThePartialFile", "want only the completed file"),
    ("empty_subfolder", "internal/sink/sink.go", "\t\tif saved == 0 {\n", "\t\tif saved < 0 {\n", "./internal/sink", "TestAFirstFileFailureLeavesNoSubfolderOnceTheDestinationCloses", "a session that saved nothing left"),
    ("bounded_buffer", "internal/sink/sink.go", "copyBufferBytes = 64 << 10", "copyBufferBytes = 1 << 20", "./internal/sink", "TestAFileIsCopiedThroughOneBoundedBufferAndNothingGoesToOSTempStorage", "want exactly 64 KiB"),
    ("sanitize_separators", "internal/sink/naming.go", "strings.LastIndexAny(raw, `/\\`)", "strings.LastIndexAny(raw, \"/\")", "./internal/sink", "TestSanitizeNameStripsBothSeparatorsOnEveryHost", 'want "evil.dll"'),
    ("dedupe_case", "internal/sink/naming.go", "key := strings.ToLower(candidate)", "key := candidate", "./internal/sink", "TestDuplicateNamesAreNumberedBeforeTheExtensionCaseInsensitively", 'want "X (4).JPG"'),
    # --- HTTP receive (internal/server) ---
    ("overflow_abort", "internal/server/upload.go", "if b.read > b.declared {", "if false {", "./internal/server", "TestTheServerItselfAbortsWhenTheBodyExceedsItsDeclaredLength", "want incomplete with exactly 1 file saved"),
    ("inactivity_refresh", "internal/server/upload.go", "\t\t\tb.meter.record(n)\n\t\t\tb.refreshDeadline()\n", "\t\t\tb.meter.record(n)\n", "./internal/server", "TestAnUploadLongerThanTheWholeRequestDeadlineStillCompletes", "inactivity bound must never have fired"),
    ("inactivity_deadline", "internal/server/upload.go", "_ = b.controller.SetReadDeadline(time.Now().Add(b.idle))", "_ = b.idle", "./internal/server", "TestAnUploadThatStallsIsEndedByTheInactivityBound", "no terminal event within 10s"),
    ("multipart_streaming", "internal/server/upload.go", "multi, err := request.MultipartReader()\n\tif err != nil {", "multi, err := request.MultipartReader()\n\tif err == nil {\n\t\t_, _ = multi.ReadForm(1 << 20)\n\t}\n\tif err != nil {", "./internal/server", "TestALargeUploadStreamsWithBoundedMemoryAndNothingInOSTempStorage", "status = 400, want 200"),
    ("multipart_streaming_source", "internal/server/upload.go", "multi, err := request.MultipartReader()\n\tif err != nil {", "multi, err := request.MultipartReader()\n\tif err == nil {\n\t\t_, _ = multi.ReadForm(1 << 20)\n\t}\n\tif err != nil {", "./internal/server", "TestTheServerNeverBuffersAMultipartBody", "buffers a multipart body"),
    ("space_refusal", "internal/server/upload.go", "if err := r.destination.CheckSpace(declared); err != nil {", "if false {", "./internal/server", "TestAnUploadTooLargeForTheDiskIsRefusedNotedAndLeavesTheSessionWaiting", "want 413"),
    ("space_notice", "internal/server/upload.go", "r.lane.publishProgress(noticeEvent(r.sessionID, transfer.NoticeReceiveTooLarge))", "_ = transfer.NoticeReceiveTooLarge", "./internal/server", "TestAnUploadTooLargeForTheDiskIsRefusedNotedAndLeavesTheSessionWaiting", "no notice within 10s"),
    ("length_required", "internal/server/upload.go", 'if request.ContentLength < 0 || (request.ContentLength == 0 && request.Header.Get("Content-Length") == "") {', "if false {", "./internal/server", "TestRejectedRequestsWriteNothingAndLeaveTheSessionWaiting", "want 411"),
    ("claimed_locked", "internal/server/upload.go", "\tif r.claimed.Load() {\n\t\twriteStatus(writer, http.StatusLocked)\n\t\treturn\n\t}\n\tif request.Method == http.MethodGet {\n\t\tr.uploadPage", "\tif false {\n\t\twriteStatus(writer, http.StatusLocked)\n\t\treturn\n\t}\n\tif request.Method == http.MethodGet {\n\t\tr.uploadPage", "./internal/server", "TestASecondRequestWhileAnUploadIsInFlightGets423AndWritesNothing", "during an upload"),
    ("complete_not_demoted", "internal/server/lifecycle.go", "event.Kind == transfer.ServerComplete && r.destination == nil {", "event.Kind == transfer.ServerComplete {", "./internal/server", "TestAFailedResultPageWriteDoesNotDemoteACompleteUpload", "want complete"),
    # --- coordinator lifecycle (internal/transfer) ---
    ("receive_admission_idle", "internal/transfer/coordinator.go", '\tif c.state != stateIdle {\n\t\tc.mu.Unlock()\n\t\treturn nil, NewError(ErrBusy, "a transfer is already in progress")', '\tif false {\n\t\tc.mu.Unlock()\n\t\treturn nil, NewError(ErrBusy, "a transfer is already in progress")', "./internal/transfer", "TestStartReceiveIsAdmittedOnlyFromIdle", "want busy"),
    ("cancel_outcome", "internal/transfer/lifecycle.go", "holdOutcome := announce && live.receive != nil", "holdOutcome := false && announce && live.receive != nil", "./internal/transfer", "TestCancellingAnUploadInFlightPublishesACancelledOutcomeAndHoldsIt", "want started then one error and no reset"),
    ("destination_close", "internal/transfer/coordinator.go", "\t\tcase resourceDestination:\n\t\t\terr = c.closeDestinationBounded(live)\n", "\t\tcase resourceDestination:\n\t\t\terr = nil\n", "./internal/transfer", "TestCancellingAWaitingReceiveWritesNothingAndResets", "the destination was closed"),
    ("outcome_held", "internal/transfer/outcomes.go", "\tif live.receive == nil {\n\t\t// A receive outcome is held", "\tif true {\n\t\t// A receive outcome is held", "./internal/transfer", "TestACompleteUploadPublishesProgressThenAnOutcomeAndHoldsIt", "want it held until the user leaves it"),
    ("show_refuses_without_subfolder", "internal/transfer/receive.go", "if !exists || folder == \"\" {", "if exists && folder == \"\" {", "./internal/transfer", "TestReceivedFolderRefusesWhenThereIsNoSubfolder", "ReceivedFolder before any file was saved"),
    # --- bound commands (app.go) ---
    ("show_in_folder_no_shell", "app.go", 'return "open", []string{dir}, nil', 'return "sh", []string{"-c", "open " + dir}, nil', ".", "TestTheFileManagerCommandIsAnArgumentVectorNeverAShellLine", 'want "open"'),
    # --- review patches (second pass) ---
    ("long_path_prefix_drive", "internal/sink/longpath.go", r"return `\\?\` + path", "return path", "./internal/sink", "TestExtendedLengthPathSpellsDrivesAndUNCSharesAndLeavesTheRestAlone", "extendedLengthPath("),
    ("long_path_prefix_unc", "internal/sink/longpath.go", r"return `\\?\UNC\` + path[2:]", "return path", "./internal/sink", "TestExtendedLengthPathSpellsDrivesAndUNCSharesAndLeavesTheRestAlone", "extendedLengthPath("),
    ("deadline_refresh_after_save", "internal/server/upload.go", "\t\tbody.refreshDeadline()\n\t\t// Each saved file is an event", "\t\t// Each saved file is an event", "./internal/server", "TestASlowSaveBetweenPartsDoesNotConsumeTheNextPartsInactivityBound", "a slow save is not a stalled phone"),
    ("raw_part", "internal/server/upload.go", "part, err := multi.NextRawPart()", "part, err := multi.NextPart()", "./internal/server", "TestAQuotedPrintablePartIsSavedAsSentNotDecoded", "want exactly the quoted-printable text that was sent"),
    ("race_exactly_one_terminal", "internal/transfer/lifecycle.go", "holdOutcome := announce && live.receive != nil", "holdOutcome := false && announce && live.receive != nil", "./internal/transfer", "TestACancelRacingACompletionPublishesExactlyOneTerminalOutcome", "want exactly 1"),
    ("close_bound_subfolder", "internal/transfer/receive.go", "snapshot.SubfolderExists = snapshot.SubfolderExists && snapshot.FilesSaved > 0", "_ = snapshot.FilesSaved", "./internal/transfer", "TestACloseBoundThatElapsesNeverClaimsAnEmptySubfolderExists", "outcome Receive ="),
    # --- inventory gaps closed (second pass) ---
    ("commit_exist_retry", "internal/sink/sink.go", "\t\tif !errors.Is(err, fs.ErrExist) {\n\t\t\td.mu.Unlock()\n\t\t\treturn transfer.WrapError(transfer.ErrTransferFailed, \"a received file could not be put in place\", err)", "\t\tif true {\n\t\t\td.mu.Unlock()\n\t\t\treturn transfer.WrapError(transfer.ErrTransferFailed, \"a received file could not be put in place\", err)", "./internal/sink", "TestAFinalNameTakenBeforeTheRenameIsNeverReplaced", "want it saved"),
    ("sanitize_device_prefix", "internal/sink/naming.go", "\t\tname = \"_\" + name\n", "\t\t_ = name\n", "./internal/sink", "TestSanitizeNameTable", 'want "_CON"'),
    ("sanitize_control_removal", "internal/sink/naming.go", "case unicode.IsControl(char), unicode.Is(unicode.Cf, char),", "case unicode.Is(unicode.Cf, char),", "./internal/sink", "TestSanitizeNameTable", 'want "name.txt"'),
    ("sanitize_bidi_removal", "internal/sink/naming.go", "case unicode.IsControl(char), unicode.Is(unicode.Cf, char),", "case unicode.IsControl(char),", "./internal/sink", "TestSanitizeNameTable", 'want "reportgpj.exe"'),
    ("dedupe_hint_never_skips", "internal/sink/naming.go", "s.next[hint] = attempt + 1", "s.next[hint] = attempt + 2", "./internal/sink", "TestManyDuplicatesAreNumberedInOrderAndAnInterveningClaimIsSteppedOver", "claim("),
    ("sanitize_fallback", "internal/sink/naming.go", 'fallbackName = "file"', 'fallbackName = "unnamed"', "./internal/sink", "TestSanitizeNameTable", 'want "file"'),
    ("short_body_incomplete", "internal/server/upload.go", "if body.read != body.declared {", "if false {", "./internal/server", "TestABodyThatEndsCleanlyShortOfItsDeclaredLengthIsIncompleteNotComplete", "want the incomplete-upload page for a body 500 bytes short"),
    ("upload_wrong_token_404", "internal/server/upload.go", 'if !tokenMatches(request.PathValue("token"), r.token) {', "if false {", "./internal/server", "TestEveryRouteOrMethodThatIsNotTheExactUploadIsTheGenericNotFound", "want the generic 404"),
    ("upload_field_filter", "internal/server/upload.go", 'params["name"] != uploadField {', "false {", "./internal/server", "TestPartsOutsideTheFilesFieldAreDiscardedNeverWritten", "want exactly 1 file saved"),
]

# Anchors shared by darwin and linux, which both build fs_posix.go.
POSIX = [
    ("writable_destination_at_open", "internal/sink/fs_posix.go", 'if err := unix.Faccessat(current, ".", unix.W_OK|unix.X_OK, 0); err != nil {', "if err := error(nil); err != nil {", "./internal/sink", "TestOpenDestinationRefusesAFolderTheUserCannotWriteInto", "returned a destination for an unwritable folder"),
    ("writable_destination_native", "internal/sink/fs_posix.go", 'if err := unix.Faccessat(current, ".", unix.W_OK|unix.X_OK, 0); err != nil {', "if err := error(nil); err != nil {", ".", "TestNativeReceiveRefusesAnUnwritableFolderBeforeAnyNetworkResource", "want path_unsupported"),
    ("never_executable_file", "internal/sink/fs_posix.go", "fileMode       = 0o644", "fileMode       = 0o755", "./internal/sink", "TestFilesAndFoldersAreNeverExecutable", "the file mode is"),
    ("never_loose_directory", "internal/sink/fs_posix.go", "directoryMode  = 0o755", "directoryMode  = 0o777", "./internal/sink", "TestFilesAndFoldersAreNeverExecutable", "the folder mode is"),
    ("destination_link_refusal", "internal/sink/fs_posix.go", "directoryFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_NOFOLLOW | unix.O_CLOEXEC", "directoryFlags = unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC", "./internal/sink", "TestOpenDestinationRefusesWhatItMustAndCreatesNothing", "returned a destination"),
    # macOS and Linux both answer O_NOFOLLOW|O_DIRECTORY on a link with ENOTDIR (measured on macOS), so the
    # link is recognised by the follow-up fstatat; the ELOOP arm is not reachable on either and has no anchor.
    ("link_classification", "internal/sink/fs_posix.go", "status.Mode&unix.S_IFMT == unix.S_IFLNK {\n\t\t\treturn errLinkLike", "status.Mode&unix.S_IFMT == unix.S_IFLNK {\n\t\t\treturn errUnusable", "./internal/sink", "TestARefusedDestinationIsDescribedByWhyItWasRefused", "OpenDestination error ="),
]

BY_PLATFORM = {
    "darwin": POSIX + [
        ("no_replace_rename", "internal/sink/rename_darwin.go", "unix.RenameatxNp(dirfd, from, dirfd, to, unix.RENAME_EXCL)", "unix.RenameatxNp(dirfd, from, dirfd, to, 0)", "./internal/sink", "TestRenameNoReplaceNeverReplacesAnExistingFile", "want fs.ErrExist"),
        ("exclusive_subfolder", "internal/sink/fs_posix.go", "if err := unix.Mkdirat(d.fd, name, directoryMode); err != nil {\n\t\treturn nil, err\n\t}", "if err := unix.Mkdirat(d.fd, name, directoryMode); err != nil && !errors.Is(err, fs.ErrExist) {\n\t\treturn nil, err\n\t}", "./internal/sink", "TestAnExistingSubfolderNameGetsANumberAndNothingInItChanges", "destination holds"),
        ("exclusive_file", "internal/sink/fs_posix.go", "unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, fileMode)", "unix.O_WRONLY|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, fileMode)", "./internal/sink", "TestCreateExclusiveNeverOpensAnExistingName", "createExclusive opened a file that already existed"),
        ("quarantine_marking", "internal/sink/mark_darwin.go", "return unix.Fsetxattr(int(file.Fd()), quarantineAttribute, quarantineValue(now), 0)", "_ = unix.Fsetxattr\n\t_ = quarantineValue\n\treturn nil", "./internal/sink", "TestSavedFilesCarryTheQuarantineAttributeOnMacOS", "want the attribute present on a saved file"),
    ],
    "linux": POSIX + [
        ("no_replace_rename", "internal/sink/rename_linux.go", "unix.Renameat2(dirfd, from, dirfd, to, unix.RENAME_NOREPLACE)", "unix.Renameat2(dirfd, from, dirfd, to, 0)", "./internal/sink", "TestRenameNoReplaceNeverReplacesAnExistingFile", "want fs.ErrExist"),
        ("exclusive_subfolder", "internal/sink/fs_posix.go", "if err := unix.Mkdirat(d.fd, name, directoryMode); err != nil {\n\t\treturn nil, err\n\t}", "if err := unix.Mkdirat(d.fd, name, directoryMode); err != nil && !errors.Is(err, fs.ErrExist) {\n\t\treturn nil, err\n\t}", "./internal/sink", "TestAnExistingSubfolderNameGetsANumberAndNothingInItChanges", "destination holds"),
        ("exclusive_file", "internal/sink/fs_posix.go", "unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, fileMode)", "unix.O_WRONLY|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC, fileMode)", "./internal/sink", "TestCreateExclusiveNeverOpensAnExistingName", "createExclusive opened a file that already existed"),
    ],
    "windows": [
        ("no_replace_rename", "internal/sink/fs_windows.go", "windows.MoveFileEx(source, target, 0)", "windows.MoveFileEx(source, target, windows.MOVEFILE_REPLACE_EXISTING)", "./internal/sink", "TestRenameNoReplaceNeverReplacesAnExistingFile", "want fs.ErrExist"),
        ("exclusive_subfolder", "internal/sink/fs_windows.go", "if err := os.Mkdir(target, 0o755); err != nil {\n\t\treturn nil, err\n\t}", "if err := os.Mkdir(target, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {\n\t\treturn nil, err\n\t}", "./internal/sink", "TestAnExistingSubfolderNameGetsANumberAndNothingInItChanges", "destination holds"),
        ("exclusive_file", "internal/sink/fs_windows.go", "os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)", "os.O_WRONLY|os.O_CREATE, 0o644)", "./internal/sink", "TestCreateExclusiveNeverOpensAnExistingName", "createExclusive opened a file that already existed"),
        ("writable_destination_at_open", "internal/sink/fs_windows.go", "if err := probeWritable(clean); err != nil {", "if err := error(nil); err != nil {", "./internal/sink", "TestOpenDestinationRefusesAFolderTheUserCannotWriteInto", "returned a destination for an unwritable folder"),
        ("writable_destination_native", "internal/sink/fs_windows.go", "if err := probeWritable(clean); err != nil {", "if err := error(nil); err != nil {", ".", "TestNativeReceiveRefusesAnUnwritableFolderBeforeAnyNetworkResource", "want path_unsupported"),
        ("writable_probe_removed", "internal/sink/fs_windows.go", "return os.Remove(probe)", "return nil", "./internal/sink", "TestOpeningAWritableFolderLeavesItUntouched", "opening a writable folder changed it"),
        ("long_path_rename_source", "internal/sink/fs_windows.go", "windows.UTF16PtrFromString(extendedLengthPath(d.child(from)))", "windows.UTF16PtrFromString(d.child(from))", "./internal/sink", "TestRenameNoReplaceWorksOnAFinalPathLongerThanMaxPath", "want it renamed"),
        ("long_path_rename_target", "internal/sink/fs_windows.go", "windows.UTF16PtrFromString(extendedLengthPath(d.child(to)))", "windows.UTF16PtrFromString(d.child(to))", "./internal/sink", "TestAFileWhoseFinalPathExceedsMaxPathIsSavedNotDiscarded", "want it saved"),
        ("mark_of_the_web", "internal/sink/fs_windows.go", "func (d *windowsDir) mark(_ *os.File, name string, _ time.Time) error {\n", "func (d *windowsDir) mark(_ *os.File, name string, _ time.Time) error {\n\tif name != \"\" {\n\t\treturn nil\n\t}\n", "./internal/sink", "TestSavedFilesCarryTheMarkOfTheWebOnWindows", "want it present on a saved file"),
    ],
}

CASES = COMMON + BY_PLATFORM.get(PLATFORM, [])


def run(directory: Path, case_id: str, phase: str, package: str, test: str, assertion: str) -> int:
    command = ["go", "test", "-json", "-count=1", "-timeout=300s", package, "-run", f"^{test}$"]
    result = subprocess.run(command, cwd=directory, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    log = LOGS / f"{case_id}-{phase}.log"
    log.write_text(result.stdout, encoding="utf-8")
    verdict = subprocess.run([
        "go", "run", "./scripts/mutationverdict", f"-mode={'baseline' if phase == 'baseline' else 'mutation'}",
        f"-test={test}", f"-assert={assertion if phase == 'mutant' else ''}",
        f"-status={result.returncode}", f"-log={log}",
    ], cwd=directory, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
    if verdict.returncode != 0:
        print(f"{case_id}-{phase}: verdict rejected: {verdict.stdout.strip()}")
    return verdict.returncode


def main() -> None:
    LOGS.mkdir(exist_ok=True)
    ids = [case[0] for case in CASES]
    if len(ids) != len(set(ids)):
        raise RuntimeError("duplicate case id in the inventory")
    # Optional case ids narrow the run for iteration. A scoped run proves only the
    # cases it executed and does not rewrite summary.txt.
    wanted = set(sys.argv[1:])
    if wanted - set(ids):
        raise SystemExit(f"unknown case(s) for platform {PLATFORM}: {', '.join(sorted(wanted - set(ids)))}")
    selected = [case for case in CASES if not wanted or case[0] in wanted]
    with tempfile.TemporaryDirectory(prefix="fairdrop-receive-mutations-") as location:
        snapshot = Path(location) / "repo"

        def ignore(directory: str, names):
            relative = Path(directory).relative_to(ROOT)
            excluded = {".git", ".claude", "build", "_bmad-output", "node_modules", "__pycache__", ".DS_Store"}
            if relative == Path("frontend"):
                excluded.update({"src", "browser"})
            return set(names) & excluded

        shutil.copytree(ROOT, snapshot, ignore=ignore)
        results = []
        for case_id, relative, original, mutant, package, test, assertion in selected:
            path = snapshot / relative
            source = path.read_text(encoding="utf-8")
            if source.count(original) != 1:
                raise RuntimeError(f"{case_id}: expected one exact anchor, found {source.count(original)}")
            baseline = run(snapshot, case_id, "baseline", package, test, assertion)
            path.write_text(source.replace(original, mutant, 1), encoding="utf-8")
            mutated = run(snapshot, case_id, "mutant", package, test, assertion)
            path.write_text(source, encoding="utf-8")
            results.append((case_id, baseline, mutated))
        summary = f"platform={PLATFORM} cases={len(results)}\n" + "\n".join(
            f"{case_id}: baseline={'pass' if baseline == 0 else 'INVALID'} mutant={'named assertion' if mutated == 0 else 'INVALID'}"
            for case_id, baseline, mutated in results
        ) + "\n"
        if not wanted:
            (LOGS / "summary.txt").write_text(summary, encoding="utf-8")
        print(summary, end="")
        if any(baseline != 0 or mutated != 0 for _, baseline, mutated in results):
            raise SystemExit(1)


if __name__ == "__main__":
    main()
